# Prometheus Operator

!!! info "Built-in Operator"
    Prometheus Operator is managed by cluster-monitoring-operator and ships as part of the OpenShift platform. It is not installed via OLM. It manages Prometheus, Alertmanager, and related monitoring resources.

## Overview

| Property | Value |
|----------|-------|
| **Operator** | prometheus-operator |
| **Namespace** | openshift-monitoring |
| **Repository** | [prometheus-operator/prometheus-operator](https://github.com/prometheus-operator/prometheus-operator) |
| **Components** | 1 |
| **Reconcile Timeout** | 300s |
| **Max Reconcile Cycles** | 10 |

## Components

### prometheus-operator

**Controller:** cluster-monitoring-operator
**Namespace:** openshift-monitoring

#### Managed Resources

| API Version | Kind | Name | Namespace |
|-------------|------|------|-----------|
| apps/v1 | Deployment | prometheus-operator | openshift-monitoring |
| v1 | ServiceAccount | prometheus-operator | openshift-monitoring |
| v1 | Service | prometheus-operator | openshift-monitoring |

**Replicas:** 1

#### Steady-State Checks

| Type | Kind | Name | Namespace | Condition |
|------|------|------|-----------|-----------|
| conditionTrue | Deployment | prometheus-operator | openshift-monitoring | Available |

Timeout: 60s

## Results Summary

6 experiments tested, 6/6 Resilient. Fully reconciled by cluster-monitoring-operator.

| Verdict | Count | Notes |
|---------|-------|-------|
| Resilient | 6 | All experiments pass, cluster-monitoring-operator reconciles all disruptions |

## Pages

- [Failure Modes](failure-modes.md)
- [Validation Results](validation-results.md)
- [Custom Experiments](custom-experiments.md)

<!-- custom-start: notes -->
<!-- custom-end: notes -->
