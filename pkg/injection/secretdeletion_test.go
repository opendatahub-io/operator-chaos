package injection

import (
	"context"
	"testing"

	v1alpha1 "github.com/opendatahub-io/operator-chaos/api/v1alpha1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestSecretDeletionValidateMissingName(t *testing.T) {
	injector := &SecretDeletionInjector{}

	spec := v1alpha1.InjectionSpec{
		Type:        v1alpha1.SecretDeletion,
		DangerLevel: v1alpha1.DangerLevelHigh,
		Parameters:  map[string]string{},
	}
	blast := v1alpha1.BlastRadiusSpec{MaxPodsAffected: 1}

	err := injector.Validate(spec, blast)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "name")
}

func TestSecretDeletionValidateRejectsNonHighDangerLevel(t *testing.T) {
	injector := &SecretDeletionInjector{}

	spec := v1alpha1.InjectionSpec{
		Type:        v1alpha1.SecretDeletion,
		DangerLevel: v1alpha1.DangerLevelMedium,
		Parameters: map[string]string{
			"name": "my-secret",
		},
	}
	blast := v1alpha1.BlastRadiusSpec{MaxPodsAffected: 1}

	err := injector.Validate(spec, blast)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "dangerLevel: high")
}

func TestSecretDeletionValidateRejectsEmptyDangerLevel(t *testing.T) {
	injector := &SecretDeletionInjector{}

	spec := v1alpha1.InjectionSpec{
		Type: v1alpha1.SecretDeletion,
		Parameters: map[string]string{
			"name": "my-secret",
		},
	}
	blast := v1alpha1.BlastRadiusSpec{MaxPodsAffected: 1}

	err := injector.Validate(spec, blast)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "dangerLevel: high")
}

func TestSecretDeletionValidateAcceptsValid(t *testing.T) {
	injector := &SecretDeletionInjector{}

	spec := v1alpha1.InjectionSpec{
		Type:        v1alpha1.SecretDeletion,
		DangerLevel: v1alpha1.DangerLevelHigh,
		Parameters: map[string]string{
			"name": "my-secret",
		},
	}
	blast := v1alpha1.BlastRadiusSpec{MaxPodsAffected: 1}

	err := injector.Validate(spec, blast)
	assert.NoError(t, err)
}

func TestSecretDeletionValidateRejectsSystemCriticalSecret(t *testing.T) {
	injector := &SecretDeletionInjector{}

	spec := v1alpha1.InjectionSpec{
		Type:        v1alpha1.SecretDeletion,
		DangerLevel: v1alpha1.DangerLevelHigh,
		Parameters: map[string]string{
			"name": "etcd-client",
		},
	}
	blast := v1alpha1.BlastRadiusSpec{MaxPodsAffected: 1}

	err := injector.Validate(spec, blast)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "system-critical")
}

func TestSecretDeletionValidateRejectsChaosManagedResource(t *testing.T) {
	injector := &SecretDeletionInjector{}

	spec := v1alpha1.InjectionSpec{
		Type:        v1alpha1.SecretDeletion,
		DangerLevel: v1alpha1.DangerLevelHigh,
		Parameters: map[string]string{
			"name": "chaos-rollback-something",
		},
	}
	blast := v1alpha1.BlastRadiusSpec{MaxPodsAffected: 1}

	err := injector.Validate(spec, blast)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "chaos-managed")
}

func newTestScheme(t *testing.T) *runtime.Scheme {
	t.Helper()
	scheme := runtime.NewScheme()
	require.NoError(t, corev1.AddToScheme(scheme))
	return scheme
}

