package certmanager_test

import (
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	certmanager "github.com/opendatahub-io/operator-chaos/tests/sdk/certmanager"
	"github.com/opendatahub-io/operator-chaos/pkg/sdk"
)

func testScheme() *runtime.Scheme {
	s := runtime.NewScheme()
	_ = corev1.AddToScheme(s)
	_ = appsv1.AddToScheme(s)
	return s
}

func certManagerCR() *unstructured.Unstructured {
	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "operator.openshift.io/v1alpha1",
			"kind":       "CertManager",
			"metadata": map[string]interface{}{
				"name": "cluster",
			},
			"spec": map[string]interface{}{
				"managementState": "Managed",
			},
		},
	}
}

func certManagerRequest() reconcile.Request {
	return reconcile.Request{
		NamespacedName: types.NamespacedName{Name: "cluster"},
	}
}

func certManagerKey() types.NamespacedName {
	return types.NamespacedName{Name: "cluster"}
}

func certManagerReconcilerFactory(c client.Client) reconcile.Reconciler {
	return &certmanager.CertManagerReconciler{
		Client: c,
		Scheme: testScheme(),
	}
}

func newChaosClient(scheme *runtime.Scheme, faults map[sdk.Operation]sdk.FaultSpec, objs ...client.Object) client.Client {
	inner := fake.NewClientBuilder().WithScheme(scheme).WithObjects(objs...).Build()
	return sdk.NewChaosClient(inner, sdk.NewFaultConfig(faults))
}
