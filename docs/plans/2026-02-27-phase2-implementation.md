# Phase 2: Controller-Runtime Middleware Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add controller-runtime middleware (chaos.WrapReconciler), ChaosClient wrapper, advanced injectors, safety mechanisms (TTL, distributed locking), and fix critical Phase 1 issues identified by 5-architect review.

**Architecture:** Phase 2 introduces a `pkg/sdk` package containing the ChaosClient hybrid wrapper and WrapReconciler middleware. Faults are activated via ConfigMap or HTTP endpoint. Build-tag guarded for heavyweight faults, nil-check passthrough for K8s client faults. Three new infrastructure injectors (WebhookDisrupt, RBACRevoke, FinalizerBlock) register via the existing Registry. Safety mechanisms add TTL enforcement and Kubernetes Lease-based distributed locking.

**Tech Stack:** Go 1.25, controller-runtime v0.23, client-go, cobra, testify, envtest, sigs.k8s.io/yaml

---

## Part A: Critical Phase 1 Fixes (from 5-Architect Review)

### Task 1: Fix verbose logging bypass

**Files:**
- Modify: `pkg/orchestrator/lifecycle.go:263-267`
- Test: `pkg/orchestrator/lifecycle_test.go`

**Step 1: Write the failing test**

In `pkg/orchestrator/lifecycle_test.go`, add:

```go
func TestOrchestratorVerboseOff(t *testing.T) {
	obs := &mockObserver{result: &v1alpha1.CheckResult{Passed: true, ChecksRun: 1, ChecksPassed: 1, Timestamp: time.Now()}}
	inj := &mockInjector{}
	registry := injection.NewRegistry()
	registry.Register(v1alpha1.PodKill, inj)

	orch := New(OrchestratorConfig{
		Registry:  registry,
		Observer:  obs,
		Evaluator: evaluator.New(10),
		Lock:      safety.NewLocalExperimentLock(),
		Verbose:   false, // verbose OFF
	})

	buf := &bytes.Buffer{}
	orch.output = buf

	_, err := orch.Run(context.Background(), newTestExperiment())
	require.NoError(t, err)

	assert.Empty(t, buf.String(), "verbose=false should produce no output")
}
```

**Step 2: Run test to verify it fails**

Run: `cd /Users/ugogiordano/workdir/rhoai/opendatahub-io/odh-platform-chaos && go test ./pkg/orchestrator/ -run TestOrchestratorVerboseOff -v`
Expected: FAIL (buf.String() is not empty because of `|| true`)

**Step 3: Fix the logging method**

In `pkg/orchestrator/lifecycle.go`, change line 264 from:

```go
if o.verbose || true { // Always log for now
```

to:

```go
if o.verbose {
```

**Step 4: Run tests to verify pass**

Run: `cd /Users/ugogiordano/workdir/rhoai/opendatahub-io/odh-platform-chaos && go test ./pkg/orchestrator/ -v`
Expected: ALL PASS

**Step 5: Commit**

```bash
git add pkg/orchestrator/lifecycle.go pkg/orchestrator/lifecycle_test.go
git commit -m "fix: remove debug bypass on verbose logging flag"
```

---

### Task 2: Fix dry-run verdict from Resilient to Inconclusive

**Files:**
- Modify: `pkg/orchestrator/lifecycle.go:128-133`
- Modify: `tests/integration/experiment_test.go:180`
- Test: `pkg/orchestrator/lifecycle_test.go`

**Step 1: Write the failing test**

In `pkg/orchestrator/lifecycle_test.go`, add:

```go
func TestOrchestratorDryRunVerdict(t *testing.T) {
	obs := &mockObserver{}
	inj := &mockInjector{}
	orch := newTestOrchestrator(obs, inj)

	exp := newTestExperiment()
	exp.Spec.BlastRadius.DryRun = true

	result, err := orch.Run(context.Background(), exp)
	require.NoError(t, err)
	assert.Equal(t, v1alpha1.PhaseComplete, result.Phase)
	// Dry run should NOT claim Resilient -- no injection occurred
	assert.Equal(t, v1alpha1.Inconclusive, result.Verdict)
}
```

**Step 2: Run test to verify it fails**

Run: `cd /Users/ugogiordano/workdir/rhoai/opendatahub-io/odh-platform-chaos && go test ./pkg/orchestrator/ -run TestOrchestratorDryRunVerdict -v`
Expected: FAIL (currently returns Resilient)

**Step 3: Fix dry-run verdict in lifecycle.go**

Change lines 128-133:

```go
if exp.Spec.BlastRadius.DryRun {
	o.log("DRY RUN: Would inject %s into %s/%s", exp.Spec.Injection.Type, exp.Spec.Target.Operator, exp.Spec.Target.Component)
	result.Phase = v1alpha1.PhaseComplete
	result.Verdict = v1alpha1.Inconclusive
	return result, nil
}
```

**Step 4: Fix the existing dry-run test assertion**

In `pkg/orchestrator/lifecycle_test.go`, update `TestOrchestratorDryRun`:

```go
func TestOrchestratorDryRun(t *testing.T) {
	obs := &mockObserver{}
	inj := &mockInjector{}
	orch := newTestOrchestrator(obs, inj)

	exp := newTestExperiment()
	exp.Spec.BlastRadius.DryRun = true

	result, err := orch.Run(context.Background(), exp)
	require.NoError(t, err)
	assert.Equal(t, v1alpha1.PhaseComplete, result.Phase)
	assert.Equal(t, v1alpha1.Inconclusive, result.Verdict)
	assert.False(t, inj.cleanupCalled)
}
```

And fix integration test at `tests/integration/experiment_test.go` line 180:

```go
assert.Equal(t, v1alpha1.Inconclusive, result.Verdict)
```

**Step 5: Run all tests**

Run: `cd /Users/ugogiordano/workdir/rhoai/opendatahub-io/odh-platform-chaos && go test ./... -short -count=1`
Expected: ALL PASS

**Step 6: Commit**

```bash
git add pkg/orchestrator/lifecycle.go pkg/orchestrator/lifecycle_test.go tests/integration/experiment_test.go
git commit -m "fix: dry-run returns Inconclusive instead of Resilient"
```

---

### Task 3: Fix ForbiddenResources validation bug

**Files:**
- Modify: `pkg/safety/blastradius.go:9-36`
- Test: `pkg/safety/blastradius_test.go`

**Step 1: Write the failing test**

In `pkg/safety/blastradius_test.go`, add test cases for ForbiddenResources:

```go
func TestValidateBlastRadiusForbiddenResources(t *testing.T) {
	tests := []struct {
		name       string
		spec       v1alpha1.BlastRadiusSpec
		target     string
		resource   string
		wantErr    bool
		errContain string
	}{
		{
			name: "resource in forbidden list",
			spec: v1alpha1.BlastRadiusSpec{
				MaxPodsAffected:    1,
				AllowedNamespaces:  []string{"opendatahub"},
				ForbiddenResources: []string{"Deployment/etcd"},
			},
			target:     "opendatahub",
			resource:   "Deployment/etcd",
			wantErr:    true,
			errContain: "forbidden",
		},
		{
			name: "resource not in forbidden list",
			spec: v1alpha1.BlastRadiusSpec{
				MaxPodsAffected:    1,
				AllowedNamespaces:  []string{"opendatahub"},
				ForbiddenResources: []string{"Deployment/etcd"},
			},
			target:   "opendatahub",
			resource: "Deployment/dashboard",
			wantErr:  false,
		},
		{
			name: "empty forbidden list",
			spec: v1alpha1.BlastRadiusSpec{
				MaxPodsAffected:   1,
				AllowedNamespaces: []string{"opendatahub"},
			},
			target:   "opendatahub",
			resource: "Deployment/anything",
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateBlastRadius(tt.spec, tt.target, tt.resource, 1)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errContain)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
```

