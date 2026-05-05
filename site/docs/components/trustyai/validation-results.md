# trustyai Validation Results

## Results

<!-- custom-start: results -->
**Cluster:** RHOAI 3.3.2 / ROSA HyperShift 4.20.11 / 2026-05-05

| Experiment | Verdict | Recovery Time | Notes |
|---|---|---|---|
| pod-kill | Resilient | 1.2s | |
| network-partition | Resilient | 1.2s | |
| finalizer-block | Resilient | 1.2s | |
| rbac-revoke | Resilient | 1.3s | |

4/4 Resilient. All experiments recovered in under 2 seconds.
<!-- custom-end: results -->

## Known Issues

<!-- custom-start: known-issues -->
<!-- custom-end: known-issues -->
