# ODH Platform Chaos - Phase 1 Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Build the zero-code chaos engineering CLI that gives teams actionable resilience findings in 30 minutes with no code changes.

**Architecture:** Go CLI using cobra for commands, client-go for K8s interactions, and a deterministic experiment lifecycle (PENDING -> STEADY_STATE_PRE -> INJECTING -> OBSERVING -> STEADY_STATE_POST -> EVALUATING -> CLEANUP -> COMPLETE). The framework loads operator knowledge from YAML, executes fault injections via the K8s API, observes recovery, and produces structured JSON/JUnit reports.

**Tech Stack:** Go 1.23+, cobra (CLI), client-go (K8s), controller-runtime (client), prometheus/client_golang (metrics), go/ast (static analysis)

**Design Doc:** `docs/plans/2026-02-26-chaos-engineering-platform-design.md`

---

## Project Structure

```
odh-platform-chaos/
├── cmd/
│   └── odh-chaos/
│       └── main.go
├── api/
│   └── v1alpha1/
│       ├── types.go
│       └── zz_generated.deepcopy.go
├── pkg/
│   ├── model/
│   │   ├── knowledge.go
│   │   ├── knowledge_test.go
│   │   ├── loader.go
│   │   └── loader_test.go
│   ├── experiment/
│   │   ├── loader.go
│   │   └── loader_test.go
│   ├── injection/
│   │   ├── engine.go
│   │   ├── engine_test.go
│   │   ├── podkill.go
│   │   ├── podkill_test.go
│   │   ├── network.go
│   │   ├── network_test.go
│   │   ├── crdmutation.go
│   │   ├── crdmutation_test.go
│   │   ├── configdrift.go
│   │   └── configdrift_test.go
│   ├── observer/
│   │   ├── engine.go
│   │   ├── engine_test.go
│   │   ├── kubernetes.go
│   │   ├── kubernetes_test.go
│   │   ├── prometheus.go
│   │   ├── prometheus_test.go
│   │   ├── reconciliation.go
│   │   └── reconciliation_test.go
│   ├── evaluator/
│   │   ├── engine.go
│   │   └── engine_test.go
│   ├── reporter/
│   │   ├── json.go
│   │   ├── json_test.go
│   │   ├── junit.go
│   │   └── junit_test.go
│   ├── orchestrator/
│   │   ├── lifecycle.go
│   │   └── lifecycle_test.go
│   ├── safety/
│   │   ├── blastradius.go
│   │   ├── blastradius_test.go
│   │   ├── mutex.go
│   │   └── mutex_test.go
│   └── analyzer/
│       ├── analyzer.go
│       ├── analyzer_test.go
│       ├── patterns.go
│       └── patterns_test.go
├── internal/
│   └── cli/
│       ├── root.go
│       ├── run.go
│       ├── validate.go
│       ├── analyze.go
│       ├── suite.go
│       ├── clean.go
│       ├── report.go
│       └── init.go
├── knowledge/
│   └── (generated operator YAML files)
├── experiments/
│   └── (experiment YAML files)
├── testdata/
│   ├── knowledge/
│   │   └── test-operator.yaml
│   ├── experiments/
│   │   ├── valid-experiment.yaml
│   │   └── invalid-experiment.yaml
│   └── go-source/
│       └── (sample Go files for analyzer tests)
├── docs/
│   └── plans/
├── Makefile
├── go.mod
├── go.sum
├── .gitignore
└── README.md
```

---

## Task 1: Project Scaffold and Go Module

**Files:**
- Create: `go.mod`
- Create: `cmd/odh-chaos/main.go`
- Create: `internal/cli/root.go`
- Create: `Makefile`
- Create: `.gitignore`

**Step 1: Initialize Go module**

```bash
cd /Users/ugogiordano/workdir/rhoai/opendatahub-io/odh-platform-chaos
go mod init github.com/opendatahub-io/odh-platform-chaos
```

**Step 2: Create main.go entry point**

Create `cmd/odh-chaos/main.go`:
```go
package main

import (
	"os"

	"github.com/opendatahub-io/odh-platform-chaos/internal/cli"
)

func main() {
	if err := cli.NewRootCommand().Execute(); err != nil {
		os.Exit(1)
	}
}
```

**Step 3: Create root CLI command with cobra**

Create `internal/cli/root.go`:
```go
package cli

import (
	"github.com/spf13/cobra"
)

func NewRootCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "odh-chaos",
		Short: "Chaos engineering framework for OpenDataHub operators",
		Long: `ODH Platform Chaos tests operator reconciliation semantics.
It validates that operators recover managed resources correctly after
fault injection, not just that pods restart.`,
	}

	cmd.PersistentFlags().String("kubeconfig", "", "path to kubeconfig file")
	cmd.PersistentFlags().String("namespace", "opendatahub", "target namespace")
	cmd.PersistentFlags().BoolP("verbose", "v", false, "verbose output")

	cmd.AddCommand(
		newValidateCommand(),
	)

	return cmd
}
```

Create a placeholder `internal/cli/validate.go`:
```go
package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newValidateCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "validate [experiment.yaml]",
		Short: "Validate experiment YAML without running",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Printf("Validating %s...\n", args[0])
			return nil
		},
	}
}
```

**Step 4: Create Makefile**

Create `Makefile`:
```makefile
BINARY := odh-chaos
PKG := github.com/opendatahub-io/odh-platform-chaos
CMD := ./cmd/odh-chaos

.PHONY: build test lint clean

build:
	go build -o bin/$(BINARY) $(CMD)

test:
	go test ./... -v -count=1

test-short:
	go test ./... -short -count=1

lint:
	golangci-lint run ./...

clean:
	rm -rf bin/

install: build
	cp bin/$(BINARY) $(GOPATH)/bin/
```

**Step 5: Create .gitignore**

Create `.gitignore`:
```
bin/
vendor/
*.exe
*.test
*.out
.idea/
.vscode/
.DS_Store
results/
```

**Step 6: Install dependencies and verify build**

```bash
cd /Users/ugogiordano/workdir/rhoai/opendatahub-io/odh-platform-chaos
go get github.com/spf13/cobra@latest
go mod tidy
make build
./bin/odh-chaos --help
```

Expected: Help output showing `odh-chaos` with `validate` subcommand.

**Step 7: Commit**

```bash
git add -A
git commit -m "feat: initialize project scaffold with cobra CLI"
```

---

## Task 2: Experiment Types (CRD-Ready)

**Files:**
- Create: `api/v1alpha1/types.go`
- Create: `api/v1alpha1/types_test.go`
- Create: `testdata/experiments/valid-experiment.yaml`
- Create: `testdata/experiments/invalid-experiment.yaml`

**Step 1: Write test for experiment type serialization**

Create `api/v1alpha1/types_test.go`:
```go
package v1alpha1

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"sigs.k8s.io/yaml"
)

func TestChaosExperimentYAMLRoundTrip(t *testing.T) {
	exp := ChaosExperiment{
		Metadata: Metadata{
			Name: "dashboard-pod-kill",
			Labels: map[string]string{
				"component": "dashboard",
			},
		},
		Spec: ChaosExperimentSpec{
			Target: TargetSpec{
				Operator:  "opendatahub-operator",
				Component: "dashboard",
				Resource:  "Deployment/odh-dashboard",
			},
			Hypothesis: HypothesisSpec{
				Description:      "Dashboard recovers from pod kill within 60s",
				ExpectedBehavior: "Deployment controller recreates pod",
				RecoveryTimeout:  Duration{60 * time.Second},
			},
			Injection: InjectionSpec{
				Type:     PodKill,
				Count:    1,
				Duration: Duration{0},
				TTL:      Duration{300 * time.Second},
				Parameters: map[string]string{
					"signal":        "SIGKILL",
					"labelSelector": "app.kubernetes.io/part-of=dashboard",
				},
			},
			BlastRadius: BlastRadiusSpec{
				MaxPodsAffected:     1,
				MaxConcurrentFaults: 1,
				AllowedNamespaces:   []string{"opendatahub"},
			},
		},
	}

	data, err := yaml.Marshal(exp)
	require.NoError(t, err)

	var loaded ChaosExperiment
	err = yaml.Unmarshal(data, &loaded)
	require.NoError(t, err)

	assert.Equal(t, exp.Metadata.Name, loaded.Metadata.Name)
	assert.Equal(t, exp.Spec.Target.Component, loaded.Spec.Target.Component)
	assert.Equal(t, exp.Spec.Injection.Type, loaded.Spec.Injection.Type)
	assert.Equal(t, PodKill, loaded.Spec.Injection.Type)
}

func TestChaosExperimentLoadFromFile(t *testing.T) {
	data, err := os.ReadFile("../../testdata/experiments/valid-experiment.yaml")
	require.NoError(t, err)

	var exp ChaosExperiment
	err = yaml.Unmarshal(data, &exp)
	require.NoError(t, err)

	assert.Equal(t, "dashboard-pod-kill-recovery", exp.Metadata.Name)
	assert.Equal(t, PodKill, exp.Spec.Injection.Type)
	assert.Equal(t, 1, exp.Spec.Injection.Count)
	assert.Equal(t, 1, exp.Spec.BlastRadius.MaxPodsAffected)
	assert.NotEmpty(t, exp.Spec.Hypothesis.Description)
}

func TestInjectionTypes(t *testing.T) {
	types := []InjectionType{
		PodKill, PodFailure, NetworkPartition, NetworkLatency,
		ResourceExhaustion, CRDMutation, ConfigDrift,
		WebhookDisrupt, RBACRevoke, FinalizerBlock, OwnerRefOrphan,
	}
	for _, it := range types {
		assert.NotEmpty(t, string(it))
	}
}

func TestVerdictValues(t *testing.T) {
	assert.Equal(t, Verdict("Resilient"), Resilient)
	assert.Equal(t, Verdict("Degraded"), Degraded)
	assert.Equal(t, Verdict("Failed"), Failed)
	assert.Equal(t, Verdict("Inconclusive"), Inconclusive)
}
```

