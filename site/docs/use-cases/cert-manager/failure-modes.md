# cert-manager Failure Modes

## Coverage

| Injection Type | Danger | Experiment | Description |
|----------------|--------|------------|-------------|
| PodKill | low | controller/controller-pod-kill.yaml | Killing the cert-manager controller pod should trigger a Deployment rollout that... |
| NetworkPartition | medium | controller/controller-network-partition.yaml | Isolating the cert-manager controller from the API server should cause certificat... |
| LabelStomping | high | controller/label-stomping.yaml | When a label used for resource discovery is overwritten on the controller Deploy... |
| QuotaExhaustion | high | controller/quota-exhaustion.yaml | When a ResourceQuota with zero limits is applied to cert-manager, no new pods ca... |
| RBACRevoke | high | controller/rbac-revoke.yaml | Revoking the cert-manager controller's ClusterRoleBinding for issuers should cau... |
| PodKill | low | webhook/pod-kill.yaml | When a webhook pod is killed, the Deployment controller should restart it and th... |
| NetworkPartition | medium | webhook/network-partition.yaml | When webhook pods are network-isolated via a deny-all NetworkPolicy, the compone... |
| LabelStomping | high | webhook/label-stomping.yaml | When a label used for resource discovery is overwritten on the webhook Deploymen... |
| QuotaExhaustion | high | webhook/quota-exhaustion.yaml | When a ResourceQuota with zero limits is applied to cert-manager, no new pods ca... |
| ConfigDrift | high | webhook/webhook-cert-corrupt.yaml | When the webhook webhook TLS certificate is corrupted, the webhook server should... |
| PodKill | low | cainjector/pod-kill.yaml | When a cainjector pod is killed, the Deployment controller should restart it and... |
| NetworkPartition | medium | cainjector/network-partition.yaml | When cainjector pods are network-isolated via a deny-all NetworkPolicy, the comp... |
| LabelStomping | high | cainjector/label-stomping.yaml | When a label used for resource discovery is overwritten on the cainjector Deploy... |
| QuotaExhaustion | high | cainjector/quota-exhaustion.yaml | When a ResourceQuota with zero limits is applied to cert-manager, no new pods ca... |

## Experiment Details

### controller

#### cert-manager-controller-pod-kill

- **Type:** PodKill
- **Danger Level:** low
- **Component:** cert-manager-controller

Killing the cert-manager controller pod should trigger a Deployment rollout that recreates it. Existing certificates remain valid since they are stored as Secrets. Pending certificate requests will stall until the controller recovers.

<details>
<summary>Experiment YAML</summary>

```yaml
apiVersion: chaos.operatorchaos.io/v1alpha1
kind: ChaosExperiment
metadata:
  name: cert-manager-controller-pod-kill
spec:
  tier: 1
  target:
    operator: cert-manager
    component: cert-manager-controller
    resource: Deployment/cert-manager
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
    type: PodKill
    parameters:
      labelSelector: app.kubernetes.io/name=cert-manager
    count: 1
    ttl: "300s"
  hypothesis:
    description: >-
      Killing the cert-manager controller pod should trigger a Deployment
      rollout that recreates it. Existing certificates remain valid since
      they are stored as Secrets. Pending certificate requests will stall
      until the controller recovers.
    recoveryTimeout: 120s
  blastRadius:
    maxPodsAffected: 1
    allowedNamespaces:
      - cert-manager
```

</details>

---

#### cert-manager-controller-network-partition

- **Type:** NetworkPartition
- **Danger Level:** medium
- **Component:** cert-manager-controller

Isolating the cert-manager controller from the API server should cause certificate reconciliation to stall. Existing certificates and Secrets remain valid. After the partition is lifted, the controller should reconnect and process any backlogged certificate requests.

<details>
<summary>Experiment YAML</summary>

```yaml
apiVersion: chaos.operatorchaos.io/v1alpha1
kind: ChaosExperiment
metadata:
  name: cert-manager-controller-network-partition
spec:
  tier: 2
  target:
    operator: cert-manager
    component: cert-manager-controller
    resource: Deployment/cert-manager
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
    type: NetworkPartition
    parameters:
      labelSelector: app.kubernetes.io/name=cert-manager
      direction: ingress
    ttl: "120s"
  hypothesis:
    description: >-
      Isolating the cert-manager controller from the API server should cause
      certificate reconciliation to stall. Existing certificates and Secrets
      remain valid. After the partition is lifted, the controller should
      reconnect and process any backlogged certificate requests.
    recoveryTimeout: 120s
  blastRadius:
    maxPodsAffected: 1
    allowedNamespaces:
      - cert-manager
```

</details>

---

#### controller-label-stomping

- **Type:** LabelStomping
- **Danger Level:** high
- **Component:** cert-manager-controller

