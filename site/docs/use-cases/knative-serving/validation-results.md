# Knative Serving Validation Results

## Test Platform

- **Platform:** ROSA HyperShift 4.21.11
- **Knative Version:** OpenShift Serverless 1.37.1
- **Test Date:** 2026-05-06

## Results

| Experiment | Component | Injection | Verdict | Recovery Time | Reconcile Cycles |
|-----------|-----------|-----------|---------|---------------|------------------|
| activator/pod-kill | activator | PodKill | Resilient | 916ms | 1 |
| activator/all-pods-kill | activator | PodKill | Resilient | 893ms | 1 |
| activator/network-partition | activator | NetworkPartition | **Failed** | N/A | N/A |
| activator/label-stomping | activator | LabelStomping | Resilient | 904ms | 1 |
| activator/quota-exhaustion | activator | QuotaExhaustion | Resilient | 934ms | 1 |
| activator/rbac-revoke | activator | RBACRevoke | Resilient | 1.3s | 1 |
| autoscaler/pod-kill | autoscaler | PodKill | Resilient | 928ms | 1 |
| autoscaler/all-pods-kill | autoscaler | PodKill | Resilient | 890ms | 1 |
| autoscaler/network-partition | autoscaler | NetworkPartition | Resilient | 907ms | 1 |
| autoscaler/label-stomping | autoscaler | LabelStomping | Resilient | 1.0s | 1 |
| autoscaler/quota-exhaustion | autoscaler | QuotaExhaustion | Resilient | 899ms | 1 |
| autoscaler-hpa/pod-kill | autoscaler-hpa | PodKill | Resilient | ~1s | 1 |
| autoscaler-hpa/network-partition | autoscaler-hpa | NetworkPartition | Resilient | ~1s | 1 |
| controller/pod-kill | controller | PodKill | Resilient | 917ms | 1 |
| controller/all-pods-kill | controller | PodKill | Resilient | ~1s | 1 |
| controller/network-partition | controller | NetworkPartition | Resilient | ~1s | 1 |
| controller/label-stomping | controller | LabelStomping | Resilient | 890ms | 1 |
| controller/quota-exhaustion | controller | QuotaExhaustion | Resilient | ~1s | 1 |
| controller/rbac-revoke | controller | RBACRevoke | Resilient | 1.4s | 1 |
| webhook/pod-kill | webhook | PodKill | Resilient | 895ms | 1 |
| webhook/all-pods-kill | webhook | PodKill | Resilient | ~1s | 1 |
| webhook/network-partition | webhook | NetworkPartition | Resilient | ~1s | 1 |
| webhook/cert-corrupt | webhook | ConfigDrift | Resilient | 879ms | 1 |
| webhook/label-stomping | webhook | LabelStomping | Resilient | ~1s | 1 |
| webhook/quota-exhaustion | webhook | QuotaExhaustion | Resilient | ~1s | 1 |
| webhook/webhook-disrupt | webhook | WebhookDisrupt | Resilient | 898ms | 1 |
| kourier-gateway/pod-kill | kourier-gateway | PodKill | Resilient | 893ms | 1 |
| kourier-gateway/all-pods-kill | kourier-gateway | PodKill | Resilient | 869ms | 1 |
| kourier-gateway/network-partition | kourier-gateway | NetworkPartition | Resilient | 893ms | 1 |
| kourier-gateway/label-stomping | kourier-gateway | LabelStomping | Resilient | ~1s | 1 |
| kourier-gateway/quota-exhaustion | kourier-gateway | QuotaExhaustion | Resilient | ~1s | 1 |
| net-kourier-controller/pod-kill | net-kourier-controller | PodKill | Resilient | ~1s | 1 |
| net-kourier-controller/all-pods-kill | net-kourier-controller | PodKill | Resilient | ~1s | 1 |
| net-kourier-controller/network-partition | net-kourier-controller | NetworkPartition | Resilient | ~1s | 1 |
| net-kourier-controller/rbac-revoke | net-kourier-controller | RBACRevoke | Resilient | 943ms | 1 |

## Key Findings

### activator/network-partition Failure

The activator network partition experiment revealed a critical failure mode. When activator pods are network-isolated and then the partition is lifted, the pods report HTTP 500 errors on health probe endpoints. The activator process does not automatically recover from network isolation. The pods require a liveness probe restart to restore service.

**Root Cause:** The activator's health check endpoint depends on a WebSocket connection to the autoscaler. When the network partition blocks this connection, the health check handler enters a failed state. After the partition lifts, the WebSocket connection does not automatically re-establish, and the health check continues returning HTTP 500.

**Impact:** During network partitions affecting the activator, scale-from-zero traffic will be blocked. After network recovery, manual intervention (liveness probe timeout and restart) is required before service resumes.

**Mitigation:** The activator should implement connection retry logic for the autoscaler WebSocket. Alternatively, the health check should differentiate between transient connectivity issues and permanent failures.

### Overall Resilience

Knative Serving demonstrates excellent resilience across 34 of 35 experiments. The platform handles pod failures, RBAC disruptions, quota exhaustion, and configuration drift without manual intervention. Recovery is consistently fast, with most experiments recovering in under 1 second and requiring only a single reconcile cycle.

The activator/network-partition failure is the only experiment that requires manual recovery (liveness probe restart). This is a known architectural limitation of the activator's dependency on persistent WebSocket connections to the autoscaler.


<!-- custom-start: analysis -->
<!-- custom-end: analysis -->
