# Chaos Dashboard Backend Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build the Go backend that watches ChaosExperiment CRs, persists them to SQLite, and serves a read-only REST API with SSE live streaming.

**Architecture:** Single Go binary under `dashboard/` serving a REST API backed by SQLite (WAL mode). A K8s informer watches ChaosExperiment CRs and upserts them into SQLite. SSE endpoint streams live updates. The frontend plan (separate document) will build the React UI that consumes this API.

**Tech Stack:** Go 1.25, `k8s.io/client-go`, `modernc.org/sqlite`, `net/http`, `encoding/json`

**Spec:** `docs/superpowers/specs/2026-03-25-chaos-dashboard-design.md`

**Module:** `github.com/opendatahub-io/odh-platform-chaos`

---

## File Structure

```
dashboard/
  cmd/dashboard/main.go           -- Entry point, flags, wiring
  internal/
    store/
      store.go                    -- Store interface
      sqlite.go                   -- SQLite implementation
      sqlite_test.go              -- Store tests
      migrate.go                  -- Migration runner
      migrate_test.go             -- Migration tests
      migrations/
        001_initial.sql           -- Initial schema
    watcher/
      watcher.go                  -- K8s informer + snapshot logic
      watcher_test.go             -- Watcher tests (with fake client)
    api/
      server.go                   -- HTTP server, router, middleware
      handler_experiments.go      -- /experiments endpoints
      handler_experiments_test.go -- Experiments handler tests
      handler_overview.go         -- /overview/stats endpoint
      handler_overview_test.go    -- Overview handler tests
      handler_operators.go        -- /operators endpoints
      handler_operators_test.go   -- Operators handler tests
      handler_knowledge.go        -- /knowledge endpoint
      handler_knowledge_test.go   -- Knowledge handler tests
      handler_suites.go           -- /suites endpoints
      handler_suites_test.go      -- Suites handler tests
      sse.go                      -- SSE live streaming
      sse_test.go                 -- SSE tests
    convert/
      convert.go                  -- ChaosExperiment CR -> store model conversion
      convert_test.go             -- Conversion tests
```

---

### Task 1: SQLite Schema and Migration Runner

**Files:**
- Create: `dashboard/internal/store/migrations/001_initial.sql`
- Create: `dashboard/internal/store/migrate.go`
- Create: `dashboard/internal/store/migrate_test.go`

- [ ] **Step 1: Write the SQL migration file**

```sql
-- dashboard/internal/store/migrations/001_initial.sql
-- Note: schema_version table is created by the migration runner itself (migrate.go),
-- so it is NOT included here. Only application tables go in migration files.

CREATE TABLE IF NOT EXISTS experiments (
    id              TEXT PRIMARY KEY,
    name            TEXT NOT NULL,
    namespace       TEXT NOT NULL,
    operator        TEXT NOT NULL,
    component       TEXT NOT NULL,
    injection_type  TEXT NOT NULL,
    phase           TEXT NOT NULL,
    verdict         TEXT,
    danger_level    TEXT,
    recovery_ms     INTEGER,
    start_time      TEXT,
    end_time        TEXT,
    suite_name      TEXT,
    suite_run_id    TEXT,
    operator_version TEXT,
    cleanup_error   TEXT,
    spec_json       TEXT NOT NULL,
    status_json     TEXT NOT NULL,
    created_at      TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at      TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX IF NOT EXISTS idx_experiments_namespace ON experiments(namespace);
CREATE INDEX IF NOT EXISTS idx_experiments_operator ON experiments(operator);
CREATE INDEX IF NOT EXISTS idx_experiments_component ON experiments(component);
CREATE INDEX IF NOT EXISTS idx_experiments_verdict ON experiments(verdict);
CREATE INDEX IF NOT EXISTS idx_experiments_phase ON experiments(phase);
CREATE INDEX IF NOT EXISTS idx_experiments_injection_type ON experiments(injection_type);
CREATE INDEX IF NOT EXISTS idx_experiments_start_time ON experiments(start_time);
CREATE INDEX IF NOT EXISTS idx_experiments_suite_run_id ON experiments(suite_run_id);
CREATE INDEX IF NOT EXISTS idx_experiments_suite_name ON experiments(suite_name);
CREATE UNIQUE INDEX IF NOT EXISTS idx_experiments_natural_key ON experiments(namespace, name, start_time);
```

- [ ] **Step 2: Add sqlite dependency (needed before tests can compile)**

Run: `cd /Users/ugogiordano/workdir/rhoai/opendatahub-io/odh-platform-chaos && go get modernc.org/sqlite && go mod tidy`

- [ ] **Step 3: Write the failing test for migration runner**

```go
// dashboard/internal/store/migrate_test.go
package store

import (
	"database/sql"
	"testing"

	_ "modernc.org/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMigrate_AppliesInitialSchema(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	require.NoError(t, err)
	defer db.Close()

	err = Migrate(db)
	require.NoError(t, err)

	// Verify experiments table exists
	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM experiments").Scan(&count)
	assert.NoError(t, err)
	assert.Equal(t, 0, count)

	// Verify schema_version was recorded
	var version int
	err = db.QueryRow("SELECT MAX(version) FROM schema_version").Scan(&version)
	assert.NoError(t, err)
	assert.Equal(t, 1, version)
}

func TestMigrate_IsIdempotent(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	require.NoError(t, err)
	defer db.Close()

	require.NoError(t, Migrate(db))
	require.NoError(t, Migrate(db))

	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM schema_version").Scan(&count)
	assert.NoError(t, err)
	assert.Equal(t, 1, count)
}
```

- [ ] **Step 4: Run test to verify it fails**

Run: `cd /Users/ugogiordano/workdir/rhoai/opendatahub-io/odh-platform-chaos && go test ./dashboard/internal/store/ -run TestMigrate -v`
Expected: FAIL with "Migrate not defined"

- [ ] **Step 5: Implement migration runner**

```go
// dashboard/internal/store/migrate.go
package store

import (
	"database/sql"
	"embed"
	"fmt"
	"sort"
	"strings"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

// Migrate applies all pending SQL migrations to the database.
func Migrate(db *sql.DB) error {
	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS schema_version (
		version    INTEGER PRIMARY KEY,
		applied_at TEXT NOT NULL DEFAULT (datetime('now'))
	)`); err != nil {
		return fmt.Errorf("creating schema_version table: %w", err)
	}

	var current int
	row := db.QueryRow("SELECT COALESCE(MAX(version), 0) FROM schema_version")
	if err := row.Scan(&current); err != nil {
		return fmt.Errorf("reading schema version: %w", err)
	}

	entries, err := migrationsFS.ReadDir("migrations")
	if err != nil {
		return fmt.Errorf("reading migrations dir: %w", err)
	}

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Name() < entries[j].Name()
	})

	for _, entry := range entries {
		name := entry.Name()
		if !strings.HasSuffix(name, ".sql") {
			continue
		}

		// Parse version from filename like "001_initial.sql"
		var version int
		if _, err := fmt.Sscanf(name, "%d_", &version); err != nil {
			return fmt.Errorf("parsing migration version from %s: %w", name, err)
		}

		if version <= current {
			continue
		}

		content, err := migrationsFS.ReadFile("migrations/" + name)
		if err != nil {
			return fmt.Errorf("reading migration %s: %w", name, err)
		}

		tx, err := db.Begin()
		if err != nil {
			return fmt.Errorf("beginning transaction for %s: %w", name, err)
		}

		if _, err := tx.Exec(string(content)); err != nil {
			tx.Rollback()
			return fmt.Errorf("executing migration %s: %w", name, err)
		}

		if _, err := tx.Exec("INSERT INTO schema_version (version) VALUES (?)", version); err != nil {
			tx.Rollback()
			return fmt.Errorf("recording version for %s: %w", name, err)
		}

		if err := tx.Commit(); err != nil {
			return fmt.Errorf("committing migration %s: %w", name, err)
		}
	}

	return nil
}
```

- [ ] **Step 6: Run test to verify it passes**

Run: `cd /Users/ugogiordano/workdir/rhoai/opendatahub-io/odh-platform-chaos && go test ./dashboard/internal/store/ -run TestMigrate -v`
Expected: PASS

- [ ] **Step 7: Commit**

```bash
git add dashboard/internal/store/migrations/ dashboard/internal/store/migrate.go dashboard/internal/store/migrate_test.go go.mod go.sum
git commit -m "feat(dashboard): add SQLite schema and migration runner"
```

---

### Task 2: Store Interface and SQLite Implementation (CRUD)

**Files:**
- Create: `dashboard/internal/store/store.go`
- Create: `dashboard/internal/store/sqlite.go`
- Create: `dashboard/internal/store/sqlite_test.go`

- [ ] **Step 1: Write the store interface and experiment model**

```go
// dashboard/internal/store/store.go
package store

import "time"

