# Prometheus Operator Custom Experiments

This page provides templates and guidance for writing custom chaos experiments targeting the Prometheus Operator.

## Component Overview

The Prometheus Operator has 1 component in the openshift-monitoring namespace:

- **prometheus-operator**: Manages Prometheus, Alertmanager, ThanosRuler, and related monitoring resources via custom resources (Prometheus, ServiceMonitor, PrometheusRule, Alertmanager)

## Example Template

### prometheus-operator

```yaml
apiVersion: chaos.operatorchaos.io/v1alpha1
kind: ChaosExperiment
metadata:
  name: prometheus-operator-custom
spec:
  target:
    operator: prometheus-operator
    component: prometheus-operator
  steadyState:
    checks:
      - type: conditionTrue
        apiVersion: apps/v1
        kind: Deployment
        name: prometheus-operator
        namespace: openshift-monitoring
        conditionType: Available
    timeout: "60s"
  injection:
    type: PodKill  # Change to desired injection type
    parameters:
      labelSelector: app.kubernetes.io/name=prometheus-operator
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

- **cluster-monitoring-operator**: The Prometheus Operator is managed by cluster-monitoring-operator, which provides aggressive reconciliation. Most disruptions are self-healed, including DeploymentScaleZero.
- **Monitoring data plane**: Disrupting the Prometheus Operator does not affect running Prometheus instances. Metrics scraping and alerting continue. Only configuration reconciliation (new ServiceMonitors, PrometheusRules) stalls.
- **openshift-monitoring namespace**: This namespace has strict RBAC and may require elevated permissions to apply chaos experiments. Ensure your chaos service account has the necessary permissions.
- **Platform component**: Be cautious with experiments in production clusters, as disrupting monitoring can mask other issues.


<!-- custom-start: examples -->
<!-- custom-end: examples -->
