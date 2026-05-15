package observer

import (
	"context"
	"fmt"
	"testing"

	v1alpha1 "github.com/opendatahub-io/operator-chaos/api/v1alpha1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
)

func TestNewKubernetesObserver(t *testing.T) {
	obs := NewKubernetesObserver(nil)
	assert.NotNil(t, obs)
}

func TestCheckSteadyState_EmptyChecks(t *testing.T) {
	obs := NewKubernetesObserver(nil)
	result, err := obs.CheckSteadyState(context.Background(), nil, "test")
	require.NoError(t, err)
	assert.True(t, result.Passed, "no checks means all passed")
	assert.Equal(t, int32(0), result.ChecksRun)
	assert.Equal(t, int32(0), result.ChecksPassed)
	assert.Empty(t, result.Details)
}

func newDeploymentWithCondition(name, namespace, condType, condStatus string) *unstructured.Unstructured {
	obj := &unstructured.Unstructured{}
	obj.SetGroupVersionKind(schema.GroupVersionKind{Group: "apps", Version: "v1", Kind: "Deployment"})
	obj.SetName(name)
	obj.SetNamespace(namespace)
	obj.Object["status"] = map[string]interface{}{
		"conditions": []interface{}{
			map[string]interface{}{
				"type":   condType,
				"status": condStatus,
			},
		},
	}
	return obj
}

func newDeploymentNoConditions(name, namespace string) *unstructured.Unstructured {
	obj := &unstructured.Unstructured{}
	obj.SetGroupVersionKind(schema.GroupVersionKind{Group: "apps", Version: "v1", Kind: "Deployment"})
	obj.SetName(name)
	obj.SetNamespace(namespace)
	// No status.conditions field at all
	return obj
}

func newConfigMap(name, namespace string) *unstructured.Unstructured {
	obj := &unstructured.Unstructured{}
	obj.SetGroupVersionKind(schema.GroupVersionKind{Group: "", Version: "v1", Kind: "ConfigMap"})
	obj.SetName(name)
	obj.SetNamespace(namespace)
	return obj
}

func buildFakeClient(objects ...runtime.Object) *KubernetesObserver {
	scheme := runtime.NewScheme()
	clientBuilder := fake.NewClientBuilder().WithScheme(scheme)
	for _, o := range objects {
		clientBuilder = clientBuilder.WithRuntimeObjects(o)
	}
	return NewKubernetesObserver(clientBuilder.Build())
}

func TestCheckSteadyState_ConditionTrue_Passed(t *testing.T) {
	deploy := newDeploymentWithCondition("test-deploy", "test-ns", "Available", "True")
	obs := buildFakeClient(deploy)

	checks := []v1alpha1.SteadyStateCheck{
		{
			Type:          v1alpha1.CheckConditionTrue,
			APIVersion:    "apps/v1",
			Kind:          "Deployment",
			Name:          "test-deploy",
			Namespace:     "test-ns",
			ConditionType: "Available",
		},
	}

	result, err := obs.CheckSteadyState(context.Background(), checks, "test-ns")
	require.NoError(t, err)
	assert.True(t, result.Passed)
	assert.Equal(t, int32(1), result.ChecksRun)
	assert.Equal(t, int32(1), result.ChecksPassed)
	require.Len(t, result.Details, 1)
	assert.True(t, result.Details[0].Passed)
	assert.Equal(t, "Available=True", result.Details[0].Value)
	assert.Empty(t, result.Details[0].Error)
}