// Experiment is the stored representation of a ChaosExperiment.
type Experiment struct {
	ID              string
	Name            string
	Namespace       string
	Operator        string
	Component       string
	InjectionType   string
	Phase           string
	Verdict         string
	DangerLevel     string
	RecoveryMs      *int64
	StartTime       *time.Time
	EndTime         *time.Time
	SuiteName       string
	SuiteRunID      string
	OperatorVersion string
	CleanupError    string
	SpecJSON        string
	StatusJSON      string
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

// ListFilter defines query filters for listing experiments.
type ListFilter struct {
	Namespace  string
	Operator   string
	Component  string
	Type       string
	Verdict    string
	Phase      string
	Search     string
	Since      *time.Time
	Sort       string // name, date, recovery
	Order      string // asc, desc
	Page       int
	PageSize   int
}

// ListResult wraps a page of experiments with total count.
type ListResult struct {
	Items      []Experiment
	TotalCount int
}

// OverviewStats holds aggregated counts for the overview view.
type OverviewStats struct {
	Total        int
	Resilient    int
	Degraded     int
	Failed       int
	Inconclusive int
	Running      int
}

// RecoveryAvg holds average recovery time per injection type.
type RecoveryAvg struct {
	InjectionType string
	AvgMs         int64
}

// SuiteRun represents a grouped suite execution.
type SuiteRun struct {
	SuiteName       string
	SuiteRunID      string
	OperatorVersion string
	Total           int
	Resilient       int
	Degraded        int
	Failed          int
}

// Store defines the data access interface for dashboard persistence.
type Store interface {
	Upsert(exp Experiment) error
	Get(namespace, name string) (*Experiment, error)
	List(filter ListFilter) (ListResult, error)
	ListRunning() ([]Experiment, error)
	OverviewStats(since *time.Time) (OverviewStats, error)
	AvgRecoveryByType(since *time.Time) ([]RecoveryAvg, error)
	ListOperators(since *time.Time) ([]string, error)
	ListByOperator(operator string, since *time.Time) ([]Experiment, error)
	ListSuiteRuns() ([]SuiteRun, error)
	ListBySuiteRunID(runID string) ([]Experiment, error)
	CompareSuiteRuns(suiteNameA, runIDA, runIDB string) ([]Experiment, []Experiment, error)
	Close() error
}
```

- [ ] **Step 2: Write failing tests for SQLite store**

```go
// dashboard/internal/store/sqlite_test.go
package store

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestStore(t *testing.T) *SQLiteStore {
	t.Helper()
	s, err := NewSQLiteStore(":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { s.Close() })
	return s
}

func sampleExperiment(name string) Experiment {
	now := time.Now().UTC().Truncate(time.Millisecond)
	recoveryMs := int64(32000)
	return Experiment{
		ID:            "opendatahub/" + name + "/" + now.Format(time.RFC3339),
		Name:          name,
		Namespace:     "opendatahub",
		Operator:      "opendatahub-operator",
		Component:     "odh-model-controller",
		InjectionType: "PodKill",
		Phase:         "Complete",
		Verdict:       "Resilient",
		RecoveryMs:    &recoveryMs,
		StartTime:     &now,
		SpecJSON:      `{"target":{}}`,
		StatusJSON:    `{"phase":"Complete"}`,
	}
}

func TestSQLiteStore_UpsertAndGet(t *testing.T) {
	s := newTestStore(t)
	exp := sampleExperiment("omc-podkill")

	require.NoError(t, s.Upsert(exp))

	got, err := s.Get("opendatahub", "omc-podkill")
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, "omc-podkill", got.Name)
	assert.Equal(t, "Resilient", got.Verdict)
	assert.Equal(t, int64(32000), *got.RecoveryMs)
}

func TestSQLiteStore_UpsertUpdatesExisting(t *testing.T) {
	s := newTestStore(t)
	exp := sampleExperiment("omc-podkill")
	require.NoError(t, s.Upsert(exp))

	exp.Phase = "Aborted"
	exp.Verdict = "Inconclusive"
	require.NoError(t, s.Upsert(exp))

	got, err := s.Get("opendatahub", "omc-podkill")
	require.NoError(t, err)
	assert.Equal(t, "Aborted", got.Phase)
	assert.Equal(t, "Inconclusive", got.Verdict)
}

func TestSQLiteStore_GetReturnsNilForMissing(t *testing.T) {
	s := newTestStore(t)
	got, err := s.Get("opendatahub", "nonexistent")
	require.NoError(t, err)
	assert.Nil(t, got)
}

func TestSQLiteStore_ListWithFilters(t *testing.T) {
	s := newTestStore(t)

	exp1 := sampleExperiment("omc-podkill")
	exp2 := sampleExperiment("omc-configdrift")
	exp2.ID = "opendatahub/omc-configdrift/" + time.Now().Format(time.RFC3339)
	exp2.InjectionType = "ConfigDrift"
	exp2.Verdict = "Degraded"

	require.NoError(t, s.Upsert(exp1))
	require.NoError(t, s.Upsert(exp2))

	result, err := s.List(ListFilter{Verdict: "Resilient", Page: 1, PageSize: 10})
	require.NoError(t, err)
	assert.Equal(t, 1, result.TotalCount)
	assert.Equal(t, "omc-podkill", result.Items[0].Name)
}

func TestSQLiteStore_ListWithSearch(t *testing.T) {
	s := newTestStore(t)

	require.NoError(t, s.Upsert(sampleExperiment("omc-podkill")))
	exp2 := sampleExperiment("kserve-podkill")
	exp2.ID = "opendatahub/kserve-podkill/" + time.Now().Format(time.RFC3339)
	exp2.Operator = "kserve"
	require.NoError(t, s.Upsert(exp2))

	result, err := s.List(ListFilter{Search: "kserve", Page: 1, PageSize: 10})
	require.NoError(t, err)
	assert.Equal(t, 1, result.TotalCount)
	assert.Equal(t, "kserve-podkill", result.Items[0].Name)
}

func TestSQLiteStore_OverviewStats(t *testing.T) {
	s := newTestStore(t)

	exp1 := sampleExperiment("e1")
	exp1.Verdict = "Resilient"
	exp2 := sampleExperiment("e2")
	exp2.ID = "opendatahub/e2/" + time.Now().Format(time.RFC3339)
	exp2.Verdict = "Degraded"
	exp3 := sampleExperiment("e3")
	exp3.ID = "opendatahub/e3/" + time.Now().Format(time.RFC3339)
	exp3.Phase = "Observing"
	exp3.Verdict = ""

	require.NoError(t, s.Upsert(exp1))
	require.NoError(t, s.Upsert(exp2))
	require.NoError(t, s.Upsert(exp3))

	stats, err := s.OverviewStats(nil)
	require.NoError(t, err)
	assert.Equal(t, 3, stats.Total)
	assert.Equal(t, 1, stats.Resilient)
	assert.Equal(t, 1, stats.Degraded)
	assert.Equal(t, 1, stats.Running)
}
```

- [ ] **Step 3: Run tests to verify they fail**

Run: `cd /Users/ugogiordano/workdir/rhoai/opendatahub-io/odh-platform-chaos && go test ./dashboard/internal/store/ -run TestSQLiteStore -v`
Expected: FAIL with "NewSQLiteStore not defined"

- [ ] **Step 4: Implement SQLite store**

```go
// dashboard/internal/store/sqlite.go
package store

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

// SQLiteStore implements Store backed by a SQLite database.
type SQLiteStore struct {
	db *sql.DB
}

// NewSQLiteStore opens (or creates) a SQLite database and applies migrations.
func NewSQLiteStore(dsn string) (*SQLiteStore, error) {
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("opening sqlite: %w", err)
	}

	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		db.Close()
		return nil, fmt.Errorf("setting WAL mode: %w", err)
	}
	if _, err := db.Exec("PRAGMA busy_timeout=5000"); err != nil {
		db.Close()
		return nil, fmt.Errorf("setting busy timeout: %w", err)
	}

	if err := Migrate(db); err != nil {
		db.Close()
		return nil, fmt.Errorf("migrating: %w", err)
	}

	return &SQLiteStore{db: db}, nil
}

func (s *SQLiteStore) Close() error {
	return s.db.Close()
}

func (s *SQLiteStore) Upsert(exp Experiment) error {
	var startStr, endStr *string
	if exp.StartTime != nil {
		v := exp.StartTime.Format(time.RFC3339)
		startStr = &v
	}
	if exp.EndTime != nil {
		v := exp.EndTime.Format(time.RFC3339)
		endStr = &v
	}

	_, err := s.db.Exec(`
		INSERT INTO experiments (id, name, namespace, operator, component, injection_type, phase,
			verdict, danger_level, recovery_ms, start_time, end_time, suite_name, suite_run_id,
			operator_version, cleanup_error, spec_json, status_json)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			phase=excluded.phase, verdict=excluded.verdict, danger_level=excluded.danger_level,
			recovery_ms=excluded.recovery_ms, end_time=excluded.end_time,
			cleanup_error=excluded.cleanup_error, status_json=excluded.status_json,
			updated_at=datetime('now')`,
		exp.ID, exp.Name, exp.Namespace, exp.Operator, exp.Component, exp.InjectionType,
		exp.Phase, exp.Verdict, exp.DangerLevel, exp.RecoveryMs, startStr, endStr,
		exp.SuiteName, exp.SuiteRunID, exp.OperatorVersion, exp.CleanupError,
		exp.SpecJSON, exp.StatusJSON,
	)
	return err
}

func (s *SQLiteStore) Get(namespace, name string) (*Experiment, error) {
	row := s.db.QueryRow(`
		SELECT id, name, namespace, operator, component, injection_type, phase,
			verdict, danger_level, recovery_ms, start_time, end_time, suite_name,
			suite_run_id, operator_version, cleanup_error, spec_json, status_json,
			created_at, updated_at
		FROM experiments
		WHERE namespace=? AND name=?
		ORDER BY start_time DESC LIMIT 1`, namespace, name)

	return scanExperiment(row)
}

