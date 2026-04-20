---
name: WebhookLatency
type: WebhookLatency
danger: High
description: Deploys a slow admission webhook to add latency to API server requests for specific resources.
spec_fields:
  - name: resources
    type: string
    required: true
    description: "Comma-separated Kubernetes resource types to intercept (e.g., 'deployments', 'services')"
  - name: apiGroups
    type: string
    required: true
    description: "Comma-separated API groups (e.g., 'apps', '' for core)"
  - name: delay
    type: duration
    required: false
    default: "25s"
    description: "Latency to add per request (range: 1s-29s)"
  - name: ttl
    type: duration
    required: false
    default: "300s"
    description: Auto-cleanup duration
---

## How It Works

WebhookLatency deploys a complete admission webhook stack that intercepts API server requests and adds a configurable delay before responding. This tests whether operators handle slow API responses without hanging or timing out.

**Resources created:**
1. **TLS Secret**: Self-signed ECDSA P-256 certificate chain (CA + server cert with proper DNS SANs)
2. **Deployment**: Runs `agnhost webhook` server with the configured delay and TLS certs
3. **Service**: Exposes the webhook server on port 443
4. **ValidatingWebhookConfiguration**: Intercepts Create/Update operations on specified resource types with `failurePolicy: Ignore`

**API calls:**
1. Generate self-signed TLS certificate (CA + server cert)
2. `Create` TLS Secret, Deployment, Service, ValidatingWebhookConfiguration (in order)
3. On cleanup: `Delete` all four resources (webhook config first)

The webhook uses `failurePolicy: Ignore` so that if the webhook pod is not ready, API requests are not blocked. The `timeoutSeconds` is set to 30s to allow the full delay to complete.

**Cleanup:** Deletes all four resources in reverse order (webhook config, service, deployment, secret). Best-effort, continues if some resources are already gone.

**Crash safety:** `Revert` deletes all resources by name. Use `operator-chaos clean` for orphaned webhook deployments.

## Disruption Rubric

**Expected behavior on a healthy operator:**
The operator experiences slow API responses (25s per Create/Update call to the targeted resource types). It should use context timeouts, not hang indefinitely. Reconciliation may be slower but should still complete. The operator should not enter error loops or drop events.

**Contract violation indicators:**
- Operator hangs waiting for API responses (indicates missing context timeouts)
- Operator drops watch events during slow periods (indicates unbuffered event handling)
- Reconciliation queue backs up and never drains (indicates no parallelism or timeout)
- Operator logs show context deadline exceeded but does not retry (indicates missing retry logic)

**Collateral damage risks:**
- **Very high.** The webhook intercepts ALL Create/Update operations on the specified resource types cluster-wide
- All controllers operating on those resources are affected, not just the target operator
- Setting delay too close to 30s risks hitting the API server's webhook timeout
- Requires `dangerLevel: high` and `allowDangerous: true`
- Use narrow resource targeting (specific API group + resource) to limit blast radius

**Recovery expectations:**
- Recovery time: immediate after webhook removal (no persistent state change)
- Reconcile cycles: 0 (no state was mutated, just delayed)
- What "recovered" means: API response times return to normal, reconciliation queues drain
