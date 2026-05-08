# Usage Modes

Operator Chaos supports four distinct modes, each targeting a different phase of the development and testing lifecycle. All modes share the same injection engine and verdict model.

## Comparison

| | CLI | SDK | Controller | Fuzz |
|---|---|---|---|---|
| **Purpose** | Run experiments against a live cluster | Inject faults into controller-runtime clients | Declarative CRD-driven chaos | Test reconciler logic with random faults |
| **Cluster required** | Yes | Yes (or fake client) | Yes | No |
| **When to use** | CI/CD, pre-release validation, one-off testing | Integration tests, operator development | GitOps-driven chaos, continuous validation | Development, unit tests, rapid iteration |
| **Input** | Experiment YAML files | Go code wrapping a client | ChaosExperiment CRDs | Reconciler function + knowledge model |
| **Execution** | Single binary, runs to completion | Embedded in test suite | Kubernetes controller loop | In-process, no network calls |
| **Verdict** | CLI output + JSON report | Programmatic result | CRD status field | Test assertion pass/fail |

## Choosing a Mode

**Start with Fuzz** during development to catch reconciler logic bugs without needing a cluster. Fuzz tests run in milliseconds and integrate with `go test`.

**Use CLI** for pre-release validation. Point it at a staging cluster, run your experiment suite, and gate your release on the verdicts.

**Use SDK** when you want fault injection as part of your existing integration test suite. The SDK wraps a controller-runtime client, so your reconciler code doesn't change.

**Use Controller** for continuous chaos in long-lived environments. Deploy the CRD and controller, and experiments run as Kubernetes-native resources with scheduling, TTLs, and status reporting.

## Mode Documentation

- [CLI Mode](cli.md): Full experiment lifecycle on a live cluster
- [SDK Mode](sdk.md): Fault injection middleware for controller-runtime clients
- [Controller Mode](controller.md): CRD-driven declarative chaos
- [Fuzz Mode](fuzz.md): Offline reconciler testing with random fault sequences
