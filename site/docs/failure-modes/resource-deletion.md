# ResourceDeletion

**Danger Level:** :material-shield-remove: High

Deletes an arbitrary namespaced Kubernetes resource after backing it up, then restores it on revert if the operator has not recreated it.

## Spec Fields

| Field | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| `apiVersion` | `string` | Yes | - | API version of the resource (e.g., `v1`, `apps/v1`) |
| `kind` | `string` | Yes | - | Kind of the resource (e.g., `Service`, `ConfigMap`) |
| `name` | `string` | Yes | - | Name of the resource to delete |
| `namespace` | `string` | No | experiment namespace | Namespace of the target resource (defaults to the experiment's namespace) |
| `ttl` | `duration` | No | `300s` | Auto-cleanup duration |

## How It Works

ResourceDeletion uses an unstructured client to get any namespaced Kubernetes resource by apiVersion/kind/name, serialize it to JSON, store the backup in a Secret (named `chaos-backup-resource-<kind>-<name>`), and delete the original. On revert, it checks whether the operator has already recreated the resource. If not, it restores from backup with server-managed fields cleared.

**API calls:**
1. `Get` the resource using unstructured client
2. `Create` (or `Update`) a backup Secret containing the serialized JSON
3. `Delete` the resource
4. On revert: check if the resource exists. If recreated by the operator, skip restore. If not, `Create` from backup (clearing UID, resourceVersion, managedFields, status, finalizers, generation). Then `Delete` the backup Secret.

**Cleanup:** If the operator recreated the resource, only the backup Secret is cleaned up. If the operator did not recreate it, the resource is restored from backup with all server-managed fields cleared to allow a clean creation.

**Crash safety:** The backup Secret persists after a crash. Use `operator-chaos clean` to find orphaned backups by the `managed-by` label and manually restore if needed.

**Kind-specific safety checks:**

- **Secret:** deny-list blocks system-critical Secrets (pull-secret, SA tokens, system-critical prefixes)
- **Service:** deny-list blocks critical Services (`kubernetes`, `openshift-apiserver`, `dns-default`, `kube-dns`)
- **ServiceAccount:** deny-list blocks critical ServiceAccounts (`default`, `deployer`, `builder`, `pipeline`)
- **Cluster-scoped kinds:** blocked entirely (Namespace, Node, ClusterRole, ClusterRoleBinding, CRD, PersistentVolume). ResourceDeletion only works with namespaced resources.
- Protected namespaces (`kube-system`, `openshift-*`) are rejected

## Disruption Rubric

**Expected behavior on a healthy operator:**
The operator detects the missing resource (via watches or periodic reconciliation) and recreates it from its desired state. For Services, the operator should recreate the Service with the same selectors and ports. The Deployment should remain Available throughout if the deleted resource is not critical to pod health.

**Contract violation indicators:**
- Operator does not recreate the deleted resource within `recoveryTimeout` (indicates the resource is not managed by reconciliation)
- Operator recreates the resource but with incorrect spec (indicates drift between desired state and actual recreation logic)
- Operator enters CrashLoopBackOff when the resource disappears (indicates fatal dependency with no error handling)
- Dependent resources (e.g., Endpoints for a Service) are not recreated

**Collateral damage risks:**
- **Varies by kind.** Service deletion breaks network connectivity to the operator but pods remain running. ConfigMap deletion may cause pod restarts if mounted as a volume. Secret deletion may break TLS.
- For Service deletion specifically: existing TCP connections may persist (via conntrack), but new connections fail until the Service is recreated
- On clusters with NetworkPolicies referencing the Service, deletion may break network rules

**Recovery expectations:**
- Recovery time: 10-60 seconds for operator-reconciled resources. Services are typically recreated within one reconciliation cycle.
- Reconcile cycles: 1-2 (detection, recreation)
- What "recovered" means: resource exists with correct spec, and all dependent resources (Endpoints, etc.) are functional

## Cross-Component Results

| Component | Experiment | Danger | Description |
|-----------|------------|--------|-------------|
| odh-model-controller | odh-model-controller-resource-deletion-service | high | Deleting the metrics Service tests whether the operator recreates it. The Deployment remains Available since the Service is not critical to pod health. |
| kserve | kserve-resource-deletion-service | high | Deleting the kserve-controller-manager metrics Service tests whether the operator recreates it. The Deployment remains Available. |
| knative-serving | knative-serving-controller-resource-deletion-service | high | Deleting the knative-serving controller Service tests whether the operator recreates it. The Deployment remains Available. |
| cert-manager | cert-manager-resource-deletion-service | high | Deleting the cert-manager Service tests whether the operator recreates it. The Deployment remains Available. |
| service-mesh | istiod-resource-deletion-service | high | Deleting the istiod Service tests whether the operator recreates it. The Deployment remains Available. |

<!-- custom-start: notes -->
<!-- custom-end: notes -->
