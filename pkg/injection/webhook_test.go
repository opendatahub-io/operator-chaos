package injection

import (
	"context"
	"testing"

	v1alpha1 "github.com/opendatahub-io/operator-chaos/api/v1alpha1"
	"github.com/opendatahub-io/operator-chaos/pkg/safety"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	admissionv1 "k8s.io/api/admissionregistration/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestWebhookDisruptValidate(t *testing.T) {
	injector := NewWebhookDisruptInjector(nil)
	blast := v1alpha1.BlastRadiusSpec{MaxPodsAffected: 1, AllowedNamespaces: []string{"test"}}

	tests := []struct {
		name    string
		spec    v1alpha1.InjectionSpec
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid spec with setFailurePolicy action",
			spec: v1alpha1.InjectionSpec{
				Type: v1alpha1.WebhookDisrupt,
				Parameters: map[string]string{
					"webhookName": "my-webhook",
					"action":      "setFailurePolicy",
					"value":       "Fail",
				},
			},
			wantErr: false,
		},
		{
			name: "valid spec without value defaults to Fail",
			spec: v1alpha1.InjectionSpec{
				Type: v1alpha1.WebhookDisrupt,
				Parameters: map[string]string{
					"webhookName": "my-webhook",
					"action":      "setFailurePolicy",
				},
			},
			wantErr: false,
		},
		{
			name: "missing webhookName",
			spec: v1alpha1.InjectionSpec{
				Type:       v1alpha1.WebhookDisrupt,
				Parameters: map[string]string{"action": "setFailurePolicy"},
			},
			wantErr: true,
			errMsg:  "webhookName",
		},
		{
			name: "missing action",
			spec: v1alpha1.InjectionSpec{
				Type:       v1alpha1.WebhookDisrupt,
				Parameters: map[string]string{"webhookName": "my-webhook"},
			},
			wantErr: true,
			errMsg:  "action",
		},
		{
			name: "nil parameters",
			spec: v1alpha1.InjectionSpec{
				Type: v1alpha1.WebhookDisrupt,
			},
			wantErr: true,
			errMsg:  "webhookName",
		},
		{
			name: "unsupported action",
			spec: v1alpha1.InjectionSpec{
				Type: v1alpha1.WebhookDisrupt,
				Parameters: map[string]string{
					"webhookName": "my-webhook",
					"action":      "deleteWebhook",
				},
			},
			wantErr: true,
			errMsg:  "unsupported action",
		},
		{
			name: "invalid webhook name",
			spec: v1alpha1.InjectionSpec{
				Type: v1alpha1.WebhookDisrupt,
				Parameters: map[string]string{
					"webhookName": "INVALID NAME!",
					"action":      "setFailurePolicy",
				},
			},
			wantErr: true,
			errMsg:  "not a valid Kubernetes resource name",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := injector.Validate(tt.spec, blast)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestWebhookDisruptInjectAndCleanup(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, admissionv1.AddToScheme(scheme))

	failPolicy := admissionv1.Ignore
	webhook := &admissionv1.ValidatingWebhookConfiguration{
		ObjectMeta: metav1.ObjectMeta{Name: "my-webhook"},
		Webhooks: []admissionv1.ValidatingWebhook{
			{
				Name:                    "test.webhook.io",
				FailurePolicy:           &failPolicy,
				ClientConfig:            admissionv1.WebhookClientConfig{URL: strPtr("https://example.com")},
				SideEffects:             sideEffectPtr(admissionv1.SideEffectClassNone),
				AdmissionReviewVersions: []string{"v1"},
			},
		},
	}

	fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(webhook).Build()
	injector := NewWebhookDisruptInjector(fakeClient)

	spec := v1alpha1.InjectionSpec{
		Type: v1alpha1.WebhookDisrupt,
		Parameters: map[string]string{
			"webhookName": "my-webhook",
			"action":      "setFailurePolicy",
			"value":       "Fail",
		},
	}

	ctx := context.Background()

	// Inject
	cleanup, events, err := injector.Inject(ctx, spec, "default")
	require.NoError(t, err)
	assert.NotEmpty(t, events)
	assert.NotNil(t, cleanup)
	assert.Equal(t, "setFailurePolicy", events[0].Action)

	// Verify the webhook was modified
	modified := &admissionv1.ValidatingWebhookConfiguration{}
	require.NoError(t, fakeClient.Get(ctx,
		client.ObjectKey{Name: "my-webhook"}, modified))
	require.NotNil(t, modified.Webhooks[0].FailurePolicy)
	assert.Equal(t, admissionv1.Fail, *modified.Webhooks[0].FailurePolicy)

	// Cleanup should restore
	require.NoError(t, cleanup(ctx))
	restored := &admissionv1.ValidatingWebhookConfiguration{}
	require.NoError(t, fakeClient.Get(ctx,
		client.ObjectKey{Name: "my-webhook"}, restored))
	require.NotNil(t, restored.Webhooks[0].FailurePolicy)
	assert.Equal(t, admissionv1.Ignore, *restored.Webhooks[0].FailurePolicy)
}

func TestWebhookDisruptInjectMultipleWebhooks(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, admissionv1.AddToScheme(scheme))

	ignorePolicy := admissionv1.Ignore
	webhook := &admissionv1.ValidatingWebhookConfiguration{
		ObjectMeta: metav1.ObjectMeta{Name: "multi-webhook"},
		Webhooks: []admissionv1.ValidatingWebhook{
			{
				Name:                    "first.webhook.io",
				FailurePolicy:           &ignorePolicy,
				ClientConfig:            admissionv1.WebhookClientConfig{URL: strPtr("https://example.com/first")},
				SideEffects:             sideEffectPtr(admissionv1.SideEffectClassNone),
				AdmissionReviewVersions: []string{"v1"},
			},
			{
				Name:                    "second.webhook.io",
				FailurePolicy:           &ignorePolicy,
				ClientConfig:            admissionv1.WebhookClientConfig{URL: strPtr("https://example.com/second")},
				SideEffects:             sideEffectPtr(admissionv1.SideEffectClassNone),
				AdmissionReviewVersions: []string{"v1"},
			},
		},
	}

	fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(webhook).Build()
	injector := NewWebhookDisruptInjector(fakeClient)

	spec := v1alpha1.InjectionSpec{
		Type: v1alpha1.WebhookDisrupt,
		Parameters: map[string]string{
			"webhookName": "multi-webhook",
			"action":      "setFailurePolicy",
			"value":       "Fail",
		},
	}

	ctx := context.Background()

	// Inject - all webhooks in the configuration should be modified
	cleanup, events, err := injector.Inject(ctx, spec, "default")
	require.NoError(t, err)
	assert.NotEmpty(t, events)

	modified := &admissionv1.ValidatingWebhookConfiguration{}
	require.NoError(t, fakeClient.Get(ctx, client.ObjectKey{Name: "multi-webhook"}, modified))
	for i, wh := range modified.Webhooks {
		require.NotNil(t, wh.FailurePolicy, "webhook %d should have failure policy set", i)
		assert.Equal(t, admissionv1.Fail, *wh.FailurePolicy, "webhook %d should be Fail", i)
	}

	// Cleanup should restore all webhooks
	require.NoError(t, cleanup(ctx))
	restored := &admissionv1.ValidatingWebhookConfiguration{}
	require.NoError(t, fakeClient.Get(ctx, client.ObjectKey{Name: "multi-webhook"}, restored))
	for i, wh := range restored.Webhooks {
		require.NotNil(t, wh.FailurePolicy, "webhook %d should have failure policy set", i)
		assert.Equal(t, admissionv1.Ignore, *wh.FailurePolicy, "webhook %d should be restored to Ignore", i)
	}
}

