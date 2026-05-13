# Spark Operator Validation Results

## Test Platform

- **Platform:** OpenShift 4.20
- **Spark Operator Version:** Helm/kustomize-managed
- **Test Date:** 2026-05

## Results

| Experiment | Component | Injection | Verdict | Notes |
|-----------|-----------|-----------|---------|-------|
| controller/pod-kill | controller | PodKill | Resilient | |
| controller/network-partition | controller | NetworkPartition | Failed | Controller permanently non-functional after partition |
| controller/label-stomping | controller | LabelStomping | Resilient | |
| controller/quota-exhaustion | controller | QuotaExhaustion | Resilient | |
| controller/rbac-revoke | controller | RBACRevoke | Resilient | |
| controller/deployment-scale-zero | controller | DeploymentScaleZero | Degraded | No controller to restore replicas |
| webhook/pod-kill | webhook | PodKill | Resilient | |
| webhook/network-partition | webhook | NetworkPartition | Resilient | |
| webhook/label-stomping | webhook | LabelStomping | Resilient | |
| webhook/quota-exhaustion | webhook | QuotaExhaustion | Resilient | |
| webhook/deployment-scale-zero | webhook | DeploymentScaleZero | Degraded | No controller to restore replicas |
| webhook/webhook-cert-corrupt | webhook | ConfigDrift | Resilient | |

## Key Findings

### NetworkPartition: Controller Permanently Non-Functional

The most critical finding. After a network partition is applied and lifted, the Spark operator controller does not re-establish its API server watch connections. The controller remains running but is permanently non-functional until the pod is manually restarted. This indicates missing reconnection logic in the controller's informer/watch setup.

### DeploymentScaleZero: No Automatic Recovery

Both the controller and webhook Deployments remain at zero replicas when scaled down. The Spark operator is Helm/kustomize-managed (not OLM), so there is no higher-level controller to detect and restore the replica count. Manual intervention is required.

### Webhook Resilience

The webhook component shows good resilience across most failure modes. Unlike the controller, it recovers from network partitions correctly.


<!-- custom-start: analysis -->
<!-- custom-end: analysis -->
