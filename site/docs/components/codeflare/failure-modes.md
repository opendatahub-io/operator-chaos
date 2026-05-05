# codeflare Failure Modes

## Coverage

| Injection Type | Danger | Experiment | Description |
|----------------|--------|------------|-------------|
| ConfigDrift | high | config-drift.yaml | When the codeflare operator configuration is corrupted, new cluster configuratio... |
| FinalizerBlock | low | finalizer-block.yaml | When a stuck finalizer prevents the codeflare-operator Deployment from being del... |
| NetworkPartition | medium | network-partition.yaml | When the codeflare-operator is network-partitioned from the API server, AppWrapp... |
| PodKill | low | pod-kill.yaml | When the codeflare-operator pod is killed, existing Ray clusters remain unaffect... |
| RBACRevoke | high | rbac-revoke.yaml | When the codeflare-operator ClusterRoleBinding subjects are revoked, the operato... |
| WebhookDisrupt | high | webhook-disrupt.yaml | When the AppWrapper validating webhook failurePolicy is weakened from Fail to Ig... |

## Experiment Details

### codeflare-config-drift

- **Type:** ConfigDrift
- **Danger Level:** high
- **Component:** codeflare-operator-manager

When the codeflare operator configuration is corrupted, new cluster configurations receive wrong parameters. Existing Ray clusters remain unaffected. The operator should detect the drift and reconcile the correct configuration.

<details>
<summary>Experiment YAML</summary>

```yaml
apiVersion: chaos.operatorchaos.io/v1alpha1
kind: ChaosExperiment
metadata:
  name: codeflare-config-drift
spec:
  tier: 2
  target:
    operator: codeflare
    component: codeflare-operator-manager
    resource: ConfigMap/codeflare-operator-config
  steadyState:
    checks:
      - type: resourceExists
        apiVersion: v1
        kind: ConfigMap
        name: codeflare-operator-config
        namespace: opendatahub
    timeout: "30s"
  injection:
    type: ConfigDrift
    dangerLevel: high
    parameters:
      name: codeflare-operator-config
      key: config.yaml
      value: '{"ray":{"defaultClusterSize":"-1","workerImage":"invalid:broken"}}'
      resourceType: ConfigMap
    ttl: "300s"
  hypothesis:
    description: >-
      When the codeflare operator configuration is corrupted, new cluster
      configurations receive wrong parameters. Existing Ray clusters remain
      unaffected. The operator should detect the drift and reconcile the
      correct configuration.
    recoveryTimeout: 180s
  blastRadius:
    maxPodsAffected: 1
    allowedNamespaces:
      - opendatahub
    allowDangerous: true
```

</details>

### codeflare-finalizer-block

- **Type:** FinalizerBlock
- **Danger Level:** low
- **Component:** codeflare-operator-manager

When a stuck finalizer prevents the codeflare-operator Deployment from being deleted, the operator lifecycle should handle the Terminating state gracefully. The chaos framework removes the finalizer via TTL-based cleanup after 300s.

<details>
<summary>Experiment YAML</summary>

```yaml
apiVersion: chaos.operatorchaos.io/v1alpha1
kind: ChaosExperiment
metadata:
  name: codeflare-finalizer-block
spec:
  tier: 3
  target:
    operator: codeflare
    component: codeflare-operator-manager
    resource: Deployment/codeflare-operator-manager
  steadyState:
    checks:
      - type: conditionTrue
        apiVersion: apps/v1
        kind: Deployment
        name: codeflare-operator-manager
        namespace: opendatahub
        conditionType: Available
    timeout: "30s"
  injection:
    type: FinalizerBlock
    parameters:
      apiVersion: apps/v1
      kind: Deployment
      name: codeflare-operator-manager
      finalizer: chaos.operatorchaos.io/block-test
    ttl: "300s"
  hypothesis:
    description: >-
      When a stuck finalizer prevents the codeflare-operator Deployment
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

### codeflare-network-partition

- **Type:** NetworkPartition
- **Danger Level:** medium
- **Component:** codeflare-operator-manager

When the codeflare-operator is network-partitioned from the API server, AppWrapper reconciliation stops. Existing Ray clusters continue running. Once the partition is removed, reconciliation resumes without manual intervention.

<details>
<summary>Experiment YAML</summary>

```yaml
apiVersion: chaos.operatorchaos.io/v1alpha1
kind: ChaosExperiment
metadata:
  name: codeflare-network-partition
