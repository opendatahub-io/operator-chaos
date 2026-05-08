package observer

import (
	"context"
	"fmt"

	v1alpha1 "github.com/opendatahub-io/operator-chaos/api/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// KubernetesObserver checks Kubernetes resource state for steady-state conditions.
type KubernetesObserver struct {
	client client.Client
}

// NewKubernetesObserver creates a new KubernetesObserver with the given client.
func NewKubernetesObserver(c client.Client) *KubernetesObserver {
	return &KubernetesObserver{client: c}
}

// CheckSteadyState evaluates a list of steady-state checks against the cluster
// and returns a CheckResult summarizing which checks passed or failed.
func (o *KubernetesObserver) CheckSteadyState(ctx context.Context, checks []v1alpha1.SteadyStateCheck, namespace string) (*v1alpha1.CheckResult, error) {
	result := &v1alpha1.CheckResult{
		ChecksRun: int32(len(checks)),
		Timestamp: metav1.Now(),
	}

	for _, check := range checks {
		detail := v1alpha1.CheckDetail{Check: check}

		switch check.Type {
		case v1alpha1.CheckConditionTrue:
			passed, value, err := o.checkCondition(ctx, check, namespace)
			detail.Passed = passed
			detail.Value = value
			if err != nil {
				detail.Error = err.Error()
			}
		case v1alpha1.CheckResourceExists:
			passed, err := o.checkResourceExists(ctx, check, namespace)
			detail.Passed = passed
			if err != nil {
				detail.Error = err.Error()
			}
		case v1alpha1.CheckReplicaCount:
			passed, value, err := o.checkReplicaCount(ctx, check, namespace)
			detail.Passed = passed
			detail.Value = value
			if err != nil {
				detail.Error = err.Error()
			}
		default:
			detail.Error = fmt.Sprintf("unknown check type: %s", check.Type)
		}

		if detail.Passed {
			result.ChecksPassed++
		}
		result.Details = append(result.Details, detail)
	}

	result.Passed = result.ChecksPassed == result.ChecksRun
	return result, nil
}

// checkCondition verifies that a specific condition on a Kubernetes resource has status "True".
func (o *KubernetesObserver) checkCondition(ctx context.Context, check v1alpha1.SteadyStateCheck, namespace string) (bool, string, error) {
	// Per-check namespace takes priority; fall back to the function parameter.
	ns := check.Namespace
	if ns == "" {
		ns = namespace
	}

	obj := &unstructured.Unstructured{}
	obj.SetAPIVersion(check.APIVersion)
	obj.SetKind(check.Kind)

	if err := o.client.Get(ctx, types.NamespacedName{Name: check.Name, Namespace: ns}, obj); err != nil {
		return false, "", fmt.Errorf("getting %s/%s: %w", check.Kind, check.Name, err)
	}

	conditions, found, err := unstructured.NestedSlice(obj.Object, "status", "conditions")
	if err != nil || !found {
		return false, "no conditions found", nil
	}

	for _, c := range conditions {
		cond, ok := c.(map[string]any)
		if !ok {
			continue
		}
		condType, typeFound, _ := unstructured.NestedString(cond, "type")
		if !typeFound {
			continue
		}
		condStatus, statusFound, _ := unstructured.NestedString(cond, "status")
		if !statusFound {
			condStatus = ""
		}

		if condType == check.ConditionType {
			return condStatus == "True", fmt.Sprintf("%s=%s", condType, condStatus), nil
		}
	}

	return false, fmt.Sprintf("condition %s not found", check.ConditionType), nil
}

// checkReplicaCount verifies that a Deployment or StatefulSet has the expected number
// of spec.replicas. This catches cases where a condition like Available might be stale
// but the actual replica count has been changed (e.g., scaled to zero).
func (o *KubernetesObserver) checkReplicaCount(ctx context.Context, check v1alpha1.SteadyStateCheck, namespace string) (bool, string, error) {
	ns := check.Namespace
	if ns == "" {
		ns = namespace
	}

	if check.ExpectedReplicas == nil {
		return false, "", fmt.Errorf("replicaCount check requires expectedReplicas field")
	}

	obj := &unstructured.Unstructured{}
	obj.SetAPIVersion(check.APIVersion)
	obj.SetKind(check.Kind)

	if err := o.client.Get(ctx, types.NamespacedName{Name: check.Name, Namespace: ns}, obj); err != nil {
		return false, "", fmt.Errorf("getting %s/%s: %w", check.Kind, check.Name, err)
	}

	replicas, found, err := unstructured.NestedFieldNoCopy(obj.Object, "spec", "replicas")
	if err != nil {
		return false, "", fmt.Errorf("reading spec.replicas from %s/%s: %w", check.Kind, check.Name, err)
	}

	expected := int64(*check.ExpectedReplicas)
	if !found {
		switch check.Kind {
		case "Deployment", "StatefulSet", "ReplicaSet":
			if expected == 1 {
				return true, "replicas=1 (default)", nil
			}
			return false, fmt.Sprintf("replicas not set (default 1), expected %d", expected), nil
		default:
			return false, "", fmt.Errorf("spec.replicas is not defined for kind %s", check.Kind)
		}
	}

	var actual int64
	switch v := replicas.(type) {
	case int64:
		actual = v
	case float64:
		if v != float64(int64(v)) {
			return false, "", fmt.Errorf("spec.replicas has non-integer value %v for %s/%s", v, check.Kind, check.Name)
		}
		actual = int64(v)
	default:
		return false, "", fmt.Errorf("spec.replicas has unexpected type %T", replicas)
	}

	if actual < 0 {
		return false, "", fmt.Errorf("spec.replicas is negative (%d) for %s/%s", actual, check.Kind, check.Name)
	}

	if actual == expected {
		return true, fmt.Sprintf("replicas=%d", actual), nil
	}
	return false, fmt.Sprintf("replicas=%d, expected %d", actual, expected), nil
}

// checkResourceExists verifies that a specific Kubernetes resource exists in the cluster.
func (o *KubernetesObserver) checkResourceExists(ctx context.Context, check v1alpha1.SteadyStateCheck, namespace string) (bool, error) {
	// Per-check namespace takes priority; fall back to the function parameter.
	ns := check.Namespace
	if ns == "" {
		ns = namespace
	}

	obj := &unstructured.Unstructured{}
	obj.SetAPIVersion(check.APIVersion)
	obj.SetKind(check.Kind)

	err := o.client.Get(ctx, types.NamespacedName{Name: check.Name, Namespace: ns}, obj)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return false, nil // Resource genuinely doesn't exist
		}
		return false, fmt.Errorf("checking %s/%s: %w", check.Kind, check.Name, err)
	}
	return true, nil
}