**Step 2: Run tests to verify they fail**

```bash
cd /Users/ugogiordano/workdir/rhoai/opendatahub-io/odh-platform-chaos
go test ./api/... -v
```

Expected: FAIL (types not defined yet)

**Step 3: Implement experiment types**

Create `api/v1alpha1/types.go`:
```go
package v1alpha1

import (
	"encoding/json"
	"time"
)

// ChaosExperiment defines a chaos engineering experiment.
// Designed as CRD-ready: kubebuilder markers will be added
// when controller mode is implemented.
type ChaosExperiment struct {
	APIVersion string              `json:"apiVersion,omitempty" yaml:"apiVersion,omitempty"`
	Kind       string              `json:"kind,omitempty" yaml:"kind,omitempty"`
	Metadata   Metadata            `json:"metadata" yaml:"metadata"`
	Spec       ChaosExperimentSpec `json:"spec" yaml:"spec"`
	Status     ChaosExperimentStatus `json:"status,omitempty" yaml:"status,omitempty"`
}

type Metadata struct {
	Name      string            `json:"name" yaml:"name"`
	Namespace string            `json:"namespace,omitempty" yaml:"namespace,omitempty"`
	Labels    map[string]string `json:"labels,omitempty" yaml:"labels,omitempty"`
}

type ChaosExperimentSpec struct {
	Target      TargetSpec      `json:"target" yaml:"target"`
	SteadyState SteadyStateDef  `json:"steadyState,omitempty" yaml:"steadyState,omitempty"`
	Injection   InjectionSpec   `json:"injection" yaml:"injection"`
	Observation ObservationSpec `json:"observation,omitempty" yaml:"observation,omitempty"`
	BlastRadius BlastRadiusSpec `json:"blastRadius" yaml:"blastRadius"`
	Hypothesis  HypothesisSpec  `json:"hypothesis" yaml:"hypothesis"`
}

type TargetSpec struct {
	Operator  string `json:"operator" yaml:"operator"`
	Component string `json:"component" yaml:"component"`
	Resource  string `json:"resource,omitempty" yaml:"resource,omitempty"`
}

type SteadyStateDef struct {
	Checks  []SteadyStateCheck `json:"checks,omitempty" yaml:"checks,omitempty"`
	Timeout Duration           `json:"timeout,omitempty" yaml:"timeout,omitempty"`
}

type SteadyStateCheck struct {
	Type          string `json:"type" yaml:"type"`
	APIVersion    string `json:"apiVersion,omitempty" yaml:"apiVersion,omitempty"`
	Kind          string `json:"kind,omitempty" yaml:"kind,omitempty"`
	Name          string `json:"name,omitempty" yaml:"name,omitempty"`
	Namespace     string `json:"namespace,omitempty" yaml:"namespace,omitempty"`
	ConditionType string `json:"conditionType,omitempty" yaml:"conditionType,omitempty"`
	Query         string `json:"query,omitempty" yaml:"query,omitempty"`
	Operator      string `json:"operator,omitempty" yaml:"operator,omitempty"`
	Value         string `json:"value,omitempty" yaml:"value,omitempty"`
	For           string `json:"for,omitempty" yaml:"for,omitempty"`
}

type InjectionSpec struct {
	Type        InjectionType     `json:"type" yaml:"type"`
	Parameters  map[string]string `json:"parameters,omitempty" yaml:"parameters,omitempty"`
	Duration    Duration          `json:"duration,omitempty" yaml:"duration,omitempty"`
	Count       int               `json:"count,omitempty" yaml:"count,omitempty"`
	TTL         Duration          `json:"ttl,omitempty" yaml:"ttl,omitempty"`
	DangerLevel string            `json:"dangerLevel,omitempty" yaml:"dangerLevel,omitempty"`
}

type InjectionType string

const (
	PodKill            InjectionType = "PodKill"
	PodFailure         InjectionType = "PodFailure"
	NetworkPartition   InjectionType = "NetworkPartition"
	NetworkLatency     InjectionType = "NetworkLatency"
	ResourceExhaustion InjectionType = "ResourceExhaustion"
	CRDMutation        InjectionType = "CRDMutation"
	ConfigDrift        InjectionType = "ConfigDrift"
	WebhookDisrupt     InjectionType = "WebhookDisrupt"
	RBACRevoke         InjectionType = "RBACRevoke"
	FinalizerBlock     InjectionType = "FinalizerBlock"
	OwnerRefOrphan     InjectionType = "OwnerRefOrphan"
	SourceHook         InjectionType = "SourceHook"
)

type ObservationSpec struct {
	Interval             Duration `json:"interval,omitempty" yaml:"interval,omitempty"`
	Duration             Duration `json:"duration,omitempty" yaml:"duration,omitempty"`
	TrackReconcileCycles bool     `json:"trackReconcileCycles,omitempty" yaml:"trackReconcileCycles,omitempty"`
}

type BlastRadiusSpec struct {
	MaxPodsAffected     int      `json:"maxPodsAffected" yaml:"maxPodsAffected"`
	MaxConcurrentFaults int      `json:"maxConcurrentFaults,omitempty" yaml:"maxConcurrentFaults,omitempty"`
	AllowedNamespaces   []string `json:"allowedNamespaces" yaml:"allowedNamespaces"`
	ForbiddenResources  []string `json:"forbiddenResources,omitempty" yaml:"forbiddenResources,omitempty"`
	RequireLabel        string   `json:"requireLabel,omitempty" yaml:"requireLabel,omitempty"`
	AllowDangerous      bool     `json:"allowDangerous,omitempty" yaml:"allowDangerous,omitempty"`
	DryRun              bool     `json:"dryRun,omitempty" yaml:"dryRun,omitempty"`
}

type HypothesisSpec struct {
	Description      string   `json:"description" yaml:"description"`
	ExpectedBehavior string   `json:"expectedBehavior" yaml:"expectedBehavior"`
	RecoveryTimeout  Duration `json:"recoveryTimeout" yaml:"recoveryTimeout"`
}

// Status types

type ChaosExperimentStatus struct {
	Phase           ExperimentPhase `json:"phase,omitempty" yaml:"phase,omitempty"`
	Verdict         Verdict         `json:"verdict,omitempty" yaml:"verdict,omitempty"`
	StartTime       *time.Time      `json:"startTime,omitempty" yaml:"startTime,omitempty"`
	EndTime         *time.Time      `json:"endTime,omitempty" yaml:"endTime,omitempty"`
	SteadyStatePre  *CheckResult    `json:"steadyStatePre,omitempty" yaml:"steadyStatePre,omitempty"`
	SteadyStatePost *CheckResult    `json:"steadyStatePost,omitempty" yaml:"steadyStatePost,omitempty"`
	InjectionLog    []InjectionEvent `json:"injectionLog,omitempty" yaml:"injectionLog,omitempty"`
	Observations    []Observation   `json:"observations,omitempty" yaml:"observations,omitempty"`
}

type ExperimentPhase string

const (
	PhasePending        ExperimentPhase = "Pending"
	PhaseSteadyStatePre ExperimentPhase = "SteadyStatePre"
	PhaseInjecting      ExperimentPhase = "Injecting"
	PhaseObserving      ExperimentPhase = "Observing"
	PhaseSteadyStatePost ExperimentPhase = "SteadyStatePost"
	PhaseEvaluating     ExperimentPhase = "Evaluating"
	PhaseCleanup        ExperimentPhase = "Cleanup"
	PhaseComplete       ExperimentPhase = "Complete"
	PhaseAborted        ExperimentPhase = "Aborted"
)

type Verdict string

const (
	Resilient    Verdict = "Resilient"
	Degraded     Verdict = "Degraded"
	Failed       Verdict = "Failed"
	Inconclusive Verdict = "Inconclusive"
)

type CheckResult struct {
	Passed       bool              `json:"passed" yaml:"passed"`
	ChecksRun    int               `json:"checksRun" yaml:"checksRun"`
	ChecksPassed int               `json:"checksPassed" yaml:"checksPassed"`
	Details      []CheckDetail     `json:"details,omitempty" yaml:"details,omitempty"`
	Timestamp    time.Time         `json:"timestamp" yaml:"timestamp"`
}

type CheckDetail struct {
	Check  SteadyStateCheck `json:"check" yaml:"check"`
	Passed bool             `json:"passed" yaml:"passed"`
	Value  string           `json:"value,omitempty" yaml:"value,omitempty"`
	Error  string           `json:"error,omitempty" yaml:"error,omitempty"`
}

type InjectionEvent struct {
	Timestamp time.Time         `json:"timestamp" yaml:"timestamp"`
	Type      InjectionType     `json:"type" yaml:"type"`
	Target    string            `json:"target" yaml:"target"`
	Action    string            `json:"action" yaml:"action"`
	Details   map[string]string `json:"details,omitempty" yaml:"details,omitempty"`
}

type Observation struct {
	Timestamp time.Time              `json:"timestamp" yaml:"timestamp"`
	Metrics   map[string]interface{} `json:"metrics,omitempty" yaml:"metrics,omitempty"`
}

// Duration wraps time.Duration for YAML/JSON serialization
type Duration struct {
	time.Duration
}

func (d Duration) MarshalJSON() ([]byte, error) {
	return json.Marshal(d.String())
}

func (d *Duration) UnmarshalJSON(b []byte) error {
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return err
	}
	dur, err := time.ParseDuration(s)
	if err != nil {
		return err
	}
	d.Duration = dur
	return nil
}

func (d Duration) MarshalYAML() (interface{}, error) {
	return d.String(), nil
}

func (d *Duration) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var s string
	if err := unmarshal(&s); err != nil {
		return err
	}
	dur, err := time.ParseDuration(s)
	if err != nil {
		return err
	}
	d.Duration = dur
	return nil
}
```

