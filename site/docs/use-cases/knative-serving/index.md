# knative-serving

!!! info "External Operator"
    Knative Serving (OpenShift Serverless) is not an RHOAI/ODH component. It is the serverless runtime used by KServe for inference serving. These experiments test the infrastructure layer that KServe depends on.

## Overview

| Property | Value |
|----------|-------|
| **Operator** | knative-serving |
| **Namespaces** | knative-serving, knative-serving-ingress |
| **Repository** | [https://github.com/openshift-knative/serverless-operator](https://github.com/openshift-knative/serverless-operator) |
| **Components** | 7 |
| **Reconcile Timeout** | 300s |
| **Max Reconcile Cycles** | 10 |

## Components

### activator

**Namespace:** knative-serving

#### Managed Resources

| API Version | Kind | Name | Namespace |
|-------------|------|------|-----------|
| apps/v1 | Deployment | activator | knative-serving |

**Replicas:** 2  
**Label Selector:** app=activator

The activator is the request-buffering proxy that holds traffic during scale-from-zero.

#### Steady-State Checks

| Type | Kind | Name | Namespace | Condition |
|------|------|------|-----------|-----------|
| conditionTrue | Deployment | activator | knative-serving | Available |

Timeout: 30s

---

### autoscaler

**Namespace:** knative-serving

#### Managed Resources

| API Version | Kind | Name | Namespace |
|-------------|------|------|-----------|
| apps/v1 | Deployment | autoscaler | knative-serving |

**Replicas:** 2  
**Label Selector:** app=autoscaler

The autoscaler makes scale decisions based on metrics from the activator.

#### Steady-State Checks

| Type | Kind | Name | Namespace | Condition |
|------|------|------|-----------|-----------|
| conditionTrue | Deployment | autoscaler | knative-serving | Available |

Timeout: 30s

---

### autoscaler-hpa

**Namespace:** knative-serving

#### Managed Resources

| API Version | Kind | Name | Namespace |
|-------------|------|------|-----------|
| apps/v1 | Deployment | autoscaler-hpa | knative-serving |

**Replicas:** 2  
**Label Selector:** app=autoscaler-hpa

The HPA-based autoscaler for Knative Serving workloads.

#### Steady-State Checks

| Type | Kind | Name | Namespace | Condition |
|------|------|------|-----------|-----------|
| conditionTrue | Deployment | autoscaler-hpa | knative-serving | Available |

Timeout: 30s

---

### controller

**Namespace:** knative-serving

#### Managed Resources

| API Version | Kind | Name | Namespace |
|-------------|------|------|-----------|
| apps/v1 | Deployment | controller | knative-serving |

**Replicas:** 2  
**Label Selector:** app=controller

The main Knative Serving controller that reconciles Service, Route, Configuration, and Revision resources.

#### Webhooks

| Name | Type | Path |
|------|------|------|
| validation.webhook.serving.knative.dev | validating | `/resource-validation` |
| webhook.serving.knative.dev | mutating | `/defaulting` |
| config.webhook.serving.knative.dev | validating | `/config-validation` |

#### RBAC

- knative-serving-controller-admin
- knative-serving-controller-addressable-resolver

#### Steady-State Checks

| Type | Kind | Name | Namespace | Condition |
|------|------|------|-----------|-----------|
| conditionTrue | Deployment | controller | knative-serving | Available |

Timeout: 30s

---

### webhook

**Namespace:** knative-serving

#### Managed Resources

| API Version | Kind | Name | Namespace |
|-------------|------|------|-----------|
| apps/v1 | Deployment | webhook | knative-serving |

**Replicas:** 2  
**Label Selector:** app=webhook

The webhook handles validation and mutation of Knative Serving resources.

#### Steady-State Checks

| Type | Kind | Name | Namespace | Condition |
|------|------|------|-----------|-----------|
| conditionTrue | Deployment | webhook | knative-serving | Available |

Timeout: 30s

---

### kourier-gateway

**Namespace:** knative-serving-ingress

#### Managed Resources

| API Version | Kind | Name | Namespace |
|-------------|------|------|-----------|
| apps/v1 | Deployment | 3scale-kourier-gateway | knative-serving-ingress |

**Replicas:** 2  
**Label Selector:** app=3scale-kourier-gateway

The Envoy-based ingress gateway for all Knative Serving traffic.

#### Steady-State Checks

| Type | Kind | Name | Namespace | Condition |
|------|------|------|-----------|-----------|
| conditionTrue | Deployment | 3scale-kourier-gateway | knative-serving-ingress | Available |

Timeout: 30s

---

### net-kourier-controller

**Namespace:** knative-serving-ingress

#### Managed Resources

| API Version | Kind | Name | Namespace |
|-------------|------|------|-----------|
| apps/v1 | Deployment | net-kourier-controller | knative-serving-ingress |

**Replicas:** 2  
**Label Selector:** app=net-kourier-controller

The controller that programs Envoy routes for Knative Services.

#### RBAC

- net-kourier

#### Steady-State Checks

| Type | Kind | Name | Namespace | Condition |
|------|------|------|-----------|-----------|
| conditionTrue | Deployment | net-kourier-controller | knative-serving-ingress | Available |

Timeout: 30s


## Pages

- [Failure Modes](failure-modes.md)
- [Validation Results](validation-results.md)
- [Custom Experiments](custom-experiments.md)

<!-- custom-start: notes -->
<!-- custom-end: notes -->
