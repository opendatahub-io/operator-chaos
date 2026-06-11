# Usage Modes

Operator Chaos supports four distinct modes, each targeting a different phase of the development and testing lifecycle. All modes share the same injection engine and verdict model.

## Comparison

| | CLI | Transport | SDK | Controller | Fuzz |
|---|---|---|---|---|---|
| **Purpose** | Run experiments against a live cluster | Inject faults at HTTP transport layer | Inject faults into controller-runtime clients | Declarative CRD-driven chaos | Test reconciler logic with random faults |
| **Cluster required** | Yes | Yes | Yes (or fake client) | Yes | No |
| **Dependencies** | None (binary) | Zero (Go stdlib only) | controller-runtime v0.19.7+ | controller-runtime | controller-runtime |
| **When to use** | CI/CD, pre-release validation | Live cluster stress testing, operators with pinned k8s.io versions | Integration tests, operator development | GitOps-driven chaos, continuous validation | Development, unit tests, rapid iteration |
| **Input** | Experiment YAML files | 3 lines of Go + ConfigMap | Go code wrapping a client | ChaosExperiment CRDs | Reconciler function + knowledge model |
| **Intercepts** | N/A (external mutations) | All HTTP (informers, cache, CRUD, leader election) | CRUD only (Get, Update, Patch, etc.) | N/A (external mutations) | CRUD via ChaosClient |

## Choosing a Mode

**Start with Fuzz** during development to catch reconciler logic bugs without needing a cluster. Fuzz tests run in milliseconds and integrate with `go test`.

**Use CLI** for pre-release validation. Point it at a staging cluster, run your experiment suite, and gate your release on the verdicts.

**Use Transport** for live cluster stress testing, especially when your operator has k8s.io dependency versions incompatible with the SDK. The `chaostransport` sub-module has zero external dependencies and intercepts all HTTP traffic including informer watches and leader election.

**Use SDK** when you want fault injection as part of your existing integration test suite. The SDK wraps a controller-runtime client, so your reconciler code doesn't change.

**Use Controller** for continuous chaos in long-lived environments. Deploy the CRD and controller, and experiments run as Kubernetes-native resources with scheduling, TTLs, and status reporting.

## Mode Documentation

- [CLI Mode](cli.md): Full experiment lifecycle on a live cluster
- [Transport Mode](transport.md): Zero-dependency HTTP transport fault injection (chaostransport)
- [SDK Mode](sdk.md): Fault injection middleware for controller-runtime clients
- [Controller Mode](controller.md): CRD-driven declarative chaos
- [Fuzz Mode](fuzz.md): Offline reconciler testing with random fault sequences
