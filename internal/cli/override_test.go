package cli

import (
	"testing"

	v1alpha1 "github.com/opendatahub-io/operator-chaos/api/v1alpha1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func newTestExperiment() *v1alpha1.ChaosExperiment {
	return &v1alpha1.ChaosExperiment{
		ObjectMeta: metav1.ObjectMeta{Name: "test-experiment"},
		Spec: v1alpha1.ChaosExperimentSpec{
			Target: v1alpha1.TargetSpec{
				Operator:  "model-registry",
				Component: "model-registry-operator",
				Resource:  "Route/model-registry-https",
			},
			Injection: v1alpha1.InjectionSpec{
				Type: v1alpha1.WebhookDisrupt,
				Parameters: map[string]string{
					"name":      "old-webhook",
					"namespace": "default",
				},
			},
			BlastRadius: v1alpha1.BlastRadiusSpec{
				MaxPodsAffected:   1,
				AllowedNamespaces: []string{"ns-a", "ns-b"},
			},
			Hypothesis: v1alpha1.HypothesisSpec{
				Description: "operator should recover",
			},
		},
	}
}

func TestApplyOverrides_ParameterName(t *testing.T) {
	exp := newTestExperiment()
	err := applyOverrides(exp, []string{"injection.parameters.name=new-webhook"})
	require.NoError(t, err)
	assert.Equal(t, "new-webhook", exp.Spec.Injection.Parameters["name"])
}

func TestApplyOverrides_TargetResource(t *testing.T) {
	exp := newTestExperiment()
	err := applyOverrides(exp, []string{"target.resource=Route/model-catalog-https"})
	require.NoError(t, err)
	assert.Equal(t, "Route/model-catalog-https", exp.Spec.Target.Resource)
}

func TestApplyOverrides_MultipleOverrides(t *testing.T) {
	exp := newTestExperiment()
	err := applyOverrides(exp, []string{
		"injection.parameters.name=new-webhook",
		"target.resource=Route/model-catalog-https",
		"target.component=catalog-operator",
	})
	require.NoError(t, err)
	assert.Equal(t, "new-webhook", exp.Spec.Injection.Parameters["name"])
	assert.Equal(t, "Route/model-catalog-https", exp.Spec.Target.Resource)
	assert.Equal(t, "catalog-operator", exp.Spec.Target.Component)
}

func TestApplyOverrides_ArrayIndex(t *testing.T) {
	exp := newTestExperiment()
	err := applyOverrides(exp, []string{"blastRadius.allowedNamespaces[0]=replaced-ns"})
	require.NoError(t, err)
	assert.Equal(t, "replaced-ns", exp.Spec.BlastRadius.AllowedNamespaces[0])
	assert.Equal(t, "ns-b", exp.Spec.BlastRadius.AllowedNamespaces[1])
}

func TestApplyOverrides_InvalidFormat_MissingEquals(t *testing.T) {
	exp := newTestExperiment()
	err := applyOverrides(exp, []string{"injection.parameters.name"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "expected key=value format")
}

func TestApplyOverrides_InvalidFormat_EmptyKey(t *testing.T) {
	exp := newTestExperiment()
	err := applyOverrides(exp, []string{"=value"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "expected key=value format")
}

func TestApplyOverrides_EmptySlice(t *testing.T) {
	exp := newTestExperiment()
	original := exp.Spec.Target.Resource
	err := applyOverrides(exp, nil)
	require.NoError(t, err)
	assert.Equal(t, original, exp.Spec.Target.Resource)
}

func TestApplyOverrides_ValueWithEquals(t *testing.T) {
	exp := newTestExperiment()
	err := applyOverrides(exp, []string{"injection.parameters.name=foo=bar"})
	require.NoError(t, err)
	assert.Equal(t, "foo=bar", exp.Spec.Injection.Parameters["name"])
}

func TestApplyOverrides_NonexistentPath(t *testing.T) {
	exp := newTestExperiment()
	err := applyOverrides(exp, []string{"nonexistent.field.path=value"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestApplyOverrides_HypothesisDescription(t *testing.T) {
	exp := newTestExperiment()
	err := applyOverrides(exp, []string{"hypothesis.description=new hypothesis"})
	require.NoError(t, err)
	assert.Equal(t, "new hypothesis", exp.Spec.Hypothesis.Description)
}

func TestParseDotPath(t *testing.T) {
	tests := []struct {
		input    string
		expected []pathSegment
	}{
		{
			input: "target.resource",
			expected: []pathSegment{
				{key: "target", index: -1},
				{key: "resource", index: -1},
			},
		},
		{
			input: "blastRadius.allowedNamespaces[0]",
			expected: []pathSegment{
				{key: "blastRadius", index: -1},
				{key: "allowedNamespaces", index: 0},
			},
		},
		{
			input: "injection.parameters.name",
			expected: []pathSegment{
				{key: "injection", index: -1},
				{key: "parameters", index: -1},
				{key: "name", index: -1},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := parseDotPath(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}
