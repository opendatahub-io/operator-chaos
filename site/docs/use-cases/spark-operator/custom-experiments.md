# Spark Operator Custom Experiments

This page provides templates and guidance for writing custom chaos experiments targeting Spark operator components.

## Component Overview

The Spark operator has 2 components in the spark-operator namespace:

- **controller**: Main controller for SparkApplication reconciliation
- **webhook**: Validates and mutates SparkApplication resources

## Example Templates

### controller

```yaml
apiVersion: chaos.operatorchaos.io/v1alpha1
kind: ChaosExperiment
metadata:
  name: spark-controller-custom
spec:
  target:
    operator: spark-operator
    component: controller
  steadyState:
    checks:
      - type: conditionTrue
        apiVersion: apps/v1
        kind: Deployment
        name: spark-operator-controller
        namespace: spark-operator
        conditionType: Available
    timeout: "60s"
  injection:
    type: PodKill  # Change to desired injection type
    parameters:
      labelSelector: app.kubernetes.io/name=spark-operator-controller
    ttl: "120s"
  hypothesis:
    description: >-
      Describe the expected behavior after fault injection.
    recoveryTimeout: 120s
```

### webhook

```yaml
apiVersion: chaos.operatorchaos.io/v1alpha1
kind: ChaosExperiment
metadata:
  name: spark-webhook-custom
spec:
  target:
    operator: spark-operator
    component: webhook
  steadyState:
    checks:
      - type: conditionTrue
        apiVersion: apps/v1
        kind: Deployment
        name: spark-operator-webhook
        namespace: spark-operator
        conditionType: Available
    timeout: "60s"
  injection:
    type: PodKill
    parameters:
      labelSelector: app.kubernetes.io/name=spark-operator-webhook
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

- **NetworkPartition is destructive**: The controller does not recover from network partitions without a pod restart. Avoid running NetworkPartition experiments in production environments.
- **Helm-managed**: There is no OLM or higher-level operator to restore state. DeploymentScaleZero and similar experiments require manual recovery.
- **Webhook dependency**: SparkApplication creation depends on the webhook. Experiments targeting the webhook should verify that admission fails gracefully.


<!-- custom-start: examples -->
<!-- custom-end: examples -->
