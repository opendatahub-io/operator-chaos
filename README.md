# Operator Chaos

Chaos engineering framework for Kubernetes operators. Tests that operators correctly restore all managed resources after faults, not just that pods restart. 

Existing chaos tools (Krkn, Litmus, Chaos Mesh) test infrastructure resilience: kill a pod, verify it comes back. But Kubernetes operators manage complex resource graphs (Deployments, Services, ConfigMaps, CRDs) where the real question is, "When something breaks, does the operator put everything back the way it should be?"

Operator Chaos tests reconciliation semantics with operator-specific faults: CRD mutation, config drift, RBAC revocation, webhook disruption, finalizer blocking. It understands what each operator manages via knowledge models and produces structured verdicts (Resilient, Degraded, Failed, Inconclusive).

## Install

```bash
go install github.com/opendatahub-io/operator-chaos/cmd/operator-chaos@latest
```

## Four Usage Modes

| Mode | What It Tests | Requires Cluster? | When to Use |
|------|---------------|-------------------|-------------|
| **CLI Experiments** | Full operator recovery on a live cluster | Yes | Pre-release validation, CI/CD pipelines |
| **SDK Middleware** | Operator behavior under API-level faults | Yes (or fake client) | Integration tests, staging environments |
| **Fuzz Testing** | Reconciler correctness under random faults | No (uses fake client) | Development, unit tests, CI |
| **Upgrade Testing** | Breaking changes between versions | No (offline analysis) | Pre-upgrade validation, release qualification |

## Quick Start

Run a chaos experiment against your operator in four steps.

1. **Create a knowledge model**. Describes what your operator manages (see [Knowledge Models](https://opendatahub-io.github.io/operator-chaos/guides/knowledge-models/) in the docs).

2. **Generate an experiment skeleton**:
   ```bash
   operator-chaos init --component my-controller --operator my-operator --type PodKill > experiment.yaml
   ```

3. **Validate the experiment**:
   ```bash
   operator-chaos validate knowledge.yaml --knowledge
   operator-chaos validate experiment.yaml
   ```

4. **Dry run and execute**:
   ```bash
   operator-chaos run experiment.yaml --knowledge knowledge.yaml --dry-run
   operator-chaos run experiment.yaml --knowledge knowledge.yaml
   ```

The CLI orchestrates the full experiment lifecycle: establish steady state, inject fault, observe recovery, evaluate verdict.

## Profiles and Templates

Generate experiments from templates using profiles for different platforms (RHOAI, ODH, cert-manager). Experiments use `${VAR}` placeholders resolved against named profiles, making chaos patterns portable across operators.

```bash
# Generate all experiments for RHOAI
operator-chaos generate experiments --profile rhoai -o experiments/

# Run the generated suite
operator-chaos suite experiments/ --profile rhoai --report-dir results/

# Generate for a single component
operator-chaos generate experiments --profile rhoai --component dashboard -o experiments/
```

Built-in profiles: `rhoai`, `odh`, `cert-manager`. See [Profiles Guide](https://opendatahub-io.github.io/operator-chaos/guides/profiles/) for creating custom profiles.

## Dashboard

Web UI for visualizing experiment results, live monitoring, and operator resilience insights. Watches ChaosExperiment CRs, persists history in SQLite, serves a React frontend.

```bash
cd dashboard/ui && npm ci && npm run build && cd ../..
go build -o bin/chaos-dashboard ./dashboard/cmd/dashboard/
bin/chaos-dashboard -addr :8080 -db dashboard.db -knowledge-dir knowledge/
```

Open `http://localhost:8080`. Features include overview metrics, real-time experiment monitoring, suite comparison, and dependency graph visualization. See the [Dashboard Guide](https://opendatahub-io.github.io/operator-chaos/guides/dashboard-operator/) for details.

## Further Reading

Full documentation at [opendatahub-io.github.io/operator-chaos](https://opendatahub-io.github.io/operator-chaos/):

- [SDK Middleware](https://opendatahub-io.github.io/operator-chaos/modes/sdk/) - ChaosClient, WrapReconciler, fuzz harness integration
- [Fuzz Testing](https://opendatahub-io.github.io/operator-chaos/modes/fuzz/) - Automated fault exploration with Go's native fuzz engine
- [Failure Modes Reference](https://opendatahub-io.github.io/operator-chaos/failure-modes/) - All 14 injection types with parameters and examples
- [Knowledge Models](https://opendatahub-io.github.io/operator-chaos/guides/knowledge-models/) - Schema, validation, and examples
- [CLI Reference](https://opendatahub-io.github.io/operator-chaos/reference/cli-commands/) - All commands, flags, and usage patterns
- [E2E Testing Guide](https://opendatahub-io.github.io/operator-chaos/guides/e2e-testing/) - Full walkthrough with suite execution and expected verdicts
- [Dashboard Guide](https://opendatahub-io.github.io/operator-chaos/guides/dashboard-operator/) - Setup, views, API reference, deployment
- [CI Integration](https://opendatahub-io.github.io/operator-chaos/guides/ci-integration/) - GitHub Actions, Tekton, JUnit reporting
- [Upgrade Testing](https://opendatahub-io.github.io/operator-chaos/guides/upgrade-testing/) - Version diff engine, upgrade playbooks, simulation
- [Contributing](https://opendatahub-io.github.io/operator-chaos/contributing/) - Development setup, testing, pull request workflow

## Contributing

Fork the repo, create a feature branch, write tests first (TDD), submit a pull request. See [Contributing Guide](https://opendatahub-io.github.io/operator-chaos/contributing/) for development setup and testing conventions.
