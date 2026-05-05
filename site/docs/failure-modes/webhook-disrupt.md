# WebhookDisrupt

**Danger Level:** :material-shield-remove: High

Modifies failure policies on a ValidatingWebhookConfiguration or MutatingWebhookConfiguration to test webhook resilience. Supports both exact-name and label-based webhook discovery.

## Spec Fields

| Field | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| `webhookName` | `string` | One of `webhookName` or `webhookLabelSelector` | - | Exact name of the webhook configuration resource |
| `webhookLabelSelector` | `string` | One of `webhookName` or `webhookLabelSelector` | - | Label selector to discover the webhook configuration at runtime (must match exactly one) |
| `webhookType` | `string` | No | `validating` | Type of webhook configuration: `validating` or `mutating` |
| `value` | `string` | No | `Fail` | New failure policy: `Fail` or `Ignore` |
| `ttl` | `duration` | No | `300s` | Auto-cleanup duration |

`webhookName` and `webhookLabelSelector` are mutually exclusive. Exactly one must be specified.

## How It Works

WebhookDisrupt reads the target webhook configuration (ValidatingWebhookConfiguration or MutatingWebhookConfiguration), saves the original `failurePolicy` for each webhook entry, and sets all entries to the specified value. This is a cluster-scoped operation.

**Target resolution:**

- **By name:** `webhookName` directly references the webhook configuration resource.
- **By label:** `webhookLabelSelector` discovers the webhook configuration at runtime using a Kubernetes label selector. The selector must match exactly one configuration. This is useful when operators like OLM generate webhook configuration names with random suffixes (e.g., `dashboard-acceleratorprofile-validator.opendatahub.io-wcd5w`), making exact-name targeting unreliable. The stable identity is typically in labels like `olm.webhook-description-generate-name`.

**API calls:**
1. Resolve the target webhook name (direct or via label selector `List`)
2. Verify the target is not on the system-critical deny-list
3. `Get` the webhook configuration (cluster-scoped)
4. Check for existing rollback annotation (prevents double-inject)
5. Store original per-webhook failure policies in rollback annotation
6. `Update` all webhook entries with new `failurePolicy`
7. On cleanup: restore original per-webhook policies from rollback annotation

**Double-inject protection:** If a webhook configuration already has a chaos rollback annotation, injection is refused. This prevents overwriting the original failure policies with already-modified values, which would make revert restore to the wrong state. Revert the existing injection before re-injecting.

**Cleanup:** Restores each webhook's original `failurePolicy`. Idempotent (safe to call multiple times). When using label-based discovery, if the webhook configuration was deleted between inject and revert, cleanup returns successfully since there is nothing to restore. The cleanup function captures the resolved webhook name at inject time, so label changes after injection don't affect cleanup.

**Crash safety:** Rollback annotation persists on the resource. `Revert` restores original policies.

## Disruption Rubric

**Expected behavior on a healthy operator:**
Setting `failurePolicy: Ignore` means webhook validation is skipped. The operator should still function correctly because webhooks are a defense-in-depth mechanism, not a required dependency. Setting `failurePolicy: Fail` when the webhook service is unavailable blocks all matching API requests.

**Contract violation indicators:**
- Invalid resources are created when webhook is set to Ignore (indicates webhook is the only validation)
- Operator becomes completely non-functional when webhook policy changes (indicates tight coupling)
- Webhook configuration is not reconciled back by the operator

**Collateral damage risks:**
- **Very high.** This is cluster-scoped. ALL namespaces are affected.
- Setting webhooks to Ignore allows potentially invalid resources cluster-wide
- Setting webhooks to Fail when service is down blocks API operations cluster-wide
- Requires `dangerLevel: high` and `allowDangerous: true`

**Recovery expectations:**
- Recovery time: 1-10 seconds (operator reconciles webhook configuration)
- Reconcile cycles: 1
- What "recovered" means: webhook has original `failurePolicy` restored

## Cross-Component Results

| Component | Experiment | Danger | Description |
|-----------|------------|--------|-------------|
| dashboard | rhoai-dashboard-webhook-disrupt-acceleratorprofile | high | When the accelerator-profile ValidatingWebhookConfiguration failurePolicy is wea... |
| dashboard | rhoai-dashboard-webhook-disrupt-hardwareprofile | high | When the hardware-profile ValidatingWebhookConfiguration failurePolicy is weaken... |
| data-science-pipelines | data-science-pipelines-webhook-disrupt | high | When the pipeline version validating webhook failurePolicy is weakened from Fail... |
| kserve | kserve-isvc-validator-disrupt | high | When the ValidatingWebhookConfiguration for InferenceService has its failurePoli... |
| kueue | kueue-webhook-disrupt | high | When the kueue validating webhook failurePolicy is weakened from Fail to Ignore,... |
| model-registry | model-registry-webhook-disrupt | high | When the ModelRegistry validating webhook failurePolicy is weakened from Fail to... |
| modelmesh | modelmesh-webhook-disrupt | high | When the modelmesh ServingRuntime validating webhook failurePolicy is weakened f... |
| odh-model-controller | odh-model-controller-webhook-disrupt | high | When the validating webhook failurePolicy is weakened from Fail to Ignore, inval... |
| opendatahub-operator | opendatahub-operator-webhook-disrupt | high | When the validating webhook failurePolicy is weakened from Fail to Ignore, inval... |
| workbenches | workbenches-webhook-disrupt | high | When the notebook mutating webhook failurePolicy is weakened from Fail to Ignore... |

<!-- custom-start: notes -->
<!-- custom-end: notes -->