func TestWebhookDisruptInjectStoresRollbackAnnotation(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, admissionv1.AddToScheme(scheme))

	ignorePolicy := admissionv1.Ignore
	webhook := &admissionv1.ValidatingWebhookConfiguration{
		ObjectMeta: metav1.ObjectMeta{Name: "annotated-webhook"},
		Webhooks: []admissionv1.ValidatingWebhook{
			{
				Name:                    "test.webhook.io",
				FailurePolicy:           &ignorePolicy,
				ClientConfig:            admissionv1.WebhookClientConfig{URL: strPtr("https://example.com")},
				SideEffects:             sideEffectPtr(admissionv1.SideEffectClassNone),
				AdmissionReviewVersions: []string{"v1"},
			},
		},
	}

	fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(webhook).Build()
	injector := NewWebhookDisruptInjector(fakeClient)

	spec := v1alpha1.InjectionSpec{
		Type: v1alpha1.WebhookDisrupt,
		Parameters: map[string]string{
			"webhookName": "annotated-webhook",
			"action":      "setFailurePolicy",
			"value":       "Fail",
		},
	}

	ctx := context.Background()

	// Inject
	cleanup, _, err := injector.Inject(ctx, spec, "default")
	require.NoError(t, err)

	// Verify the rollback annotation exists with original policy
	modified := &admissionv1.ValidatingWebhookConfiguration{}
	require.NoError(t, fakeClient.Get(ctx, client.ObjectKey{Name: "annotated-webhook"}, modified))

	rollbackJSON, ok := modified.Annotations[safety.RollbackAnnotationKey]
	require.True(t, ok, "rollback annotation should be present after injection")

	var rollbackData map[string]string
	require.NoError(t, safety.UnwrapRollbackData(rollbackJSON, &rollbackData))
	assert.Equal(t, "Ignore", rollbackData["test.webhook.io"], "rollback data should contain original Ignore policy")

	// Verify chaos labels are present
	assert.Equal(t, safety.ManagedByValue, modified.Labels[safety.ManagedByLabel])
	assert.Equal(t, string(v1alpha1.WebhookDisrupt), modified.Labels[safety.ChaosTypeLabel])

	// Cleanup should remove annotation and labels
	require.NoError(t, cleanup(ctx))
	restored := &admissionv1.ValidatingWebhookConfiguration{}
	require.NoError(t, fakeClient.Get(ctx, client.ObjectKey{Name: "annotated-webhook"}, restored))

	_, hasAnnotation := restored.Annotations[safety.RollbackAnnotationKey]
	assert.False(t, hasAnnotation, "rollback annotation should be removed after cleanup")

	_, hasManagedBy := restored.Labels[safety.ManagedByLabel]
	assert.False(t, hasManagedBy, "managed-by label should be removed after cleanup")

	_, hasChaosType := restored.Labels[safety.ChaosTypeLabel]
	assert.False(t, hasChaosType, "chaos-type label should be removed after cleanup")

	// Verify policy was restored
	require.NotNil(t, restored.Webhooks[0].FailurePolicy)
	assert.Equal(t, admissionv1.Ignore, *restored.Webhooks[0].FailurePolicy)
}

func TestWebhookDisruptRevert(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, admissionv1.AddToScheme(scheme))

	ignorePolicy := admissionv1.Ignore
	webhook := &admissionv1.ValidatingWebhookConfiguration{
		ObjectMeta: metav1.ObjectMeta{Name: "revert-webhook"},
		Webhooks: []admissionv1.ValidatingWebhook{
			{
				Name:                    "test.webhook.io",
				FailurePolicy:           &ignorePolicy,
				ClientConfig:            admissionv1.WebhookClientConfig{URL: strPtr("https://example.com")},
				SideEffects:             sideEffectPtr(admissionv1.SideEffectClassNone),
				AdmissionReviewVersions: []string{"v1"},
			},
		},
	}

	fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(webhook).Build()
	injector := NewWebhookDisruptInjector(fakeClient)

	spec := v1alpha1.InjectionSpec{
		Type: v1alpha1.WebhookDisrupt,
		Parameters: map[string]string{
			"webhookName": "revert-webhook",
			"action":      "setFailurePolicy",
			"value":       "Fail",
		},
	}

	ctx := context.Background()

	// Inject
	_, _, err := injector.Inject(ctx, spec, "default")
	require.NoError(t, err)

	// Revert
	err = injector.Revert(ctx, spec, "default")
	require.NoError(t, err)

	// Verify policy restored
	restored := &admissionv1.ValidatingWebhookConfiguration{}
	require.NoError(t, fakeClient.Get(ctx, client.ObjectKey{Name: "revert-webhook"}, restored))
	require.NotNil(t, restored.Webhooks[0].FailurePolicy)
	assert.Equal(t, admissionv1.Ignore, *restored.Webhooks[0].FailurePolicy)

	// Idempotent
	err = injector.Revert(ctx, spec, "default")
	assert.NoError(t, err)
}

func TestWebhookDisruptNotFound(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, admissionv1.AddToScheme(scheme))

	fakeClient := fake.NewClientBuilder().WithScheme(scheme).Build()
	injector := NewWebhookDisruptInjector(fakeClient)

	spec := v1alpha1.InjectionSpec{
		Type: v1alpha1.WebhookDisrupt,
		Parameters: map[string]string{
			"webhookName": "nonexistent-webhook",
			"action":      "setFailurePolicy",
			"value":       "Fail",
		},
	}

	_, _, err := injector.Inject(context.Background(), spec, "default")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "nonexistent-webhook")
}

