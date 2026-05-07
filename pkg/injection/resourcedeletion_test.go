package injection

import (
	"context"
	"testing"

	v1alpha1 "github.com/opendatahub-io/operator-chaos/api/v1alpha1"
	"github.com/opendatahub-io/operator-chaos/pkg/safety"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func newResourceDeletionTestScheme(t *testing.T) *runtime.Scheme {
	t.Helper()
	scheme := runtime.NewScheme()
	require.NoError(t, corev1.AddToScheme(scheme))
	return scheme
}

func TestResourceDeletionValidateRejectsMissingAPIVersion(t *testing.T) {
	injector := &ResourceDeletionInjector{}

	spec := v1alpha1.InjectionSpec{
		Type:        v1alpha1.ResourceDeletion,
		DangerLevel: v1alpha1.DangerLevelHigh,
		Parameters: map[string]string{
			"kind": "Service",
			"name": "my-svc",
		},
	}
	blast := v1alpha1.BlastRadiusSpec{MaxPodsAffected: 1}

	err := injector.Validate(spec, blast)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "apiVersion")
}

func TestResourceDeletionValidateRejectsMissingKind(t *testing.T) {
	injector := &ResourceDeletionInjector{}

	spec := v1alpha1.InjectionSpec{
		Type:        v1alpha1.ResourceDeletion,
		DangerLevel: v1alpha1.DangerLevelHigh,
		Parameters: map[string]string{
			"apiVersion": "v1",
			"name":       "my-svc",
		},
	}
	blast := v1alpha1.BlastRadiusSpec{MaxPodsAffected: 1}

	err := injector.Validate(spec, blast)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "kind")
}

func TestResourceDeletionValidateRejectsMissingName(t *testing.T) {
	injector := &ResourceDeletionInjector{}

	spec := v1alpha1.InjectionSpec{
		Type:        v1alpha1.ResourceDeletion,
		DangerLevel: v1alpha1.DangerLevelHigh,
		Parameters: map[string]string{
			"apiVersion": "v1",
			"kind":       "Service",
		},
	}
	blast := v1alpha1.BlastRadiusSpec{MaxPodsAffected: 1}

	err := injector.Validate(spec, blast)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "name")
}

func TestResourceDeletionValidateRejectsMissingDangerLevel(t *testing.T) {
	injector := &ResourceDeletionInjector{}

	spec := v1alpha1.InjectionSpec{
		Type: v1alpha1.ResourceDeletion,
		Parameters: map[string]string{
			"apiVersion": "v1",
			"kind":       "Service",
			"name":       "my-svc",
		},
	}
	blast := v1alpha1.BlastRadiusSpec{MaxPodsAffected: 1}

	err := injector.Validate(spec, blast)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "dangerLevel: high")
}

func TestResourceDeletionValidateRejectsChaosManagedResource(t *testing.T) {
	injector := &ResourceDeletionInjector{}

	spec := v1alpha1.InjectionSpec{
		Type:        v1alpha1.ResourceDeletion,
		DangerLevel: v1alpha1.DangerLevelHigh,
		Parameters: map[string]string{
			"apiVersion": "v1",
			"kind":       "Service",
			"name":       "chaos-backup-something",
		},
	}
	blast := v1alpha1.BlastRadiusSpec{MaxPodsAffected: 1}

	err := injector.Validate(spec, blast)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "chaos-managed")
}

func TestResourceDeletionValidateRejectsSystemCriticalSecret(t *testing.T) {
	injector := &ResourceDeletionInjector{}

	spec := v1alpha1.InjectionSpec{
		Type:        v1alpha1.ResourceDeletion,
		DangerLevel: v1alpha1.DangerLevelHigh,
		Parameters: map[string]string{
			"apiVersion": "v1",
			"kind":       "Secret",
			"name":       "etcd-client",
		},
	}
	blast := v1alpha1.BlastRadiusSpec{MaxPodsAffected: 1}

	err := injector.Validate(spec, blast)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "system-critical Secret")
}

