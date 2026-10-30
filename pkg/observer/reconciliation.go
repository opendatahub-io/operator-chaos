package observer

import (
	"context"
	"fmt"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"

	"github.com/opendatahub-io/operator-chaos/pkg/clock"
	"github.com/opendatahub-io/operator-chaos/pkg/model"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const reconciliationPollInterval = 2 * time.Second

// ReconciliationChecker verifies that an operator has properly reconciled
// all managed resources for a given component. This is the key innovation
// of operator-chaos: checking semantic reconciliation (correct metadata,
// spec, conditions, and owner references), not just pod restarts.
type ReconciliationChecker struct {
	client client.Client
	clock  clock.Clock
}

// NewReconciliationChecker creates a new ReconciliationChecker with the given client.
func NewReconciliationChecker(c client.Client) *ReconciliationChecker {
	return &ReconciliationChecker{client: c, clock: clock.RealClock{}}
}

// ReconciliationResult captures the outcome of reconciliation verification,
// including per-resource details, number of reconcile cycles observed,
// and total recovery time.
type ReconciliationResult struct {
	AllReconciled   bool                  `json:"allReconciled"`
	ReconcileCycles int                   `json:"reconcileCycles"`
	RecoveryTime    time.Duration         `json:"recoveryTime"`
	Resources       []ResourceCheckResult `json:"resources"`
}

// ResourceCheckResult captures the reconciliation status of a single managed resource.
type ResourceCheckResult struct {
	Kind       string `json:"kind"`
	Name       string `json:"name"`
	Namespace  string `json:"namespace"`
	Reconciled bool   `json:"reconciled"`
	Details    string `json:"details,omitempty"`
}

// CheckReconciliation verifies all managed resources for a component
// have been properly reconciled by the operator. It polls until all
// resources are reconciled or the timeout expires.
func (r *ReconciliationChecker) CheckReconciliation(
	ctx context.Context,
	component *model.ComponentModel,
	namespace string,
	timeout time.Duration,
) (*ReconciliationResult, error) {
	result := &ReconciliationResult{
		AllReconciled: true,
	}

	startTime := r.clock.Now()
	deadline := startTime.Add(timeout)
	cycles := 0

	ticker := time.NewTicker(reconciliationPollInterval)
	defer ticker.Stop()

	for r.clock.Now().Before(deadline) {
		cycles++
		cycle, err := r.CheckReconciliationOnce(ctx, component, namespace)
		if err != nil {
			result.AllReconciled = false
			result.ReconcileCycles = cycles
			result.RecoveryTime = r.clock.Now().Sub(startTime)
			return result, err
		}
		result.Resources = cycle.Resources
		result.AllReconciled = cycle.AllReconciled

		if cycle.AllReconciled {
			result.ReconcileCycles = cycles
			result.RecoveryTime = r.clock.Now().Sub(startTime)
			return result, nil
		}

		// Wait for poll interval or context cancellation
		select {
		case <-ticker.C:
			// continue polling
		case <-ctx.Done():
			result.AllReconciled = false
			result.ReconcileCycles = cycles
			result.RecoveryTime = r.clock.Now().Sub(startTime)
			return result, ctx.Err()
		}
	}

	result.AllReconciled = false
	result.ReconcileCycles = cycles
	result.RecoveryTime = r.clock.Now().Sub(startTime)
	return result, nil
}

// CheckReconciliationOnce runs a single pass over the component's managed
// resources, with no polling loop or deadline: each resource is looked up in
// its declared namespace (falling back to the injection namespace when
// omitted) and checked for existence, owner references, and expected spec
// fields. Callers that need polling drive this method on their own cadence —
// the CLI loops inside RunPostCheck, the controller requeues per cycle.
func (r *ReconciliationChecker) CheckReconciliationOnce(
	ctx context.Context,
	component *model.ComponentModel,
	namespace string,
) (*ReconciliationResult, error) {
	result := &ReconciliationResult{
		AllReconciled: true,
	}

	for _, mr := range component.ManagedResources {
		check, err := r.checkManagedResource(ctx, mr, namespace)
		if err != nil {
			return nil, err
		}
		if !check.Reconciled {
			result.AllReconciled = false
		}
		result.Resources = append(result.Resources, check)
	}

	return result, nil
}

// checkManagedResource looks up a single managed resource in its declared
// namespace (falling back to the injection namespace when omitted) and
// verifies existence, owner references, and expected spec fields.
func (r *ReconciliationChecker) checkManagedResource(ctx context.Context, mr model.ManagedResource, namespace string) (ResourceCheckResult, error) {
	ns := mr.Namespace
	if ns == "" {
		ns = namespace
	}

	obj := &unstructured.Unstructured{}
	obj.SetAPIVersion(mr.APIVersion)
	obj.SetKind(mr.Kind)

	err := r.client.Get(ctx, types.NamespacedName{Name: mr.Name, Namespace: ns}, obj)
	if err != nil {
		// Only NotFound is an expected lookup result for a resource the
		// operator has not reconciled yet. Other errors (Forbidden from a
		// missing RBAC rule, network failures) must propagate: recording
		// them as unreconciled resources would mask an operational failure
		// as a Degraded verdict.
		if !apierrors.IsNotFound(err) {
			return ResourceCheckResult{}, fmt.Errorf("looking up %s %s/%s: %w", mr.Kind, ns, mr.Name, err)
		}
		return ResourceCheckResult{
			Kind:       mr.Kind,
			Name:       mr.Name,
			Namespace:  ns,
			Reconciled: false,
			Details:    fmt.Sprintf("not found: %v", err),
		}, nil
	}

	// Check owner reference if expected
	reconciled := true
	details := "exists"

	if mr.OwnerRef != "" {
		hasOwner := false
		for _, ref := range obj.GetOwnerReferences() {
			if ref.Kind == mr.OwnerRef {
				hasOwner = true
				break
			}
		}
		if !hasOwner {
			reconciled = false
			details = fmt.Sprintf("missing owner reference %s", mr.OwnerRef)
		}
	}

	// Check expected spec fields if defined
	if mr.ExpectedSpec != nil {
		for field, expected := range mr.ExpectedSpec {
			actual, found, _ := unstructured.NestedFieldNoCopy(obj.Object, "spec", field)
			if !found {
				reconciled = false
				details = fmt.Sprintf("spec.%s not found", field)
			} else if fmt.Sprintf("%v", actual) != fmt.Sprintf("%v", expected) {
				reconciled = false
				details = fmt.Sprintf("spec.%s: expected %v, got %v", field, expected, actual)
			}
		}
	}

	return ResourceCheckResult{
		Kind:       mr.Kind,
		Name:       mr.Name,
		Namespace:  ns,
		Reconciled: reconciled,
		Details:    details,
	}, nil
}