**Step 4: Create test data files**

Create `testdata/experiments/valid-experiment.yaml`:
```yaml
apiVersion: chaos.opendatahub.io/v1alpha1
kind: ChaosExperiment
metadata:
  name: dashboard-pod-kill-recovery
  labels:
    component: dashboard
    severity: standard
spec:
  target:
    operator: opendatahub-operator
    component: dashboard
    resource: Deployment/odh-dashboard
  hypothesis:
    description: "Dashboard recovers from pod termination within 60s"
    expectedBehavior: "Deployment controller recreates pod, all replicas ready"
    recoveryTimeout: "60s"
  injection:
    type: PodKill
    parameters:
      signal: "SIGKILL"
      labelSelector: "app.kubernetes.io/part-of=dashboard"
    count: 1
    duration: "0s"
    ttl: "300s"
  observation:
    interval: "5s"
    duration: "120s"
    trackReconcileCycles: true
  steadyState:
    checks:
      - type: conditionTrue
        apiVersion: apps/v1
        kind: Deployment
        name: odh-dashboard
        namespace: opendatahub
        conditionType: Available
    timeout: "30s"
  blastRadius:
    maxPodsAffected: 1
    maxConcurrentFaults: 1
    allowedNamespaces:
      - opendatahub
    dryRun: false
```

**Step 5: Run tests to verify they pass**

```bash
go get github.com/stretchr/testify@latest
go get sigs.k8s.io/yaml@latest
go mod tidy
go test ./api/... -v
```

Expected: PASS

**Step 6: Commit**

```bash
git add -A
git commit -m "feat: add CRD-ready experiment types with YAML serialization"
```

---

## Task 3: Operator Knowledge Model

**Files:**
- Create: `pkg/model/knowledge.go`
- Create: `pkg/model/knowledge_test.go`
- Create: `pkg/model/loader.go`
- Create: `pkg/model/loader_test.go`
- Create: `testdata/knowledge/test-operator.yaml`

**Step 1: Write tests for knowledge model**

Create `pkg/model/knowledge_test.go`:
```go
package model

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadKnowledge(t *testing.T) {
	k, err := LoadKnowledge("../../testdata/knowledge/test-operator.yaml")
	require.NoError(t, err)

	assert.Equal(t, "test-operator", k.Operator.Name)
	assert.Equal(t, "test-ns", k.Operator.Namespace)
	assert.Len(t, k.Components, 2)
}

func TestGetComponent(t *testing.T) {
	k, err := LoadKnowledge("../../testdata/knowledge/test-operator.yaml")
	require.NoError(t, err)

	comp := k.GetComponent("dashboard")
	require.NotNil(t, comp)
	assert.Equal(t, "dashboard", comp.Name)
	assert.Len(t, comp.ManagedResources, 2)
	assert.Equal(t, "Deployment", comp.ManagedResources[0].Kind)
}

func TestGetComponentNotFound(t *testing.T) {
	k, err := LoadKnowledge("../../testdata/knowledge/test-operator.yaml")
	require.NoError(t, err)

	comp := k.GetComponent("nonexistent")
	assert.Nil(t, comp)
}

func TestKnowledgeRecoveryDefaults(t *testing.T) {
	k, err := LoadKnowledge("../../testdata/knowledge/test-operator.yaml")
	require.NoError(t, err)

	assert.Equal(t, 300*time.Second, k.Recovery.ReconcileTimeout.Duration)
	assert.Equal(t, 10, k.Recovery.MaxReconcileCycles)
}

func TestManagedResourceExpectedSpec(t *testing.T) {
	k, err := LoadKnowledge("../../testdata/knowledge/test-operator.yaml")
	require.NoError(t, err)

	comp := k.GetComponent("dashboard")
	require.NotNil(t, comp)

	deploy := comp.ManagedResources[0]
	assert.Equal(t, "Deployment", deploy.Kind)
	assert.NotNil(t, deploy.ExpectedSpec)
}
```

**Step 2: Run tests to verify they fail**

```bash
go test ./pkg/model/... -v
```

Expected: FAIL

**Step 3: Implement knowledge model types and loader**

Create `pkg/model/knowledge.go`:
```go
package model

import (
	v1alpha1 "github.com/opendatahub-io/odh-platform-chaos/api/v1alpha1"
)

type OperatorKnowledge struct {
	Operator   OperatorMeta         `yaml:"operator"`
	Components []ComponentModel     `yaml:"components"`
	Recovery   RecoveryExpectations `yaml:"recovery"`
}

type OperatorMeta struct {
	Name       string `yaml:"name"`
	Namespace  string `yaml:"namespace"`
	Repository string `yaml:"repository,omitempty"`
}

type ComponentModel struct {
	Name             string                 `yaml:"name"`
	Controller       string                 `yaml:"controller"`
	ManagedResources []ManagedResource      `yaml:"managedResources"`
	Dependencies     []string               `yaml:"dependencies,omitempty"`
	SteadyState      v1alpha1.SteadyStateDef `yaml:"steadyState,omitempty"`
	Webhooks         []WebhookSpec          `yaml:"webhooks,omitempty"`
	Finalizers       []string               `yaml:"finalizers,omitempty"`
}

type ManagedResource struct {
	APIVersion   string                 `yaml:"apiVersion"`
	Kind         string                 `yaml:"kind"`
	Name         string                 `yaml:"name"`
	Namespace    string                 `yaml:"namespace,omitempty"`
	Labels       map[string]string      `yaml:"labels,omitempty"`
	OwnerRef     string                 `yaml:"ownerRef,omitempty"`
	ExpectedSpec map[string]interface{} `yaml:"expectedSpec,omitempty"`
}

type WebhookSpec struct {
	Name string `yaml:"name"`
	Type string `yaml:"type"` // validating, mutating
	Path string `yaml:"path"`
}

type RecoveryExpectations struct {
	ReconcileTimeout   v1alpha1.Duration `yaml:"reconcileTimeout"`
	MaxReconcileCycles int               `yaml:"maxReconcileCycles"`
}

func (k *OperatorKnowledge) GetComponent(name string) *ComponentModel {
	for i := range k.Components {
		if k.Components[i].Name == name {
			return &k.Components[i]
		}
	}
	return nil
}
```

Create `pkg/model/loader.go`:
```go
package model

import (
	"fmt"
	"os"

	"sigs.k8s.io/yaml"
)

func LoadKnowledge(path string) (*OperatorKnowledge, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading knowledge file %s: %w", path, err)
	}

	var k OperatorKnowledge
	if err := yaml.Unmarshal(data, &k); err != nil {
		return nil, fmt.Errorf("parsing knowledge file %s: %w", path, err)
	}

	return &k, nil
}
```

Create `testdata/knowledge/test-operator.yaml`:
```yaml
operator:
  name: test-operator
  namespace: test-ns
  repository: https://github.com/example/test-operator

components:
  - name: dashboard
    controller: DataScienceCluster
    managedResources:
      - apiVersion: apps/v1
        kind: Deployment
        name: test-dashboard
        namespace: test-ns
        labels:
          app: dashboard
        ownerRef: Dashboard
        expectedSpec:
          replicas: 2
      - apiVersion: v1
        kind: Service
        name: test-dashboard
        namespace: test-ns
        ownerRef: Dashboard
    dependencies: []
    steadyState:
      checks:
        - type: conditionTrue
          apiVersion: apps/v1
          kind: Deployment
          name: test-dashboard
          conditionType: Available
      timeout: "60s"

  - name: model-controller
    controller: DataScienceCluster
    managedResources:
      - apiVersion: apps/v1
        kind: Deployment
        name: test-model-controller
        namespace: test-ns
        ownerRef: Kserve
    dependencies:
      - kserve

recovery:
  reconcileTimeout: "300s"
  maxReconcileCycles: 10
```

