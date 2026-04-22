---
name: NamespaceDeletion
type: NamespaceDeletion
danger: High
description: Deletes an entire namespace to test whether the operator recreates it and its managed resources.
spec_fields:
  - name: namespace
    type: string
    required: true
    description: Name of the namespace to delete
---

## How It Works

NamespaceDeletion snapshots the target namespace's labels and annotations into a rollback ConfigMap stored in the experiment's safe namespace, then deletes the target namespace entirely. This tests whether the operator detects the namespace loss and recreates both the namespace and all resources within it.

**API calls:**
1. `Get` the namespace object, serialize its labels and annotations
2. Count resources in the namespace (Deployments, Services, ConfigMaps, Pods) for event reporting
3. `Create` a rollback ConfigMap `chaos-rollback-ns-<namespace>` in the safe namespace with chaos labels
4. `Delete` the target namespace
5. On cleanup: recreate the namespace with stored labels and annotations, delete the rollback ConfigMap

**Cleanup:** Checks if the namespace already exists (operator may have recreated it). If in `Terminating` phase, returns an error asking to wait. If `Active`, just cleans up the rollback ConfigMap. If missing, recreates with stored metadata. Handles `AlreadyExists` race on Create (another process recreated between check and create).

**Crash safety:** Rollback ConfigMap persists in the safe namespace with chaos labels. `Revert` reads the ConfigMap to restore state even after a process crash. Stale ConfigMaps from prior crashed experiments are detected and updated in-place using optimistic concurrency (resourceVersion).

## Safety Rules

- **Forbidden namespaces:** `kube-system`, `default`, `kube-public`, `kube-node-lease` are hardcoded deny-listed
- **Forbidden prefixes:** Namespaces matching `openshift-*`, `chaos-*`, or `redhat-ods-*` prefixes are rejected
- **Controller namespace:** `odh-chaos-system` is always protected
- **Self-namespace guard:** Cannot target the experiment's own namespace (where rollback data is stored)
- **Always requires high danger:** `dangerLevel: high` is mandatory for all NamespaceDeletion experiments
- **Non-chaos ConfigMap protection:** Refuses to overwrite a pre-existing ConfigMap that isn't chaos-managed

## Disruption Rubric

**Expected behavior on a healthy operator:**
The operator detects that its target namespace has been deleted (via a watch on namespace events or by observing that managed resources are gone), recreates the namespace, and redeploys all managed resources within it. The reconciliation loop should fully restore the operator's intended state.

**Contract violation indicators:**
- Operator does not detect the namespace deletion (indicates missing namespace-level watch)
- Operator recreates the namespace but not the resources inside it (partial reconciliation)
- Operator enters a crash loop because it assumes the namespace always exists
- Operator recreates resources in a different namespace instead of the original one
- Resources are recreated without the original labels or annotations

**Collateral damage risks:**
- High. All resources in the namespace are destroyed (Deployments, Services, ConfigMaps, Secrets, Pods)
- Persistent data (PVCs) may be lost if the storage class has `reclaimPolicy: Delete`
- Other operators watching resources in the namespace will also be disrupted
- Network policies, service accounts, and RBAC bindings scoped to the namespace are lost

**Recovery expectations:**
- Recovery time: 30-300 seconds (depends on operator complexity and resource count)
- Reconcile cycles: 3-10 (namespace creation + resource recreation + readiness)
- What "recovered" means: namespace exists with original metadata, all managed resources are recreated and healthy
