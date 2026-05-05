package injection

import (
	"context"
	"errors"
	"fmt"
	"strings"

	v1alpha1 "github.com/opendatahub-io/operator-chaos/api/v1alpha1"
	"github.com/opendatahub-io/operator-chaos/pkg/safety"
	admissionv1 "k8s.io/api/admissionregistration/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var errNoValidatingMatch = errors.New("no ValidatingWebhookConfiguration found")
var errNoMutatingMatch = errors.New("no MutatingWebhookConfiguration found")

// WebhookDisruptInjector disrupts Kubernetes admission webhooks by modifying
// their configuration (e.g., changing FailurePolicy from Ignore to Fail).
// Supports both ValidatingWebhookConfiguration and MutatingWebhookConfiguration
// via the "webhookType" parameter ("validating" or "mutating", defaults to "validating").
type WebhookDisruptInjector struct {
	client client.Client
}

// NewWebhookDisruptInjector creates a new WebhookDisruptInjector.
func NewWebhookDisruptInjector(c client.Client) *WebhookDisruptInjector {
	return &WebhookDisruptInjector{client: c}
}

func (w *WebhookDisruptInjector) Validate(spec v1alpha1.InjectionSpec, blast v1alpha1.BlastRadiusSpec) error {
	return validateWebhookDisruptParams(spec)
}

// Inject performs the webhook disruption:
// 1. Fetches the webhook configuration (Validating or Mutating based on webhookType param)
// 2. Saves the original failure policies for all webhooks in the configuration
// 3. Sets the failure policy on all webhooks to the specified value
// 4. Returns a cleanup function that restores the original failure policies
func (w *WebhookDisruptInjector) Inject(ctx context.Context, spec v1alpha1.InjectionSpec, namespace string) (CleanupFunc, []v1alpha1.InjectionEvent, error) {
	webhookType := resolveWebhookType(spec.Parameters["webhookType"])

	webhookName, err := w.resolveWebhookName(ctx, spec.Parameters, webhookType)
	if err != nil {
		return nil, nil, fmt.Errorf("resolving webhook target: %w", err)
	}

	if err := checkResolvedWebhookDenyList(webhookName); err != nil {
		return nil, nil, err
	}

	targetPolicyStr := spec.Parameters["value"]
	if targetPolicyStr == "" {
		targetPolicyStr = "Fail"
	}
	targetPolicy, err := parseFailurePolicy(targetPolicyStr)
	if err != nil {
		return nil, nil, fmt.Errorf("parsing failure policy value: %w", err)
	}

	var originalPolicies map[string]string
	var webhookCount int

	if webhookType == "mutating" {
		originalPolicies, webhookCount, err = w.injectMutating(ctx, webhookName, targetPolicy)
	} else {
		originalPolicies, webhookCount, err = w.injectValidating(ctx, webhookName, targetPolicy)
	}
	if err != nil {
		return nil, nil, err
	}

	_ = originalPolicies // rollback data is stored as annotations on the resource

	events := []v1alpha1.InjectionEvent{
		NewEvent(v1alpha1.WebhookDisrupt, webhookName, "setFailurePolicy",
			map[string]string{
				"webhookName":   webhookName,
				"webhookType":   webhookType,
				"failurePolicy": targetPolicyStr,
				"webhookCount":  fmt.Sprintf("%d", webhookCount),
			}),
	}

	cleanup := func(ctx context.Context) error {
		return w.revertResolved(ctx, webhookName, webhookType)
	}

	return cleanup, events, nil
}

func (w *WebhookDisruptInjector) injectValidating(ctx context.Context, webhookName string, targetPolicy admissionv1.FailurePolicyType) (map[string]string, int, error) {
	webhookConfig := &admissionv1.ValidatingWebhookConfiguration{}
	if err := w.client.Get(ctx, client.ObjectKey{Name: webhookName}, webhookConfig); err != nil {
		return nil, 0, fmt.Errorf("getting ValidatingWebhookConfiguration %q: %w", webhookName, err)
	}

	if _, hasRollback := webhookConfig.GetAnnotations()[safety.RollbackAnnotationKey]; hasRollback {
		return nil, 0, fmt.Errorf("ValidatingWebhookConfiguration %q already has a chaos rollback annotation; revert the existing injection before re-injecting", webhookName)
	}

	originalPolicies := make(map[string]string, len(webhookConfig.Webhooks))
	for _, wh := range webhookConfig.Webhooks {
		if wh.FailurePolicy != nil {
			originalPolicies[wh.Name] = string(*wh.FailurePolicy)
		} else {
			originalPolicies[wh.Name] = ""
		}
	}

	rollbackStr, err := safety.WrapRollbackData(originalPolicies)
	if err != nil {
		return nil, 0, fmt.Errorf("serializing original policies for ValidatingWebhookConfiguration %q: %w", webhookName, err)
	}

	safety.ApplyChaosMetadata(webhookConfig, rollbackStr, string(v1alpha1.WebhookDisrupt))

	for i := range webhookConfig.Webhooks {
		p := targetPolicy
		webhookConfig.Webhooks[i].FailurePolicy = &p
	}

	if err := w.client.Update(ctx, webhookConfig); err != nil {
		return nil, 0, fmt.Errorf("updating ValidatingWebhookConfiguration %q: %w", webhookName, err)
	}

	return originalPolicies, len(webhookConfig.Webhooks), nil
}

func (w *WebhookDisruptInjector) injectMutating(ctx context.Context, webhookName string, targetPolicy admissionv1.FailurePolicyType) (map[string]string, int, error) {
	webhookConfig := &admissionv1.MutatingWebhookConfiguration{}
	if err := w.client.Get(ctx, client.ObjectKey{Name: webhookName}, webhookConfig); err != nil {
		return nil, 0, fmt.Errorf("getting MutatingWebhookConfiguration %q: %w", webhookName, err)
	}

	if _, hasRollback := webhookConfig.GetAnnotations()[safety.RollbackAnnotationKey]; hasRollback {
		return nil, 0, fmt.Errorf("MutatingWebhookConfiguration %q already has a chaos rollback annotation; revert the existing injection before re-injecting", webhookName)
	}

	originalPolicies := make(map[string]string, len(webhookConfig.Webhooks))
	for _, wh := range webhookConfig.Webhooks {
		if wh.FailurePolicy != nil {
			originalPolicies[wh.Name] = string(*wh.FailurePolicy)
		} else {
			originalPolicies[wh.Name] = ""
		}
	}

	rollbackStr, err := safety.WrapRollbackData(originalPolicies)
	if err != nil {
		return nil, 0, fmt.Errorf("serializing original policies for MutatingWebhookConfiguration %q: %w", webhookName, err)
	}

	safety.ApplyChaosMetadata(webhookConfig, rollbackStr, string(v1alpha1.WebhookDisrupt))

	for i := range webhookConfig.Webhooks {
		p := targetPolicy
		webhookConfig.Webhooks[i].FailurePolicy = &p
	}

	if err := w.client.Update(ctx, webhookConfig); err != nil {
		return nil, 0, fmt.Errorf("updating MutatingWebhookConfiguration %q: %w", webhookName, err)
	}

	return originalPolicies, len(webhookConfig.Webhooks), nil
}

// Revert restores the original failure policies on the webhook configuration.
// It is idempotent: if no rollback annotation is present, it returns nil.
// When using label-based discovery, if the webhook was already deleted (no match
// found), Revert returns nil since there is nothing left to restore.
func (w *WebhookDisruptInjector) Revert(ctx context.Context, spec v1alpha1.InjectionSpec, namespace string) error {
	webhookType := resolveWebhookType(spec.Parameters["webhookType"])

	webhookName, err := w.resolveWebhookName(ctx, spec.Parameters, webhookType)
	if err != nil {
		if spec.Parameters["webhookLabelSelector"] != "" && isNoMatchError(err) {
			return nil
		}
		return fmt.Errorf("resolving webhook target for revert: %w", err)
	}

	return w.revertResolved(ctx, webhookName, webhookType)
}

func (w *WebhookDisruptInjector) revertResolved(ctx context.Context, webhookName, webhookType string) error {
	if webhookType == "mutating" {
		return w.revertMutating(ctx, webhookName)
	}
	return w.revertValidating(ctx, webhookName)
}

func (w *WebhookDisruptInjector) revertValidating(ctx context.Context, webhookName string) error {
	webhookConfig := &admissionv1.ValidatingWebhookConfiguration{}
	if err := w.client.Get(ctx, client.ObjectKey{Name: webhookName}, webhookConfig); err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		return fmt.Errorf("getting ValidatingWebhookConfiguration %q for revert: %w", webhookName, err)
	}

	rollbackStr, ok := webhookConfig.GetAnnotations()[safety.RollbackAnnotationKey]
	if !ok {
		return nil
	}

	var originalPolicies map[string]string
	if err := safety.UnwrapRollbackData(rollbackStr, &originalPolicies); err != nil {
		return fmt.Errorf("unwrapping rollback data for ValidatingWebhookConfiguration %q: %w", webhookName, err)
	}

	for i, wh := range webhookConfig.Webhooks {
		if policyStr, ok := originalPolicies[wh.Name]; ok {
			if policyStr == "" {
				webhookConfig.Webhooks[i].FailurePolicy = nil
			} else {
				p := admissionv1.FailurePolicyType(policyStr)
				webhookConfig.Webhooks[i].FailurePolicy = &p
			}
		}
	}

	safety.RemoveChaosMetadata(webhookConfig, string(v1alpha1.WebhookDisrupt))
	return w.client.Update(ctx, webhookConfig)
}

