package opendatahub_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opendatahub-io/operator-chaos/pkg/sdk"
)

func TestSDK_DSCI_NoFaultsSucceeds(t *testing.T) {
	dsci := newDSCI()
	c := newChaosClient(testScheme(), nil, dsci)
	r := &dsciReconciler{client: c, scheme: testScheme()}

	_, err := r.Reconcile(context.Background(), dsciRequest())
	require.NoError(t, err)
}

func TestSDK_DSCI_GetFailsReturnsError(t *testing.T) {
	dsci := newDSCI()
	c := newChaosClient(testScheme(), map[sdk.Operation]sdk.FaultSpec{
		sdk.OpGet: {ErrorRate: 1.0, Error: "timeout"},
	}, dsci)
	r := &dsciReconciler{client: c, scheme: testScheme()}

	_, err := r.Reconcile(context.Background(), dsciRequest())
	var chaosErr *sdk.ChaosError
	assert.ErrorAs(t, err, &chaosErr)
}

func TestSDK_DSCI_CreateFailsForConfigMap(t *testing.T) {
	dsci := newDSCI()
	c := newChaosClient(testScheme(), map[sdk.Operation]sdk.FaultSpec{
		sdk.OpCreate: {ErrorRate: 1.0, Error: "resource quota exceeded"},
	}, dsci)
	r := &dsciReconciler{client: c, scheme: testScheme()}

	_, err := r.Reconcile(context.Background(), dsciRequest())
	if err != nil {
		assert.Contains(t, err.Error(), "monitoring configmap")
	}
}

func TestSDK_DSCI_UpdateConflictOnFinalizer(t *testing.T) {
	dsci := newDSCI()
	c := newChaosClient(testScheme(), map[sdk.Operation]sdk.FaultSpec{
		sdk.OpUpdate: {ErrorRate: 1.0, Error: "conflict"},
	}, dsci)
	r := &dsciReconciler{client: c, scheme: testScheme()}

	_, err := r.Reconcile(context.Background(), dsciRequest())
	require.Error(t, err)
}