func (s *SQLiteStore) List(f ListFilter) (ListResult, error) {
	var where []string
	var args []interface{}

	if f.Namespace != "" {
		where = append(where, "namespace=?")
		args = append(args, f.Namespace)
	}
	if f.Operator != "" {
		where = append(where, "operator=?")
		args = append(args, f.Operator)
	}
	if f.Component != "" {
		where = append(where, "component=?")
		args = append(args, f.Component)
	}
	if f.Type != "" {
		where = append(where, "injection_type=?")
		args = append(args, f.Type)
	}
	if f.Verdict != "" {
		where = append(where, "verdict=?")
		args = append(args, f.Verdict)
	}
	if f.Phase != "" {
		where = append(where, "phase=?")
		args = append(args, f.Phase)
	}
	if f.Search != "" {
		where = append(where, "(name LIKE ? OR operator LIKE ? OR component LIKE ?)")
		args = append(args, "%"+f.Search+"%", "%"+f.Search+"%", "%"+f.Search+"%")
	}
	if f.Since != nil {
		where = append(where, "start_time >= ?")
		args = append(args, f.Since.Format(time.RFC3339))
	}

	whereClause := ""
	if len(where) > 0 {
		whereClause = "WHERE " + strings.Join(where, " AND ")
	}

	// Count
	var total int
	countQuery := "SELECT COUNT(*) FROM experiments " + whereClause
	if err := s.db.QueryRow(countQuery, args...).Scan(&total); err != nil {
		return ListResult{}, err
	}

	// Sort
	orderCol := "start_time"
	switch f.Sort {
	case "name":
		orderCol = "name"
	case "recovery":
		orderCol = "recovery_ms"
	}
	orderDir := "DESC"
	if f.Order == "asc" {
		orderDir = "ASC"
	}

	// Paginate
	page := f.Page
	if page < 1 {
		page = 1
	}
	pageSize := f.PageSize
	if pageSize < 1 {
		pageSize = 10
	}
	offset := (page - 1) * pageSize

	query := fmt.Sprintf(`
		SELECT id, name, namespace, operator, component, injection_type, phase,
			verdict, danger_level, recovery_ms, start_time, end_time, suite_name,
			suite_run_id, operator_version, cleanup_error, spec_json, status_json,
			created_at, updated_at
		FROM experiments %s
		ORDER BY %s %s
		LIMIT ? OFFSET ?`, whereClause, orderCol, orderDir)

	listArgs := append(args, pageSize, offset)
	rows, err := s.db.Query(query, listArgs...)
	if err != nil {
		return ListResult{}, err
	}
	defer rows.Close()

	var items []Experiment
	for rows.Next() {
		exp, err := scanExperimentRows(rows)
		if err != nil {
			return ListResult{}, err
		}
		items = append(items, *exp)
	}

	return ListResult{Items: items, TotalCount: total}, nil
}

// runningPhases are the CRD phases that indicate a running experiment.
var runningPhases = []string{"Pending", "SteadyStatePre", "Injecting", "Observing", "SteadyStatePost", "Evaluating"}

func (s *SQLiteStore) OverviewStats(since *time.Time) (OverviewStats, error) {
	whereClause := ""
	var args []interface{}
	if since != nil {
		whereClause = "WHERE start_time >= ?"
		args = append(args, since.Format(time.RFC3339))
	}

	var stats OverviewStats
	q := "SELECT COUNT(*) FROM experiments " + whereClause
	if err := s.db.QueryRow(q, args...).Scan(&stats.Total); err != nil {
		return stats, err
	}

	for _, v := range []struct {
		verdict string
		dest    *int
	}{
		{"Resilient", &stats.Resilient},
		{"Degraded", &stats.Degraded},
		{"Failed", &stats.Failed},
		{"Inconclusive", &stats.Inconclusive},
	} {
		vq := "SELECT COUNT(*) FROM experiments " + whereClause
		va := append([]interface{}{}, args...)
		if whereClause == "" {
			vq += " WHERE verdict=?"
		} else {
			vq += " AND verdict=?"
		}
		va = append(va, v.verdict)
		if err := s.db.QueryRow(vq, va...).Scan(v.dest); err != nil {
			return stats, err
		}
	}

	// Running = phases that are not terminal
	placeholders := make([]string, len(runningPhases))
	rArgs := append([]interface{}{}, args...)
	for i, p := range runningPhases {
		placeholders[i] = "?"
		rArgs = append(rArgs, p)
	}
	rq := "SELECT COUNT(*) FROM experiments "
	if whereClause == "" {
		rq += "WHERE phase IN (" + strings.Join(placeholders, ",") + ")"
	} else {
		rq += whereClause + " AND phase IN (" + strings.Join(placeholders, ",") + ")"
	}
	if err := s.db.QueryRow(rq, rArgs...).Scan(&stats.Running); err != nil {
		return stats, err
	}

	return stats, nil
}

