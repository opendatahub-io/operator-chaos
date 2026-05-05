# trustyai Failure Modes

## Coverage

| Injection Type | Danger | Experiment | Description |
|----------------|--------|------------|-------------|
| FinalizerBlock | low | finalizer-block.yaml | When a stuck finalizer prevents the trustyai-service-operator Deployment from be... |
| NetworkPartition | medium | network-partition.yaml | When the trustyai-service-operator is network-partitioned from the API server, e... |
| PodKill | low | pod-kill.yaml | When the trustyai-service-operator pod is killed, existing TrustyAI services kee... |
| RBACRevoke | high | rbac-revoke.yaml | When the trustyai-service-operator ClusterRoleBinding subjects are revoked, the ... |
| WebhookDisrupt | high | webhook-disrupt.yaml | When the TrustyAIService validating webhook failurePolicy is weakened from Fail ... |

## Experiment Details

### trustyai-finalizer-block

- **Type:** FinalizerBlock
- **Danger Level:** low
- **Component:** trustyai-service-operator-controller-manager

When a stuck finalizer prevents the trustyai-service-operator Deployment from being deleted, the operator lifecycle should handle the Terminating state gracefully. The chaos framework removes the finalizer via TTL-based cleanup after 300s.

<details>
<summary>Experiment YAML</summary>

```yaml
apiVersion: chaos.operatorchaos.io/v1alpha1
kind: ChaosExperiment
metadata:
  name: trustyai-finalizer-block
spec:
  tier: 3
  target:
    operator: trustyai
    component: trustyai-service-operator-controller-manager
    resource: Deployment/trustyai-service-operator-controller-manager
  steadyState:
    checks:
      - type: conditionTrue
        apiVersion: apps/v1
        kind: Deployment
        name: trustyai-service-operator-controller-manager
        namespace: opendatahub
        conditionType: Available
    timeout: "30s"
  injection:
    type: FinalizerBlock
    parameters:
      apiVersion: apps/v1
      kind: Deployment
      name: trustyai-service-operator-controller-manager
      finalizer: chaos.operatorchaos.io/block-test
    ttl: "300s"
  hypothesis:
    description: >-
      When a stuck finalizer prevents the trustyai-service-operator Deployment
      from being deleted, the operator lifecycle should handle the Terminating
      state gracefully. The chaos framework removes the finalizer via
      TTL-based cleanup after 300s.
    recoveryTimeout: 180s
  blastRadius:
    maxPodsAffected: 1
    allowedNamespaces:
      - opendatahub
```

</details>

### trustyai-network-partition

- **Type:** NetworkPartition
- **Danger Level:** medium
- **Component:** trustyai-service-operator-controller-manager

When the trustyai-service-operator is network-partitioned from the API server, explainability reconciliation stops. Existing TrustyAI services remain available. Once the partition is removed, reconciliation resumes without manual intervention.

<details>
<summary>Experiment YAML</summary>

```yaml
apiVersion: chaos.operatorchaos.io/v1alpha1
kind: ChaosExperiment
metadata:
  name: trustyai-network-partition
spec:
  tier: 2
  target:
    operator: trustyai
    component: trustyai-service-operator-controller-manager
    resource: Deployment/trustyai-service-operator-controller-manager
  steadyState:
    checks:
      - type: conditionTrue
        apiVersion: apps/v1
        kind: Deployment
        name: trustyai-service-operator-controller-manager
        namespace: opendatahub
        conditionType: Available
    timeout: "30s"
  injection:
    type: NetworkPartition
    parameters:
      labelSelector: control-plane=controller-manager,app.kubernetes.io/name=trustyai-service-operator
    ttl: "300s"
  hypothesis:
    description: >-
      When the trustyai-service-operator is network-partitioned from the
      API server, explainability reconciliation stops. Existing TrustyAI
      services remain available. Once the partition is removed,
      reconciliation resumes without manual intervention.
    recoveryTimeout: 180s
  blastRadius:
    maxPodsAffected: 1
    allowedNamespaces:
      - opendatahub
```

</details>

### trustyai-pod-kill

- **Type:** PodKill
- **Danger Level:** low
- **Component:** trustyai-service-operator-controller-manager

When the trustyai-service-operator pod is killed, existing TrustyAI services keep running and continue providing explainability. New TrustyAIService deployments queue until the operator recovers within the recovery timeout.

<details>
<summary>Experiment YAML</summary>

