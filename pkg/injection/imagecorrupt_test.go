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

func newImageCorruptScheme(t *testing.T) *runtime.Scheme {
	t.Helper()
	scheme := runtime.NewScheme()
	require.NoError(t, appsv1.AddToScheme(scheme))
	return scheme
}

func newDeploymentWithContainers(name, namespace string, containers []corev1.Container) *appsv1.Deployment {
	replicas := int32(1)
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
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"app": name},
				},
				Spec: corev1.PodSpec{
					Containers: containers,
				},
			},
		},
	}
}

func TestImageCorruptValidateRejectsMissingName(t *testing.T) {
	injector := &ImageCorruptInjector{}

	spec := v1alpha1.InjectionSpec{
		Type:        v1alpha1.ImageCorrupt,
		DangerLevel: v1alpha1.DangerLevelHigh,
		Parameters:  map[string]string{},
	}
	blast := v1alpha1.BlastRadiusSpec{MaxPodsAffected: 1}

	err := injector.Validate(spec, blast)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "name")
}

func TestImageCorruptValidateRejectsMissingDangerLevel(t *testing.T) {
	injector := &ImageCorruptInjector{}

	spec := v1alpha1.InjectionSpec{
		Type: v1alpha1.ImageCorrupt,
		Parameters: map[string]string{
			"name": "my-deployment",
		},
	}
	blast := v1alpha1.BlastRadiusSpec{MaxPodsAffected: 1}

	err := injector.Validate(spec, blast)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "dangerLevel: high")
}

func TestImageCorruptValidateRejectsChaosManaged(t *testing.T) {
	injector := &ImageCorruptInjector{}

	spec := v1alpha1.InjectionSpec{
		Type:        v1alpha1.ImageCorrupt,
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

func TestImageCorruptValidateAcceptsValid(t *testing.T) {
	injector := &ImageCorruptInjector{}

	spec := v1alpha1.InjectionSpec{
		Type:        v1alpha1.ImageCorrupt,
		DangerLevel: v1alpha1.DangerLevelHigh,
		Parameters: map[string]string{
			"name": "my-deployment",
		},
	}
	blast := v1alpha1.BlastRadiusSpec{MaxPodsAffected: 1}

	err := injector.Validate(spec, blast)
	assert.NoError(t, err)
}

func TestImageCorruptInjectPatchesImageAndSetsAnnotation(t *testing.T) {
	scheme := newImageCorruptScheme(t)
	deploy := newDeploymentWithContainers("my-deploy", "test-ns", []corev1.Container{
		{Name: "main", Image: "quay.io/org/app:v1.0"},
	})

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(deploy).
		Build()

	injector := NewImageCorruptInjector(fakeClient)

	spec := v1alpha1.InjectionSpec{
		Type: v1alpha1.ImageCorrupt,
		Parameters: map[string]string{
			"name": "my-deploy",
		},
	}

	ctx := context.Background()
	cleanup, events, err := injector.Inject(ctx, spec, "test-ns")
	require.NoError(t, err)
	assert.Len(t, events, 1)
	assert.Equal(t, v1alpha1.ImageCorrupt, events[0].Type)
	assert.Equal(t, "image-corrupted", events[0].Action)
	assert.Equal(t, "quay.io/org/app:v1.0", events[0].Details["originalImage"])
	assert.Equal(t, defaultCorruptImage, events[0].Details["corruptImage"])
	assert.NotNil(t, cleanup)

	// Verify deployment image is corrupted
	var updated appsv1.Deployment
	require.NoError(t, fakeClient.Get(ctx, types.NamespacedName{Name: "my-deploy", Namespace: "test-ns"}, &updated))
	assert.Equal(t, defaultCorruptImage, updated.Spec.Template.Spec.Containers[0].Image)
	assert.Equal(t, "quay.io/org/app:v1.0", updated.Annotations[originalImageAnnotationPrefix+"main"])
	assert.Equal(t, "main", updated.Annotations[corruptContainerAnnotation])
}

func TestImageCorruptRestoreRestoresOriginalImage(t *testing.T) {
	scheme := newImageCorruptScheme(t)
	deploy := newDeploymentWithContainers("my-deploy", "test-ns", []corev1.Container{
		{Name: "main", Image: "quay.io/org/app:v1.0"},
	})

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(deploy).
		Build()

	injector := NewImageCorruptInjector(fakeClient)

	spec := v1alpha1.InjectionSpec{
		Type: v1alpha1.ImageCorrupt,
		Parameters: map[string]string{
			"name": "my-deploy",
		},
	}

	ctx := context.Background()
	cleanup, _, err := injector.Inject(ctx, spec, "test-ns")
	require.NoError(t, err)

	// Run cleanup
	require.NoError(t, cleanup(ctx))

	// Verify image restored
	var restored appsv1.Deployment
	require.NoError(t, fakeClient.Get(ctx, types.NamespacedName{Name: "my-deploy", Namespace: "test-ns"}, &restored))
	assert.Equal(t, "quay.io/org/app:v1.0", restored.Spec.Template.Spec.Containers[0].Image)
	_, hasAnnotation := restored.Annotations[originalImageAnnotationPrefix+"main"]
	assert.False(t, hasAnnotation, "image annotation should be removed after cleanup")
	_, hasContainerAnnotation := restored.Annotations[corruptContainerAnnotation]
	assert.False(t, hasContainerAnnotation, "container annotation should be removed after cleanup")
}

func TestImageCorruptCustomImage(t *testing.T) {
	scheme := newImageCorruptScheme(t)
	deploy := newDeploymentWithContainers("my-deploy", "test-ns", []corev1.Container{
		{Name: "main", Image: "quay.io/org/app:v1.0"},
	})

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(deploy).
		Build()

	injector := NewImageCorruptInjector(fakeClient)

	customImage := "gcr.io/fake/broken:latest"
	spec := v1alpha1.InjectionSpec{
		Type: v1alpha1.ImageCorrupt,
		Parameters: map[string]string{
			"name":  "my-deploy",
			"image": customImage,
		},
	}

	ctx := context.Background()
	_, events, err := injector.Inject(ctx, spec, "test-ns")
	require.NoError(t, err)
	assert.Equal(t, customImage, events[0].Details["corruptImage"])

	var updated appsv1.Deployment
	require.NoError(t, fakeClient.Get(ctx, types.NamespacedName{Name: "my-deploy", Namespace: "test-ns"}, &updated))
	assert.Equal(t, customImage, updated.Spec.Template.Spec.Containers[0].Image)
}

func TestImageCorruptTargetSpecificContainer(t *testing.T) {
	scheme := newImageCorruptScheme(t)
	deploy := newDeploymentWithContainers("my-deploy", "test-ns", []corev1.Container{
		{Name: "sidecar", Image: "envoyproxy/envoy:v1.20"},
		{Name: "main", Image: "quay.io/org/app:v1.0"},
	})

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(deploy).
		Build()

	injector := NewImageCorruptInjector(fakeClient)

	spec := v1alpha1.InjectionSpec{
		Type: v1alpha1.ImageCorrupt,
		Parameters: map[string]string{
			"name":          "my-deploy",
			"containerName": "main",
		},
	}

	ctx := context.Background()
	_, events, err := injector.Inject(ctx, spec, "test-ns")
	require.NoError(t, err)
	assert.Equal(t, "main", events[0].Details["container"])
	assert.Equal(t, "quay.io/org/app:v1.0", events[0].Details["originalImage"])

	var updated appsv1.Deployment
	require.NoError(t, fakeClient.Get(ctx, types.NamespacedName{Name: "my-deploy", Namespace: "test-ns"}, &updated))
	// Sidecar should be untouched
	assert.Equal(t, "envoyproxy/envoy:v1.20", updated.Spec.Template.Spec.Containers[0].Image)
	// Main should be corrupted
	assert.Equal(t, defaultCorruptImage, updated.Spec.Template.Spec.Containers[1].Image)
}

func TestImageCorruptInvalidContainerName(t *testing.T) {
	scheme := newImageCorruptScheme(t)
	deploy := newDeploymentWithContainers("my-deploy", "test-ns", []corev1.Container{
		{Name: "main", Image: "quay.io/org/app:v1.0"},
	})

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(deploy).
		Build()

	injector := NewImageCorruptInjector(fakeClient)

	spec := v1alpha1.InjectionSpec{
		Type: v1alpha1.ImageCorrupt,
		Parameters: map[string]string{
			"name":          "my-deploy",
			"containerName": "nonexistent",
		},
	}

	_, _, err := injector.Inject(context.Background(), spec, "test-ns")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "container \"nonexistent\" not found")
}

func TestImageCorruptDeploymentNotFound(t *testing.T) {
	scheme := newImageCorruptScheme(t)

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		Build()

	injector := NewImageCorruptInjector(fakeClient)

	spec := v1alpha1.InjectionSpec{
		Type: v1alpha1.ImageCorrupt,
		Parameters: map[string]string{
			"name": "nonexistent",
		},
	}

	_, _, err := injector.Inject(context.Background(), spec, "test-ns")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "getting Deployment")
}