func TestResourceDeletionValidateRejectsSystemCriticalService(t *testing.T) {
	injector := &ResourceDeletionInjector{}

	spec := v1alpha1.InjectionSpec{
		Type:        v1alpha1.ResourceDeletion,
		DangerLevel: v1alpha1.DangerLevelHigh,
		Parameters: map[string]string{
			"apiVersion": "v1",
			"kind":       "Service",
			"name":       "kubernetes",
		},
	}
	blast := v1alpha1.BlastRadiusSpec{MaxPodsAffected: 1}

	err := injector.Validate(spec, blast)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "system-critical Service")
}

func TestResourceDeletionValidateRejectsSystemCriticalServiceAccount(t *testing.T) {
	injector := &ResourceDeletionInjector{}

	spec := v1alpha1.InjectionSpec{
		Type:        v1alpha1.ResourceDeletion,
		DangerLevel: v1alpha1.DangerLevelHigh,
		Parameters: map[string]string{
			"apiVersion": "v1",
			"kind":       "ServiceAccount",
			"name":       "default",
		},
	}
	blast := v1alpha1.BlastRadiusSpec{MaxPodsAffected: 1}

	err := injector.Validate(spec, blast)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "system-critical ServiceAccount")
}

func TestResourceDeletionValidateRejectsClusterScopedKind(t *testing.T) {
	for _, kind := range []string{"Namespace", "Node", "ClusterRole", "ClusterRoleBinding", "CustomResourceDefinition", "PersistentVolume"} {
		t.Run(kind, func(t *testing.T) {
			injector := &ResourceDeletionInjector{}

			spec := v1alpha1.InjectionSpec{
				Type:        v1alpha1.ResourceDeletion,
				DangerLevel: v1alpha1.DangerLevelHigh,
				Parameters: map[string]string{
					"apiVersion": "v1",
					"kind":       kind,
					"name":       "test-resource",
				},
			}
			blast := v1alpha1.BlastRadiusSpec{MaxPodsAffected: 1}

			err := injector.Validate(spec, blast)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "cluster-scoped")
		})
	}
}

func TestResourceDeletionValidateRejectsForbiddenNamespace(t *testing.T) {
	injector := &ResourceDeletionInjector{}

	spec := v1alpha1.InjectionSpec{
		Type:        v1alpha1.ResourceDeletion,
		DangerLevel: v1alpha1.DangerLevelHigh,
		Parameters: map[string]string{
			"apiVersion": "v1",
			"kind":       "Service",
			"name":       "my-svc",
			"namespace":  "kube-system",
		},
	}
	blast := v1alpha1.BlastRadiusSpec{MaxPodsAffected: 1}

	err := injector.Validate(spec, blast)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "protected namespace")
}

func TestResourceDeletionValidateRejectsForbiddenNamespacePrefix(t *testing.T) {
	injector := &ResourceDeletionInjector{}

	spec := v1alpha1.InjectionSpec{
		Type:        v1alpha1.ResourceDeletion,
		DangerLevel: v1alpha1.DangerLevelHigh,
		Parameters: map[string]string{
			"apiVersion": "v1",
			"kind":       "Service",
			"name":       "my-svc",
			"namespace":  "openshift-monitoring",
		},
	}
	blast := v1alpha1.BlastRadiusSpec{MaxPodsAffected: 1}

	err := injector.Validate(spec, blast)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "protected")
}