func (s *SQLiteStore) AvgRecoveryByType(since *time.Time) ([]RecoveryAvg, error) {
	whereClause := "WHERE recovery_ms IS NOT NULL"
	var args []interface{}
	if since != nil {
		whereClause += " AND start_time >= ?"
		args = append(args, since.Format(time.RFC3339))
	}

	rows, err := s.db.Query(fmt.Sprintf(
		"SELECT injection_type, AVG(recovery_ms) FROM experiments %s GROUP BY injection_type ORDER BY injection_type",
		whereClause), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []RecoveryAvg
	for rows.Next() {
		var r RecoveryAvg
		if err := rows.Scan(&r.InjectionType, &r.AvgMs); err != nil {
			return nil, err
		}
		result = append(result, r)
	}
	return result, nil
}

func (s *SQLiteStore) ListOperators(since *time.Time) ([]string, error) {
	whereClause := ""
	var args []interface{}
	if since != nil {
		whereClause = "WHERE start_time >= ?"
		args = append(args, since.Format(time.RFC3339))
	}

	rows, err := s.db.Query("SELECT DISTINCT operator FROM experiments "+whereClause+" ORDER BY operator", args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var ops []string
	for rows.Next() {
		var op string
		if err := rows.Scan(&op); err != nil {
			return nil, err
		}
		ops = append(ops, op)
	}
	return ops, nil
}

func (s *SQLiteStore) ListByOperator(operator string, since *time.Time) ([]Experiment, error) {
	where := "WHERE operator=?"
	args := []interface{}{operator}
	if since != nil {
		where += " AND start_time >= ?"
		args = append(args, since.Format(time.RFC3339))
	}

	rows, err := s.db.Query(fmt.Sprintf(`
		SELECT id, name, namespace, operator, component, injection_type, phase,
			verdict, danger_level, recovery_ms, start_time, end_time, suite_name,
			suite_run_id, operator_version, cleanup_error, spec_json, status_json,
			created_at, updated_at
		FROM experiments %s ORDER BY start_time DESC`, where), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []Experiment
	for rows.Next() {
		exp, err := scanExperimentRows(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, *exp)
	}
	return items, nil
}

func (s *SQLiteStore) ListRunning() ([]Experiment, error) {
	placeholders := make([]string, len(runningPhases))
	args := make([]interface{}, len(runningPhases))
	for i, p := range runningPhases {
		placeholders[i] = "?"
		args[i] = p
	}

	rows, err := s.db.Query(fmt.Sprintf(`
		SELECT id, name, namespace, operator, component, injection_type, phase,
			verdict, danger_level, recovery_ms, start_time, end_time, suite_name,
			suite_run_id, operator_version, cleanup_error, spec_json, status_json,
			created_at, updated_at
		FROM experiments WHERE phase IN (%s) ORDER BY start_time DESC`,
		strings.Join(placeholders, ",")), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []Experiment
	for rows.Next() {
		exp, err := scanExperimentRows(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, *exp)
	}
	return items, nil
}

func (s *SQLiteStore) ListBySuiteRunID(runID string) ([]Experiment, error) {
	rows, err := s.db.Query(`
		SELECT id, name, namespace, operator, component, injection_type, phase,
			verdict, danger_level, recovery_ms, start_time, end_time, suite_name,
			suite_run_id, operator_version, cleanup_error, spec_json, status_json,
			created_at, updated_at
		FROM experiments WHERE suite_run_id=? ORDER BY name`, runID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []Experiment
	for rows.Next() {
		exp, err := scanExperimentRows(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, *exp)
	}
	return items, nil
}

func (s *SQLiteStore) ListSuiteRuns() ([]SuiteRun, error) {
	rows, err := s.db.Query(`
		SELECT suite_name, suite_run_id, operator_version,
			COUNT(*) as total,
			SUM(CASE WHEN verdict='Resilient' THEN 1 ELSE 0 END),
			SUM(CASE WHEN verdict='Degraded' THEN 1 ELSE 0 END),
			SUM(CASE WHEN verdict='Failed' THEN 1 ELSE 0 END)
		FROM experiments
		WHERE suite_run_id IS NOT NULL AND suite_run_id != ''
		GROUP BY suite_name, suite_run_id, operator_version
		ORDER BY MAX(start_time) DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var runs []SuiteRun
	for rows.Next() {
		var r SuiteRun
		if err := rows.Scan(&r.SuiteName, &r.SuiteRunID, &r.OperatorVersion,
			&r.Total, &r.Resilient, &r.Degraded, &r.Failed); err != nil {
			return nil, err
		}
		runs = append(runs, r)
	}
	return runs, nil
}

func (s *SQLiteStore) CompareSuiteRuns(suiteName, runIDA, runIDB string) ([]Experiment, []Experiment, error) {
	a, err := s.ListBySuiteRunID(runIDA)
	if err != nil {
		return nil, nil, err
	}
	b, err := s.ListBySuiteRunID(runIDB)
	if err != nil {
		return nil, nil, err
	}
	return a, b, nil
}

// scanner is an interface satisfied by both *sql.Row and *sql.Rows.
type scanner interface {
	Scan(dest ...interface{}) error
}

func scanExperimentFromScanner(s scanner) (*Experiment, error) {
	var exp Experiment
	var startStr, endStr, createdStr, updatedStr sql.NullString
	var recoveryMs sql.NullInt64
	var verdict, dangerLevel, suiteName, suiteRunID, opVersion, cleanupErr sql.NullString

	err := s.Scan(
		&exp.ID, &exp.Name, &exp.Namespace, &exp.Operator, &exp.Component,
		&exp.InjectionType, &exp.Phase, &verdict, &dangerLevel, &recoveryMs,
		&startStr, &endStr, &suiteName, &suiteRunID, &opVersion, &cleanupErr,
		&exp.SpecJSON, &exp.StatusJSON, &createdStr, &updatedStr,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}

	exp.Verdict = verdict.String
	exp.DangerLevel = dangerLevel.String
	exp.SuiteName = suiteName.String
	exp.SuiteRunID = suiteRunID.String
	exp.OperatorVersion = opVersion.String
	exp.CleanupError = cleanupErr.String

	if recoveryMs.Valid {
		exp.RecoveryMs = &recoveryMs.Int64
	}
	if startStr.Valid {
		if t, err := time.Parse(time.RFC3339, startStr.String); err == nil {
			exp.StartTime = &t
		}
	}
	if endStr.Valid {
		if t, err := time.Parse(time.RFC3339, endStr.String); err == nil {
			exp.EndTime = &t
		}
	}
	if createdStr.Valid {
		exp.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", createdStr.String)
	}
	if updatedStr.Valid {
		exp.UpdatedAt, _ = time.Parse("2006-01-02 15:04:05", updatedStr.String)
	}

	return &exp, nil
}

func scanExperiment(row *sql.Row) (*Experiment, error) {
	return scanExperimentFromScanner(row)
}

func scanExperimentRows(rows *sql.Rows) (*Experiment, error) {
	return scanExperimentFromScanner(rows)
}
```

- [ ] **Step 5: Run tests to verify they pass**

Run: `cd /Users/ugogiordano/workdir/rhoai/opendatahub-io/odh-platform-chaos && go test ./dashboard/internal/store/ -v`
Expected: ALL PASS

- [ ] **Step 6: Commit**

```bash
git add dashboard/internal/store/store.go dashboard/internal/store/sqlite.go dashboard/internal/store/sqlite_test.go
git commit -m "feat(dashboard): add SQLite store with CRUD, filtering, and stats"
```

---

### Task 3: CR-to-Store Conversion

**Files:**
- Create: `dashboard/internal/convert/convert.go`
- Create: `dashboard/internal/convert/convert_test.go`

- [ ] **Step 1: Write failing test**

```go
// dashboard/internal/convert/convert_test.go
package convert

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/opendatahub-io/odh-platform-chaos/api/v1alpha1"
	"github.com/opendatahub-io/odh-platform-chaos/dashboard/internal/store"
)

func TestFromCR(t *testing.T) {
	now := metav1.Now()
	recoveryTime := "45s"

	cr := &v1alpha1.ChaosExperiment{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "omc-podkill",
			Namespace:         "opendatahub",
			CreationTimestamp: now,
			Labels: map[string]string{
				"chaos.opendatahub.io/suite-name":        "omc-full-suite",
				"chaos.opendatahub.io/suite-run-id":      "run-123",
				"chaos.opendatahub.io/operator-version":  "v2.10.0",
			},
		},
		Spec: v1alpha1.ChaosExperimentSpec{
			Target: v1alpha1.TargetSpec{
				Operator:  "opendatahub-operator",
				Component: "odh-model-controller",
			},
			Injection: v1alpha1.InjectionSpec{
				Type:        v1alpha1.PodKill,
				DangerLevel: v1alpha1.DangerLevelLow,
			},
		},
		Status: v1alpha1.ChaosExperimentStatus{
			Phase:     v1alpha1.PhaseComplete,
			Verdict:   v1alpha1.Resilient,
			StartTime: &now,
			EvaluationResult: &v1alpha1.EvaluationSummary{
				Verdict:      v1alpha1.Resilient,
				RecoveryTime: recoveryTime,
			},
		},
	}

	exp, err := FromCR(cr)
	require.NoError(t, err)

	assert.Equal(t, "omc-podkill", exp.Name)
	assert.Equal(t, "opendatahub", exp.Namespace)
	assert.Equal(t, "opendatahub-operator", exp.Operator)
	assert.Equal(t, "odh-model-controller", exp.Component)
	assert.Equal(t, "PodKill", exp.InjectionType)
	assert.Equal(t, "Complete", exp.Phase)
	assert.Equal(t, "Resilient", exp.Verdict)
	assert.Equal(t, "low", exp.DangerLevel)
	require.NotNil(t, exp.RecoveryMs)
	assert.Equal(t, int64(45000), *exp.RecoveryMs)
	assert.Equal(t, "omc-full-suite", exp.SuiteName)
	assert.Equal(t, "run-123", exp.SuiteRunID)
	assert.Equal(t, "v2.10.0", exp.OperatorVersion)
	assert.Contains(t, exp.ID, "opendatahub/omc-podkill/")
}

func TestFromCR_NoStartTime_UsesCreationTimestamp(t *testing.T) {
	now := metav1.Now()
	cr := &v1alpha1.ChaosExperiment{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "test",
			Namespace:         "ns",
			CreationTimestamp: now,
		},
		Spec: v1alpha1.ChaosExperimentSpec{
			Target:    v1alpha1.TargetSpec{Operator: "op", Component: "comp"},
			Injection: v1alpha1.InjectionSpec{Type: v1alpha1.PodKill},
		},
		Status: v1alpha1.ChaosExperimentStatus{Phase: v1alpha1.PhasePending},
	}

	exp, err := FromCR(cr)
	require.NoError(t, err)
	assert.Contains(t, exp.ID, now.Time.Format(time.RFC3339))
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd /Users/ugogiordano/workdir/rhoai/opendatahub-io/odh-platform-chaos && go test ./dashboard/internal/convert/ -v`
Expected: FAIL

- [ ] **Step 3: Implement conversion**

```go
// dashboard/internal/convert/convert.go
package convert

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/opendatahub-io/odh-platform-chaos/api/v1alpha1"
	"github.com/opendatahub-io/odh-platform-chaos/dashboard/internal/store"
)

const (
	labelSuiteName       = "chaos.opendatahub.io/suite-name"
	labelSuiteRunID      = "chaos.opendatahub.io/suite-run-id"
	labelOperatorVersion = "chaos.opendatahub.io/operator-version"
)

// FromCR converts a ChaosExperiment CR to the store Experiment model.
func FromCR(cr *v1alpha1.ChaosExperiment) (*store.Experiment, error) {
	specJSON, err := json.Marshal(cr.Spec)
	if err != nil {
		return nil, fmt.Errorf("marshaling spec: %w", err)
	}
	statusJSON, err := json.Marshal(cr.Status)
	if err != nil {
		return nil, fmt.Errorf("marshaling status: %w", err)
	}

	// Use StartTime from status, fall back to CreationTimestamp
	startTime := cr.CreationTimestamp.Time
	if cr.Status.StartTime != nil {
		startTime = cr.Status.StartTime.Time
	}

	id := fmt.Sprintf("%s/%s/%s", cr.Namespace, cr.Name, startTime.Format(time.RFC3339))

	exp := &store.Experiment{
		ID:              id,
		Name:            cr.Name,
		Namespace:       cr.Namespace,
		Operator:        cr.Spec.Target.Operator,
		Component:       cr.Spec.Target.Component,
		InjectionType:   string(cr.Spec.Injection.Type),
		Phase:           string(cr.Status.Phase),
		Verdict:         string(cr.Status.Verdict),
		DangerLevel:     string(cr.Spec.Injection.DangerLevel),
		CleanupError:    cr.Status.CleanupError,
		SpecJSON:        string(specJSON),
		StatusJSON:      string(statusJSON),
		StartTime:       &startTime,
		SuiteName:       cr.Labels[labelSuiteName],
		SuiteRunID:      cr.Labels[labelSuiteRunID],
		OperatorVersion: cr.Labels[labelOperatorVersion],
	}

	if cr.Status.EndTime != nil {
		t := cr.Status.EndTime.Time
		exp.EndTime = &t
	}

	if cr.Status.EvaluationResult != nil && cr.Status.EvaluationResult.RecoveryTime != "" {
		d, err := time.ParseDuration(cr.Status.EvaluationResult.RecoveryTime)
		if err == nil {
			ms := d.Milliseconds()
			exp.RecoveryMs = &ms
		}
	}

	return exp, nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd /Users/ugogiordano/workdir/rhoai/opendatahub-io/odh-platform-chaos && go test ./dashboard/internal/convert/ -v`
Expected: ALL PASS

- [ ] **Step 5: Commit**

```bash
git add dashboard/internal/convert/
git commit -m "feat(dashboard): add ChaosExperiment CR to store model conversion"
```

---

### Task 4: K8s Watcher (Informer + Snapshot)

**Files:**
- Create: `dashboard/internal/watcher/watcher.go`
- Create: `dashboard/internal/watcher/watcher_test.go`

- [ ] **Step 1: Write failing test**

```go
// dashboard/internal/watcher/watcher_test.go
package watcher

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/opendatahub-io/odh-platform-chaos/api/v1alpha1"
	"github.com/opendatahub-io/odh-platform-chaos/dashboard/internal/store"
)

// fakeStore captures Upsert calls for test verification.
type fakeStore struct {
	upserted []store.Experiment
}

func (f *fakeStore) Upsert(exp store.Experiment) error {
	f.upserted = append(f.upserted, exp)
	return nil
}

func TestHandleCREvent_Upserts(t *testing.T) {
	fs := &fakeStore{}
	w := &Watcher{store: fs, broadcaster: nil}

	now := metav1.Now()
	cr := &v1alpha1.ChaosExperiment{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "test-exp",
			Namespace:         "opendatahub",
			CreationTimestamp: now,
		},
		Spec: v1alpha1.ChaosExperimentSpec{
			Target:    v1alpha1.TargetSpec{Operator: "op", Component: "comp"},
			Injection: v1alpha1.InjectionSpec{Type: v1alpha1.PodKill},
		},
		Status: v1alpha1.ChaosExperimentStatus{
			Phase: v1alpha1.PhaseObserving,
		},
	}

	err := w.handleCREvent(cr)
	require.NoError(t, err)
	require.Len(t, fs.upserted, 1)
	assert.Equal(t, "test-exp", fs.upserted[0].Name)
	assert.Equal(t, "Observing", fs.upserted[0].Phase)
}

func TestWatcher_InitialSync(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, v1alpha1.AddToScheme(scheme))

	now := metav1.Now()
	cr := &v1alpha1.ChaosExperiment{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "synced",
			Namespace:         "opendatahub",
			CreationTimestamp: now,
		},
		Spec: v1alpha1.ChaosExperimentSpec{
			Target:    v1alpha1.TargetSpec{Operator: "op", Component: "comp"},
			Injection: v1alpha1.InjectionSpec{Type: v1alpha1.ConfigDrift},
		},
		Status: v1alpha1.ChaosExperimentStatus{Phase: v1alpha1.PhaseComplete, Verdict: v1alpha1.Resilient},
	}

	client := fake.NewClientBuilder().WithScheme(scheme).WithObjects(cr).Build()
	fs := &fakeStore{}
	w := NewWatcher(client, fs, nil)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	err := w.SyncOnce(ctx)
	require.NoError(t, err)
	require.Len(t, fs.upserted, 1)
	assert.Equal(t, "synced", fs.upserted[0].Name)
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd /Users/ugogiordano/workdir/rhoai/opendatahub-io/odh-platform-chaos && go test ./dashboard/internal/watcher/ -v`
Expected: FAIL

- [ ] **Step 3: Implement watcher**

```go
// dashboard/internal/watcher/watcher.go
package watcher

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/opendatahub-io/odh-platform-chaos/api/v1alpha1"
	"github.com/opendatahub-io/odh-platform-chaos/dashboard/internal/convert"
	"github.com/opendatahub-io/odh-platform-chaos/dashboard/internal/store"
)

// Upserter is the subset of store.Store needed by the watcher.
type Upserter interface {
	Upsert(exp store.Experiment) error
}

// Broadcaster is called when an experiment changes state (for SSE integration).
type Broadcaster interface {
	Broadcast(data []byte)
}

// Watcher watches ChaosExperiment CRs and upserts them into the store.
type Watcher struct {
	client      client.Client
	store       Upserter
	broadcaster Broadcaster
}

// NewWatcher creates a new Watcher. broadcaster may be nil if SSE is not enabled.
func NewWatcher(c client.Client, s Upserter, b Broadcaster) *Watcher {
	return &Watcher{client: c, store: s, broadcaster: b}
}

// SyncOnce lists all ChaosExperiment CRs and upserts them into the store.
func (w *Watcher) SyncOnce(ctx context.Context) error {
	var list v1alpha1.ChaosExperimentList
	if err := w.client.List(ctx, &list); err != nil {
		return fmt.Errorf("listing ChaosExperiments: %w", err)
	}

	for i := range list.Items {
		if err := w.handleCREvent(&list.Items[i]); err != nil {
			log.Printf("error processing %s/%s: %v", list.Items[i].Namespace, list.Items[i].Name, err)
		}
	}
	return nil
}

func (w *Watcher) handleCREvent(cr *v1alpha1.ChaosExperiment) error {
	exp, err := convert.FromCR(cr)
	if err != nil {
		return fmt.Errorf("converting CR %s/%s: %w", cr.Namespace, cr.Name, err)
	}
	if err := w.store.Upsert(*exp); err != nil {
		return err
	}

	// Broadcast to SSE clients if a broadcaster is configured
	if w.broadcaster != nil {
		data, _ := json.Marshal(exp)
		w.broadcaster.Broadcast(data)
	}
	return nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd /Users/ugogiordano/workdir/rhoai/opendatahub-io/odh-platform-chaos && go test ./dashboard/internal/watcher/ -v`
Expected: ALL PASS

- [ ] **Step 5: Commit**

```bash
git add dashboard/internal/watcher/
git commit -m "feat(dashboard): add K8s ChaosExperiment watcher with initial sync"
```

---

### Task 5: SSE Live Streaming

**Files:**
- Create: `dashboard/internal/api/sse.go`
- Create: `dashboard/internal/api/sse_test.go`

- [ ] **Step 1: Write failing test**

```go
// dashboard/internal/api/sse_test.go
package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSSEBroker_ClientReceivesEvent(t *testing.T) {
	broker := NewSSEBroker()
	go broker.Run()
	defer broker.Stop()

	// Wait for broker to start
	time.Sleep(10 * time.Millisecond)

	done := make(chan string, 1)
	handler := broker.ServeHTTP

	req := httptest.NewRequest("GET", "/api/v1/experiments/live", nil)
	rec := &flushRecorder{ResponseRecorder: httptest.NewRecorder(), done: done}

	go handler(rec, req)

	// Wait for client registration
	time.Sleep(50 * time.Millisecond)

	broker.Broadcast([]byte(`{"name":"test","phase":"Observing"}`))

	select {
	case data := <-done:
		assert.Contains(t, data, `"name":"test"`)
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for SSE event")
	}
}

// flushRecorder captures SSE data on flush.
type flushRecorder struct {
	*httptest.ResponseRecorder
	done chan string
}

func (f *flushRecorder) Flush() {
	body := f.Body.String()
	if strings.Contains(body, "data:") {
		f.done <- body
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd /Users/ugogiordano/workdir/rhoai/opendatahub-io/odh-platform-chaos && go test ./dashboard/internal/api/ -run TestSSE -v`
Expected: FAIL

- [ ] **Step 3: Implement SSE broker**

```go
// dashboard/internal/api/sse.go
package api

import (
	"fmt"
	"net/http"
	"sync"
)

// SSEBroker manages SSE client connections and broadcasts events.
type SSEBroker struct {
	clients    map[chan []byte]struct{}
	register   chan chan []byte
	unregister chan chan []byte
	broadcast  chan []byte
	stop       chan struct{}
	mu         sync.RWMutex
}

// NewSSEBroker creates a new SSE broker.
func NewSSEBroker() *SSEBroker {
	return &SSEBroker{
		clients:    make(map[chan []byte]struct{}),
		register:   make(chan chan []byte),
		unregister: make(chan chan []byte),
		broadcast:  make(chan []byte, 64),
		stop:       make(chan struct{}),
	}
}

// Run starts the broker event loop. Call in a goroutine.
func (b *SSEBroker) Run() {
	for {
		select {
		case client := <-b.register:
			b.clients[client] = struct{}{}
		case client := <-b.unregister:
			delete(b.clients, client)
			close(client)
		case msg := <-b.broadcast:
			for client := range b.clients {
				select {
				case client <- msg:
				default:
					// Client buffer full, drop
				}
			}
		case <-b.stop:
			for client := range b.clients {
				close(client)
			}
			return
		}
	}
}

// Stop shuts down the broker.
func (b *SSEBroker) Stop() {
	close(b.stop)
}

// Broadcast sends data to all connected clients.
func (b *SSEBroker) Broadcast(data []byte) {
	b.broadcast <- data
}

// ServeHTTP handles SSE client connections.
func (b *SSEBroker) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	client := make(chan []byte, 16)
	b.register <- client

	defer func() {
		b.unregister <- client
	}()

	ctx := r.Context()
	for {
		select {
		case msg, ok := <-client:
			if !ok {
				return
			}
			fmt.Fprintf(w, "data: %s\n\n", msg)
			flusher.Flush()
		case <-ctx.Done():
			return
		}
	}
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd /Users/ugogiordano/workdir/rhoai/opendatahub-io/odh-platform-chaos && go test ./dashboard/internal/api/ -run TestSSE -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add dashboard/internal/api/sse.go dashboard/internal/api/sse_test.go
git commit -m "feat(dashboard): add SSE broker for live experiment streaming"
```

---

### Task 6: REST API Handlers - Experiments

**Files:**
- Create: `dashboard/internal/api/server.go`
- Create: `dashboard/internal/api/handler_experiments.go`
- Create: `dashboard/internal/api/handler_experiments_test.go`

- [ ] **Step 1: Write the server scaffold and experiments handler test**

```go
// dashboard/internal/api/handler_experiments_test.go
package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opendatahub-io/odh-platform-chaos/dashboard/internal/store"
)

type mockStore struct {
	experiments []store.Experiment
}

func (m *mockStore) List(f store.ListFilter) (store.ListResult, error) {
	var filtered []store.Experiment
	for _, e := range m.experiments {
		if f.Operator != "" && e.Operator != f.Operator {
			continue
		}
		if f.Verdict != "" && e.Verdict != f.Verdict {
			continue
		}
		filtered = append(filtered, e)
	}
	return store.ListResult{Items: filtered, TotalCount: len(filtered)}, nil
}

func (m *mockStore) Get(namespace, name string) (*store.Experiment, error) {
	for _, e := range m.experiments {
		if e.Namespace == namespace && e.Name == name {
			return &e, nil
		}
	}
	return nil, nil
}

func (m *mockStore) Upsert(exp store.Experiment) error                               { return nil }
func (m *mockStore) ListRunning() ([]store.Experiment, error)                          { return nil, nil }
func (m *mockStore) OverviewStats(since *time.Time) (store.OverviewStats, error)      { return store.OverviewStats{}, nil }
func (m *mockStore) AvgRecoveryByType(since *time.Time) ([]store.RecoveryAvg, error)  { return nil, nil }
func (m *mockStore) ListOperators(since *time.Time) ([]string, error)                 { return nil, nil }
func (m *mockStore) ListByOperator(op string, since *time.Time) ([]store.Experiment, error) { return nil, nil }
func (m *mockStore) ListSuiteRuns() ([]store.SuiteRun, error)                          { return nil, nil }
func (m *mockStore) ListBySuiteRunID(id string) ([]store.Experiment, error)           { return nil, nil }
func (m *mockStore) CompareSuiteRuns(sn, a, b string) ([]store.Experiment, []store.Experiment, error) { return nil, nil, nil }
func (m *mockStore) Close() error                                                      { return nil }

func TestHandleListExperiments(t *testing.T) {
	ms := &mockStore{experiments: []store.Experiment{
		{Name: "e1", Namespace: "ns", Operator: "op1", Verdict: "Resilient", SpecJSON: "{}", StatusJSON: "{}"},
		{Name: "e2", Namespace: "ns", Operator: "op2", Verdict: "Failed", SpecJSON: "{}", StatusJSON: "{}"},
	}}

	srv := NewServer(ms, nil, nil)
	req := httptest.NewRequest("GET", "/api/v1/experiments?operator=op1", nil)
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var result struct {
		Items      []json.RawMessage `json:"items"`
		TotalCount int               `json:"totalCount"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &result))
	assert.Equal(t, 1, result.TotalCount)
}

func TestHandleGetExperiment(t *testing.T) {
	ms := &mockStore{experiments: []store.Experiment{
		{Name: "e1", Namespace: "ns", Operator: "op1", SpecJSON: "{}", StatusJSON: "{}"},
	}}

	srv := NewServer(ms, nil, nil)
	req := httptest.NewRequest("GET", "/api/v1/experiments/ns/e1", nil)
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestHandleGetExperiment_NotFound(t *testing.T) {
	ms := &mockStore{}
	srv := NewServer(ms, nil, nil)
	req := httptest.NewRequest("GET", "/api/v1/experiments/ns/missing", nil)
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)

	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestSSERoute_DoesNotShadowExperimentGet(t *testing.T) {
	// Verify that GET /api/v1/experiments/live routes to SSE, not handleGetExperiment
	ms := &mockStore{experiments: []store.Experiment{
		{Name: "live", Namespace: "experiments", SpecJSON: "{}", StatusJSON: "{}"},
	}}
	broker := NewSSEBroker()
	go broker.Run()
	defer broker.Stop()

	srv := NewServer(ms, broker, nil)
	req := httptest.NewRequest("GET", "/api/v1/experiments/live", nil)
	rec := httptest.NewRecorder()
	// SSE handler will set Content-Type to text/event-stream, not application/json
	go srv.Handler().ServeHTTP(rec, req)
	time.Sleep(50 * time.Millisecond)

	assert.Equal(t, "text/event-stream", rec.Header().Get("Content-Type"))
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd /Users/ugogiordano/workdir/rhoai/opendatahub-io/odh-platform-chaos && go test ./dashboard/internal/api/ -run TestHandle -v`
Expected: FAIL

- [ ] **Step 3: Implement server and experiments handler**

```go
// dashboard/internal/api/server.go
package api

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/opendatahub-io/odh-platform-chaos/dashboard/internal/store"
	"github.com/opendatahub-io/odh-platform-chaos/pkg/model"
)

// Server is the dashboard HTTP server.
type Server struct {
	store     store.Store
	broker    *SSEBroker
	knowledge []model.OperatorKnowledge
	mux       *http.ServeMux
}

// NewServer creates a new dashboard server.
func NewServer(s store.Store, broker *SSEBroker, knowledge []model.OperatorKnowledge) *Server {
	srv := &Server{store: s, broker: broker, knowledge: knowledge}
	srv.mux = http.NewServeMux()
	srv.registerRoutes()
	return srv
}

// Handler returns the HTTP handler.
func (s *Server) Handler() http.Handler {
	return s.mux
}

func (s *Server) registerRoutes() {
	// SSE live endpoint registered first. Go 1.22+ ServeMux uses most-specific-match,
	// so the literal "/experiments/live" always wins over the wildcard "{namespace}/{name}".
	if s.broker != nil {
		s.mux.HandleFunc("GET /api/v1/experiments/live", s.broker.ServeHTTP)
	}
	s.mux.HandleFunc("GET /api/v1/experiments", s.handleListExperiments)
	s.mux.HandleFunc("GET /api/v1/experiments/{namespace}/{name}", s.handleGetExperiment)
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

// pathSegment extracts a named segment from the URL path.
// For Go 1.22+ ServeMux patterns like "GET /api/v1/experiments/{namespace}/{name}".
func pathSegment(r *http.Request, name string) string {
	return r.PathValue(name)
}
```

```go
// dashboard/internal/api/handler_experiments.go
package api

import (
	"net/http"
	"strconv"
	"time"

	"github.com/opendatahub-io/odh-platform-chaos/dashboard/internal/store"
)

func (s *Server) handleListExperiments(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

	filter := store.ListFilter{
		Namespace: q.Get("namespace"),
		Operator:  q.Get("operator"),
		Component: q.Get("component"),
		Type:      q.Get("type"),
		Verdict:   q.Get("verdict"),
		Phase:     q.Get("phase"),
		Search:    q.Get("search"),
		Sort:      q.Get("sort"),
		Order:     q.Get("order"),
		Page:      1,
		PageSize:  10,
	}

	if v := q.Get("page"); v != "" {
		if p, err := strconv.Atoi(v); err == nil {
			filter.Page = p
		}
	}
	if v := q.Get("pageSize"); v != "" {
		if ps, err := strconv.Atoi(v); err == nil {
			filter.PageSize = ps
		}
	}
	if v := q.Get("since"); v != "" {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			filter.Since = &t
		} else if d, err := time.ParseDuration(v); err == nil {
			t := time.Now().Add(-d)
			filter.Since = &t
		}
	}

	result, err := s.store.List(filter)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"items":      result.Items,
		"totalCount": result.TotalCount,
	})
}

func (s *Server) handleGetExperiment(w http.ResponseWriter, r *http.Request) {
	namespace := pathSegment(r, "namespace")
	name := pathSegment(r, "name")

	exp, err := s.store.Get(namespace, name)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if exp == nil {
		writeError(w, http.StatusNotFound, "experiment not found")
		return
	}

	writeJSON(w, http.StatusOK, exp)
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd /Users/ugogiordano/workdir/rhoai/opendatahub-io/odh-platform-chaos && go test ./dashboard/internal/api/ -v`
Expected: ALL PASS

- [ ] **Step 5: Commit**

```bash
git add dashboard/internal/api/server.go dashboard/internal/api/handler_experiments.go dashboard/internal/api/handler_experiments_test.go
git commit -m "feat(dashboard): add REST API server with experiments endpoints"
```

---

### Task 7: REST API Handlers - Overview Stats

**Note:** The spec's `trends` and `verdictTimeline` response fields require time-window comparison queries. These are deferred to a follow-up task after the core backend is working. The handler returns the core stats, avgRecoveryByType, and runningExperiments.

**Files:**
- Create: `dashboard/internal/api/handler_overview.go`
- Create: `dashboard/internal/api/handler_overview_test.go`

- [ ] **Step 1: Write failing test**

```go
// dashboard/internal/api/handler_overview_test.go
package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opendatahub-io/odh-platform-chaos/dashboard/internal/store"
)

type overviewMockStore struct {
	mockStore
	stats store.OverviewStats
	avgs  []store.RecoveryAvg
}

func (m *overviewMockStore) OverviewStats(since *time.Time) (store.OverviewStats, error) {
	return m.stats, nil
}

func (m *overviewMockStore) AvgRecoveryByType(since *time.Time) ([]store.RecoveryAvg, error) {
	return m.avgs, nil
}

func TestHandleOverviewStats(t *testing.T) {
	ms := &overviewMockStore{
		stats: store.OverviewStats{Total: 10, Resilient: 7, Degraded: 2, Failed: 1},
		avgs:  []store.RecoveryAvg{{InjectionType: "PodKill", AvgMs: 12000}},
	}

	srv := NewServer(ms, nil, nil)

	// Register overview route
	req := httptest.NewRequest("GET", "/api/v1/overview/stats", nil)
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var result map[string]interface{}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &result))
	assert.Equal(t, float64(10), result["total"])
	assert.Equal(t, float64(7), result["resilient"])
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd /Users/ugogiordano/workdir/rhoai/opendatahub-io/odh-platform-chaos && go test ./dashboard/internal/api/ -run TestHandleOverview -v`
Expected: FAIL (404 - route not registered)

- [ ] **Step 3: Implement overview handler and register route**

```go
// dashboard/internal/api/handler_overview.go
package api

import (
	"net/http"
	"time"
)

func (s *Server) handleOverviewStats(w http.ResponseWriter, r *http.Request) {
	var since *time.Time
	if v := r.URL.Query().Get("since"); v != "" {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			since = &t
		} else if d, err := time.ParseDuration(v); err == nil {
			t := time.Now().Add(-d)
			since = &t
		}
	}

	stats, err := s.store.OverviewStats(since)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	avgs, err := s.store.AvgRecoveryByType(since)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	avgMap := make(map[string]int64, len(avgs))
	for _, a := range avgs {
		avgMap[a.InjectionType] = a.AvgMs
	}

	// Running experiments: all non-terminal phases (Pending, SteadyStatePre, Injecting, Observing, SteadyStatePost, Evaluating)
	running, _ := s.store.ListRunning()

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"total":              stats.Total,
		"resilient":          stats.Resilient,
		"degraded":           stats.Degraded,
		"failed":             stats.Failed,
		"inconclusive":       stats.Inconclusive,
		"running":            stats.Running,
		"avgRecoveryByType":  avgMap,
		"runningExperiments": running,
	})
}
```

Then add to `server.go` `registerRoutes()`:
```go
s.mux.HandleFunc("GET /api/v1/overview/stats", s.handleOverviewStats)
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd /Users/ugogiordano/workdir/rhoai/opendatahub-io/odh-platform-chaos && go test ./dashboard/internal/api/ -v`
Expected: ALL PASS

- [ ] **Step 5: Commit**

```bash
git add dashboard/internal/api/handler_overview.go dashboard/internal/api/handler_overview_test.go dashboard/internal/api/server.go
git commit -m "feat(dashboard): add overview stats API endpoint"
```

---

### Task 8: REST API Handlers - Operators and Knowledge

**Files:**
- Create: `dashboard/internal/api/handler_operators.go`
- Create: `dashboard/internal/api/handler_operators_test.go`
- Create: `dashboard/internal/api/handler_knowledge.go`
- Create: `dashboard/internal/api/handler_knowledge_test.go`

- [ ] **Step 1: Write failing tests for operators handler**

```go
// dashboard/internal/api/handler_operators_test.go
package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opendatahub-io/odh-platform-chaos/dashboard/internal/store"
)

