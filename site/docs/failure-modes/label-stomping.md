# LabelStomping

**Danger Level:** :material-shield-alert: Medium

Modifies or removes labels on operator-managed resources to test label-based reconciliation.

## Spec Fields

| Field | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| `apiVersion` | `string` | Yes | - | API version of the target resource (e.g., apps/v1) |
| `kind` | `string` | Yes | - | Kind of the target resource (e.g., Deployment) |
| `name` | `string` | Yes | - | Name of the target resource instance |
| `labelKey` | `string` | Yes | - | Label key to modify or delete |
| `action` | `string` | Yes | - | Action to perform: 'overwrite' (set a new value) or 'delete' (remove the label) |
| `newValue` | `string` | No | `chaos-stomped` | Value to set when action is 'overwrite' |

## How It Works

LabelStomping uses the Unstructured client to read the target resource, records the current label value in a rollback annotation (with SHA-256 integrity checksum), then applies a JSON merge patch to overwrite or remove the specified label.

**API calls:**
1. `Get` the target resource as Unstructured
2. Read the current label value, store rollback data in annotation via `safety.WrapRollbackData`
3. `Patch` the resource with the new label value (overwrite) or `null` (delete via merge patch), plus chaos labels
4. On cleanup: restore original label value from rollback annotation, remove chaos metadata

**Cleanup:** Re-fetches the resource, restores the original label value (or removes it if it didn't exist before), and removes chaos labels and rollback annotation. Idempotent.

**Crash safety:** Rollback annotation persists on the resource with SHA-256 checksum. `Revert` reads the annotation to restore state even after a process crash.

## Safety Rules

- **Chaos-owned labels are rejected:** `app.kubernetes.io/managed-by` and any label prefixed with `chaos.operatorchaos.io/` cannot be targeted (prevents rollback corruption)
- **System labels require high danger:** Labels matching `kubernetes.io/`, `k8s.io/`, or `node-role.kubernetes.io/` patterns require `dangerLevel: high`
- **Label key/value validation:** Keys and values are validated against Kubernetes label format rules (max 63 char name, optional DNS prefix, alphanumeric with `._-`)
- **Delete non-existent label rejected:** Attempting to delete a label that doesn't exist returns an error (no-op rejection)

## Disruption Rubric

**Expected behavior on a healthy operator:**
The operator detects that a label on its managed resource has been modified or removed, and restores it to the expected value during the next reconciliation cycle. This validates that the operator's label-based selectors and reconciliation logic are working correctly.

**Contract violation indicators:**
- Operator does not detect the label change (indicates missing watch or label reconciliation)
- Operator does not restore the original label value (reconciler only checks spec, not metadata)
- Operator's label selectors break and it loses track of the resource
- Operator creates a duplicate resource because it can no longer find the original by label

**Collateral damage risks:**
- Medium. Only the target resource's label metadata is modified
- If the operator uses labels for service selectors, traffic routing may be disrupted during the test window
- System labels (kubernetes.io/) can affect scheduling, node affinity, and cluster behavior, hence the high danger requirement

**Recovery expectations:**
- Recovery time: 5-30 seconds (depends on reconciliation interval)
- Reconcile cycles: 1-2
- What "recovered" means: label restored to its original value

## Cross-Component Results

| Component | Experiment | Danger | Description |
|-----------|------------|--------|-------------|
| odh-model-controller | odh-model-controller-label-stomping | high | When a label used for resource discovery is overwritten on the odh-model-control... |

<!-- custom-start: notes -->
<!-- custom-end: notes -->
