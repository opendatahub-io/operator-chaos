# PDBBlock

**Danger Level:** :material-shield-remove: High

Creates a PodDisruptionBudget with maxUnavailable=0 to block all voluntary evictions.

## Spec Fields

| Field | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| `labelSelector` | `string` | Yes | - | Equality-based label selector to match target pods (e.g., `app=my-controller`) |
| `name` | `string` | No | auto-generated from label values | Name of the PDB to create (auto-generated as `chaos-pdb-<first-label-value>` if not specified) |
| `ttl` | `duration` | No | `300s` | Auto-cleanup duration |

## How It Works

PDBBlock creates a PodDisruptionBudget with `maxUnavailable: 0` targeting pods that match the specified label selector. This blocks all voluntary evictions, including node drain operations, cluster upgrades, and pod disruption from autoscalers. The PDB is labeled with `app.kubernetes.io/managed-by: operator-chaos` for safe identification and cleanup.

**API calls:**
1. Parse `labelSelector` into a label map (comma-separated `key=value` format)
2. Generate PDB name from label values if not provided (sorted by key, uses first value)
3. `Create` (or `Update`) the PDB with `maxUnavailable: 0` and `matchLabels` selector
4. On cleanup: `Delete` the PDB (only if it has the chaos-managed label)

**Cleanup:** Deletes the created PDB. Voluntary evictions resume immediately after deletion.

**Crash safety:** If the chaos tool crashes, the PDB persists and continues blocking evictions. Use `operator-chaos clean` to find and remove orphaned PDBs by the `managed-by` label.

## Disruption Rubric

**Expected behavior on a healthy operator:**
The operator's pods continue running normally. The PDB only affects voluntary evictions, not involuntary ones (OOM kills, node failures). A `kubectl drain` or cluster upgrade attempting to evict the matched pods will be blocked until the PDB is removed.

**Contract violation indicators:**
- Cluster upgrade hangs indefinitely on the node hosting the operator's pods (expected behavior, but the operator should handle graceful shutdown when eventually evicted)
- Operator does not set appropriate PDB policies for its own pods (indicates missing production-readiness configuration)
- After PDB removal, the operator's pods are not properly drained during the next maintenance window

**Collateral damage risks:**
- **Moderate.** The PDB only blocks voluntary evictions for matched pods. Other pods on the same node can still be drained.
- If the `labelSelector` matches pods across multiple Deployments, all of them are affected
- Cluster upgrades and node maintenance will be blocked until the PDB is removed
- If the operator already has its own PDB, the chaos PDB creates an additional constraint (the most restrictive PDB wins)

**Recovery expectations:**
- Recovery time: immediate after PDB deletion. No pod restarts needed.
- Reconcile cycles: 0 (the PDB does not affect reconciliation, only eviction)
- What "recovered" means: `kubectl drain` and cluster upgrades can proceed normally for the matched pods

## Cross-Component Results

| Component | Experiment | Danger | Description |
|-----------|------------|--------|-------------|
| odh-model-controller | odh-model-controller-pdb-block | high | Creating a PDB with maxUnavailable=0 blocks voluntary evictions of odh-model-controller pods. The operator continues running normally. |
| kserve | kserve-pdb-block | high | Creating a PDB with maxUnavailable=0 blocks voluntary evictions of kserve-controller-manager pods. The operator continues running normally. |
| knative-serving | knative-serving-controller-pdb-block | high | Creating a PDB with maxUnavailable=0 blocks voluntary evictions of the knative-serving controller pods. The operator continues running normally. |
| cert-manager | cert-manager-pdb-block | high | Creating a PDB with maxUnavailable=0 blocks voluntary evictions of cert-manager pods. The operator continues running normally. |
| service-mesh | istiod-pdb-block | high | Creating a PDB with maxUnavailable=0 blocks voluntary evictions of istiod pods. The operator continues running normally. |

<!-- custom-start: notes -->
<!-- custom-end: notes -->