type operatorsMockStore struct {
	mockStore
	operators []string
	exps      []store.Experiment
}

func (m *operatorsMockStore) ListOperators(since *time.Time) ([]string, error) {
	return m.operators, nil
}

func (m *operatorsMockStore) ListByOperator(op string, since *time.Time) ([]store.Experiment, error) {
	var result []store.Experiment
	for _, e := range m.exps {
		if e.Operator == op {
			result = append(result, e)
		}
	}
	return result, nil
}

func TestHandleListOperators(t *testing.T) {
	ms := &operatorsMockStore{operators: []string{"op1", "op2"}}
	srv := NewServer(ms, nil, nil)

	req := httptest.NewRequest("GET", "/api/v1/operators", nil)
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var result []string
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &result))
	assert.Equal(t, []string{"op1", "op2"}, result)
}
```

```go
// dashboard/internal/api/handler_knowledge_test.go
package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opendatahub-io/odh-platform-chaos/pkg/model"
)

func TestHandleKnowledge(t *testing.T) {
	knowledge := []model.OperatorKnowledge{{
		Operator: model.OperatorInfo{Name: "opendatahub-operator"},
		Components: []model.ComponentModel{{
			Name: "odh-model-controller",
			ManagedResources: []model.ManagedResource{
				{Kind: "Deployment", Name: "odh-model-controller"},
			},
		}},
	}}

	srv := NewServer(&mockStore{}, nil, knowledge)
	req := httptest.NewRequest("GET", "/api/v1/knowledge/opendatahub-operator/odh-model-controller", nil)
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var result map[string]interface{}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &result))
	assert.Equal(t, "odh-model-controller", result["name"])
}

