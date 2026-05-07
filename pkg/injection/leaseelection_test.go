package injection

import (
	"context"
	"testing"

	v1alpha1 "github.com/opendatahub-io/operator-chaos/api/v1alpha1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	coordinationv1 "k8s.io/api/coordination/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func newLeaseTestScheme(t *testing.T) *runtime.Scheme {
	t.Helper()
	scheme := runtime.NewScheme()
	require.NoError(t, corev1.AddToScheme(scheme))
	require.NoError(t, coordinationv1.AddToScheme(scheme))
	return scheme
}

func TestLeaseElectionValidateMissingName(t *testing.T) {
	injector := &LeaseElectionInjector{}

	spec := v1alpha1.InjectionSpec{
		Type:        v1alpha1.LeaderElectionDisrupt,
		DangerLevel: v1alpha1.DangerLevelHigh,
		Parameters:  map[string]string{},
	}
	blast := v1alpha1.BlastRadiusSpec{MaxPodsAffected: 1}

	err := injector.Validate(spec, blast)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "name")
}

func TestLeaseElectionValidateRejectsNonHighDangerLevel(t *testing.T) {
	injector := &LeaseElectionInjector{}

	spec := v1alpha1.InjectionSpec{
		Type:        v1alpha1.LeaderElectionDisrupt,
		DangerLevel: v1alpha1.DangerLevelMedium,
		Parameters: map[string]string{
			"name": "my-lease",
		},
	}
	blast := v1alpha1.BlastRadiusSpec{MaxPodsAffected: 1}

	err := injector.Validate(spec, blast)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "dangerLevel: high")
}

func TestLeaseElectionValidateRejectsEmptyDangerLevel(t *testing.T) {
	injector := &LeaseElectionInjector{}

	spec := v1alpha1.InjectionSpec{
		Type: v1alpha1.LeaderElectionDisrupt,
		Parameters: map[string]string{
			"name": "my-lease",
		},
	}
	blast := v1alpha1.BlastRadiusSpec{MaxPodsAffected: 1}

	err := injector.Validate(spec, blast)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "dangerLevel: high")
}

func TestLeaseElectionValidateAcceptsValid(t *testing.T) {
	injector := &LeaseElectionInjector{}

	spec := v1alpha1.InjectionSpec{
		Type:        v1alpha1.LeaderElectionDisrupt,
		DangerLevel: v1alpha1.DangerLevelHigh,
		Parameters: map[string]string{
			"name": "my-lease",
		},
	}
	blast := v1alpha1.BlastRadiusSpec{MaxPodsAffected: 1}

	err := injector.Validate(spec, blast)
	assert.NoError(t, err)
}

func TestLeaseElectionValidateRejectsChaosManagedResource(t *testing.T) {
	injector := &LeaseElectionInjector{}

	spec := v1alpha1.InjectionSpec{
		Type:        v1alpha1.LeaderElectionDisrupt,
		DangerLevel: v1alpha1.DangerLevelHigh,
		Parameters: map[string]string{
			"name": "chaos-backup-something",
		},
	}
	blast := v1alpha1.BlastRadiusSpec{MaxPodsAffected: 1}

	err := injector.Validate(spec, blast)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "chaos-managed")
}

func TestLeaseElectionInjectDeletesLeaseAndCreatesBackup(t *testing.T) {
	scheme := newLeaseTestScheme(t)

	holderID := "controller-1"
	lease := &coordinationv1.Lease{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-controller",
			Namespace: "test-ns",
		},
		Spec: coordinationv1.LeaseSpec{
			HolderIdentity: &holderID,
		},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(lease).
		Build()

	injector := NewLeaseElectionInjector(fakeClient)

	spec := v1alpha1.InjectionSpec{
		Type:        v1alpha1.LeaderElectionDisrupt,
		DangerLevel: v1alpha1.DangerLevelHigh,
		Parameters: map[string]string{
			"name": "my-controller",
		},
	}

	ctx := context.Background()
	cleanup, events, err := injector.Inject(ctx, spec, "test-ns")
	require.NoError(t, err)
	assert.Len(t, events, 1)
	assert.Equal(t, v1alpha1.LeaderElectionDisrupt, events[0].Type)
	assert.Equal(t, "deleted", events[0].Action)
	assert.NotNil(t, cleanup)

	// Verify Lease is deleted
	var deletedLease coordinationv1.Lease
	err = fakeClient.Get(ctx, types.NamespacedName{Name: "my-controller", Namespace: "test-ns"}, &deletedLease)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")

	// Verify backup ConfigMap exists
	var backupCM corev1.ConfigMap
	err = fakeClient.Get(ctx, types.NamespacedName{Name: "chaos-backup-lease-my-controller", Namespace: "test-ns"}, &backupCM)
	require.NoError(t, err)
	assert.Contains(t, backupCM.Data, "lease-backup")
}

