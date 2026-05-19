package controller

import (
	"context"
	"fmt"
	"testing"
	"time"

	v1alpha1 "github.com/opendatahub-io/operator-chaos/api/v1alpha1"
	"github.com/opendatahub-io/operator-chaos/pkg/clock"
	"github.com/opendatahub-io/operator-chaos/pkg/safety"
	"github.com/opendatahub-io/operator-chaos/pkg/sdk"
	"github.com/opendatahub-io/operator-chaos/pkg/sdk/fuzz"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

func chaosExperimentFactory(c client.Client) reconcile.Reconciler {
	return &ChaosExperimentReconciler{
		Client:       c,
		Scheme:       newTestScheme(),
		Orchestrator: &passthroughOrchestrator{},
		Lock:         safety.NewLocalExperimentLock(),
		Clock:        clock.NewFakeClock(time.Now()),
		Recorder:     record.NewFakeRecorder(100),
	}
}

func chaosExperimentPhaseValid(key types.NamespacedName) fuzz.Invariant {
	return func(ctx context.Context, c client.Client) error {
		exp := &v1alpha1.ChaosExperiment{}
		if err := c.Get(ctx, key, exp); err != nil {
			return nil
		}
		validPhases := map[v1alpha1.ExperimentPhase]bool{
			"":                           true,
			v1alpha1.PhasePending:        true,
			v1alpha1.PhaseSteadyStatePre: true,
			v1alpha1.PhaseInjecting:      true,
			v1alpha1.PhaseObserving:      true,
			v1alpha1.PhaseSteadyStatePost: true,
			v1alpha1.PhaseEvaluating:     true,
			v1alpha1.PhaseComplete:       true,
			v1alpha1.PhaseAborted:        true,
		}
		if !validPhases[exp.Status.Phase] {
			return fmt.Errorf("invalid phase %q on experiment %s", exp.Status.Phase, key)
		}
		return nil
	}
}

func FuzzChaosExperimentReconciler(f *testing.F) {
	f.Add(uint16(0x01FF), uint8(0), uint16(32768))
	f.Add(uint16(0), uint8(3), uint16(65535))
	f.Add(uint16(1), uint8(1), uint16(0))
	f.Add(uint16(0xFFFF), uint8(255), uint16(1))
	f.Add(uint16(0x0001), uint8(0), uint16(65535))
	f.Add(uint16(0x0008), uint8(2), uint16(16384))

	scheme := newTestScheme()
	key := types.NamespacedName{Name: "test-exp", Namespace: "opendatahub"}

	f.Fuzz(func(t *testing.T, opMask uint16, faultType uint8, intensity uint16) {
		exp := newTestExperiment()
		req := reconcileRequest()

		h := fuzz.NewHarness(chaosExperimentFactory, scheme, req, exp)
		h.AddInvariant(fuzz.ObjectExists(key, &v1alpha1.ChaosExperiment{}))
		h.AddInvariant(chaosExperimentPhaseValid(key))
		h.SetTimeout(5 * time.Second)

		fc := fuzz.DecodeFaultConfig(opMask, faultType, intensity)
		_ = h.Run(t, fc)
	})
}

func FuzzChaosExperimentWithSpecificFaults(f *testing.F) {
	f.Add(true, true, true, true, uint16(32768))
	f.Add(true, false, false, false, uint16(65535))
	f.Add(false, true, false, false, uint16(16384))
	f.Add(false, false, true, false, uint16(49152))

	scheme := newTestScheme()
	key := types.NamespacedName{Name: "test-exp", Namespace: "opendatahub"}

	f.Fuzz(func(t *testing.T, faultGet, faultUpdate, faultPatch, faultCreate bool, intensity uint16) {
		exp := newTestExperiment()
		req := reconcileRequest()

		faults := make(map[sdk.Operation]sdk.FaultSpec)
		rate := float64(intensity) / 65535.0

		if faultGet {
			faults[sdk.OpGet] = sdk.FaultSpec{ErrorRate: rate, Error: "connection refused"}
		}
		if faultUpdate {
			faults[sdk.OpUpdate] = sdk.FaultSpec{ErrorRate: rate, Error: "the object has been modified"}
		}
		if faultPatch {
			faults[sdk.OpPatch] = sdk.FaultSpec{ErrorRate: rate, Error: "internal server error"}
		}
		if faultCreate {
			faults[sdk.OpCreate] = sdk.FaultSpec{ErrorRate: rate, Error: "resource quota exceeded"}
		}

		h := fuzz.NewHarness(chaosExperimentFactory, scheme, req, exp)
		h.AddInvariant(fuzz.ObjectExists(key, &v1alpha1.ChaosExperiment{}))
		h.AddInvariant(chaosExperimentPhaseValid(key))

		fc := sdk.NewFaultConfig(faults)
		_ = h.Run(t, fc)
	})
}
