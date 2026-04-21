package reporter

import (
	"bytes"
	"testing"
	"time"

	v1alpha1 "github.com/opendatahub-io/operator-chaos/api/v1alpha1"
	"github.com/opendatahub-io/operator-chaos/pkg/evaluator"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMarkdownReporterWriteReport(t *testing.T) {
	reports := []ExperimentReport{
		{
			Experiment: "dashboard-pod-kill",
			Timestamp:  time.Date(2026, 4, 21, 10, 0, 0, 0, time.UTC),
			Target: TargetReport{
				Operator:  "opendatahub-operator",
				Component: "dashboard",
				Resource:  "Deployment/odh-dashboard",
			},
			Injection: InjectionReport{
				Type:      string(v1alpha1.PodKill),
				Timestamp: time.Date(2026, 4, 21, 10, 0, 5, 0, time.UTC),
				Details: map[string]string{
					"namespace": "opendatahub",
					"count":     "1",
				},
			},
			Evaluation: evaluator.EvaluationResult{
				Verdict:         v1alpha1.Resilient,
				Confidence:      "3/3 checks passed",
				RecoveryTime:    1200 * time.Millisecond,
				ReconcileCycles: 1,
			},
			SteadyState: SteadyStateReport{
				Pre:  true,
				Post: true,
			},
			Collateral: []CollateralFinding{
				{
					Operator:  "model-controller",
					Component: "inferenceservice",
					Passed:    true,
				},
			},
		},
		{
			Experiment: "model-controller-webhook-disrupt",
			Timestamp:  time.Date(2026, 4, 21, 10, 5, 0, 0, time.UTC),
			Target: TargetReport{
				Operator:  "model-controller",
				Component: "webhook",
			},
			Injection: InjectionReport{
				Type:      "WebhookDisrupt",
				Timestamp: time.Date(2026, 4, 21, 10, 5, 5, 0, time.UTC),
			},
			Evaluation: evaluator.EvaluationResult{
				Verdict:         v1alpha1.Failed,
				Confidence:      "0/3 checks passed",
				RecoveryTime:    30 * time.Second,
				ReconcileCycles: 5,
				Deviations: []evaluator.Deviation{
					{Type: "readiness_probe_failure", Detail: "pod not ready after 30s"},
					{Type: "collateral_degradation", Detail: "inferenceservice affected"},
				},
			},
			CleanupError: "failed to restore webhook config",
		},
	}

	var buf bytes.Buffer
	r := &MarkdownReporter{}
	err := r.WriteReport(&buf, reports)
	require.NoError(t, err)

	output := buf.String()

	// Title
	assert.Contains(t, output, "# Chaos Experiment Report")

	// Header stats
	assert.Contains(t, output, "**Experiments:** 2")
	assert.Contains(t, output, "**Pass Rate:** 50.0%")

	// Summary verdict table
	assert.Contains(t, output, "| Verdict | Count |")
	assert.Contains(t, output, "| Resilient | 1 |")
	assert.Contains(t, output, "| Failed | 1 |")
	// Degraded and Inconclusive are zero, should not appear
	assert.NotContains(t, output, "| Degraded |")
	assert.NotContains(t, output, "| Inconclusive |")

	// Results section
	assert.Contains(t, output, "## Results")

	// First experiment
	assert.Contains(t, output, "### dashboard-pod-kill")
	assert.Contains(t, output, "| Component | dashboard |")
	assert.Contains(t, output, "| Injection | PodKill |")
	assert.Contains(t, output, "| Verdict | Resilient |")
	assert.Contains(t, output, "| Recovery | 1.2s (1 cycle) |")

	// Details section for first experiment
	assert.Contains(t, output, "<details>")
	assert.Contains(t, output, "<summary>Details</summary>")
	assert.Contains(t, output, "count=1, namespace=opendatahub")
	assert.Contains(t, output, "Pre: PASS")
	assert.Contains(t, output, "Post: PASS")
	assert.Contains(t, output, "**Deviations:** None")
	assert.Contains(t, output, "model-controller/inferenceservice: PASS")

	// Second experiment
	assert.Contains(t, output, "### model-controller-webhook-disrupt")
	assert.Contains(t, output, "| Verdict | Failed |")
	assert.Contains(t, output, "| Recovery | 30.0s (5 cycles) |")

	// Deviations for second experiment
	assert.Contains(t, output, "readiness_probe_failure")
	assert.Contains(t, output, "pod not ready after 30s")
	assert.Contains(t, output, "collateral_degradation")

	// Cleanup error
	assert.Contains(t, output, "**Cleanup Error:** failed to restore webhook config")
}

func TestMarkdownReporterEmpty(t *testing.T) {
	var buf bytes.Buffer
	r := &MarkdownReporter{}
	err := r.WriteReport(&buf, nil)
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "**Experiments:** 0")
	assert.Contains(t, output, "**Pass Rate:** 0.0%")
}

func TestFormatRecovery(t *testing.T) {
	tests := []struct {
		name     string
		dur      time.Duration
		cycles   int
		expected string
	}{
		{"both zero", 0, 0, "N/A"},
		{"duration only", 1200 * time.Millisecond, 0, "1.2s"},
		{"single cycle", 1200 * time.Millisecond, 1, "1.2s (1 cycle)"},
		{"multiple cycles", 30 * time.Second, 5, "30.0s (5 cycles)"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, formatRecovery(tt.dur, tt.cycles))
		})
	}
}

func TestFormatCheckResult(t *testing.T) {
	assert.Equal(t, "PASS", formatCheckResult(true))
	assert.Equal(t, "FAIL", formatCheckResult(false))
	assert.Equal(t, "N/A", formatCheckResult(nil))
	assert.Equal(t, "N/A", formatCheckResult("unexpected"))
}

func TestFormatMapSorted(t *testing.T) {
	m := map[string]string{"z": "3", "a": "1", "m": "2"}
	assert.Equal(t, "a=1, m=2, z=3", formatMapSorted(m))
	assert.Equal(t, "", formatMapSorted(nil))
	assert.Equal(t, "", formatMapSorted(map[string]string{}))
}
