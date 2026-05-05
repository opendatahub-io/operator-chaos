# ray Validation Results

## Results

<!-- custom-start: results -->
**Cluster:** RHOAI 3.3.2 / ROSA HyperShift 4.20.11 / 2026-05-05

| Experiment | Verdict | Recovery Time | Notes |
|---|---|---|---|
| pod-kill | Resilient | 1.3s | |
| network-partition | Failed | - | CrashLoopBackOff during partition prevents timely recovery |
| finalizer-block | Resilient | 1.5s | |
| rbac-revoke | Resilient | 1.2s | |
| webhook-disrupt | Resilient | 1.2s | |

4/5 Resilient, 1 Failed. The network-partition failure is the same CrashLoopBackOff root cause seen across multiple RHOAI operators.
<!-- custom-end: results -->

## Known Issues

<!-- custom-start: known-issues -->
**NetworkPartition CrashLoopBackOff:** During network partitions, the operator pod enters CrashLoopBackOff due to kubelet exponential backoff, preventing recovery within the experiment timeout. This is a known issue affecting multiple RHOAI operators.
<!-- custom-end: known-issues -->