func TestSecretDeletionInjectDeletesSecretAndCreatesBackup(t *testing.T) {
	scheme := newTestScheme(t)

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-secret",
			Namespace: "test-ns",
		},
		Data: map[string][]byte{
			"password": []byte("super-secret"),
		},
		Type: corev1.SecretTypeOpaque,
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(secret).
		Build()

	injector := NewSecretDeletionInjector(fakeClient)

	spec := v1alpha1.InjectionSpec{
		Type:        v1alpha1.SecretDeletion,
		DangerLevel: v1alpha1.DangerLevelHigh,
		Parameters: map[string]string{
			"name": "my-secret",
		},
	}

	ctx := context.Background()
	cleanup, events, err := injector.Inject(ctx, spec, "test-ns")
	require.NoError(t, err)
	assert.Len(t, events, 1)
	assert.Equal(t, v1alpha1.SecretDeletion, events[0].Type)
	assert.Equal(t, "deleted", events[0].Action)
	assert.NotNil(t, cleanup)

	// Verify Secret is deleted
	var deletedSecret corev1.Secret
	err = fakeClient.Get(ctx, types.NamespacedName{Name: "my-secret", Namespace: "test-ns"}, &deletedSecret)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")

	// Verify backup Secret exists
	var backupSecret corev1.Secret
	err = fakeClient.Get(ctx, types.NamespacedName{Name: "chaos-backup-secret-my-secret", Namespace: "test-ns"}, &backupSecret)
	require.NoError(t, err)
	assert.Contains(t, backupSecret.Data, "secret-backup")
}

func TestSecretDeletionCleanupRestoresSecret(t *testing.T) {
	scheme := newTestScheme(t)

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-secret",
			Namespace: "test-ns",
		},
		Data: map[string][]byte{
			"password": []byte("super-secret"),
		},
		Type: corev1.SecretTypeOpaque,
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(secret).
		Build()

	injector := NewSecretDeletionInjector(fakeClient)

	spec := v1alpha1.InjectionSpec{
		Type:        v1alpha1.SecretDeletion,
		DangerLevel: v1alpha1.DangerLevelHigh,
		Parameters: map[string]string{
			"name": "my-secret",
		},
	}

	ctx := context.Background()
	cleanup, _, err := injector.Inject(ctx, spec, "test-ns")
	require.NoError(t, err)

	// Run cleanup
	err = cleanup(ctx)
	require.NoError(t, err)

	// Verify Secret is restored
	var restoredSecret corev1.Secret
	err = fakeClient.Get(ctx, types.NamespacedName{Name: "my-secret", Namespace: "test-ns"}, &restoredSecret)
	require.NoError(t, err)
	assert.Equal(t, []byte("super-secret"), restoredSecret.Data["password"])

	// Verify backup Secret is cleaned up
	var backupSecret corev1.Secret
	err = fakeClient.Get(ctx, types.NamespacedName{Name: "chaos-backup-secret-my-secret", Namespace: "test-ns"}, &backupSecret)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestSecretDeletionRevertRestoresSecret(t *testing.T) {
	scheme := newTestScheme(t)

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-secret",
			Namespace: "test-ns",
		},
		Data: map[string][]byte{
			"token": []byte("abc123"),
		},
		Type: corev1.SecretTypeOpaque,
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(secret).
		Build()

	injector := NewSecretDeletionInjector(fakeClient)

	spec := v1alpha1.InjectionSpec{
		Type:        v1alpha1.SecretDeletion,
		DangerLevel: v1alpha1.DangerLevelHigh,
		Parameters: map[string]string{
			"name": "my-secret",
		},
	}

	ctx := context.Background()
	_, _, err := injector.Inject(ctx, spec, "test-ns")
	require.NoError(t, err)

	// Revert should restore the Secret
	err = injector.Revert(ctx, spec, "test-ns")
	require.NoError(t, err)

	// Verify Secret is restored
	var restoredSecret corev1.Secret
	err = fakeClient.Get(ctx, types.NamespacedName{Name: "my-secret", Namespace: "test-ns"}, &restoredSecret)
	require.NoError(t, err)
	assert.Equal(t, []byte("abc123"), restoredSecret.Data["token"])

	// Verify backup Secret is cleaned up
	var backupSecret corev1.Secret
	err = fakeClient.Get(ctx, types.NamespacedName{Name: "chaos-backup-secret-my-secret", Namespace: "test-ns"}, &backupSecret)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestSecretDeletionRevertIdempotent(t *testing.T) {
	scheme := newTestScheme(t)

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		Build()

	injector := NewSecretDeletionInjector(fakeClient)

	spec := v1alpha1.InjectionSpec{
		Type:        v1alpha1.SecretDeletion,
		DangerLevel: v1alpha1.DangerLevelHigh,
		Parameters: map[string]string{
			"name": "nonexistent-secret",
		},
	}

	// Revert with no backup Secret should be a no-op
	err := injector.Revert(context.Background(), spec, "test-ns")
	assert.NoError(t, err)
}