**Step 4: Run tests to verify they pass**

```bash
go test ./pkg/model/... -v
```

Expected: PASS

**Step 5: Commit**

```bash
git add -A
git commit -m "feat: add operator knowledge model with YAML loader"
```

---

## Task 4: Experiment Loader and Validator

**Files:**
- Create: `pkg/experiment/loader.go`
- Create: `pkg/experiment/loader_test.go`
- Create: `testdata/experiments/invalid-experiment.yaml`

**Step 1: Write tests for experiment loading and validation**

Create `pkg/experiment/loader_test.go`:
```go
package experiment

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadExperiment(t *testing.T) {
	exp, err := Load("../../testdata/experiments/valid-experiment.yaml")
	require.NoError(t, err)
	assert.Equal(t, "dashboard-pod-kill-recovery", exp.Metadata.Name)
}

func TestLoadExperimentFileNotFound(t *testing.T) {
	_, err := Load("nonexistent.yaml")
	assert.Error(t, err)
}

func TestValidateExperiment(t *testing.T) {
	exp, err := Load("../../testdata/experiments/valid-experiment.yaml")
	require.NoError(t, err)

	errs := Validate(exp)
	assert.Empty(t, errs)
}

func TestValidateExperimentMissingFields(t *testing.T) {
	exp, err := Load("../../testdata/experiments/invalid-experiment.yaml")
	require.NoError(t, err)

	errs := Validate(exp)
	assert.NotEmpty(t, errs)
}

func TestValidateBlastRadius(t *testing.T) {
	exp, err := Load("../../testdata/experiments/valid-experiment.yaml")
	require.NoError(t, err)

	// Valid: maxPodsAffected > 0 and allowedNamespaces not empty
	errs := Validate(exp)
	assert.Empty(t, errs)

	// Invalid: no allowed namespaces
	exp.Spec.BlastRadius.AllowedNamespaces = nil
	errs = Validate(exp)
	assert.NotEmpty(t, errs)
}
```

**Step 2: Run tests to verify they fail**

```bash
go test ./pkg/experiment/... -v
```

Expected: FAIL

**Step 3: Implement loader and validator**

Create `pkg/experiment/loader.go`:
```go
package experiment

import (
	"fmt"
	"os"

	v1alpha1 "github.com/opendatahub-io/odh-platform-chaos/api/v1alpha1"
	"sigs.k8s.io/yaml"
)

func Load(path string) (*v1alpha1.ChaosExperiment, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading experiment file %s: %w", path, err)
	}

	var exp v1alpha1.ChaosExperiment
	if err := yaml.Unmarshal(data, &exp); err != nil {
		return nil, fmt.Errorf("parsing experiment file %s: %w", path, err)
	}

	return &exp, nil
}

func Validate(exp *v1alpha1.ChaosExperiment) []string {
	var errs []string

	if exp.Metadata.Name == "" {
		errs = append(errs, "metadata.name is required")
	}
	if exp.Spec.Target.Operator == "" {
		errs = append(errs, "spec.target.operator is required")
	}
	if exp.Spec.Target.Component == "" {
		errs = append(errs, "spec.target.component is required")
	}
	if exp.Spec.Injection.Type == "" {
		errs = append(errs, "spec.injection.type is required")
	}
	if exp.Spec.Hypothesis.Description == "" {
		errs = append(errs, "spec.hypothesis.description is required")
	}
	if exp.Spec.BlastRadius.AllowedNamespaces == nil || len(exp.Spec.BlastRadius.AllowedNamespaces) == 0 {
		errs = append(errs, "spec.blastRadius.allowedNamespaces must not be empty")
	}

	return errs
}
```

Create `testdata/experiments/invalid-experiment.yaml`:
```yaml
apiVersion: chaos.opendatahub.io/v1alpha1
kind: ChaosExperiment
metadata:
  name: ""
spec:
  target:
    operator: ""
    component: ""
  hypothesis:
    description: ""
    recoveryTimeout: "60s"
  injection:
    type: ""
    ttl: "300s"
  blastRadius:
    maxPodsAffected: 1
    allowedNamespaces: []
```

**Step 4: Run tests**

```bash
go test ./pkg/experiment/... -v
```

Expected: PASS

**Step 5: Wire validate command to use the loader**

Update `internal/cli/validate.go`:
```go
package cli

import (
	"fmt"

	"github.com/opendatahub-io/odh-platform-chaos/pkg/experiment"
	"github.com/spf13/cobra"
)

func newValidateCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "validate [experiment.yaml]",
		Short: "Validate experiment YAML without running",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			exp, err := experiment.Load(args[0])
			if err != nil {
				return fmt.Errorf("loading experiment: %w", err)
			}

			errs := experiment.Validate(exp)
			if len(errs) > 0 {
				fmt.Println("Validation FAILED:")
				for _, e := range errs {
					fmt.Printf("  - %s\n", e)
				}
				return fmt.Errorf("%d validation errors", len(errs))
			}

			fmt.Printf("Experiment '%s' is valid.\n", exp.Metadata.Name)
			return nil
		},
	}
}
```

**Step 6: Build and test CLI**

```bash
make build
./bin/odh-chaos validate testdata/experiments/valid-experiment.yaml
./bin/odh-chaos validate testdata/experiments/invalid-experiment.yaml
```

Expected: First command prints "valid", second prints errors.

**Step 7: Commit**

```bash
git add -A
git commit -m "feat: add experiment loader, validator, and validate CLI command"
```

---

## Task 5: Safety - Blast Radius Validation

**Files:**
- Create: `pkg/safety/blastradius.go`
- Create: `pkg/safety/blastradius_test.go`

**Step 1: Write tests**

Create `pkg/safety/blastradius_test.go`:
```go
package safety

import (
	"testing"

	v1alpha1 "github.com/opendatahub-io/odh-platform-chaos/api/v1alpha1"
	"github.com/stretchr/testify/assert"
)

func TestValidateBlastRadius(t *testing.T) {
	tests := []struct {
		name     string
		spec     v1alpha1.BlastRadiusSpec
		target   string
		wantErr  bool
	}{
		{
			name: "valid blast radius",
			spec: v1alpha1.BlastRadiusSpec{
				MaxPodsAffected:   1,
				AllowedNamespaces: []string{"opendatahub"},
			},
			target:  "opendatahub",
			wantErr: false,
		},
		{
			name: "namespace not allowed",
			spec: v1alpha1.BlastRadiusSpec{
				MaxPodsAffected:   1,
				AllowedNamespaces: []string{"opendatahub"},
			},
			target:  "kube-system",
			wantErr: true,
		},
		{
			name: "zero pods allowed",
			spec: v1alpha1.BlastRadiusSpec{
				MaxPodsAffected:   0,
				AllowedNamespaces: []string{"opendatahub"},
			},
			target:  "opendatahub",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateBlastRadius(tt.spec, tt.target, 1)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestCheckDangerLevel(t *testing.T) {
	err := CheckDangerLevel("high", false)
	assert.Error(t, err)

	err = CheckDangerLevel("high", true)
	assert.NoError(t, err)

	err = CheckDangerLevel("", false)
	assert.NoError(t, err)
}
```

**Step 2: Run tests to verify they fail**

```bash
go test ./pkg/safety/... -v
```

**Step 3: Implement blast radius validation**

Create `pkg/safety/blastradius.go`:
```go
package safety

import (
	"fmt"

	v1alpha1 "github.com/opendatahub-io/odh-platform-chaos/api/v1alpha1"
)

func ValidateBlastRadius(spec v1alpha1.BlastRadiusSpec, targetNamespace string, affectedCount int) error {
	if spec.MaxPodsAffected <= 0 {
		return fmt.Errorf("maxPodsAffected must be > 0, got %d", spec.MaxPodsAffected)
	}

	if affectedCount > spec.MaxPodsAffected {
		return fmt.Errorf("blast radius exceeded: %d affected > %d max", affectedCount, spec.MaxPodsAffected)
	}

	allowed := false
	for _, ns := range spec.AllowedNamespaces {
		if ns == targetNamespace {
			allowed = true
			break
		}
	}
	if !allowed {
		return fmt.Errorf("namespace %q not in allowed list %v", targetNamespace, spec.AllowedNamespaces)
	}

	for _, forbidden := range spec.ForbiddenResources {
		if forbidden == targetNamespace {
			return fmt.Errorf("resource %q is in forbidden list", forbidden)
		}
	}

	return nil
}

func CheckDangerLevel(level string, allowDangerous bool) error {
	if level == "high" && !allowDangerous {
		return fmt.Errorf("injection with dangerLevel=high requires blastRadius.allowDangerous=true")
	}
	return nil
}
```

**Step 4: Run tests**

```bash
go test ./pkg/safety/... -v
```

Expected: PASS

**Step 5: Commit**

```bash
git add -A
git commit -m "feat: add blast radius validation and danger level checks"
```