func TestCheckSteadyState_ConditionTrue_Failed(t *testing.T) {
	deploy := newDeploymentWithCondition("test-deploy", "test-ns", "Available", "False")
	obs := buildFakeClient(deploy)

	checks := []v1alpha1.SteadyStateCheck{
		{
			Type:          v1alpha1.CheckConditionTrue,
			APIVersion:    "apps/v1",
			Kind:          "Deployment",
			Name:          "test-deploy",
			Namespace:     "test-ns",
			ConditionType: "Available",
		},
	}

	result, err := obs.CheckSteadyState(context.Background(), checks, "test-ns")
	require.NoError(t, err)
	assert.False(t, result.Passed)
	assert.Equal(t, int32(1), result.ChecksRun)
	assert.Equal(t, int32(0), result.ChecksPassed)
	require.Len(t, result.Details, 1)
	assert.False(t, result.Details[0].Passed)
	assert.Equal(t, "Available=False", result.Details[0].Value)
}

func TestCheckSteadyState_ConditionTrue_NotFound(t *testing.T) {
	// Resource exists but the requested condition type is not in the conditions list
	deploy := newDeploymentWithCondition("test-deploy", "test-ns", "Progressing", "True")
	obs := buildFakeClient(deploy)

	checks := []v1alpha1.SteadyStateCheck{
		{
			Type:          v1alpha1.CheckConditionTrue,
			APIVersion:    "apps/v1",
			Kind:          "Deployment",
			Name:          "test-deploy",
			Namespace:     "test-ns",
			ConditionType: "Available",
		},
	}

	result, err := obs.CheckSteadyState(context.Background(), checks, "test-ns")
	require.NoError(t, err)
	assert.False(t, result.Passed)
	require.Len(t, result.Details, 1)
	assert.False(t, result.Details[0].Passed)
	assert.Equal(t, "condition Available not found", result.Details[0].Value)
}

func TestCheckSteadyState_ConditionTrue_ResourceNotFound(t *testing.T) {
	// No resources in the cluster
	obs := buildFakeClient()

	checks := []v1alpha1.SteadyStateCheck{
		{
			Type:          v1alpha1.CheckConditionTrue,
			APIVersion:    "apps/v1",
			Kind:          "Deployment",
			Name:          "nonexistent",
			Namespace:     "test-ns",
			ConditionType: "Available",
		},
	}

	result, err := obs.CheckSteadyState(context.Background(), checks, "test-ns")
	require.NoError(t, err)
	assert.False(t, result.Passed)
	require.Len(t, result.Details, 1)
	assert.False(t, result.Details[0].Passed)
	assert.NotEmpty(t, result.Details[0].Error)
	assert.Contains(t, result.Details[0].Error, "getting Deployment/nonexistent")
}

func TestCheckSteadyState_ConditionTrue_NoConditions(t *testing.T) {
	deploy := newDeploymentNoConditions("test-deploy", "test-ns")
	obs := buildFakeClient(deploy)

	checks := []v1alpha1.SteadyStateCheck{
		{
			Type:          v1alpha1.CheckConditionTrue,
			APIVersion:    "apps/v1",
			Kind:          "Deployment",
			Name:          "test-deploy",
			Namespace:     "test-ns",
			ConditionType: "Available",
		},
	}

	result, err := obs.CheckSteadyState(context.Background(), checks, "test-ns")
	require.NoError(t, err)
	assert.False(t, result.Passed)
	require.Len(t, result.Details, 1)
	assert.False(t, result.Details[0].Passed)
	assert.Equal(t, "no conditions found", result.Details[0].Value)
}

func TestCheckSteadyState_ResourceExists_Exists(t *testing.T) {
	cm := newConfigMap("my-config", "test-ns")
	obs := buildFakeClient(cm)

	checks := []v1alpha1.SteadyStateCheck{
		{
			Type:       v1alpha1.CheckResourceExists,
			APIVersion: "v1",
			Kind:       "ConfigMap",
			Name:       "my-config",
			Namespace:  "test-ns",
		},
	}

	result, err := obs.CheckSteadyState(context.Background(), checks, "test-ns")
	require.NoError(t, err)
	assert.True(t, result.Passed)
	assert.Equal(t, int32(1), result.ChecksRun)
	assert.Equal(t, int32(1), result.ChecksPassed)
	require.Len(t, result.Details, 1)
	assert.True(t, result.Details[0].Passed)
}

