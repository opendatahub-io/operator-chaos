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
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func newDeploymentWithCommand(name, namespace string, command []string) *appsv1.Deployment {
	d := newDeployment(name, namespace, 1)
	d.Spec.Template.Spec.Containers[0].Command = command
	return d
}

func newMultiContainerDeployment(name, namespace string) *appsv1.Deployment {
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
					Containers: []corev1.Container{
						{Name: "main", Image: "main:latest", Command: []string{"/main-binary"}},
						{Name: "sidecar", Image: "sidecar:latest", Command: []string{"/sidecar-binary", "--flag"}},
					},
				},
			},
		},
	}
}

func TestCrashLoopValidateRejectsMissingName(t *testing.T) {
	injector := &CrashLoopInjector{}

	spec := v1alpha1.InjectionSpec{
		Type:        v1alpha1.CrashLoopInject,
		DangerLevel: v1alpha1.DangerLevelHigh,
		Parameters:  map[string]string{},
	}
	blast := v1alpha1.BlastRadiusSpec{MaxPodsAffected: 1}

	err := injector.Validate(spec, blast)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "name")
}

func TestCrashLoopValidateRejectsMissingDangerLevel(t *testing.T) {
	injector := &CrashLoopInjector{}

	spec := v1alpha1.InjectionSpec{
		Type: v1alpha1.CrashLoopInject,
		Parameters: map[string]string{
			"name": "my-deployment",
		},
	}
	blast := v1alpha1.BlastRadiusSpec{MaxPodsAffected: 1}

	err := injector.Validate(spec, blast)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "dangerLevel: high")
}

func TestCrashLoopValidateRejectsNonHighDangerLevel(t *testing.T) {
	injector := &CrashLoopInjector{}

	spec := v1alpha1.InjectionSpec{
		Type:        v1alpha1.CrashLoopInject,
		DangerLevel: v1alpha1.DangerLevelMedium,
		Parameters: map[string]string{
			"name": "my-deployment",
		},
	}
	blast := v1alpha1.BlastRadiusSpec{MaxPodsAffected: 1}

	err := injector.Validate(spec, blast)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "dangerLevel: high")
}

func TestCrashLoopValidateAcceptsValid(t *testing.T) {
	injector := &CrashLoopInjector{}

	spec := v1alpha1.InjectionSpec{
		Type:        v1alpha1.CrashLoopInject,
		DangerLevel: v1alpha1.DangerLevelHigh,
		Parameters: map[string]string{
			"name": "my-deployment",
		},
	}
	blast := v1alpha1.BlastRadiusSpec{MaxPodsAffected: 1}

	err := injector.Validate(spec, blast)
	assert.NoError(t, err)
}

func TestCrashLoopValidateAcceptsValidWithContainerName(t *testing.T) {
	injector := &CrashLoopInjector{}

	spec := v1alpha1.InjectionSpec{
		Type:        v1alpha1.CrashLoopInject,
		DangerLevel: v1alpha1.DangerLevelHigh,
		Parameters: map[string]string{
			"name":          "my-deployment",
			"containerName": "my-container",
		},
	}
	blast := v1alpha1.BlastRadiusSpec{MaxPodsAffected: 1}

	err := injector.Validate(spec, blast)
	assert.NoError(t, err)
}