func (w *WebhookDisruptInjector) revertMutating(ctx context.Context, webhookName string) error {
	webhookConfig := &admissionv1.MutatingWebhookConfiguration{}
	if err := w.client.Get(ctx, client.ObjectKey{Name: webhookName}, webhookConfig); err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		return fmt.Errorf("getting MutatingWebhookConfiguration %q for revert: %w", webhookName, err)
	}

	rollbackStr, ok := webhookConfig.GetAnnotations()[safety.RollbackAnnotationKey]
	if !ok {
		return nil
	}

	var originalPolicies map[string]string
	if err := safety.UnwrapRollbackData(rollbackStr, &originalPolicies); err != nil {
		return fmt.Errorf("unwrapping rollback data for MutatingWebhookConfiguration %q: %w", webhookName, err)
	}

	for i, wh := range webhookConfig.Webhooks {
		if policyStr, ok := originalPolicies[wh.Name]; ok {
			if policyStr == "" {
				webhookConfig.Webhooks[i].FailurePolicy = nil
			} else {
				p := admissionv1.FailurePolicyType(policyStr)
				webhookConfig.Webhooks[i].FailurePolicy = &p
			}
		}
	}

	safety.RemoveChaosMetadata(webhookConfig, string(v1alpha1.WebhookDisrupt))
	return w.client.Update(ctx, webhookConfig)
}

