# SecretDeletion

**Danger Level:** :material-shield-remove: High

Deletes a Kubernetes Secret after backing it up, then restores it on revert.

## Spec Fields

| Field | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| `name` | `string` | Yes | - | Name of the Secret to delete |
| `namespace` | `string` | No | experiment namespace | Namespace of the target Secret (defaults to the experiment's namespace) |
| `ttl` | `duration` | No | `300s` | Auto-cleanup duration |

## How It Works

SecretDeletion uses the Kubernetes API to get a Secret, serialize its full contents to JSON, store the backup in a new Secret (named `chaos-backup-secret-<name>`), and then delete the original. The backup Secret is labeled with `app.kubernetes.io/managed-by: operator-chaos` for safe identification.

**API calls:**
1. `Get` the target Secret by name and namespace
2. `Create` (or `Update`) a backup Secret containing the serialized JSON
3. `Delete` the original Secret
4. On revert: `Create` the Secret from backup (clearing UID, resourceVersion, managedFields), then `Delete` the backup Secret

**Cleanup:** Restores the Secret from the backup. If the Secret already exists (recreated by an operator or prior revert), the restore is skipped and only the backup Secret is deleted.

**Crash safety:** If the chaos tool crashes after deletion, the backup Secret persists in the namespace. Use `operator-chaos clean` to find orphaned backups by the `managed-by` label and manually restore if needed.

**Safety checks:**

- System-critical Secrets are blocked by a deny-list (e.g., `pull-secret`, SA tokens)
- Secrets with system-critical prefixes are also blocked
- Protected namespaces (`kube-system`, `openshift-*`) are rejected
- The backup Secret name length is validated against the 253-character K8s limit

## Disruption Rubric

**Expected behavior on a healthy operator:**
The operator detects the missing Secret (via watches or reconciliation) and either recreates it from its desired state or enters a degraded mode with clear status reporting. Operators that depend on TLS certificates or registry credentials should either regenerate them or report the missing dependency.

**Contract violation indicators:**
- Operator crashes or enters CrashLoopBackOff when the Secret disappears (indicates no nil-check or missing error handling)
- Operator silently continues without the Secret but produces incorrect behavior (indicates missing dependency validation)
- Operator does not recreate or report the missing Secret within `recoveryTimeout`
- Stale Secret data is served from cache after deletion

**Collateral damage risks:**
- **High.** Deleting a TLS Secret can break webhook configurations, causing API server errors for the entire namespace
- Deleting registry pull secrets can prevent new pod scheduling cluster-wide
- If the Secret is shared across multiple operators, all consumers are affected
- cert-manager Secrets may trigger cascading certificate re-issuance

**Recovery expectations:**
- Recovery time: varies significantly by Secret type. TLS secrets managed by cert-manager typically regenerate within 30-60 seconds. Operator-created config secrets depend on reconciliation interval.
- Reconcile cycles: 1-2 (detection, recreation)
- What "recovered" means: Secret exists with correct data, and all dependent resources are functional

## Cross-Component Results

| Component | Experiment | Danger | Description |
|-----------|------------|--------|-------------|
| cert-manager | cert-manager-secret-deletion | high | Deleting a cert-manager-managed TLS Secret tests whether cert-manager detects the missing certificate and re-issues it. |

<!-- custom-start: notes -->
<!-- custom-end: notes -->