func TestImageCorruptRevertIdempotent(t *testing.T) {
	scheme := newImageCorruptScheme(t)

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		Build()

	injector := NewImageCorruptInjector(fakeClient)

	spec := v1alpha1.InjectionSpec{
		Type: v1alpha1.ImageCorrupt,
		Parameters: map[string]string{
			"name": "nonexistent",
		},
	}

	// Revert with no deployment should be a no-op
	err := injector.Revert(context.Background(), spec, "test-ns")
	assert.NoError(t, err)
}

func TestImageCorruptRevertRestoresImageAndRemovesAnnotation(t *testing.T) {
	scheme := newImageCorruptScheme(t)
	deploy := newDeploymentWithContainers("my-deploy", "test-ns", []corev1.Container{
		{Name: "main", Image: "quay.io/org/app:v1.0"},
	})

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(deploy).
		Build()

	injector := NewImageCorruptInjector(fakeClient)

	spec := v1alpha1.InjectionSpec{
		Type: v1alpha1.ImageCorrupt,
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
	assert.Equal(t, "quay.io/org/app:v1.0", restored.Spec.Template.Spec.Containers[0].Image)
	_, hasAnnotation := restored.Annotations[originalImageAnnotationPrefix+"main"]
	assert.False(t, hasAnnotation)
	_, hasContainerAnnotation := restored.Annotations[corruptContainerAnnotation]
	assert.False(t, hasContainerAnnotation)
}

func TestImageCorruptImplementsInjector(t *testing.T) {
	scheme := newImageCorruptScheme(t)
	fakeClient := fake.NewClientBuilder().WithScheme(scheme).Build()
	var _ Injector = NewImageCorruptInjector(fakeClient)
}

func TestImageCorruptTypeIsValid(t *testing.T) {
	err := v1alpha1.ValidateInjectionType(v1alpha1.ImageCorrupt)
	assert.NoError(t, err)
}
