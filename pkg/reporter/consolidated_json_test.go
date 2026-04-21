package reporter

import (
	"bytes"
	"encoding/json"
	"testing"
	"time"

	v1alpha1 "github.com/opendatahub-io/operator-chaos/api/v1alpha1"
	"github.com/opendatahub-io/operator-chaos/pkg/evaluator"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConsolidatedJSONReporterWriteReport(t *testing.T) {
	reports := []ExperimentReport{
		{Experiment: "test-a", Evaluation: evaluator.EvaluationResult{Verdict: v1alpha1.Resilient, RecoveryTime: 2 * time.Second}},
		{Experiment: "test-b", Evaluation: evaluator.EvaluationResult{Verdict: v1alpha1.Failed}},
	}

	var buf bytes.Buffer
	r := &ConsolidatedJSONReporter{}
	err := r.WriteReport(&buf, reports)
	require.NoError(t, err)

	var result ConsolidatedReport
	require.NoError(t, json.Unmarshal(buf.Bytes(), &result))

	assert.Equal(t, 2, len(result.Experiments))
	assert.Equal(t, 2, result.Summary.Total)
	assert.Equal(t, 1, result.Summary.Resilient)
	assert.Equal(t, 1, result.Summary.Failed)
	assert.InDelta(t, 0.50, result.Summary.PassRate, 0.001)
	assert.Equal(t, "test-a", result.Experiments[0].Experiment)
}

func TestConsolidatedJSONReporterEmpty(t *testing.T) {
	var buf bytes.Buffer
	r := &ConsolidatedJSONReporter{}
	require.NoError(t, r.WriteReport(&buf, nil))

	var result ConsolidatedReport
	require.NoError(t, json.Unmarshal(buf.Bytes(), &result))
	assert.Equal(t, 0, result.Summary.Total)
	assert.Empty(t, result.Experiments)
}
