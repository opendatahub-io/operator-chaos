package injection

import (
	"context"
	"testing"

	v1alpha1 "github.com/opendatahub-io/operator-chaos/api/v1alpha1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func newScaleZeroScheme(t *testing.T) *runtime.Scheme {
	t.Helper()
	scheme := runtime.NewScheme()
	require.NoError(t, appsv1.AddToScheme(scheme))
	return scheme
}

func newDeployment(name, namespace string, replicas int32) *appsv1.Deployment {
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": name},
			},
			Template: corev1PodTemplate(name),
		},
	}
}

func corev1PodTemplate(name string) corev1.PodTemplateSpec {
	return corev1.PodTemplateSpec{
		ObjectMeta: metav1.ObjectMeta{
			Labels: map[string]string{"app": name},
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{Name: name, Image: "busybox"},
			},
		},
	}
}

func TestScaleZeroValidateRejectsMissingDangerLevel(t *testing.T) {
	injector := &ScaleZeroInjector{}

	spec := v1alpha1.InjectionSpec{
		Type: v1alpha1.DeploymentScaleZero,
		Parameters: map[string]string{
			"name": "my-deployment",
		},
	}
	blast := v1alpha1.BlastRadiusSpec{MaxPodsAffected: 1}

	err := injector.Validate(spec, blast)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "dangerLevel: high")
}

func TestScaleZeroValidateRejectsMissingName(t *testing.T) {
	injector := &ScaleZeroInjector{}

	spec := v1alpha1.InjectionSpec{
		Type:        v1alpha1.DeploymentScaleZero,
		DangerLevel: v1alpha1.DangerLevelHigh,
		Parameters:  map[string]string{},
	}
	blast := v1alpha1.BlastRadiusSpec{MaxPodsAffected: 1}

	err := injector.Validate(spec, blast)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "name")
}

