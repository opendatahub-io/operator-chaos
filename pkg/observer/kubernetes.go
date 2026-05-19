package observer

import (
	"context"
	"fmt"
	"strings"

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
		case v1alpha1.CheckFieldEquals:
			passed, value, err := o.checkFieldEquals(ctx, check, namespace)
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

// checkFieldEquals verifies that a specific field in a Kubernetes resource has the expected value.
// FieldPath uses dot notation to traverse the object (e.g., "data.OAUTH2_PROXY_CLIENT_ID" for a Secret).
func (o *KubernetesObserver) checkFieldEquals(ctx context.Context, check v1alpha1.SteadyStateCheck, namespace string) (bool, string, error) {
	ns := check.Namespace
	if ns == "" {
		ns = namespace
	}

	obj := &unstructured.Unstructured{}
	obj.SetAPIVersion(check.APIVersion)
	obj.SetKind(check.Kind)

	if err := o.client.Get(ctx, types.NamespacedName{Name: check.Name, Namespace: ns}, obj); err != nil {
		if apierrors.IsNotFound(err) {
			return false, "resource not found", nil
		}
		return false, "", fmt.Errorf("getting %s/%s: %w", check.Kind, check.Name, err)
	}

	fields := strings.Split(check.FieldPath, ".")
	val, found, err := unstructured.NestedString(obj.Object, fields...)
	if err != nil {
		rawVal, rawFound, rawErr := unstructured.NestedFieldNoCopy(obj.Object, fields...)
		if rawErr != nil || !rawFound {
			return false, fmt.Sprintf("field %s not found", check.FieldPath), nil
		}
		val = fmt.Sprintf("%v", rawVal)
	}
	if !found {
		return false, fmt.Sprintf("field %s not found", check.FieldPath), nil
	}

	if val == check.ExpectedValue {
		return true, val, nil
	}
	return false, fmt.Sprintf("expected %q, got %q", check.ExpectedValue, val), nil
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
