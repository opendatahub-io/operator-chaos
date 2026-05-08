# Service Mesh Validation Results

!!! success "22/22 Resilient"
    All Service Mesh experiments passed with sub-3 second recovery times. Tested on ROSA HyperShift 4.21.11 with OSSM 3.2.0 (Istio 1.27.3).

## Test Environment

| Property | Value |
|----------|-------|
| **Platform** | ROSA HyperShift 4.21.11 |
| **OSSM Version** | 3.2.0 |
| **Istio Version** | 1.27.3 |
| **Operator CSV** | servicemeshoperator3.v3.2.0 |
| **Test Date** | 2026-05-07 |

## Results

### servicemesh-operator3

| Experiment | Verdict | Recovery Time | Cycles |
|-----------|---------|---------------|--------|
| pod-kill | Resilient | 1.8s | 1 |
| network-partition | Resilient | 2s | 1 |
| quota-exhaustion | Resilient | 2s | 1 |
| label-stomping | Resilient | 2s | 1 |
| rbac-revoke | Resilient | 1.5s | 1 |

### istiod

| Experiment | Verdict | Recovery Time | Cycles |
|-----------|---------|---------------|--------|
| pod-kill | Resilient | 2s | 1 |
| network-partition | Resilient | 1s | 1 |
| quota-exhaustion | Resilient | 2s | 1 |
| label-stomping | Resilient | 2s | 1 |
| rbac-revoke | Resilient | 1.5s | 1 |
| webhook-disrupt | Resilient | 2.6s | 1 |
| sidecar-injector-disrupt | Resilient | 1.5s | 1 |
| crd-mutation | Resilient | 1.6s | 1 |
| finalizer-block | Resilient | 1.5s | 1 |
| config-drift | Resilient | 1.7s | 1 |
| webhook-latency | Resilient | 1.6s | 1 |
| ownerref-orphan | Resilient | 1.8s | 1 |
| crashloop-inject | Resilient | <3s | 1 |
| image-corrupt | Resilient | <3s | 1 |
| resource-deletion-service | Resilient | <3s | 1 |
| pdb-block | Resilient | <3s | 1 |

### OLM Lifecycle

| Experiment | Verdict | Recovery Time | Cycles |
|-----------|---------|---------------|--------|
| olm-subscription-corrupt | Resilient | 1.5s | 1 |

## Analysis

Service Mesh v3 (Sail Operator) shows excellent resilience across all fault categories:

**Operator Layer:** OLM restores the operator deployment, RBAC, and labels within 2 seconds. The operator uses a single replica, but the fast recovery via OLM CSV reconciliation means downtime is minimal.

**Control Plane Layer:** istiod recovers quickly from all faults. The Deployment controller handles pod-kill, the operator handles label restoration and webhook recovery, and OLM handles RBAC restoration. The Istio CR's Ready condition is maintained through all experiments.

**Webhook Resilience:** Both the validating webhook (`/validate`) and the mutating sidecar injector (`/inject`) are restored within 3 seconds after disruption. istiod actively monitors and re-registers its webhooks.

**CRD Validation:** The Istio CR has CEL validation on `spec.version` that prevents invalid values from being set. This blocked our initial CRDMutation attempt, which is actually a positive finding: the CRD schema prevents accidental misconfiguration at the admission layer.

**Config Recovery:** Both the Istio CR spec drift (CRDMutation) and ConfigMap corruption (ConfigDrift) were recovered in under 2 seconds. The Sail operator actively reconciles the full resource tree, not just the deployment.

**Key Observation:** Unlike some operators where RBAC revocation causes prolonged degradation (e.g., Kueue OLM taking ~3 minutes), Service Mesh recovers RBAC in 1.5 seconds. OLM's CSV reconciliation for the Service Mesh operator is highly efficient.
