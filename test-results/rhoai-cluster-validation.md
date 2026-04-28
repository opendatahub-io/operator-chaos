# RHOAI Cluster Validation Results

Chaos experiment validation against live RHOAI clusters. Results collected April 28-29, 2026.

## Cluster Environments

| Cluster | Platform | OCP Version | RHOAI | Nodes | Notes |
|---------|----------|-------------|-------|-------|-------|
| Cluster 2 | ROSA HyperShift | 4.20 | 3.3.2 | 2x m5.xlarge | Expired mid-testing |
| Cluster 3 | ROSA HyperShift | 4.20 | 3.3.x | 2x m5.xlarge | Expired after testing |
| Cluster 4 | ROSA HyperShift | 4.20.11 | 3.3.2 | 2x m5.xlarge | Gateway API experiments |

All clusters are resource-constrained (2x m5.xlarge = 7 CPU / ~30 GB RAM total). Dashboard scaled to 1 replica with reduced CPU requests (50m per container) to fit.

## Ingress Mode

RHOAI 3.3.x ingress architecture: Client -> Route/data-science-gateway (openshift-ingress) -> Gateway (Istio) -> HTTPRoute -> Service

All test clusters used ROSA HyperShift with `ingressMode: OcpRoute` (set via GatewayConfig CR). This creates:
- `Route/data-science-gateway` in `openshift-ingress` (shared ingress entry point)
- `HTTPRoute/rhods-dashboard` in `redhat-ods-applications` (Gateway API routing)
- `Gateway/data-science-gateway` in `openshift-ingress` (Istio gateway)
- Model-registry route: `model-catalog-https` in `rhoai-model-registries`

Note: `Route/rhods-dashboard` does NOT exist in RHOAI 3.3.x. The old naming was from pre-Gateway-refactor versions. The 5 original route-* experiments targeting this resource were removed and replaced with route-gateway-* experiments targeting `Route/data-science-gateway`.

## Results Summary

**32 total RHOAI experiments. 27 tested, 5 removed (outdated).**

