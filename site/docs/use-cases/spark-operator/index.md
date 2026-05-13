# Spark Operator (Kubeflow)

!!! info "External Operator"
    The Spark Operator is a Helm/kustomize-managed operator from [kubeflow/spark-operator](https://github.com/kubeflow/spark-operator). ODH maintains a fork at [opendatahub-io/spark-operator](https://github.com/opendatahub-io/spark-operator). It manages SparkApplication lifecycle on Kubernetes.

## Overview

| Property | Value |
|----------|-------|
| **Operator** | spark-operator |
| **Namespace** | spark-operator |
| **Repository** | [kubeflow/spark-operator](https://github.com/kubeflow/spark-operator) |
| **Components** | 2 |
| **Reconcile Timeout** | 300s |
| **Max Reconcile Cycles** | 10 |

## Components

### controller

**Namespace:** spark-operator

#### Managed Resources

| API Version | Kind | Name | Namespace |
|-------------|------|------|-----------|
| apps/v1 | Deployment | spark-operator-controller | spark-operator |
| v1 | ServiceAccount | spark-operator-controller | spark-operator |
| coordination.k8s.io/v1 | Lease | spark-operator-controller | spark-operator |

**Replicas:** 1

#### Steady-State Checks

| Type | Kind | Name | Namespace | Condition |
|------|------|------|-----------|-----------|
| conditionTrue | Deployment | spark-operator-controller | spark-operator | Available |

Timeout: 60s

---

### webhook

**Namespace:** spark-operator

#### Managed Resources

| API Version | Kind | Name | Namespace |
|-------------|------|------|-----------|
| apps/v1 | Deployment | spark-operator-webhook | spark-operator |
| v1 | ServiceAccount | spark-operator-webhook | spark-operator |
| v1 | Service | spark-operator-webhook | spark-operator |
| coordination.k8s.io/v1 | Lease | spark-operator-webhook | spark-operator |

**Replicas:** 1

#### Webhooks

| Name | Type | Path |
|------|------|------|
| spark-operator-webhook | validating | `/validate` |
| spark-operator-webhook | mutating | `/mutate` |

#### Steady-State Checks

| Type | Kind | Name | Namespace | Condition |
|------|------|------|-----------|-----------|
| conditionTrue | Deployment | spark-operator-webhook | spark-operator | Available |

Timeout: 60s

## Results Summary

12 experiments tested. Verified findings: NetworkPartition Failed (controller permanently non-functional), DeploymentScaleZero Degraded (both components). Other experiments Resilient.

| Verdict | Count | Notes |
|---------|-------|-------|
| Resilient | 9 | Core failure modes recover automatically |
| Degraded | 2 | DeploymentScaleZero for both controller and webhook |
| Failed | 1 | NetworkPartition leaves controller permanently non-functional |

## Pages

- [Failure Modes](failure-modes.md)
- [Validation Results](validation-results.md)
- [Custom Experiments](custom-experiments.md)

<!-- custom-start: notes -->
<!-- custom-end: notes -->