---

## Task 6: Safety - Experiment Mutual Exclusion

**Files:**
- Create: `pkg/safety/mutex.go`
- Create: `pkg/safety/mutex_test.go`

**Step 1: Write tests**

Create `pkg/safety/mutex_test.go`:
```go
package safety

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExperimentLock(t *testing.T) {
	lock := NewLocalExperimentLock()

	// First lock should succeed
	err := lock.Acquire(context.Background(), "opendatahub-operator", "test-exp-1")
	require.NoError(t, err)

	// Second lock on same operator should fail
	err = lock.Acquire(context.Background(), "opendatahub-operator", "test-exp-2")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "test-exp-1")

	// Release and re-acquire should work
	lock.Release("opendatahub-operator")

	err = lock.Acquire(context.Background(), "opendatahub-operator", "test-exp-2")
	assert.NoError(t, err)

	lock.Release("opendatahub-operator")
}

func TestExperimentLockDifferentOperators(t *testing.T) {
	lock := NewLocalExperimentLock()

	err := lock.Acquire(context.Background(), "operator-a", "exp-1")
	require.NoError(t, err)

	// Different operator should work
	err = lock.Acquire(context.Background(), "operator-b", "exp-2")
	assert.NoError(t, err)

	lock.Release("operator-a")
	lock.Release("operator-b")
}
```

**Step 2: Run tests to verify they fail**

```bash
go test ./pkg/safety/... -v -run TestExperiment
```

**Step 3: Implement local experiment lock**

Create `pkg/safety/mutex.go`:
```go
package safety

import (
	"context"
	"fmt"
	"sync"
)

type ExperimentLock interface {
	Acquire(ctx context.Context, operator string, experimentName string) error
	Release(operator string)
}

type localExperimentLock struct {
	mu    sync.Mutex
	locks map[string]string // operator -> experimentName
}

func NewLocalExperimentLock() ExperimentLock {
	return &localExperimentLock{
		locks: make(map[string]string),
	}
}

func (l *localExperimentLock) Acquire(_ context.Context, operator string, experimentName string) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if existing, ok := l.locks[operator]; ok {
		return fmt.Errorf("operator %q already has active experiment %q", operator, existing)
	}

	l.locks[operator] = experimentName
	return nil
}

func (l *localExperimentLock) Release(operator string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.locks, operator)
}
```

**Step 4: Run tests**

```bash
go test ./pkg/safety/... -v
```

Expected: PASS

**Step 5: Commit**

```bash
git add -A
git commit -m "feat: add experiment mutual exclusion lock"
```

---

## Task 7: Evaluator Engine

**Files:**
- Create: `pkg/evaluator/engine.go`
- Create: `pkg/evaluator/engine_test.go`

**Step 1: Write tests**

Create `pkg/evaluator/engine_test.go`:
```go
package evaluator

import (
	"testing"
	"time"

	v1alpha1 "github.com/opendatahub-io/odh-platform-chaos/api/v1alpha1"
	"github.com/stretchr/testify/assert"
)

func TestEvaluateResilient(t *testing.T) {
	e := New(10)

	result := e.Evaluate(
		&v1alpha1.CheckResult{Passed: true, ChecksRun: 3, ChecksPassed: 3},
		&v1alpha1.CheckResult{Passed: true, ChecksRun: 3, ChecksPassed: 3},
		true, // all reconciled
		2,    // reconcile cycles
		12*time.Second, // recovery time
		v1alpha1.HypothesisSpec{RecoveryTimeout: v1alpha1.Duration{Duration: 60 * time.Second}},
	)

	assert.Equal(t, v1alpha1.Resilient, result.Verdict)
	assert.Equal(t, 12*time.Second, result.RecoveryTime)
	assert.Equal(t, 2, result.ReconcileCycles)
	assert.NotEmpty(t, result.Confidence)
}

func TestEvaluateFailed(t *testing.T) {
	e := New(10)

	result := e.Evaluate(
		&v1alpha1.CheckResult{Passed: true, ChecksRun: 3, ChecksPassed: 3},
		&v1alpha1.CheckResult{Passed: false, ChecksRun: 3, ChecksPassed: 1},
		false, 0, 120*time.Second,
		v1alpha1.HypothesisSpec{RecoveryTimeout: v1alpha1.Duration{Duration: 60 * time.Second}},
	)

	assert.Equal(t, v1alpha1.Failed, result.Verdict)
}

func TestEvaluateDegraded_SlowRecovery(t *testing.T) {
	e := New(10)

	result := e.Evaluate(
		&v1alpha1.CheckResult{Passed: true, ChecksRun: 3, ChecksPassed: 3},
		&v1alpha1.CheckResult{Passed: true, ChecksRun: 3, ChecksPassed: 3},
		true, 3, 90*time.Second,
		v1alpha1.HypothesisSpec{RecoveryTimeout: v1alpha1.Duration{Duration: 60 * time.Second}},
	)

	assert.Equal(t, v1alpha1.Degraded, result.Verdict)
}

func TestEvaluateDegraded_ExcessiveCycles(t *testing.T) {
	e := New(5) // max 5 cycles

	result := e.Evaluate(
		&v1alpha1.CheckResult{Passed: true, ChecksRun: 3, ChecksPassed: 3},
		&v1alpha1.CheckResult{Passed: true, ChecksRun: 3, ChecksPassed: 3},
		true, 15, 30*time.Second, // 15 cycles > max 5
		v1alpha1.HypothesisSpec{RecoveryTimeout: v1alpha1.Duration{Duration: 60 * time.Second}},
	)

	assert.Equal(t, v1alpha1.Degraded, result.Verdict)
	assert.NotEmpty(t, result.Deviations)
}

func TestEvaluateInconclusive(t *testing.T) {
	e := New(10)

	result := e.Evaluate(
		&v1alpha1.CheckResult{Passed: false, ChecksRun: 3, ChecksPassed: 1}, // pre-check failed
		&v1alpha1.CheckResult{Passed: false, ChecksRun: 3, ChecksPassed: 1},
		false, 0, 0,
		v1alpha1.HypothesisSpec{RecoveryTimeout: v1alpha1.Duration{Duration: 60 * time.Second}},
	)

	assert.Equal(t, v1alpha1.Inconclusive, result.Verdict)
}
```

**Step 2: Run tests to verify they fail**

```bash
go test ./pkg/evaluator/... -v
```

**Step 3: Implement evaluator**

Create `pkg/evaluator/engine.go`:
```go
package evaluator

import (
	"fmt"
	"time"

	v1alpha1 "github.com/opendatahub-io/odh-platform-chaos/api/v1alpha1"
)

type Evaluator struct {
	maxReconcileCycles int
}

type EvaluationResult struct {
	Verdict         v1alpha1.Verdict `json:"verdict"`
	Confidence      string           `json:"confidence"`
	RecoveryTime    time.Duration    `json:"recoveryTime"`
	ReconcileCycles int              `json:"reconcileCycles"`
	Deviations      []Deviation      `json:"deviations,omitempty"`
}

type Deviation struct {
	Type   string `json:"type"`
	Detail string `json:"detail"`
}

func New(maxReconcileCycles int) *Evaluator {
	return &Evaluator{maxReconcileCycles: maxReconcileCycles}
}

func (e *Evaluator) Evaluate(
	preCheck *v1alpha1.CheckResult,
	postCheck *v1alpha1.CheckResult,
	allReconciled bool,
	reconcileCycles int,
	recoveryTime time.Duration,
	hypothesis v1alpha1.HypothesisSpec,
) *EvaluationResult {
	result := &EvaluationResult{
		RecoveryTime:    recoveryTime,
		ReconcileCycles: reconcileCycles,
	}

	// 1. Baseline not established
	if !preCheck.Passed {
		result.Verdict = v1alpha1.Inconclusive
		result.Confidence = fmt.Sprintf(
			"pre-check failed: %d/%d checks passed",
			preCheck.ChecksPassed, preCheck.ChecksRun)
		return result
	}

	// 2. Did it recover?
	if postCheck.Passed && allReconciled {
		result.Verdict = v1alpha1.Resilient
	} else if postCheck.Passed && !allReconciled {
		result.Verdict = v1alpha1.Degraded
		result.Deviations = append(result.Deviations, Deviation{
			Type:   "partial_reconciliation",
			Detail: "steady state checks passed but not all resources reconciled",
		})
	} else {
		result.Verdict = v1alpha1.Failed
	}

	// 3. Recovery time
	if recoveryTime > hypothesis.RecoveryTimeout.Duration {
		if result.Verdict == v1alpha1.Resilient {
			result.Verdict = v1alpha1.Degraded
		}
		result.Deviations = append(result.Deviations, Deviation{
			Type: "slow_recovery",
			Detail: fmt.Sprintf("recovered in %s, expected within %s",
				recoveryTime, hypothesis.RecoveryTimeout.Duration),
		})
	}

	// 4. Excessive reconcile cycles
	if e.maxReconcileCycles > 0 && reconcileCycles > e.maxReconcileCycles {
		if result.Verdict == v1alpha1.Resilient {
			result.Verdict = v1alpha1.Degraded
		}
		result.Deviations = append(result.Deviations, Deviation{
			Type: "excessive_reconciliation",
			Detail: fmt.Sprintf("%d cycles (max %d)",
				reconcileCycles, e.maxReconcileCycles),
		})
	}

	// 5. Confidence qualifier
	result.Confidence = fmt.Sprintf(
		"%d/%d steady-state checks passed, %s recovery, %d reconcile cycles",
		postCheck.ChecksPassed, postCheck.ChecksRun,
		recoveryTime, reconcileCycles)

	return result
}
```

