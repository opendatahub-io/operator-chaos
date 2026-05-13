# Service Mesh Custom Experiments

## Writing Custom Experiments

Service Mesh experiments target two namespaces: `openshift-operators` for the operator and `openshift-ingress` for the control plane. Both require `allowDangerous: true` in the blast radius since they use `openshift-` prefixed namespaces.

### Example: CRD Mutation on Istio CR

```yaml
apiVersion: chaos.operatorchaos.io/v1alpha1
kind: ChaosExperiment
metadata:
  name: istio-cr-mutation
spec:
  tier: 3
  target:
    operator: service-mesh
    component: istiod
    resource: Istio/openshift-gateway
  steadyState:
    checks:
      - type: conditionTrue
        apiVersion: sailoperator.io/v1
        kind: Istio
        name: openshift-gateway
        namespace: openshift-ingress
        conditionType: Ready
    timeout: "30s"
  injection:
    type: CRDMutation
    dangerLevel: high
    parameters:
      apiVersion: sailoperator.io/v1
      kind: Istio
      name: openshift-gateway
      namespace: openshift-ingress
      fieldPath: spec.version
      mutatedValue: "invalid-version"
    ttl: "120s"
  hypothesis:
    description: >-
      Mutating the Istio CR's version field tests whether the operator
      detects and reverts the change, or whether it attempts to reconcile
      to an invalid state.
    recoveryTimeout: 180s
  blastRadius:
    maxPodsAffected: 1
    allowDangerous: true
```

### Running Experiments

```bash
# Validate the knowledge model
operator-chaos validate knowledge/service-mesh.yaml --knowledge

# Run a single experiment
operator-chaos run experiments/service-mesh/istiod/pod-kill.yaml \
  --knowledge knowledge/service-mesh.yaml -v

# Run the full suite
operator-chaos suite experiments/service-mesh/ \
  --knowledge knowledge/service-mesh.yaml \
  --recursive \
  --report-dir results/service-mesh/
```

### Namespace Considerations

All Service Mesh experiments operate in `openshift-` prefixed namespaces, which are protected by default. Every experiment must include `allowDangerous: true` in its blast radius configuration.

For cluster-scoped injection types (RBACRevoke, WebhookDisrupt), do not include `allowedNamespaces` as these types operate on cluster-scoped resources (ClusterRoleBindings, ValidatingWebhookConfigurations, MutatingWebhookConfigurations).