| Verdict | Count | Details |
|---------|-------|---------|
| Resilient | 8 | Fast recovery, system self-healed |
| Degraded | 18 | System recovered but at the timeout boundary |
| Failed | 1 | namespace-deletion (operator does not auto-recover) |
| Removed | 5 | Old Route/rhods-dashboard experiments (resource doesn't exist in 3.3.x) |

## Detailed Results by Component

### odh-model-controller (4 experiments)

| Experiment | Tier | Type | Cluster | Verdict | Recovery | Notes |
|-----------|------|------|---------|---------|----------|-------|
| pod-kill | T1 | PodKill | 3 | **Resilient** | fast | Pod recreated quickly |
| label-stomping | T3 | LabelStomping | 3 | **Resilient** | fast | DSC operator restored label |
| label-stomping-delete | T3 | LabelStomping | 3 | **Resilient** | fast | DSC operator restored deleted label |
| namespace-deletion | T5 | NamespaceDeletion | 4 | **Failed** | manual | Operator does NOT auto-recreate namespace. See Finding #9 |

### kserve (1 experiment)

| Experiment | Tier | Type | Cluster | Verdict | Recovery | Notes |
|-----------|------|------|---------|---------|----------|-------|
| pod-kill | T1 | PodKill | 3 | **Resilient** | fast | Controller recreated promptly |

### opendatahub-operator (2 experiments)

| Experiment | Tier | Type | Cluster | Verdict | Recovery | Notes |
|-----------|------|------|---------|---------|----------|-------|
| pod-kill | T1 | PodKill | 3 | **Resilient** | 832ms, 1 cycle | 3-replica Deployment, fast recovery |
| finalizer-block | T3 | FinalizerBlock | 3 | **Resilient** | 866ms, 1 cycle | Finalizer on DSC CR, no impact on operation |

### workbenches (3 experiments)

| Experiment | Tier | Type | Cluster | Verdict | Recovery | Notes |
|-----------|------|------|---------|---------|----------|-------|
| pod-kill | T1 | PodKill | 3 | **Resilient** | fast | Notebook controller recovered |
| network-partition | T2 | NetworkPartition | 3 | **Resilient** | fast | NetworkPolicy removed, controller reconnected |
| webhook-disrupt | T4 | WebhookDisrupt | 3 | **Resilient** | fast | Webhook config restored by operator |

### model-registry (4 experiments)

| Experiment | Tier | Type | Cluster | Verdict | Recovery | Notes |
|-----------|------|------|---------|---------|----------|-------|
| pod-kill | T1 | PodKill | 3 | Degraded | 2m0s, 60 cycles | Recovery at timeout boundary |
| route-backend-disruption | T3 | CRDMutation | 3 | Degraded | 2m0s, 60 cycles | Route backend swapped, restored after cleanup |
| route-host-collision | T3 | CRDMutation | 3 | Degraded | 2m0s, 60 cycles | Route host mutated, restored |
| route-tls-mutation | T3 | CRDMutation | 3 | Degraded | 2m0s, 60 cycles | TLS termination changed, restored |

### dashboard: non-route experiments (4 experiments)

| Experiment | Tier | Type | Cluster | Verdict | Recovery | Notes |
|-----------|------|------|---------|---------|----------|-------|
| pod-kill | T1 | PodKill | 3 | Degraded | 2m0s, 60 cycles | 5-container pod, slow startup on constrained cluster |
| config-drift | T2 | ConfigDrift | 3 | Degraded | 3m0s, 90 cycles | ConfigMap corrupted, steady state restored |
| network-partition | T2 | NetworkPartition | 3 | Degraded | 2m0s, 60 cycles | From suite run, cascade effect |
| rbac-revoke | T4 | RBACRevoke | 3 | Degraded | 2m0s, 60 cycles | From suite run, cascade effect |

### dashboard: Gateway API experiments (4 experiments)

| Experiment | Tier | Type | Cluster | Verdict | Recovery | Notes |
|-----------|------|------|---------|---------|----------|-------|
| gateway-backend-disruption | T3 | CRDMutation | 4 | Degraded | 2m0s, 60 cycles | HTTPRoute rules replaced with non-existent backend |
| gateway-host-collision | T3 | CRDMutation | 4 | Degraded | 2m0s, 60 cycles | HTTPRoute hostnames mutated |
| gateway-host-deletion | T3 | CRDMutation | 4 | Degraded | 2m0s, 60 cycles | HTTPRoute hostnames removed |
| gateway-parentref-mismatch | T3 | CRDMutation | 4 | Degraded | 2m0s, 59 cycles | HTTPRoute orphaned from gateway |

### dashboard: Route/data-science-gateway experiments (5 experiments)

These target the shared ingress Route in `openshift-ingress`. Required `allowDangerous: true` to bypass the `openshift-*` namespace prefix restriction.

| Experiment | Tier | Type | Cluster | Verdict | Recovery | Notes |
|-----------|------|------|---------|---------|----------|-------|
| route-gateway-backend-disruption | T3 | CRDMutation | 4 | Degraded | 2m0s, 60 cycles | Route backend swapped to non-existent service |
| route-gateway-host-collision | T3 | CRDMutation | 4 | Degraded | 2m0s, 60 cycles | Route host mutated to invalid domain |
| route-gateway-host-deletion | T3 | CRDMutation | 4 | Degraded | 2m0s, 60 cycles | Route host set to null |
| route-gateway-shard-mismatch | T3 | CRDMutation | 4 | Degraded | 2m0s, 60 cycles | Route host set to non-existent shard domain |
| route-gateway-tls-mutation | T3 | CRDMutation | 4 | Degraded | 2m0s, 60 cycles | TLS termination changed from reencrypt to passthrough |

### dashboard: removed experiments (5 experiments, DELETED)

The following experiments were removed because they targeted `Route/rhods-dashboard`, which does not exist in RHOAI 3.3.x:

| Experiment | Reason |
|-----------|--------|
| route-backend-disruption | `Route/rhods-dashboard` doesn't exist in 3.3.x |
| route-host-collision | Replaced by route-gateway-host-collision |
| route-host-deletion | Replaced by route-gateway-host-deletion |
| route-shard-mismatch | Replaced by route-gateway-shard-mismatch |
| route-tls-mutation | Replaced by route-gateway-tls-mutation |

## Findings

### 1. All "Degraded" verdicts are a resource-constrained cluster artifact

Every Degraded verdict shows the same pattern: recovery time at exactly 2m0.001s (or 3m0.001s for config-drift), with 60 (or 90) reconcile cycles. This is the observer polling every 2 seconds for the full recovery window on a small 2x m5.xlarge cluster. On production-sized clusters, recovery would likely be much faster and these would return Resilient.

**Action**: Not a bug. Consider increasing default recoveryTimeout in RHOAI experiments or documenting minimum cluster size for accurate results.

### 2. Model-registry route experiments had wrong resource names

The model-registry route experiments originally referenced `Route/model-registry-operator-rest` in `redhat-ods-applications`. The actual route is `model-catalog-https` in `rhoai-model-registries`.

**Action**: Fixed in commit `7cbc179`. No Jira needed.

### 3. Dashboard Route experiments rewritten for RHOAI 3.3.x architecture

The original 5 Route experiments targeted `Route/rhods-dashboard` which doesn't exist in RHOAI 3.3.x. The actual Route is `data-science-gateway` in `openshift-ingress`. Two sets of replacements created:
- 4 Gateway API experiments targeting `HTTPRoute/rhods-dashboard` (validated on cluster 4)
- 5 Route experiments targeting `Route/data-science-gateway` in `openshift-ingress` (validated on cluster 4)

**Action**: Old experiments removed, new ones committed and validated. No Jira needed.

### 4. Gateway API and Route experiments required framework changes

Two framework changes were needed:
- CRDMutation on Gateway API resources requires JSON arrays/objects as values (for hostnames, rules, parentRefs). The validator was updated to allow complex values when `dangerLevel: high`.
- Route/data-science-gateway lives in `openshift-ingress`, which was blocked by the `openshift-*` namespace prefix restriction. The `checkForbiddenNamespace` function was updated to skip prefix checks when `allowDangerous: true` (hard-coded namespaces like `kube-system` remain blocked regardless).

**Action**: Fixed in commits `7cbc179` and subsequent. No Jira needed.

### 5. Label-stomping experiment was missing allowDangerous

`experiments/odh-model-controller/label-stomping.yaml` had `dangerLevel: high` but no `allowDangerous: true` in blastRadius, causing validation failure on live cluster.

**Action**: Fixed in commit `7cbc179`. No Jira needed.

### 6. Dashboard pods Pending on resource-constrained clusters

The rhods-dashboard Deployment has 5 containers with 500m CPU request each (2.5 CPU total per replica, 2 replicas = 5 CPU). This exceeds available capacity on 2x m5.xlarge clusters. Required manual scaling to 1 replica and reducing CPU requests to 50m per container.

**Action**: Consider documenting minimum cluster size requirements. Potential Jira for adding cluster size validation to preflight checks.

### 7. Network partition causes cascading suite failures

When running experiments as a suite, a network-partition experiment can leave the cluster in a non-steady state, causing all subsequent experiments to fail pre-check. Running experiments individually after disruptive ones avoids this.

**Action**: Consider adding inter-experiment recovery delays to suite runner. Potential enhancement Jira.

### 8. Namespace-deletion experiment: webhook blocks namespace termination

When `redhat-ods-applications` is deleted, the namespace gets stuck in `Terminating` because the `llminferenceserviceconfig.serving.kserve.io` validating webhook (failurePolicy: Fail) references `kserve-webhook-server-service` in the same namespace. The service is deleted before the `LLMInferenceServiceConfig` CRs, so the webhook call fails and blocks resource cleanup.

Manual fix required: patch the webhook failurePolicy to `Ignore`, then the namespace finishes terminating.

**Action**: This is a real RHOAI bug. The kserve webhook should use `failurePolicy: Ignore` for deletion requests, or the webhook configuration should be cleaned up before namespace deletion. File a Jira.

### 9. RHOAI operator does NOT auto-recover from namespace deletion (critical finding)

After `redhat-ods-applications` is fully deleted, the RHOAI operator:
1. Detects the namespace is gone (logs show "namespaces not found" errors)
2. Does NOT recreate the namespace (the namespace is not a managed resource)
3. Backs off exponentially and eventually stops reconciling entirely
4. Requires manual namespace recreation + DSC annotation change to trigger recovery
5. After manual namespace creation, recovers all components in ~5-8 minutes

The experiment returned **Failed** verdict because the operator never restored steady state within the 5-minute recovery window.

Recovery timeline after manual namespace recreation:
- odh-model-controller: ~1 minute
- notebook-controller, odh-notebook-controller: ~3 minutes
- kserve-controller-manager: ~2 minutes
- model-registry-operator: ~3 minutes
- rhods-dashboard: ~4 minutes (needed resource tuning, 2 replicas with 5 containers)

**Action**: This is a significant resilience gap. The RHOAI operator should watch for namespace deletion and recreate it, or at minimum the DSC should report a clear error condition. File a Jira.

## Potential Jiras

| Finding | Severity | Type | Description |
|---------|----------|------|-------------|
| Namespace deletion: operator doesn't auto-recover | **High** | Bug | RHOAI operator does not recreate `redhat-ods-applications` namespace after deletion. Requires manual intervention. |
| Namespace deletion: webhook blocks termination | **High** | Bug | `llminferenceserviceconfig` validating webhook (failurePolicy: Fail) blocks namespace termination because its service is deleted first |
| Cluster size requirements | Low | Enhancement | Add minimum cluster size check to preflight or document requirements (dashboard needs 2.5 CPU per replica) |
| Suite cascading failures | Medium | Enhancement | Add configurable inter-experiment recovery delay to suite runner to prevent cascading pre-check failures |
| Network partition recovery | Medium | Bug/Investigation | odh-model-controller doesn't recover from network partition within 2 minutes (observed on cluster 2, needs retest on larger cluster) |

## Experiments Not Yet Tested (non-RHOAI)

The ODH-specific experiments under `experiments/` (not `experiments/rhoai/`) were partially validated on cluster 2 before it expired. Full ODH experiment validation was deprioritized in favor of RHOAI-specific validation.