func TestSecretDeletionInjectSecretNotFound(t *testing.T) {
	scheme := newTestScheme(t)

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		Build()

	injector := NewSecretDeletionInjector(fakeClient)

	spec := v1alpha1.InjectionSpec{
		Type:        v1alpha1.SecretDeletion,
		DangerLevel: v1alpha1.DangerLevelHigh,
		Parameters: map[string]string{
			"name": "nonexistent",
		},
	}

	_, _, err := injector.Inject(context.Background(), spec, "test-ns")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "getting Secret")
}

func TestSecretDeletionInjectCustomNamespace(t *testing.T) {
	scheme := newTestScheme(t)

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-secret",
			Namespace: "custom-ns",
		},
		Data: map[string][]byte{
			"key": []byte("value"),
		},
		Type: corev1.SecretTypeOpaque,
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(secret).
		Build()

	injector := NewSecretDeletionInjector(fakeClient)

	spec := v1alpha1.InjectionSpec{
		Type:        v1alpha1.SecretDeletion,
		DangerLevel: v1alpha1.DangerLevelHigh,
		Parameters: map[string]string{
			"name":      "my-secret",
			"namespace": "custom-ns",
		},
	}

	ctx := context.Background()
	cleanup, events, err := injector.Inject(ctx, spec, "experiment-ns")
	require.NoError(t, err)
	assert.Len(t, events, 1)
	assert.NotNil(t, cleanup)

	// Verify Secret is deleted from the custom namespace
	var deleted corev1.Secret
	err = fakeClient.Get(ctx, types.NamespacedName{Name: "my-secret", Namespace: "custom-ns"}, &deleted)
	require.Error(t, err)

	// Verify backup is in the custom namespace
	var backupSecret corev1.Secret
	err = fakeClient.Get(ctx, types.NamespacedName{Name: "chaos-backup-secret-my-secret", Namespace: "custom-ns"}, &backupSecret)
	require.NoError(t, err)

	// Cleanup should restore in custom namespace
	err = cleanup(ctx)
	require.NoError(t, err)

	var restored corev1.Secret
	err = fakeClient.Get(ctx, types.NamespacedName{Name: "my-secret", Namespace: "custom-ns"}, &restored)
	require.NoError(t, err)
	assert.Equal(t, []byte("value"), restored.Data["key"])
}

func TestSecretDeletionRevertWhenSecretAlreadyExists(t *testing.T) {
	scheme := newTestScheme(t)

	// Simulate: Secret was recreated by the operator, but backup Secret still exists
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-secret",
			Namespace: "test-ns",
		},
		Data: map[string][]byte{
			"key": []byte("new-value"),
		},
	}
	backupSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "chaos-backup-secret-my-secret",
			Namespace: "test-ns",
			Labels: map[string]string{
				"app.kubernetes.io/managed-by": "operator-chaos",
				"chaos.operatorchaos.io/type":  "SecretDeletion",
			},
		},
		Data: map[string][]byte{
			"secret-backup": []byte(`{"metadata":{"name":"my-secret","namespace":"test-ns"},"data":{"key":"b2xkLXZhbHVl"},"type":"Opaque"}`),
		},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(secret, backupSecret).
		Build()

	injector := NewSecretDeletionInjector(fakeClient)

	spec := v1alpha1.InjectionSpec{
		Type:        v1alpha1.SecretDeletion,
		DangerLevel: v1alpha1.DangerLevelHigh,
		Parameters: map[string]string{
			"name": "my-secret",
		},
	}

	ctx := context.Background()
	err := injector.Revert(ctx, spec, "test-ns")
	require.NoError(t, err)

	// Backup Secret should be cleaned up
	var bkp corev1.Secret
	err = fakeClient.Get(ctx, types.NamespacedName{Name: "chaos-backup-secret-my-secret", Namespace: "test-ns"}, &bkp)
	require.Error(t, err)

	// Secret should still exist (the already-exists one)
	var s corev1.Secret
	err = fakeClient.Get(ctx, types.NamespacedName{Name: "my-secret", Namespace: "test-ns"}, &s)
	require.NoError(t, err)
}