// resolveWebhookName returns the target webhook configuration name, either from
// the explicit webhookName parameter or by listing configurations matching the
// webhookLabelSelector. When using a label selector, exactly one matching
// configuration must exist.
func (w *WebhookDisruptInjector) resolveWebhookName(ctx context.Context, params map[string]string, webhookType string) (string, error) {
	if name := params["webhookName"]; name != "" {
		return name, nil
	}

	selector, err := labels.Parse(params["webhookLabelSelector"])
	if err != nil {
		return "", fmt.Errorf("parsing webhookLabelSelector: %w", err)
	}

	listOpts := &client.ListOptions{LabelSelector: selector}

	if webhookType == "mutating" {
		list := &admissionv1.MutatingWebhookConfigurationList{}
		if err := w.client.List(ctx, list, listOpts); err != nil {
			return "", fmt.Errorf("listing MutatingWebhookConfigurations: %w", err)
		}
		if len(list.Items) == 0 {
			return "", fmt.Errorf("%w matching label selector %q", errNoMutatingMatch, params["webhookLabelSelector"])
		}
		if len(list.Items) > 1 {
			names := make([]string, len(list.Items))
			for i, item := range list.Items {
				names[i] = item.Name
			}
			return "", fmt.Errorf("label selector %q matched %d MutatingWebhookConfigurations %v; must match exactly one", params["webhookLabelSelector"], len(list.Items), names)
		}
		return list.Items[0].Name, nil
	}

	list := &admissionv1.ValidatingWebhookConfigurationList{}
	if err := w.client.List(ctx, list, listOpts); err != nil {
		return "", fmt.Errorf("listing ValidatingWebhookConfigurations: %w", err)
	}
	if len(list.Items) == 0 {
		return "", fmt.Errorf("%w matching label selector %q", errNoValidatingMatch, params["webhookLabelSelector"])
	}
	if len(list.Items) > 1 {
		names := make([]string, len(list.Items))
		for i, item := range list.Items {
			names[i] = item.Name
		}
		return "", fmt.Errorf("label selector %q matched %d ValidatingWebhookConfigurations %v; must match exactly one", params["webhookLabelSelector"], len(list.Items), names)
	}
	return list.Items[0].Name, nil
}

// checkResolvedWebhookDenyList checks whether a resolved webhook name is
// in the system-critical deny-list. This is needed because label-based
// discovery bypasses the name-based deny-list checks in validation.
func checkResolvedWebhookDenyList(name string) error {
	if systemCriticalWebhooks[name] {
		return fmt.Errorf("targeting system-critical webhook %q is not allowed", name)
	}
	if strings.HasPrefix(name, "system:") {
		return fmt.Errorf("targeting system webhook %q is not allowed", name)
	}
	if strings.HasPrefix(name, "openshift-") {
		return fmt.Errorf("targeting OpenShift webhook %q is not allowed", name)
	}
	return nil
}

func isNoMatchError(err error) bool {
	return errors.Is(err, errNoValidatingMatch) || errors.Is(err, errNoMutatingMatch)
}

// resolveWebhookType returns the webhook type, defaulting to "validating".
func resolveWebhookType(t string) string {
	if t == "mutating" {
		return "mutating"
	}
	return "validating"
}

// parseFailurePolicy converts a string to an admissionv1.FailurePolicyType.
func parseFailurePolicy(s string) (admissionv1.FailurePolicyType, error) {
	switch admissionv1.FailurePolicyType(s) {
	case admissionv1.Fail:
		return admissionv1.Fail, nil
	case admissionv1.Ignore:
		return admissionv1.Ignore, nil
	default:
		return "", fmt.Errorf("invalid failure policy %q; must be 'Fail' or 'Ignore'", s)
	}
}