```yaml
apiVersion: chaos.operatorchaos.io/v1alpha1
kind: ChaosExperiment
metadata:
  name: trustyai-pod-kill
spec:
  tier: 1
  target:
    operator: trustyai
    component: trustyai-service-operator-controller-manager
    resource: Deployment/trustyai-service-operator-controller-manager
  steadyState:
    checks:
      - type: conditionTrue
        apiVersion: apps/v1
        kind: Deployment
        name: trustyai-service-operator-controller-manager
        namespace: opendatahub
        conditionType: Available
    timeout: "30s"
  injection:
    type: PodKill
    parameters:
      labelSelector: control-plane=controller-manager,app.kubernetes.io/name=trustyai-service-operator
    count: 1
    ttl: "300s"
  hypothesis:
    description: >-
      When the trustyai-service-operator pod is killed, existing TrustyAI
      services keep running and continue providing explainability. New
      TrustyAIService deployments queue until the operator recovers within
      the recovery timeout.
    recoveryTimeout: 120s
  blastRadius:
    maxPodsAffected: 1
    allowedNamespaces:
      - opendatahub
```

</details>

### trustyai-rbac-revoke

- **Type:** RBACRevoke
- **Danger Level:** high
- **Component:** trustyai-service-operator-controller-manager

When the trustyai-service-operator ClusterRoleBinding subjects are revoked, the operator loses cluster access and can no longer manage TrustyAIService resources. API calls return 403 errors. Once permissions are restored, normal operation resumes without restart.

<details>
<summary>Experiment YAML</summary>

```yaml
apiVersion: chaos.operatorchaos.io/v1alpha1
kind: ChaosExperiment
metadata:
  name: trustyai-rbac-revoke
spec:
  tier: 4
  target:
    operator: trustyai
    component: trustyai-service-operator-controller-manager
    resource: ClusterRoleBinding/trustyai-service-operator-manager-rolebinding
  steadyState:
    checks:
      - type: conditionTrue
        apiVersion: apps/v1
        kind: Deployment
        name: trustyai-service-operator-controller-manager
        namespace: opendatahub
        conditionType: Available
    timeout: "30s"
  injection:
    type: RBACRevoke
    dangerLevel: high
    parameters:
      bindingName: trustyai-service-operator-manager-rolebinding
      bindingType: ClusterRoleBinding
    ttl: "60s"
  hypothesis:
    description: >-
      When the trustyai-service-operator ClusterRoleBinding subjects are
      revoked, the operator loses cluster access and can no longer manage
      TrustyAIService resources. API calls return 403 errors. Once
      permissions are restored, normal operation resumes without restart.
    recoveryTimeout: 120s
  blastRadius:
    maxPodsAffected: 1
    allowDangerous: true
```

</details>


### trustyai-webhook-disrupt

- **Type:** WebhookDisrupt
- **Danger Level:** high
- **Component:** trustyai-service-operator-controller-manager

When the TrustyAIService validating webhook failurePolicy is weakened from Fail to Ignore, invalid TrustyAIService resources may be admitted to the cluster. The chaos framework restores the original failurePolicy via TTL-based cleanup after 60s.

<details>
<summary>Experiment YAML</summary>

```yaml
apiVersion: chaos.operatorchaos.io/v1alpha1
kind: ChaosExperiment
metadata:
  name: trustyai-webhook-disrupt
spec:
  tier: 4
  target:
    operator: trustyai
    component: trustyai-service-operator-controller-manager
    resource: ValidatingWebhookConfiguration/vtrustyaiservice.trustyai.opendatahub.io
  steadyState:
    checks:
      - type: conditionTrue
        apiVersion: apps/v1
        kind: Deployment
        name: trustyai-service-operator-controller-manager
        namespace: opendatahub
        conditionType: Available
    timeout: "30s"
  injection:
    type: WebhookDisrupt
    dangerLevel: high
    parameters:
      webhookName: vtrustyaiservice.trustyai.opendatahub.io
      webhookType: validating
      action: setFailurePolicy
      value: Ignore
    ttl: "60s"
  hypothesis:
    description: >-
      When the TrustyAIService validating webhook failurePolicy is weakened
      from Fail to Ignore, invalid TrustyAIService resources may be admitted
      to the cluster. The chaos framework restores the original failurePolicy
      via TTL-based cleanup after 60s.
    recoveryTimeout: 120s
  blastRadius:
    maxPodsAffected: 1
    allowDangerous: true
```

</details>


<!-- custom-start: known-issues -->
<!-- custom-end: known-issues -->