func TestCheckSteadyState_ResourceExists_NotFound(t *testing.T) {
	obs := buildFakeClient()

	checks := []v1alpha1.SteadyStateCheck{
		{
			Type:       v1alpha1.CheckResourceExists,
			APIVersion: "v1",
			Kind:       "ConfigMap",
			Name:       "nonexistent",
			Namespace:  "test-ns",
		},
	}

	result, err := obs.CheckSteadyState(context.Background(), checks, "test-ns")
	require.NoError(t, err)
	assert.False(t, result.Passed)
	assert.Equal(t, int32(1), result.ChecksRun)
	assert.Equal(t, int32(0), result.ChecksPassed)
	require.Len(t, result.Details, 1)
	assert.False(t, result.Details[0].Passed)
}

func TestCheckSteadyState_UnknownType(t *testing.T) {
	obs := buildFakeClient()

	checks := []v1alpha1.SteadyStateCheck{
		{
			Type:       "invalid",
			APIVersion: "v1",
			Kind:       "ConfigMap",
			Name:       "test",
			Namespace:  "test-ns",
		},
	}

	result, err := obs.CheckSteadyState(context.Background(), checks, "test-ns")
	require.NoError(t, err)
	assert.False(t, result.Passed)
	require.Len(t, result.Details, 1)
	assert.False(t, result.Details[0].Passed)
	assert.Contains(t, result.Details[0].Error, "unknown check type: invalid")
}

func TestCheckSteadyState_MixedChecks(t *testing.T) {
	deploy := newDeploymentWithCondition("test-deploy", "test-ns", "Available", "True")
	cm := newConfigMap("existing-cm", "test-ns")
	obs := buildFakeClient(deploy, cm)

	checks := []v1alpha1.SteadyStateCheck{
		{
			// This passes: condition is True
			Type:          v1alpha1.CheckConditionTrue,
			APIVersion:    "apps/v1",
			Kind:          "Deployment",
			Name:          "test-deploy",
			Namespace:     "test-ns",
			ConditionType: "Available",
		},
		{
			// This passes: resource exists
			Type:       v1alpha1.CheckResourceExists,
			APIVersion: "v1",
			Kind:       "ConfigMap",
			Name:       "existing-cm",
			Namespace:  "test-ns",
		},
		{
			// This fails: resource does not exist
			Type:       v1alpha1.CheckResourceExists,
			APIVersion: "v1",
			Kind:       "ConfigMap",
			Name:       "missing-cm",
			Namespace:  "test-ns",
		},
	}

	result, err := obs.CheckSteadyState(context.Background(), checks, "test-ns")
	require.NoError(t, err)
	assert.False(t, result.Passed, "overall should fail because one check failed")
	assert.Equal(t, int32(3), result.ChecksRun)
	assert.Equal(t, int32(2), result.ChecksPassed)
	require.Len(t, result.Details, 3)
	assert.True(t, result.Details[0].Passed)
	assert.True(t, result.Details[1].Passed)
	assert.False(t, result.Details[2].Passed)
}

func TestCheckSteadyState_NamespaceFallback(t *testing.T) {
	// The check has no Namespace set; expect fallback to the namespace argument
	deploy := newDeploymentWithCondition("test-deploy", "fallback-ns", "Available", "True")
	obs := buildFakeClient(deploy)

	checks := []v1alpha1.SteadyStateCheck{
		{
			Type:          v1alpha1.CheckConditionTrue,
			APIVersion:    "apps/v1",
			Kind:          "Deployment",
			Name:          "test-deploy",
			Namespace:     "", // empty -> should use fallback
			ConditionType: "Available",
		},
	}

	result, err := obs.CheckSteadyState(context.Background(), checks, "fallback-ns")
	require.NoError(t, err)
	assert.True(t, result.Passed, "should find resource using fallback namespace")
	assert.Equal(t, int32(1), result.ChecksPassed)
}