func TestLeaseElectionCleanupRestoresLease(t *testing.T) {
	scheme := newLeaseTestScheme(t)

	holderID := "controller-1"
	lease := &coordinationv1.Lease{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-controller",
			Namespace: "test-ns",
		},
		Spec: coordinationv1.LeaseSpec{
			HolderIdentity: &holderID,
		},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(lease).
		Build()

	injector := NewLeaseElectionInjector(fakeClient)

	spec := v1alpha1.InjectionSpec{
		Type:        v1alpha1.LeaderElectionDisrupt,
		DangerLevel: v1alpha1.DangerLevelHigh,
		Parameters: map[string]string{
			"name": "my-controller",
		},
	}

	ctx := context.Background()
	cleanup, _, err := injector.Inject(ctx, spec, "test-ns")
	require.NoError(t, err)

	// Run cleanup (lease not recreated by operator)
	err = cleanup(ctx)
	require.NoError(t, err)

	// Verify Lease is restored
	var restoredLease coordinationv1.Lease
	err = fakeClient.Get(ctx, types.NamespacedName{Name: "my-controller", Namespace: "test-ns"}, &restoredLease)
	require.NoError(t, err)
	assert.Equal(t, "controller-1", *restoredLease.Spec.HolderIdentity)

	// Verify backup ConfigMap is cleaned up
	var backupCM corev1.ConfigMap
	err = fakeClient.Get(ctx, types.NamespacedName{Name: "chaos-backup-lease-my-controller", Namespace: "test-ns"}, &backupCM)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestLeaseElectionRevertWhenLeaseAlreadyRecreated(t *testing.T) {
	scheme := newLeaseTestScheme(t)

	// Simulate: Lease was recreated by the operator, but backup ConfigMap still exists
	newHolderID := "controller-2"
	lease := &coordinationv1.Lease{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-controller",
			Namespace: "test-ns",
		},
		Spec: coordinationv1.LeaseSpec{
			HolderIdentity: &newHolderID,
		},
	}
	backupCM := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "chaos-backup-lease-my-controller",
			Namespace: "test-ns",
			Labels: map[string]string{
				"app.kubernetes.io/managed-by":  "operator-chaos",
				"chaos.operatorchaos.io/type":   "LeaderElectionDisrupt",
			},
		},
		Data: map[string]string{
			"lease-backup": `{"metadata":{"name":"my-controller","namespace":"test-ns"},"spec":{"holderIdentity":"controller-1"}}`,
		},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(lease, backupCM).
		Build()

	injector := NewLeaseElectionInjector(fakeClient)

	spec := v1alpha1.InjectionSpec{
		Type:        v1alpha1.LeaderElectionDisrupt,
		DangerLevel: v1alpha1.DangerLevelHigh,
		Parameters: map[string]string{
			"name": "my-controller",
		},
	}

	ctx := context.Background()
	err := injector.Revert(ctx, spec, "test-ns")
	require.NoError(t, err)

	// Backup ConfigMap should be cleaned up
	var cm corev1.ConfigMap
	err = fakeClient.Get(ctx, types.NamespacedName{Name: "chaos-backup-lease-my-controller", Namespace: "test-ns"}, &cm)
	require.Error(t, err)

	// Lease should still exist with the operator's new holder
	var l coordinationv1.Lease
	err = fakeClient.Get(ctx, types.NamespacedName{Name: "my-controller", Namespace: "test-ns"}, &l)
	require.NoError(t, err)
	assert.Equal(t, "controller-2", *l.Spec.HolderIdentity)
}

func TestLeaseElectionRevertIdempotent(t *testing.T) {
	scheme := newLeaseTestScheme(t)

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		Build()

	injector := NewLeaseElectionInjector(fakeClient)

	spec := v1alpha1.InjectionSpec{
		Type:        v1alpha1.LeaderElectionDisrupt,
		DangerLevel: v1alpha1.DangerLevelHigh,
		Parameters: map[string]string{
			"name": "nonexistent-lease",
		},
	}

	// Revert with no backup ConfigMap should be a no-op
	err := injector.Revert(context.Background(), spec, "test-ns")
	assert.NoError(t, err)
}

func TestLeaseElectionInjectLeaseNotFound(t *testing.T) {
	scheme := newLeaseTestScheme(t)

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		Build()

	injector := NewLeaseElectionInjector(fakeClient)

	spec := v1alpha1.InjectionSpec{
		Type:        v1alpha1.LeaderElectionDisrupt,
		DangerLevel: v1alpha1.DangerLevelHigh,
		Parameters: map[string]string{
			"name": "nonexistent",
		},
	}

	_, _, err := injector.Inject(context.Background(), spec, "test-ns")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "getting Lease")
}

func TestLeaseElectionImplementsInjector(t *testing.T) {
	scheme := newLeaseTestScheme(t)
	fakeClient := fake.NewClientBuilder().WithScheme(scheme).Build()

	var _ Injector = NewLeaseElectionInjector(fakeClient)
}

func TestLeaseElectionTypeIsValid(t *testing.T) {
	err := v1alpha1.ValidateInjectionType(v1alpha1.LeaderElectionDisrupt)
	assert.NoError(t, err)
}

func TestLeaseBackupConfigMapName(t *testing.T) {
	assert.Equal(t, "chaos-backup-lease-my-controller", leaseBackupConfigMapName("my-controller"))
	assert.Equal(t, "chaos-backup-lease-operator-lock", leaseBackupConfigMapName("operator-lock"))
}
