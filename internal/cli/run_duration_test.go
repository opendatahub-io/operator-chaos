package cli

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/opendatahub-io/operator-chaos/api/v1alpha1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func newDurationTestExperiment(ttl, recovery time.Duration) *v1alpha1.ChaosExperiment {
	exp := &v1alpha1.ChaosExperiment{}
	exp.Spec.Injection.TTL = metav1.Duration{Duration: ttl}
	exp.Spec.Hypothesis.RecoveryTimeout = metav1.Duration{Duration: recovery}
	return exp
}

// TestValidateRunDuration_AcceptsFittingDuration verifies a TTL-bearing
// experiment whose TTL plus recovery window fits the run timeout is accepted.
func TestValidateRunDuration_AcceptsFittingDuration(t *testing.T) {
	exp := newDurationTestExperiment(2*time.Minute, 3*time.Minute)
	assert.NoError(t, validateRunDuration(exp, 10*time.Minute))
}

// TestValidateRunDuration_RejectsExceedingDuration verifies a TTL-bearing
// experiment that cannot finish inside the run timeout is rejected upfront
// instead of aborting mid-recovery-window and losing the verdict.
func TestValidateRunDuration_RejectsExceedingDuration(t *testing.T) {
	exp := newDurationTestExperiment(2*time.Minute, 3*time.Minute)
	err := validateRunDuration(exp, 4*time.Minute)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "8m45s")
	assert.Contains(t, err.Error(), "--timeout")
}

// TestValidateRunDuration_NoTTLUsesRecoveryTimeout verifies TTL-less
// experiments are checked against their recovery timeout alone.
func TestValidateRunDuration_NoTTLUsesRecoveryTimeout(t *testing.T) {
	exp := newDurationTestExperiment(0, 3*time.Minute)
	assert.NoError(t, validateRunDuration(exp, 10*time.Minute))

	exp = newDurationTestExperiment(0, 15*time.Minute)
	err := validateRunDuration(exp, 10*time.Minute)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "15m0s")
}

// TestValidateRunDuration_ExactFitRejected pins the boundary: a run duration
// exactly equal to the timeout is rejected, because setup (validation, lock,
// pre-check, injection) and the post-check all run inside the same timeout
// context — an exact fit cannot finish.
func TestValidateRunDuration_ExactFitRejected(t *testing.T) {
	exp := newDurationTestExperiment(2*time.Minute, 3*time.Minute)
	err := validateRunDuration(exp, 5*time.Minute)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "exceeds --timeout")
}

// TestValidateRunDuration_ReservesPostCheckBudget verifies the guard reserves
// time for the post-check's reconciliation poll, which gets up to another
// full recoveryTimeout on the same context after the recovery window: a run
// whose TTL plus one recoveryTimeout fits the timeout but whose worst-case
// total (TTL + 2x recoveryTimeout) does not must be rejected, or a slow
// operator's Degraded verdict is lost to a context abort mid-poll.
func TestValidateRunDuration_ReservesPostCheckBudget(t *testing.T) {
	exp := newDurationTestExperiment(2*time.Minute, 3*time.Minute)
	err := validateRunDuration(exp, 7*time.Minute)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "exceeds --timeout")
}

// TestRunTierFilterPrecedesDurationCheck verifies an experiment filtered out
// by --max-tier is skipped even when its duration exceeds the timeout: the
// tier filter must not turn into a hard error for experiments the user never
// asked to run.
func TestRunTierFilterPrecedesDurationCheck(t *testing.T) {
	file := filepath.Join(t.TempDir(), "exp.yaml")
	require.NoError(t, os.WriteFile(file, []byte(durationOverflowExperiment), 0o644))

	r := runSingleExperiment(context.Background(), file, nil, false, 4*time.Minute, "", 1, nil)

	assert.Equal(t, "skip", r.status)
	assert.Nil(t, r.err, "a tier-filtered experiment must not fail the duration check")
}

// TestValidateRunDuration_ReservesRevertRetryAllowance verifies the budget
// reserves slack for transient TTL-revert retries: each failed cleanup
// attempt costs up to the cleanup timeout plus the retry interval, so a run
// whose nominal worst case exactly fits minus that allowance must still be
// rejected — otherwise a single transient revert failure aborts the run
// before recovery instead of finishing with a verdict.
func TestValidateRunDuration_ReservesRevertRetryAllowance(t *testing.T) {
	exp := newDurationTestExperiment(2*time.Minute, 3*time.Minute)
	// Nominal worst case 8m; timeout 8m30s leaves 30s — less than one
	// transient revert retry costs — so the guard must reject.
	err := validateRunDuration(exp, 8*time.Minute+30*time.Second)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "exceeds --timeout")

	// Enough slack for a transient retry passes.
	assert.NoError(t, validateRunDuration(exp, 9*time.Minute+30*time.Second))
}

// TestValidateRunDuration_NoRetryAllowanceWithoutTTL verifies the transient
// revert-retry allowance applies only to TTL-bearing faults: without a TTL
// there is no mid-run revert and hence no retry, so a timeout that covers
// the two recovery windows must be accepted (recoveryTimeout 3m, timeout
// 6m30s — 30s beyond the nominal 6m worst case).
func TestValidateRunDuration_NoRetryAllowanceWithoutTTL(t *testing.T) {
	exp := newDurationTestExperiment(0, 3*time.Minute)
	assert.NoError(t, validateRunDuration(exp, 6*time.Minute+30*time.Second))
}