func TestCheckSteadyState_NamespaceFallback_ResourceExists(t *testing.T) {
	cm := newConfigMap("my-cm", "fallback-ns")
	obs := buildFakeClient(cm)

	checks := []v1alpha1.SteadyStateCheck{
		{
			Type:       v1alpha1.CheckResourceExists,
			APIVersion: "v1",
			Kind:       "ConfigMap",
			Name:       "my-cm",
			Namespace:  "", // empty -> should use fallback
		},
	}

	result, err := obs.CheckSteadyState(context.Background(), checks, "fallback-ns")
	require.NoError(t, err)
	assert.True(t, result.Passed, "should find resource using fallback namespace")
}

func TestCheckSteadyState_AllChecksPassed(t *testing.T) {
	deploy := newDeploymentWithCondition("d1", "ns", "Available", "True")
	cm := newConfigMap("cm1", "ns")
	obs := buildFakeClient(deploy, cm)

	checks := []v1alpha1.SteadyStateCheck{
		{
			Type:          v1alpha1.CheckConditionTrue,
			APIVersion:    "apps/v1",
			Kind:          "Deployment",
			Name:          "d1",
			Namespace:     "ns",
			ConditionType: "Available",
		},
		{
			Type:       v1alpha1.CheckResourceExists,
			APIVersion: "v1",
			Kind:       "ConfigMap",
			Name:       "cm1",
			Namespace:  "ns",
		},
	}

	result, err := obs.CheckSteadyState(context.Background(), checks, "ns")
	require.NoError(t, err)
	assert.True(t, result.Passed)
	assert.Equal(t, int32(2), result.ChecksRun)
	assert.Equal(t, int32(2), result.ChecksPassed)
}

func TestCheckSteadyState_PerCheckNamespaceTakesPriority(t *testing.T) {
	// Resource exists in "check-ns" but NOT in "function-ns".
	// The check has Namespace="check-ns", the function parameter is "function-ns".
	// Per-check namespace should take priority, so the check should pass.
	deploy := newDeploymentWithCondition("test-deploy", "check-ns", "Available", "True")
	obs := buildFakeClient(deploy)

	checks := []v1alpha1.SteadyStateCheck{
		{
			Type:          v1alpha1.CheckConditionTrue,
			APIVersion:    "apps/v1",
			Kind:          "Deployment",
			Name:          "test-deploy",
			Namespace:     "check-ns",
			ConditionType: "Available",
		},
	}

	result, err := obs.CheckSteadyState(context.Background(), checks, "function-ns")
	require.NoError(t, err)
	assert.True(t, result.Passed, "per-check namespace should override function parameter")
	assert.Equal(t, int32(1), result.ChecksPassed)
}

func TestCheckSteadyState_PerCheckNamespaceTakesPriority_ResourceExists(t *testing.T) {
	// Same test for resourceExists check type
	cm := newConfigMap("my-cm", "check-ns")
	obs := buildFakeClient(cm)

	checks := []v1alpha1.SteadyStateCheck{
		{
			Type:       v1alpha1.CheckResourceExists,
			APIVersion: "v1",
			Kind:       "ConfigMap",
			Name:       "my-cm",
			Namespace:  "check-ns",
		},
	}

	result, err := obs.CheckSteadyState(context.Background(), checks, "function-ns")
	require.NoError(t, err)
	assert.True(t, result.Passed, "per-check namespace should override function parameter")
}

func newDeploymentWithReplicas(name, namespace string, replicas int64) *unstructured.Unstructured {
	obj := &unstructured.Unstructured{}
	obj.SetGroupVersionKind(schema.GroupVersionKind{Group: "apps", Version: "v1", Kind: "Deployment"})
	obj.SetName(name)
	obj.SetNamespace(namespace)
	obj.Object["spec"] = map[string]interface{}{
		"replicas": replicas,
	}
	return obj
}

func int32Ptr(i int32) *int32 { return &i }