func TestCrashLoopValidateRejectsChaosManaged(t *testing.T) {
	injector := &CrashLoopInjector{}

	spec := v1alpha1.InjectionSpec{
		Type:        v1alpha1.CrashLoopInject,
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

func TestCrashLoopInjectPatchesCommandAndSetsAnnotation(t *testing.T) {
	scheme := newScaleZeroScheme(t)
	deploy := newDeploymentWithCommand("my-deploy", "test-ns", []string{"/original-binary", "--flag"})

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(deploy).
		Build()

	injector := NewCrashLoopInjector(fakeClient)

	spec := v1alpha1.InjectionSpec{
		Type: v1alpha1.CrashLoopInject,
		Parameters: map[string]string{
			"name": "my-deploy",
		},
	}

	ctx := context.Background()
	cleanup, events, err := injector.Inject(ctx, spec, "test-ns")
	require.NoError(t, err)
	assert.Len(t, events, 1)
	assert.Equal(t, v1alpha1.CrashLoopInject, events[0].Type)
	assert.Equal(t, "command-replaced", events[0].Action)
	assert.NotNil(t, cleanup)

	// Verify deployment command is patched
	var updated appsv1.Deployment
	require.NoError(t, fakeClient.Get(ctx, types.NamespacedName{Name: "my-deploy", Namespace: "test-ns"}, &updated))
	assert.Equal(t, []string{"/chaos-nonexistent-binary"}, updated.Spec.Template.Spec.Containers[0].Command)
	assert.Equal(t, `["/original-binary","--flag"]`, updated.Annotations[originalCommandAnnotationPrefix+"my-deploy"])
	assert.Equal(t, "my-deploy", updated.Annotations[crashContainerAnnotation])
}

func TestCrashLoopInjectNilCommandStoresNull(t *testing.T) {
	scheme := newScaleZeroScheme(t)
	// Create a deployment with no command (ENTRYPOINT-based)
	deploy := newDeploymentWithCommand("entrypoint-deploy", "test-ns", nil)

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(deploy).
		Build()

	injector := NewCrashLoopInjector(fakeClient)

	spec := v1alpha1.InjectionSpec{
		Type: v1alpha1.CrashLoopInject,
		Parameters: map[string]string{
			"name": "entrypoint-deploy",
		},
	}

	ctx := context.Background()
	_, events, err := injector.Inject(ctx, spec, "test-ns")
	require.NoError(t, err)
	assert.Equal(t, "null", events[0].Details["originalCommand"])

	var updated appsv1.Deployment
	require.NoError(t, fakeClient.Get(ctx, types.NamespacedName{Name: "entrypoint-deploy", Namespace: "test-ns"}, &updated))
	assert.Equal(t, []string{"/chaos-nonexistent-binary"}, updated.Spec.Template.Spec.Containers[0].Command)
	assert.Equal(t, "null", updated.Annotations[originalCommandAnnotationPrefix+"entrypoint-deploy"])
	assert.Equal(t, "entrypoint-deploy", updated.Annotations[crashContainerAnnotation])
}

func TestCrashLoopRestoreOriginalCommand(t *testing.T) {
	scheme := newScaleZeroScheme(t)
	deploy := newDeploymentWithCommand("my-deploy", "test-ns", []string{"/original-binary", "--flag"})

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(deploy).
		Build()

	injector := NewCrashLoopInjector(fakeClient)

	spec := v1alpha1.InjectionSpec{
		Type: v1alpha1.CrashLoopInject,
		Parameters: map[string]string{
			"name": "my-deploy",
		},
	}

	ctx := context.Background()
	cleanup, _, err := injector.Inject(ctx, spec, "test-ns")
	require.NoError(t, err)

	// Run cleanup
	require.NoError(t, cleanup(ctx))

	// Verify command restored
	var restored appsv1.Deployment
	require.NoError(t, fakeClient.Get(ctx, types.NamespacedName{Name: "my-deploy", Namespace: "test-ns"}, &restored))
	assert.Equal(t, []string{"/original-binary", "--flag"}, restored.Spec.Template.Spec.Containers[0].Command)
	_, hasAnnotation := restored.Annotations[originalCommandAnnotationPrefix+"my-deploy"]
	assert.False(t, hasAnnotation, "command annotation should be removed after cleanup")
	_, hasContainerAnnotation := restored.Annotations[crashContainerAnnotation]
	assert.False(t, hasContainerAnnotation, "container annotation should be removed after cleanup")
}

func TestCrashLoopRestoreNullCommand(t *testing.T) {
	scheme := newScaleZeroScheme(t)
	deploy := newDeploymentWithCommand("entrypoint-deploy", "test-ns", nil)

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(deploy).
		Build()

	injector := NewCrashLoopInjector(fakeClient)

	spec := v1alpha1.InjectionSpec{
		Type: v1alpha1.CrashLoopInject,
		Parameters: map[string]string{
			"name": "entrypoint-deploy",
		},
	}

	ctx := context.Background()
	cleanup, _, err := injector.Inject(ctx, spec, "test-ns")
	require.NoError(t, err)

	// Run cleanup
	require.NoError(t, cleanup(ctx))

	// Verify command is nil (ENTRYPOINT restored)
	var restored appsv1.Deployment
	require.NoError(t, fakeClient.Get(ctx, types.NamespacedName{Name: "entrypoint-deploy", Namespace: "test-ns"}, &restored))
	assert.Nil(t, restored.Spec.Template.Spec.Containers[0].Command, "command should be nil after restoring ENTRYPOINT-based container")
	_, hasAnnotation := restored.Annotations[originalCommandAnnotationPrefix+"entrypoint-deploy"]
	assert.False(t, hasAnnotation, "command annotation should be removed after cleanup")
	_, hasContainerAnnotation := restored.Annotations[crashContainerAnnotation]
	assert.False(t, hasContainerAnnotation, "container annotation should be removed after cleanup")
}

func TestCrashLoopInjectDeploymentNotFound(t *testing.T) {
	scheme := newScaleZeroScheme(t)

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		Build()

	injector := NewCrashLoopInjector(fakeClient)

	spec := v1alpha1.InjectionSpec{
		Type: v1alpha1.CrashLoopInject,
		Parameters: map[string]string{
			"name": "nonexistent",
		},
	}

	_, _, err := injector.Inject(context.Background(), spec, "test-ns")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "getting Deployment")
}

func TestCrashLoopInjectInvalidContainerName(t *testing.T) {
	scheme := newScaleZeroScheme(t)
	deploy := newMultiContainerDeployment("multi-deploy", "test-ns")

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(deploy).
		Build()

	injector := NewCrashLoopInjector(fakeClient)

	spec := v1alpha1.InjectionSpec{
		Type: v1alpha1.CrashLoopInject,
		Parameters: map[string]string{
			"name":          "multi-deploy",
			"containerName": "nonexistent-container",
		},
	}

	_, _, err := injector.Inject(context.Background(), spec, "test-ns")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "container \"nonexistent-container\" not found")
}

