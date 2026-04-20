---
name: QuotaExhaustion
type: QuotaExhaustion
danger: Medium
description: Creates a restrictive ResourceQuota to test operator behavior under resource pressure.
spec_fields:
  - name: quotaName
    type: string
    required: true
    description: Name for the ResourceQuota to create
  - name: cpu
    type: string
    required: false
    description: "CPU limit (e.g., '100m', '1')"
  - name: memory
    type: string
    required: false
    description: "Memory limit (e.g., '128Mi', '1Gi')"
  - name: pods
    type: string
    required: false
    description: "Maximum number of pods (e.g., '0', '5')"
  - name: ttl
    type: duration
    required: false
    default: "300s"
    description: Auto-cleanup duration
---

## How It Works

QuotaExhaustion creates a Kubernetes ResourceQuota with intentionally tight limits in the target namespace. This forces the operator to handle resource creation failures (pods, PVCs, etc.) that would normally succeed.

**API calls:**
1. Check if a quota with the given name already exists (reject if so)
2. Build a `ResourceList` from the provided parameters (cpu, memory, pods, etc.)
3. `Create` the ResourceQuota with chaos labels
4. On cleanup: `Delete` the ResourceQuota

**At least one resource limit parameter is required.** Setting `pods: "0"` is the most aggressive option, preventing any new pod creation in the namespace.

**Cleanup:** Deletes the ResourceQuota. Idempotent.

**Crash safety:** `Revert` checks for chaos labels before deleting, so it won't accidentally remove user-created quotas. Use `operator-chaos clean` for orphaned quotas.

## Disruption Rubric

**Expected behavior on a healthy operator:**
The operator attempts to create or scale resources and encounters quota errors (403 Forbidden). It should handle these errors gracefully: log the failure, set degraded status conditions on the CR, and retry with backoff. Once the quota is removed, the operator should resume normal operation.

**Contract violation indicators:**
- Operator crashes or panics on quota errors (indicates missing error handling for resource creation)
- Operator enters infinite tight loop retrying without backoff (indicates missing retry logic)
- Operator does not surface quota errors in CR status (indicates swallowed errors)
- Operator does not recover after quota is removed (indicates no retry mechanism)

**Collateral damage risks:**
- Medium to high. The quota affects ALL pod/resource creation in the namespace, not just the operator
- Other controllers and workloads in the same namespace are also restricted
- Setting `pods: "0"` blocks all new pod creation, including rollout restarts
- Use a dedicated test namespace when possible

**Recovery expectations:**
- Recovery time: immediate after quota removal (pending pods should be created)
- Reconcile cycles: 1-3 (detect quota removal, retry resource creation, verify)
- What "recovered" means: operator successfully creates resources that were previously blocked