spec:
  tier: 2
  target:
    operator: codeflare
    component: codeflare-operator-manager
    resource: Deployment/codeflare-operator-manager
  steadyState:
    checks:
      - type: conditionTrue
        apiVersion: apps/v1
        kind: Deployment
        name: codeflare-operator-manager
        namespace: opendatahub
        conditionType: Available
    timeout: "30s"
  injection:
    type: NetworkPartition
    parameters:
      labelSelector: control-plane=manager,app.kubernetes.io/name=codeflare-operator
    ttl: "300s"
  hypothesis:
    description: >-
      When the codeflare-operator is network-partitioned from the API
      server, AppWrapper reconciliation stops. Existing Ray clusters
      continue running. Once the partition is removed, reconciliation
      resumes without manual intervention.
    recoveryTimeout: 180s
  blastRadius:
    maxPodsAffected: 1
    allowedNamespaces:
      - opendatahub
```

</details>

### codeflare-pod-kill

- **Type:** PodKill
- **Danger Level:** low
- **Component:** codeflare-operator-manager

When the codeflare-operator pod is killed, existing Ray clusters remain unaffected. New AppWrapper submissions queue until the operator recovers within the recovery timeout.

<details>
<summary>Experiment YAML</summary>

```yaml
apiVersion: chaos.operatorchaos.io/v1alpha1
kind: ChaosExperiment
metadata:
  name: codeflare-pod-kill
spec:
  tier: 1
  target:
    operator: codeflare
    component: codeflare-operator-manager
    resource: Deployment/codeflare-operator-manager
  steadyState:
    checks:
      - type: conditionTrue
        apiVersion: apps/v1
        kind: Deployment
        name: codeflare-operator-manager
        namespace: opendatahub
        conditionType: Available
    timeout: "30s"
  injection:
    type: PodKill
    parameters:
      labelSelector: control-plane=manager,app.kubernetes.io/name=codeflare-operator
    count: 1
    ttl: "300s"
  hypothesis:
    description: >-
      When the codeflare-operator pod is killed, existing Ray clusters
      remain unaffected. New AppWrapper submissions queue until the
      operator recovers within the recovery timeout.
    recoveryTimeout: 120s
  blastRadius:
    maxPodsAffected: 1
    allowedNamespaces:
      - opendatahub
```

</details>

### codeflare-rbac-revoke

- **Type:** RBACRevoke
- **Danger Level:** high
- **Component:** codeflare-operator-manager

When the codeflare-operator ClusterRoleBinding subjects are revoked, the operator can no longer manage AppWrapper resources. API calls return 403 errors. Once permissions are restored, normal operation resumes without restart.

<details>
<summary>Experiment YAML</summary>

```yaml
apiVersion: chaos.operatorchaos.io/v1alpha1
kind: ChaosExperiment
metadata:
  name: codeflare-rbac-revoke
spec:
  tier: 4
  target:
    operator: codeflare
    component: codeflare-operator-manager
    resource: ClusterRoleBinding/codeflare-operator-manager-rolebinding
  steadyState:
    checks:
      - type: conditionTrue
        apiVersion: apps/v1
        kind: Deployment
        name: codeflare-operator-manager
        namespace: opendatahub
        conditionType: Available
    timeout: "30s"
  injection:
    type: RBACRevoke
    dangerLevel: high
    parameters:
      bindingName: codeflare-operator-manager-rolebinding
      bindingType: ClusterRoleBinding
    ttl: "60s"
  hypothesis:
    description: >-
      When the codeflare-operator ClusterRoleBinding subjects are revoked,
      the operator can no longer manage AppWrapper resources. API calls
      return 403 errors. Once permissions are restored, normal operation
      resumes without restart.
    recoveryTimeout: 120s
  blastRadius:
    maxPodsAffected: 1
    allowDangerous: true
```

</details>


### codeflare-webhook-disrupt

- **Type:** WebhookDisrupt
- **Danger Level:** high
- **Component:** codeflare-operator-manager

When the AppWrapper validating webhook failurePolicy is weakened from Fail to Ignore, invalid AppWrapper resources may be admitted to the cluster. The chaos framework restores the original failurePolicy via TTL-based cleanup after 60s.

<details>
<summary>Experiment YAML</summary>

```yaml
apiVersion: chaos.operatorchaos.io/v1alpha1
kind: ChaosExperiment
metadata:
  name: codeflare-webhook-disrupt
spec:
  tier: 4
  target:
    operator: codeflare
    component: codeflare-operator-manager
    resource: ValidatingWebhookConfiguration/vappwrapper.codeflare.dev
  steadyState:
    checks:
      - type: conditionTrue
        apiVersion: apps/v1
        kind: Deployment
        name: codeflare-operator-manager
        namespace: opendatahub
        conditionType: Available
    timeout: "30s"
  injection:
    type: WebhookDisrupt
    dangerLevel: high
    parameters:
      webhookName: vappwrapper.codeflare.dev
      webhookType: validating
      action: setFailurePolicy
      value: Ignore
    ttl: "60s"
  hypothesis:
    description: >-
      When the AppWrapper validating webhook failurePolicy is weakened from
      Fail to Ignore, invalid AppWrapper resources may be admitted to the
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
