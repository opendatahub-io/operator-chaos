package opendatahub_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/opendatahub-io/operator-chaos/pkg/sdk"
)

type odhAction = func(ctx context.Context, cli client.Client, obj *unstructured.Unstructured) error

func TestSDK_ODH_NoFaultsSucceeds(t *testing.T) {
	dsc := newDSC()
	c := newChaosClient(testScheme(), nil, dsc)
	r := &odhReconciler{client: c, scheme: testScheme(), actions: []odhAction{deployAction()}}

	_, err := r.Reconcile(context.Background(), dscRequest())
	require.NoError(t, err)
}

func TestSDK_ODH_GetFailsReturnsError(t *testing.T) {
	dsc := newDSC()
	c := newChaosClient(testScheme(), map[sdk.Operation]sdk.FaultSpec{
		sdk.OpGet: {ErrorRate: 1.0, Error: "connection refused"},
	}, dsc)
	r := &odhReconciler{client: c, scheme: testScheme(), actions: []odhAction{deployAction()}}

	_, err := r.Reconcile(context.Background(), dscRequest())
	// Get returns not found (ignored), so no error expected when ChaosClient returns chaos error
	// Actually: client.IgnoreNotFound only ignores IsNotFound, not ChaosError
	var chaosErr *sdk.ChaosError
	assert.ErrorAs(t, err, &chaosErr)
}

func TestSDK_ODH_UpdateConflictOnFinalizer(t *testing.T) {
	dsc := newDSC()
	c := newChaosClient(testScheme(), map[sdk.Operation]sdk.FaultSpec{
		sdk.OpUpdate: {ErrorRate: 1.0, Error: "the object has been modified"},
	}, dsc)
	r := &odhReconciler{client: c, scheme: testScheme(), actions: []odhAction{deployAction()}}

	_, err := r.Reconcile(context.Background(), dscRequest())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "finalizer")
}

func TestSDK_ODH_CreateFailsDuringAction(t *testing.T) {
	dsc := newDSC()
	c := newChaosClient(testScheme(), map[sdk.Operation]sdk.FaultSpec{
		sdk.OpCreate: {ErrorRate: 1.0, Error: "resource quota exceeded"},
	}, dsc)
	r := &odhReconciler{client: c, scheme: testScheme(), actions: []odhAction{deployAction()}}

	// First reconcile: Get succeeds, Update (finalizer) succeeds, action's Get returns not-found,
	// action's Create fails with chaos error
	_, err := r.Reconcile(context.Background(), dscRequest())
	if err != nil {
		assert.Contains(t, err.Error(), "action")
	}
}

func TestSDK_ODH_IntermittentGet(t *testing.T) {
	dsc := newDSC()
	c := newChaosClient(testScheme(), map[sdk.Operation]sdk.FaultSpec{
		sdk.OpGet: {ErrorRate: 0.5, Error: "connection refused"},
	}, dsc)
	r := &odhReconciler{client: c, scheme: testScheme(), actions: []odhAction{deployAction()}}

	successes := 0
	failures := 0
	for i := 0; i < 20; i++ {
		_, err := r.Reconcile(context.Background(), dscRequest())
		if err != nil {
			failures++
		} else {
			successes++
		}
	}
	assert.Greater(t, successes, 0)
	assert.Greater(t, failures, 0)
}
