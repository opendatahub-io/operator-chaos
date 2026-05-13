# RHOAI / ODH

Red Hat OpenShift AI (RHOAI) and Open Data Hub (ODH) share the same set of operator components. The chaos experiments work with both distributions; the only differences are namespace names and the managing operator (DataScienceCluster for RHOAI, similar CR for ODH).

| Distribution | Operator Namespace | Components Namespace | Managing CR |
|--|--|--|--|
| RHOAI | redhat-ods-operator | redhat-ods-applications | DataScienceCluster |
| ODH | opendatahub-operator-system | opendatahub | DataScienceCluster |

## Components

| Component | Experiments | Key Findings |
|--|--|--|
| [Dashboard](dashboard/index.md) | 7 | All Resilient |
| [Data Science Pipelines](data-science-pipelines/index.md) | 5 | All Resilient |
| [Feast](feast/index.md) | 4 | All Resilient |
| [KServe](kserve/index.md) | 11 | Route recovery slow (RHOAIENG-60900) |
| [LlamaStack](llamastack/index.md) | 4 | All Resilient |
| [Model Registry](model-registry/index.md) | 6 | All Resilient |
| [odh-model-controller](odh-model-controller/index.md) | 20 | DeploymentScaleZero Degraded (RHOAIENG-61458) |
| [OpenDataHub Operator](opendatahub-operator/index.md) | 5 | All Resilient |
| [Ray](ray/index.md) | 4 | All Resilient |
| [Training Operator](training-operator/index.md) | 4 | All Resilient |
| [TrustyAI](trustyai/index.md) | 3 | All Resilient |
| [Workbenches](workbenches/index.md) | 4 | All Resilient |
| [CodeFlare](codeflare/index.md) | 4 | Removed in RHOAI 3.0 |
| [ModelMesh](modelmesh/index.md) | 5 | Removed in RHOAI 3.0 |

To use these experiments with ODH instead of RHOAI, use the `--profile odh` flag or create experiments targeting the `opendatahub` namespace.
