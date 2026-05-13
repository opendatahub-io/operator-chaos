# Tekton (OpenShift Pipelines)

!!! info "External Operator"
    Tekton is the CI/CD engine behind OpenShift Pipelines. It is installed via OLM and provides cloud-native pipeline execution through custom resources (Task, Pipeline, PipelineRun).

## Overview

| Property | Value |
|----------|-------|
| **Operator** | tekton |
| **Namespace** | openshift-pipelines |
| **Repository** | [tektoncd/pipeline](https://github.com/tektoncd/pipeline) |
| **Components** | 2 |
| **Reconcile Timeout** | 300s |
| **Max Reconcile Cycles** | 10 |

## Components

### pipelines-controller

**Controller:** OLM (OpenShift Pipelines Operator)
**Namespace:** openshift-pipelines

#### Managed Resources

| API Version | Kind | Name | Namespace |
|-------------|------|------|-----------|
| apps/v1 | Deployment | tekton-pipelines-controller | openshift-pipelines |
| v1 | ServiceAccount | tekton-pipelines-controller | openshift-pipelines |
| v1 | Service | tekton-pipelines-controller | openshift-pipelines |

**Replicas:** 1

#### Steady-State Checks

| Type | Kind | Name | Namespace | Condition |
|------|------|------|-----------|-----------|
| conditionTrue | Deployment | tekton-pipelines-controller | openshift-pipelines | Available |

Timeout: 60s

---

### pipelines-webhook

**Controller:** OLM (OpenShift Pipelines Operator)
**Namespace:** openshift-pipelines

#### Managed Resources

| API Version | Kind | Name | Namespace |
|-------------|------|------|-----------|
| apps/v1 | Deployment | tekton-pipelines-webhook | openshift-pipelines |
| v1 | ServiceAccount | tekton-pipelines-webhook | openshift-pipelines |
| v1 | Service | tekton-pipelines-webhook | openshift-pipelines |

**Replicas:** 1

#### Steady-State Checks

| Type | Kind | Name | Namespace | Condition |
|------|------|------|-----------|-----------|
| conditionTrue | Deployment | tekton-pipelines-webhook | openshift-pipelines | Available |

Timeout: 60s

## Results Summary

14 experiments tested, ALL Resilient. Initial Service deletion Degraded was a false positive confirmed by manual testing (Service recreated in 10s).

| Verdict | Count | Notes |
|---------|-------|-------|
| Resilient | 14 | All experiments pass across both pipelines-controller and pipelines-webhook |

## Pages

- [Failure Modes](failure-modes.md)
- [Validation Results](validation-results.md)
- [Custom Experiments](custom-experiments.md)

<!-- custom-start: notes -->
<!-- custom-end: notes -->