func TestScaleZeroValidateRejectsChaosManagedResource(t *testing.T) {
	injector := &ScaleZeroInjector{}

	spec := v1alpha1.InjectionSpec{
		Type:        v1alpha1.DeploymentScaleZero,
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

func TestScaleZeroValidateAcceptsValid(t *testing.T) {
	injector := &ScaleZeroInjector{}

	spec := v1alpha1.InjectionSpec{
		Type:        v1alpha1.DeploymentScaleZero,
		DangerLevel: v1alpha1.DangerLevelHigh,
		Parameters: map[string]string{
			"name": "my-deployment",
		},
	}
	blast := v1alpha1.BlastRadiusSpec{MaxPodsAffected: 1}

	err := injector.Validate(spec, blast)
	assert.NoError(t, err)
}

func TestScaleZeroInjectScalesToZeroAndSetsAnnotation(t *testing.T) {
	scheme := newScaleZeroScheme(t)
	deploy := newDeployment("my-deploy", "test-ns", 1)

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(deploy).
		Build()

	injector := NewScaleZeroInjector(fakeClient)

	spec := v1alpha1.InjectionSpec{
		Type: v1alpha1.DeploymentScaleZero,
		Parameters: map[string]string{
			"name": "my-deploy",
		},
	}

	ctx := context.Background()
	cleanup, events, err := injector.Inject(ctx, spec, "test-ns")
	require.NoError(t, err)
	assert.Len(t, events, 1)
	assert.Equal(t, v1alpha1.DeploymentScaleZero, events[0].Type)
	assert.Equal(t, "scaled-to-zero", events[0].Action)
	assert.NotNil(t, cleanup)

	// Verify deployment is scaled to 0
	var updated appsv1.Deployment
	require.NoError(t, fakeClient.Get(ctx, types.NamespacedName{Name: "my-deploy", Namespace: "test-ns"}, &updated))
	assert.Equal(t, int32(0), *updated.Spec.Replicas)
	assert.Equal(t, "1", updated.Annotations[originalReplicasAnnotation])
}

func TestScaleZeroCleanupRestoresReplicas(t *testing.T) {
	scheme := newScaleZeroScheme(t)
	deploy := newDeployment("my-deploy", "test-ns", 1)

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(deploy).
		Build()

	injector := NewScaleZeroInjector(fakeClient)

	spec := v1alpha1.InjectionSpec{
		Type: v1alpha1.DeploymentScaleZero,
		Parameters: map[string]string{
			"name": "my-deploy",
		},
	}

	ctx := context.Background()
	cleanup, _, err := injector.Inject(ctx, spec, "test-ns")
	require.NoError(t, err)

	// Run cleanup
	require.NoError(t, cleanup(ctx))

	// Verify replicas restored
	var restored appsv1.Deployment
	require.NoError(t, fakeClient.Get(ctx, types.NamespacedName{Name: "my-deploy", Namespace: "test-ns"}, &restored))
	assert.Equal(t, int32(1), *restored.Spec.Replicas)
	_, hasAnnotation := restored.Annotations[originalReplicasAnnotation]
	assert.False(t, hasAnnotation, "annotation should be removed after cleanup")
}

func TestScaleZeroRevertRestoresReplicasAndRemovesAnnotation(t *testing.T) {
	scheme := newScaleZeroScheme(t)
	deploy := newDeployment("my-deploy", "test-ns", 3)

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(deploy).
		Build()

	injector := NewScaleZeroInjector(fakeClient)

	spec := v1alpha1.InjectionSpec{
		Type: v1alpha1.DeploymentScaleZero,
		Parameters: map[string]string{
			"name": "my-deploy",
		},
	}

	ctx := context.Background()
	_, _, err := injector.Inject(ctx, spec, "test-ns")
	require.NoError(t, err)

	// Revert should restore
	require.NoError(t, injector.Revert(ctx, spec, "test-ns"))

	var restored appsv1.Deployment
	require.NoError(t, fakeClient.Get(ctx, types.NamespacedName{Name: "my-deploy", Namespace: "test-ns"}, &restored))
	assert.Equal(t, int32(3), *restored.Spec.Replicas)
	_, hasAnnotation := restored.Annotations[originalReplicasAnnotation]
	assert.False(t, hasAnnotation)
}

func TestScaleZeroInjectDeploymentNotFound(t *testing.T) {
	scheme := newScaleZeroScheme(t)

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		Build()

	injector := NewScaleZeroInjector(fakeClient)

	spec := v1alpha1.InjectionSpec{
		Type: v1alpha1.DeploymentScaleZero,
		Parameters: map[string]string{
			"name": "nonexistent",
		},
	}

	_, _, err := injector.Inject(context.Background(), spec, "test-ns")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "getting Deployment")
}

func TestScaleZeroRevertIdempotent(t *testing.T) {
	scheme := newScaleZeroScheme(t)

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		Build()

	injector := NewScaleZeroInjector(fakeClient)

	spec := v1alpha1.InjectionSpec{
		Type: v1alpha1.DeploymentScaleZero,
		Parameters: map[string]string{
			"name": "nonexistent",
		},
	}

	// Revert with no deployment should be a no-op
	err := injector.Revert(context.Background(), spec, "test-ns")
	assert.NoError(t, err)
}

func TestScaleZeroWithMultipleReplicas(t *testing.T) {
	scheme := newScaleZeroScheme(t)
	deploy := newDeployment("ha-deploy", "test-ns", 5)

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(deploy).
		Build()

	injector := NewScaleZeroInjector(fakeClient)

	spec := v1alpha1.InjectionSpec{
		Type: v1alpha1.DeploymentScaleZero,
		Parameters: map[string]string{
			"name": "ha-deploy",
		},
	}

	ctx := context.Background()
	cleanup, events, err := injector.Inject(ctx, spec, "test-ns")
	require.NoError(t, err)
	assert.Equal(t, "5", events[0].Details["originalReplicas"])

	// Verify scaled to zero
	var updated appsv1.Deployment
	require.NoError(t, fakeClient.Get(ctx, types.NamespacedName{Name: "ha-deploy", Namespace: "test-ns"}, &updated))
	assert.Equal(t, int32(0), *updated.Spec.Replicas)
	assert.Equal(t, "5", updated.Annotations[originalReplicasAnnotation])

	// Cleanup should restore to 5
	require.NoError(t, cleanup(ctx))

	var restored appsv1.Deployment
	require.NoError(t, fakeClient.Get(ctx, types.NamespacedName{Name: "ha-deploy", Namespace: "test-ns"}, &restored))
	assert.Equal(t, int32(5), *restored.Spec.Replicas)
}

func TestScaleZeroImplementsInjector(t *testing.T) {
	scheme := newScaleZeroScheme(t)
	fakeClient := fake.NewClientBuilder().WithScheme(scheme).Build()
	var _ Injector = NewScaleZeroInjector(fakeClient)
}

func TestScaleZeroTypeIsValid(t *testing.T) {
	err := v1alpha1.ValidateInjectionType(v1alpha1.DeploymentScaleZero)
	assert.NoError(t, err)
}