**Step 4: Run tests**

```bash
go test ./pkg/evaluator/... -v
```

Expected: PASS

**Step 5: Commit**

```bash
git add -A
git commit -m "feat: add evaluator engine with verdict classification"
```

---

## Task 8: JSON Reporter

**Files:**
- Create: `pkg/reporter/json.go`
- Create: `pkg/reporter/json_test.go`

**Step 1: Write tests**

Create `pkg/reporter/json_test.go`:
```go
package reporter

import (
	"bytes"
	"encoding/json"
	"testing"
	"time"

	v1alpha1 "github.com/opendatahub-io/odh-platform-chaos/api/v1alpha1"
	"github.com/opendatahub-io/odh-platform-chaos/pkg/evaluator"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestJSONReporterWrite(t *testing.T) {
	var buf bytes.Buffer
	r := NewJSONReporter(&buf)

	report := ExperimentReport{
		Experiment: "dashboard-pod-kill",
		Timestamp:  time.Date(2026, 2, 26, 10, 0, 0, 0, time.UTC),
		Target: TargetReport{
			Operator:  "opendatahub-operator",
			Component: "dashboard",
			Resource:  "Deployment/odh-dashboard",
		},
		Injection: InjectionReport{
			Type:      string(v1alpha1.PodKill),
			Timestamp: time.Date(2026, 2, 26, 10, 0, 5, 0, time.UTC),
		},
		Evaluation: evaluator.EvaluationResult{
			Verdict:      v1alpha1.Resilient,
			Confidence:   "3/3 checks passed, 12s recovery, 2 cycles",
			RecoveryTime: 12 * time.Second,
		},
	}

	err := r.Write(report)
	require.NoError(t, err)

	var parsed map[string]interface{}
	err = json.Unmarshal(buf.Bytes(), &parsed)
	require.NoError(t, err)

	assert.Equal(t, "dashboard-pod-kill", parsed["experiment"])
	assert.Equal(t, "Resilient", parsed["evaluation"].(map[string]interface{})["verdict"])
}

func TestJSONReporterToFile(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/report.json"

	r, err := NewJSONFileReporter(path)
	require.NoError(t, err)

	report := ExperimentReport{
		Experiment: "test",
		Timestamp:  time.Now(),
		Evaluation: evaluator.EvaluationResult{Verdict: v1alpha1.Failed},
	}

	err = r.Write(report)
	require.NoError(t, err)
}
```

**Step 2: Run tests to verify they fail**

```bash
go test ./pkg/reporter/... -v
```

**Step 3: Implement JSON reporter**

Create `pkg/reporter/json.go`:
```go
package reporter

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/opendatahub-io/odh-platform-chaos/pkg/evaluator"
)

type ExperimentReport struct {
	Experiment string                    `json:"experiment"`
	Timestamp  time.Time                 `json:"timestamp"`
	Target     TargetReport              `json:"target"`
	Injection  InjectionReport           `json:"injection"`
	Evaluation evaluator.EvaluationResult `json:"evaluation"`
	SteadyState SteadyStateReport        `json:"steadyState,omitempty"`
}

type TargetReport struct {
	Operator  string `json:"operator"`
	Component string `json:"component"`
	Resource  string `json:"resource,omitempty"`
}

type InjectionReport struct {
	Type      string            `json:"type"`
	Targets   []string          `json:"targets,omitempty"`
	Timestamp time.Time         `json:"timestamp"`
	Details   map[string]string `json:"details,omitempty"`
}

type SteadyStateReport struct {
	Pre  interface{} `json:"pre,omitempty"`
	Post interface{} `json:"post,omitempty"`
}

type JSONReporter struct {
	writer io.Writer
}

func NewJSONReporter(w io.Writer) *JSONReporter {
	return &JSONReporter{writer: w}
}

func NewJSONFileReporter(path string) (*JSONReporter, error) {
	f, err := os.Create(path)
	if err != nil {
		return nil, fmt.Errorf("creating report file: %w", err)
	}
	return &JSONReporter{writer: f}, nil
}

func (r *JSONReporter) Write(report ExperimentReport) error {
	encoder := json.NewEncoder(r.writer)
	encoder.SetIndent("", "  ")
	return encoder.Encode(report)
}
```

**Step 4: Run tests**

```bash
go test ./pkg/reporter/... -v
```

Expected: PASS

**Step 5: Commit**

```bash
git add -A
git commit -m "feat: add JSON reporter for experiment results"
```

---

## Task 9: JUnit Reporter (CI Integration)

**Files:**
- Create: `pkg/reporter/junit.go`
- Create: `pkg/reporter/junit_test.go`

**Step 1: Write tests**

Create `pkg/reporter/junit_test.go`:
```go
package reporter

import (
	"bytes"
	"strings"
	"testing"
	"time"

	v1alpha1 "github.com/opendatahub-io/odh-platform-chaos/api/v1alpha1"
	"github.com/opendatahub-io/odh-platform-chaos/pkg/evaluator"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestJUnitReporter(t *testing.T) {
	var buf bytes.Buffer
	r := NewJUnitReporter(&buf)

	reports := []ExperimentReport{
		{
			Experiment: "test-resilient",
			Timestamp:  time.Now(),
			Evaluation: evaluator.EvaluationResult{
				Verdict:      v1alpha1.Resilient,
				RecoveryTime: 12 * time.Second,
			},
		},
		{
			Experiment: "test-failed",
			Timestamp:  time.Now(),
			Evaluation: evaluator.EvaluationResult{
				Verdict: v1alpha1.Failed,
			},
		},
	}

	err := r.WriteSuite("chaos-tests", reports)
	require.NoError(t, err)

	output := buf.String()
	assert.True(t, strings.Contains(output, "<testsuite"))
	assert.True(t, strings.Contains(output, "test-resilient"))
	assert.True(t, strings.Contains(output, "test-failed"))
	assert.True(t, strings.Contains(output, "<failure"))
}
```

**Step 2: Run tests to verify they fail**

```bash
go test ./pkg/reporter/... -v -run TestJUnit
```

**Step 3: Implement JUnit reporter**

Create `pkg/reporter/junit.go`:
```go
package reporter

import (
	"encoding/xml"
	"fmt"
	"io"

	v1alpha1 "github.com/opendatahub-io/odh-platform-chaos/api/v1alpha1"
)

type JUnitReporter struct {
	writer io.Writer
}

type junitTestSuites struct {
	XMLName xml.Name         `xml:"testsuites"`
	Suites  []junitTestSuite `xml:"testsuite"`
}

type junitTestSuite struct {
	Name     string          `xml:"name,attr"`
	Tests    int             `xml:"tests,attr"`
	Failures int             `xml:"failures,attr"`
	Time     string          `xml:"time,attr"`
	Cases    []junitTestCase `xml:"testcase"`
}

type junitTestCase struct {
	Name      string        `xml:"name,attr"`
	ClassName string        `xml:"classname,attr"`
	Time      string        `xml:"time,attr"`
	Failure   *junitFailure `xml:"failure,omitempty"`
	Skipped   *junitSkipped `xml:"skipped,omitempty"`
}

type junitFailure struct {
	Message string `xml:"message,attr"`
	Type    string `xml:"type,attr"`
	Body    string `xml:",chardata"`
}

type junitSkipped struct {
	Message string `xml:"message,attr"`
}

func NewJUnitReporter(w io.Writer) *JUnitReporter {
	return &JUnitReporter{writer: w}
}

func (r *JUnitReporter) WriteSuite(name string, reports []ExperimentReport) error {
	suite := junitTestSuite{
		Name:  name,
		Tests: len(reports),
	}

	for _, report := range reports {
		tc := junitTestCase{
			Name:      report.Experiment,
			ClassName: fmt.Sprintf("chaos.%s", report.Target.Component),
			Time:      fmt.Sprintf("%.3f", report.Evaluation.RecoveryTime.Seconds()),
		}

		switch report.Evaluation.Verdict {
		case v1alpha1.Failed:
			suite.Failures++
			tc.Failure = &junitFailure{
				Message: "Chaos experiment failed",
				Type:    "FAILED",
				Body:    report.Evaluation.Confidence,
			}
		case v1alpha1.Degraded:
			suite.Failures++
			tc.Failure = &junitFailure{
				Message: "System degraded under chaos",
				Type:    "DEGRADED",
				Body:    report.Evaluation.Confidence,
			}
		case v1alpha1.Inconclusive:
			tc.Skipped = &junitSkipped{
				Message: "Could not establish baseline: " + report.Evaluation.Confidence,
			}
		}

		suite.Cases = append(suite.Cases, tc)
	}

	suites := junitTestSuites{Suites: []junitTestSuite{suite}}

	output, err := xml.MarshalIndent(suites, "", "  ")
	if err != nil {
		return err
	}

	_, err = r.writer.Write([]byte(xml.Header))
	if err != nil {
		return err
	}
	_, err = r.writer.Write(output)
	return err
}
```

