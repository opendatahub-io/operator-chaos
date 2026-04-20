# kserve Failure Modes

## Coverage

| Injection Type | Danger | Experiment | Description |
|----------------|--------|------------|-------------|
| PodKill | low | dependency-odh-model-controller-kill.yaml | Killing odh-model-controller (which kserve depends on for model serving routing)... |
| ConfigDrift | high | isvc-config-corruption.yaml | When the deploy key in the inferenceservice-config ConfigMap is overwritten with... |
| WebhookDisrupt | high | isvc-validator-disrupt.yaml | When the ValidatingWebhookConfiguration for InferenceService has its failurePoli... |
| NetworkPartition | medium | llm-controller-isolation.yaml | When the llmisvc-controller-manager is network-partitioned from the API server, ... |
| PodKill | low | main-controller-kill.yaml | When the kserve-controller-manager pod is killed, the Deployment controller recr... |
| OwnerRefOrphan | medium | ownerref-orphan.yaml | Removing ownerReferences from the kserve-controller-manager Deployment should tr... |

## Experiment Details

### kserve-dependency-odh-model-controller-kill

- **Type:** PodKill
- **Danger Level:** low
- **Component:** kserve-controller

Killing odh-model-controller (which kserve depends on for model serving routing) should not crash kserve. Kserve should continue operating in degraded mode and recover when the dependency is restored.

<details>
<summary>Experiment YAML</summary>

```yaml
apiVersion: chaos.operatorchaos.io/v1alpha1
kind: ChaosExperiment
metadata:
  name: kserve-dependency-odh-model-controller-kill
spec:
  target:
    operator: kserve
    component: kserve-controller
  steadyState:
    checks:
      - type: conditionTrue
        apiVersion: apps/v1
        kind: Deployment
        name: kserve-controller-manager
        namespace: opendatahub
        conditionType: Available
    timeout: "30s"
  injection:
    type: PodKill
    parameters:
      labelSelector: "app=odh-model-controller"
    ttl: "300s"
  hypothesis:
    description: >-
      Killing odh-model-controller (which kserve depends on for model serving
      routing) should not crash kserve. Kserve should continue operating in
      degraded mode and recover when the dependency is restored.
    recoveryTimeout: 120s
  blastRadius:
    maxPodsAffected: 1
    allowedNamespaces:
      - opendatahub
```

</details>

### kserve-isvc-config-corruption

- **Type:** ConfigDrift
- **Danger Level:** high
- **Component:** kserve-controller-manager

When the deploy key in the inferenceservice-config ConfigMap is overwritten with an empty JSON object, kserve should detect the partial config corruption and recover within 120s. Existing InferenceService resources should continue serving, and the controller should either fall back to built-in defaults or surface clear error conditions rather than silently failing.

<details>
<summary>Experiment YAML</summary>

```yaml
apiVersion: chaos.operatorchaos.io/v1alpha1
kind: ChaosExperiment
metadata:
  name: kserve-isvc-config-corruption
  namespace: kserve
spec:
  target:
    operator: kserve
    component: kserve-controller-manager
    resource: ConfigMap/inferenceservice-config
  steadyState:
    checks:
      - type: resourceExists
        apiVersion: v1
        kind: ConfigMap
        name: inferenceservice-config
        namespace: kserve
    timeout: "30s"
  injection:
    type: ConfigDrift
    dangerLevel: high
    parameters:
      name: inferenceservice-config
      key: deploy
      value: "{}"
      resourceType: ConfigMap
    ttl: "300s"
  hypothesis:
    description: >-
      When the deploy key in the inferenceservice-config ConfigMap is
      overwritten with an empty JSON object, kserve should detect the
      partial config corruption and recover within 120s. Existing
      InferenceService resources should continue serving, and the
      controller should either fall back to built-in defaults or
      surface clear error conditions rather than silently failing.
    recoveryTimeout: 120s
  blastRadius:
    maxPodsAffected: 1
    allowedNamespaces:
      - kserve
    allowDangerous: true
```

</details>

### kserve-isvc-validator-disrupt

- **Type:** WebhookDisrupt
- **Danger Level:** high
- **Component:** kserve-controller-manager

When the ValidatingWebhookConfiguration for InferenceService has its failurePolicy weakened from Fail to Ignore, invalid InferenceService resources can bypass validation. This tests the blast radius of a weakened admission policy. Recovery is provided by the chaos framework's TTL-based cleanup after 60s, since kserve does not self-heal webhook configuration drift.

<details>
<summary>Experiment YAML</summary>

