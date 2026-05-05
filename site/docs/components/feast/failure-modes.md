# feast Failure Modes

## Coverage

| Injection Type | Danger | Experiment | Description |
|----------------|--------|------------|-------------|
| FinalizerBlock | low | finalizer-block.yaml | When a stuck finalizer prevents the feast-operator Deployment from being deleted... |
| NetworkPartition | medium | network-partition.yaml | When the feast-operator is network-partitioned from the API server, FeatureStore... |
| PodKill | low | pod-kill.yaml | When the feast-operator pod is killed, existing FeatureStore instances continue ... |
| RBACRevoke | high | rbac-revoke.yaml | When the feast-operator ClusterRoleBinding subjects are revoked, the operator lo... |
| WebhookDisrupt | high | webhook-disrupt.yaml | When the FeatureStore validating webhook failurePolicy is weakened from Fail to ... |

## Experiment Details

### feast-finalizer-block

- **Type:** FinalizerBlock
- **Danger Level:** low
- **Component:** feast-operator-controller-manager

When a stuck finalizer prevents the feast-operator Deployment from being deleted, the operator lifecycle should handle the Terminating state gracefully. The chaos framework removes the finalizer via TTL-based cleanup after 300s.

<details>
<summary>Experiment YAML</summary>

```yaml
apiVersion: chaos.operatorchaos.io/v1alpha1
kind: ChaosExperiment
metadata:
  name: feast-finalizer-block
spec:
  tier: 3
  target:
    operator: feast
    component: feast-operator-controller-manager
    resource: Deployment/feast-operator-controller-manager
  steadyState:
    checks:
      - type: conditionTrue
        apiVersion: apps/v1
        kind: Deployment
        name: feast-operator-controller-manager
        namespace: opendatahub
        conditionType: Available
    timeout: "30s"
  injection:
    type: FinalizerBlock
    parameters:
      apiVersion: apps/v1
      kind: Deployment
      name: feast-operator-controller-manager
      finalizer: chaos.operatorchaos.io/block-test
    ttl: "300s"
  hypothesis:
    description: >-
      When a stuck finalizer prevents the feast-operator Deployment
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

### feast-network-partition

- **Type:** NetworkPartition
- **Danger Level:** medium
- **Component:** feast-operator-controller-manager

When the feast-operator is network-partitioned from the API server, FeatureStore reconciliation stops. Existing feature servers remain available and continue serving features. Once the partition is removed, reconciliation resumes without manual intervention.

<details>
<summary>Experiment YAML</summary>

```yaml
apiVersion: chaos.operatorchaos.io/v1alpha1
kind: ChaosExperiment
metadata:
  name: feast-network-partition
spec:
  tier: 2
  target:
    operator: feast
    component: feast-operator-controller-manager
    resource: Deployment/feast-operator-controller-manager
  steadyState:
    checks:
      - type: conditionTrue
        apiVersion: apps/v1
        kind: Deployment
        name: feast-operator-controller-manager
        namespace: opendatahub
        conditionType: Available
    timeout: "30s"
  injection:
    type: NetworkPartition
    parameters:
      labelSelector: control-plane=controller-manager,app.kubernetes.io/name=feast-operator
    ttl: "300s"
  hypothesis:
    description: >-
      When the feast-operator is network-partitioned from the API server,
      FeatureStore reconciliation stops. Existing feature servers remain
      available and continue serving features. Once the partition is
      removed, reconciliation resumes without manual intervention.
    recoveryTimeout: 180s
  blastRadius:
    maxPodsAffected: 1
    allowedNamespaces:
      - opendatahub
```

</details>

### feast-pod-kill

- **Type:** PodKill
- **Danger Level:** low
- **Component:** feast-operator-controller-manager

When the feast-operator pod is killed, existing FeatureStore instances continue serving features. New FeatureStore deployments queue until the operator recovers and processes the backlog within the recovery timeout.

<details>
<summary>Experiment YAML</summary>

```yaml
apiVersion: chaos.operatorchaos.io/v1alpha1
kind: ChaosExperiment
metadata:
  name: feast-pod-kill
