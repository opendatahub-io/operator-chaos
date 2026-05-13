# cert-manager Custom Experiments

This page provides templates and guidance for writing custom chaos experiments targeting cert-manager components.

## Component Overview

cert-manager has 3 components in the cert-manager namespace:

- **cert-manager-controller**: Main controller for Certificate/Issuer/CertificateRequest reconciliation (label: `app.kubernetes.io/name=cert-manager`)
- **cert-manager-webhook**: Validation and mutation webhook (label: `app.kubernetes.io/name=webhook`)
- **cert-manager-cainjector**: CA bundle injector for webhooks and API services (label: `app.kubernetes.io/name=cainjector`)

## Key Architectural Relationships

Understanding these relationships helps design meaningful experiments:

1. **controller → Issuer/ClusterIssuer**: The controller watches Issuer and ClusterIssuer resources to determine which CA should sign certificates. RBAC disruptions targeting the `cert-manager-controller-issuers` ClusterRoleBinding block issuance.

2. **webhook → certificate validation**: The webhook validates Certificate and Issuer resources before admission. Network partitions or TLS certificate corruption block all cert-manager resource creation.

3. **cainjector → webhook CA bundles**: The cainjector injects CA bundles into ValidatingWebhookConfiguration and MutatingWebhookConfiguration resources. This ensures the webhook can be called by the API server.

4. **controller → self-signed bootstrapping**: cert-manager uses self-signed certificates during initial bootstrap. Certificate corruption experiments test whether the controller can regenerate its own webhook certificates.

## Example Templates

### cert-manager-controller

```yaml
apiVersion: chaos.operatorchaos.io/v1alpha1
kind: ChaosExperiment
metadata:
  name: cert-manager-controller-custom
spec:
  target:
    operator: cert-manager
    component: cert-manager-controller
  steadyState:
    checks:
      - type: conditionTrue
        apiVersion: apps/v1
        kind: Deployment
        name: cert-manager
        namespace: cert-manager
        conditionType: Available
    timeout: "30s"
  injection:
    type: PodKill  # Change to desired injection type
    parameters:
      labelSelector: app.kubernetes.io/name=cert-manager
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
  name: cert-manager-webhook-custom
spec:
  target:
    operator: cert-manager
    component: webhook
  steadyState:
    checks:
      - type: conditionTrue
        apiVersion: apps/v1
        kind: Deployment
        name: cert-manager-webhook
        namespace: cert-manager
        conditionType: Available
    timeout: "30s"
  injection:
    type: NetworkPartition
    parameters:
      labelSelector: app.kubernetes.io/name=webhook
    ttl: "60s"
  hypothesis:
    description: >-
      Describe the expected behavior after fault injection.
    recoveryTimeout: 120s
```

### cainjector

```yaml
apiVersion: chaos.operatorchaos.io/v1alpha1
kind: ChaosExperiment
metadata:
  name: cert-manager-cainjector-custom
spec:
  target:
    operator: cert-manager
    component: cainjector
  steadyState:
    checks:
      - type: conditionTrue
        apiVersion: apps/v1
        kind: Deployment
        name: cert-manager-cainjector
        namespace: cert-manager
        conditionType: Available
    timeout: "30s"
  injection:
    type: PodKill
    parameters:
      labelSelector: app.kubernetes.io/name=cainjector
    ttl: "30s"
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

When designing custom experiments for cert-manager:

- **Test certificate renewal**: Many experiments should validate that existing certificates continue working during faults, and that renewal processes complete after recovery.
- **Single-replica deployment**: cert-manager typically runs with single replicas. Experiments should account for zero-downtime not being guaranteed during pod failures.
- **Webhook dependency**: Certificate and Issuer creation depends on the webhook. Experiments targeting the webhook should verify that cert-manager resource creation fails gracefully and resumes after recovery.
- **Bootstrap certificate regeneration**: The webhook's TLS certificate is managed by cert-manager itself. Experiments that corrupt the webhook certificate test the self-healing bootstrap process.


<!-- custom-start: examples -->
<!-- custom-end: examples -->
