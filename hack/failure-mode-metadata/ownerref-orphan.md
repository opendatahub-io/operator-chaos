---
name: OwnerRefOrphan
type: OwnerRefOrphan
danger: Medium
description: Removes ownerReferences from operator-managed resources to test re-adoption logic.
spec_fields:
  - name: apiVersion
    type: string
    required: true
    description: API version of the target resource (e.g., apps/v1)
  - name: kind
    type: string
    required: true
    description: Kind of the target resource (e.g., Deployment)
  - name: name
    type: string
    required: true
    description: Name of the target resource instance
  - name: ttl
    type: duration
    required: false
    default: "300s"
    description: Auto-cleanup duration
---

## How It Works

OwnerRefOrphan reads the target resource, saves its `ownerReferences` in a rollback annotation, then clears all ownerReferences via a JSON merge patch. This simulates a resource becoming "orphaned" from its parent controller.

**API calls:**
1. `Get` the target resource as Unstructured
2. Serialize original `ownerReferences` to rollback annotation
3. `Patch` the resource with empty `ownerReferences` array, add chaos labels
4. On cleanup: check if operator re-adopted, restore original ownerReferences only if still orphaned

**Cleanup:** Checks whether the operator has already re-adopted the resource (non-empty ownerReferences). If so, only removes chaos metadata. If still orphaned, restores the original ownerReferences from the rollback annotation. Idempotent.

**Crash safety:** Rollback annotation persists on the resource. `Revert` also checks for re-adoption before restoring.

## Disruption Rubric

**Expected behavior on a healthy operator:**
The parent controller detects that its child resource no longer has an ownerReference pointing back to it, and re-adopts the resource by adding a new ownerReference. The resource should never be garbage collected during the test window because the experiment uses a short TTL.

**Contract violation indicators:**
- Operator does not detect the orphaned resource (indicates missing watch or adoption logic)
- Resource is garbage collected because the operator relied solely on ownerReferences for lifecycle management
- Operator creates a duplicate resource instead of re-adopting the existing one
- Operator enters error loop trying to manage a resource it no longer owns

**Collateral damage risks:**
- Medium. Only the target resource's metadata is modified
- If the operator uses ownerReferences for cascading deletion, orphaning may prevent cleanup
- Protected kinds (Namespace, Node, ChaosExperiment) are rejected by validation

**Recovery expectations:**
- Recovery time: 5-60 seconds (depends on reconciliation interval)
- Reconcile cycles: 1-2
- What "recovered" means: resource has ownerReferences restored (either by operator or cleanup)
