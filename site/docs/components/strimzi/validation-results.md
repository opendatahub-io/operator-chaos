# Strimzi Validation Results

## Test Platform

- **Platform:** OpenShift 4.20
- **Strimzi Version:** OLM-managed (AMQ Streams)
- **Test Date:** 2026-05

## Results

| Experiment | Component | Injection | Verdict | Notes |
|-----------|-----------|-----------|---------|-------|
| cluster-operator/pod-kill | cluster-operator | PodKill | Resilient | |
| cluster-operator/network-partition | cluster-operator | NetworkPartition | Resilient | |
| cluster-operator/label-stomping | cluster-operator | LabelStomping | Resilient | |
| cluster-operator/quota-exhaustion | cluster-operator | QuotaExhaustion | Resilient | |
| cluster-operator/rbac-revoke | cluster-operator | RBACRevoke | Resilient | |
| cluster-operator/deployment-scale-zero | cluster-operator | DeploymentScaleZero | Degraded | OLM does not restore replicas |
| cluster-operator/leader-election-disrupt | cluster-operator | LeaderElectionDisrupt | Resilient | |
| cluster-operator/config-drift | cluster-operator | ConfigDrift | Resilient | |

## Key Findings

### DeploymentScaleZero: OLM Gap

The only non-Resilient result. When the cluster-operator Deployment is scaled to zero replicas, OLM does not automatically restore the replica count. This is a known OLM limitation affecting all OLM-managed operators. Manual intervention (or an external controller) is needed to restore replicas.

### Strong Core Resilience

All other failure modes (PodKill, NetworkPartition, LabelStomping, QuotaExhaustion, RBACRevoke, LeaderElectionDisrupt, ConfigDrift) recover automatically. The Strimzi operator handles disruptions well across the board.


<!-- custom-start: analysis -->
<!-- custom-end: analysis -->