func TestResourceDeletionInjectDeletesResourceAndCreatesBackup(t *testing.T) {
	scheme := newResourceDeletionTestScheme(t)

	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-svc",
			Namespace: "test-ns",
		},
		Spec: corev1.ServiceSpec{
			Ports: []corev1.ServicePort{
				{Port: 80, Protocol: corev1.ProtocolTCP},
			},
		},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(svc).
		Build()

	injector := NewResourceDeletionInjector(fakeClient)

	spec := v1alpha1.InjectionSpec{
		Type:        v1alpha1.ResourceDeletion,
		DangerLevel: v1alpha1.DangerLevelHigh,
		Parameters: map[string]string{
			"apiVersion": "v1",
			"kind":       "Service",
			"name":       "my-svc",
		},
	}

	ctx := context.Background()
	cleanup, events, err := injector.Inject(ctx, spec, "test-ns")
	require.NoError(t, err)
	assert.Len(t, events, 1)
	assert.Equal(t, v1alpha1.ResourceDeletion, events[0].Type)
	assert.Equal(t, "deleted", events[0].Action)
	assert.NotNil(t, cleanup)

	// Verify Service is deleted
	var deleted corev1.Service
	err = fakeClient.Get(ctx, types.NamespacedName{Name: "my-svc", Namespace: "test-ns"}, &deleted)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")

	// Verify backup Secret exists
	var backupSecret corev1.Secret
	err = fakeClient.Get(ctx, types.NamespacedName{Name: "chaos-backup-resource-service-my-svc", Namespace: "test-ns"}, &backupSecret)
	require.NoError(t, err)
	assert.Contains(t, backupSecret.Data, "resource-backup")
}

func TestResourceDeletionRevertRestoresResourceWhenNotRecreated(t *testing.T) {
	scheme := newResourceDeletionTestScheme(t)

	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-svc",
			Namespace: "test-ns",
		},
		Spec: corev1.ServiceSpec{
			Ports: []corev1.ServicePort{
				{Port: 80, Protocol: corev1.ProtocolTCP},
			},
		},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(svc).
		Build()

	injector := NewResourceDeletionInjector(fakeClient)

	spec := v1alpha1.InjectionSpec{
		Type:        v1alpha1.ResourceDeletion,
		DangerLevel: v1alpha1.DangerLevelHigh,
		Parameters: map[string]string{
			"apiVersion": "v1",
			"kind":       "Service",
			"name":       "my-svc",
		},
	}

	ctx := context.Background()
	_, _, err := injector.Inject(ctx, spec, "test-ns")
	require.NoError(t, err)

	// Revert should restore the Service
	err = injector.Revert(ctx, spec, "test-ns")
	require.NoError(t, err)

	// Verify Service is restored
	var restored corev1.Service
	err = fakeClient.Get(ctx, types.NamespacedName{Name: "my-svc", Namespace: "test-ns"}, &restored)
	require.NoError(t, err)

	// Verify backup Secret is cleaned up
	var backupSecret corev1.Secret
	err = fakeClient.Get(ctx, types.NamespacedName{Name: "chaos-backup-resource-service-my-svc", Namespace: "test-ns"}, &backupSecret)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestResourceDeletionRevertCleansUpBackupWhenAlreadyRecreated(t *testing.T) {
	scheme := newResourceDeletionTestScheme(t)

	// Simulate: Service was recreated by operator, backup Secret still exists
	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-svc",
			Namespace: "test-ns",
		},
		Spec: corev1.ServiceSpec{
			Ports: []corev1.ServicePort{
				{Port: 80, Protocol: corev1.ProtocolTCP},
			},
		},
	}
	backupSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "chaos-backup-resource-service-my-svc",
			Namespace: "test-ns",
			Labels: map[string]string{
				"app.kubernetes.io/managed-by": "operator-chaos",
				"chaos.operatorchaos.io/type":  "ResourceDeletion",
			},
		},
		Data: map[string][]byte{
			"resource-backup": []byte(`{"apiVersion":"v1","kind":"Service","metadata":{"name":"my-svc","namespace":"test-ns"}}`),
		},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(svc, backupSecret).
		Build()

	injector := NewResourceDeletionInjector(fakeClient)

	spec := v1alpha1.InjectionSpec{
		Type:        v1alpha1.ResourceDeletion,
		DangerLevel: v1alpha1.DangerLevelHigh,
		Parameters: map[string]string{
			"apiVersion": "v1",
			"kind":       "Service",
			"name":       "my-svc",
		},
	}

	ctx := context.Background()
	err := injector.Revert(ctx, spec, "test-ns")
	require.NoError(t, err)

	// Backup Secret should be cleaned up
	var s corev1.Secret
	err = fakeClient.Get(ctx, types.NamespacedName{Name: "chaos-backup-resource-service-my-svc", Namespace: "test-ns"}, &s)
	require.Error(t, err)

	// Service should still exist
	var svcCheck corev1.Service
	err = fakeClient.Get(ctx, types.NamespacedName{Name: "my-svc", Namespace: "test-ns"}, &svcCheck)
	require.NoError(t, err)
}

