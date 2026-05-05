# llamastack Validation Results

## Results

<!-- custom-start: results -->
**Cluster:** RHOAI 3.3.2 / ROSA HyperShift 4.20.11 / 2026-05-05

| Experiment | Verdict | Recovery Time | Notes |
|---|---|---|---|
| pod-kill | Resilient | 1.5s | |
| network-partition | Failed | - | CrashLoopBackOff during partition prevents timely recovery |
| finalizer-block | Resilient | 1.4s | |
| rbac-revoke | Resilient | 1.3s | |

3/4 Resilient, 1 Failed. Same CrashLoopBackOff root cause as ray and other operators.
<!-- custom-end: results -->

## Known Issues

<!-- custom-start: known-issues -->
**NetworkPartition CrashLoopBackOff:** During network partitions, the operator pod enters CrashLoopBackOff due to kubelet exponential backoff, preventing recovery within the experiment timeout. This is a known issue affecting multiple RHOAI operators.
<!-- custom-end: known-issues -->
