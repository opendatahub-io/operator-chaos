# ODH Chaos Dashboard Design Spec

## Overview

A standalone web dashboard for visualizing chaos experiment results, live monitoring, and operator resilience insights for the odh-platform-chaos project. The dashboard reads ChaosExperiment CRs from the Kubernetes API and persists historical data in SQLite.

## Goals

1. Provide at-a-glance cluster-wide resilience health
2. Enable live monitoring of running experiments with phase-by-phase progress
3. Allow filtering, sorting, and drilling into experiment history
4. Visualize operator dependency graphs with chaos coverage overlays
5. Support suite-based runs with version-to-version comparison
6. Deploy standalone (separate binary from the controller)

## Non-Goals

- Triggering, creating, or mutating experiments from the dashboard (strictly read-only in v1; no re-run, delete, or abort actions)
- Multi-cluster federation
- User authentication (relies on cluster RBAC / proxy)

## Architecture

### Deployment Model

```
                  +-----------+
                  | K8s API   |
                  | (watch)   |
                  +-----+-----+
                        |
               +--------v--------+
               |  Dashboard      |
               |  Backend (Go)   |
               |                 |
               |  - REST API     |
               |  - K8s watcher  |
               |  - SQLite (WAL) |
               +--------+--------+
                        |
               +--------v--------+
               |  Dashboard      |
               |  Frontend       |
               |  (React + PF5)  |
               +-----------------+
```

The dashboard runs as a single Go binary that:
- Watches ChaosExperiment CRs via the Kubernetes API (informer/watch)
- Persists experiment snapshots to SQLite for historical queries
- Serves a REST API consumed by the React frontend
- Serves the static React build at `/`
- Frontend and backend are same-origin (Go embed), so no CORS configuration is needed

### Tech Stack

| Layer    | Technology                          |
|----------|-------------------------------------|
| Frontend | React 18, TypeScript, PatternFly 5  |
| Backend  | Go, `net/http`, `k8s.io/client-go`  |
| Storage  | SQLite (via `modernc.org/sqlite`)   |
| Build    | Vite (frontend), Go embed (serve)   |

### SQLite Configuration

The database must be opened in WAL (Write-Ahead Logging) mode to support concurrent reads (SSE connections, API requests) while the K8s watcher writes snapshots:

```go
db.Exec("PRAGMA journal_mode=WAL")
db.Exec("PRAGMA busy_timeout=5000")
```

### Data Flow

1. **Live data**: K8s informer watches `ChaosExperiment` CRs. Changes are pushed to connected frontend clients via Server-Sent Events (SSE). SSE is chosen over WebSockets because the dashboard is read-only (unidirectional data flow) and SSE has simpler reconnection semantics.
2. **Historical data**: On every status update, the backend upserts the experiment into SQLite. This captures the full lifecycle, not just terminal states.
3. **Knowledge model**: The backend reads `OperatorKnowledge` YAML files (already defined in the project) to populate dependency graphs. Node types are derived from the YAML `Kind` field, not hardcoded.

## Frontend Views

Seven views organized into three navigation groups.

### UX States

All views implement four standard states:
- **Loading**: PatternFly Spinner centered in the content area
- **Empty**: PatternFly EmptyState with descriptive text and call-to-action (e.g., "No experiments found. Adjust filters or run your first experiment.")
- **Error**: PatternFly Alert (danger variant) with error message and retry action
- **SSE reconnection**: When the live SSE connection drops, show a dismissible warning banner: "Live updates disconnected. Reconnecting..."

### Monitor

#### Overview (`/`)
- 5 summary stat cards: Total, Resilient, Degraded, Failed, Running
- Trend indicators comparing to previous period
- Operator health summary with stacked health bars
- Verdict trend sparkline (30-day)
- Average recovery time by injection type
- Running experiments panel
- Recent experiments table

#### Live (`/live`)
- Real-time cards for each running experiment
- Phase stepper with display name mapping:

  | CRD Phase        | Display Label |
  |------------------|---------------|
  | Pending          | Pending       |
  | SteadyStatePre   | Pre-check     |
  | Injecting        | Injecting     |
  | Observing        | Observing     |
  | SteadyStatePost  | Post-check    |
  | Evaluating       | Evaluating    |
  | Complete         | Complete      |
  | Aborted          | Aborted       |

  The stepper shows all 7 non-terminal phases. If the experiment is `Aborted`, the stepper marks the current phase as failed (red) and skips remaining phases.