func TestHandleKnowledge_NotFound(t *testing.T) {
	srv := NewServer(&mockStore{}, nil, nil)
	req := httptest.NewRequest("GET", "/api/v1/knowledge/unknown/unknown", nil)
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)

	assert.Equal(t, http.StatusNotFound, rec.Code)
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd /Users/ugogiordano/workdir/rhoai/opendatahub-io/odh-platform-chaos && go test ./dashboard/internal/api/ -run "TestHandleListOperators|TestHandleKnowledge" -v`
Expected: FAIL

- [ ] **Step 3: Implement operators and knowledge handlers**

```go
// dashboard/internal/api/handler_operators.go
package api

import "net/http"

func (s *Server) handleListOperators(w http.ResponseWriter, r *http.Request) {
	ops, err := s.store.ListOperators(nil)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, ops)
}

func (s *Server) handleListComponents(w http.ResponseWriter, r *http.Request) {
	operator := pathSegment(r, "operator")
	exps, err := s.store.ListByOperator(operator, nil)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Extract unique components
	seen := map[string]bool{}
	var components []string
	for _, e := range exps {
		if !seen[e.Component] {
			seen[e.Component] = true
			components = append(components, e.Component)
		}
	}
	writeJSON(w, http.StatusOK, components)
}
```

```go
// dashboard/internal/api/handler_knowledge.go
package api