func TestWebhookDisruptMutatingInjectAndCleanup(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, admissionv1.AddToScheme(scheme))

	failPolicy := admissionv1.Fail
	webhook := &admissionv1.MutatingWebhookConfiguration{
		ObjectMeta: metav1.ObjectMeta{Name: "my-mutating-webhook"},
		Webhooks: []admissionv1.MutatingWebhook{
			{
				Name:                    "mutate.test.io",
				FailurePolicy:           &failPolicy,
				ClientConfig:            admissionv1.WebhookClientConfig{URL: strPtr("https://example.com")},
				SideEffects:             sideEffectPtr(admissionv1.SideEffectClassNone),
				AdmissionReviewVersions: []string{"v1"},
			},
		},
	}

	fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(webhook).Build()
	injector := NewWebhookDisruptInjector(fakeClient)

	spec := v1alpha1.InjectionSpec{
		Type: v1alpha1.WebhookDisrupt,
		Parameters: map[string]string{
			"webhookName": "my-mutating-webhook",
			"webhookType": "mutating",
			"action":      "setFailurePolicy",
			"value":       "Ignore",
		},
	}

	ctx := context.Background()

	// Inject
	cleanup, events, err := injector.Inject(ctx, spec, "default")
	require.NoError(t, err)
	assert.NotEmpty(t, events)
	assert.Equal(t, "mutating", events[0].Details["webhookType"])

	// Verify the webhook was modified
	modified := &admissionv1.MutatingWebhookConfiguration{}
	require.NoError(t, fakeClient.Get(ctx, client.ObjectKey{Name: "my-mutating-webhook"}, modified))
	require.NotNil(t, modified.Webhooks[0].FailurePolicy)
	assert.Equal(t, admissionv1.Ignore, *modified.Webhooks[0].FailurePolicy)

	// Cleanup should restore
	require.NoError(t, cleanup(ctx))
	restored := &admissionv1.MutatingWebhookConfiguration{}
	require.NoError(t, fakeClient.Get(ctx, client.ObjectKey{Name: "my-mutating-webhook"}, restored))
	require.NotNil(t, restored.Webhooks[0].FailurePolicy)
	assert.Equal(t, admissionv1.Fail, *restored.Webhooks[0].FailurePolicy)
}

func TestWebhookDisruptValidateWebhookType(t *testing.T) {
	injector := NewWebhookDisruptInjector(nil)
	blast := v1alpha1.BlastRadiusSpec{MaxPodsAffected: 1}

	tests := []struct {
		name    string
		wtype   string
		wantErr bool
	}{
		{"valid validating", "validating", false},
		{"valid mutating", "mutating", false},
		{"omitted defaults to validating", "", false},
		{"invalid type", "unknown", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			params := map[string]string{
				"webhookName": "test-webhook",
				"action":      "setFailurePolicy",
			}
			if tt.wtype != "" {
				params["webhookType"] = tt.wtype
			}
			spec := v1alpha1.InjectionSpec{
				Type:       v1alpha1.WebhookDisrupt,
				Parameters: params,
			}
			err := injector.Validate(spec, blast)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), "webhookType")
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestWebhookDisruptValidateLabelSelector(t *testing.T) {
	injector := NewWebhookDisruptInjector(nil)
	blast := v1alpha1.BlastRadiusSpec{MaxPodsAffected: 1}

	tests := []struct {
		name    string
		params  map[string]string
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid label selector",
			params: map[string]string{
				"webhookLabelSelector": "olm.webhook-description-generate-name=my-webhook.io",
				"action":              "setFailurePolicy",
			},
		},
		{
			name: "both webhookName and webhookLabelSelector",
			params: map[string]string{
				"webhookName":          "my-webhook",
				"webhookLabelSelector": "app=test",
				"action":              "setFailurePolicy",
			},
			wantErr: true,
			errMsg:  "not both",
		},
		{
			name: "neither webhookName nor webhookLabelSelector",
			params: map[string]string{
				"action": "setFailurePolicy",
			},
			wantErr: true,
			errMsg:  "requires either",
		},
		{
			name: "invalid label selector syntax",
			params: map[string]string{
				"webhookLabelSelector": "!!!invalid",
				"action":              "setFailurePolicy",
			},
			wantErr: true,
			errMsg:  "invalid webhookLabelSelector",
		},
		{
			name: "empty label selector (matches everything)",
			params: map[string]string{
				"webhookLabelSelector": "",
				"action":              "setFailurePolicy",
			},
			wantErr: true,
			errMsg:  "requires either",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			spec := v1alpha1.InjectionSpec{
				Type:       v1alpha1.WebhookDisrupt,
				Parameters: tt.params,
			}
			err := injector.Validate(spec, blast)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestWebhookDisruptLabelSelectorInjectAndCleanup(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, admissionv1.AddToScheme(scheme))

	ignorePolicy := admissionv1.Ignore
	webhook := &admissionv1.ValidatingWebhookConfiguration{
		ObjectMeta: metav1.ObjectMeta{
			Name: "dashboard-acceleratorprofile-validator.opendatahub.io-wcd5w",
			Labels: map[string]string{
				"olm.webhook-description-generate-name": "dashboard-acceleratorprofile-validator.opendatahub.io",
				"olm.managed":                          "true",
			},
		},
		Webhooks: []admissionv1.ValidatingWebhook{
			{
				Name:                    "dashboard-acceleratorprofile-validator.opendatahub.io",
				FailurePolicy:           &ignorePolicy,
				ClientConfig:            admissionv1.WebhookClientConfig{URL: strPtr("https://example.com")},
				SideEffects:             sideEffectPtr(admissionv1.SideEffectClassNone),
				AdmissionReviewVersions: []string{"v1"},
			},
		},
	}

	fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(webhook).Build()
	injector := NewWebhookDisruptInjector(fakeClient)

	spec := v1alpha1.InjectionSpec{
		Type: v1alpha1.WebhookDisrupt,
		Parameters: map[string]string{
			"webhookLabelSelector": "olm.webhook-description-generate-name=dashboard-acceleratorprofile-validator.opendatahub.io",
			"action":              "setFailurePolicy",
			"value":               "Fail",
		},
	}

	ctx := context.Background()

	cleanup, events, err := injector.Inject(ctx, spec, "default")
	require.NoError(t, err)
	assert.NotEmpty(t, events)
	assert.Equal(t, "dashboard-acceleratorprofile-validator.opendatahub.io-wcd5w", events[0].Details["webhookName"])

	modified := &admissionv1.ValidatingWebhookConfiguration{}
	require.NoError(t, fakeClient.Get(ctx,
		client.ObjectKey{Name: "dashboard-acceleratorprofile-validator.opendatahub.io-wcd5w"}, modified))
	require.NotNil(t, modified.Webhooks[0].FailurePolicy)
	assert.Equal(t, admissionv1.Fail, *modified.Webhooks[0].FailurePolicy)

	require.NoError(t, cleanup(ctx))
	restored := &admissionv1.ValidatingWebhookConfiguration{}
	require.NoError(t, fakeClient.Get(ctx,
		client.ObjectKey{Name: "dashboard-acceleratorprofile-validator.opendatahub.io-wcd5w"}, restored))
	require.NotNil(t, restored.Webhooks[0].FailurePolicy)
	assert.Equal(t, admissionv1.Ignore, *restored.Webhooks[0].FailurePolicy)
}

func TestWebhookDisruptLabelSelectorNoMatch(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, admissionv1.AddToScheme(scheme))

	fakeClient := fake.NewClientBuilder().WithScheme(scheme).Build()
	injector := NewWebhookDisruptInjector(fakeClient)

	spec := v1alpha1.InjectionSpec{
		Type: v1alpha1.WebhookDisrupt,
		Parameters: map[string]string{
			"webhookLabelSelector": "olm.webhook-description-generate-name=nonexistent",
			"action":              "setFailurePolicy",
		},
	}

	_, _, err := injector.Inject(context.Background(), spec, "default")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no ValidatingWebhookConfiguration found")
}

func TestWebhookDisruptLabelSelectorMultipleMatches(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, admissionv1.AddToScheme(scheme))

	ignorePolicy := admissionv1.Ignore
	webhook1 := &admissionv1.ValidatingWebhookConfiguration{
		ObjectMeta: metav1.ObjectMeta{
			Name:   "webhook-one-abc12",
			Labels: map[string]string{"app": "dashboard"},
		},
		Webhooks: []admissionv1.ValidatingWebhook{
			{
				Name:                    "one.webhook.io",
				FailurePolicy:           &ignorePolicy,
				ClientConfig:            admissionv1.WebhookClientConfig{URL: strPtr("https://example.com")},
				SideEffects:             sideEffectPtr(admissionv1.SideEffectClassNone),
				AdmissionReviewVersions: []string{"v1"},
			},
		},
	}
	webhook2 := &admissionv1.ValidatingWebhookConfiguration{
		ObjectMeta: metav1.ObjectMeta{
			Name:   "webhook-two-def34",
			Labels: map[string]string{"app": "dashboard"},
		},
		Webhooks: []admissionv1.ValidatingWebhook{
			{
				Name:                    "two.webhook.io",
				FailurePolicy:           &ignorePolicy,
				ClientConfig:            admissionv1.WebhookClientConfig{URL: strPtr("https://example.com")},
				SideEffects:             sideEffectPtr(admissionv1.SideEffectClassNone),
				AdmissionReviewVersions: []string{"v1"},
			},
		},
	}

	fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(webhook1, webhook2).Build()
	injector := NewWebhookDisruptInjector(fakeClient)

	spec := v1alpha1.InjectionSpec{
		Type: v1alpha1.WebhookDisrupt,
		Parameters: map[string]string{
			"webhookLabelSelector": "app=dashboard",
			"action":              "setFailurePolicy",
		},
	}

	_, _, err := injector.Inject(context.Background(), spec, "default")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "must match exactly one")
}

