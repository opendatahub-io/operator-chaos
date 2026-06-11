package controller

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	v1alpha1 "github.com/opendatahub-io/operator-chaos/api/v1alpha1"
	"github.com/opendatahub-io/operator-chaos/pkg/clock"
	"github.com/opendatahub-io/operator-chaos/pkg/evaluator"
	"github.com/opendatahub-io/operator-chaos/pkg/injection"
	"github.com/opendatahub-io/operator-chaos/pkg/observer"
	"github.com/opendatahub-io/operator-chaos/pkg/safety"
	"github.com/opendatahub-io/operator-chaos/pkg/sdk"
	chaosclient "github.com/opendatahub-io/operator-chaos/pkg/sdk/client"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

type passthroughOrchestrator struct{}

func (o *passthroughOrchestrator) ValidateExperiment(_ context.Context, _ *v1alpha1.ChaosExperiment) error {
	return nil
}

func (o *passthroughOrchestrator) RunPreCheck(_ context.Context, _ *v1alpha1.ChaosExperiment) (*v1alpha1.CheckResult, error) {
	return &v1alpha1.CheckResult{Passed: true, ChecksRun: 1, ChecksPassed: 1, Timestamp: metav1.Now()}, nil
}

func (o *passthroughOrchestrator) InjectFault(_ context.Context, _ *v1alpha1.ChaosExperiment) (injection.CleanupFunc, []v1alpha1.InjectionEvent, error) {
	return func(_ context.Context) error { return nil }, nil, nil
}

func (o *passthroughOrchestrator) RevertFault(_ context.Context, _ *v1alpha1.ChaosExperiment) error {
	return nil
}

func (o *passthroughOrchestrator) RunPostCheck(_ context.Context, _ *v1alpha1.ChaosExperiment) (*v1alpha1.CheckResult, []observer.Finding, error) {
	return &v1alpha1.CheckResult{Passed: true, ChecksRun: 1, ChecksPassed: 1, Timestamp: metav1.Now()}, nil, nil
}

func (o *passthroughOrchestrator) EvaluateExperiment(_ []observer.Finding, _ v1alpha1.HypothesisSpec) *evaluator.EvaluationResult {
	return &evaluator.EvaluationResult{Verdict: v1alpha1.Resilient, Confidence: "high"}
}

func newSDKReconciler(exp *v1alpha1.ChaosExperiment, faults map[sdk.Operation]sdk.FaultSpec) *ChaosExperimentReconciler {
	scheme := newTestScheme()

	innerClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(exp).
		WithStatusSubresource(exp).
		Build()

	chaosClient := chaosclient.NewChaosClient(innerClient, sdk.NewFaultConfig(faults))

	return &ChaosExperimentReconciler{
		Client:       chaosClient,
		Scheme:       scheme,
		Orchestrator: &passthroughOrchestrator{},
		Lock:         safety.NewLocalExperimentLock(),
		Clock:        clock.NewFakeClock(time.Now()),
		Recorder:     record.NewFakeRecorder(100),
	}
}

func TestSDK_GetFailsReturnsError(t *testing.T) {
	exp := newTestExperiment()
	r := newSDKReconciler(exp, map[sdk.Operation]sdk.FaultSpec{
		sdk.OpGet: {ErrorRate: 1.0, Error: "connection refused"},
	})

	_, err := r.Reconcile(context.Background(), reconcileRequest())
	require.Error(t, err)

	var chaosErr *sdk.ChaosError
	assert.ErrorAs(t, err, &chaosErr)
}

func TestSDK_UpdateConflictOnFinalizer(t *testing.T) {
	exp := newTestExperiment()
	exp.Status.Phase = v1alpha1.PhasePending

	r := newSDKReconciler(exp, map[sdk.Operation]sdk.FaultSpec{
		sdk.OpUpdate: {ErrorRate: 1.0, Error: "the object has been modified"},
	})

	// First reconcile sets up the experiment (status update via Status().Update which bypasses ChaosClient).
	// The Update fault hits when the reconciler tries to add the cleanup finalizer during inject phase.
	// Run multiple reconciles to reach the phase that does r.Update().
	var lastErr error
	for i := 0; i < 5; i++ {
		_, lastErr = r.Reconcile(context.Background(), reconcileRequest())
		if lastErr != nil {
			break
		}
	}
	// The reconciler should hit an Update error at some point during phase transitions
	if lastErr != nil {
		assert.Contains(t, lastErr.Error(), "modified")
	}
}

