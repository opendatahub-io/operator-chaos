# LeaderElectionDisrupt

**Danger Level:** :material-shield-remove: High

Deletes a Lease object to force leader re-election, then cleans up the backup on revert.

## Spec Fields

| Field | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| `name` | `string` | Yes | - | Name of the Lease to delete |
| `ttl` | `duration` | No | `300s` | Auto-cleanup duration |

## How It Works

LeaderElectionDisrupt gets the target Lease, serializes it to JSON, stores the backup in a ConfigMap (named `chaos-backup-lease-<name>`), and deletes the Lease. This forces all candidates in the leader election to compete for a new Lease, triggering re-election.

**API calls:**
1. `Get` the target Lease
2. `Create` (or `Update`) a backup ConfigMap with the serialized Lease JSON
3. `Delete` the Lease to trigger re-election
4. On revert: check if the Lease was recreated by the controller. If not, restore from backup. Then `Delete` the backup ConfigMap.

**Cleanup:** The revert intentionally does NOT restore a stale Lease if the controller has already re-elected. It only restores the Lease from backup if no new Lease exists, preventing conflicts with a legitimately re-elected leader. The backup ConfigMap is always cleaned up.

**Crash safety:** The backup ConfigMap persists after a crash. Use `operator-chaos clean` to find orphaned backups by the `managed-by` label. In most cases, the controller will have already re-created the Lease on its own.

## Disruption Rubric

**Expected behavior on a healthy operator:**
The controller detects the missing Lease within one lease renewal interval (typically 2-10 seconds). All candidates attempt to acquire a new Lease. One wins the election and resumes reconciliation. The brief gap in reconciliation should not cause data loss or corruption.

**Contract violation indicators:**
- No new Lease is created within 60 seconds (indicates broken leader election configuration)
- Multiple controllers believe they are leader simultaneously (split-brain, indicates missing fencing)
- Reconciliation does not resume after re-election (indicates the new leader failed to initialize)
- Controller enters CrashLoopBackOff after losing the Lease (indicates fatal error handling on Lease loss)

**Collateral damage risks:**
- Minimal for the Lease deletion itself. The disruption window is typically under 10 seconds.
- If the operator has in-flight writes when the Lease disappears, those writes may be duplicated by the new leader (at-least-once semantics)
- On single-replica Deployments, the same pod re-acquires the Lease, so the disruption is just the re-election delay

**Recovery expectations:**
- Recovery time: 2-15 seconds for most controllers using client-go leader election with default settings (LeaseDuration=15s, RenewDeadline=10s, RetryPeriod=2s)
- Reconcile cycles: 1 (new leader starts reconciliation from current state)
- What "recovered" means: a new Lease exists with a valid holder, and reconciliation is actively processing

## Cross-Component Results

| Component | Experiment | Danger | Description |
|-----------|------------|--------|-------------|
| odh-model-controller | odh-model-controller-leader-election-disrupt | high | Deleting the leader Lease forces re-election. The controller re-acquires the Lease and resumes reconciliation within seconds. |
| kserve | kserve-leader-election-disrupt | high | Deleting the kserve-controller-manager Lease forces re-election. The controller re-acquires the Lease and resumes reconciliation within seconds. |
| cert-manager | cert-manager-leader-election-disrupt | high | Deleting the cert-manager Lease forces re-election. The controller re-acquires the Lease and resumes reconciliation within seconds. |
| knative-serving | knative-serving-controller-leader-election-disrupt | high | Deleting the knative-serving controller Lease forces re-election. The controller re-acquires the Lease and resumes reconciliation within seconds. |

<!-- custom-start: notes -->
<!-- custom-end: notes -->