func TestCheckSteadyState_ReplicaCount_Passed(t *testing.T) {
	deploy := newDeploymentWithReplicas("test-deploy", "test-ns", 1)
	obs := buildFakeClient(deploy)

	checks := []v1alpha1.SteadyStateCheck{
		{
			Type:             v1alpha1.CheckReplicaCount,
			APIVersion:       "apps/v1",
			Kind:             "Deployment",
			Name:             "test-deploy",
			Namespace:        "test-ns",
			ExpectedReplicas: int32Ptr(1),
		},
	}

	result, err := obs.CheckSteadyState(context.Background(), checks, "test-ns")
	require.NoError(t, err)
	assert.True(t, result.Passed)
	assert.Equal(t, int32(1), result.ChecksRun)
	assert.Equal(t, int32(1), result.ChecksPassed)
	require.Len(t, result.Details, 1)
	assert.True(t, result.Details[0].Passed)
	assert.Equal(t, "replicas=1", result.Details[0].Value)
}

func TestCheckSteadyState_ReplicaCount_ScaledToZero(t *testing.T) {
	deploy := newDeploymentWithReplicas("test-deploy", "test-ns", 0)
	obs := buildFakeClient(deploy)

	checks := []v1alpha1.SteadyStateCheck{
		{
			Type:             v1alpha1.CheckReplicaCount,
			APIVersion:       "apps/v1",
			Kind:             "Deployment",
			Name:             "test-deploy",
			Namespace:        "test-ns",
			ExpectedReplicas: int32Ptr(1),
		},
	}

	result, err := obs.CheckSteadyState(context.Background(), checks, "test-ns")
	require.NoError(t, err)
	assert.False(t, result.Passed)
	assert.Equal(t, int32(1), result.ChecksRun)
	assert.Equal(t, int32(0), result.ChecksPassed)
	require.Len(t, result.Details, 1)
	assert.False(t, result.Details[0].Passed)
	assert.Equal(t, "replicas=0, expected 1", result.Details[0].Value)
}

func TestCheckSteadyState_ReplicaCount_ResourceNotFound(t *testing.T) {
	obs := buildFakeClient()

	checks := []v1alpha1.SteadyStateCheck{
		{
			Type:             v1alpha1.CheckReplicaCount,
			APIVersion:       "apps/v1",
			Kind:             "Deployment",
			Name:             "nonexistent",
			Namespace:        "test-ns",
			ExpectedReplicas: int32Ptr(1),
		},
	}

	result, err := obs.CheckSteadyState(context.Background(), checks, "test-ns")
	require.NoError(t, err)
	assert.False(t, result.Passed)
	require.Len(t, result.Details, 1)
	assert.False(t, result.Details[0].Passed)
	assert.NotEmpty(t, result.Details[0].Error)
}

func TestCheckSteadyState_ReplicaCount_MissingExpectedReplicas(t *testing.T) {
	deploy := newDeploymentWithReplicas("test-deploy", "test-ns", 1)
	obs := buildFakeClient(deploy)

	checks := []v1alpha1.SteadyStateCheck{
		{
			Type:       v1alpha1.CheckReplicaCount,
			APIVersion: "apps/v1",
			Kind:       "Deployment",
			Name:       "test-deploy",
			Namespace:  "test-ns",
			// ExpectedReplicas not set
		},
	}

	result, err := obs.CheckSteadyState(context.Background(), checks, "test-ns")
	require.NoError(t, err)
	assert.False(t, result.Passed)
	require.Len(t, result.Details, 1)
	assert.False(t, result.Details[0].Passed)
	assert.Contains(t, result.Details[0].Error, "expectedReplicas")
}