- Animated progress indicators (pulsing dots, countdowns)
- Live event log per experiment
- Progress metadata: elapsed time, reconcile cycles, target pod
- Note: No abort action in v1 (read-only)

### Experiments

#### All Experiments (`/experiments`)
- Filterable, sortable table with columns: Name, Operator, Component, Type, Phase, Verdict, Recovery, Date
- Toolbar dropdown filters: Namespace, Operator, Component, Type, Verdict, Phase, Time range
- Active filter chips with clear-all
- Search by name
- Pagination with configurable page size
- Bulk selection via checkboxes (for future actions, no-op in v1)
- Sortable columns: Name, Operator, Component, Type, Verdict

#### Experiment Detail (`/experiments/:namespace/:name`)
- Header with experiment name, verdict badge, phase badge, danger level
- Action button: Export YAML (only non-mutating action in v1)
- Status message banner (info/error/warning). Shows `cleanupError` as a danger banner when present.
- 7 tabs:
  - **Summary**: Key-value metadata (operator, component, type, recovery time, hypothesis, blast radius including maxPodsAffected, allowedNamespaces, forbiddenResources, allowDangerous, dryRun)
  - **Evaluation**: Verdict, confidence, recovery time, reconcile cycles, deviations list
  - **Steady State**: Pre-check and post-check results with pass/fail per check, value, and error fields
  - **Injection Log**: Timestamped inject/revert events with target and details. Shows injection duration (injectionStartedAt to first post-injection event).
  - **Conditions**: Status conditions table (type, status, reason, message, last transition)
  - **YAML**: Full CR YAML with syntax highlighting, copy and download buttons
  - **Debug**: observedGeneration, cleanupError, raw status JSON (collapsed by default)

#### Suites (`/suites`)
- Suite run history cards with summary stats (Resilient/Degraded/Failed counts)
- Stacked progress bar per suite
- Expandable experiment table per suite run
- Version-to-version comparison table with delta indicators (improved/regressed/no change)
- Compare selector dropdowns for version A vs version B

### Insights

#### Operators (`/operators`)
- Per-operator cards with health bar and verdict counts
- Expandable component accordion per operator
- Injection coverage matrix per component (8 injection types x pass/warn/fail/untested)
- Recent experiment history per component with links to detail view

#### Knowledge (`/knowledge`)
- Interactive SVG dependency graph per operator/component
- Node types are dynamic, derived from the `Kind` field in OperatorKnowledge YAML files. Common types include Deployment, ServiceAccount, ClusterRole, ClusterRoleBinding, ConfigMap, ValidatingWebhookConfig, MutatingWebhookConfig, CRD, Service.
- Nodes colored by chaos coverage status (tested-resilient, tested-degraded, tested-failed, not-tested)
- Experiment count badges on nodes
- Side panel showing: component info, managed resources list with coverage tags, chaos coverage summary
- Zoom/pan controls
- Legend

## Data Model

### ChaosExperiment CR (source of truth)

The dashboard reads the existing `ChaosExperiment` CRD (api/v1alpha1/types.go):

- **Spec**: target (operator, component, resource), steadyState (checks, timeout), injection (type, parameters, count, ttl, dangerLevel), blastRadius (maxPodsAffected, allowedNamespaces, forbiddenResources, allowDangerous, dryRun), hypothesis
- **Status**: phase, verdict, message, observedGeneration, startTime, endTime, injectionStartedAt, steadyStatePre, steadyStatePost, injectionLog, evaluationResult, cleanupError, conditions

Key enums consumed by the dashboard:
- **InjectionType**: PodKill, NetworkPartition, CRDMutation, ConfigDrift, WebhookDisrupt, RBACRevoke, FinalizerBlock, ClientFault
- **ExperimentPhase**: Pending, SteadyStatePre, Injecting, Observing, SteadyStatePost, Evaluating, Complete, Aborted
- **Verdict**: Resilient, Degraded, Failed, Inconclusive
- **DangerLevel**: low, medium, high

### Suite Grouping via Well-Known Labels

