# OpenShift Service Mesh

!!! info "External Operator"
    OpenShift Service Mesh 3.x is based on the [Sail Operator](https://github.com/openshift-service-mesh/sail-operator) (upstream Istio). It is installed via OLM as a separate operator and manages Istio control plane instances through the `Istio` CR.

## Overview

The project tests Service Mesh resilience at two layers:

1. **Service Mesh Operator** (6 experiments): The OLM-managed operator (`servicemesh-operator3`) in `openshift-operators`. Tests operator-layer resilience (OLM subscription, CSV, deployment, leader election).

2. **Istiod Control Plane** (12 experiments): The `istiod-openshift-gateway` deployment and its webhooks in `openshift-ingress`. Tests control plane resilience (config distribution, sidecar injection, validation, ownership, finalizers).

## Service Mesh Operator (OSSM 3.x)

| Property | Value |
|----------|-------|
| **Operator** | service-mesh |
| **Namespace** | openshift-operators |
| **Repository** | [openshift-service-mesh/sail-operator](https://github.com/openshift-service-mesh/sail-operator) |
| **Components** | 2 |
| **Reconcile Timeout** | 300s |
| **Max Reconcile Cycles** | 10 |

### servicemesh-operator3

**Controller:** OLM

#### Managed Resources

| API Version | Kind | Name | Namespace |
|-------------|------|------|-----------|
| apps/v1 | Deployment | servicemesh-operator3 | openshift-operators |
| v1 | ServiceAccount | servicemesh-operator3 | openshift-operators |
| v1 | Service | servicemesh-operator3-metrics-service | openshift-operators |
| coordination.k8s.io/v1 | Lease | sail-operator-lock | openshift-operators |

**Replicas:** 1
**Namespace:** openshift-operators

#### Steady-State Checks

| Type | Kind | Name | Namespace | Condition |
|------|------|------|-----------|-----------|
| conditionTrue | Deployment | servicemesh-operator3 | openshift-operators | Available |

Timeout: 60s

### istiod

**Controller:** servicemesh-operator3

#### Managed Resources

| API Version | Kind | Name | Namespace |
|-------------|------|------|-----------|
| apps/v1 | Deployment | istiod-openshift-gateway | openshift-ingress |
| v1 | ServiceAccount | istiod-openshift-gateway | openshift-ingress |
| v1 | Service | istiod-openshift-gateway | openshift-ingress |
| sailoperator.io/v1 | Istio | openshift-gateway | openshift-ingress |

**Replicas:** 1
**Namespace:** openshift-ingress

#### Webhooks

| Name | Type | Path |
|------|------|------|
| istio-validator-openshift-gateway-openshift-ingress | validating | `/validate` |
| istio-sidecar-injector-openshift-gateway-openshift-ingress | mutating | `/inject` |

#### Steady-State Checks

| Type | Kind | Name | Namespace | Condition |
|------|------|------|-----------|-----------|
| conditionTrue | Deployment | istiod-openshift-gateway | openshift-ingress | Available |
| conditionTrue | Istio | openshift-gateway | openshift-ingress | Ready |

Timeout: 60s


<!-- custom-start: notes -->
<!-- custom-end: notes -->
