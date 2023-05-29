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
	"github.com/henderiw-nephio/network/pkg/rootpaths"
	"github.com/henderiw-nephio/network/pkg/targets"
	commonv1alpha1 "github.com/nephio-project/api/common/v1alpha1"
	reconcilerinterface "github.com/nephio-project/nephio/controllers/pkg/reconcilers/reconciler-interface"
	invv1alpha1 "github.com/nokia/k8s-ipam/apis/inv/v1alpha1"
	"github.com/nokia/k8s-ipam/pkg/meta"
	"github.com/openconfig/gnmi/proto/gnmi"
	"github.com/openconfig/gnmic/utils"
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
		if cr.Spec.Lifecycle.DeletionPolicy == commonv1alpha1.DeletionDelete {
			if err := r.Delete(ctx, cr); err != nil {
				r.l.Error(err, "cannot delete resource on target device")
				cr.SetConditions(configv1alpha1.Failed(err.Error()))
				return ctrl.Result{Requeue: true}, errors.Wrap(r.Status().Update(ctx, cr), errUpdateStatus)
			}
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

	if err := r.Upsert(ctx, cr); err != nil {
		r.l.Error(err, "cannot update resource")
		cr.SetConditions(configv1alpha1.Failed(err.Error()))
		return ctrl.Result{Requeue: true}, errors.Wrap(r.Status().Update(ctx, cr), errUpdateStatus)
	}

	cr.SetConditions(configv1alpha1.Ready())
	return ctrl.Result{}, errors.Wrap(r.Status().Update(ctx, cr), errUpdateStatus)
}

func (r *reconciler) Upsert(ctx context.Context, cr *configv1alpha1.Network) error {
	nodeName := cr.Labels[invv1alpha1.NephioNodeNameKey]
	tg := r.targets.Get(types.NamespacedName{Namespace: cr.Namespace, Name: nodeName})
	if tg == nil {
		return fmt.Errorf("no target client available")
	}

	desiredGoStruct, err := r.m.NewConfigStruct(cr.Spec.Config.Raw, true)
	if err != nil {
		r.l.Error(err, "cannot get goStruct")
		return err
	}

	appliedGoStruct, err := r.m.NewConfigStruct(nil, true)
	if err != nil {
		r.l.Error(err, "cannot get goStruct")
		return err
	}
	if len(cr.Status.LastAppliedConfig.Raw) != 0 {
		// create a new spec
		appliedGoStruct, err = r.m.NewConfigStruct(cr.Status.LastAppliedConfig.Raw, true)
		if err != nil {
			r.l.Error(err, "cannot get goStruct")
			return err
		}
	}

	notification, err := ygot.Diff(appliedGoStruct, desiredGoStruct, &ygot.DiffPathOpt{})
	if err != nil {
		r.l.Error(err, "cannot get notifications")
		return nil
	}

	/*
		opts := []api.GNMIOption{
			api.EncodingASCII(),
		}
	*/
	for _, d := range notification.GetDelete() {
		fmt.Println("deletePath xpath: ", utils.GnmiPathToXPath(d, false))
		fmt.Println("deletePath gpath: ", d)
		//opts = append(opts, api.Delete(d))
	}
	for _, u := range notification.GetUpdate() {
		fmt.Println("updatePath xpath: ", utils.GnmiPathToXPath(u.GetPath(), false))
		fmt.Println("value: ", u.GetVal())
		fmt.Println("uodatePath gpath: ", u.GetPath())
		/*
			opts = append(opts, api.Update(
				api.Path(u.GetPath()),
				api.Value(u.GetVal(), api.EncodingASCII()),
			))
		*/
	}

	/*
		req, err := api.NewSetRequest(opts...)
		if err != nil {
			return err
		}
		setResp, err := tg.Set(ctx, req)
		if err != nil {
			return err
		}
	*/

	// delete the config from the device

	setResp, err := tg.Set(ctx, &gnmi.SetRequest{Prefix: &gnmi.Path{
		Elem: []*gnmi.PathElem{

		},
	}, Delete: notification.GetDelete(), Update: notification.GetUpdate()})
	if err != nil {
		return err
	}

	r.l.Info("update", "resp", prototext.Format(setResp))

	// set the last applied config to nil
	cr.Status.LastAppliedConfig.Raw = cr.Spec.Config.Raw
	return nil
}

func (r *reconciler) Delete(ctx context.Context, cr *configv1alpha1.Network) error {
	// if there is no configutation applied to the device, there is no need to delete it
	if len(cr.Status.LastAppliedConfig.Raw) == 0 {
		// no config applied tot he device
		return nil
	}

	// find the target, if not target is available we cannot delete
	nodeName := cr.Labels[invv1alpha1.NephioNodeNameKey]
	tg := r.targets.Get(types.NamespacedName{Namespace: cr.Namespace, Name: nodeName})
	if tg == nil {
		return fmt.Errorf("no target client available")
	}

	// get go struct from the last applied config
	goStruct, err := r.m.NewConfigStruct(cr.Status.LastAppliedConfig.Raw, true)
	if err != nil {
		r.l.Error(err, "cannot get goStruct")
		return err
	}

	// get notifications from the go struct to calculate the delete paths
	notifications, err := ygot.TogNMINotifications(goStruct, 0, ygot.GNMINotificationsConfig{UsePathElem: true})
	if err != nil {
		r.l.Error(err, "cannot get notifications")
		return nil
	}

	// get the update paths to calculate the delete paths
	gnmiPaths := []*gnmi.Path{}
	for _, n := range notifications {
		for _, u := range n.GetUpdate() {
			gnmiPaths = append(gnmiPaths, u.GetPath())
		}
	}

	// delete the config from the device
	setResp, err := tg.Set(ctx, &gnmi.SetRequest{Delete: rootpaths.GetDeletePaths(gnmiPaths)})
	if err != nil {
		return err
	}
	r.l.Info("update", "resp", prototext.Format(setResp))

	// set the last applied config to nil
	cr.Status.LastAppliedConfig.Raw = nil
	return nil

}