```yaml
apiVersion: chaos.operatorchaos.io/v1alpha1
kind: ChaosExperiment
metadata:
  name: kserve-isvc-validator-disrupt
  namespace: kserve
spec:
  target:
    operator: kserve
    component: kserve-controller-manager
    resource: ValidatingWebhookConfiguration/inferenceservice.serving.kserve.io
  steadyState:
    checks:
      - type: conditionTrue
        apiVersion: apps/v1
        kind: Deployment
        name: kserve-controller-manager
        namespace: kserve
        conditionType: Available
    timeout: "30s"
  injection:
    type: WebhookDisrupt
    dangerLevel: high
    parameters:
      webhookName: "inferenceservice.serving.kserve.io"
      action: "setFailurePolicy"
      value: "Ignore"
    ttl: "60s"
  hypothesis:
    description: >-
      When the ValidatingWebhookConfiguration for InferenceService has
      its failurePolicy weakened from Fail to Ignore, invalid
      InferenceService resources can bypass validation. This tests the
      blast radius of a weakened admission policy. Recovery is provided
      by the chaos framework's TTL-based cleanup after 60s, since kserve
      does not self-heal webhook configuration drift.
    recoveryTimeout: 120s
  blastRadius:
    maxPodsAffected: 1
    allowDangerous: true
```

</details>

### kserve-llm-controller-isolation

- **Type:** NetworkPartition
- **Danger Level:** medium
- **Component:** llmisvc-controller-manager

When the llmisvc-controller-manager is network-partitioned from the API server, it should lose its leader lease and stop reconciling LLM resources. The main kserve-controller-manager must remain unaffected. Once the partition is lifted after 60s, the LLM controller should re-acquire its lease and resume normal operation within 120s.

<details>
<summary>Experiment YAML</summary>

```yaml
apiVersion: chaos.operatorchaos.io/v1alpha1
kind: ChaosExperiment
metadata:
  name: kserve-llm-controller-isolation
  namespace: kserve
spec:
  target:
    operator: kserve
    component: llmisvc-controller-manager
    resource: Deployment/llmisvc-controller-manager
  steadyState:
    checks:
      - type: conditionTrue
        apiVersion: apps/v1
        kind: Deployment
        name: llmisvc-controller-manager
        namespace: kserve
        conditionType: Available
    timeout: "30s"
  injection:
    type: NetworkPartition
    parameters:
      labelSelector: control-plane=llmisvc-controller-manager
    ttl: "60s"
  hypothesis:
    description: >-
      When the llmisvc-controller-manager is network-partitioned from the
      API server, it should lose its leader lease and stop reconciling LLM
      resources. The main kserve-controller-manager must remain unaffected.
      Once the partition is lifted after 60s, the LLM controller should
      re-acquire its lease and resume normal operation within 120s.
    recoveryTimeout: 120s
  blastRadius:
    maxPodsAffected: 1
    allowedNamespaces:
      - kserve
```

</details>

### kserve-main-controller-kill

- **Type:** PodKill
- **Danger Level:** low
- **Component:** kserve-controller-manager

When the kserve-controller-manager pod is killed, the Deployment controller recreates it and leader election completes recovery. InferenceService reconciliation should resume within 60s without data loss or duplicate work.

<details>
<summary>Experiment YAML</summary>

```yaml
apiVersion: chaos.operatorchaos.io/v1alpha1
kind: ChaosExperiment
metadata:
  name: kserve-main-controller-kill
  namespace: kserve
spec:
  target:
    operator: kserve
    component: kserve-controller-manager
    resource: Deployment/kserve-controller-manager
  steadyState:
    checks:
      - type: conditionTrue
        apiVersion: apps/v1
        kind: Deployment
        name: kserve-controller-manager
        namespace: kserve
        conditionType: Available
    timeout: "30s"
  injection:
    type: PodKill
    parameters:
      labelSelector: control-plane=kserve-controller-manager
    count: 1
    ttl: "300s"
  hypothesis:
    description: >-
      When the kserve-controller-manager pod is killed, the Deployment
      controller recreates it and leader election completes recovery.
      InferenceService reconciliation should resume within 60s without
      data loss or duplicate work.
    recoveryTimeout: 60s
  blastRadius:
    maxPodsAffected: 1
    allowedNamespaces:
      - kserve
```

</details>

### kserve-ownerref-orphan

- **Type:** OwnerRefOrphan
- **Danger Level:** medium
- **Component:** kserve-controller

Removing ownerReferences from the kserve-controller-manager Deployment should trigger the operator to re-adopt it within the recovery timeout.

<details>
<summary>Experiment YAML</summary>

```yaml
apiVersion: chaos.operatorchaos.io/v1alpha1
kind: ChaosExperiment
metadata:
  name: kserve-ownerref-orphan
spec:
  target:
    operator: kserve
    component: kserve-controller
  steadyState:
    checks:
      - type: conditionTrue
        apiVersion: apps/v1
        kind: Deployment
        name: kserve-controller-manager
        namespace: opendatahub
        conditionType: Available
    timeout: "30s"
  injection:
    type: OwnerRefOrphan
    parameters:
      apiVersion: "apps/v1"
      kind: "Deployment"
      name: "kserve-controller-manager"
    ttl: "120s"
  hypothesis:
    description: >-
      Removing ownerReferences from the kserve-controller-manager Deployment
      should trigger the operator to re-adopt it within the recovery timeout.
    recoveryTimeout: 60s
  blastRadius:
    maxPodsAffected: 1
    allowedNamespaces:
      - opendatahub
```

</details>


<!-- custom-start: known-issues -->
<!-- custom-end: known-issues -->
