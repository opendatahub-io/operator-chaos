# Chaos Dashboard Frontend Phase 2 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build the remaining 4 frontend views — Live monitoring (SSE), Suites (with comparison), Operators (with coverage matrix), and Knowledge (dependency graph) — replacing the Placeholder components from Phase 1.

**Architecture:** Builds on the Phase 1 React SPA (Vite, PF5, react-router-dom v6). Adds a `useSSE` hook for live streaming, reusable Phase Stepper and Progress Bar components, and an SVG-based dependency graph for the Knowledge view. All data comes from existing backend APIs.

**Tech Stack:** React 18, TypeScript, PatternFly 5, Vite, Vitest + React Testing Library

---

## Spec Reference

- Design spec: `docs/superpowers/specs/2026-03-25-chaos-dashboard-design.md`
- HTML mockups: `.superpowers/brainstorm/80649-1774450280/` (live.html, suites.html, operators.html, knowledge.html)
- Phase 1 plan: `docs/superpowers/plans/2026-03-26-chaos-dashboard-frontend.md`
- Backend handlers: `dashboard/internal/api/handler_suites.go`, `handler_operators.go`, `handler_knowledge.go`, `sse.go`
- Knowledge model: `pkg/model/knowledge.go` (ComponentModel, ManagedResource, WebhookSpec types)

## Existing Phase 1 Context

**Already implemented:**
- Vite scaffold, TypeScript types (`Experiment`, `OverviewStats`, `SuiteRun`, `ListResult`, etc.)
- `apiUrl()`, `apiFetch()`, `useApi()` hook
- Layout with sidebar nav, VerdictBadge, PhaseBadge, StatusBanner, TrendIndicator
- Overview, ExperimentsList, ExperimentDetail pages
- Go embed integration serving built UI assets
- App.tsx has placeholder routes for `/live`, `/suites`, `/operators`, `/knowledge`

**Backend APIs for Phase 2:**
- `GET /api/v1/experiments/live` — SSE stream (text/event-stream), broadcasts JSON experiment updates
- `GET /api/v1/experiments?operator=X` — Filter experiments by operator (for operator stats)
- `GET /api/v1/suites` — Returns `SuiteRun[]` with verdict counts
- `GET /api/v1/suites/:runId` — Returns `Experiment[]` for a suite run
- `GET /api/v1/suites/compare?suite=X&runA=Y&runB=Z` — Returns `{ runA: Experiment[], runB: Experiment[] }`
- `GET /api/v1/operators` — Returns `string[]` of operator names
- `GET /api/v1/operators/:operator/components` — Returns `string[]` of component names
- `GET /api/v1/knowledge/:operator/:component` — Returns `ComponentModel` JSON

## File Structure (Phase 2 additions)

```
dashboard/ui/src/
  types/
    api.ts                          # Add Knowledge types (ComponentModel, ManagedResource)
  api/
    hooks.ts                        # Add useSSE hook
  components/
    PhaseStepper.tsx                # Phase stepper with done/active/pending dots
    PhaseStepper.css                # Stepper styles, pulse animation
    ProgressBar.tsx                 # Stacked verdict progress bar
    CoverageMatrix.tsx              # Injection type coverage grid
    CoverageMatrix.css              # Coverage cell styles
  pages/
    Live.tsx                        # Live monitoring with SSE
    Live.css                        # Live card, event log, progress info styles
    Suites.tsx                      # Suite runs + comparison view
    Suites.css                      # Suite card, comparison table styles
    Operators.tsx                   # Operator cards with component accordion
    Operators.css                   # Operator card, health bar, accordion styles
    Knowledge.tsx                   # SVG dependency graph + detail panel
    Knowledge.css                   # Graph, node, panel styles
```

---

### Task 1: Types and SSE Hook

**Files:**
- Modify: `dashboard/ui/src/types/api.ts`
- Modify: `dashboard/ui/src/api/hooks.ts`
- Modify: `dashboard/ui/src/types/api.test.ts`
- Create: `dashboard/ui/src/api/hooks.test.ts` (already exists — add SSE tests)

**Context:** Phase 2 needs types for the Knowledge model (returned by `GET /knowledge/:op/:comp`) and a `useSSE` hook for the Live page. The Knowledge API returns a Go `ComponentModel` struct serialized as JSON.

- [ ] **Step 1: Write tests for new types and useSSE**

Add to `dashboard/ui/src/types/api.test.ts`:

```typescript
describe('Knowledge types', () => {
  it('defines ManagedResource shape', () => {
    const r: ManagedResource = {
      apiVersion: 'apps/v1',
      kind: 'Deployment',
      name: 'odh-model-controller',
    };
    expect(r.kind).toBe('Deployment');
  });

  it('defines ComponentModel shape', () => {
    const c: ComponentModel = {
      name: 'odh-model-controller',
      controller: 'odh-model-controller',
      managedResources: [],
    };
    expect(c.name).toBe('odh-model-controller');
  });
});
```

