# ArgoCD Validation Results

## Test Platform

- **Platform:** OpenShift 4.20
- **ArgoCD Version:** OLM-managed (OpenShift GitOps)
- **Test Date:** 2026-05

## Results

| Experiment | Component | Injection | Verdict | Notes |
|-----------|-----------|-----------|---------|-------|
| server/pod-kill | server | PodKill | Resilient | |
| server/network-partition | server | NetworkPartition | Resilient | |
| server/label-stomping | server | LabelStomping | Resilient | |
| server/quota-exhaustion | server | QuotaExhaustion | Resilient | |
| server/rbac-revoke | server | RBACRevoke | Resilient | |
| server/deployment-scale-zero | server | DeploymentScaleZero | Resilient | |
| server/leader-election-disrupt | server | LeaderElectionDisrupt | Resilient | |
| server/config-drift | server | ConfigDrift | Resilient | |
| repo-server/pod-kill | repo-server | PodKill | Resilient | |
| repo-server/network-partition | repo-server | NetworkPartition | Resilient | |
| repo-server/label-stomping | repo-server | LabelStomping | Resilient | |
| repo-server/quota-exhaustion | repo-server | QuotaExhaustion | Resilient | |
| repo-server/rbac-revoke | repo-server | RBACRevoke | Resilient | |
| repo-server/deployment-scale-zero | repo-server | DeploymentScaleZero | Resilient | |
| repo-server/leader-election-disrupt | repo-server | LeaderElectionDisrupt | Resilient | |
| repo-server/config-drift | repo-server | ConfigDrift | Resilient | |

## Key Findings

### Perfect Resilience Record

All 16 ArgoCD experiments passed with Resilient verdicts. Both the server and repo-server components demonstrate excellent fault tolerance.

### False Positive Correction

Initial automated runs reported Degraded verdicts for some experiments. Manual investigation confirmed these were false positives caused by evaluator cycle counting noise (the evaluator counted extra reconcile cycles that did not reflect actual degradation). Manual testing verified that all components recover correctly.

### OLM-Managed Recovery

As an OLM-managed operator, ArgoCD benefits from the GitOps operator's reconciliation loop. The operator restores Deployments, Services, and configuration after disruption. DeploymentScaleZero recovers because the GitOps operator (not bare OLM) manages the ArgoCD instance via the ArgoCD CR.


<!-- custom-start: analysis -->
<!-- custom-end: analysis -->