import "net/http"

func (s *Server) handleKnowledge(w http.ResponseWriter, r *http.Request) {
	operator := pathSegment(r, "operator")
	component := pathSegment(r, "component")

	for _, k := range s.knowledge {
		if k.Operator.Name == operator {
			for _, c := range k.Components {
				if c.Name == component {
					writeJSON(w, http.StatusOK, c)
					return
				}
			}
		}
	}
	writeError(w, http.StatusNotFound, "component not found")
}
```

Add to `server.go` `registerRoutes()`:
```go
s.mux.HandleFunc("GET /api/v1/operators", s.handleListOperators)
s.mux.HandleFunc("GET /api/v1/operators/{operator}/components", s.handleListComponents)
s.mux.HandleFunc("GET /api/v1/knowledge/{operator}/{component}", s.handleKnowledge)
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd /Users/ugogiordano/workdir/rhoai/opendatahub-io/odh-platform-chaos && go test ./dashboard/internal/api/ -v`
Expected: ALL PASS

- [ ] **Step 5: Commit**

```bash
git add dashboard/internal/api/handler_operators.go dashboard/internal/api/handler_operators_test.go dashboard/internal/api/handler_knowledge.go dashboard/internal/api/handler_knowledge_test.go dashboard/internal/api/server.go
git commit -m "feat(dashboard): add operators and knowledge API endpoints"
```

---

### Task 9: REST API Handlers - Suites

**Files:**
- Create: `dashboard/internal/api/handler_suites.go`
- Create: `dashboard/internal/api/handler_suites_test.go`

- [ ] **Step 1: Write failing test**

```go
// dashboard/internal/api/handler_suites_test.go
package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opendatahub-io/odh-platform-chaos/dashboard/internal/store"
)

type suitesMockStore struct {
	mockStore
	suiteRuns []store.SuiteRun
}

func (m *suitesMockStore) ListSuiteRuns() ([]store.SuiteRun, error) {
	return m.suiteRuns, nil
}

func (m *suitesMockStore) ListBySuiteRunID(id string) ([]store.Experiment, error) {
	var result []store.Experiment
	for _, e := range m.experiments {
		if e.SuiteRunID == id {
			result = append(result, e)
		}
	}
	return result, nil
}

func TestHandleListSuiteRuns(t *testing.T) {
	ms := &suitesMockStore{
		mockStore: mockStore{experiments: []store.Experiment{
			{Name: "e1", SuiteRunID: "run-1", SuiteName: "suite-a", Verdict: "Resilient", SpecJSON: "{}", StatusJSON: "{}"},
			{Name: "e2", SuiteRunID: "run-1", SuiteName: "suite-a", Verdict: "Failed", SpecJSON: "{}", StatusJSON: "{}"},
		}},
		suiteRuns: []store.SuiteRun{
			{SuiteName: "suite-a", SuiteRunID: "run-1", OperatorVersion: "v1.0", Total: 2, Resilient: 1, Failed: 1},
		},
	}

	srv := NewServer(ms, nil, nil)
	req := httptest.NewRequest("GET", "/api/v1/suites", nil)
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var result []store.SuiteRun
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &result))
	assert.Len(t, result, 1)
	assert.Equal(t, "suite-a", result[0].SuiteName)
}

func TestHandleGetSuiteRun(t *testing.T) {
	ms := &suitesMockStore{mockStore: mockStore{experiments: []store.Experiment{
		{Name: "e1", SuiteRunID: "run-1", SuiteName: "suite-a", Verdict: "Resilient", SpecJSON: "{}", StatusJSON: "{}"},
		{Name: "e2", SuiteRunID: "run-1", SuiteName: "suite-a", Verdict: "Failed", SpecJSON: "{}", StatusJSON: "{}"},
		{Name: "e3", SuiteRunID: "run-2", SuiteName: "suite-a", Verdict: "Resilient", SpecJSON: "{}", StatusJSON: "{}"},
	}}}

	srv := NewServer(ms, nil, nil)
	req := httptest.NewRequest("GET", "/api/v1/suites/run-1", nil)
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var result []json.RawMessage
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &result))
	assert.Len(t, result, 2)
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd /Users/ugogiordano/workdir/rhoai/opendatahub-io/odh-platform-chaos && go test ./dashboard/internal/api/ -run TestHandleGetSuite -v`
Expected: FAIL

- [ ] **Step 3: Implement suites handler**

```go
// dashboard/internal/api/handler_suites.go
package api