When a label used for resource discovery is overwritten on the controller Deployment, the operator should detect the label drift and restore the correct label value.

<details>
<summary>Experiment YAML</summary>

```yaml
apiVersion: chaos.operatorchaos.io/v1alpha1
kind: ChaosExperiment
metadata:
  name: controller-label-stomping
spec:
  tier: 3
  target:
    operator: cert-manager
    component: cert-manager-controller
    resource: Deployment/cert-manager
  steadyState:
    checks:
      - type: resourceExists
        apiVersion: apps/v1
        kind: Deployment
        name: cert-manager
        namespace: cert-manager
    timeout: "30s"
  injection:
    type: LabelStomping
    dangerLevel: high
    parameters:
      apiVersion: apps/v1
      kind: Deployment
      name: cert-manager
      labelKey: app.kubernetes.io/name
      action: overwrite
    ttl: "300s"
  hypothesis:
    description: >-
      When a label used for resource discovery is overwritten on the
      controller Deployment, the operator should detect the label drift
      and restore the correct label value.
    recoveryTimeout: 120s
  blastRadius:
    maxPodsAffected: 1
    allowDangerous: true
    allowedNamespaces:
      - cert-manager
```

</details>

---

#### controller-quota-exhaustion

- **Type:** QuotaExhaustion
- **Danger Level:** high
- **Component:** cert-manager-controller

When a ResourceQuota with zero limits is applied to cert-manager, no new pods can be created. The controller should handle quota exhaustion gracefully. The chaos framework removes the quota via TTL-based cleanup.

<details>
<summary>Experiment YAML</summary>

```yaml
apiVersion: chaos.operatorchaos.io/v1alpha1
kind: ChaosExperiment
metadata:
  name: controller-quota-exhaustion
spec:
  tier: 5
  target:
    operator: cert-manager
    component: cert-manager-controller
    resource: ResourceQuota/chaos-quota-controller
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
    type: QuotaExhaustion
    dangerLevel: high
    parameters:
      quotaName: chaos-quota-controller
      pods: "0"
      cpu: "0"
      memory: "0"
    ttl: "60s"
  hypothesis:
    description: >-
      When a ResourceQuota with zero limits is applied to cert-manager,
      no new pods can be created. The controller should handle quota
      exhaustion gracefully. The chaos framework removes the quota via
      TTL-based cleanup.
    recoveryTimeout: 120s
  blastRadius:
    maxPodsAffected: 1
    allowDangerous: true
    allowedNamespaces:
      - cert-manager
```

</details>

---

#### cert-manager-rbac-revoke

- **Type:** RBACRevoke
- **Danger Level:** high
- **Component:** cert-manager-controller

Revoking the cert-manager controller's ClusterRoleBinding for issuers should cause certificate issuance to fail with RBAC errors. The controller pod remains running but cannot reconcile Issuer resources. After rollback, the ClusterRoleBinding is restored and pending issuance resumes.

<details>
<summary>Experiment YAML</summary>

```yaml
apiVersion: chaos.operatorchaos.io/v1alpha1
kind: ChaosExperiment
metadata:
  name: cert-manager-rbac-revoke
spec:
  tier: 4
  target:
    operator: cert-manager
    component: cert-manager-controller
    resource: ClusterRoleBinding/cert-manager-controller-issuers
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
    type: RBACRevoke
    dangerLevel: high
    parameters:
      bindingName: cert-manager-controller-issuers
      bindingType: ClusterRoleBinding
    ttl: "120s"
  hypothesis:
    description: >-
      Revoking the cert-manager controller's ClusterRoleBinding for issuers
      should cause certificate issuance to fail with RBAC errors. The controller
      pod remains running but cannot reconcile Issuer resources. After rollback,
      the ClusterRoleBinding is restored and pending issuance resumes.
    recoveryTimeout: 120s
  blastRadius:
    maxPodsAffected: 1
    allowDangerous: true
```

</details>

---

### webhook

#### webhook-pod-kill

- **Type:** PodKill
- **Danger Level:** low
- **Component:** webhook

When a webhook pod is killed, the Deployment controller should restart it and the component should recover within the recovery timeout.

<details>
<summary>Experiment YAML</summary>

```yaml
apiVersion: chaos.operatorchaos.io/v1alpha1
kind: ChaosExperiment
metadata:
  name: webhook-pod-kill
spec:
  tier: 1
  target:
    operator: cert-manager
    component: webhook
    resource: Deployment/cert-manager-webhook
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
    type: PodKill
    parameters:
      labelSelector: app.kubernetes.io/name=webhook
    ttl: "30s"
  hypothesis:
    description: >-
      When a webhook pod is killed, the Deployment controller should
      restart it and the component should recover within the recovery timeout.
    recoveryTimeout: 120s
  blastRadius:
    maxPodsAffected: 1
    allowedNamespaces:
      - cert-manager
```