**Step 4: Run tests**

```bash
go test ./pkg/reporter/... -v
```

Expected: PASS

**Step 5: Commit**

```bash
git add -A
git commit -m "feat: add JUnit XML reporter for CI integration"
```

---

## Task 10: Injection Engine Interface and PodKill Injector

**Files:**
- Create: `pkg/injection/engine.go`
- Create: `pkg/injection/engine_test.go`
- Create: `pkg/injection/podkill.go`
- Create: `pkg/injection/podkill_test.go`

**Step 1: Write tests for injection engine and PodKill**

Create `pkg/injection/engine_test.go`:
```go
package injection

import (
	"testing"

	v1alpha1 "github.com/opendatahub-io/odh-platform-chaos/api/v1alpha1"
	"github.com/stretchr/testify/assert"
)

func TestRegistryLookup(t *testing.T) {
	r := NewRegistry()
	r.Register(v1alpha1.PodKill, &PodKillInjector{})

	injector, err := r.Get(v1alpha1.PodKill)
	assert.NoError(t, err)
	assert.NotNil(t, injector)

	_, err = r.Get("UnknownType")
	assert.Error(t, err)
}

func TestRegistryListTypes(t *testing.T) {
	r := NewRegistry()
	r.Register(v1alpha1.PodKill, &PodKillInjector{})

	types := r.ListTypes()
	assert.Contains(t, types, v1alpha1.PodKill)
}
```

Create `pkg/injection/podkill_test.go`:
```go
package injection

import (
	"testing"

	v1alpha1 "github.com/opendatahub-io/odh-platform-chaos/api/v1alpha1"
	"github.com/stretchr/testify/assert"
)

func TestPodKillValidate(t *testing.T) {
	injector := &PodKillInjector{}

	// Valid spec
	spec := v1alpha1.InjectionSpec{
		Type:  v1alpha1.PodKill,
		Count: 1,
		Parameters: map[string]string{
			"labelSelector": "app=dashboard",
		},
	}
	blast := v1alpha1.BlastRadiusSpec{
		MaxPodsAffected:   1,
		AllowedNamespaces: []string{"test"},
	}

	err := injector.Validate(spec, blast)
	assert.NoError(t, err)

	// Invalid: count exceeds blast radius
	spec.Count = 5
	err = injector.Validate(spec, blast)
	assert.Error(t, err)
}

func TestPodKillValidateMissingSelector(t *testing.T) {
	injector := &PodKillInjector{}

	spec := v1alpha1.InjectionSpec{
		Type:  v1alpha1.PodKill,
		Count: 1,
	}
	blast := v1alpha1.BlastRadiusSpec{
		MaxPodsAffected:   1,
		AllowedNamespaces: []string{"test"},
	}

	err := injector.Validate(spec, blast)
	assert.Error(t, err)
}
```

**Step 2: Run tests to verify they fail**

```bash
go test ./pkg/injection/... -v
```

**Step 3: Implement engine and PodKill injector**

Create `pkg/injection/engine.go`:
```go
package injection

import (
	"context"
	"fmt"
	"time"

	v1alpha1 "github.com/opendatahub-io/odh-platform-chaos/api/v1alpha1"
)

type CleanupFunc func(ctx context.Context) error

type Injector interface {
	Validate(spec v1alpha1.InjectionSpec, blast v1alpha1.BlastRadiusSpec) error
	Inject(ctx context.Context, spec v1alpha1.InjectionSpec, namespace string) (CleanupFunc, []v1alpha1.InjectionEvent, error)
}

type Registry struct {
	injectors map[v1alpha1.InjectionType]Injector
}

func NewRegistry() *Registry {
	return &Registry{
		injectors: make(map[v1alpha1.InjectionType]Injector),
	}
}

func (r *Registry) Register(t v1alpha1.InjectionType, i Injector) {
	r.injectors[t] = i
}

func (r *Registry) Get(t v1alpha1.InjectionType) (Injector, error) {
	i, ok := r.injectors[t]
	if !ok {
		return nil, fmt.Errorf("unknown injection type: %s", t)
	}
	return i, nil
}

func (r *Registry) ListTypes() []v1alpha1.InjectionType {
	types := make([]v1alpha1.InjectionType, 0, len(r.injectors))
	for t := range r.injectors {
		types = append(types, t)
	}
	return types
}

func NewEvent(t v1alpha1.InjectionType, target string, action string, details map[string]string) v1alpha1.InjectionEvent {
	return v1alpha1.InjectionEvent{
		Timestamp: time.Now(),
		Type:      t,
		Target:    target,
		Action:    action,
		Details:   details,
	}
}
```

Create `pkg/injection/podkill.go`:
```go
package injection

import (
	"context"
	"fmt"

	v1alpha1 "github.com/opendatahub-io/odh-platform-chaos/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type PodKillInjector struct {
	client client.Client
}

func NewPodKillInjector(c client.Client) *PodKillInjector {
	return &PodKillInjector{client: c}
}

func (p *PodKillInjector) Validate(spec v1alpha1.InjectionSpec, blast v1alpha1.BlastRadiusSpec) error {
	if spec.Count > blast.MaxPodsAffected {
		return fmt.Errorf("pod kill count %d exceeds blast radius %d", spec.Count, blast.MaxPodsAffected)
	}

	if _, ok := spec.Parameters["labelSelector"]; !ok {
		return fmt.Errorf("PodKill requires 'labelSelector' parameter")
	}

	return nil
}

func (p *PodKillInjector) Inject(ctx context.Context, spec v1alpha1.InjectionSpec, namespace string) (CleanupFunc, []v1alpha1.InjectionEvent, error) {
	selector, err := labels.Parse(spec.Parameters["labelSelector"])
	if err != nil {
		return nil, nil, fmt.Errorf("parsing label selector: %w", err)
	}

	podList := &corev1.PodList{}
	if err := p.client.List(ctx, podList,
		client.InNamespace(namespace),
		client.MatchingLabelsSelector{Selector: selector},
	); err != nil {
		return nil, nil, fmt.Errorf("listing pods: %w", err)
	}

	if len(podList.Items) == 0 {
		return nil, nil, fmt.Errorf("no pods found matching selector %s in namespace %s", selector.String(), namespace)
	}

	killCount := spec.Count
	if killCount <= 0 {
		killCount = 1
	}
	if killCount > len(podList.Items) {
		killCount = len(podList.Items)
	}

	var events []v1alpha1.InjectionEvent
	gracePeriod := int64(0)

	for i := 0; i < killCount; i++ {
		pod := podList.Items[i]
		if err := p.client.Delete(ctx, &pod, &client.DeleteOptions{
			GracePeriodSeconds: &gracePeriod,
			Preconditions:      &metav1.Preconditions{UID: &pod.UID},
		}); err != nil {
			return nil, events, fmt.Errorf("killing pod %s: %w", pod.Name, err)
		}

		events = append(events, NewEvent(
			v1alpha1.PodKill,
			pod.Name,
			"deleted",
			map[string]string{
				"namespace": namespace,
				"node":      pod.Spec.NodeName,
			},
		))
	}

	// No cleanup needed -- Deployment controller will recreate pods
	cleanup := func(ctx context.Context) error { return nil }

	return cleanup, events, nil
}
```

**Step 4: Run tests**

```bash
go get k8s.io/api@latest
go get k8s.io/apimachinery@latest
go get sigs.k8s.io/controller-runtime@latest
go mod tidy
go test ./pkg/injection/... -v
```

Expected: PASS

**Step 5: Commit**

```bash
git add -A
git commit -m "feat: add injection engine with PodKill injector"
```

---

## Task 11: CRD Mutation and Config Drift Injectors

**Files:**
- Create: `pkg/injection/crdmutation.go`
- Create: `pkg/injection/crdmutation_test.go`
- Create: `pkg/injection/configdrift.go`
- Create: `pkg/injection/configdrift_test.go`

These are the operator-semantic injectors that no other chaos tool has.
Implementation follows the same pattern as PodKill: Validate + Inject + CleanupFunc.

The CRDMutationInjector patches a managed CR's spec field and verifies
the operator reconciles it back. The ConfigDriftInjector modifies a managed
ConfigMap/Secret and checks if the operator detects and corrects the drift.

Both injectors save the original state and provide a cleanup function that
restores it if the operator doesn't reconcile within the experiment timeout.

**Step 1: Write tests** (unit tests with mock client)

**Step 2: Implement injectors**

**Step 3: Run tests**

**Step 4: Commit**

```bash
git add -A
git commit -m "feat: add CRDMutation and ConfigDrift injectors"
```

---

## Task 12: Network Partition Injector

**Files:**
- Create: `pkg/injection/network.go`
- Create: `pkg/injection/network_test.go`