Suites are identified by well-known labels on `ChaosExperiment` CRs. The CLI suite runner must set these labels when creating experiments as part of a suite run:

| Label | Description | Example |
|-------|-------------|---------|
| `chaos.opendatahub.io/suite-name` | Name of the suite definition | `omc-full-suite` |
| `chaos.opendatahub.io/suite-run-id` | Unique ID for this suite execution | `run-20260325-090000` |
| `chaos.opendatahub.io/operator-version` | Version of the operator under test | `v2.10.0` |

The dashboard groups experiments into suite runs by querying for experiments sharing the same `suite-run-id` label. Version comparison works by finding suite runs with the same `suite-name` but different `operator-version` labels.

This approach requires no new CRD and works with any experiment, whether created by the CLI suite runner or manually labeled.

### SQLite Schema (historical persistence)

Schema versioning: migrations are embedded Go files, applied sequentially on startup. A `schema_version` table tracks the current version.

```sql
CREATE TABLE schema_version (
    version     INTEGER PRIMARY KEY,
    applied_at  TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE experiments (
    id              TEXT PRIMARY KEY,  -- {namespace}/{name}/{startTime}
    name            TEXT NOT NULL,
    namespace       TEXT NOT NULL,
    operator        TEXT NOT NULL,
    component       TEXT NOT NULL,
    injection_type  TEXT NOT NULL,
    phase           TEXT NOT NULL,
    verdict         TEXT,
    danger_level    TEXT,
    recovery_ms     INTEGER,          -- milliseconds, NULL if not recovered
    start_time      TEXT,
    end_time        TEXT,
    suite_name      TEXT,             -- from label, nullable
    suite_run_id    TEXT,             -- from label, nullable
    operator_version TEXT,            -- from label, nullable
    cleanup_error   TEXT,
    spec_json       TEXT NOT NULL,
    status_json     TEXT NOT NULL,
    created_at      TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at      TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX idx_experiments_namespace ON experiments(namespace);
CREATE INDEX idx_experiments_operator ON experiments(operator);
CREATE INDEX idx_experiments_component ON experiments(component);
CREATE INDEX idx_experiments_verdict ON experiments(verdict);
CREATE INDEX idx_experiments_phase ON experiments(phase);
CREATE INDEX idx_experiments_injection_type ON experiments(injection_type);
CREATE INDEX idx_experiments_start_time ON experiments(start_time);
CREATE INDEX idx_experiments_suite_run_id ON experiments(suite_run_id);
CREATE INDEX idx_experiments_suite_name ON experiments(suite_name);

CREATE UNIQUE INDEX idx_experiments_natural_key ON experiments(namespace, name, start_time);
```

Note: The composite primary key `{namespace}/{name}/{startTime}` ensures that re-running an experiment with the same name creates a new row rather than overwriting history. If `startTime` is not yet set (Pending phase), use `metadata.creationTimestamp` as the fallback. The `recovery_ms` column is derived by parsing the Go duration string from `EvaluationSummary.RecoveryTime` (e.g., `"45s"` → `45000`). The `suite_runs` table is eliminated in favor of label-based grouping queries.

## REST API

All endpoints prefixed with `/api/v1/`. All endpoints are GET-only (read-only dashboard). Input parameters are validated against known enum values and sanitized to prevent SQL injection (parameterized queries only).

| Method | Path                          | Description                          |
|--------|-------------------------------|--------------------------------------|
| GET    | `/experiments`                | List experiments (with filters)      |
| GET    | `/experiments/:namespace/:name` | Get single experiment (latest run) |
| GET    | `/experiments/live`           | SSE stream of running experiments    |
| GET    | `/overview/stats`             | Aggregated stats for overview        |
| GET    | `/operators`                  | List operators with health summaries |
| GET    | `/operators/:name/components` | Components for an operator           |
| GET    | `/knowledge/:operator/:component` | Dependency graph data           |
| GET    | `/suites`                     | List suite runs (grouped by label)   |
| GET    | `/suites/:runId`              | Suite run detail                     |
| GET    | `/suites/compare`             | Version comparison                   |

### Query Parameters (GET /experiments)

