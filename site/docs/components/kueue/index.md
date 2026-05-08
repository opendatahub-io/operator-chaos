# kueue

!!! info "External Operator"
    Starting with RHOAI 3.x, Kueue is no longer managed as a DSC component. It is now the [Red Hat Build of Kueue Operator](https://docs.redhat.com/en/documentation/red_hat_openshift_ai_self-managed/3.4/html/managing_openshift_ai/managing-workloads-with-kueue), an OLM-managed operator installed separately. The kueue-operator and kueue-operand experiments target this OLM-managed deployment. Legacy experiments in the root kueue directory target DSC-managed Kueue for ODH or RHOAI 2.x.

## Overview

The project now tests Kueue resilience at three layers:

1. **Legacy DSC-managed Kueue** (5 experiments): The original ODH/RHOAI 2.x deployment model where Kueue was managed by the DataScienceCluster controller in the opendatahub namespace.

2. **OLM-managed Kueue Operator** (11 experiments): The Red Hat Build of Kueue Operator installed via OLM in openshift-kueue-operator namespace. Tests operator-layer resilience (OLM subscription, CSV, CRDs, operator deployment).

3. **Kueue Operand** (14 experiments): The Kueue controller-manager and its CRs (ClusterQueues, LocalQueues, ResourceFlavors, Workloads). Tests workload admission control resilience independent of deployment method.

## Legacy DSC-managed Kueue (RHOAI 2.x / ODH)

| Property | Value |
|----------|-------|
| **Operator** | kueue |
| **Namespace** | opendatahub |
| **Repository** | [https://github.com/kubernetes-sigs/kueue](https://github.com/kubernetes-sigs/kueue) |
| **Components** | 1 |
| **Reconcile Timeout** | 300s |
| **Max Reconcile Cycles** | 10 |

### Resource Summary

| Kind | Count |
|------|-------|
| ClusterRoleBinding | 1 |
| Deployment | 1 |
| Lease | 1 |
| Service | 1 |
| ServiceAccount | 1 |
| **Total** | **5** |

### kueue-controller-manager

**Controller:** DataScienceCluster

#### Managed Resources

| API Version | Kind | Name | Namespace |
|-------------|------|------|-----------|
| apps/v1 | Deployment | kueue-controller-manager | opendatahub |
| v1 | ServiceAccount | kueue-controller-manager | opendatahub |
| rbac.authorization.k8s.io/v1 | ClusterRoleBinding | kueue-controller-manager-rolebinding |  |
| coordination.k8s.io/v1 | Lease | kueue-controller-manager-leader-election | opendatahub |
| v1 | Service | kueue-controller-manager-metrics-service | opendatahub |

#### Webhooks

| Name | Type | Path |
|------|------|------|
| vworkload.kb.io | validating | `/validate-kueue-x-k8s-io-v1beta1-workload` |
| vclusterqueue.kb.io | validating | `/validate-kueue-x-k8s-io-v1beta1-clusterqueue` |
| vlocalqueue.kb.io | validating | `/validate-kueue-x-k8s-io-v1beta1-localqueue` |
| vresourceflavor.kb.io | validating | `/validate-kueue-x-k8s-io-v1beta1-resourceflavor` |
| mworkload.kb.io | mutating | `/mutate-kueue-x-k8s-io-v1beta1-workload` |

#### Finalizers
- `kueue.x-k8s.io/managed-resources`

#### Steady-State Checks

| Type | Kind | Name | Namespace | Condition |
|------|------|------|-----------|-----------|
| conditionTrue | Deployment | kueue-controller-manager | opendatahub | Available |

Timeout: 60s

## Red Hat Build of Kueue Operator (RHOAI 3.x)

| Property | Value |
|----------|-------|
| **Operator** | kueue-operator |
| **Namespace** | openshift-kueue-operator |
| **Repository** | [https://github.com/openshift/kueue-operator](https://github.com/openshift/kueue-operator) |
| **Components** | 1 |
| **Reconcile Timeout** | 300s |
| **Max Reconcile Cycles** | 10 |

### kueue-operator

**Controller:** OLM

#### Managed Resources

| API Version | Kind | Name | Namespace |
|-------------|------|------|-----------|
| apps/v1 | Deployment | openshift-kueue-operator | openshift-kueue-operator |
| v1 | ServiceAccount | openshift-kueue-operator | openshift-kueue-operator |
| coordination.k8s.io/v1 | Lease | openshift-kueue-operator-lock | openshift-kueue-operator |

**Replicas:** 2  
**Namespace:** openshift-kueue-operator

#### Steady-State Checks

| Type | Kind | Name | Namespace | Condition |
|------|------|------|-----------|-----------|
| conditionTrue | Deployment | openshift-kueue-operator | openshift-kueue-operator | Available |

Timeout: 60s


## Pages

- [Failure Modes](failure-modes.md)
- [Validation Results](validation-results.md)
- [Custom Experiments](custom-experiments.md)

<!-- custom-start: notes -->
<!-- custom-end: notes -->