func TestWebhookDisruptLabelSelectorMutating(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, admissionv1.AddToScheme(scheme))

	failPolicy := admissionv1.Fail
	webhook := &admissionv1.MutatingWebhookConfiguration{
		ObjectMeta: metav1.ObjectMeta{
			Name:   "mutate-webhook-xyz99",
			Labels: map[string]string{"component": "injector"},
		},
		Webhooks: []admissionv1.MutatingWebhook{
			{
				Name:                    "mutate.test.io",
				FailurePolicy:           &failPolicy,
				ClientConfig:            admissionv1.WebhookClientConfig{URL: strPtr("https://example.com")},
				SideEffects:             sideEffectPtr(admissionv1.SideEffectClassNone),
				AdmissionReviewVersions: []string{"v1"},
			},
		},
	}

	fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(webhook).Build()
	injector := NewWebhookDisruptInjector(fakeClient)

	spec := v1alpha1.InjectionSpec{
		Type: v1alpha1.WebhookDisrupt,
		Parameters: map[string]string{
			"webhookLabelSelector": "component=injector",
			"webhookType":         "mutating",
			"action":              "setFailurePolicy",
			"value":               "Ignore",
		},
	}

	ctx := context.Background()

	cleanup, events, err := injector.Inject(ctx, spec, "default")
	require.NoError(t, err)
	assert.Equal(t, "mutate-webhook-xyz99", events[0].Details["webhookName"])
	assert.Equal(t, "mutating", events[0].Details["webhookType"])

	modified := &admissionv1.MutatingWebhookConfiguration{}
	require.NoError(t, fakeClient.Get(ctx, client.ObjectKey{Name: "mutate-webhook-xyz99"}, modified))
	assert.Equal(t, admissionv1.Ignore, *modified.Webhooks[0].FailurePolicy)

	require.NoError(t, cleanup(ctx))
	restored := &admissionv1.MutatingWebhookConfiguration{}
	require.NoError(t, fakeClient.Get(ctx, client.ObjectKey{Name: "mutate-webhook-xyz99"}, restored))
	assert.Equal(t, admissionv1.Fail, *restored.Webhooks[0].FailurePolicy)
}

func TestWebhookDisruptLabelSelectorRevert(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, admissionv1.AddToScheme(scheme))

	ignorePolicy := admissionv1.Ignore
	webhook := &admissionv1.ValidatingWebhookConfiguration{
		ObjectMeta: metav1.ObjectMeta{
			Name:   "labeled-webhook-qrs78",
			Labels: map[string]string{"managed-by": "olm", "component": "validator"},
		},
		Webhooks: []admissionv1.ValidatingWebhook{
			{
				Name:                    "validate.test.io",
				FailurePolicy:           &ignorePolicy,
				ClientConfig:            admissionv1.WebhookClientConfig{URL: strPtr("https://example.com")},
				SideEffects:             sideEffectPtr(admissionv1.SideEffectClassNone),
				AdmissionReviewVersions: []string{"v1"},
			},
		},
	}

	fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(webhook).Build()
	injector := NewWebhookDisruptInjector(fakeClient)

	spec := v1alpha1.InjectionSpec{
		Type: v1alpha1.WebhookDisrupt,
		Parameters: map[string]string{
			"webhookLabelSelector": "component=validator,managed-by=olm",
			"action":              "setFailurePolicy",
			"value":               "Fail",
		},
	}

	ctx := context.Background()

	_, _, err := injector.Inject(ctx, spec, "default")
	require.NoError(t, err)

	err = injector.Revert(ctx, spec, "default")
	require.NoError(t, err)

	restored := &admissionv1.ValidatingWebhookConfiguration{}
	require.NoError(t, fakeClient.Get(ctx, client.ObjectKey{Name: "labeled-webhook-qrs78"}, restored))
	assert.Equal(t, admissionv1.Ignore, *restored.Webhooks[0].FailurePolicy)
}

func TestWebhookDisruptLabelSelectorRevertDeletedWebhook(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, admissionv1.AddToScheme(scheme))

	fakeClient := fake.NewClientBuilder().WithScheme(scheme).Build()
	injector := NewWebhookDisruptInjector(fakeClient)

	spec := v1alpha1.InjectionSpec{
		Type: v1alpha1.WebhookDisrupt,
		Parameters: map[string]string{
			"webhookLabelSelector": "olm.webhook-description-generate-name=gone-webhook.io",
			"action":              "setFailurePolicy",
		},
	}

	err := injector.Revert(context.Background(), spec, "default")
	assert.NoError(t, err, "revert should succeed gracefully when label selector finds no match (webhook already deleted)")
}