</details>

---

#### webhook-network-partition

- **Type:** NetworkPartition
- **Danger Level:** medium
- **Component:** webhook

When webhook pods are network-isolated via a deny-all NetworkPolicy, the component should detect the partition and recover once connectivity is restored.

<details>
<summary>Experiment YAML</summary>

```yaml
apiVersion: chaos.operatorchaos.io/v1alpha1
kind: ChaosExperiment
metadata:
  name: webhook-network-partition
spec:
  tier: 2
  target:
    operator: cert-manager
    component: webhook
    resource: Deployment/cert-manager-webhook
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
      When webhook pods are network-isolated via a deny-all NetworkPolicy,
      the component should detect the partition and recover once connectivity
      is restored.
    recoveryTimeout: 180s
  blastRadius:
    maxPodsAffected: 1
    allowedNamespaces:
      - cert-manager
```

</details>

---

#### webhook-label-stomping

- **Type:** LabelStomping
- **Danger Level:** high
- **Component:** webhook

When a label used for resource discovery is overwritten on the webhook Deployment, the operator should detect the label drift and restore the correct label value.

<details>
<summary>Experiment YAML</summary>

```yaml
apiVersion: chaos.operatorchaos.io/v1alpha1
kind: ChaosExperiment
metadata:
  name: webhook-label-stomping
spec:
  tier: 3
  target:
    operator: cert-manager
    component: webhook
    resource: Deployment/cert-manager-webhook
  steadyState:
    checks:
      - type: resourceExists
        apiVersion: apps/v1
        kind: Deployment
        name: cert-manager-webhook
        namespace: cert-manager
    timeout: "30s"
  injection:
    type: LabelStomping
    dangerLevel: high
    parameters:
      apiVersion: apps/v1
      kind: Deployment
      name: cert-manager-webhook
      labelKey: app.kubernetes.io/name
      action: overwrite
    ttl: "300s"
  hypothesis:
    description: >-
      When a label used for resource discovery is overwritten on the
      webhook Deployment, the operator should detect the label drift
      and restore the correct label value.
    recoveryTimeout: 120s
  blastRadius:
    maxPodsAffected: 1
    allowDangerous: true
    allowedNamespaces:
      - cert-manager
```

</details>

---

#### webhook-quota-exhaustion

- **Type:** QuotaExhaustion
- **Danger Level:** high
- **Component:** webhook

When a ResourceQuota with zero limits is applied to cert-manager, no new pods can be created. The webhook should handle quota exhaustion gracefully. The chaos framework removes the quota via TTL-based cleanup.

<details>
<summary>Experiment YAML</summary>

```yaml
apiVersion: chaos.operatorchaos.io/v1alpha1
kind: ChaosExperiment
metadata:
  name: webhook-quota-exhaustion
spec:
  tier: 5
  target:
    operator: cert-manager
    component: webhook
    resource: ResourceQuota/chaos-quota-webhook
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
    type: QuotaExhaustion
    dangerLevel: high
    parameters:
      quotaName: chaos-quota-webhook
      pods: "0"
      cpu: "0"
      memory: "0"
    ttl: "60s"
  hypothesis:
    description: >-
      When a ResourceQuota with zero limits is applied to cert-manager,
      no new pods can be created. The webhook should handle quota
      exhaustion gracefully. The chaos framework removes the quota via
      TTL-based cleanup.
    recoveryTimeout: 120s
  blastRadius:
    maxPodsAffected: 1
    allowDangerous: true
    allowedNamespaces:
      - cert-manager
```

</details>

---

#### webhook-webhook-cert-corrupt

- **Type:** ConfigDrift
- **Danger Level:** high
- **Component:** webhook

When the webhook webhook TLS certificate is corrupted, the webhook server should fail to serve and the controller should detect and regenerate the certificate.

<details>
<summary>Experiment YAML</summary>

```yaml
apiVersion: chaos.operatorchaos.io/v1alpha1
kind: ChaosExperiment
metadata:
  name: webhook-webhook-cert-corrupt
spec:
  tier: 2
  target:
    operator: cert-manager
    component: webhook
    resource: Secret/cert-manager-webhook-ca
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
    type: ConfigDrift
    dangerLevel: high
    parameters:
      name: cert-manager-webhook-ca
      key: tls.crt
      value: "Y2hhb3MtY29ycnVwdGVk"
      resourceType: Secret
    ttl: "60s"
  hypothesis:
    description: >-
      When the webhook webhook TLS certificate is corrupted, the
      webhook server should fail to serve and the controller should
      detect and regenerate the certificate.
    recoveryTimeout: 120s
  blastRadius:
    maxPodsAffected: 1
    allowDangerous: true
    allowedNamespaces:
      - cert-manager
```

