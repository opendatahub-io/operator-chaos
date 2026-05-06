# kueue Custom Experiments

This page provides templates for writing custom chaos experiments targeting Kueue.

## Red Hat Build of Kueue Operator

For RHOAI 3.x, target the OLM-managed kueue operator in the openshift-kueue-operator namespace.

```yaml
apiVersion: chaos.operatorchaos.io/v1alpha1
kind: ChaosExperiment
metadata:
  name: kueue-operator-custom
spec:
  target:
    operator: rh-kueue
    component: kueue-operator
  steadyState:
    checks:
      - type: conditionTrue
        apiVersion: apps/v1
        kind: Deployment
        name: openshift-kueue-operator
        namespace: openshift-kueue-operator
        conditionType: Available
    timeout: "60s"
  injection:
    type: PodKill  # Change to desired injection type
    parameters:
      labelSelector: name=openshift-kueue-operator
    ttl: "300s"
  hypothesis:
    description: >-
      Describe the expected behavior after fault injection.
    recoveryTimeout: 120s
```

## Kueue Operand (Controller Manager)

Target the Kueue operand (controller-manager) that handles workload admission.

```yaml
apiVersion: chaos.operatorchaos.io/v1alpha1
kind: ChaosExperiment
metadata:
  name: kueue-operand-custom
spec:
  target:
    operator: rh-kueue
    component: kueue-operand
  steadyState:
    checks:
      - type: conditionTrue
        apiVersion: apps/v1
        kind: Deployment
        name: kueue-controller-manager
        namespace: openshift-kueue-operator
        conditionType: Available
    timeout: "60s"
  injection:
    type: PodKill  # Change to desired injection type
    parameters:
      labelSelector: control-plane=controller-manager
    ttl: "300s"
  hypothesis:
    description: >-
      Describe the expected behavior after fault injection.
    recoveryTimeout: 120s
```

## Legacy DSC-managed Kueue (RHOAI 2.x / ODH)

For ODH or RHOAI 2.x, target the DSC-managed kueue in the opendatahub namespace.

```yaml
apiVersion: chaos.operatorchaos.io/v1alpha1
kind: ChaosExperiment
metadata:
  name: kueue-controller-manager-custom
spec:
  target:
    operator: kueue
    component: kueue-controller-manager
  steadyState:
    checks:
      - type: conditionTrue
        apiVersion: apps/v1
        kind: Deployment
        name: kueue-controller-manager
        namespace: opendatahub
        conditionType: Available
    timeout: "60s"
  injection:
    type: PodKill  # Change to desired injection type
    parameters:
      labelSelector: control-plane=controller-manager,app.kubernetes.io/name=kueue
    ttl: "300s"
  hypothesis:
    description: >-
      Describe the expected behavior after fault injection.
    recoveryTimeout: 120s
```


## Running Custom Experiments

1. Save your experiment YAML to a file
2. Run: `operator-chaos run --experiment <file>`
3. Check results: `operator-chaos results --latest`

<!-- custom-start: examples -->
<!-- custom-end: examples -->