Creates a NetworkPolicy that blocks ingress/egress for targeted pods.
Cleanup deletes the NetworkPolicy.

**Step 1-4: Same TDD pattern as previous injectors**

**Step 5: Commit**

```bash
git add -A
git commit -m "feat: add NetworkPartition injector via NetworkPolicy"
```

---

## Task 13: Orchestrator Lifecycle

**Files:**
- Create: `pkg/orchestrator/lifecycle.go`
- Create: `pkg/orchestrator/lifecycle_test.go`

The orchestrator wires together all engines and manages the experiment
lifecycle state machine.

**Step 1: Write tests for lifecycle transitions**

Test cases:
- Happy path: PENDING -> STEADY_STATE_PRE -> INJECT -> OBSERVE -> STEADY_STATE_POST -> EVALUATE -> COMPLETE
- Abort on failed pre-check
- Abort on blast radius violation
- Cleanup called even on errors
- Dry run mode skips injection

**Step 2: Implement orchestrator**

```go
type Orchestrator struct {
    registry  *injection.Registry
    observer  observer.Observer
    evaluator *evaluator.Evaluator
    reporter  *reporter.JSONReporter
    lock      safety.ExperimentLock
    knowledge *model.OperatorKnowledge
}

func (o *Orchestrator) Run(ctx context.Context, exp v1alpha1.ChaosExperiment) (*ExperimentResult, error) {
    // Full lifecycle implementation
}
```

**Step 3-5: Run tests, commit**

```bash
git add -A
git commit -m "feat: add experiment orchestrator with lifecycle state machine"
```

---

## Task 14: Observer Engine - Kubernetes State Checker

**Files:**
- Create: `pkg/observer/engine.go`
- Create: `pkg/observer/kubernetes.go`
- Create: `pkg/observer/kubernetes_test.go`

Checks K8s resource existence, conditions, and ready replicas against
steady-state definitions.

**Step 1-5: TDD pattern, commit**

```bash
git add -A
git commit -m "feat: add Kubernetes state observer for steady-state checks"
```

---

## Task 15: Observer Engine - Reconciliation Checker

**Files:**
- Create: `pkg/observer/reconciliation.go`
- Create: `pkg/observer/reconciliation_test.go`

The key innovation: verifies that the operator reconciled ALL managed
resources with correct metadata, spec, and conditions.

**Step 1-5: TDD pattern, commit**

```bash
git add -A
git commit -m "feat: add ReconciliationChecker for operator convergence verification"
```

---

## Task 16: CLI run Command

**Files:**
- Create: `internal/cli/run.go`

Wires the orchestrator to the CLI. Loads experiment YAML, creates K8s
client from kubeconfig, runs experiment, outputs report.

**Step 1: Implement run command**

```go
func newRunCommand() *cobra.Command {
    cmd := &cobra.Command{
        Use:   "run [experiment.yaml]",
        Short: "Run a chaos experiment",
        Args:  cobra.ExactArgs(1),
        RunE:  runExperiment,
    }
    cmd.Flags().String("knowledge", "", "path to knowledge YAML")
    cmd.Flags().String("report-dir", "", "directory for report output")
    cmd.Flags().Bool("dry-run", false, "validate without injecting")
    return cmd
}
```

**Step 2: Wire into root command**

**Step 3: Build and test with dry-run**

```bash
make build
./bin/odh-chaos run testdata/experiments/valid-experiment.yaml --dry-run
```

**Step 4: Commit**

```bash
git add -A
git commit -m "feat: add run CLI command with dry-run support"
```

---

## Task 17: CLI clean Command (Emergency Stop)

**Files:**
- Create: `internal/cli/clean.go`

Scans ConfigMaps with chaos labels and deletes them. Works even without
knowledge of running experiments.

**Step 1-3: Implement and test**

**Step 4: Commit**

```bash
git add -A
git commit -m "feat: add clean CLI command for emergency fault removal"
```

---

## Task 18: CLI init Command (Scaffold Experiments)

**Files:**
- Create: `internal/cli/init.go`

Generates a skeleton experiment YAML for a given component and injection type.

```bash
./bin/odh-chaos init --component dashboard --type PodKill > experiments/new.yaml
```

**Step 1-3: Implement and test**

**Step 4: Commit**

```bash
git add -A
git commit -m "feat: add init CLI command for experiment scaffolding"
```

---

## Task 19: Static Analyzer - AST Patterns

**Files:**
- Create: `pkg/analyzer/analyzer.go`
- Create: `pkg/analyzer/analyzer_test.go`
- Create: `pkg/analyzer/patterns.go`
- Create: `pkg/analyzer/patterns_test.go`
- Create: `testdata/go-source/sample_controller.go`

**Step 1: Create sample Go file for testing**

Create `testdata/go-source/sample_controller.go`:
```go
package sample

import (
	"context"
	"database/sql"
	"net/http"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Reconciler struct {
	client client.Client
	db     *sql.DB
}

func (r *Reconciler) Reconcile(ctx context.Context) error {
	obj := &unstructured.Unstructured{}
	_ = r.client.Get(ctx, types.NamespacedName{}, obj) // ignored error

	if err := r.client.Create(ctx, obj); err != nil {
		return err
	}

	go func() { // goroutine launch
		http.Get("http://example.com") // network call
	}()

	r.db.Query("SELECT 1") // db call, error ignored

	return nil
}
```

**Step 2: Write tests for pattern detection**

Test that the analyzer finds:
- Ignored error on `client.Get`
- K8s API calls (`Get`, `Create`)
- Goroutine launch
- Network call (`http.Get`)
- Database call with ignored error

**Step 3: Implement AST analyzer**

Uses `go/ast` and `go/parser` to detect:
1. K8s API calls (matches `client.Get/Create/Update/Delete/Patch`)
2. Ignored errors (assignments to `_`)
3. Goroutine launches (`go func()`)
4. Network calls (`http.Get`, `grpc.Dial`, `sql.Open`)
5. Context-accepting functions

**Step 4-5: Run tests, commit**

```bash
git add -A
git commit -m "feat: add static analyzer for fault point candidate detection"
```

---

## Task 20: CLI analyze Command

**Files:**
- Create: `internal/cli/analyze.go`

Wires the static analyzer to the CLI. Scans a Go module directory
and outputs fault point candidates.

```bash
./bin/odh-chaos analyze /path/to/operator
./bin/odh-chaos analyze /path/to/operator --generate-experiments
```

**Step 1-3: Implement and test**

**Step 4: Commit**

```bash
git add -A
git commit -m "feat: add analyze CLI command for static code analysis"
```

---

## Task 21: CLI suite and report Commands

**Files:**
- Create: `internal/cli/suite.go`
- Create: `internal/cli/report.go`

Suite runs multiple experiments from a directory. Report generates
summary reports from stored results.

**Step 1-3: Implement**

**Step 4: Commit**

```bash
git add -A
git commit -m "feat: add suite runner and report generator CLI commands"
```

---

## Task 22: Integration Test with envtest

**Files:**
- Create: `tests/integration/experiment_test.go`

Uses controller-runtime's envtest to spin up a local K8s API server
and run a PodKill experiment end-to-end.

**Step 1: Write integration test**

```go
func TestPodKillExperimentE2E(t *testing.T) {
    // 1. Start envtest
    // 2. Create a Deployment
    // 3. Load experiment YAML
    // 4. Run orchestrator
    // 5. Verify verdict is Resilient
    // 6. Verify pod was recreated
}
```

**Step 2-4: Implement, run, commit**

```bash
git add -A
git commit -m "test: add integration test with envtest for PodKill experiment"
```

---

## Task 23: README and Documentation

**Files:**
- Create: `README.md`

Quick start guide, CLI reference, example experiments.

**Step 1: Write README**

**Step 2: Commit**

```bash
git add -A
git commit -m "docs: add README with quick start guide and CLI reference"
```

---

## Summary

| Task | Component | Complexity |
|---|---|---|
| 1 | Project scaffold + CLI | Low |
| 2 | Experiment types (CRD-ready) | Medium |
| 3 | Knowledge model | Medium |
| 4 | Experiment loader + validator | Low |
| 5 | Blast radius validation | Low |
| 6 | Experiment mutual exclusion | Low |
| 7 | Evaluator engine | Medium |
| 8 | JSON reporter | Low |
| 9 | JUnit reporter | Low |
| 10 | Injection engine + PodKill | Medium |
| 11 | CRDMutation + ConfigDrift | Medium |
| 12 | Network partition | Medium |
| 13 | Orchestrator lifecycle | High |
| 14 | K8s state observer | Medium |
| 15 | Reconciliation checker | High |
| 16 | CLI run command | Medium |
| 17 | CLI clean command | Low |
| 18 | CLI init command | Low |
| 19 | Static analyzer | High |
| 20 | CLI analyze command | Low |
| 21 | Suite + report commands | Medium |
| 22 | Integration test | High |
| 23 | README | Low |

**Execution order**: Tasks 1-9 have no dependencies on K8s client (pure Go).
Tasks 10-15 require K8s types. Tasks 16-21 wire everything to CLI.
Tasks 22-23 are verification and documentation.
