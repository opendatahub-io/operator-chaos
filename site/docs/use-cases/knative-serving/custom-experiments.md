# knative-serving Custom Experiments

This page provides templates and guidance for writing custom chaos experiments targeting Knative Serving components.

## Component Overview

Knative Serving has 7 main components across two namespaces:

### knative-serving namespace

- **activator**: Request buffering proxy for scale-from-zero (label: `app=activator`)
- **autoscaler**: Makes scaling decisions based on metrics (label: `app=autoscaler`)
- **autoscaler-hpa**: HPA-based autoscaler (label: `app=autoscaler-hpa`)
- **controller**: Main controller for Service/Route/Configuration resources (label: `app=controller`)
- **webhook**: Validation and mutation webhook (label: `app=webhook`)

### knative-serving-ingress namespace

- **kourier-gateway**: Envoy-based ingress gateway (label: `app=3scale-kourier-gateway`)
- **net-kourier-controller**: Programs Envoy routes (label: `app=net-kourier-controller`)

## Key Architectural Relationships

Understanding these relationships helps design meaningful experiments:

1. **activator → autoscaler WebSocket**: The activator maintains a persistent WebSocket connection to the autoscaler for metric streaming. Network partitions that break this connection can cause health check failures.

2. **controller → leader election leases**: The controller uses leader election (Lease resources in knative-serving). Experiments that disrupt API server connectivity or RBAC can trigger leader failover.

3. **net-kourier-controller → kourier-gateway Envoy config**: The controller programs Envoy routes via ConfigMap updates. Disrupting the controller blocks new route creation but doesn't affect existing routes.

4. **webhook → cert-manager**: The webhook's TLS certificates are managed by cert-manager. Certificate corruption triggers automatic regeneration.

## Example Templates

### activator

```yaml
apiVersion: chaos.operatorchaos.io/v1alpha1
kind: ChaosExperiment
metadata:
  name: knative-activator-custom
spec:
  target:
    operator: knative-serving
    component: activator
  steadyState:
    checks:
      - type: conditionTrue
        apiVersion: apps/v1
        kind: Deployment
        name: activator
        namespace: knative-serving
        conditionType: Available
    timeout: "30s"
  injection:
    type: PodKill  # Change to desired injection type
    parameters:
      labelSelector: app=activator
    ttl: "120s"
  hypothesis:
    description: >-
      Describe the expected behavior after fault injection.
    recoveryTimeout: 120s
```

### controller

```yaml
apiVersion: chaos.operatorchaos.io/v1alpha1
kind: ChaosExperiment
metadata:
  name: knative-controller-custom
spec:
  target:
    operator: knative-serving
    component: controller
  steadyState:
    checks:
      - type: conditionTrue
        apiVersion: apps/v1
        kind: Deployment
        name: controller
        namespace: knative-serving
        conditionType: Available
    timeout: "30s"
  injection:
    type: NetworkPartition
    parameters:
      labelSelector: app=controller
      direction: ingress
    ttl: "120s"
  hypothesis:
    description: >-
      Describe the expected behavior after fault injection.
    recoveryTimeout: 120s
```

### kourier-gateway

```yaml
apiVersion: chaos.operatorchaos.io/v1alpha1
kind: ChaosExperiment
metadata:
  name: knative-kourier-custom
spec:
  target:
    operator: knative-serving
    component: kourier-gateway
  steadyState:
    checks:
      - type: conditionTrue
        apiVersion: apps/v1
        kind: Deployment
        name: 3scale-kourier-gateway
        namespace: knative-serving-ingress
        conditionType: Available
    timeout: "30s"
  injection:
    type: PodKill
    parameters:
      labelSelector: app=3scale-kourier-gateway
    count: 1
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

When designing custom experiments for Knative Serving:

- **Test scale-from-zero behavior**: Many experiments should validate that scale-from-zero continues working during and after faults.
- **Measure inference latency**: Knative Serving experiments should measure request latency and availability, not just component recovery.
- **Consider multi-component faults**: Knative Serving's distributed architecture means that simultaneous faults in activator + autoscaler or controller + webhook can have compounding effects.
- **Validate route programming**: For kourier-gateway and net-kourier-controller experiments, verify that new Knative Services can be created and routed after recovery.


<!-- custom-start: examples -->
<!-- custom-end: examples -->
