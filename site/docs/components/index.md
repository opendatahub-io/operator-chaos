# Component Overview

Coverage matrix showing which failure modes have experiments defined for each component.

### Active Components (RHOAI 3.x / ODH)

| Component | PodKill | ConfigDrift | CRDMutation | FinalizerBlock | NetworkPartition | RBACRevoke | WebhookDisrupt | ClientFault |
|-----------|---------|-------------|-------------|----------------|------------------|------------|----------------|-------------|
| [dashboard](dashboard/index.md) | Y | Y | Y | - | Y | Y | Y | - |
| [data-science-pipelines](data-science-pipelines/index.md) | Y | Y | - | Y | Y | Y | Y | - |
| [feast](feast/index.md) | Y | - | - | Y | Y | Y | Y | - |
| [kserve](kserve/index.md) | Y | Y | Y | - | Y | - | Y | - |
| [llamastack](llamastack/index.md) | Y | Y | - | Y | Y | Y | Y | - |
| [model-registry](model-registry/index.md) | Y | - | Y | Y | Y | Y | Y | - |
| [odh-model-controller](odh-model-controller/index.md) | Y | Y | Y | Y | Y | Y | Y | Y |
| [opendatahub-operator](opendatahub-operator/index.md) | Y | - | - | Y | Y | Y | Y | - |
| [ray](ray/index.md) | Y | - | - | Y | Y | Y | Y | - |
| [training-operator](training-operator/index.md) | Y | - | - | Y | Y | Y | Y | - |
| [trustyai](trustyai/index.md) | Y | - | - | Y | Y | Y | Y | - |
| [workbenches](workbenches/index.md) | Y | - | - | - | Y | Y | Y | - |

### Removed/Replaced (RHOAI 3.x)

These components have been removed or replaced in RHOAI 3.x. Experiments are still available for ODH or RHOAI 2.x testing.

| Component | PodKill | ConfigDrift | CRDMutation | FinalizerBlock | NetworkPartition | RBACRevoke | WebhookDisrupt | ClientFault | Status |
|-----------|---------|-------------|-------------|----------------|------------------|------------|----------------|-------------|--------|
| [codeflare](codeflare/index.md) | Y | Y | - | Y | Y | Y | Y | - | Removed in RHOAI 3.0 |
| [kueue](kueue/index.md) | Y | - | - | Y | Y | Y | Y | - | Replaced by Red Hat Build of Kueue Operator |
| [modelmesh](modelmesh/index.md) | Y | Y | - | Y | Y | Y | Y | - | Removed in RHOAI 3.0 |
