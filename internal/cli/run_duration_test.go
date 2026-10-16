package cli

import (
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
	assert.Contains(t, err.Error(), "8m0s")
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