func TestCrashLoopInjectTargetsSpecificContainer(t *testing.T) {
	scheme := newScaleZeroScheme(t)
	deploy := newMultiContainerDeployment("multi-deploy", "test-ns")

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(deploy).
		Build()

	injector := NewCrashLoopInjector(fakeClient)

	spec := v1alpha1.InjectionSpec{
		Type: v1alpha1.CrashLoopInject,
		Parameters: map[string]string{
			"name":          "multi-deploy",
			"containerName": "sidecar",
		},
	}

	ctx := context.Background()
	_, events, err := injector.Inject(ctx, spec, "test-ns")
	require.NoError(t, err)
	assert.Equal(t, "sidecar", events[0].Details["container"])

	// Verify sidecar is patched but main is untouched
	var updated appsv1.Deployment
	require.NoError(t, fakeClient.Get(ctx, types.NamespacedName{Name: "multi-deploy", Namespace: "test-ns"}, &updated))
	assert.Equal(t, []string{"/main-binary"}, updated.Spec.Template.Spec.Containers[0].Command, "main container should be untouched")
	assert.Equal(t, []string{"/chaos-nonexistent-binary"}, updated.Spec.Template.Spec.Containers[1].Command, "sidecar should be patched")
}

func TestCrashLoopRevertIdempotent(t *testing.T) {
	scheme := newScaleZeroScheme(t)

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		Build()

	injector := NewCrashLoopInjector(fakeClient)

	spec := v1alpha1.InjectionSpec{
		Type: v1alpha1.CrashLoopInject,
		Parameters: map[string]string{
			"name": "nonexistent",
		},
	}

	// Revert with no deployment should be a no-op
	err := injector.Revert(context.Background(), spec, "test-ns")
	assert.NoError(t, err)
}

func TestCrashLoopRevertRestoresViaRevertMethod(t *testing.T) {
	scheme := newScaleZeroScheme(t)
	deploy := newDeploymentWithCommand("my-deploy", "test-ns", []string{"/original"})

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(deploy).
		Build()

	injector := NewCrashLoopInjector(fakeClient)

	spec := v1alpha1.InjectionSpec{
		Type: v1alpha1.CrashLoopInject,
		Parameters: map[string]string{
			"name": "my-deploy",
		},
	}

	ctx := context.Background()
	_, _, err := injector.Inject(ctx, spec, "test-ns")
	require.NoError(t, err)

	// Use Revert method (controller path)
	require.NoError(t, injector.Revert(ctx, spec, "test-ns"))

	var restored appsv1.Deployment
	require.NoError(t, fakeClient.Get(ctx, types.NamespacedName{Name: "my-deploy", Namespace: "test-ns"}, &restored))
	assert.Equal(t, []string{"/original"}, restored.Spec.Template.Spec.Containers[0].Command)
	_, hasAnnotation := restored.Annotations[originalCommandAnnotationPrefix+"my-deploy"]
	assert.False(t, hasAnnotation)
	_, hasContainerAnnotation := restored.Annotations[crashContainerAnnotation]
	assert.False(t, hasContainerAnnotation)
}

func TestCrashLoopImplementsInjector(t *testing.T) {
	scheme := newScaleZeroScheme(t)
	fakeClient := fake.NewClientBuilder().WithScheme(scheme).Build()
	var _ Injector = NewCrashLoopInjector(fakeClient)
}

func TestCrashLoopTypeIsValid(t *testing.T) {
	err := v1alpha1.ValidateInjectionType(v1alpha1.CrashLoopInject)
	assert.NoError(t, err)
}

func TestCrashLoopInjectDefaultsToFirstContainer(t *testing.T) {
	scheme := newScaleZeroScheme(t)
	deploy := newMultiContainerDeployment("multi-deploy", "test-ns")

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(deploy).
		Build()

	injector := NewCrashLoopInjector(fakeClient)

	spec := v1alpha1.InjectionSpec{
		Type: v1alpha1.CrashLoopInject,
		Parameters: map[string]string{
			"name": "multi-deploy",
			// no containerName: should default to first container
		},
	}

	ctx := context.Background()
	_, events, err := injector.Inject(ctx, spec, "test-ns")
	require.NoError(t, err)
	assert.Equal(t, "main", events[0].Details["container"])

	var updated appsv1.Deployment
	require.NoError(t, fakeClient.Get(ctx, types.NamespacedName{Name: "multi-deploy", Namespace: "test-ns"}, &updated))
	assert.Equal(t, []string{"/chaos-nonexistent-binary"}, updated.Spec.Template.Spec.Containers[0].Command, "first container should be patched")
	assert.Equal(t, []string{"/sidecar-binary", "--flag"}, updated.Spec.Template.Spec.Containers[1].Command, "second container should be untouched")
}
