# Strimzi Custom Experiments

This page provides templates and guidance for writing custom chaos experiments targeting Strimzi components.

## Component Overview

Strimzi has 1 component in the openshift-operators namespace:

- **cluster-operator**: Main controller for Kafka, KafkaTopic, KafkaUser, and related CRs

## Example Template

### cluster-operator

```yaml
apiVersion: chaos.operatorchaos.io/v1alpha1
kind: ChaosExperiment
metadata:
  name: strimzi-cluster-operator-custom
spec:
  target:
    operator: strimzi
    component: cluster-operator
  steadyState:
    checks:
      - type: conditionTrue
        apiVersion: apps/v1
        kind: Deployment
        name: strimzi-cluster-operator
        namespace: openshift-operators
        conditionType: Available
    timeout: "60s"
  injection:
    type: PodKill  # Change to desired injection type
    parameters:
      labelSelector: strimzi.io/kind=cluster-operator
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

- **Kafka clusters are independent**: Killing or partitioning the cluster-operator does not affect running Kafka brokers. Experiments validate operator-layer recovery, not data-plane availability.
- **OLM reconciliation**: Strimzi is OLM-managed. OLM handles CSV, Subscription, and Deployment lifecycle but does not restore replica counts (DeploymentScaleZero).
- **Leader election**: The cluster-operator uses a Lease for leader election. Disrupting the Lease forces re-election but does not cause data loss.


<!-- custom-start: examples -->
<!-- custom-end: examples -->