Add to `dashboard/ui/src/api/hooks.test.ts`:

```typescript
import { renderHook, act } from '@testing-library/react';
import { useSSE } from './hooks';

describe('useSSE', () => {
  it('returns empty events initially', () => {
    const { result } = renderHook(() => useSSE('/api/v1/experiments/live'));
    expect(result.current.events).toEqual([]);
    expect(result.current.connected).toBe(false);
  });

  it('accepts null URL and does nothing', () => {
    const { result } = renderHook(() => useSSE(null));
    expect(result.current.events).toEqual([]);
    expect(result.current.connected).toBe(false);
  });
});
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
cd dashboard/ui && npx vitest run src/types/api.test.ts src/api/hooks.test.ts
```

Expected: FAIL — `ManagedResource`, `ComponentModel`, `useSSE` not found

- [ ] **Step 3: Add Knowledge types to api.ts**

Append to `dashboard/ui/src/types/api.ts`:

```typescript
export interface ManagedResource {
  apiVersion: string;
  kind: string;
  name: string;
  namespace?: string;
  labels?: Record<string, string>;
  ownerRef?: string;
  expectedSpec?: Record<string, unknown>;
}

export interface WebhookSpec {
  name: string;
  type: string;
  path: string;
}

export interface ComponentModel {
  name: string;
  controller: string;
  managedResources: ManagedResource[];
  dependencies?: string[];
  webhooks?: WebhookSpec[];
  finalizers?: string[];
}
```

- [ ] **Step 4: Add useSSE hook to hooks.ts**

Append to `dashboard/ui/src/api/hooks.ts`:

```typescript
interface SSEState<T> {
  events: T[];
  connected: boolean;
  error: string | null;
}

export function useSSE<T = unknown>(url: string | null): SSEState<T> {
  const [events, setEvents] = useState<T[]>([]);
  const [connected, setConnected] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (url === null) return;

    const es = new EventSource(url);

    es.onopen = () => {
      setConnected(true);
      setError(null);
    };

    es.onmessage = (event) => {
      try {
        const data = JSON.parse(event.data) as T;
        setEvents((prev) => {
          // Replace existing experiment by id, or append
          const existing = prev.findIndex(
            (e: any) => e.id && (e as any).id === (data as any).id
          );
          if (existing >= 0) {
            const next = [...prev];
            next[existing] = data;
            return next;
          }
          return [...prev, data];
        });
      } catch {
        // Skip malformed messages
      }
    };

    es.onerror = () => {
      setConnected(false);
      setError('Connection lost. Reconnecting...');
    };

    return () => {
      es.close();
      setConnected(false);
    };
  }, [url]);

  return { events, connected, error };
}
```

- [ ] **Step 5: Run tests to verify they pass**

```bash
cd dashboard/ui && npx vitest run src/types/api.test.ts src/api/hooks.test.ts
```

Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add dashboard/ui/src/types/api.ts dashboard/ui/src/types/api.test.ts dashboard/ui/src/api/hooks.ts dashboard/ui/src/api/hooks.test.ts
git commit -m "feat(dashboard): add Knowledge types and useSSE hook"
```

---

### Task 2: Phase Stepper and Progress Bar Components

**Files:**
- Create: `dashboard/ui/src/components/PhaseStepper.tsx`
- Create: `dashboard/ui/src/components/PhaseStepper.css`
- Create: `dashboard/ui/src/components/ProgressBar.tsx`
- Modify: `dashboard/ui/src/components/components.test.tsx`

**Context:** The Phase Stepper shows experiment phases as a horizontal dot-and-line stepper (done/active/pending). Used by the Live page. The ProgressBar shows stacked Resilient/Degraded/Failed segments. Used by Suites and Operators pages. See `live.html` mockup for stepper design, `suites.html` for progress bar.

- [ ] **Step 1: Write tests**

Add to `dashboard/ui/src/components/components.test.tsx`:

```typescript
import { PhaseStepper } from './PhaseStepper';
import { ProgressBar } from './ProgressBar';

describe('PhaseStepper', () => {
  it('renders all phase steps', () => {
    render(<PhaseStepper currentPhase="Injecting" />);
    expect(screen.getByText('Pending')).toBeInTheDocument();
    expect(screen.getByText('Pre-check')).toBeInTheDocument();
    expect(screen.getByText('Injecting')).toBeInTheDocument();
    expect(screen.getByText('Observing')).toBeInTheDocument();
  });

  it('marks completed phases as done', () => {
    const { container } = render(<PhaseStepper currentPhase="Observing" />);
    const dots = container.querySelectorAll('.step-dot');
    // Pending, Pre-check, Injecting should be "done"
    expect(dots[0]).toHaveClass('done');
    expect(dots[1]).toHaveClass('done');
    expect(dots[2]).toHaveClass('done');
    // Observing should be "active"
    expect(dots[3]).toHaveClass('active');
  });

  it('handles Aborted phase', () => {
    const { container } = render(<PhaseStepper currentPhase="Aborted" abortedAtPhase="Injecting" />);
    const dots = container.querySelectorAll('.step-dot');
    expect(dots[2]).toHaveClass('aborted');
  });
});

