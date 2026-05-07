package injection

import (
	"context"
	"testing"

	v1alpha1 "github.com/opendatahub-io/operator-chaos/api/v1alpha1"
	"github.com/opendatahub-io/operator-chaos/pkg/safety"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	policyv1 "k8s.io/api/policy/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func newPDBTestScheme(t *testing.T) *runtime.Scheme {
	t.Helper()
	scheme := runtime.NewScheme()
	require.NoError(t, corev1.AddToScheme(scheme))
	require.NoError(t, policyv1.AddToScheme(scheme))
	return scheme
}

func TestPDBBlockValidateRejectsMissingLabelSelector(t *testing.T) {
	injector := &PDBBlockInjector{}

	spec := v1alpha1.InjectionSpec{
		Type:        v1alpha1.PDBBlock,
		DangerLevel: v1alpha1.DangerLevelHigh,
		Parameters:  map[string]string{},
	}
	blast := v1alpha1.BlastRadiusSpec{MaxPodsAffected: 1}

	err := injector.Validate(spec, blast)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "labelSelector")
}

func TestPDBBlockValidateRejectsMissingDangerLevel(t *testing.T) {
	injector := &PDBBlockInjector{}

	spec := v1alpha1.InjectionSpec{
		Type: v1alpha1.PDBBlock,
		Parameters: map[string]string{
			"labelSelector": "app=my-app",
		},
	}
	blast := v1alpha1.BlastRadiusSpec{MaxPodsAffected: 1}

	err := injector.Validate(spec, blast)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "dangerLevel: high")
}

func TestPDBBlockValidateAcceptsValid(t *testing.T) {
	injector := &PDBBlockInjector{}

	spec := v1alpha1.InjectionSpec{
		Type:        v1alpha1.PDBBlock,
		DangerLevel: v1alpha1.DangerLevelHigh,
		Parameters: map[string]string{
			"labelSelector": "app=my-app",
		},
	}
	blast := v1alpha1.BlastRadiusSpec{MaxPodsAffected: 1}

	err := injector.Validate(spec, blast)
	assert.NoError(t, err)
}

func TestPDBBlockValidateRejectsInvalidLabelSelector(t *testing.T) {
	injector := &PDBBlockInjector{}

	spec := v1alpha1.InjectionSpec{
		Type:        v1alpha1.PDBBlock,
		DangerLevel: v1alpha1.DangerLevelHigh,
		Parameters: map[string]string{
			"labelSelector": "invalid-no-equals",
		},
	}
	blast := v1alpha1.BlastRadiusSpec{MaxPodsAffected: 1}

	err := injector.Validate(spec, blast)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "key=value")
}

func TestPDBBlockInjectCreatesPDB(t *testing.T) {
	scheme := newPDBTestScheme(t)

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		Build()

	injector := NewPDBBlockInjector(fakeClient)

	spec := v1alpha1.InjectionSpec{
		Type:        v1alpha1.PDBBlock,
		DangerLevel: v1alpha1.DangerLevelHigh,
		Parameters: map[string]string{
			"labelSelector": "app=my-app",
		},
	}

	ctx := context.Background()
	cleanup, events, err := injector.Inject(ctx, spec, "test-ns")
	require.NoError(t, err)
	assert.Len(t, events, 1)
	assert.Equal(t, v1alpha1.PDBBlock, events[0].Type)
	assert.Equal(t, "created", events[0].Action)
	assert.NotNil(t, cleanup)

	// Verify PDB exists with correct spec
	var pdb policyv1.PodDisruptionBudget
	err = fakeClient.Get(ctx, types.NamespacedName{Name: "chaos-pdb-my-app", Namespace: "test-ns"}, &pdb)
	require.NoError(t, err)
	assert.Equal(t, intstr.FromInt32(0), *pdb.Spec.MaxUnavailable)
	assert.Equal(t, map[string]string{"app": "my-app"}, pdb.Spec.Selector.MatchLabels)
	assert.Equal(t, safety.ManagedByValue, pdb.Labels[safety.ManagedByLabel])
}