func TestCheckSteadyState_ReplicaCount_NamespaceFallback(t *testing.T) {
	deploy := newDeploymentWithReplicas("test-deploy", "fallback-ns", 3)
	obs := buildFakeClient(deploy)

	checks := []v1alpha1.SteadyStateCheck{
		{
			Type:             v1alpha1.CheckReplicaCount,
			APIVersion:       "apps/v1",
			Kind:             "Deployment",
			Name:             "test-deploy",
			Namespace:        "",
			ExpectedReplicas: int32Ptr(3),
		},
	}

	result, err := obs.CheckSteadyState(context.Background(), checks, "fallback-ns")
	require.NoError(t, err)
	assert.True(t, result.Passed)
}

func TestCheckSteadyState_ReplicaCount_NoReplicasFieldDefaultsTo1(t *testing.T) {
	obj := &unstructured.Unstructured{}
	obj.SetGroupVersionKind(schema.GroupVersionKind{Group: "apps", Version: "v1", Kind: "Deployment"})
	obj.SetName("test-deploy")
	obj.SetNamespace("test-ns")
	obj.Object["spec"] = map[string]interface{}{}
	obs := buildFakeClient(obj)

	checks := []v1alpha1.SteadyStateCheck{
		{
			Type:             v1alpha1.CheckReplicaCount,
			APIVersion:       "apps/v1",
			Kind:             "Deployment",
			Name:             "test-deploy",
			Namespace:        "test-ns",
			ExpectedReplicas: int32Ptr(1),
		},
	}

	result, err := obs.CheckSteadyState(context.Background(), checks, "test-ns")
	require.NoError(t, err)
	assert.True(t, result.Passed, "missing replicas field defaults to 1")
}

func TestCheckSteadyState_ReplicaCount_NonScalableKindMissingReplicas(t *testing.T) {
	obj := &unstructured.Unstructured{}
	obj.SetGroupVersionKind(schema.GroupVersionKind{Group: "", Version: "v1", Kind: "ConfigMap"})
	obj.SetName("test-cm")
	obj.SetNamespace("test-ns")
	obj.Object["spec"] = map[string]interface{}{}
	obs := buildFakeClient(obj)

	checks := []v1alpha1.SteadyStateCheck{
		{
			Type:             v1alpha1.CheckReplicaCount,
			APIVersion:       "v1",
			Kind:             "ConfigMap",
			Name:             "test-cm",
			Namespace:        "test-ns",
			ExpectedReplicas: int32Ptr(1),
		},
	}

	result, err := obs.CheckSteadyState(context.Background(), checks, "test-ns")
	require.NoError(t, err)
	assert.False(t, result.Passed)
	require.Len(t, result.Details, 1)
	assert.False(t, result.Details[0].Passed)
	assert.Contains(t, result.Details[0].Error, "spec.replicas is not defined for kind ConfigMap")
}

func TestCheckSteadyState_ReplicaCount_FractionalReplicasRejected(t *testing.T) {
	obj := &unstructured.Unstructured{}
	obj.SetGroupVersionKind(schema.GroupVersionKind{Group: "apps", Version: "v1", Kind: "Deployment"})
	obj.SetName("test-deploy")
	obj.SetNamespace("test-ns")
	obj.Object["spec"] = map[string]interface{}{
		"replicas": float64(1.5),
	}
	obs := buildFakeClient(obj)

	checks := []v1alpha1.SteadyStateCheck{
		{
			Type:             v1alpha1.CheckReplicaCount,
			APIVersion:       "apps/v1",
			Kind:             "Deployment",
			Name:             "test-deploy",
			Namespace:        "test-ns",
			ExpectedReplicas: int32Ptr(1),
		},
	}

	result, err := obs.CheckSteadyState(context.Background(), checks, "test-ns")
	require.NoError(t, err)
	assert.False(t, result.Passed)
	require.Len(t, result.Details, 1)
	assert.False(t, result.Details[0].Passed)
	assert.Contains(t, result.Details[0].Error, "non-integer value")
}