func TestResourceDeletionFullLifecycle(t *testing.T) {
	scheme := newResourceDeletionTestScheme(t)

	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "lifecycle-svc",
			Namespace: "test-ns",
			Labels:    map[string]string{"app": "test"},
		},
		Spec: corev1.ServiceSpec{
			Ports: []corev1.ServicePort{
				{Port: 8080, Protocol: corev1.ProtocolTCP},
			},
		},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(svc).
		Build()

	injector := NewResourceDeletionInjector(fakeClient)

	spec := v1alpha1.InjectionSpec{
		Type:        v1alpha1.ResourceDeletion,
		DangerLevel: v1alpha1.DangerLevelHigh,
		Parameters: map[string]string{
			"apiVersion": "v1",
			"kind":       "Service",
			"name":       "lifecycle-svc",
		},
	}

	ctx := context.Background()

	// Inject
	cleanup, events, err := injector.Inject(ctx, spec, "test-ns")
	require.NoError(t, err)
	assert.Len(t, events, 1)

	// Verify deleted
	var deleted corev1.Service
	err = fakeClient.Get(ctx, types.NamespacedName{Name: "lifecycle-svc", Namespace: "test-ns"}, &deleted)
	require.Error(t, err)

	// Cleanup
	require.NoError(t, cleanup(ctx))

	// Verify restored
	var restored corev1.Service
	err = fakeClient.Get(ctx, types.NamespacedName{Name: "lifecycle-svc", Namespace: "test-ns"}, &restored)
	require.NoError(t, err)

	// Verify backup is gone
	var backupSecret corev1.Secret
	err = fakeClient.Get(ctx, types.NamespacedName{Name: "chaos-backup-resource-service-lifecycle-svc", Namespace: "test-ns"}, &backupSecret)
	require.Error(t, err)
}

func TestResourceDeletionBackupCollisionRefusal(t *testing.T) {
	scheme := newResourceDeletionTestScheme(t)

	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-svc",
			Namespace: "test-ns",
		},
		Spec: corev1.ServiceSpec{
			Ports: []corev1.ServicePort{
				{Port: 80, Protocol: corev1.ProtocolTCP},
			},
		},
	}

	// Non-chaos-managed Secret with the backup name
	existingSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "chaos-backup-resource-service-my-svc",
			Namespace: "test-ns",
			Labels: map[string]string{
				"app.kubernetes.io/managed-by": "someone-else",
			},
		},
		Data: map[string][]byte{
			"some-data": []byte("important"),
		},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(svc, existingSecret).
		Build()

	injector := NewResourceDeletionInjector(fakeClient)

	spec := v1alpha1.InjectionSpec{
		Type:        v1alpha1.ResourceDeletion,
		DangerLevel: v1alpha1.DangerLevelHigh,
		Parameters: map[string]string{
			"apiVersion": "v1",
			"kind":       "Service",
			"name":       "my-svc",
		},
	}

	ctx := context.Background()
	_, _, err := injector.Inject(ctx, spec, "test-ns")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not chaos-managed")
}

func TestResourceDeletionImplementsInjector(t *testing.T) {
	scheme := newResourceDeletionTestScheme(t)
	fakeClient := fake.NewClientBuilder().WithScheme(scheme).Build()

	var _ Injector = NewResourceDeletionInjector(fakeClient)
}

func TestResourceDeletionTypeIsValid(t *testing.T) {
	err := v1alpha1.ValidateInjectionType(v1alpha1.ResourceDeletion)
	assert.NoError(t, err)
}