**Step 2: Run test to verify it fails**

Run: `cd /Users/ugogiordano/workdir/rhoai/opendatahub-io/odh-platform-chaos && go test ./pkg/safety/ -run TestValidateBlastRadiusForbiddenResources -v`
Expected: FAIL (compilation error, new `resource` parameter)

**Step 3: Fix ValidateBlastRadius signature and logic**

Update `pkg/safety/blastradius.go`:

```go
func ValidateBlastRadius(spec v1alpha1.BlastRadiusSpec, targetNamespace string, targetResource string, affectedCount int) error {
	// Check namespace
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

	// Check forbidden resources
	for _, forbidden := range spec.ForbiddenResources {
		if forbidden == targetResource {
			return fmt.Errorf("resource %q is in the forbidden list and cannot be targeted", forbidden)
		}
	}

	// Check max pods
	if spec.MaxPodsAffected <= 0 {
		return fmt.Errorf("maxPodsAffected must be > 0, got %d", spec.MaxPodsAffected)
	}
	if affectedCount > spec.MaxPodsAffected {
		return fmt.Errorf("affected count %d exceeds maxPodsAffected %d", affectedCount, spec.MaxPodsAffected)
	}

	return nil
}
```

**Step 4: Update all callers to pass the resource parameter**

Update `pkg/orchestrator/lifecycle.go` line 91:

```go
targetResource := exp.Spec.Target.Resource
if targetResource == "" {
	targetResource = fmt.Sprintf("%s/%s", exp.Spec.Target.Component, exp.Metadata.Name)
}
if err := safety.ValidateBlastRadius(exp.Spec.BlastRadius, namespace, targetResource, exp.Spec.Injection.Count); err != nil {
```

Update existing tests in `pkg/safety/blastradius_test.go` to add the `resource` parameter:

```go
err := ValidateBlastRadius(tt.spec, tt.target, "Deployment/test", 1)
```

Update `pkg/orchestrator/lifecycle_test.go` if needed (should be fine since it goes through the orchestrator).

**Step 5: Run all tests**

Run: `cd /Users/ugogiordano/workdir/rhoai/opendatahub-io/odh-platform-chaos && go test ./... -short -count=1`
Expected: ALL PASS

**Step 6: Commit**

```bash
git add pkg/safety/blastradius.go pkg/safety/blastradius_test.go pkg/orchestrator/lifecycle.go
git commit -m "fix: ForbiddenResources validates against target resource not namespace"
```

---

### Task 4: Add signal handling and cleanup-safe context

**Files:**
- Modify: `cmd/odh-chaos/main.go`
- Modify: `pkg/orchestrator/lifecycle.go:167-174`
- Test: `pkg/orchestrator/lifecycle_test.go`

**Step 1: Write the failing test**

```go
func TestOrchestratorCleanupUsesBackgroundContext(t *testing.T) {
	obs := &mockObserver{result: &v1alpha1.CheckResult{Passed: true, ChecksRun: 1, ChecksPassed: 1, Timestamp: time.Now()}}
	inj := &mockInjector{}
	orch := newTestOrchestrator(obs, inj)

	// Use a cancelled context
	ctx, cancel := context.WithCancel(context.Background())

	// Run with a mock that cancels context during injection
	inj2 := &contextCancellingInjector{cancel: cancel}
	registry := injection.NewRegistry()
	registry.Register(v1alpha1.PodKill, inj2)
	orch.registry = registry

	_, _ = orch.Run(ctx, newTestExperiment())
	// Cleanup should still have been called even though ctx was cancelled
	assert.True(t, inj2.cleanupCalled, "cleanup must run even with cancelled context")
}
```

**Step 2: Run test to verify it fails**

Expected: Depends on implementation, may pass with current defer but cleanup may error.

**Step 3: Implement cleanup-safe context**

In `pkg/orchestrator/lifecycle.go`, change the cleanup defer block (lines 167-174):

```go
defer func() {
	if cleanup != nil {
		o.log("Phase: CLEANUP")
		// Use a fresh context for cleanup so it works even if original context was cancelled
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cleanupCancel()
		if cleanErr := cleanup(cleanupCtx); cleanErr != nil {
			o.log("Cleanup warning: %v", cleanErr)
		}
	}
}()
```

In `cmd/odh-chaos/main.go`, add signal handling:

```go
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/opendatahub-io/odh-platform-chaos/internal/cli"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	cmd := cli.NewRootCommand()
	cmd.SetContext(ctx)

	if err := cmd.ExecuteContext(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
```

**Step 4: Run all tests**

Run: `cd /Users/ugogiordano/workdir/rhoai/opendatahub-io/odh-platform-chaos && go test ./... -short -count=1`
Expected: ALL PASS

**Step 5: Commit**

```bash
git add cmd/odh-chaos/main.go pkg/orchestrator/lifecycle.go pkg/orchestrator/lifecycle_test.go
git commit -m "fix: add signal handling and cleanup-safe context"
```

---

### Task 5: Add Observer error return

**Files:**
- Modify: `pkg/observer/engine.go`
- Modify: `pkg/observer/kubernetes.go`
- Modify: `pkg/orchestrator/lifecycle.go`
- Modify: `pkg/orchestrator/lifecycle_test.go`
- Test: `pkg/observer/kubernetes_test.go`

**Step 1: Update the Observer interface**

In `pkg/observer/engine.go`:

```go
type Observer interface {
	CheckSteadyState(ctx context.Context, checks []v1alpha1.SteadyStateCheck, namespace string) (*v1alpha1.CheckResult, error)
}
```

**Step 2: Update KubernetesObserver to return errors**

In `pkg/observer/kubernetes.go`, update the method signature:

```go
func (o *KubernetesObserver) CheckSteadyState(ctx context.Context, checks []v1alpha1.SteadyStateCheck, namespace string) (*v1alpha1.CheckResult, error) {
```

Return `(result, nil)` for check pass/fail results, return `(nil, err)` for infrastructure errors (e.g., K8s API unreachable).

**Step 3: Update orchestrator callers**

In `pkg/orchestrator/lifecycle.go`, update pre-check and post-check:

```go
preCheck, err := o.observer.CheckSteadyState(ctx, exp.Spec.SteadyState.Checks, namespace)
if err != nil {
	result.Error = fmt.Sprintf("steady state pre-check error: %v", err)
	result.Phase = v1alpha1.PhaseAborted
	return result, fmt.Errorf("pre-check: %w", err)
}
```

**Step 4: Update mock observer in tests**

In `pkg/orchestrator/lifecycle_test.go`:

```go
func (m *mockObserver) CheckSteadyState(ctx context.Context, checks []v1alpha1.SteadyStateCheck, namespace string) (*v1alpha1.CheckResult, error) {
	if m.result != nil {
		return m.result, nil
	}
	return &v1alpha1.CheckResult{Passed: true, ChecksRun: 0, Timestamp: time.Now()}, nil
}
```

**Step 5: Update observer tests**

In `pkg/observer/kubernetes_test.go`:

```go
func TestCheckSteadyStateEmptyChecks(t *testing.T) {
	obs := NewKubernetesObserver(nil)
	result, err := obs.CheckSteadyState(nil, nil, "test")
	assert.NoError(t, err)
	assert.True(t, result.Passed)
	assert.Equal(t, 0, result.ChecksRun)
}
```