import "net/http"

func (s *Server) handleListSuiteRuns(w http.ResponseWriter, r *http.Request) {
	runs, err := s.store.ListSuiteRuns()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, runs)
}

func (s *Server) handleGetSuiteRun(w http.ResponseWriter, r *http.Request) {
	runID := pathSegment(r, "runId")
	exps, err := s.store.ListBySuiteRunID(runID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if len(exps) == 0 {
		writeError(w, http.StatusNotFound, "suite run not found")
		return
	}
	writeJSON(w, http.StatusOK, exps)
}

func (s *Server) handleCompareSuiteRuns(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	suiteName := q.Get("suite")
	runIDA := q.Get("runA")
	runIDB := q.Get("runB")
	if suiteName == "" || runIDA == "" || runIDB == "" {
		writeError(w, http.StatusBadRequest, "suite, runA, and runB query params required")
		return
	}

	a, b, err := s.store.CompareSuiteRuns(suiteName, runIDA, runIDB)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"runA": a,
		"runB": b,
	})
}
```

Add to `server.go` `registerRoutes()`:
```go
s.mux.HandleFunc("GET /api/v1/suites", s.handleListSuiteRuns)
s.mux.HandleFunc("GET /api/v1/suites/compare", s.handleCompareSuiteRuns)
s.mux.HandleFunc("GET /api/v1/suites/{runId}", s.handleGetSuiteRun)
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd /Users/ugogiordano/workdir/rhoai/opendatahub-io/odh-platform-chaos && go test ./dashboard/internal/api/ -v`
Expected: ALL PASS

- [ ] **Step 5: Commit**

```bash
git add dashboard/internal/api/handler_suites.go dashboard/internal/api/handler_suites_test.go dashboard/internal/api/server.go
git commit -m "feat(dashboard): add suites API endpoint"
```

---

### Task 10: Dashboard Binary Entry Point

**Files:**
- Create: `dashboard/cmd/dashboard/main.go`

Note: `embed.go` for serving the React frontend is deferred to the frontend implementation plan. The binary works standalone via its REST API.

- [ ] **Step 1: Create main.go**

```go
// dashboard/cmd/dashboard/main.go
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/opendatahub-io/odh-platform-chaos/api/v1alpha1"
	"github.com/opendatahub-io/odh-platform-chaos/dashboard/internal/api"
	"github.com/opendatahub-io/odh-platform-chaos/dashboard/internal/store"
	"github.com/opendatahub-io/odh-platform-chaos/dashboard/internal/watcher"
	"github.com/opendatahub-io/odh-platform-chaos/pkg/model"
	"k8s.io/apimachinery/pkg/runtime"
)

func main() {
	var (
		addr       = flag.String("addr", ":8080", "HTTP listen address")
		dbPath     = flag.String("db", "dashboard.db", "SQLite database path")
		kubeconfig = flag.String("kubeconfig", "", "Path to kubeconfig (uses in-cluster if empty)")
		syncInterval = flag.Duration("sync-interval", 30*time.Second, "Interval for K8s sync")
	)
	flag.Parse()

	// Open store
	s, err := store.NewSQLiteStore(*dbPath)
	if err != nil {
		log.Fatalf("opening store: %v", err)
	}
	defer s.Close()

	// Build K8s client
	scheme := runtime.NewScheme()
	if err := v1alpha1.AddToScheme(scheme); err != nil {
		log.Fatalf("adding scheme: %v", err)
	}

	config, err := clientcmd.BuildConfigFromFlags("", *kubeconfig)
	if err != nil {
		log.Fatalf("building kubeconfig: %v", err)
	}

	k8sClient, err := client.New(config, client.Options{Scheme: scheme})
	if err != nil {
		log.Fatalf("creating k8s client: %v", err)
	}

	// Load knowledge (optional, log warning if missing)
	var knowledge []model.OperatorKnowledge
	// Knowledge files are loaded from the knowledge/ directory if present

	// SSE broker
	broker := api.NewSSEBroker()
	go broker.Run()

	// Watcher (connected to SSE broker for live updates)
	w := watcher.NewWatcher(k8sClient, s, broker)

	// Initial sync
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := w.SyncOnce(ctx); err != nil {
		log.Printf("warning: initial sync failed: %v", err)
	}

	// Periodic sync
	go func() {
		ticker := time.NewTicker(*syncInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				if err := w.SyncOnce(ctx); err != nil {
					log.Printf("sync error: %v", err)
				}
			case <-ctx.Done():
				return
			}
		}
	}()

	// HTTP server
	srv := api.NewServer(s, broker, knowledge)
	httpServer := &http.Server{
		Addr:    *addr,
		Handler: srv.Handler(),
	}

	// Graceful shutdown
	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		<-sigCh
		log.Println("shutting down...")
		cancel()
		broker.Stop()
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer shutdownCancel()
		httpServer.Shutdown(shutdownCtx)
	}()

	log.Printf("dashboard listening on %s", *addr)
	if err := httpServer.ListenAndServe(); err != http.ErrServerClosed {
		log.Fatalf("server error: %v", err)
	}
	fmt.Println("dashboard stopped")
}
```

- [ ] **Step 2: Verify it compiles**

Run: `cd /Users/ugogiordano/workdir/rhoai/opendatahub-io/odh-platform-chaos && go build ./dashboard/cmd/dashboard/`
Expected: Compiles successfully

- [ ] **Step 3: Run all tests**

Run: `cd /Users/ugogiordano/workdir/rhoai/opendatahub-io/odh-platform-chaos && go test ./dashboard/... -v`
Expected: ALL PASS

- [ ] **Step 4: Commit**

```bash
git add dashboard/cmd/
git commit -m "feat(dashboard): add binary entry point with K8s watcher and HTTP server"
```

---

### Task 11: Integration Test

**Files:**
- Create: `dashboard/internal/store/integration_test.go`

- [ ] **Step 1: Write end-to-end test that exercises the full stack (store + conversion + API)**

```go
// dashboard/internal/store/integration_test.go
package store

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIntegration_FullLifecycle(t *testing.T) {
	s := newTestStore(t)

	// Insert multiple experiments simulating a suite run
	now := time.Now().UTC().Truncate(time.Millisecond)
	recovery1 := int64(32000)
	recovery2 := int64(58000)

	exps := []Experiment{
		{ID: "ns/e1/" + now.Format(time.RFC3339), Name: "e1", Namespace: "ns", Operator: "op1", Component: "comp1", InjectionType: "PodKill", Phase: "Complete", Verdict: "Resilient", RecoveryMs: &recovery1, StartTime: &now, SuiteRunID: "run-1", SuiteName: "suite-a", OperatorVersion: "v1.0", SpecJSON: "{}", StatusJSON: "{}"},
		{ID: "ns/e2/" + now.Format(time.RFC3339), Name: "e2", Namespace: "ns", Operator: "op1", Component: "comp1", InjectionType: "ConfigDrift", Phase: "Complete", Verdict: "Degraded", RecoveryMs: &recovery2, StartTime: &now, SuiteRunID: "run-1", SuiteName: "suite-a", OperatorVersion: "v1.0", SpecJSON: "{}", StatusJSON: "{}"},
		{ID: "ns/e3/" + now.Format(time.RFC3339), Name: "e3", Namespace: "ns", Operator: "op1", Component: "comp1", InjectionType: "PodKill", Phase: "Observing", Verdict: "", StartTime: &now, SpecJSON: "{}", StatusJSON: "{}"},
	}

	for _, e := range exps {
		require.NoError(t, s.Upsert(e))
	}

	// Test overview stats
	stats, err := s.OverviewStats(nil)
	require.NoError(t, err)
	assert.Equal(t, 3, stats.Total)
	assert.Equal(t, 1, stats.Resilient)
	assert.Equal(t, 1, stats.Degraded)
	assert.Equal(t, 1, stats.Running)

	// Test avg recovery
	avgs, err := s.AvgRecoveryByType(nil)
	require.NoError(t, err)
	require.Len(t, avgs, 2) // ConfigDrift + PodKill

	// Test list by suite
	suiteExps, err := s.ListBySuiteRunID("run-1")
	require.NoError(t, err)
	assert.Len(t, suiteExps, 2)

	// Test list operators
	ops, err := s.ListOperators(nil)
	require.NoError(t, err)
	assert.Equal(t, []string{"op1"}, ops)

	// Test filtering
	result, err := s.List(ListFilter{Phase: "Observing", Page: 1, PageSize: 10})
	require.NoError(t, err)
	assert.Equal(t, 1, result.TotalCount)
	assert.Equal(t, "e3", result.Items[0].Name)
}
```

- [ ] **Step 2: Run test**

Run: `cd /Users/ugogiordano/workdir/rhoai/opendatahub-io/odh-platform-chaos && go test ./dashboard/internal/store/ -run TestIntegration -v`
Expected: PASS

- [ ] **Step 3: Run full test suite**

Run: `cd /Users/ugogiordano/workdir/rhoai/opendatahub-io/odh-platform-chaos && go test ./dashboard/... -v -count=1`
Expected: ALL PASS

- [ ] **Step 4: Commit**

```bash
git add dashboard/internal/store/integration_test.go
git commit -m "test(dashboard): add integration test for full store lifecycle"
```
