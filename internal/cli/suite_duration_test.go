package cli

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const durationOverflowExperiment = `apiVersion: chaos.operatorchaos.io/v1alpha1
kind: ChaosExperiment
metadata:
  name: duration-overflow
spec:
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

// TestRunSingleExperiment_SkipsWhenDurationExceedsTimeout verifies the suite
// path rejects an experiment whose TTL plus recovery window cannot fit the
// per-experiment timeout, instead of aborting mid-run. deps is nil: the skip
// must happen before deps.Orchestrator is ever reached.
func TestRunSingleExperiment_SkipsWhenDurationExceedsTimeout(t *testing.T) {
	file := filepath.Join(t.TempDir(), "exp.yaml")
	require.NoError(t, os.WriteFile(file, []byte(durationOverflowExperiment), 0o644))

	r := runSingleExperiment(context.Background(), file, nil, false, 4*time.Minute, "", 0, nil)

	assert.Equal(t, "skip", r.status)
	require.Error(t, r.err)
	assert.Contains(t, r.err.Error(), "exceeds --timeout")
	assert.Contains(t, r.verdict, "duration-overflow")
}
