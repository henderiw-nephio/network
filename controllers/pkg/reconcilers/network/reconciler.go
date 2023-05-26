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

package network

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"

	"github.com/go-logr/logr"
	ctrlrconfig "github.com/henderiw-nephio/nephio-controllers/controllers/pkg/reconcilers/config"
	infrav1alpha1 "github.com/henderiw-nephio/network/apis/infra/v1alpha1"
	"github.com/henderiw-nephio/network/pkg/targets"
	reconcilerinterface "github.com/nephio-project/nephio/controllers/pkg/reconcilers/reconciler-interface"
	"github.com/nephio-project/nephio/controllers/pkg/resource"
	ipamv1alpha1 "github.com/nokia/k8s-ipam/apis/alloc/ipam/v1alpha1"
	vlanv1alpha1 "github.com/nokia/k8s-ipam/apis/alloc/vlan/v1alpha1"
	invv1alpha1 "github.com/nokia/k8s-ipam/apis/inv/v1alpha1"
	"github.com/nokia/k8s-ipam/pkg/hash"
	"github.com/nokia/k8s-ipam/pkg/proxy/clientproxy"
	"github.com/openconfig/gnmic/api"
	"github.com/openconfig/ygot/ygot"
	"google.golang.org/protobuf/encoding/prototext"

	"github.com/pkg/errors"
	"github.com/srl-labs/ygotsrl/v22"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

func init() {
	reconcilerinterface.Register("networks", &reconciler{})
}

const (
	finalizer        = "infra.nephio.org/finalizer"
	nokiaSRLProvider = "srl.nokia.com"
	// errors
	errGetCr        = "cannot get cr"
	errUpdateStatus = "cannot update status"
)

//+kubebuilder:rbac:groups=infra.nephio.org,resources=networks,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=infra.nephio.org,resources=networks/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=ipam.alloc.nephio.org,resources=networkinstances,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=ipam.alloc.nephio.org,resources=networkinstances/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=ipam.alloc.nephio.org,resources=ipprefixes,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=ipam.alloc.nephio.org,resources=ipprefixes/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=inv.nephio.org,resources=endpoints,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=inv.nephio.org,resources=endpoints/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=inv.nephio.org,resources=targets,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=inv.nephio.org,resources=targets/status,verbs=get;update;patch

// SetupWithManager sets up the controller with the Manager.
func (r *reconciler) SetupWithManager(mgr ctrl.Manager, c interface{}) (map[schema.GroupVersionKind]chan event.GenericEvent, error) {

	cfg, ok := c.(*ctrlrconfig.ControllerConfig)
	if !ok {
		return nil, fmt.Errorf("cannot initialize, expecting controllerConfig, got: %s", reflect.TypeOf(c).Name())
	}

	if err := infrav1alpha1.AddToScheme(mgr.GetScheme()); err != nil {
		return nil, err
	}
	if err := ipamv1alpha1.AddToScheme(mgr.GetScheme()); err != nil {
		return nil, err
	}
	if err := vlanv1alpha1.AddToScheme(mgr.GetScheme()); err != nil {
		return nil, err
	}
	if err := invv1alpha1.AddToScheme(mgr.GetScheme()); err != nil {
		return nil, err
	}

	r.APIPatchingApplicator = resource.NewAPIPatchingApplicator(mgr.GetClient())
	r.finalizer = resource.NewAPIFinalizer(mgr.GetClient(), finalizer)
	r.devices = map[string]*ygotsrl.Device{}
	r.VlanClientProxy = cfg.VlanClientProxy
	r.IpamClientProxy = cfg.IpamClientProxy
	r.targets = cfg.Targets

	return nil, ctrl.NewControllerManagedBy(mgr).
		Named("NetworkController").
		For(&infrav1alpha1.Network{}).
		Complete(r)
}

type reconciler struct {
	resource.APIPatchingApplicator
	finalizer       *resource.APIFinalizer
	IpamClientProxy clientproxy.Proxy[*ipamv1alpha1.NetworkInstance, *ipamv1alpha1.IPAllocation]
	VlanClientProxy clientproxy.Proxy[*vlanv1alpha1.VLANDatabase, *vlanv1alpha1.VLANAllocation]

	l       logr.Logger
	devices map[string]*ygotsrl.Device
	targets targets.Target
}