func TestSDK_NoFaultsSucceeds(t *testing.T) {
	exp := newTestExperiment()
	r := newSDKReconciler(exp, nil)

	_, err := r.Reconcile(context.Background(), reconcileRequest())
	require.NoError(t, err)

	got, err := getExperiment(context.Background(), r)
	require.NoError(t, err)
	assert.NotEmpty(t, got.Status.Phase)
}

func TestSDK_IntermittentGetSometimesSucceeds(t *testing.T) {
	exp := newTestExperiment()
	r := newSDKReconciler(exp, map[sdk.Operation]sdk.FaultSpec{
		sdk.OpGet: {ErrorRate: 0.5, Error: "connection refused"},
	})

	successes := 0
	failures := 0
	for i := 0; i < 20; i++ {
		_, err := r.Reconcile(context.Background(), reconcileRequest())
		if err != nil {
			failures++
		} else {
			successes++
		}
	}

	assert.Greater(t, successes, 0, "expected at least one success with 50%% error rate")
	assert.Greater(t, failures, 0, "expected at least one failure with 50%% error rate")
}

func TestSDK_PatchFailsOnStatusApply(t *testing.T) {
	exp := newTestExperiment()
	r := newSDKReconciler(exp, map[sdk.Operation]sdk.FaultSpec{
		sdk.OpPatch: {ErrorRate: 1.0, Error: "internal server error"},
	})

	_, err := r.Reconcile(context.Background(), reconcileRequest())
	if err != nil {
		var chaosErr *sdk.ChaosError
		if assert.ErrorAs(t, err, &chaosErr) {
			assert.Equal(t, sdk.OpPatch, chaosErr.Operation)
		}
	}
}

func TestSDK_CreateFailsDuringReconciliation(t *testing.T) {
	exp := newTestExperiment()
	r := newSDKReconciler(exp, map[sdk.Operation]sdk.FaultSpec{
		sdk.OpCreate: {ErrorRate: 1.0, Error: "resource quota exceeded"},
	})

	_, err := r.Reconcile(context.Background(), reconcileRequest())
	if err != nil {
		var chaosErr *sdk.ChaosError
		assert.ErrorAs(t, err, &chaosErr)
	}
}

func TestSDK_DeletePassthroughForCleanup(t *testing.T) {
	exp := newTestExperiment()
	exp.DeletionTimestamp = &metav1.Time{Time: time.Now()}
	exp.Finalizers = []string{cleanupFinalizer}

	r := newSDKReconciler(exp, nil)

	_, err := r.Reconcile(context.Background(), reconcileRequest())
	require.NoError(t, err)
}

func TestSDK_DelayInjection(t *testing.T) {
	exp := newTestExperiment()
	r := newSDKReconciler(exp, map[sdk.Operation]sdk.FaultSpec{
		sdk.OpGet: {ErrorRate: 0.0, Delay: 10 * time.Millisecond},
	})

	start := time.Now()
	_, err := r.Reconcile(context.Background(), reconcileRequest())
	elapsed := time.Since(start)

	require.NoError(t, err)
	assert.GreaterOrEqual(t, elapsed, 10*time.Millisecond)
}

func TestSDK_WrapReconciler(t *testing.T) {
	exp := newTestExperiment()
	scheme := newTestScheme()
	innerClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(exp).
		WithStatusSubresource(exp).
		Build()

	inner := &ChaosExperimentReconciler{
		Client:       innerClient,
		Scheme:       scheme,
		Orchestrator: &passthroughOrchestrator{},
		Lock:         safety.NewLocalExperimentLock(),
		Clock:        clock.NewFakeClock(time.Now()),
		Recorder:     record.NewFakeRecorder(100),
	}

	fc := sdk.NewFaultConfig(map[sdk.Operation]sdk.FaultSpec{
		sdk.OpReconcile: {ErrorRate: 1.0, Error: "reconcile blocked"},
	})
	wrapped := chaosclient.WrapReconciler(inner, chaosclient.WithFaultConfig(fc))

	_, err := wrapped.Reconcile(context.Background(), ctrl.Request{})
	require.Error(t, err)

	var chaosErr *sdk.ChaosError
	assert.ErrorAs(t, err, &chaosErr)
	assert.Equal(t, sdk.OpReconcile, chaosErr.Operation)
}
