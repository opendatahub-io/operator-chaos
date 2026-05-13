# ArgoCD Custom Experiments

This page provides templates and guidance for writing custom chaos experiments targeting ArgoCD components.

## Component Overview

ArgoCD has 2 components in the openshift-gitops namespace:

- **server**: The ArgoCD API server and UI, handles sync operations and application management
- **repo-server**: Fetches Git repositories and generates Kubernetes manifests

## Example Templates

### server

```yaml
apiVersion: chaos.operatorchaos.io/v1alpha1
kind: ChaosExperiment
metadata:
  name: argocd-server-custom
spec:
  target:
    operator: argocd
    component: server
  steadyState:
    checks:
      - type: conditionTrue
        apiVersion: apps/v1
        kind: Deployment
        name: openshift-gitops-server
        namespace: openshift-gitops
        conditionType: Available
    timeout: "60s"
  injection:
    type: PodKill  # Change to desired injection type
    parameters:
      labelSelector: app.kubernetes.io/name=openshift-gitops-server
    ttl: "120s"
  hypothesis:
    description: >-
      Describe the expected behavior after fault injection.
    recoveryTimeout: 120s
```

### repo-server

```yaml
apiVersion: chaos.operatorchaos.io/v1alpha1
kind: ChaosExperiment
metadata:
  name: argocd-repo-server-custom
spec:
  target:
    operator: argocd
    component: repo-server
  steadyState:
    checks:
      - type: conditionTrue
        apiVersion: apps/v1
        kind: Deployment
        name: openshift-gitops-repo-server
        namespace: openshift-gitops
        conditionType: Available
    timeout: "60s"
  injection:
    type: PodKill
    parameters:
      labelSelector: app.kubernetes.io/name=openshift-gitops-repo-server
    ttl: "120s"
  hypothesis:
    description: >-
      Describe the expected behavior after fault injection.
    recoveryTimeout: 120s
```

## Running Custom Experiments

1. Save your experiment YAML to a file
2. Run: `chaos-cli run --experiment <file>`
3. Check results: `chaos-cli results --latest`

## Design Considerations

- **GitOps operator manages ArgoCD**: Unlike standalone ArgoCD, OpenShift GitOps uses a higher-level operator that reconciles the ArgoCD CR. This means DeploymentScaleZero and similar experiments may recover automatically through the GitOps operator.
- **Sync operations vs. control plane**: Disrupting the server affects sync operations and the UI, but already-deployed resources continue running. The repo-server is needed for manifest generation from Git.
- **Evaluator noise**: Some experiments may report extra reconcile cycles due to evaluator timing. If results look marginal, verify with manual testing.


<!-- custom-start: examples -->
<!-- custom-end: examples -->
