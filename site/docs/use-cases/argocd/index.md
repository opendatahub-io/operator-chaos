# ArgoCD (OpenShift GitOps)

!!! info "External Operator"
    ArgoCD is the core engine of OpenShift GitOps. It is installed via OLM and manages continuous delivery of Kubernetes resources from Git repositories.

## Overview

| Property | Value |
|----------|-------|
| **Operator** | argocd |
| **Namespace** | openshift-gitops |
| **Repository** | [argoproj/argo-cd](https://github.com/argoproj/argo-cd) |
| **Components** | 2 |
| **Reconcile Timeout** | 300s |
| **Max Reconcile Cycles** | 10 |

## Components

### server

**Controller:** OLM (OpenShift GitOps Operator)
**Namespace:** openshift-gitops

#### Managed Resources

| API Version | Kind | Name | Namespace |
|-------------|------|------|-----------|
| apps/v1 | Deployment | openshift-gitops-server | openshift-gitops |
| v1 | ServiceAccount | openshift-gitops-argocd-server | openshift-gitops |
| v1 | Service | openshift-gitops-server | openshift-gitops |

**Replicas:** 1

#### Steady-State Checks

| Type | Kind | Name | Namespace | Condition |
|------|------|------|-----------|-----------|
| conditionTrue | Deployment | openshift-gitops-server | openshift-gitops | Available |

Timeout: 60s

---

### repo-server

**Controller:** OLM (OpenShift GitOps Operator)
**Namespace:** openshift-gitops

#### Managed Resources

| API Version | Kind | Name | Namespace |
|-------------|------|------|-----------|
| apps/v1 | Deployment | openshift-gitops-repo-server | openshift-gitops |
| v1 | ServiceAccount | openshift-gitops-argocd-repo-server | openshift-gitops |
| v1 | Service | openshift-gitops-repo-server | openshift-gitops |

**Replicas:** 1

#### Steady-State Checks

| Type | Kind | Name | Namespace | Condition |
|------|------|------|-----------|-----------|
| conditionTrue | Deployment | openshift-gitops-repo-server | openshift-gitops | Available |

Timeout: 60s

## Results Summary

16 experiments tested, ALL Resilient. Initial Degraded verdicts for some experiments were false positives from evaluator cycle counting noise, confirmed by manual testing.

| Verdict | Count | Notes |
|---------|-------|-------|
| Resilient | 16 | All experiments pass across both server and repo-server |

## Pages

- [Failure Modes](failure-modes.md)
- [Validation Results](validation-results.md)
- [Custom Experiments](custom-experiments.md)

<!-- custom-start: notes -->
<!-- custom-end: notes -->