func TestWebhookDisruptNameRevertDeletedWebhook(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, admissionv1.AddToScheme(scheme))

	fakeClient := fake.NewClientBuilder().WithScheme(scheme).Build()
	injector := NewWebhookDisruptInjector(fakeClient)

	spec := v1alpha1.InjectionSpec{
		Type: v1alpha1.WebhookDisrupt,
		Parameters: map[string]string{
			"webhookName": "already-gone",
			"action":      "setFailurePolicy",
		},
	}

	err := injector.Revert(context.Background(), spec, "default")
	assert.NoError(t, err, "name-based revert should also handle deleted webhook gracefully")
}

func TestWebhookDisruptLabelSelectorDenyListEnforced(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, admissionv1.AddToScheme(scheme))

	ignorePolicy := admissionv1.Ignore
	webhook := &admissionv1.ValidatingWebhookConfiguration{
		ObjectMeta: metav1.ObjectMeta{
			Name:   "cert-manager-webhook",
			Labels: map[string]string{"app": "cert-manager"},
		},
		Webhooks: []admissionv1.ValidatingWebhook{
			{
				Name:                    "webhook.cert-manager.io",
				FailurePolicy:           &ignorePolicy,
				ClientConfig:            admissionv1.WebhookClientConfig{URL: strPtr("https://example.com")},
				SideEffects:             sideEffectPtr(admissionv1.SideEffectClassNone),
				AdmissionReviewVersions: []string{"v1"},
			},
		},
	}

	fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(webhook).Build()
	injector := NewWebhookDisruptInjector(fakeClient)

	spec := v1alpha1.InjectionSpec{
		Type: v1alpha1.WebhookDisrupt,
		Parameters: map[string]string{
			"webhookLabelSelector": "app=cert-manager",
			"action":              "setFailurePolicy",
		},
	}

	_, _, err := injector.Inject(context.Background(), spec, "default")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "system-critical")
}

func TestWebhookDisruptLabelSelectorSystemPrefixDenyList(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, admissionv1.AddToScheme(scheme))

	ignorePolicy := admissionv1.Ignore
	webhook := &admissionv1.ValidatingWebhookConfiguration{
		ObjectMeta: metav1.ObjectMeta{
			Name:   "openshift-machine-api-webhook",
			Labels: map[string]string{"component": "machine-api"},
		},
		Webhooks: []admissionv1.ValidatingWebhook{
			{
				Name:                    "machine.openshift.io",
				FailurePolicy:           &ignorePolicy,
				ClientConfig:            admissionv1.WebhookClientConfig{URL: strPtr("https://example.com")},
				SideEffects:             sideEffectPtr(admissionv1.SideEffectClassNone),
				AdmissionReviewVersions: []string{"v1"},
			},
		},
	}

	fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(webhook).Build()
	injector := NewWebhookDisruptInjector(fakeClient)

	spec := v1alpha1.InjectionSpec{
		Type: v1alpha1.WebhookDisrupt,
		Parameters: map[string]string{
			"webhookLabelSelector": "component=machine-api",
			"action":              "setFailurePolicy",
		},
	}

	_, _, err := injector.Inject(context.Background(), spec, "default")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "OpenShift webhook")
}

func TestWebhookDisruptLabelSelectorSystemColonPrefixDenyList(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, admissionv1.AddToScheme(scheme))

	ignorePolicy := admissionv1.Ignore
	webhook := &admissionv1.ValidatingWebhookConfiguration{
		ObjectMeta: metav1.ObjectMeta{
			Name:   "system:kube-scheduler-webhook",
			Labels: map[string]string{"component": "scheduler"},
		},
		Webhooks: []admissionv1.ValidatingWebhook{
			{
				Name:                    "scheduler.k8s.io",
				FailurePolicy:           &ignorePolicy,
				ClientConfig:            admissionv1.WebhookClientConfig{URL: strPtr("https://example.com")},
				SideEffects:             sideEffectPtr(admissionv1.SideEffectClassNone),
				AdmissionReviewVersions: []string{"v1"},
			},
		},
	}

	fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(webhook).Build()
	injector := NewWebhookDisruptInjector(fakeClient)

	spec := v1alpha1.InjectionSpec{
		Type: v1alpha1.WebhookDisrupt,
		Parameters: map[string]string{
			"webhookLabelSelector": "component=scheduler",
			"action":              "setFailurePolicy",
		},
	}

	_, _, err := injector.Inject(context.Background(), spec, "default")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "system webhook")
}

func TestWebhookDisruptLabelSelectorSetBasedRequirement(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, admissionv1.AddToScheme(scheme))

	ignorePolicy := admissionv1.Ignore
	webhook := &admissionv1.ValidatingWebhookConfiguration{
		ObjectMeta: metav1.ObjectMeta{
			Name:   "target-webhook-abc12",
			Labels: map[string]string{"tier": "operator", "env": "prod"},
		},
		Webhooks: []admissionv1.ValidatingWebhook{
			{
				Name:                    "validate.target.io",
				FailurePolicy:           &ignorePolicy,
				ClientConfig:            admissionv1.WebhookClientConfig{URL: strPtr("https://example.com")},
				SideEffects:             sideEffectPtr(admissionv1.SideEffectClassNone),
				AdmissionReviewVersions: []string{"v1"},
			},
		},
	}
	nonMatchWebhook := &admissionv1.ValidatingWebhookConfiguration{
		ObjectMeta: metav1.ObjectMeta{
			Name:   "other-webhook-def34",
			Labels: map[string]string{"tier": "infra", "env": "prod"},
		},
		Webhooks: []admissionv1.ValidatingWebhook{
			{
				Name:                    "validate.other.io",
				FailurePolicy:           &ignorePolicy,
				ClientConfig:            admissionv1.WebhookClientConfig{URL: strPtr("https://example.com")},
				SideEffects:             sideEffectPtr(admissionv1.SideEffectClassNone),
				AdmissionReviewVersions: []string{"v1"},
			},
		},
	}

	fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(webhook, nonMatchWebhook).Build()
	injector := NewWebhookDisruptInjector(fakeClient)

	spec := v1alpha1.InjectionSpec{
		Type: v1alpha1.WebhookDisrupt,
		Parameters: map[string]string{
			"webhookLabelSelector": "tier in (operator)",
			"action":              "setFailurePolicy",
			"value":               "Fail",
		},
	}

	ctx := context.Background()
	cleanup, events, err := injector.Inject(ctx, spec, "default")
	require.NoError(t, err)
	assert.Equal(t, "target-webhook-abc12", events[0].Details["webhookName"])

	modified := &admissionv1.ValidatingWebhookConfiguration{}
	require.NoError(t, fakeClient.Get(ctx, client.ObjectKey{Name: "target-webhook-abc12"}, modified))
	assert.Equal(t, admissionv1.Fail, *modified.Webhooks[0].FailurePolicy)

	unchanged := &admissionv1.ValidatingWebhookConfiguration{}
	require.NoError(t, fakeClient.Get(ctx, client.ObjectKey{Name: "other-webhook-def34"}, unchanged))
	assert.Equal(t, admissionv1.Ignore, *unchanged.Webhooks[0].FailurePolicy, "non-matching webhook should be untouched")

	require.NoError(t, cleanup(ctx))
}