func TestCheckSteadyState_ReplicaCount_NegativeReplicasRejected(t *testing.T) {
	deploy := newDeploymentWithReplicas("test-deploy", "test-ns", -1)
	obs := buildFakeClient(deploy)

	checks := []v1alpha1.SteadyStateCheck{
		{
			Type:             v1alpha1.CheckReplicaCount,
			APIVersion:       "apps/v1",
			Kind:             "Deployment",
			Name:             "test-deploy",
			Namespace:        "test-ns",
			ExpectedReplicas: int32Ptr(1),
		},
	}

	result, err := obs.CheckSteadyState(context.Background(), checks, "test-ns")
	require.NoError(t, err)
	assert.False(t, result.Passed)
	require.Len(t, result.Details, 1)
	assert.False(t, result.Details[0].Passed)
	assert.Contains(t, result.Details[0].Error, "negative")
}

func TestCheckSteadyState_ReplicaCount_ErrorFailsOverallResult(t *testing.T) {
	// A replicaCount check that hits a fatal error (fractional replicas) mixed
	// with a passing conditionTrue check must fail the overall result—callers
	// rely on result.Passed, not the top-level error, to detect per-check failures.
	deploy := newDeploymentWithCondition("healthy", "test-ns", "Available", "True")
	badDeploy := &unstructured.Unstructured{}
	badDeploy.SetGroupVersionKind(schema.GroupVersionKind{Group: "apps", Version: "v1", Kind: "Deployment"})
	badDeploy.SetName("bad-replicas")
	badDeploy.SetNamespace("test-ns")
	badDeploy.Object["spec"] = map[string]interface{}{
		"replicas": float64(1.5),
	}
	obs := buildFakeClient(deploy, badDeploy)

	checks := []v1alpha1.SteadyStateCheck{
		{
			Type:          v1alpha1.CheckConditionTrue,
			APIVersion:    "apps/v1",
			Kind:          "Deployment",
			Name:          "healthy",
			Namespace:     "test-ns",
			ConditionType: "Available",
		},
		{
			Type:             v1alpha1.CheckReplicaCount,
			APIVersion:       "apps/v1",
			Kind:             "Deployment",
			Name:             "bad-replicas",
			Namespace:        "test-ns",
			ExpectedReplicas: int32Ptr(1),
		},
	}

	result, err := obs.CheckSteadyState(context.Background(), checks, "test-ns")
	require.NoError(t, err, "top-level error is nil; per-check errors live in CheckDetail.Error")
	assert.False(t, result.Passed, "overall result must fail when any check errors")
	assert.Equal(t, int32(2), result.ChecksRun)
	assert.Equal(t, int32(1), result.ChecksPassed)
	require.Len(t, result.Details, 2)
	assert.True(t, result.Details[0].Passed)
	assert.False(t, result.Details[1].Passed)
	assert.Contains(t, result.Details[1].Error, "non-integer value")
}

func TestCheckSteadyState_ResourceExists_NonNotFoundError(t *testing.T) {
	// Use an interceptor to return a non-NotFound error (simulating RBAC denied).
	scheme := runtime.NewScheme()
	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithInterceptorFuncs(interceptor.Funcs{
			Get: func(ctx context.Context, client client.WithWatch, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
				return fmt.Errorf("forbidden: user cannot get deployments")
			},
		}).
		Build()
	obs := NewKubernetesObserver(fakeClient)

	checks := []v1alpha1.SteadyStateCheck{
		{
			Type:       v1alpha1.CheckResourceExists,
			APIVersion: "apps/v1",
			Kind:       "Deployment",
			Name:       "test-deploy",
			Namespace:  "test-ns",
		},
	}

	result, err := obs.CheckSteadyState(context.Background(), checks, "test-ns")
	require.NoError(t, err) // CheckSteadyState wraps check errors in details, not top-level
	assert.False(t, result.Passed)
	require.Len(t, result.Details, 1)
	assert.False(t, result.Details[0].Passed)
	assert.NotEmpty(t, result.Details[0].Error, "non-NotFound errors should be surfaced in Error field")
	assert.Contains(t, result.Details[0].Error, "checking")
}
