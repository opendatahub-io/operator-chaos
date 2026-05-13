# Tekton Validation Results

## Test Platform

- **Platform:** OpenShift 4.20
- **Tekton Version:** OLM-managed (OpenShift Pipelines)
- **Test Date:** 2026-05

## Results

| Experiment | Component | Injection | Verdict | Notes |
|-----------|-----------|-----------|---------|-------|
| pipelines-controller/pod-kill | pipelines-controller | PodKill | Resilient | |
| pipelines-controller/network-partition | pipelines-controller | NetworkPartition | Resilient | |
| pipelines-controller/deployment-scale-zero | pipelines-controller | DeploymentScaleZero | Resilient | |
| pipelines-controller/leader-election-disrupt | pipelines-controller | LeaderElectionDisrupt | Resilient | |
| pipelines-controller/crashloop-inject | pipelines-controller | CrashLoopInject | Resilient | |
| pipelines-controller/image-corrupt | pipelines-controller | ImageCorrupt | Resilient | |
| pipelines-controller/pdb-block | pipelines-controller | PDBBlock | Resilient | |
| pipelines-webhook/pod-kill | pipelines-webhook | PodKill | Resilient | |
| pipelines-webhook/network-partition | pipelines-webhook | NetworkPartition | Resilient | |
| pipelines-webhook/deployment-scale-zero | pipelines-webhook | DeploymentScaleZero | Resilient | |
| pipelines-webhook/crashloop-inject | pipelines-webhook | CrashLoopInject | Resilient | |
| pipelines-webhook/image-corrupt | pipelines-webhook | ImageCorrupt | Resilient | |
| pipelines-webhook/resource-deletion-service | pipelines-webhook | ResourceDeletion | Resilient | Service recreated in ~10s |
| pipelines-webhook/pdb-block | pipelines-webhook | PDBBlock | Resilient | |

## Key Findings

### Perfect Resilience Record

All 14 Tekton experiments passed with Resilient verdicts. Both the pipelines-controller and pipelines-webhook components demonstrate excellent fault tolerance.

### Service Deletion False Positive

The initial automated run reported Degraded for the webhook Service deletion experiment. Manual testing confirmed this was a false positive: the OpenShift Pipelines operator recreates the deleted Service within approximately 10 seconds. The evaluator flagged extra reconcile cycles during restoration, but the end state was fully recovered.

### OLM-Managed Recovery

As an OLM-managed operator, Tekton benefits from the OpenShift Pipelines operator's reconciliation loop. The operator actively manages the Tekton installation through a TektonConfig CR, which ensures Deployments, Services, and configuration are restored after disruption.


<!-- custom-start: analysis -->
<!-- custom-end: analysis -->
