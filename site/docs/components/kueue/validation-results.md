# kueue Validation Results

## Test Platform

- **Platform:** ROSA HyperShift 4.20.11
- **Kueue Version:** Red Hat Build of Kueue Operator (OLM-managed)
- **Test Date:** 2026-05-06

## Results

### kueue-operator (Red Hat Build of Kueue)

All 11 kueue-operator experiments validated on 2026-05-06. All experiments passed with perfect resilience, demonstrating robust operator-layer recovery.

| Experiment | Injection | Verdict | Recovery Time | Reconcile Cycles |
|-----------|-----------|---------|---------------|------------------|
| crd-mutation | CRDMutation | Resilient | 1.3s | 1 |
| label-stomping | LabelStomping | Resilient | 1.3s | 1 |
| leader-lease-corrupt | CRDMutation | Resilient | 0.9s | 1 |
| network-partition | NetworkPartition | Resilient | 1.3s | 1 |
| olm-csv-owned-crd-corrupt | CRDMutation | Resilient | 1.3s | 1 |
| olm-subscription-approval-flip | CRDMutation | Resilient | 1.2s | 1 |
| olm-subscription-channel-corrupt | CRDMutation | Resilient | 1.2s | 1 |
| ownerref-orphan | OwnerRefOrphan | Resilient | 1.3s | 1 |
| pod-kill | PodKill | Resilient | 1.8s | 1 |
| quota-exhaustion | QuotaExhaustion | Resilient | 1.3s | 1 |
| rbac-revoke | RBACRevoke | Resilient | 1.4s | 1 |

### kueue-operand

Not yet tested on this cluster. The kueue-operand experiments test workload admission resources (ClusterQueues, LocalQueues, ResourceFlavors, Workloads) and require a full Kueue deployment with test workloads.

### Legacy DSC-managed Kueue

Not yet tested on this cluster. The legacy experiments target ODH/RHOAI 2.x deployments where Kueue was managed as a DSC component in the opendatahub namespace.

## Key Findings

### Perfect Operator Resilience

The Red Hat Build of Kueue Operator demonstrated flawless resilience across all 11 operator-layer experiments. Recovery times were consistently under 2 seconds, with most experiments recovering in 1.3s or less. Every experiment completed in a single reconcile cycle, indicating efficient detection and recovery logic.

### Sub-2s Recovery Window

The entire operator layer recovers from faults in under 2 seconds:

- **Fastest:** leader-lease-corrupt (0.9s)
- **Slowest:** pod-kill (1.8s)
- **Median:** 1.3s

This tight recovery window is critical for production environments where workload admission delays directly impact user experience.

### OLM Integration Resilience

The operator handled OLM-specific faults gracefully:

- **CSV corruption:** Operator deployment remained available despite invalid CSV metadata
- **Subscription corruption:** Channel and approval changes did not affect the running operator
- **CRD mutation:** API server confusion from CRD name corruption was transient

These results validate that the OLM-managed deployment model is production-ready and resilient to operator lifecycle faults.

### Deployment-Scale Recovery

The operator deployment runs with 2 replicas and uses leader election. Pod kill and network partition experiments validated that:

- Leader election completes within 1.8s after pod failure
- Network isolation does not cause split-brain scenarios
- Quota exhaustion is detected and recovered from gracefully

### Outstanding Validation Work

The kueue-operand and legacy experiments remain untested. The operand experiments are particularly important as they validate workload admission resilience (the core Kueue functionality), not just operator-layer recovery.


<!-- custom-start: analysis -->
<!-- custom-end: analysis -->