| Param      | Type   | Description                           |
|------------|--------|---------------------------------------|
| namespace  | string | Filter by namespace                   |
| operator   | string | Filter by operator name               |
| component  | string | Filter by component name              |
| type       | string | Filter by injection type              |
| verdict    | string | Filter by verdict                     |
| phase      | string | Filter by phase                       |
| search     | string | Name substring search                 |
| since      | string | ISO 8601 datetime or duration (24h)   |
| sort       | string | Sort field (name, date, recovery)     |
| order      | string | asc or desc                           |
| page       | int    | Page number (1-based)                 |
| pageSize   | int    | Items per page (default 10)           |

### Response Schema (GET /overview/stats)

```json
{
  "total": 30,
  "resilient": 23,
  "degraded": 4,
  "failed": 1,
  "inconclusive": 0,
  "running": 2,
  "trends": {
    "total": 5,
    "resilient": 3,
    "degraded": 1,
    "failed": -1
  },
  "verdictTimeline": [
    { "date": "2026-03-01", "resilient": 3, "degraded": 1, "failed": 0 }
  ],
  "avgRecoveryByType": {
    "PodKill": 12000,
    "ConfigDrift": 28000,
    "RBACRevoke": 45000
  },
  "runningExperiments": [
    {
      "name": "omc-configdrift",
      "namespace": "opendatahub",
      "phase": "Observing",
      "component": "odh-model-controller",
      "type": "ConfigDrift"
    }
  ]
}
```

## UI Design Tokens

Consistent across all views:

| Element          | Value                                        |
|------------------|----------------------------------------------|
| Sidebar bg       | `#212427`                                    |
| Active nav       | `border-left: 3px solid #06c`               |
| Card shadow      | `0 1px 2px rgba(0,0,0,0.08)`                |
| Badge font-size  | `11px`                                       |
| Button padding   | `8px 16px`, font-size `13px`                 |
| Resilient badge  | bg `#e6f9e6`, color `#1e4f18`                |
| Degraded badge   | bg `#fef3cd`, color `#795600`                |
| Failed badge     | bg `#fce8e6`, color `#7d1007`                |
| Running badge    | bg `#e7f1fa`, color `#004080`                |
| Complete badge   | bg `#e8e8e8`, color `#151515`                |
| Aborted badge    | bg `#f0f0f0`, color `#6a6e73`                |
| Inconclusive     | bg `#f5f0ff`, color `#6753ac` (purple)       |
| Pending badge    | bg `#f0f0f0`, color `#6a6e73` (neutral gray) |
| Primary action   | `#06c` (blue)                                |
| Danger action    | `#c9190b` (red, for destructive actions only)|
| Trend up (good)  | `#3e8635` (green)                            |
| Trend down (bad) | `#c9190b` (red)                              |

## Mockups

HTML mockups for all 7 views are available at:
`.superpowers/brainstorm/80649-1774450280/`

- `overview.html` - Overview dashboard
- `live.html` - Live monitoring
- `experiments-list.html` - All Experiments table
- `experiment-detail.html` - Experiment detail with tabs
- `suites.html` - Suites and version comparison
- `operators.html` - Operator resilience insights
- `knowledge.html` - Knowledge dependency graph

## Project Structure

```
dashboard/
  cmd/
    dashboard/
      main.go              # Entry point, flags, server setup
  internal/
    api/
      handler.go           # HTTP handlers
      routes.go            # Router setup
      sse.go               # Server-Sent Events for live updates
    store/
      sqlite.go            # SQLite operations
      migrations/          # Embedded SQL migration files
        001_initial.sql
      migrate.go           # Migration runner with schema_version tracking
    watcher/
      informer.go          # K8s ChaosExperiment informer
      snapshot.go          # CR -> SQLite snapshot logic
    knowledge/
      graph.go             # Dependency graph builder
  ui/
    src/
      components/          # React components
      pages/               # Route-level page components
      api/                 # API client hooks
      types/               # TypeScript types mirroring CRD
    package.json
    vite.config.ts
  embed.go                 # go:embed for built UI assets
```

## Testing Strategy

- **Backend**: Go unit tests for store, watcher, and API handlers
- **Frontend**: Vitest + React Testing Library for component tests
- **Integration**: Test against a real K8s cluster with ChaosExperiment CRs
- **E2E**: Playwright for critical user flows (filter, drill-down, live monitoring)