func TestSecretDeletionInjectMultipleKeys(t *testing.T) {
	scheme := newTestScheme(t)

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "multi-key-secret",
			Namespace: "test-ns",
			Labels: map[string]string{
				"app": "myapp",
			},
		},
		Data: map[string][]byte{
			"username": []byte("admin"),
			"password": []byte("p@ssw0rd"),
			"token":    []byte("jwt-token-here"),
		},
		Type: corev1.SecretTypeOpaque,
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(secret).
		Build()

	injector := NewSecretDeletionInjector(fakeClient)

	spec := v1alpha1.InjectionSpec{
		Type:        v1alpha1.SecretDeletion,
		DangerLevel: v1alpha1.DangerLevelHigh,
		Parameters: map[string]string{
			"name": "multi-key-secret",
		},
	}

	ctx := context.Background()
	cleanup, _, err := injector.Inject(ctx, spec, "test-ns")
	require.NoError(t, err)

	// Restore
	err = cleanup(ctx)
	require.NoError(t, err)

	// All keys should be restored
	var restored corev1.Secret
	err = fakeClient.Get(ctx, types.NamespacedName{Name: "multi-key-secret", Namespace: "test-ns"}, &restored)
	require.NoError(t, err)
	assert.Equal(t, []byte("admin"), restored.Data["username"])
	assert.Equal(t, []byte("p@ssw0rd"), restored.Data["password"])
	assert.Equal(t, []byte("jwt-token-here"), restored.Data["token"])
	assert.Equal(t, "myapp", restored.Labels["app"])
}

func TestBackupSecretName(t *testing.T) {
	assert.Equal(t, "chaos-backup-secret-my-secret", backupSecretName("my-secret"))
	assert.Equal(t, "chaos-backup-secret-tls-cert", backupSecretName("tls-cert"))
}

// Verify the injection type is properly registered as valid.
func TestSecretDeletionTypeIsValid(t *testing.T) {
	err := v1alpha1.ValidateInjectionType(v1alpha1.SecretDeletion)
	assert.NoError(t, err)
}

// Verify all injector methods exist and NewSecretDeletionInjector returns the right type.
func TestSecretDeletionImplementsInjector(t *testing.T) {
	scheme := newTestScheme(t)
	fakeClient := fake.NewClientBuilder().WithScheme(scheme).Build()

	var _ Injector = NewSecretDeletionInjector(fakeClient)
}

// Verify List+Get works to confirm cleanup is complete.
func TestSecretDeletionFullCycleVerifiesCleanup(t *testing.T) {
	scheme := newTestScheme(t)

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "lifecycle-secret",
			Namespace: "test-ns",
		},
		Data: map[string][]byte{
			"data": []byte("important"),
		},
		Type: corev1.SecretTypeOpaque,
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(secret).
		Build()

	injector := NewSecretDeletionInjector(fakeClient)

	spec := v1alpha1.InjectionSpec{
		Type:        v1alpha1.SecretDeletion,
		DangerLevel: v1alpha1.DangerLevelHigh,
		Parameters: map[string]string{
			"name": "lifecycle-secret",
		},
	}

	ctx := context.Background()

	// Inject
	cleanup, _, err := injector.Inject(ctx, spec, "test-ns")
	require.NoError(t, err)

	// Verify backup Secret exists
	var backupSecret corev1.Secret
	err = fakeClient.Get(ctx, types.NamespacedName{Name: "chaos-backup-secret-lifecycle-secret", Namespace: "test-ns"}, &backupSecret)
	require.NoError(t, err)

	// Cleanup
	require.NoError(t, cleanup(ctx))

	// Verify backup Secret is removed
	err = fakeClient.Get(ctx, types.NamespacedName{Name: "chaos-backup-secret-lifecycle-secret", Namespace: "test-ns"}, &backupSecret)
	require.Error(t, err)

	// Verify Secret is back
	var restored corev1.Secret
	require.NoError(t, fakeClient.Get(ctx, types.NamespacedName{Name: "lifecycle-secret", Namespace: "test-ns"}, &restored))
	assert.Equal(t, []byte("important"), restored.Data["data"])
}
