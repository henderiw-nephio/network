/*
Copyright 2023 The Nephio Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package target

import (
	"context"
	"fmt"
	"reflect"

	"github.com/go-logr/logr"
	ctrlrconfig "github.com/henderiw-nephio/nephio-controllers/controllers/pkg/reconcilers/config"
	"github.com/henderiw-nephio/network/pkg/targets"
	reconcilerinterface "github.com/nephio-project/nephio/controllers/pkg/reconcilers/reconciler-interface"
	"github.com/nephio-project/nephio/controllers/pkg/resource"
	allocv1alpha1 "github.com/nokia/k8s-ipam/apis/alloc/common/v1alpha1"
	invv1alpha1 "github.com/nokia/k8s-ipam/apis/inv/v1alpha1"
	"github.com/openconfig/gnmic/api"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

func init() {
	reconcilerinterface.Register("targets", &reconciler{})
}

const (
	finalizer        = "infra.nephio.org/finalizer"
	nokiaSRLProvider = "srl.nokia.com"
	// errors
	errGetCr        = "cannot get cr"
	errUpdateStatus = "cannot update status"
)

//+kubebuilder:rbac:groups=inv.nephio.org,resources=targets,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=inv.nephio.org,resources=targets/status,verbs=get;update;patch

// SetupWithManager sets up the controller with the Manager.
func (r *reconciler) SetupWithManager(ctx context.Context, mgr ctrl.Manager, c interface{}) (map[schema.GroupVersionKind]chan event.GenericEvent, error) {

	cfg, ok := c.(*ctrlrconfig.ControllerConfig)
	if !ok {
		return nil, fmt.Errorf("cannot initialize, expecting controllerConfig, got: %s", reflect.TypeOf(c).Name())
	}

	if err := invv1alpha1.AddToScheme(mgr.GetScheme()); err != nil {
		return nil, err
	}

	r.Client = mgr.GetClient()
	r.finalizer = resource.NewAPIFinalizer(mgr.GetClient(), finalizer)
	r.targets = cfg.Targets

	return nil, ctrl.NewControllerManagedBy(mgr).
		Named("TargetController").
		For(&invv1alpha1.Target{}).
		Complete(r)
}

type reconciler struct {
	client.Client
	finalizer *resource.APIFinalizer

	l       logr.Logger
	targets targets.Target
}

func (r *reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	r.l = log.FromContext(ctx)
	r.l.Info("reconcile", "req", req)

	cr := &invv1alpha1.Target{}
	if err := r.Get(ctx, req.NamespacedName, cr); err != nil {
		// if the resource no longer exists the reconcile loop is done
		if resource.IgnoreNotFound(err) != nil {
			r.l.Error(err, errGetCr)
			return ctrl.Result{}, errors.Wrap(resource.IgnoreNotFound(err), errGetCr)
		}
		return ctrl.Result{}, nil
	}

	if cr.Labels[invv1alpha1.NephioProviderKey] != nokiaSRLProvider {
		// this is a target that is not acted upon by this contoller
		return ctrl.Result{}, nil
	}

	if resource.WasDeleted(cr) {
		if tg := r.targets.Get(req.NamespacedName); tg != nil {
			if err := tg.Close(); err != nil {
				r.l.Error(err, "cannot close client")
				cr.SetConditions(allocv1alpha1.Failed(err.Error()))
				return ctrl.Result{Requeue: true}, errors.Wrap(r.Status().Update(ctx, cr), errUpdateStatus)
			}

		}
		if err := r.finalizer.RemoveFinalizer(ctx, cr); err != nil {
			r.l.Error(err, "cannot remove finalizer")
			cr.SetConditions(allocv1alpha1.Failed(err.Error()))
			return ctrl.Result{Requeue: true}, errors.Wrap(r.Status().Update(ctx, cr), errUpdateStatus)
		}

		r.l.Info("Successfully deleted resource")
		return ctrl.Result{}, nil
	}
	if err := r.finalizer.AddFinalizer(ctx, cr); err != nil {
		r.l.Error(err, "cannot add finalizer")
		cr.SetConditions(allocv1alpha1.Failed(err.Error()))
		return ctrl.Result{}, errors.Wrap(r.Status().Update(ctx, cr), errUpdateStatus)
	}

	secret := &corev1.Secret{}
	if err := r.Get(ctx, types.NamespacedName{Namespace: cr.GetNamespace(), Name: cr.Spec.SecretName}, secret); err != nil {
		if resource.IgnoreNotFound(err) != nil {
			cr.SetConditions(allocv1alpha1.Failed(err.Error()))
			return ctrl.Result{}, errors.Wrap(r.Status().Update(ctx, cr), errUpdateStatus)
		}
		// TBD How de we handle secret unaviabalility -> do we watch secrets?
		cr.SetConditions(allocv1alpha1.Failed("cannot connect to target, secret not available"))
		return ctrl.Result{}, errors.Wrap(r.Status().Update(ctx, cr), errUpdateStatus)
	}

	if cr.Spec.Address == nil {
		cr.SetConditions(allocv1alpha1.Failed("cannot connect to target, address not available"))
		return ctrl.Result{}, errors.Wrap(r.Status().Update(ctx, cr), errUpdateStatus)
	}
	tg, err := api.NewTarget(
		api.Name(req.NamespacedName.String()),
		api.Address(*cr.Spec.Address),
		api.SkipVerify(true),
		api.Username(string(secret.Data["username"])),
		api.Password(string(secret.Data["password"])),
	)
	if err != nil {
		r.l.Error(err, "cannot create target")
		cr.SetConditions(allocv1alpha1.Failed(err.Error()))
		return ctrl.Result{Requeue: true}, errors.Wrap(r.Status().Update(ctx, cr), errUpdateStatus)
	}
	err = tg.CreateGNMIClient(ctx)
	if err != nil {
		r.l.Error(err, "cannot create client")
		cr.SetConditions(allocv1alpha1.Failed(err.Error()))
		return ctrl.Result{Requeue: true}, errors.Wrap(r.Status().Update(ctx, cr), errUpdateStatus)
	}
	r.targets.Set(req.NamespacedName, tg)
	cr.SetConditions(allocv1alpha1.Ready())
	return ctrl.Result{}, errors.Wrap(r.Status().Update(ctx, cr), errUpdateStatus)
}