func (r *reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	r.l = log.FromContext(ctx)
	r.l.Info("reconcile", "req", req)

	cr := &infrav1alpha1.Network{}
	if err := r.Get(ctx, req.NamespacedName, cr); err != nil {
		// if the resource no longer exists the reconcile loop is done
		if resource.IgnoreNotFound(err) != nil {
			r.l.Error(err, errGetCr)
			return ctrl.Result{}, errors.Wrap(resource.IgnoreNotFound(err), errGetCr)
		}
		return ctrl.Result{}, nil
	}

	// TODO validation
	// validate itfce/node or selector
	// validate in rt + bd -> the itfce/node or selector is coming from the bd

	if resource.WasDeleted(cr) {
		if err := r.finalizer.RemoveFinalizer(ctx, cr); err != nil {
			r.l.Error(err, "cannot remove finalizer")
			cr.SetConditions(infrav1alpha1.Failed(err.Error()))
			return ctrl.Result{Requeue: true}, errors.Wrap(r.Status().Update(ctx, cr), errUpdateStatus)
		}

		r.l.Info("Successfully deleted resource")
		return ctrl.Result{Requeue: false}, nil
	}

	// add finalizer to avoid deleting the token w/o it being deleted from the git server
	if err := r.finalizer.AddFinalizer(ctx, cr); err != nil {
		r.l.Error(err, "cannot add finalizer")
		cr.SetConditions(infrav1alpha1.Failed(err.Error()))
		return ctrl.Result{Requeue: true}, errors.Wrap(r.Status().Update(ctx, cr), errUpdateStatus)
	}

	eps, err := r.getProviderEndpoints(ctx, cr.Spec.Topology)
	if err != nil {
		r.l.Error(err, "cannot list provider endpoints")
		cr.SetConditions(infrav1alpha1.Failed(err.Error()))
		return ctrl.Result{Requeue: true}, errors.Wrap(r.Status().Update(ctx, cr), errUpdateStatus)
	}

	if err := r.applyInitialresources(ctx, cr, eps); err != nil {
		r.l.Error(err, "cannot apply initial resources")
		cr.SetConditions(infrav1alpha1.Failed(err.Error()))
		return ctrl.Result{Requeue: true}, errors.Wrap(r.Status().Update(ctx, cr), errUpdateStatus)
	}

	if err := r.getNewResources(ctx, cr, eps); err != nil {
		r.l.Error(err, "cannot get new resources")
		cr.SetConditions(infrav1alpha1.Failed(err.Error()))
		return ctrl.Result{Requeue: true}, errors.Wrap(r.Status().Update(ctx, cr), errUpdateStatus)
	}

	cr.SetConditions(infrav1alpha1.Ready())
	return ctrl.Result{}, errors.Wrap(r.Status().Update(ctx, cr), errUpdateStatus)
}

func (r *reconciler) getProviderEndpoints(ctx context.Context, topology string) (*endpoints, error) {
	opts := []client.ListOption{
		client.MatchingLabels{
			invv1alpha1.NephioProviderKey: nokiaSRLProvider,
			invv1alpha1.NephioTopologyKey: topology,
		},
	}
	eps := &invv1alpha1.EndpointList{}
	if err := r.List(ctx, eps, opts...); err != nil {
		r.l.Error(err, "cannot list endpoints")
		return nil, err
	}
	return &endpoints{eps}, nil
}

func (r *reconciler) applyInitialresources(ctx context.Context, cr *infrav1alpha1.Network, eps *endpoints) error {
	n := &network{
		APIPatchingApplicator: r.APIPatchingApplicator,
		apply:                 true,
		Network:               cr,
		devices:               map[string]*ygotsrl.Device{},
		resources:             map[corev1.ObjectReference]client.Object{},
		eps:                   eps,
		hash:                  hash.New(10000),
	}
	if err := n.PopulateBridgeDomains(ctx); err != nil {
		r.l.Error(err, "cannot populate bridgedomains")
		return err
	}
	if err := n.PopulateRoutingTables(ctx); err != nil {
		r.l.Error(err, "cannot populate routing Tables")
		return err
	}
	return nil
}

func (r *reconciler) getNewResources(ctx context.Context, cr *infrav1alpha1.Network, eps *endpoints) error {
	n := &network{
		Network:         cr,
		devices:         map[string]*ygotsrl.Device{},
		resources:       map[corev1.ObjectReference]client.Object{},
		eps:             eps,
		hash:            hash.New(10000),
		IpamClientProxy: r.IpamClientProxy,
		VlanClientProxy: r.VlanClientProxy,
	}
	if err := n.PopulateBridgeDomains(ctx); err != nil {
		r.l.Error(err, "cannot populate bridgedomains")
		return err
	}
	if err := n.PopulateRoutingTables(ctx); err != nil {
		r.l.Error(err, "cannot populate routing Tables")
		return err
	}

	for nodeName, device := range n.devices {
		r.l.Info("node config", "nodeName", nodeName)

		j, err := ygot.EmitJSON(device, &ygot.EmitJSONConfig{
			Format:        ygot.RFC7951,
			Indent:        "  ",
			RFC7951Config: &ygot.RFC7951JSONConfig{
				AppendModuleName: true,
			},
			SkipValidation: false,
		})
		if err != nil {
			r.l.Error(err, "cannot construct json device info")
			return err
		}
		fmt.Println(j)

		tg := r.targets.Get(types.NamespacedName{Namespace: cr.Namespace, Name: nodeName})
		if tg == nil {
			return fmt.Errorf("no target client available")
		}
		setReq, err := api.NewSetRequest(
			api.Update(
				api.Path("/"),
				api.Value(j, "json_ietf"),
			))
		if err != nil {
			return err
		}
		setResp, err := tg.Set(ctx, setReq)
		if err != nil {
			return err
		}
		fmt.Println(prototext.Format(setResp))

	}
	for resourceName, r := range n.resources {
		fmt.Println(resourceName)
		b, _ := json.MarshalIndent(r, "", "  ")
		fmt.Println(string(b))
	}
	return nil
}
