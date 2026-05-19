package opendatahub_test

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/opendatahub-io/operator-chaos/pkg/sdk"
	"github.com/opendatahub-io/operator-chaos/pkg/sdk/fuzz"
)

const (
	dscFinalizer  = "platform.opendatahub.io/finalizer"
	dsciFinalizer = "dscinitialization.opendatahub.io/finalizer"
)

var (
	dscGVK = schema.GroupVersionKind{
		Group:   "datasciencecluster.opendatahub.io",
		Version: "v1",
		Kind:    "DataScienceCluster",
	}
	dsciGVK = schema.GroupVersionKind{
		Group:   "dscinitialization.opendatahub.io",
		Version: "v1",
		Kind:    "DSCInitialization",
	}
)

// odhReconciler mimics the opendatahub-operator's generic Reconciler pattern.
type odhReconciler struct {
	client  client.Client
	scheme  *runtime.Scheme
	actions []func(ctx context.Context, cli client.Client, obj *unstructured.Unstructured) error
}

func (r *odhReconciler) Reconcile(ctx context.Context, req reconcile.Request) (ctrl.Result, error) {
	obj := &unstructured.Unstructured{}
	obj.SetGroupVersionKind(dscGVK)
	if err := r.client.Get(ctx, req.NamespacedName, obj); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	if !obj.GetDeletionTimestamp().IsZero() {
		if controllerutil.ContainsFinalizer(obj, dscFinalizer) {
			controllerutil.RemoveFinalizer(obj, dscFinalizer)
			if err := r.client.Update(ctx, obj); err != nil {
				return ctrl.Result{}, fmt.Errorf("failed to remove finalizer: %w", err)
			}
		}
		return ctrl.Result{}, nil
	}

	if !controllerutil.ContainsFinalizer(obj, dscFinalizer) {
		controllerutil.AddFinalizer(obj, dscFinalizer)
		if err := r.client.Update(ctx, obj); err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to add finalizer: %w", err)
		}
	}

	for i, action := range r.actions {
		if err := action(ctx, r.client, obj); err != nil {
			return ctrl.Result{}, fmt.Errorf("action %d failed: %w", i, err)
		}
	}

	return ctrl.Result{}, nil
}

// dsciReconciler mimics DSCInitializationReconciler's pattern.
type dsciReconciler struct {
	client client.Client
	scheme *runtime.Scheme
}

func (r *dsciReconciler) Reconcile(ctx context.Context, req reconcile.Request) (ctrl.Result, error) {
	instance := &unstructured.Unstructured{}
	instance.SetGroupVersionKind(dsciGVK)
	if err := r.client.Get(ctx, req.NamespacedName, instance); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	if !instance.GetDeletionTimestamp().IsZero() {
		if controllerutil.ContainsFinalizer(instance, dsciFinalizer) {
			controllerutil.RemoveFinalizer(instance, dsciFinalizer)
			if err := r.client.Update(ctx, instance); err != nil {
				return ctrl.Result{}, err
			}
		}
		return ctrl.Result{}, nil
	}

	if !controllerutil.ContainsFinalizer(instance, dsciFinalizer) {
		controllerutil.AddFinalizer(instance, dsciFinalizer)
		if err := r.client.Update(ctx, instance); err != nil {
			return ctrl.Result{}, err
		}
	}

	cm := &corev1.ConfigMap{}
	cmKey := types.NamespacedName{Name: "odh-monitoring", Namespace: req.Namespace}
	if err := r.client.Get(ctx, cmKey, cm); err != nil {
		if k8serrors.IsNotFound(err) {
			cm = &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "odh-monitoring",
					Namespace: req.Namespace,
				},
				Data: map[string]string{"config": "default"},
			}
			if err := r.client.Create(ctx, cm); err != nil {
				return ctrl.Result{}, fmt.Errorf("failed to create monitoring configmap: %w", err)
			}
		} else {
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{}, nil
}

func deployAction() func(ctx context.Context, cli client.Client, obj *unstructured.Unstructured) error {
	return func(ctx context.Context, cli client.Client, obj *unstructured.Unstructured) error {
		cm := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "managed-config",
				Namespace: obj.GetNamespace(),
			},
			Data: map[string]string{"deployed": "true"},
		}
		existing := &corev1.ConfigMap{}
		err := cli.Get(ctx, client.ObjectKeyFromObject(cm), existing)
		if k8serrors.IsNotFound(err) {
			return cli.Create(ctx, cm)
		}
		if err != nil {
			return err
		}
		existing.Data = cm.Data
		return cli.Update(ctx, existing)
	}
}

func odhReconcilerFactory(c client.Client) reconcile.Reconciler {
	return &odhReconciler{
		client:  c,
		scheme:  testScheme(),
		actions: []func(ctx context.Context, cli client.Client, obj *unstructured.Unstructured) error{deployAction()},
	}
}

func dsciReconcilerFactory(c client.Client) reconcile.Reconciler {
	return &dsciReconciler{client: c, scheme: testScheme()}
}

func testScheme() *runtime.Scheme {
	s := runtime.NewScheme()
	_ = corev1.AddToScheme(s)
	return s
}

func newDSC() *unstructured.Unstructured {
	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "datasciencecluster.opendatahub.io/v1",
			"kind":       "DataScienceCluster",
			"metadata": map[string]interface{}{
				"name":      "default-dsc",
				"namespace": "opendatahub",
			},
			"spec": map[string]interface{}{
				"components": map[string]interface{}{
					"dashboard": map[string]interface{}{
						"managementState": "Managed",
					},
				},
			},
		},
	}
}

func newDSCI() *unstructured.Unstructured {
	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "dscinitialization.opendatahub.io/v1",
			"kind":       "DSCInitialization",
			"metadata": map[string]interface{}{
				"name":      "default-dsci",
				"namespace": "opendatahub",
			},
			"spec": map[string]interface{}{
				"applicationsNamespace": "opendatahub",
			},
		},
	}
}

func dscRequest() reconcile.Request {
	return reconcile.Request{NamespacedName: types.NamespacedName{Name: "default-dsc", Namespace: "opendatahub"}}
}

func dsciRequest() reconcile.Request {
	return reconcile.Request{NamespacedName: types.NamespacedName{Name: "default-dsci", Namespace: "opendatahub"}}
}

func dscKey() types.NamespacedName {
	return types.NamespacedName{Name: "default-dsc", Namespace: "opendatahub"}
}

func dsciKey() types.NamespacedName {
	return types.NamespacedName{Name: "default-dsci", Namespace: "opendatahub"}
}

func finalizerValid(key types.NamespacedName, gvk schema.GroupVersionKind, validFinalizers []string) fuzz.Invariant {
	return func(ctx context.Context, c client.Client) error {
		obj := &unstructured.Unstructured{}
		obj.SetGroupVersionKind(gvk)
		if err := c.Get(ctx, key, obj); err != nil {
			return nil
		}
		for _, f := range obj.GetFinalizers() {
			valid := false
			for _, vf := range validFinalizers {
				if f == vf {
					valid = true
					break
				}
			}
			if !valid {
				return fmt.Errorf("unexpected finalizer %q on %s", f, key)
			}
		}
		return nil
	}
}

func newChaosClient(scheme *runtime.Scheme, faults map[sdk.Operation]sdk.FaultSpec, objs ...client.Object) client.Client {
	inner := fake.NewClientBuilder().WithScheme(scheme).WithObjects(objs...).Build()
	return sdk.NewChaosClient(inner, sdk.NewFaultConfig(faults))
}
