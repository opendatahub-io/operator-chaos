# codeflare

!!! warning "Removed in RHOAI 3.x"
    CodeFlare has been removed from RHOAI starting with version 3.0. These experiments are only applicable to ODH deployments or RHOAI 2.x.

## Overview

| Property | Value |
|----------|-------|
| **Operator** | codeflare |
| **Namespace** | opendatahub |
| **Repository** | [https://github.com/project-codeflare/codeflare-operator](https://github.com/project-codeflare/codeflare-operator) |
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

### codeflare-operator-manager

**Controller:** DataScienceCluster
**Dependencies:** ray-operator-controller-manager

#### Managed Resources

| API Version | Kind | Name | Namespace |
|-------------|------|------|-----------|
| apps/v1 | Deployment | codeflare-operator-manager | opendatahub |
| v1 | ServiceAccount | codeflare-operator-manager | opendatahub |
| rbac.authorization.k8s.io/v1 | ClusterRoleBinding | codeflare-operator-manager-rolebinding |  |
| coordination.k8s.io/v1 | Lease | codeflare-operator-manager-leader-election | opendatahub |
| v1 | Service | codeflare-operator-manager-metrics-service | opendatahub |
| v1 | ConfigMap | codeflare-operator-config | opendatahub |

#### Webhooks

| Name | Type | Path |
|------|------|------|
| vappwrapper.codeflare.dev | validating | `/validate-codeflare-dev-v1beta2-appwrapper` |

#### Finalizers
- `codeflare.opendatahub.io/finalizer`

#### Steady-State Checks

| Type | Kind | Name | Namespace | Condition |
|------|------|------|-----------|-----------|
| conditionTrue | Deployment | codeflare-operator-manager | opendatahub | Available |

Timeout: 60s


<!-- custom-start: notes -->
<!-- custom-end: notes -->