**Step 6: Run all tests**

Run: `cd /Users/ugogiordano/workdir/rhoai/opendatahub-io/odh-platform-chaos && go test ./... -short -count=1`
Expected: ALL PASS

**Step 7: Commit**

```bash
git add pkg/observer/engine.go pkg/observer/kubernetes.go pkg/observer/kubernetes_test.go pkg/orchestrator/lifecycle.go pkg/orchestrator/lifecycle_test.go
git commit -m "fix: Observer.CheckSteadyState returns error for infrastructure failures"
```

---

### Task 6: Fix CRDMutation cleanup resourceVersion conflict

**Files:**
- Modify: `pkg/injection/crdmutation.go:84-91`
- Test: `pkg/injection/crdmutation_test.go`

**Step 1: Write the failing test**

```go
func TestCRDMutationCleanupHandlesResourceVersionConflict(t *testing.T) {
	// Test that cleanup uses server-side merge patch instead of full replace
	// so it handles resourceVersion conflicts from operator reconciliation
	scheme := runtime.NewScheme()
	fake := fake.NewClientBuilder().WithScheme(scheme).Build()
	injector := NewCRDMutationInjector(fake)

	// Verify the injector uses merge patch for cleanup (not full replace)
	assert.NotNil(t, injector)
	// Full test requires creating an unstructured object, injecting, and cleaning up
}
```

**Step 2: Fix cleanup to use merge patch**

In `pkg/injection/crdmutation.go`, change the cleanup function to re-fetch and use merge patch:

```go
cleanup := func(ctx context.Context) error {
	// Re-fetch to get current resourceVersion
	current := &unstructured.Unstructured{}
	current.SetAPIVersion(spec.Parameters["apiVersion"])
	current.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   gvr.Group,
		Version: gvr.Version,
		Kind:    spec.Parameters["kind"],
	})
	if err := m.client.Get(ctx, key, current); err != nil {
		return fmt.Errorf("getting current resource for cleanup: %w", err)
	}

	// Restore just the mutated field using merge patch
	patchMap := map[string]interface{}{
		"spec": map[string]interface{}{
			spec.Parameters["field"]: originalValue,
		},
	}
	patchBytes, err := json.Marshal(patchMap)
	if err != nil {
		return fmt.Errorf("marshaling cleanup patch: %w", err)
	}
	return m.client.Patch(ctx, current, client.RawPatch(types.MergePatchType, patchBytes))
}
```

Where `originalValue` is captured from the original resource before mutation.

**Step 3: Run all tests**

Run: `cd /Users/ugogiordano/workdir/rhoai/opendatahub-io/odh-platform-chaos && go test ./... -short -count=1`
Expected: ALL PASS

**Step 4: Commit**

```bash
git add pkg/injection/crdmutation.go pkg/injection/crdmutation_test.go
git commit -m "fix: CRDMutation cleanup uses merge patch to avoid resourceVersion conflicts"
```

---

### Task 7: Add context timeouts to K8s API calls

**Files:**
- Modify: `internal/cli/run.go`
- Modify: `internal/cli/clean.go`
- Modify: `pkg/observer/reconciliation.go`
- Test: `pkg/observer/reconciliation_test.go`

**Step 1: Add --timeout flag to run command**

In `internal/cli/run.go`, add a `--timeout` flag (default 10m) and wrap the context:

```go
var timeout time.Duration
cmd.Flags().DurationVar(&timeout, "timeout", 10*time.Minute, "total experiment timeout")

// In RunE:
ctx, cancel := context.WithTimeout(cmd.Context(), timeout)
defer cancel()
```

**Step 2: Fix ReconciliationChecker to use context-aware polling**

In `pkg/observer/reconciliation.go`, replace `time.Sleep(2 * time.Second)` with:

```go
select {
case <-time.After(pollInterval):
	// continue polling
case <-ctx.Done():
	return result, ctx.Err()
}
```

**Step 3: Add timeout to clean command**

In `internal/cli/clean.go`, use a context with timeout:

```go
ctx, cancel := context.WithTimeout(cmd.Context(), 60*time.Second)
defer cancel()
```

**Step 4: Run all tests**

Run: `cd /Users/ugogiordano/workdir/rhoai/opendatahub-io/odh-platform-chaos && go test ./... -short -count=1`
Expected: ALL PASS

**Step 5: Commit**

```bash
git add internal/cli/run.go internal/cli/clean.go pkg/observer/reconciliation.go pkg/observer/reconciliation_test.go
git commit -m "fix: add context timeouts to K8s API calls and ReconciliationChecker"
```

---

### Task 8: Add file size limits to YAML loaders

**Files:**
- Modify: `pkg/experiment/loader.go`
- Modify: `pkg/model/loader.go`
- Test: `pkg/experiment/loader_test.go`

**Step 1: Write the failing test**

```go
func TestLoadRejectsOversizedFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "huge.yaml")
	// Create a file larger than 1MB
	data := make([]byte, 2*1024*1024)
	require.NoError(t, os.WriteFile(path, data, 0644))

	_, err := Load(path)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "exceeds maximum")
}
```

**Step 2: Run test to verify it fails**

Expected: FAIL (currently reads any size)

**Step 3: Add file size check**

In `pkg/experiment/loader.go`:

```go
const maxExperimentFileSize = 1 * 1024 * 1024 // 1 MB

func Load(path string) (*v1alpha1.ChaosExperiment, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("stat %s: %w", path, err)
	}
	if info.Size() > maxExperimentFileSize {
		return nil, fmt.Errorf("file %s (%d bytes) exceeds maximum size of %d bytes", path, info.Size(), maxExperimentFileSize)
	}

	data, err := os.ReadFile(path)
	// ... rest unchanged
}
```

Same pattern in `pkg/model/loader.go`.

**Step 4: Run all tests**

Run: `cd /Users/ugogiordano/workdir/rhoai/opendatahub-io/odh-platform-chaos && go test ./... -short -count=1`
Expected: ALL PASS

**Step 5: Commit**

```bash
git add pkg/experiment/loader.go pkg/experiment/loader_test.go pkg/model/loader.go
git commit -m "fix: add file size limits to YAML loaders (max 1MB)"
```

---

## Part B: Safety Mechanisms (Design Doc M12)

### Task 9: TTL enforcement via annotations

**Files:**
- Create: `pkg/safety/ttl.go`
- Create: `pkg/safety/ttl_test.go`
- Modify: `pkg/injection/network.go`
- Modify: `pkg/orchestrator/lifecycle.go`

**Step 1: Write the failing test**

```go
// pkg/safety/ttl_test.go
package safety

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestTTLAnnotation(t *testing.T) {
	expiry := TTLExpiry(5 * time.Minute)
	assert.False(t, IsExpired(expiry))

	pastExpiry := time.Now().Add(-1 * time.Minute).Format(time.RFC3339)
	assert.True(t, IsExpired(pastExpiry))
}

func TestTTLAnnotationKey(t *testing.T) {
	assert.Equal(t, "chaos.opendatahub.io/expires", TTLAnnotationKey)
}
```

**Step 2: Run test to verify it fails**