describe('ProgressBar', () => {
  it('renders segments with correct widths', () => {
    const { container } = render(<ProgressBar resilient={7} degraded={2} failed={1} />);
    const segments = container.querySelectorAll('.progress-segment');
    expect(segments).toHaveLength(3);
  });

  it('handles all zeros gracefully', () => {
    const { container } = render(<ProgressBar resilient={0} degraded={0} failed={0} />);
    const segments = container.querySelectorAll('.progress-segment');
    expect(segments).toHaveLength(0);
  });
});
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
cd dashboard/ui && npx vitest run src/components/components.test.tsx
```

- [ ] **Step 3: Implement PhaseStepper**

Create `dashboard/ui/src/components/PhaseStepper.tsx`:

```typescript
import { phaseDisplayName } from '../types/api';
import './PhaseStepper.css';

const STEPPER_PHASES = [
  'Pending', 'SteadyStatePre', 'Injecting', 'Observing',
  'SteadyStatePost', 'Evaluating', 'Complete',
];

interface Props {
  currentPhase: string;
  abortedAtPhase?: string;
}

export function PhaseStepper({ currentPhase, abortedAtPhase }: Props) {
  const isAborted = currentPhase === 'Aborted';
  const activePhase = isAborted ? abortedAtPhase : currentPhase;
  const activeIdx = STEPPER_PHASES.indexOf(activePhase ?? '');

  return (
    <div className="phase-stepper">
      {STEPPER_PHASES.map((phase, i) => {
        let status: 'done' | 'active' | 'pending' | 'aborted' = 'pending';
        if (i < activeIdx) status = 'done';
        else if (i === activeIdx) status = isAborted ? 'aborted' : 'active';

        return (
          <div key={phase} className="step">
            {i > 0 && <div className={`step-line ${i <= activeIdx ? 'done' : 'pending'}`} />}
            <div className="step-col">
              <div className={`step-dot ${status}`}>
                {status === 'done' ? '\u2713' : i + 1}
              </div>
              <div className="step-label">{phaseDisplayName(phase)}</div>
            </div>
          </div>
        );
      })}
    </div>
  );
}
```

Create `dashboard/ui/src/components/PhaseStepper.css`:

```css
.phase-stepper {
  display: flex;
  align-items: flex-start;
  padding: 0 20px 16px;
}
.step { display: flex; align-items: center; }
.step-col { display: flex; flex-direction: column; align-items: center; }
.step-dot {
  width: 28px; height: 28px; border-radius: 50%;
  display: flex; align-items: center; justify-content: center;
  font-size: 12px; font-weight: 700; flex-shrink: 0;
}
.step-dot.done { background: var(--trend-up, #3e8635); color: white; }
.step-dot.active {
  background: var(--primary, #06c); color: white;
  box-shadow: 0 0 0 4px rgba(0,102,204,0.2);
  animation: pulse-step 1.5s infinite;
}
.step-dot.pending { background: #f0f0f0; color: #6a6e73; }
.step-dot.aborted { background: #c9190b; color: white; }
@keyframes pulse-step {
  0%, 100% { box-shadow: 0 0 0 4px rgba(0,102,204,0.2); }
  50% { box-shadow: 0 0 0 8px rgba(0,102,204,0); }
}
.step-line { width: 40px; height: 2px; flex-shrink: 0; }
.step-line.done { background: var(--trend-up, #3e8635); }
.step-line.pending { background: #d2d2d2; }
.step-label { font-size: 10px; color: #6a6e73; text-align: center; margin-top: 4px; }
```

- [ ] **Step 4: Implement ProgressBar**

Create `dashboard/ui/src/components/ProgressBar.tsx`:

```typescript
interface Props {
  resilient: number;
  degraded: number;
  failed: number;
}

export function ProgressBar({ resilient, degraded, failed }: Props) {
  const total = resilient + degraded + failed;
  if (total === 0) return <div className="progress-bar" />;

  const segments = [
    { value: resilient, className: 'green' },
    { value: degraded, className: 'yellow' },
    { value: failed, className: 'red' },
  ].filter((s) => s.value > 0);

  return (
    <div className="progress-bar">
      {segments.map((seg) => (
        <div
          key={seg.className}
          className={`progress-segment ${seg.className}`}
          style={{ width: `${(seg.value / total) * 100}%` }}
        />
      ))}
    </div>
  );
}
```

Add to `dashboard/ui/src/App.css` (or existing component CSS):

```css
.progress-bar {
  display: flex;
  height: 12px;
  border-radius: 6px;
  overflow: hidden;
  background: #f0f0f0;
  flex: 1;
}
.progress-segment { height: 100%; }
.progress-segment.green { background: var(--trend-up, #3e8635); }
.progress-segment.yellow { background: #f0ab00; }
.progress-segment.red { background: var(--trend-down, #c9190b); }
```

- [ ] **Step 5: Run tests to verify they pass**

```bash
cd dashboard/ui && npx vitest run src/components/components.test.tsx
```

- [ ] **Step 6: Commit**

```bash
git add dashboard/ui/src/components/PhaseStepper.tsx dashboard/ui/src/components/PhaseStepper.css dashboard/ui/src/components/ProgressBar.tsx dashboard/ui/src/components/components.test.tsx dashboard/ui/src/App.css
git commit -m "feat(dashboard): add PhaseStepper and ProgressBar components"
```

---

### Task 3: Live Monitoring Page

**Files:**
- Create: `dashboard/ui/src/pages/Live.tsx`
- Create: `dashboard/ui/src/pages/Live.css`
- Create: `dashboard/ui/src/pages/Live.test.tsx`

**Context:** The Live page connects to SSE at `/api/v1/experiments/live` and displays real-time experiment cards. Each card shows: name, phase badge, operator/type metadata, PhaseStepper, event log from statusJson, and progress metadata (elapsed time, target info). If no experiments are running, show EmptyState. Shows a reconnection banner when SSE disconnects. See `live.html` mockup. The SSE backend sends full experiment JSON on each status change.

- [ ] **Step 1: Write tests**

Create `dashboard/ui/src/pages/Live.test.tsx`:

```typescript
import { render, screen } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { Live } from './Live';
import { vi } from 'vitest';

// Mock useSSE hook
vi.mock('../api/hooks', () => ({
  useSSE: vi.fn(() => ({ events: [], connected: false, error: null })),
  useApi: vi.fn(() => ({ data: null, loading: true, error: null, refetch: vi.fn() })),
}));

import { useSSE } from '../api/hooks';

function renderLive() {
  return render(
    <MemoryRouter>
      <Live />
    </MemoryRouter>
  );
}

describe('Live', () => {
  it('shows empty state when no experiments running', () => {
    (useSSE as ReturnType<typeof vi.fn>).mockReturnValue({
      events: [], connected: true, error: null,
    });
    renderLive();
    expect(screen.getByText(/no experiments/i)).toBeInTheDocument();
  });

  it('renders experiment card with name and phase', () => {
    (useSSE as ReturnType<typeof vi.fn>).mockReturnValue({
      events: [{
        id: '1', name: 'omc-podkill', namespace: 'test', operator: 'op',
        component: 'comp', type: 'PodKill', phase: 'Injecting',
        specJson: '{}', statusJson: '{}', createdAt: '', updatedAt: '',
      }],
      connected: true,
      error: null,
    });
    renderLive();
    expect(screen.getByText('omc-podkill')).toBeInTheDocument();
  });

  it('shows reconnection banner on error', () => {
    (useSSE as ReturnType<typeof vi.fn>).mockReturnValue({
      events: [], connected: false, error: 'Connection lost',
    });
    renderLive();
    expect(screen.getByText(/connection lost/i)).toBeInTheDocument();
  });

  it('shows connected indicator when streaming', () => {
    (useSSE as ReturnType<typeof vi.fn>).mockReturnValue({
      events: [], connected: true, error: null,
    });
    renderLive();
    expect(screen.getByText(/live/i)).toBeInTheDocument();
  });
});
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
cd dashboard/ui && npx vitest run src/pages/Live.test.tsx
```

- [ ] **Step 3: Implement Live page**

Create `dashboard/ui/src/pages/Live.tsx` — renders SSE-fed experiment cards with PhaseStepper, event log parsed from statusJson, progress metadata, link to detail page, reconnection warning banner, and empty state.

Create `dashboard/ui/src/pages/Live.css` — live-dot pulse animation, live-card with colored left border based on phase, event log rows, progress info bar.

The page should:
- Use `useSSE<Experiment>('/api/v1/experiments/live')` to get streaming experiment data
- Also fetch initial running experiments via `useApi` on `/api/v1/overview/stats` for the `runningExperiments` field as seed data
- Display each experiment as a card with: name + phase badge header, operator/type/start time metadata, PhaseStepper component, event log (parsed from statusJson), elapsed time
- Show a pulsing red dot in the page header when connected
- Show StatusBanner warning when disconnected
- Show EmptyState when no running experiments
- Link "View Detail" button to `/experiments/:namespace/:name`

- [ ] **Step 4: Run tests to verify they pass**

```bash
cd dashboard/ui && npx vitest run src/pages/Live.test.tsx
```

- [ ] **Step 5: Verify build**

```bash
cd dashboard/ui && npm run build
```

- [ ] **Step 6: Commit**

```bash
git add dashboard/ui/src/pages/Live.tsx dashboard/ui/src/pages/Live.css dashboard/ui/src/pages/Live.test.tsx
git commit -m "feat(dashboard): add Live monitoring page with SSE streaming"
```

---

### Task 4: Suites Page

**Files:**
- Create: `dashboard/ui/src/pages/Suites.tsx`
- Create: `dashboard/ui/src/pages/Suites.css`
- Create: `dashboard/ui/src/pages/Suites.test.tsx`

**Context:** The Suites page shows suite run history and version-to-version comparison. It fetches from `GET /api/v1/suites` (returns `SuiteRun[]`), `GET /api/v1/suites/:runId` (returns `Experiment[]`), and `GET /api/v1/suites/compare?suite=X&runA=Y&runB=Z` (returns `{runA: Experiment[], runB: Experiment[]}`). See `suites.html` mockup for the layout.

**Key UI elements:**
- Suite run cards with name, version, experiment count, date, verdict stat counters, and ProgressBar
- Expandable experiment table per suite run (click header to expand/collapse)
- Version comparison section: two dropdown selectors (version A vs B from same suite name), comparison table showing experiment name, type, verdict A, recovery A, verdict B, recovery B, delta indicator (improved/regressed/no change)
- Loading/empty/error states

- [ ] **Step 1: Write tests**

Create `dashboard/ui/src/pages/Suites.test.tsx`:

```typescript
import { render, screen, waitFor } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { Suites } from './Suites';
import { vi } from 'vitest';

vi.mock('../api/hooks', () => ({
  useApi: vi.fn(() => ({ data: null, loading: false, error: null, refetch: vi.fn() })),
}));

import { useApi } from '../api/hooks';

function renderSuites() {
  return render(
    <MemoryRouter>
      <Suites />
    </MemoryRouter>
  );
}

describe('Suites', () => {
  it('shows loading state', () => {
    (useApi as ReturnType<typeof vi.fn>).mockReturnValue({
      data: null, loading: true, error: null, refetch: vi.fn(),
    });
    renderSuites();
    expect(screen.getByText(/loading/i)).toBeInTheDocument();
  });

  it('shows empty state when no suites', () => {
    (useApi as ReturnType<typeof vi.fn>).mockReturnValue({
      data: [], loading: false, error: null, refetch: vi.fn(),
    });
    renderSuites();
    expect(screen.getByText(/no suite runs/i)).toBeInTheDocument();
  });

  it('renders suite run cards', () => {
    (useApi as ReturnType<typeof vi.fn>).mockReturnValue({
      data: [{
        suiteName: 'omc-full-suite', suiteRunId: 'run-1',
        operatorVersion: 'v2.10.0', total: 7, resilient: 5, degraded: 1, failed: 1,
      }],
      loading: false, error: null, refetch: vi.fn(),
    });
    renderSuites();
    expect(screen.getByText('omc-full-suite')).toBeInTheDocument();
    expect(screen.getByText('v2.10.0')).toBeInTheDocument();
  });

  it('shows error state', () => {
    (useApi as ReturnType<typeof vi.fn>).mockReturnValue({
      data: null, loading: false, error: 'Server error', refetch: vi.fn(),
    });
    renderSuites();
    expect(screen.getByText(/server error/i)).toBeInTheDocument();
  });
});
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
cd dashboard/ui && npx vitest run src/pages/Suites.test.tsx
```

- [ ] **Step 3: Implement Suites page**

Create `dashboard/ui/src/pages/Suites.tsx` and `Suites.css`.

The page should:
- Fetch suite runs via `useApi<SuiteRun[]>(apiUrl('/suites'))`
- Display each suite run as a card with: suite name + version header, experiment count, verdict stat counters (Resilient/Degraded/Failed), ProgressBar component
- Click on suite header to expand → fetch `useApi<Experiment[]>(apiUrl('/suites/' + runId))` and display experiment table with columns: Name (link to detail), Type, Verdict badge, Recovery time
- Comparison section (shown when 2+ runs have the same suiteName):
  - Two dropdowns to select version A and B (filtered by same suiteName)
  - On selection, fetch via `useApi` with `apiUrl('/suites/compare', { suite, runA, runB })`
  - Render comparison table: Experiment name, Type, Verdict A, Recovery A, Verdict B, Recovery B, Delta (improved = green, regressed = red, same = gray)
- Standard loading/empty/error states

Create `dashboard/ui/src/pages/Suites.css` — suite-card, suite-header, suite-summary, suite-table, comparison-card, comparison-table styles matching the mockup.

- [ ] **Step 4: Run tests to verify they pass**

```bash
cd dashboard/ui && npx vitest run src/pages/Suites.test.tsx
```

- [ ] **Step 5: Commit**

```bash
git add dashboard/ui/src/pages/Suites.tsx dashboard/ui/src/pages/Suites.css dashboard/ui/src/pages/Suites.test.tsx
git commit -m "feat(dashboard): add Suites page with run history and comparison"
```

---

### Task 5: Operators Page

**Files:**
- Create: `dashboard/ui/src/pages/Operators.tsx`
- Create: `dashboard/ui/src/pages/Operators.css`
- Create: `dashboard/ui/src/pages/Operators.test.tsx`
- Create: `dashboard/ui/src/components/CoverageMatrix.tsx`
- Create: `dashboard/ui/src/components/CoverageMatrix.css`

**Context:** The Operators page shows per-operator resilience insights. It fetches operator names from `GET /operators`, then experiments per operator from `GET /experiments?operator=X` (computing verdict stats client-side). See `operators.html` mockup. The CoverageMatrix shows an 8-column grid (one per injection type) with pass/warn/fail/untested status per component.

**Key UI elements:**
- Operator cards with name, health bar (ProgressBar), verdict mini-badges (colored dots with counts)
- Component accordion: click to expand, showing CoverageMatrix and recent experiment history
- CoverageMatrix: 8 injection types as columns, cells colored by best verdict for that type (green=Resilient, yellow=Degraded, red=Failed, gray=untested), showing count
- Recent experiments table per component with links to detail
- Loading/empty/error states

- [ ] **Step 1: Write tests**

Create `dashboard/ui/src/pages/Operators.test.tsx`:

```typescript
import { render, screen } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { Operators } from './Operators';
import { vi } from 'vitest';

vi.mock('../api/hooks', () => ({
  useApi: vi.fn(() => ({ data: null, loading: false, error: null, refetch: vi.fn() })),
}));

import { useApi } from '../api/hooks';

function renderOperators() {
  return render(
    <MemoryRouter>
      <Operators />
    </MemoryRouter>
  );
}

describe('Operators', () => {
  it('shows loading state', () => {
    (useApi as ReturnType<typeof vi.fn>).mockReturnValue({
      data: null, loading: true, error: null, refetch: vi.fn(),
    });
    renderOperators();
    expect(screen.getByText(/loading/i)).toBeInTheDocument();
  });

  it('shows empty state when no operators', () => {
    (useApi as ReturnType<typeof vi.fn>).mockReturnValue({
      data: [], loading: false, error: null, refetch: vi.fn(),
    });
    renderOperators();
    expect(screen.getByText(/no operators/i)).toBeInTheDocument();
  });

  it('renders operator names', () => {
    (useApi as ReturnType<typeof vi.fn>).mockImplementation((url: string) => {
      if (url.includes('/operators')) {
        return { data: ['opendatahub-operator'], loading: false, error: null, refetch: vi.fn() };
      }
      return { data: { items: [], totalCount: 0 }, loading: false, error: null, refetch: vi.fn() };
    });
    renderOperators();
    expect(screen.getByText('opendatahub-operator')).toBeInTheDocument();
  });
});
```

Add CoverageMatrix tests to `dashboard/ui/src/components/components.test.tsx`:

```typescript
import { CoverageMatrix } from './CoverageMatrix';

describe('CoverageMatrix', () => {
  it('renders injection type columns', () => {
    render(<CoverageMatrix experiments={[]} />);
    expect(screen.getByText('PodKill')).toBeInTheDocument();
    expect(screen.getByText('ConfigDrift')).toBeInTheDocument();
  });

  it('shows tested count for injection types with experiments', () => {
    const exps = [
      { type: 'PodKill', verdict: 'Resilient' },
      { type: 'PodKill', verdict: 'Resilient' },
    ];
    render(<CoverageMatrix experiments={exps as any} />);
    expect(screen.getByText(/2x/)).toBeInTheDocument();
  });
});
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
cd dashboard/ui && npx vitest run src/pages/Operators.test.tsx src/components/components.test.tsx
```

- [ ] **Step 3: Implement CoverageMatrix component**

Create `dashboard/ui/src/components/CoverageMatrix.tsx` — takes an array of experiments (filtered to one component), groups by injection type, shows best verdict per type with count. 8 columns for INJECTION_TYPES, cells colored: green (all Resilient), yellow (any Degraded), red (any Failed), gray (untested/no experiments).

Create `dashboard/ui/src/components/CoverageMatrix.css` — coverage-grid (8-column CSS grid), coverage-cell with tested-pass/tested-warn/tested-fail/not-tested variants, coverage-label.

- [ ] **Step 4: Implement Operators page**

Create `dashboard/ui/src/pages/Operators.tsx` and `Operators.css`.

The page should:
- Fetch operator names via `useApi<string[]>(apiUrl('/operators'))`
- For each operator, render a card with: operator name header, ProgressBar, mini-badges with verdict counts
- Verdict counts are computed by fetching `useApi<ListResult>(apiUrl('/experiments', { operator, pageSize: 1000 }))` and counting verdicts client-side
- Component accordion: group experiments by component, show toggle header with component name and verdict mini-badges, expandable body with CoverageMatrix and recent experiment list (last 5, with links to detail)
- Loading/empty/error states

Create `dashboard/ui/src/pages/Operators.css` — operator-card, operator-header, stat-row, component-section, component-header with toggle, component-body, exp-history-row, mini-badge with colored dot.

- [ ] **Step 5: Run tests to verify they pass**

```bash
cd dashboard/ui && npx vitest run src/pages/Operators.test.tsx src/components/components.test.tsx
```

- [ ] **Step 6: Commit**

```bash
git add dashboard/ui/src/pages/Operators.tsx dashboard/ui/src/pages/Operators.css dashboard/ui/src/pages/Operators.test.tsx dashboard/ui/src/components/CoverageMatrix.tsx dashboard/ui/src/components/CoverageMatrix.css dashboard/ui/src/components/components.test.tsx
git commit -m "feat(dashboard): add Operators page with component accordion and coverage matrix"
```

---

### Task 6: Knowledge Page

**Files:**
- Create: `dashboard/ui/src/pages/Knowledge.tsx`
- Create: `dashboard/ui/src/pages/Knowledge.css`
- Create: `dashboard/ui/src/pages/Knowledge.test.tsx`

**Context:** The Knowledge page shows an operator's component dependency graph. It fetches operator/component lists and the component model from `GET /knowledge/:operator/:component` (returns `ComponentModel`). See `knowledge.html` mockup. The graph is rendered as inline SVG with nodes for each managed resource, colored by chaos coverage status. A side panel shows resource details.

**Key UI elements:**
- Toolbar with Operator and Component dropdown selectors
- SVG dependency graph: nodes for each ManagedResource (positioned by a simple layout algorithm), colored by chaos coverage (Resilient=green, Degraded=yellow, Failed=red, untested=gray), experiment count badges on nodes, arrow connections from deployment to managed resources
- Side panel (right): component info (operator, namespace), managed resources list with Kind icon + name + coverage tag (covered/partial/uncovered), chaos coverage summary (tested count / untested count)
- Legend bar below graph
- Zoom/pan via SVG viewBox manipulation (zoom buttons, not scroll)
- Loading/empty/error states

**Simplification:** Use a deterministic layout algorithm (central deployment node, resources arranged in a circle or grid around it) rather than a force-directed graph library. This avoids adding a dependency.

- [ ] **Step 1: Write tests**

Create `dashboard/ui/src/pages/Knowledge.test.tsx`:

```typescript
import { render, screen } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { Knowledge } from './Knowledge';
import { vi } from 'vitest';

vi.mock('../api/hooks', () => ({
  useApi: vi.fn(() => ({ data: null, loading: false, error: null, refetch: vi.fn() })),
}));

import { useApi } from '../api/hooks';

function renderKnowledge() {
  return render(
    <MemoryRouter>
      <Knowledge />
    </MemoryRouter>
  );
}

describe('Knowledge', () => {
  it('shows loading state', () => {
    (useApi as ReturnType<typeof vi.fn>).mockReturnValue({
      data: null, loading: true, error: null, refetch: vi.fn(),
    });
    renderKnowledge();
    expect(screen.getByText(/loading/i)).toBeInTheDocument();
  });

  it('renders operator selector', () => {
    (useApi as ReturnType<typeof vi.fn>).mockImplementation((url: string) => {
      if (url && url.includes('/operators')) {
        return { data: ['opendatahub-operator'], loading: false, error: null, refetch: vi.fn() };
      }
      return { data: null, loading: false, error: null, refetch: vi.fn() };
    });
    renderKnowledge();
    expect(screen.getByText(/operator/i)).toBeInTheDocument();
  });

  it('renders graph area when component data loaded', () => {
    (useApi as ReturnType<typeof vi.fn>).mockImplementation((url: string) => {
      if (url && url.includes('/operators') && !url.includes('/components')) {
        return { data: ['op1'], loading: false, error: null, refetch: vi.fn() };
      }
      if (url && url.includes('/components')) {
        return { data: ['comp1'], loading: false, error: null, refetch: vi.fn() };
      }
      if (url && url.includes('/knowledge')) {
        return {
          data: {
            name: 'comp1', controller: 'ctrl',
            managedResources: [
              { apiVersion: 'apps/v1', kind: 'Deployment', name: 'deploy1' },
              { apiVersion: 'v1', kind: 'ConfigMap', name: 'cm1' },
            ],
          },
          loading: false, error: null, refetch: vi.fn(),
        };
      }
      return { data: null, loading: false, error: null, refetch: vi.fn() };
    });
    renderKnowledge();
    expect(screen.getByText('Dependency Graph')).toBeInTheDocument();
  });

  it('shows empty state when no knowledge data', () => {
    (useApi as ReturnType<typeof vi.fn>).mockImplementation((url: string) => {
      if (url && url.includes('/operators') && !url.includes('/components')) {
        return { data: ['op1'], loading: false, error: null, refetch: vi.fn() };
      }
      if (url && url.includes('/components')) {
        return { data: ['comp1'], loading: false, error: null, refetch: vi.fn() };
      }
      return { data: null, loading: false, error: null, refetch: vi.fn() };
    });
    renderKnowledge();
    expect(screen.getByText(/select.*component/i)).toBeInTheDocument();
  });
});
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
cd dashboard/ui && npx vitest run src/pages/Knowledge.test.tsx
```

- [ ] **Step 3: Implement Knowledge page**

Create `dashboard/ui/src/pages/Knowledge.tsx` and `Knowledge.css`.

The page should:
- Toolbar: Operator dropdown (from `useApi<string[]>(apiUrl('/operators'))`), Component dropdown (from `useApi<string[]>(apiUrl('/operators/' + operator + '/components'))`, only when operator selected)
- When both selected, fetch `useApi<ComponentModel>(apiUrl('/knowledge/' + operator + '/' + component))`
- Also fetch experiments for coverage overlay: `useApi<ListResult>(apiUrl('/experiments', { operator, component: comp, pageSize: 1000 }))`
- SVG graph area:
  - Central node for the component's controller (Deployment)
  - Managed resources arranged around it (deterministic grid/radial layout)
  - Each node: rect with Kind label + resource name, colored by coverage (match resource name against experiments to determine verdict)
  - Experiment count badge on each node
  - Lines connecting controller to resources
- Side panel (right column):
  - Component info section (operator, namespace from first experiment)
  - Managed resources list with Kind icon (colored by resource type), name, namespace, coverage tag
  - Chaos coverage summary: X tested / Y untested
- Legend below graph: color meanings
- Zoom controls: +/- buttons adjusting SVG viewBox
- Empty state when no operator/component selected

Kind icon colors (matching mockup):
- Deployment: `#06c`, ConfigMap: `#3e8635`, ServiceAccount: `#8a8d90`
- ClusterRole/ClusterRoleBinding: `#6753ac`, Service: `#f0ab00`
- ValidatingWebhookConfig/MutatingWebhookConfig: `#c9190b`, CRD: `#004080`

Create `dashboard/ui/src/pages/Knowledge.css` — content-area 2-column grid (1fr 360px), graph-card, graph-header with zoom controls, graph-area with SVG styling, node-box, detail-panel, panel-section, resource-row with Kind icons, legend bar, coverage-tag.

- [ ] **Step 4: Run tests to verify they pass**

```bash
cd dashboard/ui && npx vitest run src/pages/Knowledge.test.tsx
```

- [ ] **Step 5: Commit**

```bash
git add dashboard/ui/src/pages/Knowledge.tsx dashboard/ui/src/pages/Knowledge.css dashboard/ui/src/pages/Knowledge.test.tsx
git commit -m "feat(dashboard): add Knowledge page with SVG dependency graph"
```

---

### Task 7: Wire Routes, Rebuild, and Final Verification

**Files:**
- Modify: `dashboard/ui/src/App.tsx`
- Modify: `dashboard/ui/src/App.test.tsx`

**Context:** Replace the Placeholder components in App.tsx with the real page components. Rebuild the frontend and Go binary to update the embedded assets.

- [ ] **Step 1: Update App.tsx**

Replace placeholder imports and routes:

```typescript
import { Live } from './pages/Live';
import { Suites } from './pages/Suites';
import { Operators } from './pages/Operators';
import { Knowledge } from './pages/Knowledge';
```

Remove the `Placeholder` component. Update routes:

```typescript
<Route path="live" element={<Live />} />
<Route path="suites" element={<Suites />} />
<Route path="operators" element={<Operators />} />
<Route path="knowledge" element={<Knowledge />} />
```

- [ ] **Step 2: Update App.test.tsx**

Ensure App tests still pass — they test sidebar nav rendering and route existence, not page content. May need to mock `useApi` and `useSSE` at the App level if the real pages try to fetch on render.

- [ ] **Step 3: Run all frontend tests**

```bash
cd dashboard/ui && npx vitest run
```

Expected: All tests pass

- [ ] **Step 4: Build frontend**

```bash
cd dashboard/ui && npm run build
```

- [ ] **Step 5: Build Go binary with embedded assets**

```bash
cd /path/to/odh-platform-chaos && go build -o chaos-dashboard ./dashboard/cmd/dashboard/
```

- [ ] **Step 6: Run all backend tests**

```bash
go test ./dashboard/...
```

- [ ] **Step 7: Commit**

```bash
git add dashboard/ui/src/App.tsx dashboard/ui/src/App.test.tsx
git commit -m "feat(dashboard): wire Phase 2 pages and rebuild embedded assets"
```

- [ ] **Step 8: Clean up build artifact**

```bash
rm -f chaos-dashboard
```