func TestWebhookDisruptNilFailurePolicyDefaultsPreserved(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, admissionv1.AddToScheme(scheme))

	webhook := &admissionv1.ValidatingWebhookConfiguration{
		ObjectMeta: metav1.ObjectMeta{
			Name:   "nil-policy-webhook-xyz99",
			Labels: map[string]string{"app": "test-nil"},
		},
		Webhooks: []admissionv1.ValidatingWebhook{
			{
				Name:                    "nil-policy.webhook.io",
				FailurePolicy:           nil,
				ClientConfig:            admissionv1.WebhookClientConfig{URL: strPtr("https://example.com")},
				SideEffects:             sideEffectPtr(admissionv1.SideEffectClassNone),
				AdmissionReviewVersions: []string{"v1"},
			},
		},
	}

	fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(webhook).Build()
	injector := NewWebhookDisruptInjector(fakeClient)

	spec := v1alpha1.InjectionSpec{
		Type: v1alpha1.WebhookDisrupt,
		Parameters: map[string]string{
			"webhookLabelSelector": "app=test-nil",
			"action":              "setFailurePolicy",
			"value":               "Fail",
		},
	}

	ctx := context.Background()

	cleanup, _, err := injector.Inject(ctx, spec, "default")
	require.NoError(t, err)

	modified := &admissionv1.ValidatingWebhookConfiguration{}
	require.NoError(t, fakeClient.Get(ctx, client.ObjectKey{Name: "nil-policy-webhook-xyz99"}, modified))
	require.NotNil(t, modified.Webhooks[0].FailurePolicy)
	assert.Equal(t, admissionv1.Fail, *modified.Webhooks[0].FailurePolicy)

	require.NoError(t, cleanup(ctx))
	restored := &admissionv1.ValidatingWebhookConfiguration{}
	require.NoError(t, fakeClient.Get(ctx, client.ObjectKey{Name: "nil-policy-webhook-xyz99"}, restored))
	assert.Nil(t, restored.Webhooks[0].FailurePolicy, "nil FailurePolicy should be restored to nil, not set to a value")
}

func TestWebhookDisruptLabelSelectorMixedPolicies(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, admissionv1.AddToScheme(scheme))

	ignorePolicy := admissionv1.Ignore
	failPolicy := admissionv1.Fail
	webhook := &admissionv1.ValidatingWebhookConfiguration{
		ObjectMeta: metav1.ObjectMeta{
			Name:   "mixed-policy-webhook",
			Labels: map[string]string{"app": "mixed"},
		},
		Webhooks: []admissionv1.ValidatingWebhook{
			{
				Name:                    "first.webhook.io",
				FailurePolicy:           &ignorePolicy,
				ClientConfig:            admissionv1.WebhookClientConfig{URL: strPtr("https://example.com/1")},
				SideEffects:             sideEffectPtr(admissionv1.SideEffectClassNone),
				AdmissionReviewVersions: []string{"v1"},
			},
			{
				Name:                    "second.webhook.io",
				FailurePolicy:           &failPolicy,
				ClientConfig:            admissionv1.WebhookClientConfig{URL: strPtr("https://example.com/2")},
				SideEffects:             sideEffectPtr(admissionv1.SideEffectClassNone),
				AdmissionReviewVersions: []string{"v1"},
			},
			{
				Name:                    "third.webhook.io",
				FailurePolicy:           nil,
				ClientConfig:            admissionv1.WebhookClientConfig{URL: strPtr("https://example.com/3")},
				SideEffects:             sideEffectPtr(admissionv1.SideEffectClassNone),
				AdmissionReviewVersions: []string{"v1"},
			},
		},
	}

	fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(webhook).Build()
	injector := NewWebhookDisruptInjector(fakeClient)

	spec := v1alpha1.InjectionSpec{
		Type: v1alpha1.WebhookDisrupt,
		Parameters: map[string]string{
			"webhookLabelSelector": "app=mixed",
			"action":              "setFailurePolicy",
			"value":               "Fail",
		},
	}

	ctx := context.Background()

	cleanup, events, err := injector.Inject(ctx, spec, "default")
	require.NoError(t, err)
	assert.Equal(t, "3", events[0].Details["webhookCount"])

	modified := &admissionv1.ValidatingWebhookConfiguration{}
	require.NoError(t, fakeClient.Get(ctx, client.ObjectKey{Name: "mixed-policy-webhook"}, modified))
	for i, wh := range modified.Webhooks {
		require.NotNil(t, wh.FailurePolicy, "webhook %d (%s) should have Fail policy after inject", i, wh.Name)
		assert.Equal(t, admissionv1.Fail, *wh.FailurePolicy)
	}

	require.NoError(t, cleanup(ctx))
	restored := &admissionv1.ValidatingWebhookConfiguration{}
	require.NoError(t, fakeClient.Get(ctx, client.ObjectKey{Name: "mixed-policy-webhook"}, restored))

	require.NotNil(t, restored.Webhooks[0].FailurePolicy)
	assert.Equal(t, admissionv1.Ignore, *restored.Webhooks[0].FailurePolicy, "first webhook should restore to Ignore")
	require.NotNil(t, restored.Webhooks[1].FailurePolicy)
	assert.Equal(t, admissionv1.Fail, *restored.Webhooks[1].FailurePolicy, "second webhook should restore to Fail")
	assert.Nil(t, restored.Webhooks[2].FailurePolicy, "third webhook should restore to nil")
}

func TestWebhookDisruptLabelSelectorRevertDeletedMutating(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, admissionv1.AddToScheme(scheme))

	fakeClient := fake.NewClientBuilder().WithScheme(scheme).Build()
	injector := NewWebhookDisruptInjector(fakeClient)

	spec := v1alpha1.InjectionSpec{
		Type: v1alpha1.WebhookDisrupt,
		Parameters: map[string]string{
			"webhookLabelSelector": "component=gone-mutating",
			"webhookType":         "mutating",
			"action":              "setFailurePolicy",
		},
	}

	err := injector.Revert(context.Background(), spec, "default")
	assert.NoError(t, err, "mutating revert should succeed gracefully when webhook is already deleted")
}

