# Tekton Custom Experiments

This page provides templates and guidance for writing custom chaos experiments targeting Tekton components.

## Component Overview

Tekton has 2 components in the openshift-pipelines namespace:

- **pipelines-controller**: Main controller for Task, Pipeline, PipelineRun, and TaskRun reconciliation
- **pipelines-webhook**: Validates and mutates Tekton resources

## Example Templates

### pipelines-controller

```yaml
apiVersion: chaos.operatorchaos.io/v1alpha1
kind: ChaosExperiment
metadata:
  name: tekton-controller-custom
spec:
  target:
    operator: tekton
    component: pipelines-controller
  steadyState:
    checks:
      - type: conditionTrue
        apiVersion: apps/v1
        kind: Deployment
        name: tekton-pipelines-controller
        namespace: openshift-pipelines
        conditionType: Available
    timeout: "60s"
  injection:
    type: PodKill  # Change to desired injection type
    parameters:
      labelSelector: app.kubernetes.io/name=controller
    ttl: "120s"
  hypothesis:
    description: >-
      Describe the expected behavior after fault injection.
    recoveryTimeout: 120s
```

### pipelines-webhook

```yaml
apiVersion: chaos.operatorchaos.io/v1alpha1
kind: ChaosExperiment
metadata:
  name: tekton-webhook-custom
spec:
  target:
    operator: tekton
    component: pipelines-webhook
  steadyState:
    checks:
      - type: conditionTrue
        apiVersion: apps/v1
        kind: Deployment
        name: tekton-pipelines-webhook
        namespace: openshift-pipelines
        conditionType: Available
    timeout: "60s"
  injection:
    type: PodKill
    parameters:
      labelSelector: app.kubernetes.io/name=webhook
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

- **TektonConfig CR**: The OpenShift Pipelines operator manages Tekton through a TektonConfig CR. This provides an additional layer of reconciliation beyond standard Deployment controllers.
- **Running PipelineRuns**: Killing the controller does not terminate running PipelineRun pods. The pods continue executing their steps. However, no new steps are scheduled and status updates stall until the controller recovers.
- **Webhook dependency**: Task and Pipeline creation depends on the webhook. Experiments targeting the webhook should verify that Tekton resource creation fails gracefully.


<!-- custom-start: examples -->
<!-- custom-end: examples -->