spec:
  tier: 1
  target:
    operator: feast
    component: feast-operator-controller-manager
    resource: Deployment/feast-operator-controller-manager
  steadyState:
    checks:
      - type: conditionTrue
        apiVersion: apps/v1
        kind: Deployment
        name: feast-operator-controller-manager
        namespace: opendatahub
        conditionType: Available
    timeout: "30s"
  injection:
    type: PodKill
    parameters:
      labelSelector: control-plane=controller-manager,app.kubernetes.io/name=feast-operator
    count: 1
    ttl: "300s"
  hypothesis:
    description: >-
      When the feast-operator pod is killed, existing FeatureStore instances
      continue serving features. New FeatureStore deployments queue until
      the operator recovers and processes the backlog within the recovery
      timeout.
    recoveryTimeout: 120s
  blastRadius:
    maxPodsAffected: 1
    allowedNamespaces:
      - opendatahub
```

</details>

### feast-rbac-revoke

- **Type:** RBACRevoke
- **Danger Level:** high
- **Component:** feast-operator-controller-manager

When the feast-operator ClusterRoleBinding subjects are revoked, the operator loses cluster access and can no longer manage FeatureStore resources. API calls return 403 errors. Once permissions are restored, normal operation resumes without restart.

<details>
<summary>Experiment YAML</summary>

```yaml
apiVersion: chaos.operatorchaos.io/v1alpha1
kind: ChaosExperiment
metadata:
  name: feast-rbac-revoke
spec:
  tier: 4
  target:
    operator: feast
    component: feast-operator-controller-manager
    resource: ClusterRoleBinding/feast-operator-manager-rolebinding
  steadyState:
    checks:
      - type: conditionTrue
        apiVersion: apps/v1
        kind: Deployment
        name: feast-operator-controller-manager
        namespace: opendatahub
        conditionType: Available
    timeout: "30s"
  injection:
    type: RBACRevoke
    dangerLevel: high
    parameters:
      bindingName: feast-operator-manager-rolebinding
      bindingType: ClusterRoleBinding
    ttl: "60s"
  hypothesis:
    description: >-
      When the feast-operator ClusterRoleBinding subjects are revoked, the
      operator loses cluster access and can no longer manage FeatureStore
      resources. API calls return 403 errors. Once permissions are restored,
      normal operation resumes without restart.
    recoveryTimeout: 120s
  blastRadius:
    maxPodsAffected: 1
    allowDangerous: true
```

</details>


### feast-webhook-disrupt

- **Type:** WebhookDisrupt
- **Danger Level:** high
- **Component:** feast-operator-controller-manager

When the FeatureStore validating webhook failurePolicy is weakened from Fail to Ignore, invalid FeatureStore resources may be admitted to the cluster. The chaos framework restores the original failurePolicy via TTL-based cleanup after 60s.

<details>
<summary>Experiment YAML</summary>

```yaml
apiVersion: chaos.operatorchaos.io/v1alpha1
kind: ChaosExperiment
metadata:
  name: feast-webhook-disrupt
spec:
  tier: 4
  target:
    operator: feast
    component: feast-operator-controller-manager
    resource: ValidatingWebhookConfiguration/vfeaturestore.feast.dev
  steadyState:
    checks:
      - type: conditionTrue
        apiVersion: apps/v1
        kind: Deployment
        name: feast-operator-controller-manager
        namespace: opendatahub
        conditionType: Available
    timeout: "30s"
  injection:
    type: WebhookDisrupt
    dangerLevel: high
    parameters:
      webhookName: vfeaturestore.feast.dev
      webhookType: validating
      action: setFailurePolicy
      value: Ignore
    ttl: "60s"
  hypothesis:
    description: >-
      When the FeatureStore validating webhook failurePolicy is weakened from
      Fail to Ignore, invalid FeatureStore resources may be admitted to the
      cluster. The chaos framework restores the original failurePolicy via
      TTL-based cleanup after 60s.
    recoveryTimeout: 120s
  blastRadius:
    maxPodsAffected: 1
    allowDangerous: true
```

</details>


<!-- custom-start: known-issues -->
<!-- custom-end: known-issues -->
