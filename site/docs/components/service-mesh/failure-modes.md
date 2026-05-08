# Service Mesh Failure Modes

## Experiment Summary

| # | Experiment | Component | Type | Tier | Danger |
|---|-----------|-----------|------|------|--------|
| 1 | servicemesh-operator3-pod-kill | servicemesh-operator3 | PodKill | 1 | low |
| 2 | istiod-pod-kill | istiod | PodKill | 1 | low |
| 3 | servicemesh-operator3-network-partition | servicemesh-operator3 | NetworkPartition | 2 | low |
| 4 | istiod-network-partition | istiod | NetworkPartition | 2 | low |
| 5 | servicemesh-operator3-quota-exhaustion | servicemesh-operator3 | QuotaExhaustion | 3 | low |
| 6 | istiod-quota-exhaustion | istiod | QuotaExhaustion | 3 | low |
| 7 | servicemesh-operator3-label-stomping | servicemesh-operator3 | LabelStomping | 3 | high |
| 8 | istiod-label-stomping | istiod | LabelStomping | 3 | low |
| 9 | servicemesh-operator3-rbac-revoke | servicemesh-operator3 | RBACRevoke | 4 | high |
| 10 | istiod-rbac-revoke | istiod | RBACRevoke | 4 | high |
| 11 | istiod-webhook-disrupt | istiod | WebhookDisrupt | 4 | high |
| 12 | istiod-sidecar-injector-disrupt | istiod | WebhookDisrupt | 4 | high |
| 13 | istiod-crd-mutation | istiod | CRDMutation | 3 | high |
| 14 | istiod-finalizer-block | istiod | FinalizerBlock | 3 | high |
| 15 | istiod-config-drift | istiod | ConfigDrift | 2 | high |
| 16 | istiod-webhook-latency | istiod | WebhookLatency | 4 | high |
| 17 | istiod-ownerref-orphan | istiod | OwnerRefOrphan | 3 | low |
| 18 | servicemesh-operator3-olm-subscription-corrupt | servicemesh-operator3 | CRDMutation | 3 | low |
| 19 | istiod-crashloop-inject | istiod | CrashLoopInject | 3 | high |
| 20 | istiod-image-corrupt | istiod | ImageCorrupt | 3 | high |
| 21 | istiod-resource-deletion-service | istiod | ResourceDeletion | 3 | high |
| 22 | istiod-pdb-block | istiod | PDBBlock | 2 | high |

## servicemesh-operator3 (Operator Layer)

### PodKill (Tier 1)

Kills the Service Mesh operator pod. OLM and the Deployment controller restore the pod. Existing istiod instances remain running independently since they don't depend on the operator for runtime operation.

### NetworkPartition (Tier 2)

Isolates the operator from the API server. Istiod and gateway deployments remain operational when the operator cannot reconcile. The data plane is unaffected.

### QuotaExhaustion (Tier 3)

Applies restrictive resource quotas to prevent the operator from restarting after a failure. Existing pods continue running but new pods cannot be scheduled until the quota is removed.

### LabelStomping (Tier 3, High Danger)

Removes the `app.kubernetes.io/name` label from the operator deployment. Tests whether OLM or the deployment controller restores the expected labels.

### RBACRevoke (Tier 4, High Danger)

Revokes the OLM-managed ClusterRoleBinding. Tests whether OLM restores RBAC from the CSV. Without permissions, the operator cannot reconcile Istio CRs.

## istiod (Control Plane Layer)

### PodKill (Tier 1)

Kills the istiod control plane pod. Existing data plane proxies continue serving traffic with cached config, but new proxy connections and config pushes fail until istiod recovers.

### NetworkPartition (Tier 2)

Isolates istiod from the API server and data plane proxies. Envoy proxies continue with their last-known-good config. After recovery, istiod reconnects and pushes updates.

### QuotaExhaustion (Tier 3)

Applies restrictive quotas to the openshift-ingress namespace. Existing istiod and gateway pods continue running but new pods cannot be scheduled.

### LabelStomping (Tier 3)

Removes the `app` label from the istiod deployment. Tests whether the Service Mesh operator restores labels during reconciliation.

### RBACRevoke (Tier 4, High Danger)

Revokes the istiod ClusterRoleBinding. Without API access, istiod cannot watch Services, Endpoints, or push config to proxies. Tests whether the operator restores RBAC.

### WebhookDisrupt: Validating (Tier 4, High Danger)

Changes the Istio validating webhook's failurePolicy from Fail to Ignore. Without strict validation, invalid Istio resources (VirtualServices, DestinationRules) could be applied.

### WebhookDisrupt: Sidecar Injector (Tier 4, High Danger)

Changes the sidecar injector mutating webhook's failurePolicy from Fail to Ignore. Without strict enforcement, new pods in mesh-enabled namespaces may not get Envoy sidecars injected.

### CRDMutation (Tier 3, High Danger)

Mutates the Istio CR's meshConfig (accessLogFile) to test whether the Sail operator detects spec drift and reconciles the CR back. Note: the `spec.version` field is protected by CEL validation and cannot be set to invalid values.

### FinalizerBlock (Tier 3, High Danger)

Adds a blocking finalizer to the Istio CR to simulate a stuck-in-termination state. Tests whether the operator and istiod continue operating while the CR cannot be deleted.

### ConfigDrift (Tier 2, High Danger)

Corrupts the `istio-openshift-gateway` ConfigMap's mesh config key. istiod watches this ConfigMap and hot-reloads on changes. Tests whether the operator reconciles the ConfigMap back to the expected state.

### WebhookLatency (Tier 4, High Danger)

Deploys a slow admission webhook (25s delay) intercepting VirtualService resources. Tests whether the mesh control plane handles slow API responses gracefully without hanging or crashing.

### OwnerRefOrphan (Tier 3)

Removes ownerReferences from the istiod deployment, orphaning it from the Istio CR's resource graph. Tests whether the Sail operator detects the missing ownership and re-adopts the deployment.

## servicemesh-operator3 (Additional)

### OLM Subscription Channel Corrupt (Tier 3)

Mutates the OLM subscription channel to a non-existent value. The operator deployment remains running (channel changes only affect future upgrades). Tests OLM's handling of invalid subscription state.
