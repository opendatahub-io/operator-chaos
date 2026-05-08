# modelmesh

!!! warning "Removed in RHOAI 3.x"
    ModelMesh has been removed from RHOAI starting with version 3.0. These experiments are only applicable to ODH deployments or RHOAI 2.x.

## Overview

| Property | Value |
|----------|-------|
| **Operator** | modelmesh |
| **Namespace** | opendatahub |
| **Repository** | [https://github.com/kserve/modelmesh-serving](https://github.com/kserve/modelmesh-serving) |
| **Components** | 1 |
| **Reconcile Timeout** | 300s |
| **Max Reconcile Cycles** | 10 |

## Resource Summary

| Kind | Count |
|------|-------|
| ClusterRoleBinding | 1 |
| ConfigMap | 1 |
| Deployment | 1 |
| Lease | 1 |
| Service | 1 |
| ServiceAccount | 1 |
| **Total** | **6** |

## Components

### modelmesh-controller

**Controller:** DataScienceCluster
**Dependencies:** kserve-controller-manager

#### Managed Resources

| API Version | Kind | Name | Namespace |
|-------------|------|------|-----------|
| apps/v1 | Deployment | modelmesh-controller | opendatahub |
| v1 | ServiceAccount | modelmesh-controller | opendatahub |
| rbac.authorization.k8s.io/v1 | ClusterRoleBinding | modelmesh-controller-rolebinding |  |
| coordination.k8s.io/v1 | Lease | modelmesh-controller-leader-election | opendatahub |
| v1 | Service | modelmesh-controller-metrics-service | opendatahub |
| v1 | ConfigMap | modelmesh-serving-config | opendatahub |

#### Webhooks

| Name | Type | Path |
|------|------|------|
| vservingruntime.modelmesh.io | validating | `/validate-serving-kserve-io-v1alpha1-servingruntime` |
| mservingruntime.modelmesh.io | mutating | `/mutate-serving-kserve-io-v1alpha1-servingruntime` |

#### Finalizers
- `modelmesh.serving.kserve.io/finalizer`

#### Steady-State Checks

| Type | Kind | Name | Namespace | Condition |
|------|------|------|-----------|-----------|
| conditionTrue | Deployment | modelmesh-controller | opendatahub | Available |

Timeout: 60s


## Pages

- [Failure Modes](failure-modes.md)
- [Validation Results](validation-results.md)
- [Custom Experiments](custom-experiments.md)

<!-- custom-start: notes -->
<!-- custom-end: notes -->
