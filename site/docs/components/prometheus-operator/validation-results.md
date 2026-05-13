# Prometheus Operator Validation Results

## Test Platform

- **Platform:** OpenShift 4.20
- **Prometheus Operator Version:** Managed by cluster-monitoring-operator (built-in)
- **Test Date:** 2026-05

## Results

| Experiment | Component | Injection | Verdict | Notes |
|-----------|-----------|-----------|---------|-------|
| prometheus-operator/pod-kill | prometheus-operator | PodKill | Resilient | |
| prometheus-operator/network-partition | prometheus-operator | NetworkPartition | Resilient | |
| prometheus-operator/label-stomping | prometheus-operator | LabelStomping | Resilient | |
| prometheus-operator/quota-exhaustion | prometheus-operator | QuotaExhaustion | Resilient | |
| prometheus-operator/rbac-revoke | prometheus-operator | RBACRevoke | Resilient | |
| prometheus-operator/deployment-scale-zero | prometheus-operator | DeploymentScaleZero | Resilient | cluster-monitoring-operator restores replicas |

## Key Findings

### Perfect Resilience Record

All 6 Prometheus Operator experiments passed with Resilient verdicts. The operator demonstrates excellent fault tolerance thanks to cluster-monitoring-operator managing its lifecycle.

### cluster-monitoring-operator Reconciliation

Unlike OLM-managed operators (where DeploymentScaleZero is typically Degraded), the Prometheus Operator is fully reconciled by cluster-monitoring-operator. This includes restoring replica counts, labels, and configuration. cluster-monitoring-operator actively monitors the state of all monitoring components and corrects any drift.

### Platform-Level Resilience

As a built-in OpenShift component, the Prometheus Operator benefits from platform-level lifecycle management. This makes it one of the most resilient operators tested, with automatic recovery from all tested failure modes.


<!-- custom-start: analysis -->
<!-- custom-end: analysis -->
