package cli

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	v1alpha1 "github.com/opendatahub-io/odh-platform-chaos/api/v1alpha1"
	"github.com/opendatahub-io/odh-platform-chaos/pkg/model"
)

const validKnowledgeYAML = `operator:
  name: test-operator
  namespace: test-ns
components:
  - name: dashboard
    controller: DataScienceCluster
    managedResources:
      - apiVersion: apps/v1
        kind: Deployment
        name: test-dashboard
        namespace: test-ns
      - apiVersion: v1
        kind: Service
        name: test-dashboard-svc
        namespace: test-ns
    steadyState:
      checks:
        - type: conditionTrue
          apiVersion: apps/v1
          kind: Deployment
          name: test-dashboard
          conditionType: Available
      timeout: "60s"
recovery:
  reconcileTimeout: "300s"
  maxReconcileCycles: 10
`

const invalidKnowledgeYAML = `operator:
  name: ""
  namespace: ""
components: []
recovery:
  reconcileTimeout: "0s"
  maxReconcileCycles: 0
`

// writeTestKnowledge writes YAML content to a temp file and returns its path.
func writeTestKnowledge(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	p := filepath.Join(dir, "knowledge.yaml")
	require.NoError(t, os.WriteFile(p, []byte(content), 0600))
	return p
}

func TestPreflightLocalValidKnowledge(t *testing.T) {
	path := writeTestKnowledge(t, validKnowledgeYAML)
	cmd := newPreflightCommand()
	cmd.SetArgs([]string{
		"--knowledge", path,
		"--local",
	})

	err := cmd.Execute()
	assert.NoError(t, err)
}

func TestPreflightLocalInvalidKnowledge(t *testing.T) {
	path := writeTestKnowledge(t, invalidKnowledgeYAML)
	cmd := newPreflightCommand()
	cmd.SetArgs([]string{
		"--knowledge", path,
		"--local",
	})

	err := cmd.Execute()
	assert.Error(t, err)
}

func TestPreflightLocalNonExistentFile(t *testing.T) {
	cmd := newPreflightCommand()
	cmd.SetArgs([]string{
		"--knowledge", "/nonexistent/path/knowledge.yaml",
		"--local",
	})

	err := cmd.Execute()
	assert.Error(t, err)
}

func TestPreflightCrossReferenceCheckSteadyStateRef(t *testing.T) {
	knowledge := &model.OperatorKnowledge{
		Operator: model.OperatorMeta{
			Name:      "test-operator",
			Namespace: "test-ns",
		},
		Components: []model.ComponentModel{
			{
				Name:       "comp1",
				Controller: "TestController",
				ManagedResources: []model.ManagedResource{
					{
						APIVersion: "apps/v1",
						Kind:       "Deployment",
						Name:       "my-deploy",
					},
				},
				SteadyState: v1alpha1.SteadyStateDef{
					Checks: []v1alpha1.SteadyStateCheck{
						{
							Type: v1alpha1.CheckConditionTrue,
							Name: "nonexistent-resource",
						},
					},
				},
			},
		},
	}

	errs := crossReferenceChecks(knowledge)
	require.Len(t, errs, 1)
	assert.Contains(t, errs[0], "nonexistent-resource")
	assert.Contains(t, errs[0], "not declared in managedResources")
}

func TestPreflightCrossReferenceCheckValid(t *testing.T) {
	knowledge := &model.OperatorKnowledge{
		Operator: model.OperatorMeta{
			Name:      "test-operator",
			Namespace: "test-ns",
		},
		Components: []model.ComponentModel{
			{
				Name:       "comp1",
				Controller: "TestController",
				ManagedResources: []model.ManagedResource{
					{
						APIVersion: "apps/v1",
						Kind:       "Deployment",
						Name:       "my-deploy",
					},
				},
				SteadyState: v1alpha1.SteadyStateDef{
					Checks: []v1alpha1.SteadyStateCheck{
						{
							Type: v1alpha1.CheckConditionTrue,
							Name: "my-deploy",
						},
					},
				},
			},
		},
	}

	errs := crossReferenceChecks(knowledge)
	assert.Empty(t, errs)
}

func TestPreflightFlagRegistration(t *testing.T) {
	cmd := newPreflightCommand()

	knowledgeFlag := cmd.Flags().Lookup("knowledge")
	require.NotNil(t, knowledgeFlag, "--knowledge flag should be registered")

	localFlag := cmd.Flags().Lookup("local")
	require.NotNil(t, localFlag, "--local flag should be registered")
}

func TestPreflightClusterResourceCheck(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = appsv1.AddToScheme(scheme)
	_ = corev1.AddToScheme(scheme)

	deploy := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-dashboard",
			Namespace: "test-ns",
		},
		Status: appsv1.DeploymentStatus{
			Conditions: []appsv1.DeploymentCondition{
				{
					Type:   appsv1.DeploymentAvailable,
					Status: corev1.ConditionTrue,
				},
			},
		},
	}

	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-service",
			Namespace: "test-ns",
		},
	}

	k8sClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(deploy, svc).
		WithStatusSubresource(deploy).
		Build()

	knowledge := &model.OperatorKnowledge{
		Components: []model.ComponentModel{
			{
				Name: "dashboard",
				ManagedResources: []model.ManagedResource{
					{
						APIVersion: "apps/v1",
						Kind:       "Deployment",
						Name:       "test-dashboard",
						Namespace:  "test-ns",
					},
					{
						APIVersion: "v1",
						Kind:       "Service",
						Name:       "test-service",
						Namespace:  "test-ns",
					},
					{
						APIVersion: "v1",
						Kind:       "Service",
						Name:       "missing-service",
						Namespace:  "test-ns",
					},
				},
			},
		},
	}

	results, err := checkClusterResources(context.Background(), k8sClient, knowledge, "test-ns")
	require.NoError(t, err)
	require.Len(t, results, 3)

	// The deployment exists and has Available=True -> Found
	assert.Equal(t, "Found", results[0].Status)
	assert.Equal(t, "test-dashboard", results[0].Name)

	// The service exists -> Found
	assert.Equal(t, "Found", results[1].Status)
	assert.Equal(t, "test-service", results[1].Name)

	// The missing service -> Missing
	assert.Equal(t, "Missing", results[2].Status)
	assert.Equal(t, "missing-service", results[2].Name)
}

func TestPreflightDeploymentDegraded(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = appsv1.AddToScheme(scheme)

	deploy := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "degraded-deploy",
			Namespace: "test-ns",
		},
		Status: appsv1.DeploymentStatus{
			Conditions: []appsv1.DeploymentCondition{
				{
					Type:   appsv1.DeploymentAvailable,
					Status: corev1.ConditionFalse,
				},
			},
		},
	}

	k8sClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(deploy).
		WithStatusSubresource(deploy).
		Build()

	mr := model.ManagedResource{
		APIVersion: "apps/v1",
		Kind:       "Deployment",
		Name:       "degraded-deploy",
	}

	status := checkSingleResource(context.Background(), k8sClient, mr, "test-ns")
	assert.Equal(t, "Degraded", status)
}