Expected: FAIL (package doesn't exist)

**Step 3: Implement TTL utilities**

```go
// pkg/safety/ttl.go
package safety

import "time"

const TTLAnnotationKey = "chaos.opendatahub.io/expires"

func TTLExpiry(ttl time.Duration) string {
	return time.Now().Add(ttl).Format(time.RFC3339)
}

func IsExpired(expiryStr string) bool {
	expiry, err := time.Parse(time.RFC3339, expiryStr)
	if err != nil {
		return true // malformed = treat as expired
	}
	return time.Now().After(expiry)
}
```

**Step 4: Wire TTL annotations into NetworkPartition injector**

In `pkg/injection/network.go`, add the TTL annotation to the created NetworkPolicy:

```go
Annotations: map[string]string{
	safety.TTLAnnotationKey: safety.TTLExpiry(spec.TTL.Duration),
},
```

**Step 5: Run all tests**

Run: `cd /Users/ugogiordano/workdir/rhoai/opendatahub-io/odh-platform-chaos && go test ./... -short -count=1`
Expected: ALL PASS

**Step 6: Commit**

```bash
git add pkg/safety/ttl.go pkg/safety/ttl_test.go pkg/injection/network.go
git commit -m "feat: add TTL enforcement via annotations on chaos artifacts"
```

---

### Task 10: Kubernetes Lease-based distributed locking

**Files:**
- Create: `pkg/safety/lease.go`
- Create: `pkg/safety/lease_test.go`
- Modify: `internal/cli/run.go`

**Step 1: Write the failing test**

```go
// pkg/safety/lease_test.go
package safety

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	coordinationv1 "k8s.io/api/coordination/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

func TestLeaseExperimentLockAcquire(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, coordinationv1.AddToScheme(scheme))
	client := fake.NewClientBuilder().WithScheme(scheme).Build()

	lock := NewLeaseExperimentLock(client, "opendatahub")
	err := lock.Acquire(context.Background(), "test-operator", "test-experiment")
	assert.NoError(t, err)
}

func TestLeaseExperimentLockConflict(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, coordinationv1.AddToScheme(scheme))
	client := fake.NewClientBuilder().WithScheme(scheme).Build()

	lock := NewLeaseExperimentLock(client, "opendatahub")
	err := lock.Acquire(context.Background(), "test-operator", "experiment-1")
	require.NoError(t, err)

	err = lock.Acquire(context.Background(), "test-operator", "experiment-2")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "experiment-1")
}
```

**Step 2: Run test to verify it fails**

Expected: FAIL (function doesn't exist)

**Step 3: Implement Lease-based lock**

```go
// pkg/safety/lease.go
package safety

import (
	"context"
	"fmt"

	coordinationv1 "k8s.io/api/coordination/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type LeaseExperimentLock struct {
	client    client.Client
	namespace string
}

func NewLeaseExperimentLock(c client.Client, namespace string) *LeaseExperimentLock {
	return &LeaseExperimentLock{client: c, namespace: namespace}
}

func (l *LeaseExperimentLock) Acquire(ctx context.Context, operator string, experimentName string) error {
	leaseName := fmt.Sprintf("odh-chaos-lock-%s", operator)
	lease := &coordinationv1.Lease{
		ObjectMeta: metav1.ObjectMeta{
			Name:      leaseName,
			Namespace: l.namespace,
			Labels:    map[string]string{"app.kubernetes.io/managed-by": "odh-chaos"},
		},
		Spec: coordinationv1.LeaseSpec{
			HolderIdentity: &experimentName,
		},
	}

	err := l.client.Create(ctx, lease)
	if errors.IsAlreadyExists(err) {
		// Check who holds it
		existing := &coordinationv1.Lease{}
		if getErr := l.client.Get(ctx, client.ObjectKeyFromObject(lease), existing); getErr != nil {
			return fmt.Errorf("checking existing lease: %w", getErr)
		}
		holder := ""
		if existing.Spec.HolderIdentity != nil {
			holder = *existing.Spec.HolderIdentity
		}
		return fmt.Errorf("experiment %q already running for operator %s (held by %s)", holder, operator, holder)
	}
	return err
}

func (l *LeaseExperimentLock) Release(operator string) {
	leaseName := fmt.Sprintf("odh-chaos-lock-%s", operator)
	lease := &coordinationv1.Lease{
		ObjectMeta: metav1.ObjectMeta{
			Name:      leaseName,
			Namespace: l.namespace,
		},
	}
	_ = l.client.Delete(context.Background(), lease)
}
```

**Step 4: Wire into CLI run command**

In `internal/cli/run.go`, when a K8s client is available, use `NewLeaseExperimentLock` instead of `NewLocalExperimentLock`.

**Step 5: Run all tests**

Run: `cd /Users/ugogiordano/workdir/rhoai/opendatahub-io/odh-platform-chaos && go test ./... -short -count=1`
Expected: ALL PASS

**Step 6: Commit**

```bash
git add pkg/safety/lease.go pkg/safety/lease_test.go internal/cli/run.go
git commit -m "feat: add Kubernetes Lease-based distributed experiment locking"
```

---

### Task 11: Expand clean command for all injector types

**Files:**
- Modify: `internal/cli/clean.go`
- Create: `pkg/safety/rollback.go`
- Create: `pkg/safety/rollback_test.go`

**Step 1: Write the failing test**

```go
// pkg/safety/rollback_test.go
package safety

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRollbackAnnotationKey(t *testing.T) {
	assert.Equal(t, "chaos.opendatahub.io/rollback-data", RollbackAnnotationKey)
}

func TestManagedByLabel(t *testing.T) {
	assert.Equal(t, "odh-chaos", ManagedByValue)
}
```

**Step 2: Implement rollback tracking**

```go
// pkg/safety/rollback.go
package safety

const (
	RollbackAnnotationKey = "chaos.opendatahub.io/rollback-data"
	ManagedByLabel        = "app.kubernetes.io/managed-by"
	ManagedByValue        = "odh-chaos"
	ChaosTypeLabel        = "chaos.opendatahub.io/type"
)
```

**Step 3: Expand clean command**

In `internal/cli/clean.go`, add scanning for:
- NetworkPolicies (existing)
- Leases with chaos labels
- ConfigMaps with chaos annotations (for rollback data)

**Step 4: Run all tests**

Run: `cd /Users/ugogiordano/workdir/rhoai/opendatahub-io/odh-platform-chaos && go test ./... -short -count=1`
Expected: ALL PASS

**Step 5: Commit**

```bash
git add internal/cli/clean.go pkg/safety/rollback.go pkg/safety/rollback_test.go
git commit -m "feat: expand clean command to scan all chaos artifact types"
```

---

## Part C: Advanced Injectors (Design Doc M13)

### Task 12: WebhookDisrupt injector

**Files:**
- Create: `pkg/injection/webhook.go`
- Create: `pkg/injection/webhook_test.go`
- Modify: `internal/cli/run.go` (register injector)

**Step 1: Write the failing test**

```go
// pkg/injection/webhook_test.go
package injection

import (
	"testing"

	v1alpha1 "github.com/opendatahub-io/odh-platform-chaos/api/v1alpha1"
	"github.com/stretchr/testify/assert"
)

func TestWebhookDisruptValidate(t *testing.T) {
	injector := &WebhookDisruptInjector{}
	blast := v1alpha1.BlastRadiusSpec{
		MaxPodsAffected:   1,
		AllowedNamespaces: []string{"test"},
	}

	tests := []struct {
		name    string
		spec    v1alpha1.InjectionSpec
		wantErr bool
	}{
		{
			name: "valid spec",
			spec: v1alpha1.InjectionSpec{
				Type: v1alpha1.WebhookDisrupt,
				Parameters: map[string]string{
					"webhookName": "my-webhook",
					"action":      "setFailurePolicy",
					"value":       "Fail",
				},
			},
			wantErr: false,
		},
		{
			name: "missing webhookName",
			spec: v1alpha1.InjectionSpec{
				Type:       v1alpha1.WebhookDisrupt,
				Parameters: map[string]string{"action": "setFailurePolicy"},
			},
			wantErr: true,
		},
		{
			name: "missing action",
			spec: v1alpha1.InjectionSpec{
				Type:       v1alpha1.WebhookDisrupt,
				Parameters: map[string]string{"webhookName": "my-webhook"},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := injector.Validate(tt.spec, blast)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
```

**Step 2: Run test to verify it fails**

Expected: FAIL (type doesn't exist)

**Step 3: Implement WebhookDisruptInjector**

```go
// pkg/injection/webhook.go
package injection

import (
	"context"
	"fmt"

	v1alpha1 "github.com/opendatahub-io/odh-platform-chaos/api/v1alpha1"
	admissionv1 "k8s.io/api/admissionregistration/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type WebhookDisruptInjector struct {
	client client.Client
}

func NewWebhookDisruptInjector(c client.Client) *WebhookDisruptInjector {
	return &WebhookDisruptInjector{client: c}
}

func (w *WebhookDisruptInjector) Validate(spec v1alpha1.InjectionSpec, blast v1alpha1.BlastRadiusSpec) error {
	if spec.Parameters == nil || spec.Parameters["webhookName"] == "" {
		return fmt.Errorf("webhookName parameter is required")
	}
	if spec.Parameters["action"] == "" {
		return fmt.Errorf("action parameter is required (setFailurePolicy, setTimeout)")
	}
	return nil
}

func (w *WebhookDisruptInjector) Inject(ctx context.Context, spec v1alpha1.InjectionSpec, namespace string) (CleanupFunc, []v1alpha1.InjectionEvent, error) {
	// Implementation: modify ValidatingWebhookConfiguration or MutatingWebhookConfiguration
	// Save original, apply disruption, return cleanup that restores original
	// ...
	return nil, nil, fmt.Errorf("not yet implemented")
}
```

**Step 4: Run tests**

Run: `cd /Users/ugogiordano/workdir/rhoai/opendatahub-io/odh-platform-chaos && go test ./pkg/injection/ -v`
Expected: ALL PASS

**Step 5: Commit**

```bash
git add pkg/injection/webhook.go pkg/injection/webhook_test.go
git commit -m "feat: add WebhookDisrupt injector with validation"
```

---

### Task 13: RBACRevoke injector

**Files:**
- Create: `pkg/injection/rbac.go`
- Create: `pkg/injection/rbac_test.go`

**Step 1: Write the failing test**

```go
// pkg/injection/rbac_test.go
package injection

import (
	"testing"

	v1alpha1 "github.com/opendatahub-io/odh-platform-chaos/api/v1alpha1"
	"github.com/stretchr/testify/assert"
)

func TestRBACRevokeValidate(t *testing.T) {
	injector := &RBACRevokeInjector{}
	blast := v1alpha1.BlastRadiusSpec{
		MaxPodsAffected:   1,
		AllowedNamespaces: []string{"test"},
	}

	tests := []struct {
		name    string
		spec    v1alpha1.InjectionSpec
		wantErr bool
	}{
		{
			name: "valid spec with clusterRoleBinding",
			spec: v1alpha1.InjectionSpec{
				Type: v1alpha1.RBACRevoke,
				Parameters: map[string]string{
					"bindingName": "my-operator-binding",
					"bindingType": "ClusterRoleBinding",
				},
			},
			wantErr: false,
		},
		{
			name: "missing bindingName",
			spec: v1alpha1.InjectionSpec{
				Type:       v1alpha1.RBACRevoke,
				Parameters: map[string]string{"bindingType": "ClusterRoleBinding"},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := injector.Validate(tt.spec, blast)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
```

**Step 2: Implement RBACRevokeInjector**

Similar pattern to WebhookDisrupt. Modifies ClusterRoleBinding or RoleBinding subjects, saves original, restores on cleanup.

**Step 3: Run tests, commit**

```bash
git add pkg/injection/rbac.go pkg/injection/rbac_test.go
git commit -m "feat: add RBACRevoke injector with validation"
```

---

### Task 14: FinalizerBlock injector

**Files:**
- Create: `pkg/injection/finalizer.go`
- Create: `pkg/injection/finalizer_test.go`

**Step 1: Write the failing test**

```go
// pkg/injection/finalizer_test.go
package injection

import (
	"testing"

	v1alpha1 "github.com/opendatahub-io/odh-platform-chaos/api/v1alpha1"
	"github.com/stretchr/testify/assert"
)

func TestFinalizerBlockValidate(t *testing.T) {
	injector := &FinalizerBlockInjector{}
	blast := v1alpha1.BlastRadiusSpec{
		MaxPodsAffected:   1,
		AllowedNamespaces: []string{"test"},
	}

	tests := []struct {
		name    string
		spec    v1alpha1.InjectionSpec
		wantErr bool
	}{
		{
			name: "valid spec",
			spec: v1alpha1.InjectionSpec{
				Type: v1alpha1.FinalizerBlock,
				Parameters: map[string]string{
					"apiVersion": "apps/v1",
					"kind":       "Deployment",
					"name":       "my-deploy",
					"finalizer":  "chaos.opendatahub.io/block",
				},
			},
			wantErr: false,
		},
		{
			name: "missing name",
			spec: v1alpha1.InjectionSpec{
				Type: v1alpha1.FinalizerBlock,
				Parameters: map[string]string{
					"apiVersion": "apps/v1",
					"kind":       "Deployment",
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := injector.Validate(tt.spec, blast)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
```

**Step 2: Implement FinalizerBlockInjector**

Adds a stuck finalizer to the target resource. Cleanup removes it.

**Step 3: Run tests, commit**

```bash
git add pkg/injection/finalizer.go pkg/injection/finalizer_test.go
git commit -m "feat: add FinalizerBlock injector with validation"
```

---

## Part D: Chaos SDK Core (Design Doc M9)

### Task 15: FaultConfig types and activation model

**Files:**
- Create: `pkg/sdk/types.go`
- Create: `pkg/sdk/types_test.go`

**Step 1: Write the failing test**

```go
// pkg/sdk/types_test.go
package sdk

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFaultConfigDefaults(t *testing.T) {
	cfg := &FaultConfig{}
	assert.False(t, cfg.IsActive())
	assert.Nil(t, cfg.MaybeInject("get"))
}

func TestFaultConfigActive(t *testing.T) {
	cfg := &FaultConfig{
		Active: true,
		Faults: map[string]FaultSpec{
			"get": {ErrorRate: 1.0, Error: "simulated error"},
		},
	}
	assert.True(t, cfg.IsActive())
	err := cfg.MaybeInject("get")
	assert.Error(t, err)
	assert.Equal(t, "simulated error", err.Error())
}

func TestFaultConfigInactiveNoInjection(t *testing.T) {
	cfg := &FaultConfig{
		Active: false,
		Faults: map[string]FaultSpec{
			"get": {ErrorRate: 1.0, Error: "simulated error"},
		},
	}
	assert.Nil(t, cfg.MaybeInject("get"))
}
```

**Step 2: Run test to verify it fails**

Expected: FAIL (package doesn't exist)

**Step 3: Implement FaultConfig types**

```go
// pkg/sdk/types.go
package sdk

import (
	"fmt"
	"math/rand"
	"sync"
	"time"
)

type FaultSpec struct {
	ErrorRate float64       `json:"errorRate" yaml:"errorRate"`
	Error     string        `json:"error" yaml:"error"`
	Delay     time.Duration `json:"delay,omitempty" yaml:"delay,omitempty"`
}

type FaultConfig struct {
	mu     sync.RWMutex
	Active bool                 `json:"active" yaml:"active"`
	Faults map[string]FaultSpec `json:"faults,omitempty" yaml:"faults,omitempty"`
}

func (f *FaultConfig) IsActive() bool {
	if f == nil {
		return false
	}
	f.mu.RLock()
	defer f.mu.RUnlock()
	return f.Active
}

func (f *FaultConfig) MaybeInject(operation string) error {
	if f == nil || !f.IsActive() {
		return nil
	}
	f.mu.RLock()
	spec, ok := f.Faults[operation]
	f.mu.RUnlock()
	if !ok {
		return nil
	}
	if spec.Delay > 0 {
		time.Sleep(spec.Delay)
	}
	if spec.ErrorRate > 0 && rand.Float64() < spec.ErrorRate {
		return fmt.Errorf("%s", spec.Error)
	}
	return nil
}
```

**Step 4: Run tests**

Run: `cd /Users/ugogiordano/workdir/rhoai/opendatahub-io/odh-platform-chaos && go test ./pkg/sdk/ -v`
Expected: ALL PASS

**Step 5: Commit**

```bash
git add pkg/sdk/types.go pkg/sdk/types_test.go
git commit -m "feat: add FaultConfig types and activation model for Chaos SDK"
```

---

### Task 16: ChaosClient hybrid wrapper

**Files:**
- Create: `pkg/sdk/client.go`
- Create: `pkg/sdk/client_test.go`

**Step 1: Write the failing test**

```go
// pkg/sdk/client_test.go
package sdk

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestChaosClientPassthrough(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, corev1.AddToScheme(scheme))
	inner := fake.NewClientBuilder().WithScheme(scheme).WithObjects(
		&corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{Name: "test-cm", Namespace: "default"},
		},
	).Build()

	cc := NewChaosClient(inner, nil) // nil faults = pure passthrough

	cm := &corev1.ConfigMap{}
	err := cc.Get(context.Background(), client.ObjectKey{Name: "test-cm", Namespace: "default"}, cm)
	assert.NoError(t, err)
	assert.Equal(t, "test-cm", cm.Name)
}

func TestChaosClientFaultInjection(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, corev1.AddToScheme(scheme))
	inner := fake.NewClientBuilder().WithScheme(scheme).WithObjects(
		&corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{Name: "test-cm", Namespace: "default"},
		},
	).Build()

	faults := &FaultConfig{
		Active: true,
		Faults: map[string]FaultSpec{
			"get": {ErrorRate: 1.0, Error: "api server error"},
		},
	}

	cc := NewChaosClient(inner, faults)

	cm := &corev1.ConfigMap{}
	err := cc.Get(context.Background(), client.ObjectKey{Name: "test-cm", Namespace: "default"}, cm)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "api server error")
}
```

**Step 2: Run test to verify it fails**

Expected: FAIL (type doesn't exist)

**Step 3: Implement ChaosClient**

```go
// pkg/sdk/client.go
package sdk

import (
	"context"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type ChaosClient struct {
	inner  client.Client
	faults *FaultConfig
}

func NewChaosClient(inner client.Client, faults *FaultConfig) *ChaosClient {
	return &ChaosClient{inner: inner, faults: faults}
}

func (c *ChaosClient) Get(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
	if err := c.faults.MaybeInject("get"); err != nil {
		return err
	}
	return c.inner.Get(ctx, key, obj, opts...)
}

func (c *ChaosClient) List(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error {
	if err := c.faults.MaybeInject("list"); err != nil {
		return err
	}
	return c.inner.List(ctx, list, opts...)
}

func (c *ChaosClient) Create(ctx context.Context, obj client.Object, opts ...client.CreateOption) error {
	if err := c.faults.MaybeInject("create"); err != nil {
		return err
	}
	return c.inner.Create(ctx, obj, opts...)
}

func (c *ChaosClient) Delete(ctx context.Context, obj client.Object, opts ...client.DeleteOption) error {
	if err := c.faults.MaybeInject("delete"); err != nil {
		return err
	}
	return c.inner.Delete(ctx, obj, opts...)
}

func (c *ChaosClient) Update(ctx context.Context, obj client.Object, opts ...client.UpdateOption) error {
	if err := c.faults.MaybeInject("update"); err != nil {
		return err
	}
	return c.inner.Update(ctx, obj, opts...)
}

func (c *ChaosClient) Patch(ctx context.Context, obj client.Object, patch client.Patch, opts ...client.PatchOption) error {
	if err := c.faults.MaybeInject("patch"); err != nil {
		return err
	}
	return c.inner.Patch(ctx, obj, patch, opts...)
}

func (c *ChaosClient) DeleteAllOf(ctx context.Context, obj client.Object, opts ...client.DeleteAllOfOption) error {
	if err := c.faults.MaybeInject("deleteAllOf"); err != nil {
		return err
	}
	return c.inner.DeleteAllOf(ctx, obj, opts...)
}

// Delegate non-mutation methods directly
func (c *ChaosClient) Scheme() *runtime.Scheme          { return c.inner.Scheme() }
func (c *ChaosClient) RESTMapper() meta.RESTMapper       { return c.inner.RESTMapper() }
func (c *ChaosClient) GroupVersionKindFor(obj runtime.Object) (schema.GroupVersionKind, error) {
	return c.inner.GroupVersionKindFor(obj)
}
func (c *ChaosClient) IsObjectNamespaced(obj runtime.Object) (bool, error) {
	return c.inner.IsObjectNamespaced(obj)
}
func (c *ChaosClient) Status() client.SubResourceWriter { return c.inner.Status() }
func (c *ChaosClient) SubResource(subResource string) client.SubResourceClient {
	return c.inner.SubResource(subResource)
}
```

**Step 4: Run tests**

Run: `cd /Users/ugogiordano/workdir/rhoai/opendatahub-io/odh-platform-chaos && go test ./pkg/sdk/ -v`
Expected: ALL PASS

**Step 5: Commit**

```bash
git add pkg/sdk/client.go pkg/sdk/client_test.go
git commit -m "feat: add ChaosClient hybrid wrapper for controller-runtime"
```

---

### Task 17: chaos.WrapReconciler middleware

**Files:**
- Create: `pkg/sdk/wrap.go`
- Create: `pkg/sdk/wrap_test.go`

**Step 1: Write the failing test**

```go
// pkg/sdk/wrap_test.go
package sdk

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

type fakeReconciler struct {
	called bool
}

func (f *fakeReconciler) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	f.called = true
	return ctrl.Result{}, nil
}

func TestWrapReconcilerPassthrough(t *testing.T) {
	inner := &fakeReconciler{}
	wrapped := WrapReconciler(inner)
	require.NotNil(t, wrapped)

	result, err := wrapped.Reconcile(context.Background(), reconcile.Request{})
	assert.NoError(t, err)
	assert.True(t, inner.called)
	assert.Equal(t, ctrl.Result{}, result)
}
```

**Step 2: Run test to verify it fails**

Expected: FAIL (function doesn't exist)

**Step 3: Implement WrapReconciler**

```go
// pkg/sdk/wrap.go
package sdk

import (
	"context"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

type Option func(*chaosReconciler)

func WithFaultConfig(fc *FaultConfig) Option {
	return func(cr *chaosReconciler) {
		cr.faults = fc
	}
}

type chaosReconciler struct {
	inner  reconcile.Reconciler
	faults *FaultConfig
}

func WrapReconciler(inner reconcile.Reconciler, opts ...Option) reconcile.Reconciler {
	cr := &chaosReconciler{inner: inner}
	for _, opt := range opts {
		opt(cr)
	}
	return cr
}

func (cr *chaosReconciler) Reconcile(ctx context.Context, req reconcile.Request) (ctrl.Result, error) {
	// Pre-reconcile fault injection could go here
	if cr.faults != nil {
		if err := cr.faults.MaybeInject("reconcile"); err != nil {
			return ctrl.Result{}, err
		}
	}
	return cr.inner.Reconcile(ctx, req)
}
```

**Step 4: Run tests**

Run: `cd /Users/ugogiordano/workdir/rhoai/opendatahub-io/odh-platform-chaos && go test ./pkg/sdk/ -v`
Expected: ALL PASS

**Step 5: Commit**

```bash
git add pkg/sdk/wrap.go pkg/sdk/wrap_test.go
git commit -m "feat: add chaos.WrapReconciler middleware for one-line integration"
```

---

## Part E: ConfigMap Activation + HTTP Endpoint (Design Doc M10)

### Task 18: ConfigMap-based fault activation

**Files:**
- Create: `pkg/sdk/configmap.go`
- Create: `pkg/sdk/configmap_test.go`

**Step 1: Write the failing test**

```go
// pkg/sdk/configmap_test.go
package sdk

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseFaultConfigFromData(t *testing.T) {
	data := map[string]string{
		"config": `{
			"active": true,
			"faults": {
				"get": {"errorRate": 0.5, "error": "timeout"}
			}
		}`,
	}

	cfg, err := ParseFaultConfigFromData(data)
	require.NoError(t, err)
	assert.True(t, cfg.Active)
	assert.Equal(t, 0.5, cfg.Faults["get"].ErrorRate)
}

func TestParseFaultConfigEmpty(t *testing.T) {
	cfg, err := ParseFaultConfigFromData(nil)
	require.NoError(t, err)
	assert.False(t, cfg.IsActive())
}
```

**Step 2: Implement ConfigMap parser**

```go
// pkg/sdk/configmap.go
package sdk

import (
	"encoding/json"
	"fmt"
)

const (
	ChaosConfigMapName = "odh-chaos-config"
	ChaosConfigKey     = "config"
)

func ParseFaultConfigFromData(data map[string]string) (*FaultConfig, error) {
	if data == nil {
		return &FaultConfig{}, nil
	}
	configJSON, ok := data[ChaosConfigKey]
	if !ok || configJSON == "" {
		return &FaultConfig{}, nil
	}
	cfg := &FaultConfig{}
	if err := json.Unmarshal([]byte(configJSON), cfg); err != nil {
		return nil, fmt.Errorf("parsing chaos config: %w", err)
	}
	return cfg, nil
}
```

**Step 3: Run tests, commit**

```bash
git add pkg/sdk/configmap.go pkg/sdk/configmap_test.go
git commit -m "feat: add ConfigMap-based fault activation for Chaos SDK"
```

---

### Task 19: HTTP admin endpoint

**Files:**
- Create: `pkg/sdk/admin.go`
- Create: `pkg/sdk/admin_test.go`

**Step 1: Write the failing test**

```go
// pkg/sdk/admin_test.go
package sdk

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAdminEndpointFaultPoints(t *testing.T) {
	cfg := &FaultConfig{
		Active: true,
		Faults: map[string]FaultSpec{
			"get":    {ErrorRate: 0.5, Error: "timeout"},
			"create": {ErrorRate: 1.0, Error: "forbidden"},
		},
	}

	handler := NewAdminHandler(cfg)
	req := httptest.NewRequest("GET", "/chaos/faultpoints", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "get")
	assert.Contains(t, w.Body.String(), "create")
}
```

**Step 2: Implement admin handler**

```go
// pkg/sdk/admin.go
package sdk

import (
	"encoding/json"
	"net/http"
)

func NewAdminHandler(cfg *FaultConfig) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/chaos/faultpoints", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if cfg == nil {
			json.NewEncoder(w).Encode([]string{})
			return
		}
		cfg.mu.RLock()
		defer cfg.mu.RUnlock()
		type faultPoint struct {
			Name     string  `json:"name"`
			Active   bool    `json:"active"`
			ErrorRate float64 `json:"errorRate"`
		}
		points := make([]faultPoint, 0, len(cfg.Faults))
		for name, spec := range cfg.Faults {
			points = append(points, faultPoint{
				Name:     name,
				Active:   cfg.Active,
				ErrorRate: spec.ErrorRate,
			})
		}
		json.NewEncoder(w).Encode(points)
	})
	mux.HandleFunc("/chaos/status", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		status := map[string]interface{}{
			"active":     cfg != nil && cfg.IsActive(),
			"faultCount": 0,
		}
		if cfg != nil {
			cfg.mu.RLock()
			status["faultCount"] = len(cfg.Faults)
			cfg.mu.RUnlock()
		}
		json.NewEncoder(w).Encode(status)
	})
	return mux
}
```

**Step 3: Run tests, commit**

```bash
git add pkg/sdk/admin.go pkg/sdk/admin_test.go
git commit -m "feat: add HTTP admin endpoint for chaos fault point discovery"
```

---

### Task 20: Go test helper

**Files:**
- Create: `pkg/sdk/testing.go`
- Create: `pkg/sdk/testing_test.go`

**Step 1: Write the failing test**

```go
// pkg/sdk/testing_test.go
package sdk

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewForTest(t *testing.T) {
	ch := NewForTest(t, "model-registry")
	assert.NotNil(t, ch)

	ch.Activate("get", FaultSpec{ErrorRate: 1.0, Error: "test error"})

	err := ch.Config().MaybeInject("get")
	assert.Error(t, err)
	assert.Equal(t, "test error", err.Error())
}

func TestNewForTestAutoCleanup(t *testing.T) {
	ch := NewForTest(t, "model-registry")
	ch.Activate("get", FaultSpec{ErrorRate: 1.0, Error: "test error"})
	// t.Cleanup should deactivate - verified by test not leaking state
}
```

**Step 2: Implement test helper**

```go
// pkg/sdk/testing.go
package sdk

import "testing"

type TestChaos struct {
	component string
	config    *FaultConfig
}

func NewForTest(t *testing.T, component string) *TestChaos {
	tc := &TestChaos{
		component: component,
		config: &FaultConfig{
			Active: true,
			Faults: make(map[string]FaultSpec),
		},
	}
	t.Cleanup(func() {
		tc.config.mu.Lock()
		tc.config.Active = false
		tc.config.Faults = nil
		tc.config.mu.Unlock()
	})
	return tc
}

func (tc *TestChaos) Activate(operation string, spec FaultSpec) {
	tc.config.mu.Lock()
	defer tc.config.mu.Unlock()
	tc.config.Faults[operation] = spec
}

func (tc *TestChaos) Deactivate(operation string) {
	tc.config.mu.Lock()
	defer tc.config.mu.Unlock()
	delete(tc.config.Faults, operation)
}

func (tc *TestChaos) Config() *FaultConfig {
	return tc.config
}
```

**Step 3: Run tests, commit**

```bash
git add pkg/sdk/testing.go pkg/sdk/testing_test.go
git commit -m "feat: add Go test helper for standalone chaos testing"
```

---

## Part F: Phase 2 Injection Types (Design Doc M11)

### Task 21: Add Phase 2 injection type constants

**Files:**
- Modify: `api/v1alpha1/types.go`
- Test: `api/v1alpha1/types_test.go`

**Step 1: Add new injection type constants**

In `api/v1alpha1/types.go`, add after the existing constants:

```go
// Phase 2 injection types (middleware, one-line change)
const (
	ClientThrottle     InjectionType = "ClientThrottle"
	APIServerError     InjectionType = "APIServerError"
	WatchDisconnect    InjectionType = "WatchDisconnect"
	LeaderElectionLoss InjectionType = "LeaderElectionLoss"
	WebhookTimeout     InjectionType = "WebhookTimeout"
	WebhookReject      InjectionType = "WebhookReject"
)
```

**Step 2: Update test**

Add the new types to `TestInjectionTypes`.

**Step 3: Run tests, commit**

```bash
git add api/v1alpha1/types.go api/v1alpha1/types_test.go
git commit -m "feat: add Phase 2 injection type constants"
```

---

### Task 22: Implement SDK-based fault injectors (ClientThrottle, APIServerError, WatchDisconnect)

**Files:**
- Create: `pkg/sdk/faults/k8s.go`
- Create: `pkg/sdk/faults/k8s_test.go`

**Step 1: Write the failing tests**

```go
// pkg/sdk/faults/k8s_test.go
package faults

import (
	"testing"
	"time"

	"github.com/opendatahub-io/odh-platform-chaos/pkg/sdk"
	"github.com/stretchr/testify/assert"
)

func TestClientThrottleFault(t *testing.T) {
	cfg := ClientThrottleConfig(100*time.Millisecond, 1.0)
	assert.Equal(t, 1.0, cfg.ErrorRate)
	assert.Equal(t, 100*time.Millisecond, cfg.Delay)
}

func TestAPIServerErrorFault(t *testing.T) {
	cfg := APIServerErrorConfig("internal error", 1.0)
	assert.Equal(t, 1.0, cfg.ErrorRate)
	assert.Equal(t, "internal error", cfg.Error)
}
```

**Step 2: Implement fault configs**

```go
// pkg/sdk/faults/k8s.go
package faults

import (
	"time"

	"github.com/opendatahub-io/odh-platform-chaos/pkg/sdk"
)

func ClientThrottleConfig(delay time.Duration, rate float64) sdk.FaultSpec {
	return sdk.FaultSpec{
		Delay:     delay,
		ErrorRate: rate,
		Error:     "client throttled",
	}
}

func APIServerErrorConfig(errMsg string, rate float64) sdk.FaultSpec {
	return sdk.FaultSpec{
		ErrorRate: rate,
		Error:     errMsg,
	}
}

func WatchDisconnectConfig(rate float64) sdk.FaultSpec {
	return sdk.FaultSpec{
		ErrorRate: rate,
		Error:     "watch channel closed",
	}
}
```

**Step 3: Run tests, commit**

```bash
git add pkg/sdk/faults/k8s.go pkg/sdk/faults/k8s_test.go
git commit -m "feat: add K8s fault category configs (ClientThrottle, APIServerError, WatchDisconnect)"
```

---

### Task 23: Implement Application fault configs (ForceError, Skip, Panic)

**Files:**
- Create: `pkg/sdk/faults/application.go`
- Create: `pkg/sdk/faults/application_test.go`

Similar pattern to Task 22. ForceError returns an error, Skip returns a sentinel value, Panic calls `panic()` for testing panic recovery.

**Step 1: Write tests, implement, commit**

```bash
git add pkg/sdk/faults/application.go pkg/sdk/faults/application_test.go
git commit -m "feat: add Application fault category configs (ForceError, Skip, Panic)"
```

---

### Task 24: Implement Timing fault configs (Delay, Jitter, DeadlineExceed)

**Files:**
- Create: `pkg/sdk/faults/timing.go`
- Create: `pkg/sdk/faults/timing_test.go`

Delay adds a fixed sleep, Jitter adds random delay, DeadlineExceed cancels the context.

**Step 1: Write tests, implement, commit**

```bash
git add pkg/sdk/faults/timing.go pkg/sdk/faults/timing_test.go
git commit -m "feat: add Timing fault category configs (Delay, Jitter, DeadlineExceed)"
```

---

### Task 25: Register all new injectors in CLI

**Files:**
- Modify: `internal/cli/run.go`

**Step 1: Register WebhookDisrupt, RBACRevoke, FinalizerBlock in the registry**

In `internal/cli/run.go`, add registration of the three new injectors alongside the existing four.

**Step 2: Run all tests**

Run: `cd /Users/ugogiordano/workdir/rhoai/opendatahub-io/odh-platform-chaos && go test ./... -short -count=1`
Expected: ALL PASS

**Step 3: Commit**

```bash
git add internal/cli/run.go
git commit -m "feat: register WebhookDisrupt, RBACRevoke, FinalizerBlock injectors in CLI"
```

---

### Task 26: Final integration test for Phase 2

**Files:**
- Modify: `tests/integration/experiment_test.go`

**Step 1: Add integration test for ChaosClient**

```go
func TestChaosClientIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	testEnv := &envtest.Environment{}
	cfg, err := testEnv.Start()
	if err != nil {
		t.Skipf("skipping: envtest not available: %v", err)
	}
	defer testEnv.Stop()

	k8sClient, err := client.New(cfg, client.Options{})
	require.NoError(t, err)

	// Wrap with chaos client
	faults := &sdk.FaultConfig{Active: false}
	chaosClient := sdk.NewChaosClient(k8sClient, faults)

	// Normal operation should work
	ns := &corev1.Namespace{}
	err = chaosClient.Get(context.Background(), client.ObjectKey{Name: "default"}, ns)
	assert.NoError(t, err)

	// Activate faults
	faults.Active = true
	faults.Faults = map[string]sdk.FaultSpec{
		"get": {ErrorRate: 1.0, Error: "chaos: api server error"},
	}

	err = chaosClient.Get(context.Background(), client.ObjectKey{Name: "default"}, ns)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "chaos: api server error")
}
```

**Step 2: Run full test suite**

Run: `cd /Users/ugogiordano/workdir/rhoai/opendatahub-io/odh-platform-chaos && go test ./... -short -count=1`
Expected: ALL PASS

**Step 3: Commit**

```bash
git add tests/integration/experiment_test.go
git commit -m "test: add ChaosClient integration test"
```

---

## Summary

| Part | Tasks | Focus |
|------|-------|-------|
| A | 1-8 | Critical Phase 1 fixes from architect review |
| B | 9-11 | Safety mechanisms (TTL, distributed locking, clean expansion) |
| C | 12-14 | Advanced injectors (WebhookDisrupt, RBACRevoke, FinalizerBlock) |
| D | 15-17 | Chaos SDK core (FaultConfig, ChaosClient, WrapReconciler) |
| E | 18-20 | ConfigMap activation, HTTP endpoint, Go test helper |
| F | 21-26 | Phase 2 injection types and final integration |

**Total: 26 tasks**

Design doc milestones covered:
- **M9**: Tasks 15-17 (SDK core, ChaosClient, WrapReconciler)
- **M10**: Tasks 18-20 (ConfigMap activation, HTTP endpoint, test helper)
- **M11**: Tasks 21-24 (Application, Timing, K8s, Webhook fault categories)
- **M12**: Tasks 4, 7, 9-11 (TTL, distributed locking, signal handling, timeouts)
- **M13**: Tasks 12-14 (WebhookDisrupt, RBACRevoke, FinalizerBlock)

Critical Phase 1 fixes addressed: Tasks 1-8 (logging, dry-run verdict, ForbiddenResources, signal handling, Observer error return, CRDMutation cleanup, timeouts, file size limits)
