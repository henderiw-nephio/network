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

package networkconfig

import (
	"context"
	"fmt"
	"reflect"

	"github.com/go-logr/logr"
	ctrlrconfig "github.com/henderiw-nephio/nephio-controllers/controllers/pkg/reconcilers/config"
	configv1alpha1 "github.com/henderiw-nephio/network/apis/config/v1alpha1"
	"github.com/henderiw-nephio/network/pkg/model"
	"github.com/henderiw-nephio/network/pkg/targets"
	reconcilerinterface "github.com/nephio-project/nephio/controllers/pkg/reconcilers/reconciler-interface"
	invv1alpha1 "github.com/nokia/k8s-ipam/apis/inv/v1alpha1"
	"github.com/nokia/k8s-ipam/pkg/meta"
	"github.com/openconfig/gnmic/api"
	"github.com/openconfig/ygot/ygot"
	"github.com/srl-labs/ygotsrl/v22"
	"google.golang.org/protobuf/encoding/prototext"

	//"github.com/nokia/k8s-ipam/pkg/resource"
	"github.com/nephio-project/nephio/controllers/pkg/resource"

	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

func init() {
	reconcilerinterface.Register("networkconfigs", &reconciler{})
}

const (
	finalizer        = "infra.nephio.org/finalizer"
	nokiaSRLProvider = "srl.nokia.com"
	// errors
	errGetCr        = "cannot get cr"
	errUpdateStatus = "cannot update status"
)

//+kubebuilder:rbac:groups=config.alloc.nephio.org,resources=networks,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=config.alloc.nephio.org,resources=networks/status,verbs=get;update;patch

// SetupWithManager sets up the controller with the Manager.
func (r *reconciler) SetupWithManager(ctx context.Context, mgr ctrl.Manager, c interface{}) (map[schema.GroupVersionKind]chan event.GenericEvent, error) {
	cfg, ok := c.(*ctrlrconfig.ControllerConfig)
	if !ok {
		return nil, fmt.Errorf("cannot initialize, expecting controllerConfig, got: %s", reflect.TypeOf(c).Name())
	}

	if err := configv1alpha1.AddToScheme(mgr.GetScheme()); err != nil {
		return nil, err
	}

	r.Client = mgr.GetClient()
	r.finalizer = resource.NewAPIFinalizer(mgr.GetClient(), finalizer)
	r.targets = cfg.Targets

	r.m = &model.Model{
		StructRootType:  reflect.TypeOf((*ygotsrl.Device)(nil)),
		SchemaTreeRoot:  ygotsrl.SchemaTree["Device"],
		JsonUnmarshaler: ygotsrl.Unmarshal,
		EnumData:        ygotsrl.ΛEnum,
	}

	return nil, ctrl.NewControllerManagedBy(mgr).
		Named("NetworkConfigController").
		For(&configv1alpha1.Network{}).
		Complete(r)
}

type reconciler struct {
	client.Client
	finalizer *resource.APIFinalizer

	l       logr.Logger
	targets targets.Target
	m       *model.Model
}

func (r *reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	r.l = log.FromContext(ctx)
	r.l.Info("reconcile", "req", req)

	cr := &configv1alpha1.Network{}
	if err := r.Get(ctx, req.NamespacedName, cr); err != nil {
		// if the resource no longer exists the reconcile loop is done
		if resource.IgnoreNotFound(err) != nil {
			r.l.Error(err, errGetCr)
			return ctrl.Result{}, errors.Wrap(resource.IgnoreNotFound(err), errGetCr)
		}
		return ctrl.Result{}, nil
	}

	if meta.WasDeleted(cr) {
		if err := r.Delete(ctx, cr); err != nil {
			r.l.Error(err, "cannot delete resource on target device")
			cr.SetConditions(configv1alpha1.Failed(err.Error()))
			return ctrl.Result{Requeue: true}, errors.Wrap(r.Status().Update(ctx, cr), errUpdateStatus)
		}

		if err := r.finalizer.RemoveFinalizer(ctx, cr); err != nil {
			r.l.Error(err, "cannot remove finalizer")
			cr.SetConditions(configv1alpha1.Failed(err.Error()))
			return ctrl.Result{Requeue: true}, errors.Wrap(r.Status().Update(ctx, cr), errUpdateStatus)
		}

		r.l.Info("Successfully deleted resource")
		return ctrl.Result{Requeue: false}, nil
	}

	// add finalizer to avoid deleting the token w/o it being deleted from the git server
	if err := r.finalizer.AddFinalizer(ctx, cr); err != nil {
		r.l.Error(err, "cannot add finalizer")
		cr.SetConditions(configv1alpha1.Failed(err.Error()))
		return ctrl.Result{Requeue: true}, errors.Wrap(r.Status().Update(ctx, cr), errUpdateStatus)
	}

	if err := r.Update(ctx, cr); err != nil {
		r.l.Error(err, "cannot update resource")
		cr.SetConditions(configv1alpha1.Failed(err.Error()))
		return ctrl.Result{Requeue: true}, errors.Wrap(r.Status().Update(ctx, cr), errUpdateStatus)
	}

	cr.SetConditions(configv1alpha1.Ready())
	return ctrl.Result{}, errors.Wrap(r.Status().Update(ctx, cr), errUpdateStatus)
}

func (r *reconciler) Update(ctx context.Context, cr *configv1alpha1.Network) error {
	nodeName := cr.Labels[invv1alpha1.NephioNodeNameKey]
	tg := r.targets.Get(types.NamespacedName{Namespace: cr.Namespace, Name: nodeName})
	if tg == nil {
		return fmt.Errorf("no target client available")
	}
	fmt.Println(string(cr.Spec.Config.Raw))
	goStruct, err := r.m.NewConfigStruct(cr.Spec.Config.Raw, true)
	if err != nil {
		r.l.Error(err, "cannot get goStruct")
		return err
	}

	j, err := ygot.EmitJSON(goStruct, &ygot.EmitJSONConfig{
		Format: ygot.RFC7951,
		Indent: "  ",
		RFC7951Config: &ygot.RFC7951JSONConfig{
			AppendModuleName: true,
		},
		SkipValidation: false,
	})
	if err != nil {
		r.l.Error(err, "cannot construct json device info")
		return err
	}

	setReq, err := api.NewSetRequest(
		api.Update(
			api.Path("/"),
			api.Value(j, "json_ietf"),
		))
	if err != nil {
		r.l.Error(err, "cannot create SetRequest")
		return err
	}
	setResp, err := tg.Set(ctx, setReq)
	if err != nil {
		r.l.Error(err, "cannot set config")
		return err
	}
	r.l.Info("update", "resp", prototext.Format(setResp))

	cr.Status.LastAppliedConfig = cr.Spec.Config
	return nil
}

func (r *reconciler) Delete(ctx context.Context, cr *configv1alpha1.Network) error {

	if len(cr.Status.LastAppliedConfig.Raw) == 0 {
		// no config applied tot he device
		return nil
	}

	goStruct, err := r.m.NewConfigStruct(cr.Spec.Config.Raw, true)
	if err != nil {
		r.l.Error(err, "cannot get goStruct")
		return err
	}

	notifications, err := ygot.TogNMINotifications(goStruct, 0, ygot.GNMINotificationsConfig{UsePathElem: true})
	if err != nil {
		r.l.Error(err, "cannot get notifications")
		return nil
	}

	for _, n := range notifications {
		for _, u := range n.GetUpdate() {
			r.l.Info("update", "data", u)
		}
		for _, d := range n.GetDelete() {
			r.l.Info("update", "data", d)
		}
	}

	/*
		nodeName := cr.Labels[invv1alpha1.NephioNodeNameKey]
		tg := r.targets.Get(types.NamespacedName{Namespace: cr.Namespace, Name: nodeName})
		if tg == nil {
			return fmt.Errorf("no target client available")
		}
		setReq, err := api.NewSetRequest(
			api.Delete(
				api.Path("/"),
				api.Value(cr.Status.LastAppliedConfig, "json_ietf"),
			))
		if err != nil {
			return err
		}
		setResp, err := tg.Set(ctx, setReq)
		if err != nil {
			return err
		}
		r.l.Info("update", "resp", prototext.Format(setResp))

		cr.Status.LastAppliedConfig = cr.Spec.Config
	*/
	return nil

}
