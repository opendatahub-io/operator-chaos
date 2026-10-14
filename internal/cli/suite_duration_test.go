package cli

import (
	"context"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/opendatahub-io/operator-chaos/api/v1alpha1"
	"github.com/opendatahub-io/operator-chaos/pkg/evaluator"
	"github.com/opendatahub-io/operator-chaos/pkg/injection"
	"github.com/opendatahub-io/operator-chaos/pkg/observer"
	"github.com/opendatahub-io/operator-chaos/pkg/orchestrator"
	"github.com/opendatahub-io/operator-chaos/pkg/safety"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

const durationOverflowExperiment = `apiVersion: chaos.operatorchaos.io/v1alpha1
kind: ChaosExperiment
metadata:
  name: duration-overflow
spec:
  tier: 3
  target:
    operator: test-operator
    component: test-component
  injection:
    type: PodKill
    parameters:
      labelSelector: app=test
    ttl: "2m"
  hypothesis:
    description: test
    recoveryTimeout: 3m
  blastRadius:
    maxPodsAffected: 1
`

// TestRunSingleExperiment_FailsWhenDurationExceedsTimeout verifies the suite
// path rejects an experiment whose TTL plus recovery window cannot fit the
// per-experiment timeout, instead of aborting mid-run. deps is nil: the
// rejection must happen before deps.Orchestrator is ever reached.
func TestRunSingleExperiment_FailsWhenDurationExceedsTimeout(t *testing.T) {
	file := filepath.Join(t.TempDir(), "exp.yaml")
	require.NoError(t, os.WriteFile(file, []byte(durationOverflowExperiment), 0o644))

	r := runSingleExperiment(context.Background(), file, nil, false, 4*time.Minute, "", 0, nil)

	assert.Equal(t, "fail", r.status, "a timeout misconfiguration must fail the suite, not skip green")
	require.Error(t, r.err)
	assert.Contains(t, r.err.Error(), "exceeds --timeout")
	assert.Contains(t, r.verdict, "duration-overflow")
}

// TestRunSingleExperiment_DryRunSkipsDurationCheck verifies a dry run does
// not fail on duration: it never injects a fault or waits through recovery,
// so the worst-case budget does not apply.
func TestRunSingleExperiment_DryRunSkipsDurationCheck(t *testing.T) {
	file := filepath.Join(t.TempDir(), "exp.yaml")
	require.NoError(t, os.WriteFile(file, []byte(durationOverflowExperiment), 0o644))

	r := runSingleExperiment(context.Background(), file, nil, true, 4*time.Minute, "", 0, nil)

	assert.Equal(t, "pass", r.status)
	assert.Nil(t, r.err)
	assert.Contains(t, r.verdict, "VALID")
}

// TestRunSingleExperiment_ManifestDryRunSkipsDurationCheck verifies a
// manifest-level blastRadius.dryRun experiment also skips the duration
// check: like a CLI dry run, it never injects a fault or waits through
// recovery, so the worst-case budget does not apply.
func TestRunSingleExperiment_ManifestDryRunSkipsDurationCheck(t *testing.T) {
	manifest := strings.Replace(durationOverflowExperiment, "blastRadius:", "blastRadius:\n    dryRun: true\n    allowedNamespaces: [test-ns]", 1)
	file := filepath.Join(t.TempDir(), "exp.yaml")
	require.NoError(t, os.WriteFile(file, []byte(manifest), 0o644))

	// A real orchestrator: the manifest-level dry run proceeds to execution,
	// where Run returns Inconclusive without injecting.
	fakeClient := fake.NewClientBuilder().WithScheme(runtime.NewScheme()).Build()
	registry := injection.NewRegistry()
	registry.Register(v1alpha1.PodKill, injection.NewPodKillInjector(fakeClient))
	deps := &orchestratorDeps{Orchestrator: orchestrator.New(orchestrator.OrchestratorConfig{
		Registry:  registry,
		Observer:  observer.NewKubernetesObserver(fakeClient),
		Evaluator: evaluator.New(10),
		Lock:      safety.NewLocalExperimentLock(),
		Logger:    slog.New(slog.NewTextHandler(io.Discard, nil)),
	})}

	r := runSingleExperiment(context.Background(), file, deps, false, 4*time.Minute, "", 0, nil)

	assert.NotEqual(t, "fail", r.status, "a manifest-level dry run must not fail the duration check")
	assert.Nil(t, r.err)
}