func TestResourceDeletionBackupSecretName(t *testing.T) {
	assert.Equal(t, "chaos-backup-resource-service-my-svc", resourceDeletionBackupSecretName("Service", "my-svc"))
	assert.Equal(t, "chaos-backup-resource-configmap-my-cm", resourceDeletionBackupSecretName("ConfigMap", "my-cm"))
}

func TestResourceDeletionRevertIdempotent(t *testing.T) {
	scheme := newResourceDeletionTestScheme(t)

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		Build()

	injector := NewResourceDeletionInjector(fakeClient)

	spec := v1alpha1.InjectionSpec{
		Type:        v1alpha1.ResourceDeletion,
		DangerLevel: v1alpha1.DangerLevelHigh,
		Parameters: map[string]string{
			"apiVersion": "v1",
			"kind":       "Service",
			"name":       "nonexistent-svc",
		},
	}

	// Revert with no backup Secret should be a no-op
	err := injector.Revert(context.Background(), spec, "test-ns")
	assert.NoError(t, err)
}

func TestResourceDeletionValidateAcceptsValid(t *testing.T) {
	injector := &ResourceDeletionInjector{}

	spec := v1alpha1.InjectionSpec{
		Type:        v1alpha1.ResourceDeletion,
		DangerLevel: v1alpha1.DangerLevelHigh,
		Parameters: map[string]string{
			"apiVersion": "v1",
			"kind":       "Service",
			"name":       "my-svc",
		},
	}
	blast := v1alpha1.BlastRadiusSpec{MaxPodsAffected: 1}

	err := injector.Validate(spec, blast)
	assert.NoError(t, err)
}

func TestResourceDeletionValidateBackupNameLength(t *testing.T) {
	injector := &ResourceDeletionInjector{}

	// Create a name that would make the backup name exceed 253 chars
	longName := ""
	for i := 0; i < 240; i++ {
		longName += "a"
	}

	spec := v1alpha1.InjectionSpec{
		Type:        v1alpha1.ResourceDeletion,
		DangerLevel: v1alpha1.DangerLevelHigh,
		Parameters: map[string]string{
			"apiVersion": "v1",
			"kind":       "Service",
			"name":       longName,
		},
	}
	blast := v1alpha1.BlastRadiusSpec{MaxPodsAffected: 1}

	err := injector.Validate(spec, blast)
	require.Error(t, err)
	// Either the name validation or the backup name length check catches it
	assert.Error(t, err)
}

func TestResourceDeletionBackupSecretHasChaosLabels(t *testing.T) {
	scheme := newResourceDeletionTestScheme(t)

	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "labeled-svc",
			Namespace: "test-ns",
		},
		Spec: corev1.ServiceSpec{
			Ports: []corev1.ServicePort{
				{Port: 80, Protocol: corev1.ProtocolTCP},
			},
		},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(svc).
		Build()

	injector := NewResourceDeletionInjector(fakeClient)

	spec := v1alpha1.InjectionSpec{
		Type:        v1alpha1.ResourceDeletion,
		DangerLevel: v1alpha1.DangerLevelHigh,
		Parameters: map[string]string{
			"apiVersion": "v1",
			"kind":       "Service",
			"name":       "labeled-svc",
		},
	}

	ctx := context.Background()
	_, _, err := injector.Inject(ctx, spec, "test-ns")
	require.NoError(t, err)

	// Verify backup Secret has chaos labels
	var backupSecret corev1.Secret
	err = fakeClient.Get(ctx, types.NamespacedName{Name: "chaos-backup-resource-service-labeled-svc", Namespace: "test-ns"}, &backupSecret)
	require.NoError(t, err)
	assert.Equal(t, safety.ManagedByValue, backupSecret.Labels[safety.ManagedByLabel])
	assert.Equal(t, string(v1alpha1.ResourceDeletion), backupSecret.Labels[safety.ChaosTypeLabel])
}