func TestWebhookDisruptCleanupUsesResolvedName(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, admissionv1.AddToScheme(scheme))

	ignorePolicy := admissionv1.Ignore
	webhook := &admissionv1.ValidatingWebhookConfiguration{
		ObjectMeta: metav1.ObjectMeta{
			Name:   "ephemeral-webhook-abc12",
			Labels: map[string]string{"app": "ephemeral"},
		},
		Webhooks: []admissionv1.ValidatingWebhook{
			{
				Name:                    "validate.ephemeral.io",
				FailurePolicy:           &ignorePolicy,
				ClientConfig:            admissionv1.WebhookClientConfig{URL: strPtr("https://example.com")},
				SideEffects:             sideEffectPtr(admissionv1.SideEffectClassNone),
				AdmissionReviewVersions: []string{"v1"},
			},
		},
	}

	fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(webhook).Build()
	injector := NewWebhookDisruptInjector(fakeClient)

	spec := v1alpha1.InjectionSpec{
		Type: v1alpha1.WebhookDisrupt,
		Parameters: map[string]string{
			"webhookLabelSelector": "app=ephemeral",
			"action":              "setFailurePolicy",
			"value":               "Fail",
		},
	}

	ctx := context.Background()

	cleanup, _, err := injector.Inject(ctx, spec, "default")
	require.NoError(t, err)

	// Remove the label so re-resolution would fail
	modified := &admissionv1.ValidatingWebhookConfiguration{}
	require.NoError(t, fakeClient.Get(ctx, client.ObjectKey{Name: "ephemeral-webhook-abc12"}, modified))
	delete(modified.Labels, "app")
	require.NoError(t, fakeClient.Update(ctx, modified))

	// Cleanup should still work because it captured the resolved name
	err = cleanup(ctx)
	assert.NoError(t, err, "cleanup func should use captured name, not re-resolve via labels")

	restored := &admissionv1.ValidatingWebhookConfiguration{}
	require.NoError(t, fakeClient.Get(ctx, client.ObjectKey{Name: "ephemeral-webhook-abc12"}, restored))
	assert.Equal(t, admissionv1.Ignore, *restored.Webhooks[0].FailurePolicy)
}

func TestWebhookDisruptValidateLabelSelectorSetBased(t *testing.T) {
	injector := NewWebhookDisruptInjector(nil)
	blast := v1alpha1.BlastRadiusSpec{MaxPodsAffected: 1}

	tests := []struct {
		name     string
		selector string
		wantErr  bool
	}{
		{"in operator", "tier in (operator, infra)", false},
		{"notin operator", "tier notin (test)", false},
		{"exists operator", "app", false},
		{"not-exists operator", "!debug", false},
		{"combined operators", "tier in (operator),env=prod", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			spec := v1alpha1.InjectionSpec{
				Type: v1alpha1.WebhookDisrupt,
				Parameters: map[string]string{
					"webhookLabelSelector": tt.selector,
					"action":              "setFailurePolicy",
				},
			}
			err := injector.Validate(spec, blast)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestWebhookDisruptDoubleInjectBlocked(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, admissionv1.AddToScheme(scheme))

	ignorePolicy := admissionv1.Ignore
	webhook := &admissionv1.ValidatingWebhookConfiguration{
		ObjectMeta: metav1.ObjectMeta{Name: "double-inject-webhook"},
		Webhooks: []admissionv1.ValidatingWebhook{
			{
				Name:                    "test.webhook.io",
				FailurePolicy:           &ignorePolicy,
				ClientConfig:            admissionv1.WebhookClientConfig{URL: strPtr("https://example.com")},
				SideEffects:             sideEffectPtr(admissionv1.SideEffectClassNone),
				AdmissionReviewVersions: []string{"v1"},
			},
		},
	}

	fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(webhook).Build()
	injector := NewWebhookDisruptInjector(fakeClient)

	spec := v1alpha1.InjectionSpec{
		Type: v1alpha1.WebhookDisrupt,
		Parameters: map[string]string{
			"webhookName": "double-inject-webhook",
			"action":      "setFailurePolicy",
			"value":       "Fail",
		},
	}

	ctx := context.Background()

	_, _, err := injector.Inject(ctx, spec, "default")
	require.NoError(t, err)

	_, _, err = injector.Inject(ctx, spec, "default")
	assert.Error(t, err, "second inject should be blocked")
	assert.Contains(t, err.Error(), "already has a chaos rollback annotation")
}

func TestWebhookDisruptDoubleInjectMutatingBlocked(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, admissionv1.AddToScheme(scheme))

	failPolicy := admissionv1.Fail
	webhook := &admissionv1.MutatingWebhookConfiguration{
		ObjectMeta: metav1.ObjectMeta{Name: "double-mutating"},
		Webhooks: []admissionv1.MutatingWebhook{
			{
				Name:                    "mutate.test.io",
				FailurePolicy:           &failPolicy,
				ClientConfig:            admissionv1.WebhookClientConfig{URL: strPtr("https://example.com")},
				SideEffects:             sideEffectPtr(admissionv1.SideEffectClassNone),
				AdmissionReviewVersions: []string{"v1"},
			},
		},
	}

	fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(webhook).Build()
	injector := NewWebhookDisruptInjector(fakeClient)

	spec := v1alpha1.InjectionSpec{
		Type: v1alpha1.WebhookDisrupt,
		Parameters: map[string]string{
			"webhookName": "double-mutating",
			"webhookType": "mutating",
			"action":      "setFailurePolicy",
			"value":       "Ignore",
		},
	}

	ctx := context.Background()

	_, _, err := injector.Inject(ctx, spec, "default")
	require.NoError(t, err)

	_, _, err = injector.Inject(ctx, spec, "default")
	assert.Error(t, err, "second mutating inject should be blocked")
	assert.Contains(t, err.Error(), "already has a chaos rollback annotation")
}

func TestWebhookDisruptInjectInvalidValueParam(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, admissionv1.AddToScheme(scheme))

	ignorePolicy := admissionv1.Ignore
	webhook := &admissionv1.ValidatingWebhookConfiguration{
		ObjectMeta: metav1.ObjectMeta{Name: "value-test-webhook"},
		Webhooks: []admissionv1.ValidatingWebhook{
			{
				Name:                    "test.webhook.io",
				FailurePolicy:           &ignorePolicy,
				ClientConfig:            admissionv1.WebhookClientConfig{URL: strPtr("https://example.com")},
				SideEffects:             sideEffectPtr(admissionv1.SideEffectClassNone),
				AdmissionReviewVersions: []string{"v1"},
			},
		},
	}

	fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(webhook).Build()
	injector := NewWebhookDisruptInjector(fakeClient)

	spec := v1alpha1.InjectionSpec{
		Type: v1alpha1.WebhookDisrupt,
		Parameters: map[string]string{
			"webhookName": "value-test-webhook",
			"action":      "setFailurePolicy",
			"value":       "Unknown",
		},
	}

	_, _, err := injector.Inject(context.Background(), spec, "default")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid failure policy")
}

func TestWebhookDisruptInjectEmptyWebhooksArray(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, admissionv1.AddToScheme(scheme))

	webhook := &admissionv1.ValidatingWebhookConfiguration{
		ObjectMeta: metav1.ObjectMeta{Name: "empty-webhooks"},
		Webhooks:   []admissionv1.ValidatingWebhook{},
	}

	fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(webhook).Build()
	injector := NewWebhookDisruptInjector(fakeClient)

	spec := v1alpha1.InjectionSpec{
		Type: v1alpha1.WebhookDisrupt,
		Parameters: map[string]string{
			"webhookName": "empty-webhooks",
			"action":      "setFailurePolicy",
			"value":       "Fail",
		},
	}

	ctx := context.Background()

	cleanup, events, err := injector.Inject(ctx, spec, "default")
	require.NoError(t, err)
	assert.Equal(t, "0", events[0].Details["webhookCount"])

	require.NoError(t, cleanup(ctx))
}