func TestPDBBlockCleanupDeletesPDB(t *testing.T) {
	scheme := newPDBTestScheme(t)

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		Build()

	injector := NewPDBBlockInjector(fakeClient)

	spec := v1alpha1.InjectionSpec{
		Type:        v1alpha1.PDBBlock,
		DangerLevel: v1alpha1.DangerLevelHigh,
		Parameters: map[string]string{
			"labelSelector": "app=my-app",
		},
	}

	ctx := context.Background()
	cleanup, _, err := injector.Inject(ctx, spec, "test-ns")
	require.NoError(t, err)

	// Cleanup should delete the PDB
	err = cleanup(ctx)
	require.NoError(t, err)

	// Verify PDB is gone
	var pdb policyv1.PodDisruptionBudget
	err = fakeClient.Get(ctx, types.NamespacedName{Name: "chaos-pdb-my-app", Namespace: "test-ns"}, &pdb)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestPDBBlockRevertDeletesPDB(t *testing.T) {
	scheme := newPDBTestScheme(t)

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		Build()

	injector := NewPDBBlockInjector(fakeClient)

	spec := v1alpha1.InjectionSpec{
		Type:        v1alpha1.PDBBlock,
		DangerLevel: v1alpha1.DangerLevelHigh,
		Parameters: map[string]string{
			"labelSelector": "app=my-app",
		},
	}

	ctx := context.Background()
	_, _, err := injector.Inject(ctx, spec, "test-ns")
	require.NoError(t, err)

	// Revert should delete the PDB
	err = injector.Revert(ctx, spec, "test-ns")
	require.NoError(t, err)

	// Verify PDB is gone
	var pdb policyv1.PodDisruptionBudget
	err = fakeClient.Get(ctx, types.NamespacedName{Name: "chaos-pdb-my-app", Namespace: "test-ns"}, &pdb)
	require.Error(t, err)
}

func TestPDBBlockHandlesNonChaosManagedCollision(t *testing.T) {
	scheme := newPDBTestScheme(t)

	existingPDB := &policyv1.PodDisruptionBudget{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "chaos-pdb-my-app",
			Namespace: "test-ns",
			Labels: map[string]string{
				"app.kubernetes.io/managed-by": "someone-else",
			},
		},
		Spec: policyv1.PodDisruptionBudgetSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": "my-app"},
			},
		},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(existingPDB).
		Build()

	injector := NewPDBBlockInjector(fakeClient)

	spec := v1alpha1.InjectionSpec{
		Type:        v1alpha1.PDBBlock,
		DangerLevel: v1alpha1.DangerLevelHigh,
		Parameters: map[string]string{
			"labelSelector": "app=my-app",
		},
	}

	ctx := context.Background()
	_, _, err := injector.Inject(ctx, spec, "test-ns")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not chaos-managed")
}

func TestPDBBlockUpdatesExistingChaosManaged(t *testing.T) {
	scheme := newPDBTestScheme(t)

	chaosLabels := safety.ChaosLabels(string(v1alpha1.PDBBlock))
	oldMaxUnavailable := intstr.FromInt32(1)
	existingPDB := &policyv1.PodDisruptionBudget{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "chaos-pdb-my-app",
			Namespace: "test-ns",
			Labels:    chaosLabels,
		},
		Spec: policyv1.PodDisruptionBudgetSpec{
			MaxUnavailable: &oldMaxUnavailable,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": "old-app"},
			},
		},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(existingPDB).
		Build()

	injector := NewPDBBlockInjector(fakeClient)

	spec := v1alpha1.InjectionSpec{
		Type:        v1alpha1.PDBBlock,
		DangerLevel: v1alpha1.DangerLevelHigh,
		Parameters: map[string]string{
			"labelSelector": "app=my-app",
		},
	}

	ctx := context.Background()
	_, events, err := injector.Inject(ctx, spec, "test-ns")
	require.NoError(t, err)
	assert.Len(t, events, 1)

	// Verify PDB was updated
	var pdb policyv1.PodDisruptionBudget
	err = fakeClient.Get(ctx, types.NamespacedName{Name: "chaos-pdb-my-app", Namespace: "test-ns"}, &pdb)
	require.NoError(t, err)
	assert.Equal(t, intstr.FromInt32(0), *pdb.Spec.MaxUnavailable)
	assert.Equal(t, map[string]string{"app": "my-app"}, pdb.Spec.Selector.MatchLabels)
}