</details>

---

### cainjector

#### cainjector-pod-kill

- **Type:** PodKill
- **Danger Level:** low
- **Component:** cainjector

When a cainjector pod is killed, the Deployment controller should restart it and the component should recover within the recovery timeout.

<details>
<summary>Experiment YAML</summary>

```yaml
apiVersion: chaos.operatorchaos.io/v1alpha1
kind: ChaosExperiment
metadata:
  name: cainjector-pod-kill
spec:
  tier: 1
  target:
    operator: cert-manager
    component: cainjector
    resource: Deployment/cert-manager-cainjector
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
      When a cainjector pod is killed, the Deployment controller should
      restart it and the component should recover within the recovery timeout.
    recoveryTimeout: 120s
  blastRadius:
    maxPodsAffected: 1
    allowedNamespaces:
      - cert-manager
```

</details>

---

#### cainjector-network-partition

- **Type:** NetworkPartition
- **Danger Level:** medium
- **Component:** cainjector

When cainjector pods are network-isolated via a deny-all NetworkPolicy, the component should detect the partition and recover once connectivity is restored.

<details>
<summary>Experiment YAML</summary>

```yaml
apiVersion: chaos.operatorchaos.io/v1alpha1
kind: ChaosExperiment
metadata:
  name: cainjector-network-partition
spec:
  tier: 2
  target:
    operator: cert-manager
    component: cainjector
    resource: Deployment/cert-manager-cainjector
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
    type: NetworkPartition
    parameters:
      labelSelector: app.kubernetes.io/name=cainjector
    ttl: "60s"
  hypothesis:
    description: >-
      When cainjector pods are network-isolated via a deny-all NetworkPolicy,
      the component should detect the partition and recover once connectivity
      is restored.
    recoveryTimeout: 180s
  blastRadius:
    maxPodsAffected: 1
    allowedNamespaces:
      - cert-manager
```

</details>

---

#### cainjector-label-stomping

- **Type:** LabelStomping
- **Danger Level:** high
- **Component:** cainjector

When a label used for resource discovery is overwritten on the cainjector Deployment, the operator should detect the label drift and restore the correct label value.

<details>
<summary>Experiment YAML</summary>

```yaml
apiVersion: chaos.operatorchaos.io/v1alpha1
kind: ChaosExperiment
metadata:
  name: cainjector-label-stomping
spec:
  tier: 3
  target:
    operator: cert-manager
    component: cainjector
    resource: Deployment/cert-manager-cainjector
  steadyState:
    checks:
      - type: resourceExists
        apiVersion: apps/v1
        kind: Deployment
        name: cert-manager-cainjector
        namespace: cert-manager
    timeout: "30s"
  injection:
    type: LabelStomping
    dangerLevel: high
    parameters:
      apiVersion: apps/v1
      kind: Deployment
      name: cert-manager-cainjector
      labelKey: app.kubernetes.io/name
      action: overwrite
    ttl: "300s"
  hypothesis:
    description: >-
      When a label used for resource discovery is overwritten on the
      cainjector Deployment, the operator should detect the label drift
      and restore the correct label value.
    recoveryTimeout: 120s
  blastRadius:
    maxPodsAffected: 1
    allowDangerous: true
    allowedNamespaces:
      - cert-manager
```

</details>

---

#### cainjector-quota-exhaustion

- **Type:** QuotaExhaustion
- **Danger Level:** high
- **Component:** cainjector

When a ResourceQuota with zero limits is applied to cert-manager, no new pods can be created. The cainjector should handle quota exhaustion gracefully. The chaos framework removes the quota via TTL-based cleanup.

<details>
<summary>Experiment YAML</summary>

```yaml
apiVersion: chaos.operatorchaos.io/v1alpha1
kind: ChaosExperiment
metadata:
  name: cainjector-quota-exhaustion
spec:
  tier: 5
  target:
    operator: cert-manager
    component: cainjector
    resource: ResourceQuota/chaos-quota-cainjector
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
    type: QuotaExhaustion
    dangerLevel: high
    parameters:
      quotaName: chaos-quota-cainjector
      pods: "0"
      cpu: "0"
      memory: "0"
    ttl: "60s"
  hypothesis:
    description: >-
      When a ResourceQuota with zero limits is applied to cert-manager,
      no new pods can be created. The cainjector should handle quota
      exhaustion gracefully. The chaos framework removes the quota via
      TTL-based cleanup.
    recoveryTimeout: 120s
  blastRadius:
    maxPodsAffected: 1
    allowDangerous: true
    allowedNamespaces:
      - cert-manager
```

</details>


<!-- custom-start: known-issues -->
<!-- custom-end: known-issues -->
