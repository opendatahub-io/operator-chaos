# Strimzi (AMQ Streams)

!!! info "External Operator"
    Strimzi is the upstream Kafka operator used by AMQ Streams. It is installed via OLM as a separate operator and manages Kafka clusters through custom resources.

## Overview

| Property | Value |
|----------|-------|
| **Operator** | strimzi |
| **Namespace** | openshift-operators |
| **Repository** | [strimzi/strimzi-kafka-operator](https://github.com/strimzi/strimzi-kafka-operator) |
| **Components** | 1 |
| **Reconcile Timeout** | 300s |
| **Max Reconcile Cycles** | 10 |

## Components

### cluster-operator

**Controller:** OLM

#### Managed Resources

| API Version | Kind | Name | Namespace |
|-------------|------|------|-----------|
| apps/v1 | Deployment | strimzi-cluster-operator | openshift-operators |
| v1 | ServiceAccount | strimzi-cluster-operator | openshift-operators |
| coordination.k8s.io/v1 | Lease | strimzi-cluster-operator | openshift-operators |

**Replicas:** 1
**Namespace:** openshift-operators

#### Steady-State Checks

| Type | Kind | Name | Namespace | Condition |
|------|------|------|-----------|-----------|
| conditionTrue | Deployment | strimzi-cluster-operator | openshift-operators | Available |

Timeout: 60s

## Results Summary

8 experiments tested, 7 Resilient, 1 Degraded.

| Verdict | Count | Notes |
|---------|-------|-------|
| Resilient | 7 | All core failure modes recover automatically |
| Degraded | 1 | DeploymentScaleZero: OLM does not restore replicas after scale-to-zero |

## Pages

- [Failure Modes](failure-modes.md)
- [Validation Results](validation-results.md)
- [Custom Experiments](custom-experiments.md)

<!-- custom-start: notes -->
<!-- custom-end: notes -->