func TestPDBBlockAutoGeneratedName(t *testing.T) {
	scheme := newPDBTestScheme(t)

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		Build()

	injector := NewPDBBlockInjector(fakeClient)

	spec := v1alpha1.InjectionSpec{
		Type:        v1alpha1.PDBBlock,
		DangerLevel: v1alpha1.DangerLevelHigh,
		Parameters: map[string]string{
			"labelSelector": "component=worker",
		},
	}

	ctx := context.Background()
	_, _, err := injector.Inject(ctx, spec, "test-ns")
	require.NoError(t, err)

	// Verify PDB was created with auto-generated name
	var pdb policyv1.PodDisruptionBudget
	err = fakeClient.Get(ctx, types.NamespacedName{Name: "chaos-pdb-worker", Namespace: "test-ns"}, &pdb)
	require.NoError(t, err)
}

func TestPDBBlockImplementsInjector(t *testing.T) {
	scheme := newPDBTestScheme(t)
	fakeClient := fake.NewClientBuilder().WithScheme(scheme).Build()

	var _ Injector = NewPDBBlockInjector(fakeClient)
}

func TestPDBBlockTypeIsValid(t *testing.T) {
	err := v1alpha1.ValidateInjectionType(v1alpha1.PDBBlock)
	assert.NoError(t, err)
}

func TestPDBBlockRevertIdempotent(t *testing.T) {
	scheme := newPDBTestScheme(t)

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		Build()

	injector := NewPDBBlockInjector(fakeClient)

	spec := v1alpha1.InjectionSpec{
		Type:        v1alpha1.PDBBlock,
		DangerLevel: v1alpha1.DangerLevelHigh,
		Parameters: map[string]string{
			"labelSelector": "app=my-app",
		},
	}

	// Revert with no PDB should be a no-op
	err := injector.Revert(context.Background(), spec, "test-ns")
	assert.NoError(t, err)
}

func TestPDBBlockWithCustomName(t *testing.T) {
	scheme := newPDBTestScheme(t)

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		Build()

	injector := NewPDBBlockInjector(fakeClient)

	spec := v1alpha1.InjectionSpec{
		Type:        v1alpha1.PDBBlock,
		DangerLevel: v1alpha1.DangerLevelHigh,
		Parameters: map[string]string{
			"labelSelector": "app=my-app",
			"name":          "my-custom-pdb",
		},
	}

	ctx := context.Background()
	_, _, err := injector.Inject(ctx, spec, "test-ns")
	require.NoError(t, err)

	// Verify PDB was created with the custom name
	var pdb policyv1.PodDisruptionBudget
	err = fakeClient.Get(ctx, types.NamespacedName{Name: "my-custom-pdb", Namespace: "test-ns"}, &pdb)
	require.NoError(t, err)

	// Revert should also use the custom name
	err = injector.Revert(ctx, spec, "test-ns")
	require.NoError(t, err)

	err = fakeClient.Get(ctx, types.NamespacedName{Name: "my-custom-pdb", Namespace: "test-ns"}, &pdb)
	require.Error(t, err)
}

func TestParseLabelSelector(t *testing.T) {
	tests := []struct {
		input    string
		expected map[string]string
		wantErr  bool
	}{
		{"app=my-app", map[string]string{"app": "my-app"}, false},
		{"app=my-app,env=prod", map[string]string{"app": "my-app", "env": "prod"}, false},
		{"invalid", nil, true},
		{"=value", nil, true},
		{"key=", nil, true},
		{"", nil, true},
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			result, err := parseLabelSelector(tc.input)
			if tc.wantErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tc.expected, result)
			}
		})
	}
}
