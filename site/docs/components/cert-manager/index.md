# cert-manager

!!! info "External Operator"
    cert-manager is not an RHOAI/ODH component. It provides TLS certificate management used by multiple platform components.

## Overview

| Property | Value |
|----------|-------|
| **Operator** | cert-manager |
| **Namespace** | cert-manager |
| **Repository** | [https://github.com/cert-manager/cert-manager](https://github.com/cert-manager/cert-manager) |
| **Components** | 3 |
| **Reconcile Timeout** | 300s |
| **Max Reconcile Cycles** | 10 |

## Components

### cert-manager-controller

**Namespace:** cert-manager

#### Managed Resources

| API Version | Kind | Name | Namespace |
|-------------|------|------|-----------|
| apps/v1 | Deployment | cert-manager | cert-manager |

**Replicas:** 1  
**Label Selector:** app.kubernetes.io/name=cert-manager

The main cert-manager controller that reconciles Certificate, Issuer, and CertificateRequest resources.

#### Steady-State Checks

| Type | Kind | Name | Namespace | Condition |
|------|------|------|-----------|-----------|
| conditionTrue | Deployment | cert-manager | cert-manager | Available |

Timeout: 30s

---

### cert-manager-webhook

**Namespace:** cert-manager

#### Managed Resources

| API Version | Kind | Name | Namespace |
|-------------|------|------|-----------|
| apps/v1 | Deployment | cert-manager-webhook | cert-manager |

**Replicas:** 1  
**Label Selector:** app.kubernetes.io/name=webhook

The webhook validates and mutates cert-manager resources.

#### Steady-State Checks

| Type | Kind | Name | Namespace | Condition |
|------|------|------|-----------|-----------|
| conditionTrue | Deployment | cert-manager-webhook | cert-manager | Available |

Timeout: 30s

---

### cert-manager-cainjector

**Namespace:** cert-manager

#### Managed Resources

| API Version | Kind | Name | Namespace |
|-------------|------|------|-----------|
| apps/v1 | Deployment | cert-manager-cainjector | cert-manager |

**Replicas:** 1  
**Label Selector:** app.kubernetes.io/name=cainjector

The CA injector injects CA bundles into webhooks and API services.

#### Steady-State Checks

| Type | Kind | Name | Namespace | Condition |
|------|------|------|-----------|-----------|
| conditionTrue | Deployment | cert-manager-cainjector | cert-manager | Available |

Timeout: 30s


<!-- custom-start: notes -->
<!-- custom-end: notes -->