func TestWebhookDisruptLabelSelectorMultipleMatchesMutating(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, admissionv1.AddToScheme(scheme))

	failPolicy := admissionv1.Fail
	w1 := &admissionv1.MutatingWebhookConfiguration{
		ObjectMeta: metav1.ObjectMeta{
			Name:   "mutating-one",
			Labels: map[string]string{"tier": "operator"},
		},
		Webhooks: []admissionv1.MutatingWebhook{
			{
				Name:                    "one.mutate.io",
				FailurePolicy:           &failPolicy,
				ClientConfig:            admissionv1.WebhookClientConfig{URL: strPtr("https://example.com")},
				SideEffects:             sideEffectPtr(admissionv1.SideEffectClassNone),
				AdmissionReviewVersions: []string{"v1"},
			},
		},
	}
	w2 := &admissionv1.MutatingWebhookConfiguration{
		ObjectMeta: metav1.ObjectMeta{
			Name:   "mutating-two",
			Labels: map[string]string{"tier": "operator"},
		},
		Webhooks: []admissionv1.MutatingWebhook{
			{
				Name:                    "two.mutate.io",
				FailurePolicy:           &failPolicy,
				ClientConfig:            admissionv1.WebhookClientConfig{URL: strPtr("https://example.com")},
				SideEffects:             sideEffectPtr(admissionv1.SideEffectClassNone),
				AdmissionReviewVersions: []string{"v1"},
			},
		},
	}

	fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(w1, w2).Build()
	injector := NewWebhookDisruptInjector(fakeClient)

	spec := v1alpha1.InjectionSpec{
		Type: v1alpha1.WebhookDisrupt,
		Parameters: map[string]string{
			"webhookLabelSelector": "tier=operator",
			"webhookType":         "mutating",
			"action":              "setFailurePolicy",
		},
	}

	_, _, err := injector.Inject(context.Background(), spec, "default")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "must match exactly one")
}

func TestWebhookDisruptLabelSelectorNoMatchMutatingInject(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, admissionv1.AddToScheme(scheme))

	fakeClient := fake.NewClientBuilder().WithScheme(scheme).Build()
	injector := NewWebhookDisruptInjector(fakeClient)

	spec := v1alpha1.InjectionSpec{
		Type: v1alpha1.WebhookDisrupt,
		Parameters: map[string]string{
			"webhookLabelSelector": "component=nonexistent",
			"webhookType":         "mutating",
			"action":              "setFailurePolicy",
		},
	}

	_, _, err := injector.Inject(context.Background(), spec, "default")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no MutatingWebhookConfiguration found")
}

func TestWebhookDisruptLabelSelectorRevertMultipleMatches(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, admissionv1.AddToScheme(scheme))

	ignorePolicy := admissionv1.Ignore
	w1 := &admissionv1.ValidatingWebhookConfiguration{
		ObjectMeta: metav1.ObjectMeta{
			Name:   "revert-multi-one",
			Labels: map[string]string{"app": "ambiguous"},
		},
		Webhooks: []admissionv1.ValidatingWebhook{
			{
				Name:                    "one.webhook.io",
				FailurePolicy:           &ignorePolicy,
				ClientConfig:            admissionv1.WebhookClientConfig{URL: strPtr("https://example.com")},
				SideEffects:             sideEffectPtr(admissionv1.SideEffectClassNone),
				AdmissionReviewVersions: []string{"v1"},
			},
		},
	}
	w2 := &admissionv1.ValidatingWebhookConfiguration{
		ObjectMeta: metav1.ObjectMeta{
			Name:   "revert-multi-two",
			Labels: map[string]string{"app": "ambiguous"},
		},
		Webhooks: []admissionv1.ValidatingWebhook{
			{
				Name:                    "two.webhook.io",
				FailurePolicy:           &ignorePolicy,
				ClientConfig:            admissionv1.WebhookClientConfig{URL: strPtr("https://example.com")},
				SideEffects:             sideEffectPtr(admissionv1.SideEffectClassNone),
				AdmissionReviewVersions: []string{"v1"},
			},
		},
	}

	fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(w1, w2).Build()
	injector := NewWebhookDisruptInjector(fakeClient)

	spec := v1alpha1.InjectionSpec{
		Type: v1alpha1.WebhookDisrupt,
		Parameters: map[string]string{
			"webhookLabelSelector": "app=ambiguous",
			"action":              "setFailurePolicy",
		},
	}

	err := injector.Revert(context.Background(), spec, "default")
	assert.Error(t, err, "revert should fail when label selector matches multiple webhooks")
	assert.Contains(t, err.Error(), "must match exactly one")
}

func TestWebhookDisruptLabelSelectorRevertIdempotent(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, admissionv1.AddToScheme(scheme))

	ignorePolicy := admissionv1.Ignore
	webhook := &admissionv1.ValidatingWebhookConfiguration{
		ObjectMeta: metav1.ObjectMeta{
			Name:   "idempotent-label-webhook",
			Labels: map[string]string{"app": "idempotent"},
		},
		Webhooks: []admissionv1.ValidatingWebhook{
			{
				Name:                    "test.webhook.io",
				FailurePolicy:           &ignorePolicy,
				ClientConfig:            admissionv1.WebhookClientConfig{URL: strPtr("https://example.com")},
				SideEffects:             sideEffectPtr(admissionv1.SideEffectClassNone),
				AdmissionReviewVersions: []string{"v1"},
			},
		},
	}

	fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(webhook).Build()
	injector := NewWebhookDisruptInjector(fakeClient)

	spec := v1alpha1.InjectionSpec{
		Type: v1alpha1.WebhookDisrupt,
		Parameters: map[string]string{
			"webhookLabelSelector": "app=idempotent",
			"action":              "setFailurePolicy",
			"value":               "Fail",
		},
	}

	ctx := context.Background()

	_, _, err := injector.Inject(ctx, spec, "default")
	require.NoError(t, err)

	err = injector.Revert(ctx, spec, "default")
	require.NoError(t, err)

	err = injector.Revert(ctx, spec, "default")
	assert.NoError(t, err, "second revert via label selector should be idempotent (no rollback annotation)")

	restored := &admissionv1.ValidatingWebhookConfiguration{}
	require.NoError(t, fakeClient.Get(ctx, client.ObjectKey{Name: "idempotent-label-webhook"}, restored))
	assert.Equal(t, admissionv1.Ignore, *restored.Webhooks[0].FailurePolicy)
}

func strPtr(s string) *string { return &s }

func sideEffectPtr(se admissionv1.SideEffectClass) *admissionv1.SideEffectClass { return &se }
