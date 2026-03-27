# Chaos Dashboard Frontend Implementation Plan (Phase 1)

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build the React frontend for the chaos dashboard covering the app shell, Overview, Experiments List, and Experiment Detail views.

**Architecture:** Vite-bundled React 18 SPA using PatternFly 5 for UI components. Custom fetch-based API client with React hooks. Go `embed` serves the built assets from the dashboard binary. This plan covers foundation + 3 core views; advanced views (Live, Suites, Operators, Knowledge) are deferred to Phase 2.

**Tech Stack:** React 18, TypeScript, PatternFly 5, Vite, Vitest + React Testing Library, react-router-dom v6

---

## Spec Reference

- Design spec: `docs/superpowers/specs/2026-03-25-chaos-dashboard-design.md`
- HTML mockups: `.superpowers/brainstorm/80649-1774450280/` (overview.html, experiments-list.html, experiment-detail.html)
- Backend API: `dashboard/internal/api/` (all endpoints are `GET /api/v1/...`)
- Backend types: `dashboard/internal/store/store.go` (JSON tags define API response shapes)

## File Structure

```
dashboard/ui/
  index.html                          # Vite entry HTML
  package.json                        # Dependencies and scripts
  tsconfig.json                       # TypeScript config
  tsconfig.node.json                  # TS config for Vite/node files
  vite.config.ts                      # Vite build config (outDir: ../ui-dist)
  vitest.setup.ts                     # Test setup (PF5 mocks)
  src/
    main.tsx                          # React DOM entry
    App.tsx                           # Router + Layout wrapper
    App.css                           # Global overrides + design tokens
    types/
      api.ts                          # TS types mirroring backend JSON
    api/
      client.ts                       # Fetch wrappers (get, buildUrl)
      hooks.ts                        # useApi hook (fetch + loading/error state)
    components/
      Layout.tsx                      # Sidebar nav + content area
      Layout.css                      # Sidebar + nav styles
      VerdictBadge.tsx                # Verdict pill badge
      PhaseBadge.tsx                  # Phase pill badge
      StatusBanner.tsx                # Info/warning/error banner
      TrendIndicator.tsx              # Trend arrow with delta value
    pages/
      Overview.tsx                    # Overview dashboard page
      Overview.css                    # Overview grid + card styles
      ExperimentsList.tsx             # Filterable experiments table
      ExperimentsList.css             # Table + toolbar styles
      ExperimentDetail.tsx            # Detail page with 7 tabs
      ExperimentDetail.css            # Detail + tab styles
dashboard/
  embed.go                           # go:embed directive for ui-dist/
```

## Backend API Endpoints (consumed by frontend)

| Endpoint | Response Shape | Used By |
|----------|---------------|---------|
| `GET /api/v1/overview/stats?since=` | `{ total, resilient, degraded, failed, inconclusive, running, trends, verdictTimeline, avgRecoveryByType, runningExperiments }` | Overview |
| `GET /api/v1/experiments?namespace=&operator=&...&page=&pageSize=` | `{ items: Experiment[], totalCount: number }` | ExperimentsList, Overview (recent) |
| `GET /api/v1/experiments/:namespace/:name` | `Experiment` | ExperimentDetail |
| `GET /api/v1/operators` | `string[]` | ExperimentsList (filter options) |
| `GET /api/v1/operators/:name/components` | `string[]` | ExperimentsList (filter options) |

## Experiment JSON Shape (from `store.go` JSON tags)

```typescript
interface Experiment {
  id: string;
  name: string;
  namespace: string;
  operator: string;
  component: string;
  type: string;           // InjectionType enum
  phase: string;          // ExperimentPhase enum
  verdict?: string;       // Verdict enum
  dangerLevel?: string;   // DangerLevel enum
  recoveryMs?: number;
  startTime?: string;     // RFC3339
  endTime?: string;
  suiteName?: string;
  suiteRunId?: string;
  operatorVersion?: string;
  cleanupError?: string;
  specJson: string;       // raw JSON string
  statusJson: string;     // raw JSON string
  createdAt: string;
  updatedAt: string;
}
```

---

### Task 1: Vite Project Scaffold

**Files:**
- Create: `dashboard/ui/index.html`
- Create: `dashboard/ui/package.json`
- Create: `dashboard/ui/tsconfig.json`
- Create: `dashboard/ui/tsconfig.node.json`
- Create: `dashboard/ui/vite.config.ts`
- Create: `dashboard/ui/vitest.setup.ts`
- Create: `dashboard/ui/src/main.tsx`

**Context:** This scaffolds an empty Vite + React + TypeScript project with PatternFly 5 and Vitest configured. The Vite dev server proxies `/api` to `localhost:8080` (the Go backend). Build output goes to `../ui-dist/` for Go embed.

- [ ] **Step 1: Create package.json**

```json
{
  "name": "chaos-dashboard",
  "private": true,
  "version": "0.0.1",
  "type": "module",
  "scripts": {
    "dev": "vite",
    "build": "tsc -b && vite build",
    "preview": "vite preview",
    "test": "vitest run",
    "test:watch": "vitest"
  },
  "dependencies": {
    "@patternfly/patternfly": "^5.4.0",
    "@patternfly/react-core": "^5.4.0",
    "@patternfly/react-icons": "^5.4.0",
    "@patternfly/react-table": "^5.4.0",
    "react": "^18.3.1",
    "react-dom": "^18.3.1",
    "react-router-dom": "^6.28.0"
  },
  "devDependencies": {
    "@testing-library/jest-dom": "^6.6.0",
    "@testing-library/react": "^16.0.0",
    "@types/react": "^18.3.0",
    "@types/react-dom": "^18.3.0",
    "@vitejs/plugin-react": "^4.3.0",
    "jsdom": "^25.0.0",
    "typescript": "^5.6.0",
    "vite": "^5.4.0",
    "vitest": "^2.1.0"
  }
}
```

- [ ] **Step 2: Create tsconfig.json**

```json
{
  "compilerOptions": {
    "target": "ES2020",
    "useDefineForClassFields": true,
    "lib": ["ES2020", "DOM", "DOM.Iterable"],
    "module": "ESNext",
    "skipLibCheck": true,
    "moduleResolution": "bundler",
    "allowImportingTsExtensions": true,
    "isolatedModules": true,
    "moduleDetection": "force",
    "noEmit": true,
    "jsx": "react-jsx",
    "strict": true,
    "noUnusedLocals": true,
    "noUnusedParameters": true,
    "noFallthroughCasesInSwitch": true,
    "noUncheckedIndexedAccess": true
  },
  "include": ["src"]
}
```

- [ ] **Step 3: Create tsconfig.node.json**

```json
{
  "compilerOptions": {
    "target": "ES2022",
    "lib": ["ES2023"],
    "module": "ESNext",
    "skipLibCheck": true,
    "moduleResolution": "bundler",
    "allowImportingTsExtensions": true,
    "isolatedModules": true,
    "moduleDetection": "force",
    "noEmit": true,
    "strict": true
  },
  "include": ["vite.config.ts"]
}
```

- [ ] **Step 4: Create vite.config.ts**

```typescript
import { defineConfig } from 'vite';
import react from '@vitejs/plugin-react';

export default defineConfig({
  plugins: [react()],
  build: {
    outDir: '../ui-dist',
    emptyOutDir: true,
  },
  server: {
    proxy: {
      '/api': 'http://localhost:8080',
    },
  },
  test: {
    globals: true,
    environment: 'jsdom',
    setupFiles: './vitest.setup.ts',
    css: false,
  },
});
```

- [ ] **Step 5: Create vitest.setup.ts**

```typescript
import '@testing-library/jest-dom/vitest';
```

- [ ] **Step 6: Create index.html**

```html
<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8" />
  <meta name="viewport" content="width=device-width, initial-scale=1.0" />
  <title>ODH Chaos Dashboard</title>
</head>
<body>
  <div id="root"></div>
  <script type="module" src="/src/main.tsx"></script>
</body>
</html>
```

- [ ] **Step 7: Create src/main.tsx**

```typescript
import React from 'react';
import ReactDOM from 'react-dom/client';
import '@patternfly/patternfly/patternfly.min.css';
import { App } from './App';
import './App.css';

ReactDOM.createRoot(document.getElementById('root')!).render(
  <React.StrictMode>
    <App />
  </React.StrictMode>
);
```

- [ ] **Step 8: Create minimal src/App.tsx placeholder**

```typescript
export function App() {
  return <div>Chaos Dashboard</div>;
}
```

- [ ] **Step 9: Create empty src/App.css**

```css
/* Global styles and design tokens — populated in Task 5 */
```

- [ ] **Step 10: Install dependencies and verify build**

```bash
cd dashboard/ui && npm install && npm run build
```

Expected: Build succeeds, output in `dashboard/ui-dist/`

- [ ] **Step 11: Verify tests run**

```bash
cd dashboard/ui && npm test
```

Expected: "No test files found" or passes (no tests yet)

- [ ] **Step 12: Add ui/node_modules and ui-dist to .gitignore, commit**

Add to the project root `.gitignore`:
```
dashboard/ui/node_modules/
dashboard/ui-dist/
```

```bash
git add dashboard/ui/ .gitignore
git commit -m "feat(dashboard): scaffold Vite + React + PF5 frontend"
```

---

### Task 2: TypeScript Types

**Files:**
- Create: `dashboard/ui/src/types/api.ts`
- Test: `dashboard/ui/src/types/api.test.ts`

**Context:** Define TypeScript types that mirror the backend JSON response shapes. These are the contract between frontend and backend. All field names must match the `json:"..."` tags in `dashboard/internal/store/store.go`.

- [ ] **Step 1: Write the test**

```typescript
// src/types/api.test.ts
import { describe, it, expect } from 'vitest';
import type { Experiment, OverviewStats, ListResult, SuiteRun, TrendStats, DayVerdicts } from './api';
import { VERDICTS, PHASES, INJECTION_TYPES, DANGER_LEVELS, phaseDisplayName } from './api';

describe('API types', () => {
  it('defines all injection types', () => {
    expect(INJECTION_TYPES).toHaveLength(8);
    expect(INJECTION_TYPES).toContain('PodKill');
    expect(INJECTION_TYPES).toContain('ClientFault');
  });

  it('defines all phases', () => {
    expect(PHASES).toHaveLength(8);
    expect(PHASES).toContain('Pending');
    expect(PHASES).toContain('Aborted');
  });

  it('defines all verdicts', () => {
    expect(VERDICTS).toHaveLength(4);
    expect(VERDICTS).toContain('Resilient');
    expect(VERDICTS).toContain('Inconclusive');
  });

  it('maps phase to display name', () => {
    expect(phaseDisplayName('SteadyStatePre')).toBe('Pre-check');
    expect(phaseDisplayName('SteadyStatePost')).toBe('Post-check');
    expect(phaseDisplayName('Injecting')).toBe('Injecting');
    expect(phaseDisplayName('Unknown')).toBe('Unknown');
  });

  it('type-checks an Experiment shape', () => {
    const exp: Experiment = {
      id: 'ns/name/2026-01-01T00:00:00Z',
      name: 'test',
      namespace: 'ns',
      operator: 'op',
      component: 'comp',
      type: 'PodKill',
      phase: 'Complete',
      specJson: '{}',
      statusJson: '{}',
      createdAt: '2026-01-01T00:00:00Z',
      updatedAt: '2026-01-01T00:00:00Z',
    };
    expect(exp.name).toBe('test');
  });
});
```

- [ ] **Step 2: Run test to verify it fails**

```bash
cd dashboard/ui && npx vitest run src/types/api.test.ts
```

Expected: FAIL — module not found

- [ ] **Step 3: Implement types**

```typescript
// src/types/api.ts

export const INJECTION_TYPES = [
  'PodKill', 'NetworkPartition', 'CRDMutation', 'ConfigDrift',
  'WebhookDisrupt', 'RBACRevoke', 'FinalizerBlock', 'ClientFault',
] as const;
export type InjectionType = typeof INJECTION_TYPES[number];

export const PHASES = [
  'Pending', 'SteadyStatePre', 'Injecting', 'Observing',
  'SteadyStatePost', 'Evaluating', 'Complete', 'Aborted',
] as const;
export type ExperimentPhase = typeof PHASES[number];

export const VERDICTS = ['Resilient', 'Degraded', 'Failed', 'Inconclusive'] as const;
export type Verdict = typeof VERDICTS[number];

export const DANGER_LEVELS = ['low', 'medium', 'high'] as const;
export type DangerLevel = typeof DANGER_LEVELS[number];

const PHASE_DISPLAY: Record<string, string> = {
  Pending: 'Pending',
  SteadyStatePre: 'Pre-check',
  Injecting: 'Injecting',
  Observing: 'Observing',
  SteadyStatePost: 'Post-check',
  Evaluating: 'Evaluating',
  Complete: 'Complete',
  Aborted: 'Aborted',
};

export function phaseDisplayName(phase: string): string {
  return PHASE_DISPLAY[phase] ?? phase;
}

export interface Experiment {
  id: string;
  name: string;
  namespace: string;
  operator: string;
  component: string;
  type: string;
  phase: string;
  verdict?: string;
  dangerLevel?: string;
  recoveryMs?: number;
  startTime?: string;
  endTime?: string;
  suiteName?: string;
  suiteRunId?: string;
  operatorVersion?: string;
  cleanupError?: string;
  specJson: string;
  statusJson: string;
  createdAt: string;
  updatedAt: string;
}

export interface ListResult {
  items: Experiment[];
  totalCount: number;
}

export interface TrendStats {
  total: number;
  resilient: number;
  degraded: number;
  failed: number;
}

export interface DayVerdicts {
  date: string;
  resilient: number;
  degraded: number;
  failed: number;
}

export interface RunningExperimentSummary {
  name: string;
  namespace: string;
  phase: string;
  component: string;
  type: string;
}

export interface OverviewStats {
  total: number;
  resilient: number;
  degraded: number;
  failed: number;
  inconclusive: number;
  running: number;
  trends: TrendStats | null;
  verdictTimeline: DayVerdicts[] | null;
  avgRecoveryByType: Record<string, number>;
  runningExperiments: RunningExperimentSummary[];
}

export interface SuiteRun {
  suiteName: string;
  suiteRunId: string;
  operatorVersion: string;
  total: number;
  resilient: number;
  degraded: number;
  failed: number;
}
```

- [ ] **Step 4: Run test to verify it passes**

```bash
cd dashboard/ui && npx vitest run src/types/api.test.ts
```

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add dashboard/ui/src/types/
git commit -m "feat(dashboard): add TypeScript types mirroring backend API"
```

---

### Task 3: API Client and Hooks

**Files:**
- Create: `dashboard/ui/src/api/client.ts`
- Create: `dashboard/ui/src/api/hooks.ts`
- Test: `dashboard/ui/src/api/hooks.test.ts`

**Context:** A lightweight fetch-based API client with a `useApi` React hook that manages loading/error/data state. No external dependencies (no React Query). The hook accepts a URL string and returns `{ data, loading, error, refetch }`.

- [ ] **Step 1: Write the test**

```typescript
// src/api/hooks.test.ts
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { renderHook, waitFor } from '@testing-library/react';
import { useApi } from './hooks';

describe('useApi', () => {
  beforeEach(() => {
    vi.restoreAllMocks();
  });

  it('fetches data and returns it', async () => {
    const mockData = { total: 10, resilient: 5 };
    vi.spyOn(globalThis, 'fetch').mockResolvedValue({
      ok: true,
      json: () => Promise.resolve(mockData),
    } as Response);

    const { result } = renderHook(() => useApi<typeof mockData>('/api/v1/overview/stats'));

    expect(result.current.loading).toBe(true);

    await waitFor(() => expect(result.current.loading).toBe(false));

    expect(result.current.data).toEqual(mockData);
    expect(result.current.error).toBeNull();
  });

  it('sets error on fetch failure', async () => {
    vi.spyOn(globalThis, 'fetch').mockResolvedValue({
      ok: false,
      status: 500,
      json: () => Promise.resolve({ error: 'internal error' }),
    } as unknown as Response);

    const { result } = renderHook(() => useApi('/api/v1/overview/stats'));

    await waitFor(() => expect(result.current.loading).toBe(false));

    expect(result.current.data).toBeNull();
    expect(result.current.error).toBe('internal error');
  });

  it('does not fetch when url is null', () => {
    vi.spyOn(globalThis, 'fetch');

    renderHook(() => useApi(null));

    expect(globalThis.fetch).not.toHaveBeenCalled();
  });
});
```

- [ ] **Step 2: Run test to verify it fails**

```bash
cd dashboard/ui && npx vitest run src/api/hooks.test.ts
```

Expected: FAIL — module not found

- [ ] **Step 3: Implement client.ts**

```typescript
// src/api/client.ts
const BASE = '/api/v1';

export function apiUrl(path: string, params?: Record<string, string | number | undefined>): string {
  const url = new URL(`${BASE}${path}`, window.location.origin);
  if (params) {
    for (const [key, value] of Object.entries(params)) {
      if (value !== undefined && value !== '') {
        url.searchParams.set(key, String(value));
      }
    }
  }
  return url.pathname + url.search;
}

export async function apiFetch<T>(url: string): Promise<T> {
  const res = await fetch(url);
  if (!res.ok) {
    const body = await res.json().catch(() => ({}));
    throw new Error((body as { error?: string }).error ?? `HTTP ${res.status}`);
  }
  return res.json() as Promise<T>;
}
```

- [ ] **Step 4: Implement hooks.ts**

```typescript
// src/api/hooks.ts
import { useState, useEffect, useCallback, useRef } from 'react';
import { apiFetch } from './client';

interface ApiState<T> {
  data: T | null;
  loading: boolean;
  error: string | null;
  refetch: () => void;
}

export function useApi<T>(url: string | null): ApiState<T> {
  const [data, setData] = useState<T | null>(null);
  const [loading, setLoading] = useState(url !== null);
  const [error, setError] = useState<string | null>(null);
  const mountedRef = useRef(true);

  const fetchData = useCallback(async () => {
    if (url === null) return;
    setLoading(true);
    setError(null);
    try {
      const result = await apiFetch<T>(url);
      if (mountedRef.current) {
        setData(result);
        setLoading(false);
      }
    } catch (err) {
      if (mountedRef.current) {
        setError(err instanceof Error ? err.message : 'Unknown error');
        setData(null);
        setLoading(false);
      }
    }
  }, [url]);

  useEffect(() => {
    mountedRef.current = true;
    fetchData();
    return () => { mountedRef.current = false; };
  }, [fetchData]);

  return { data, loading, error, refetch: fetchData };
}
```

- [ ] **Step 5: Run test to verify it passes**

```bash
cd dashboard/ui && npx vitest run src/api/hooks.test.ts
```

Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add dashboard/ui/src/api/
git commit -m "feat(dashboard): add API client and useApi hook"
```

---

### Task 4: App Shell (Layout + Routing)

**Files:**
- Create: `dashboard/ui/src/components/Layout.tsx`
- Create: `dashboard/ui/src/components/Layout.css`
- Modify: `dashboard/ui/src/App.tsx`
- Modify: `dashboard/ui/src/App.css`
- Test: `dashboard/ui/src/App.test.tsx`

**Context:** The app shell has a dark sidebar (220px, `#212427`) with grouped navigation (Monitor, Experiments, Insights) and a content area. Uses react-router-dom v6 for routing. The sidebar highlights the active route. Placeholder pages for all 7 routes — the real page components come in later tasks.

Reference mockup: `.superpowers/brainstorm/80649-1774450280/overview.html` for sidebar structure.

- [ ] **Step 1: Write the test**

```typescript
// src/App.test.tsx
import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import { App } from './App';

describe('App shell', () => {
  it('renders the sidebar with navigation groups', () => {
    render(<App />);
    expect(screen.getByText('Monitor')).toBeInTheDocument();
    expect(screen.getByText('Experiments')).toBeInTheDocument();
    expect(screen.getByText('Insights')).toBeInTheDocument();
  });

  it('renders navigation links', () => {
    render(<App />);
    expect(screen.getByRole('link', { name: /overview/i })).toBeInTheDocument();
    expect(screen.getByRole('link', { name: /live/i })).toBeInTheDocument();
    expect(screen.getByRole('link', { name: /all experiments/i })).toBeInTheDocument();
  });

  it('renders the overview page at root route', () => {
    render(<App />);
    expect(screen.getByText(/overview/i)).toBeInTheDocument();
  });
});
```

- [ ] **Step 2: Run test to verify it fails**

```bash
cd dashboard/ui && npx vitest run src/App.test.tsx
```

Expected: FAIL

- [ ] **Step 3: Implement Layout.css**

```css
/* src/components/Layout.css */
.app-layout {
  display: flex;
  min-height: 100vh;
}

.app-sidebar {
  width: 220px;
  min-width: 220px;
  background: #212427;
  color: white;
  display: flex;
  flex-direction: column;
}

.app-logo {
  padding: 16px;
  font-size: 14px;
  font-weight: 700;
  border-bottom: 1px solid rgba(255, 255, 255, 0.1);
}

.app-logo span {
  color: #e00;
}

.nav-section-title {
  font-size: 11px;
  text-transform: uppercase;
  letter-spacing: 1px;
  color: rgba(255, 255, 255, 0.5);
  padding: 16px 16px 6px;
  font-weight: 600;
}

.nav-item {
  padding: 10px 16px 10px 24px;
  font-size: 14px;
  color: rgba(255, 255, 255, 0.8);
  cursor: pointer;
  display: flex;
  align-items: center;
  gap: 10px;
  text-decoration: none;
}

.nav-item:hover {
  background: rgba(255, 255, 255, 0.08);
  color: rgba(255, 255, 255, 0.8);
}

.nav-item.active {
  background: rgba(255, 255, 255, 0.12);
  color: white;
  border-left: 3px solid #06c;
  padding-left: 21px;
}

.app-content {
  flex: 1;
  overflow: auto;
  background: #f0f0f0;
}
```

- [ ] **Step 4: Implement Layout.tsx**

```tsx
// src/components/Layout.tsx
import { NavLink, Outlet } from 'react-router-dom';
import './Layout.css';

const NAV_GROUPS = [
  {
    title: 'Monitor',
    items: [
      { label: 'Overview', to: '/' },
      { label: 'Live', to: '/live' },
    ],
  },
  {
    title: 'Experiments',
    items: [
      { label: 'All Experiments', to: '/experiments' },
      { label: 'Suites', to: '/suites' },
    ],
  },
  {
    title: 'Insights',
    items: [
      { label: 'Operators', to: '/operators' },
      { label: 'Knowledge', to: '/knowledge' },
    ],
  },
];

export function Layout() {
  return (
    <div className="app-layout">
      <nav className="app-sidebar">
        <div className="app-logo">
          <span>ODH</span> Chaos Dashboard
        </div>
        {NAV_GROUPS.map((group) => (
          <div key={group.title}>
            <div className="nav-section-title">{group.title}</div>
            {group.items.map((item) => (
              <NavLink
                key={item.to}
                to={item.to}
                end={item.to === '/'}
                className={({ isActive }) => `nav-item${isActive ? ' active' : ''}`}
              >
                {item.label}
              </NavLink>
            ))}
          </div>
        ))}
      </nav>
      <main className="app-content">
        <Outlet />
      </main>
    </div>
  );
}
```

- [ ] **Step 5: Implement App.tsx with routes**

```tsx
// src/App.tsx
import { BrowserRouter, Routes, Route } from 'react-router-dom';
import { Layout } from './components/Layout';

function Placeholder({ title }: { title: string }) {
  return <div style={{ padding: 24 }}><h1>{title}</h1></div>;
}

export function App() {
  return (
    <BrowserRouter>
      <Routes>
        <Route element={<Layout />}>
          <Route index element={<Placeholder title="Overview" />} />
          <Route path="live" element={<Placeholder title="Live" />} />
          <Route path="experiments" element={<Placeholder title="All Experiments" />} />
          <Route path="experiments/:namespace/:name" element={<Placeholder title="Experiment Detail" />} />
          <Route path="suites" element={<Placeholder title="Suites" />} />
          <Route path="operators" element={<Placeholder title="Operators" />} />
          <Route path="knowledge" element={<Placeholder title="Knowledge" />} />
        </Route>
      </Routes>
    </BrowserRouter>
  );
}
```

- [ ] **Step 6: Populate App.css with design tokens**

```css
/* src/App.css */

/* Design tokens from spec */
:root {
  --color-resilient-bg: #e6f9e6;
  --color-resilient-fg: #1e4f18;
  --color-degraded-bg: #fef3cd;
  --color-degraded-fg: #795600;
  --color-failed-bg: #fce8e6;
  --color-failed-fg: #7d1007;
  --color-running-bg: #e7f1fa;
  --color-running-fg: #004080;
  --color-complete-bg: #e8e8e8;
  --color-complete-fg: #151515;
  --color-aborted-bg: #f0f0f0;
  --color-aborted-fg: #6a6e73;
  --color-inconclusive-bg: #f5f0ff;
  --color-inconclusive-fg: #6753ac;
  --color-pending-bg: #f0f0f0;
  --color-pending-fg: #6a6e73;
  --color-primary: #06c;
  --color-danger: #c9190b;
  --color-trend-up: #3e8635;
  --color-trend-down: #c9190b;
  --card-shadow: 0 1px 2px rgba(0, 0, 0, 0.08);
}

body {
  margin: 0;
}

/* Badge base */
.badge {
  display: inline-block;
  padding: 3px 10px;
  border-radius: 12px;
  font-size: 11px;
  font-weight: 600;
  white-space: nowrap;
}
```

- [ ] **Step 7: Run test to verify it passes**

```bash
cd dashboard/ui && npx vitest run src/App.test.tsx
```

Expected: PASS

- [ ] **Step 8: Verify dev server starts**

```bash
cd dashboard/ui && npx vite --host 0.0.0.0 &
# Visit http://localhost:5173 — should show sidebar with nav groups
kill %1
```

- [ ] **Step 9: Commit**

```bash
git add dashboard/ui/src/
git commit -m "feat(dashboard): add app shell with sidebar navigation and routing"
```

---

### Task 5: Shared Components (Badges, Status Banner, Trend Indicator)

**Files:**
- Create: `dashboard/ui/src/components/VerdictBadge.tsx`
- Create: `dashboard/ui/src/components/PhaseBadge.tsx`
- Create: `dashboard/ui/src/components/StatusBanner.tsx`
- Create: `dashboard/ui/src/components/TrendIndicator.tsx`
- Test: `dashboard/ui/src/components/components.test.tsx`

**Context:** Reusable UI atoms used across all views. Colors and styles come from the design tokens in App.css. Reference: spec section "UI Design Tokens" and mockup badge classes.

- [ ] **Step 1: Write the test**

```typescript
// src/components/components.test.tsx
import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import { VerdictBadge } from './VerdictBadge';
import { PhaseBadge } from './PhaseBadge';
import { StatusBanner } from './StatusBanner';
import { TrendIndicator } from './TrendIndicator';

describe('VerdictBadge', () => {
  it('renders verdict text with correct class', () => {
    const { container } = render(<VerdictBadge verdict="Resilient" />);
    const badge = container.querySelector('.badge');
    expect(badge).toHaveTextContent('Resilient');
    expect(badge).toHaveClass('badge-resilient');
  });

  it('renders nothing when verdict is empty', () => {
    const { container } = render(<VerdictBadge verdict="" />);
    expect(container.firstChild).toBeNull();
  });
});

describe('PhaseBadge', () => {
  it('renders display name for CRD phase', () => {
    render(<PhaseBadge phase="SteadyStatePre" />);
    expect(screen.getByText('Pre-check')).toBeInTheDocument();
  });
});

describe('StatusBanner', () => {
  it('renders error variant', () => {
    render(<StatusBanner variant="error" message="Something broke" />);
    expect(screen.getByText('Something broke')).toBeInTheDocument();
  });

  it('renders nothing when message is empty', () => {
    const { container } = render(<StatusBanner variant="info" message="" />);
    expect(container.firstChild).toBeNull();
  });
});

describe('TrendIndicator', () => {
  it('shows positive trend with up arrow', () => {
    render(<TrendIndicator value={5} goodDirection="up" />);
    expect(screen.getByText(/\+5/)).toBeInTheDocument();
  });

  it('shows negative trend with down arrow', () => {
    render(<TrendIndicator value={-3} goodDirection="up" />);
    expect(screen.getByText(/-3/)).toBeInTheDocument();
  });

  it('shows zero as neutral', () => {
    render(<TrendIndicator value={0} goodDirection="up" />);
    expect(screen.getByText('0')).toBeInTheDocument();
  });
});
```

- [ ] **Step 2: Run test to verify it fails**

```bash
cd dashboard/ui && npx vitest run src/components/components.test.tsx
```

Expected: FAIL

- [ ] **Step 3: Implement VerdictBadge**

```tsx
// src/components/VerdictBadge.tsx
interface Props {
  verdict: string;
}

export function VerdictBadge({ verdict }: Props) {
  if (!verdict) return null;
  const cls = `badge badge-${verdict.toLowerCase()}`;
  return <span className={cls}>{verdict}</span>;
}
```

Add to `App.css`:
```css
.badge-resilient { background: var(--color-resilient-bg); color: var(--color-resilient-fg); }
.badge-degraded { background: var(--color-degraded-bg); color: var(--color-degraded-fg); }
.badge-failed { background: var(--color-failed-bg); color: var(--color-failed-fg); }
.badge-inconclusive { background: var(--color-inconclusive-bg); color: var(--color-inconclusive-fg); }
```

- [ ] **Step 4: Implement PhaseBadge**

```tsx
// src/components/PhaseBadge.tsx
import { phaseDisplayName } from '../types/api';

interface Props {
  phase: string;
}

const PHASE_VARIANT: Record<string, string> = {
  Complete: 'complete',
  Aborted: 'aborted',
  Pending: 'pending',
};

export function PhaseBadge({ phase }: Props) {
  const variant = PHASE_VARIANT[phase] ?? 'running';
  return <span className={`badge badge-${variant}`}>{phaseDisplayName(phase)}</span>;
}
```

Add to `App.css`:
```css
.badge-running { background: var(--color-running-bg); color: var(--color-running-fg); }
.badge-complete { background: var(--color-complete-bg); color: var(--color-complete-fg); }
.badge-aborted { background: var(--color-aborted-bg); color: var(--color-aborted-fg); }
.badge-pending { background: var(--color-pending-bg); color: var(--color-pending-fg); }
```

- [ ] **Step 5: Implement StatusBanner**

```tsx
// src/components/StatusBanner.tsx
import { Alert } from '@patternfly/react-core';

interface Props {
  variant: 'info' | 'warning' | 'error';
  message: string;
}

const PF_VARIANT: Record<string, 'info' | 'warning' | 'danger'> = {
  info: 'info',
  warning: 'warning',
  error: 'danger',
};

export function StatusBanner({ variant, message }: Props) {
  if (!message) return null;
  return (
    <Alert variant={PF_VARIANT[variant]} isInline isPlain title={message} />
  );
}
```

- [ ] **Step 6: Implement TrendIndicator**

```tsx
// src/components/TrendIndicator.tsx
interface Props {
  value: number;
  goodDirection: 'up' | 'down';
}

export function TrendIndicator({ value, goodDirection }: Props) {
  if (value === 0) {
    return <span className="trend-neutral">0</span>;
  }

  const isPositive = value > 0;
  const isGood = goodDirection === 'up' ? isPositive : !isPositive;
  const arrow = isPositive ? '▲' : '▼';
  const colorClass = isGood ? 'trend-good' : 'trend-bad';
  const label = isPositive ? `${arrow} +${value}` : `${arrow} ${value}`;

  return <span className={colorClass}>{label}</span>;
}
```

Add to `App.css`:
```css
.trend-good { color: var(--color-trend-up); font-size: 11px; font-weight: 600; }
.trend-bad { color: var(--color-trend-down); font-size: 11px; font-weight: 600; }
.trend-neutral { color: #6a6e73; font-size: 11px; font-weight: 600; }
```

- [ ] **Step 7: Run test to verify it passes**

```bash
cd dashboard/ui && npx vitest run src/components/components.test.tsx
```

Expected: PASS

- [ ] **Step 8: Commit**

```bash
git add dashboard/ui/src/components/ dashboard/ui/src/App.css
git commit -m "feat(dashboard): add shared badge, banner, and trend components"
```

---

### Task 6: Overview Page

**Files:**
- Create: `dashboard/ui/src/pages/Overview.tsx`
- Create: `dashboard/ui/src/pages/Overview.css`
- Modify: `dashboard/ui/src/App.tsx` (wire in real page component)
- Test: `dashboard/ui/src/pages/Overview.test.tsx`

**Context:** The dashboard home page. Shows 5 stat cards (Total, Resilient, Degraded, Failed, Running) with trend indicators, avg recovery times by injection type, and a running experiments panel. Fetches data from `GET /api/v1/overview/stats`. Reference mockup: `.superpowers/brainstorm/80649-1774450280/overview.html`.

- [ ] **Step 1: Write the test**

```typescript
// src/pages/Overview.test.tsx
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { Overview } from './Overview';

const mockStats = {
  total: 30,
  resilient: 23,
  degraded: 4,
  failed: 1,
  inconclusive: 0,
  running: 2,
  trends: { total: 5, resilient: 3, degraded: 1, failed: -1 },
  verdictTimeline: null,
  avgRecoveryByType: { PodKill: 12000, ConfigDrift: 28000 },
  runningExperiments: [
    { name: 'exp-1', namespace: 'ns', phase: 'Observing', component: 'comp', type: 'PodKill' },
  ],
};

describe('Overview', () => {
  beforeEach(() => {
    vi.restoreAllMocks();
  });

  it('renders stat cards with data', async () => {
    vi.spyOn(globalThis, 'fetch').mockResolvedValue({
      ok: true,
      json: () => Promise.resolve(mockStats),
    } as Response);

    render(<MemoryRouter><Overview /></MemoryRouter>);

    await waitFor(() => {
      expect(screen.getByText('30')).toBeInTheDocument();
    });
    expect(screen.getByText('23')).toBeInTheDocument();
    expect(screen.getByText('4')).toBeInTheDocument();
    expect(screen.getByText('1')).toBeInTheDocument();
    expect(screen.getByText('2')).toBeInTheDocument();
  });

  it('renders running experiments', async () => {
    vi.spyOn(globalThis, 'fetch').mockResolvedValue({
      ok: true,
      json: () => Promise.resolve(mockStats),
    } as Response);

    render(<MemoryRouter><Overview /></MemoryRouter>);

    await waitFor(() => {
      expect(screen.getByText('exp-1')).toBeInTheDocument();
    });
  });

  it('renders avg recovery times', async () => {
    vi.spyOn(globalThis, 'fetch').mockResolvedValue({
      ok: true,
      json: () => Promise.resolve(mockStats),
    } as Response);

    render(<MemoryRouter><Overview /></MemoryRouter>);

    await waitFor(() => {
      expect(screen.getByText('PodKill')).toBeInTheDocument();
      expect(screen.getByText('12.0s')).toBeInTheDocument();
    });
  });

  it('shows spinner while loading', () => {
    vi.spyOn(globalThis, 'fetch').mockReturnValue(new Promise(() => {}));
    render(<MemoryRouter><Overview /></MemoryRouter>);
    expect(screen.getByRole('progressbar')).toBeInTheDocument();
  });

  it('shows error on fetch failure', async () => {
    vi.spyOn(globalThis, 'fetch').mockResolvedValue({
      ok: false,
      status: 500,
      json: () => Promise.resolve({ error: 'db error' }),
    } as unknown as Response);

    render(<MemoryRouter><Overview /></MemoryRouter>);

    await waitFor(() => {
      expect(screen.getByText(/db error/)).toBeInTheDocument();
    });
  });
});
```

- [ ] **Step 2: Run test to verify it fails**

```bash
cd dashboard/ui && npx vitest run src/pages/Overview.test.tsx
```

Expected: FAIL

- [ ] **Step 3: Implement Overview.css**

```css
/* src/pages/Overview.css */
.overview-header {
  padding: 20px 24px;
  background: white;
  border-bottom: 1px solid #d2d2d2;
}

.overview-header h1 {
  margin: 0;
  font-size: 22px;
}

.overview-content {
  padding: 24px;
}

.stat-cards {
  display: grid;
  grid-template-columns: repeat(5, 1fr);
  gap: 16px;
  margin-bottom: 24px;
}

.stat-card {
  background: white;
  border-radius: 8px;
  padding: 20px;
  box-shadow: var(--card-shadow);
  text-align: center;
}

.stat-card .stat-value {
  font-size: 36px;
  font-weight: 700;
}

.stat-card .stat-label {
  font-size: 12px;
  color: #6a6e73;
  text-transform: uppercase;
  letter-spacing: 0.5px;
  margin-top: 4px;
}

.stat-card .stat-trend {
  margin-top: 6px;
}

.stat-value-green { color: var(--color-trend-up); }
.stat-value-yellow { color: #f0ab00; }
.stat-value-red { color: var(--color-danger); }
.stat-value-blue { color: var(--color-primary); }

.overview-grid {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 24px;
}

.section-card {
  background: white;
  border-radius: 8px;
  box-shadow: var(--card-shadow);
  overflow: hidden;
}

.section-card .card-header {
  padding: 14px 16px;
  font-weight: 600;
  font-size: 15px;
  border-bottom: 1px solid #d2d2d2;
}

.section-card .card-body {
  padding: 16px;
}

.recovery-row {
  display: flex;
  justify-content: space-between;
  padding: 8px 0;
  font-size: 14px;
  border-bottom: 1px solid #f0f0f0;
}

.recovery-row:last-child {
  border-bottom: none;
}

.running-item {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 10px 0;
  font-size: 14px;
  border-bottom: 1px solid #f0f0f0;
}

.running-item:last-child {
  border-bottom: none;
}
```

- [ ] **Step 4: Implement Overview.tsx**

```tsx
// src/pages/Overview.tsx
import { Spinner, Alert, EmptyState, EmptyStateBody } from '@patternfly/react-core';
import { useApi } from '../api/hooks';
import { apiUrl } from '../api/client';
import { TrendIndicator } from '../components/TrendIndicator';
import { PhaseBadge } from '../components/PhaseBadge';
import type { OverviewStats } from '../types/api';
import './Overview.css';

function formatMs(ms: number): string {
  if (ms < 1000) return `${ms}ms`;
  return `${(ms / 1000).toFixed(1)}s`;
}

export function Overview() {
  const { data, loading, error } = useApi<OverviewStats>(apiUrl('/overview/stats'));

  if (loading) {
    return (
      <div style={{ display: 'flex', justifyContent: 'center', padding: 80 }}>
        <Spinner aria-label="Loading" />
      </div>
    );
  }

  if (error) {
    return (
      <div style={{ padding: 24 }}>
        <Alert variant="danger" title="Failed to load overview">{error}</Alert>
      </div>
    );
  }

  if (!data) {
    return (
      <EmptyState>
        <EmptyStateBody>No data available.</EmptyStateBody>
      </EmptyState>
    );
  }

  const cards = [
    { label: 'Total', value: data.total, color: '', trend: data.trends?.total, goodDir: 'up' as const },
    { label: 'Resilient', value: data.resilient, color: 'stat-value-green', trend: data.trends?.resilient, goodDir: 'up' as const },
    { label: 'Degraded', value: data.degraded, color: 'stat-value-yellow', trend: data.trends?.degraded, goodDir: 'down' as const },
    { label: 'Failed', value: data.failed, color: 'stat-value-red', trend: data.trends?.failed, goodDir: 'down' as const },
    { label: 'Running', value: data.running, color: 'stat-value-blue', trend: undefined, goodDir: 'up' as const },
  ];

  return (
    <>
      <div className="overview-header">
        <h1>Overview</h1>
      </div>
      <div className="overview-content">
        <div className="stat-cards">
          {cards.map((c) => (
            <div key={c.label} className="stat-card">
              <div className={`stat-value ${c.color}`}>{c.value}</div>
              <div className="stat-label">{c.label}</div>
              {c.trend !== undefined && (
                <div className="stat-trend">
                  <TrendIndicator value={c.trend} goodDirection={c.goodDir} />
                </div>
              )}
            </div>
          ))}
        </div>

        <div className="overview-grid">
          <div className="section-card">
            <div className="card-header">Avg Recovery Time by Type</div>
            <div className="card-body">
              {Object.keys(data.avgRecoveryByType).length === 0 ? (
                <div style={{ color: '#6a6e73', fontSize: 13 }}>No recovery data yet</div>
              ) : (
                Object.entries(data.avgRecoveryByType).map(([type, ms]) => (
                  <div key={type} className="recovery-row">
                    <span>{type}</span>
                    <span style={{ fontWeight: 600 }}>{formatMs(ms)}</span>
                  </div>
                ))
              )}
            </div>
          </div>

          <div className="section-card">
            <div className="card-header">Running Experiments ({data.runningExperiments.length})</div>
            <div className="card-body">
              {data.runningExperiments.length === 0 ? (
                <div style={{ color: '#6a6e73', fontSize: 13 }}>No experiments running</div>
              ) : (
                data.runningExperiments.map((exp) => (
                  <div key={`${exp.namespace}/${exp.name}`} className="running-item">
                    <div>
                      <div style={{ fontWeight: 500 }}>{exp.name}</div>
                      <div style={{ fontSize: 12, color: '#6a6e73' }}>{exp.component} / {exp.type}</div>
                    </div>
                    <PhaseBadge phase={exp.phase} />
                  </div>
                ))
              )}
            </div>
          </div>
        </div>
      </div>
    </>
  );
}
```

- [ ] **Step 5: Wire Overview into App.tsx**

Replace the `Placeholder` for the index route:

```tsx
// In App.tsx, add import:
import { Overview } from './pages/Overview';

// Replace: <Route index element={<Placeholder title="Overview" />} />
// With:    <Route index element={<Overview />} />
```

- [ ] **Step 6: Run test to verify it passes**

```bash
cd dashboard/ui && npx vitest run src/pages/Overview.test.tsx
```

Expected: PASS

- [ ] **Step 7: Commit**

```bash
git add dashboard/ui/src/pages/Overview.tsx dashboard/ui/src/pages/Overview.css dashboard/ui/src/App.tsx
git commit -m "feat(dashboard): add Overview page with stat cards and recovery times"
```

---

### Task 7: Experiments List Page

**Files:**
- Create: `dashboard/ui/src/pages/ExperimentsList.tsx`
- Create: `dashboard/ui/src/pages/ExperimentsList.css`
- Modify: `dashboard/ui/src/App.tsx` (wire in real page)
- Test: `dashboard/ui/src/pages/ExperimentsList.test.tsx`

**Context:** Filterable, sortable, paginated table of all experiments. Uses PF5 `Table` from `@patternfly/react-table`. Toolbar has dropdown filters for Operator, Type, Verdict, Phase, and a search input. Active filters shown as chips. Columns: Name, Operator, Component, Type, Phase, Verdict, Recovery, Date. Clicking a row navigates to detail. Fetches from `GET /api/v1/experiments` with query params.

Reference mockup: `.superpowers/brainstorm/80649-1774450280/experiments-list.html`

- [ ] **Step 1: Write the test**

```typescript
// src/pages/ExperimentsList.test.tsx
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor, fireEvent } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { ExperimentsList } from './ExperimentsList';

const mockList = {
  items: [
    {
      id: 'ns/exp1/2026-01-01T00:00:00Z',
      name: 'omc-podkill',
      namespace: 'opendatahub',
      operator: 'odh-model-controller',
      component: 'controller',
      type: 'PodKill',
      phase: 'Complete',
      verdict: 'Resilient',
      recoveryMs: 12000,
      startTime: '2026-03-25T10:00:00Z',
      specJson: '{}',
      statusJson: '{}',
      createdAt: '2026-03-25T10:00:00Z',
      updatedAt: '2026-03-25T10:05:00Z',
    },
    {
      id: 'ns/exp2/2026-01-01T00:00:00Z',
      name: 'omc-configdrift',
      namespace: 'opendatahub',
      operator: 'odh-model-controller',
      component: 'controller',
      type: 'ConfigDrift',
      phase: 'Complete',
      verdict: 'Failed',
      recoveryMs: 45000,
      startTime: '2026-03-25T11:00:00Z',
      specJson: '{}',
      statusJson: '{}',
      createdAt: '2026-03-25T11:00:00Z',
      updatedAt: '2026-03-25T11:05:00Z',
    },
  ],
  totalCount: 2,
};

describe('ExperimentsList', () => {
  beforeEach(() => {
    vi.restoreAllMocks();
  });

  it('renders experiments in a table', async () => {
    vi.spyOn(globalThis, 'fetch').mockResolvedValue({
      ok: true,
      json: () => Promise.resolve(mockList),
    } as Response);

    render(<MemoryRouter><ExperimentsList /></MemoryRouter>);

    await waitFor(() => {
      expect(screen.getByText('omc-podkill')).toBeInTheDocument();
      expect(screen.getByText('omc-configdrift')).toBeInTheDocument();
    });
  });

  it('shows verdict badges', async () => {
    vi.spyOn(globalThis, 'fetch').mockResolvedValue({
      ok: true,
      json: () => Promise.resolve(mockList),
    } as Response);

    render(<MemoryRouter><ExperimentsList /></MemoryRouter>);

    await waitFor(() => {
      expect(screen.getByText('Resilient')).toBeInTheDocument();
      expect(screen.getByText('Failed')).toBeInTheDocument();
    });
  });

  it('shows empty state when no experiments', async () => {
    vi.spyOn(globalThis, 'fetch').mockResolvedValue({
      ok: true,
      json: () => Promise.resolve({ items: [], totalCount: 0 }),
    } as Response);

    render(<MemoryRouter><ExperimentsList /></MemoryRouter>);

    await waitFor(() => {
      expect(screen.getByText(/no experiments found/i)).toBeInTheDocument();
    });
  });

  it('includes search param in fetch URL', async () => {
    const fetchSpy = vi.spyOn(globalThis, 'fetch').mockResolvedValue({
      ok: true,
      json: () => Promise.resolve(mockList),
    } as Response);

    render(<MemoryRouter><ExperimentsList /></MemoryRouter>);

    await waitFor(() => {
      expect(fetchSpy).toHaveBeenCalled();
    });

    const searchInput = screen.getByPlaceholderText(/search by name/i);
    fireEvent.change(searchInput, { target: { value: 'omc' } });

    await waitFor(() => {
      const lastCall = fetchSpy.mock.calls[fetchSpy.mock.calls.length - 1]?.[0];
      expect(String(lastCall)).toContain('search=omc');
    });
  });
});
```

- [ ] **Step 2: Run test to verify it fails**

```bash
cd dashboard/ui && npx vitest run src/pages/ExperimentsList.test.tsx
```

Expected: FAIL

- [ ] **Step 3: Implement ExperimentsList.css**

```css
/* src/pages/ExperimentsList.css */
.experiments-header {
  padding: 20px 24px;
  background: white;
  border-bottom: 1px solid #d2d2d2;
}

.experiments-header h1 {
  margin: 0;
  font-size: 22px;
}

.experiments-toolbar {
  padding: 12px 24px;
  background: white;
  border-bottom: 1px solid #d2d2d2;
  display: flex;
  align-items: center;
  gap: 12px;
  flex-wrap: wrap;
}

.toolbar-group {
  display: flex;
  align-items: center;
  gap: 8px;
}

.toolbar-label {
  font-size: 12px;
  color: #6a6e73;
  font-weight: 600;
}

.toolbar-select {
  padding: 6px 28px 6px 10px;
  border: 1px solid #d2d2d2;
  border-radius: 4px;
  font-size: 13px;
  background: white;
  cursor: pointer;
}

.toolbar-search {
  padding: 6px 10px;
  border: 1px solid #d2d2d2;
  border-radius: 4px;
  font-size: 13px;
  min-width: 200px;
}

.toolbar-divider {
  width: 1px;
  height: 24px;
  background: #d2d2d2;
  margin: 0 4px;
}

.experiments-table-wrapper {
  background: white;
  margin: 0;
}

.experiment-row {
  cursor: pointer;
}

.experiment-row:hover {
  background: #f8f8f8;
}

.experiments-pagination {
  padding: 12px 24px;
  background: white;
  border-top: 1px solid #d2d2d2;
  display: flex;
  justify-content: space-between;
  align-items: center;
  font-size: 13px;
  color: #6a6e73;
}

.pagination-controls {
  display: flex;
  gap: 8px;
  align-items: center;
}

.pagination-btn {
  padding: 4px 12px;
  border: 1px solid #d2d2d2;
  border-radius: 4px;
  background: white;
  cursor: pointer;
  font-size: 13px;
}

.pagination-btn:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.filter-chips {
  padding: 8px 24px;
  background: white;
  border-bottom: 1px solid #d2d2d2;
  display: flex;
  gap: 8px;
  flex-wrap: wrap;
  align-items: center;
}

.filter-chip {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  padding: 2px 8px;
  border-radius: 12px;
  background: #e7f1fa;
  color: #004080;
  font-size: 12px;
}

.filter-chip button {
  background: none;
  border: none;
  cursor: pointer;
  padding: 0 2px;
  font-size: 14px;
  color: #004080;
  line-height: 1;
}

.clear-all {
  font-size: 12px;
  color: var(--color-primary);
  cursor: pointer;
  background: none;
  border: none;
  padding: 0;
}
```

- [ ] **Step 4: Implement ExperimentsList.tsx**

```tsx
// src/pages/ExperimentsList.tsx
import { useState, useMemo } from 'react';
import { useNavigate } from 'react-router-dom';
import { Spinner, Alert, EmptyState, EmptyStateBody } from '@patternfly/react-core';
import { Table, Thead, Tr, Th, Tbody, Td } from '@patternfly/react-table';
import { useApi } from '../api/hooks';
import { apiUrl } from '../api/client';
import { VerdictBadge } from '../components/VerdictBadge';
import { PhaseBadge } from '../components/PhaseBadge';
import { INJECTION_TYPES, VERDICTS, PHASES } from '../types/api';
import type { ListResult } from '../types/api';
import './ExperimentsList.css';

function formatMs(ms: number | undefined): string {
  if (ms === undefined) return '—';
  if (ms < 1000) return `${ms}ms`;
  return `${(ms / 1000).toFixed(1)}s`;
}

function formatDate(iso: string | undefined): string {
  if (!iso) return '—';
  return new Date(iso).toLocaleString();
}

interface Filters {
  operator: string;
  type: string;
  verdict: string;
  phase: string;
  search: string;
}

const EMPTY_FILTERS: Filters = { operator: '', type: '', verdict: '', phase: '', search: '' };

export function ExperimentsList() {
  const navigate = useNavigate();
  const [filters, setFilters] = useState<Filters>(EMPTY_FILTERS);
  const [page, setPage] = useState(1);
  const pageSize = 10;

  const url = useMemo(() => apiUrl('/experiments', {
    ...filters,
    page,
    pageSize,
  }), [filters, page]);

  const { data, loading, error } = useApi<ListResult>(url);

  const activeFilters = Object.entries(filters).filter(([, v]) => v !== '');
  const totalPages = data ? Math.ceil(data.totalCount / pageSize) : 0;

  const setFilter = (key: keyof Filters, value: string) => {
    setFilters((prev) => ({ ...prev, [key]: value }));
    setPage(1);
  };

  const clearFilter = (key: keyof Filters) => {
    setFilters((prev) => ({ ...prev, [key]: '' }));
    setPage(1);
  };

  return (
    <>
      <div className="experiments-header">
        <h1>Experiments</h1>
      </div>

      <div className="experiments-toolbar">
        <div className="toolbar-group">
          <span className="toolbar-label">Type</span>
          <select
            className="toolbar-select"
            value={filters.type}
            onChange={(e) => setFilter('type', e.target.value)}
          >
            <option value="">All Types</option>
            {INJECTION_TYPES.map((t) => <option key={t} value={t}>{t}</option>)}
          </select>
        </div>

        <div className="toolbar-group">
          <span className="toolbar-label">Verdict</span>
          <select
            className="toolbar-select"
            value={filters.verdict}
            onChange={(e) => setFilter('verdict', e.target.value)}
          >
            <option value="">All Verdicts</option>
            {VERDICTS.map((v) => <option key={v} value={v}>{v}</option>)}
          </select>
        </div>

        <div className="toolbar-group">
          <span className="toolbar-label">Phase</span>
          <select
            className="toolbar-select"
            value={filters.phase}
            onChange={(e) => setFilter('phase', e.target.value)}
          >
            <option value="">All Phases</option>
            {PHASES.map((p) => <option key={p} value={p}>{p}</option>)}
          </select>
        </div>

        <div className="toolbar-divider" />

        <input
          className="toolbar-search"
          type="text"
          placeholder="Search by name..."
          value={filters.search}
          onChange={(e) => setFilter('search', e.target.value)}
        />
      </div>

      {activeFilters.length > 0 && (
        <div className="filter-chips">
          {activeFilters.map(([key, value]) => (
            <span key={key} className="filter-chip">
              {key}: {value}
              <button onClick={() => clearFilter(key as keyof Filters)}>&times;</button>
            </span>
          ))}
          <button className="clear-all" onClick={() => { setFilters(EMPTY_FILTERS); setPage(1); }}>
            Clear all
          </button>
        </div>
      )}

      {loading && (
        <div style={{ display: 'flex', justifyContent: 'center', padding: 80 }}>
          <Spinner aria-label="Loading" />
        </div>
      )}

      {error && (
        <div style={{ padding: 24 }}>
          <Alert variant="danger" title="Failed to load experiments">{error}</Alert>
        </div>
      )}

      {data && data.items.length === 0 && (
        <div style={{ padding: 24 }}>
          <EmptyState>
            <EmptyStateBody>No experiments found. Adjust filters or run your first experiment.</EmptyStateBody>
          </EmptyState>
        </div>
      )}

      {data && data.items.length > 0 && (
        <>
          <div className="experiments-table-wrapper">
            <Table aria-label="Experiments" variant="compact">
              <Thead>
                <Tr>
                  <Th>Name</Th>
                  <Th>Operator</Th>
                  <Th>Component</Th>
                  <Th>Type</Th>
                  <Th>Phase</Th>
                  <Th>Verdict</Th>
                  <Th>Recovery</Th>
                  <Th>Date</Th>
                </Tr>
              </Thead>
              <Tbody>
                {data.items.map((exp) => (
                  <Tr
                    key={exp.id}
                    className="experiment-row"
                    onRowClick={() => navigate(`/experiments/${exp.namespace}/${exp.name}`)}
                    isClickable
                  >
                    <Td dataLabel="Name">{exp.name}</Td>
                    <Td dataLabel="Operator">{exp.operator}</Td>
                    <Td dataLabel="Component">{exp.component}</Td>
                    <Td dataLabel="Type">{exp.type}</Td>
                    <Td dataLabel="Phase"><PhaseBadge phase={exp.phase} /></Td>
                    <Td dataLabel="Verdict"><VerdictBadge verdict={exp.verdict ?? ''} /></Td>
                    <Td dataLabel="Recovery">{formatMs(exp.recoveryMs)}</Td>
                    <Td dataLabel="Date">{formatDate(exp.startTime)}</Td>
                  </Tr>
                ))}
              </Tbody>
            </Table>
          </div>

          <div className="experiments-pagination">
            <span>
              Showing {(page - 1) * pageSize + 1}–{Math.min(page * pageSize, data.totalCount)} of {data.totalCount}
            </span>
            <div className="pagination-controls">
              <button className="pagination-btn" disabled={page <= 1} onClick={() => setPage(page - 1)}>
                Previous
              </button>
              <span>Page {page} of {totalPages}</span>
              <button className="pagination-btn" disabled={page >= totalPages} onClick={() => setPage(page + 1)}>
                Next
              </button>
            </div>
          </div>
        </>
      )}
    </>
  );
}
```

- [ ] **Step 5: Wire ExperimentsList into App.tsx**

```tsx
// In App.tsx, add import:
import { ExperimentsList } from './pages/ExperimentsList';

// Replace: <Route path="experiments" element={<Placeholder title="All Experiments" />} />
// With:    <Route path="experiments" element={<ExperimentsList />} />
```

- [ ] **Step 6: Run test to verify it passes**

```bash
cd dashboard/ui && npx vitest run src/pages/ExperimentsList.test.tsx
```

Expected: PASS

- [ ] **Step 7: Commit**

```bash
git add dashboard/ui/src/pages/ExperimentsList.tsx dashboard/ui/src/pages/ExperimentsList.css dashboard/ui/src/App.tsx
git commit -m "feat(dashboard): add Experiments List page with filters and pagination"
```

---

### Task 8: Experiment Detail Page

**Files:**
- Create: `dashboard/ui/src/pages/ExperimentDetail.tsx`
- Create: `dashboard/ui/src/pages/ExperimentDetail.css`
- Modify: `dashboard/ui/src/App.tsx` (wire in real page)
- Test: `dashboard/ui/src/pages/ExperimentDetail.test.tsx`

**Context:** Detail view for a single experiment at `/experiments/:namespace/:name`. Shows header with name, verdict badge, phase badge, danger level. Status message banner when `cleanupError` is present. 7 tabs: Summary, Evaluation, Steady State, Injection Log, Conditions, YAML, Debug. Fetches from `GET /api/v1/experiments/:namespace/:name`. The `specJson` and `statusJson` fields are raw JSON strings that must be parsed to render tab content.

Reference mockup: `.superpowers/brainstorm/80649-1774450280/experiment-detail.html`

**Important:** The experiment's `specJson` and `statusJson` contain the full CRD spec and status. The tab content renders data from these parsed JSON objects. The types for the inner structures are:

```typescript
// Parsed from specJson
interface ExperimentSpec {
  target: { operator: string; component: string; resource?: { kind: string; name: string } };
  steadyState?: { checks: { name: string; type: string; expected: string }[]; timeout?: string };
  injection: { type: string; parameters?: Record<string, string>; count?: number; ttl?: string; dangerLevel?: string };
  blastRadius?: { maxPodsAffected?: number; allowedNamespaces?: string[]; forbiddenResources?: string[]; allowDangerous?: boolean; dryRun?: boolean };
  hypothesis?: string;
}

// Parsed from statusJson
interface ExperimentStatus {
  phase: string;
  verdict?: string;
  message?: string;
  observedGeneration?: number;
  startTime?: string;
  endTime?: string;
  injectionStartedAt?: string;
  steadyStatePre?: { checks: { name: string; passed: boolean; value?: string; error?: string }[] };
  steadyStatePost?: { checks: { name: string; passed: boolean; value?: string; error?: string }[] };
  injectionLog?: { timestamp: string; action: string; target?: string; details?: string }[];
  evaluationResult?: { verdict: string; confidence?: number; recoveryTime?: string; reconcileCycles?: number; deviations?: string[] };
  cleanupError?: string;
  conditions?: { type: string; status: string; reason?: string; message?: string; lastTransitionTime?: string }[];
}
```

- [ ] **Step 1: Write the test**

```typescript
// src/pages/ExperimentDetail.test.tsx
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import { MemoryRouter, Route, Routes } from 'react-router-dom';
import { ExperimentDetail } from './ExperimentDetail';

const mockExp = {
  id: 'opendatahub/omc-podkill/2026-03-25T10:00:00Z',
  name: 'omc-podkill',
  namespace: 'opendatahub',
  operator: 'odh-model-controller',
  component: 'controller',
  type: 'PodKill',
  phase: 'Complete',
  verdict: 'Resilient',
  dangerLevel: 'medium',
  recoveryMs: 12000,
  startTime: '2026-03-25T10:00:00Z',
  endTime: '2026-03-25T10:05:00Z',
  cleanupError: '',
  specJson: JSON.stringify({
    target: { operator: 'odh-model-controller', component: 'controller' },
    injection: { type: 'PodKill', dangerLevel: 'medium' },
    hypothesis: 'Controller recovers within 60s',
  }),
  statusJson: JSON.stringify({
    phase: 'Complete',
    verdict: 'Resilient',
    message: 'Experiment completed successfully',
    evaluationResult: { verdict: 'Resilient', confidence: 95, recoveryTime: '12s', reconcileCycles: 3 },
    steadyStatePre: { checks: [{ name: 'pod-running', passed: true, value: '1/1' }] },
    steadyStatePost: { checks: [{ name: 'pod-running', passed: true, value: '1/1' }] },
    injectionLog: [{ timestamp: '2026-03-25T10:01:00Z', action: 'inject', target: 'pod/omc-abc', details: 'killed' }],
    conditions: [{ type: 'Ready', status: 'True', reason: 'Complete' }],
  }),
  createdAt: '2026-03-25T10:00:00Z',
  updatedAt: '2026-03-25T10:05:00Z',
};

function renderDetail() {
  return render(
    <MemoryRouter initialEntries={['/experiments/opendatahub/omc-podkill']}>
      <Routes>
        <Route path="/experiments/:namespace/:name" element={<ExperimentDetail />} />
      </Routes>
    </MemoryRouter>
  );
}

describe('ExperimentDetail', () => {
  beforeEach(() => {
    vi.restoreAllMocks();
    vi.spyOn(globalThis, 'fetch').mockResolvedValue({
      ok: true,
      json: () => Promise.resolve(mockExp),
    } as Response);
  });

  it('renders experiment name and badges', async () => {
    renderDetail();
    await waitFor(() => {
      expect(screen.getByText('omc-podkill')).toBeInTheDocument();
      expect(screen.getByText('Resilient')).toBeInTheDocument();
      expect(screen.getByText('Complete')).toBeInTheDocument();
    });
  });

  it('renders all 7 tab labels', async () => {
    renderDetail();
    await waitFor(() => {
      expect(screen.getByText('Summary')).toBeInTheDocument();
      expect(screen.getByText('Evaluation')).toBeInTheDocument();
      expect(screen.getByText('Steady State')).toBeInTheDocument();
      expect(screen.getByText('Injection Log')).toBeInTheDocument();
      expect(screen.getByText('Conditions')).toBeInTheDocument();
      expect(screen.getByText('YAML')).toBeInTheDocument();
      expect(screen.getByText('Debug')).toBeInTheDocument();
    });
  });

  it('shows Summary tab content by default', async () => {
    renderDetail();
    await waitFor(() => {
      expect(screen.getByText('odh-model-controller')).toBeInTheDocument();
      expect(screen.getByText('PodKill')).toBeInTheDocument();
    });
  });

  it('shows cleanup error banner when present', async () => {
    const expWithError = { ...mockExp, cleanupError: 'failed to remove finalizer' };
    vi.spyOn(globalThis, 'fetch').mockResolvedValue({
      ok: true,
      json: () => Promise.resolve(expWithError),
    } as Response);

    renderDetail();
    await waitFor(() => {
      expect(screen.getByText(/failed to remove finalizer/)).toBeInTheDocument();
    });
  });

  it('shows 404 when experiment not found', async () => {
    vi.spyOn(globalThis, 'fetch').mockResolvedValue({
      ok: false,
      status: 404,
      json: () => Promise.resolve({ error: 'experiment not found' }),
    } as unknown as Response);

    renderDetail();
    await waitFor(() => {
      expect(screen.getByText(/experiment not found/)).toBeInTheDocument();
    });
  });
});
```

- [ ] **Step 2: Run test to verify it fails**

```bash
cd dashboard/ui && npx vitest run src/pages/ExperimentDetail.test.tsx
```

Expected: FAIL

- [ ] **Step 3: Implement ExperimentDetail.css**

```css
/* src/pages/ExperimentDetail.css */
.detail-breadcrumb {
  padding: 12px 24px;
  font-size: 13px;
  color: #6a6e73;
  background: white;
  border-bottom: 1px solid #d2d2d2;
}

.detail-breadcrumb a {
  color: var(--color-primary);
  text-decoration: none;
}

.detail-header {
  padding: 20px 24px;
  background: white;
  border-bottom: 1px solid #d2d2d2;
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
}

.detail-header h1 {
  margin: 0 0 6px;
  font-size: 22px;
}

.detail-header .meta {
  display: flex;
  gap: 12px;
  align-items: center;
  flex-wrap: wrap;
}

.detail-header .actions {
  display: flex;
  gap: 8px;
}

.detail-tabs {
  background: white;
}

.tab-content {
  padding: 24px;
}

.kv-table {
  width: 100%;
  border-collapse: collapse;
}

.kv-table td {
  padding: 8px 12px;
  border-bottom: 1px solid #f0f0f0;
  font-size: 14px;
  vertical-align: top;
}

.kv-table td:first-child {
  font-weight: 600;
  color: #6a6e73;
  width: 200px;
  white-space: nowrap;
}

.check-row {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 8px 0;
  border-bottom: 1px solid #f0f0f0;
}

.check-pass { color: var(--color-trend-up); }
.check-fail { color: var(--color-danger); }

.log-entry {
  padding: 8px 0;
  border-bottom: 1px solid #f0f0f0;
  font-size: 13px;
}

.log-entry .log-time {
  color: #6a6e73;
  font-family: monospace;
  margin-right: 12px;
}

.yaml-block {
  background: #1e1e1e;
  color: #d4d4d4;
  padding: 16px;
  border-radius: 4px;
  font-family: monospace;
  font-size: 13px;
  overflow-x: auto;
  white-space: pre-wrap;
  word-break: break-all;
  max-height: 600px;
  overflow-y: auto;
}

.yaml-actions {
  display: flex;
  gap: 8px;
  margin-bottom: 12px;
}

.detail-danger-badge {
  padding: 2px 8px;
  border-radius: 4px;
  font-size: 11px;
  font-weight: 600;
  text-transform: uppercase;
}

.danger-low { background: var(--color-resilient-bg); color: var(--color-resilient-fg); }
.danger-medium { background: var(--color-degraded-bg); color: var(--color-degraded-fg); }
.danger-high { background: var(--color-failed-bg); color: var(--color-failed-fg); }
```

- [ ] **Step 4: Implement ExperimentDetail.tsx**

```tsx
// src/pages/ExperimentDetail.tsx
import { useState, useMemo } from 'react';
import { useParams, Link } from 'react-router-dom';
import { Spinner, Alert, Tabs, Tab, TabTitleText, Button, EmptyState, EmptyStateBody } from '@patternfly/react-core';
import { Table, Thead, Tr, Th, Tbody, Td } from '@patternfly/react-table';
import { useApi } from '../api/hooks';
import { apiUrl } from '../api/client';
import { VerdictBadge } from '../components/VerdictBadge';
import { PhaseBadge } from '../components/PhaseBadge';
import { StatusBanner } from '../components/StatusBanner';
import type { Experiment } from '../types/api';
import './ExperimentDetail.css';

function formatMs(ms: number | undefined): string {
  if (ms === undefined) return '—';
  if (ms < 1000) return `${ms}ms`;
  return `${(ms / 1000).toFixed(1)}s`;
}

function formatDate(iso: string | undefined): string {
  if (!iso) return '—';
  return new Date(iso).toLocaleString();
}

export function ExperimentDetail() {
  const { namespace, name } = useParams<{ namespace: string; name: string }>();
  const [activeTab, setActiveTab] = useState(0);

  const { data: exp, loading, error } = useApi<Experiment>(
    apiUrl(`/experiments/${namespace}/${name}`)
  );

  const spec = useMemo(() => {
    if (!exp) return null;
    try { return JSON.parse(exp.specJson); } catch { return null; }
  }, [exp]);

  const status = useMemo(() => {
    if (!exp) return null;
    try { return JSON.parse(exp.statusJson); } catch { return null; }
  }, [exp]);

  if (loading) {
    return (
      <div style={{ display: 'flex', justifyContent: 'center', padding: 80 }}>
        <Spinner aria-label="Loading" />
      </div>
    );
  }

  if (error) {
    return (
      <div style={{ padding: 24 }}>
        <Alert variant="danger" title="Failed to load experiment">{error}</Alert>
      </div>
    );
  }

  if (!exp) {
    return (
      <EmptyState>
        <EmptyStateBody>Experiment not found.</EmptyStateBody>
      </EmptyState>
    );
  }

  const evaluation = status?.evaluationResult;

  return (
    <>
      <div className="detail-breadcrumb">
        <Link to="/experiments">Experiments</Link> / {namespace} / {name}
      </div>

      {exp.cleanupError && (
        <StatusBanner variant="error" message={`Cleanup error: ${exp.cleanupError}`} />
      )}
      {status?.message && !exp.cleanupError && (
        <StatusBanner variant="info" message={status.message} />
      )}

      <div className="detail-header">
        <div>
          <h1>{exp.name}</h1>
          <div className="meta">
            <VerdictBadge verdict={exp.verdict ?? ''} />
            <PhaseBadge phase={exp.phase} />
            {exp.dangerLevel && (
              <span className={`detail-danger-badge danger-${exp.dangerLevel}`}>
                {exp.dangerLevel}
              </span>
            )}
            <span style={{ color: '#6a6e73', fontSize: 13 }}>{exp.namespace}</span>
          </div>
        </div>
        <div className="actions">
          <Button
            variant="secondary"
            onClick={() => {
              const blob = new Blob([JSON.stringify({ spec: spec, status: status }, null, 2)], { type: 'application/json' });
              const url = URL.createObjectURL(blob);
              const a = document.createElement('a');
              a.href = url;
              a.download = `${exp.name}.json`;
              a.click();
              URL.revokeObjectURL(url);
            }}
          >
            Export YAML
          </Button>
        </div>
      </div>

      <div className="detail-tabs">
        <Tabs activeKey={activeTab} onSelect={(_, key) => setActiveTab(key as number)}>
          <Tab eventKey={0} title={<TabTitleText>Summary</TabTitleText>}>
            <div className="tab-content">
              <table className="kv-table">
                <tbody>
                  <tr><td>Operator</td><td>{exp.operator}</td></tr>
                  <tr><td>Component</td><td>{exp.component}</td></tr>
                  <tr><td>Injection Type</td><td>{exp.type}</td></tr>
                  <tr><td>Danger Level</td><td>{exp.dangerLevel ?? '—'}</td></tr>
                  <tr><td>Recovery Time</td><td>{formatMs(exp.recoveryMs)}</td></tr>
                  <tr><td>Start Time</td><td>{formatDate(exp.startTime)}</td></tr>
                  <tr><td>End Time</td><td>{formatDate(exp.endTime)}</td></tr>
                  {spec?.hypothesis && <tr><td>Hypothesis</td><td>{spec.hypothesis}</td></tr>}
                  {spec?.blastRadius && (
                    <>
                      {spec.blastRadius.maxPodsAffected !== undefined && (
                        <tr><td>Max Pods Affected</td><td>{spec.blastRadius.maxPodsAffected}</td></tr>
                      )}
                      {spec.blastRadius.dryRun !== undefined && (
                        <tr><td>Dry Run</td><td>{spec.blastRadius.dryRun ? 'Yes' : 'No'}</td></tr>
                      )}
                    </>
                  )}
                </tbody>
              </table>
            </div>
          </Tab>

          <Tab eventKey={1} title={<TabTitleText>Evaluation</TabTitleText>}>
            <div className="tab-content">
              {evaluation ? (
                <table className="kv-table">
                  <tbody>
                    <tr><td>Verdict</td><td><VerdictBadge verdict={evaluation.verdict} /></td></tr>
                    {evaluation.confidence !== undefined && <tr><td>Confidence</td><td>{evaluation.confidence}%</td></tr>}
                    {evaluation.recoveryTime && <tr><td>Recovery Time</td><td>{evaluation.recoveryTime}</td></tr>}
                    {evaluation.reconcileCycles !== undefined && <tr><td>Reconcile Cycles</td><td>{evaluation.reconcileCycles}</td></tr>}
                    {evaluation.deviations && evaluation.deviations.length > 0 && (
                      <tr><td>Deviations</td><td><ul>{evaluation.deviations.map((d: string, i: number) => <li key={i}>{d}</li>)}</ul></td></tr>
                    )}
                  </tbody>
                </table>
              ) : (
                <div style={{ color: '#6a6e73' }}>No evaluation data available.</div>
              )}
            </div>
          </Tab>

          <Tab eventKey={2} title={<TabTitleText>Steady State</TabTitleText>}>
            <div className="tab-content">
              {['Pre-check', 'Post-check'].map((label, idx) => {
                const checks = idx === 0 ? status?.steadyStatePre?.checks : status?.steadyStatePost?.checks;
                return (
                  <div key={label} style={{ marginBottom: 24 }}>
                    <h3>{label}</h3>
                    {checks && checks.length > 0 ? (
                      checks.map((c: { name: string; passed: boolean; value?: string; error?: string }, i: number) => (
                        <div key={i} className="check-row">
                          <span className={c.passed ? 'check-pass' : 'check-fail'}>
                            {c.passed ? '✓' : '✗'}
                          </span>
                          <span style={{ fontWeight: 500 }}>{c.name}</span>
                          {c.value && <span style={{ color: '#6a6e73' }}>({c.value})</span>}
                          {c.error && <span className="check-fail">{c.error}</span>}
                        </div>
                      ))
                    ) : (
                      <div style={{ color: '#6a6e73' }}>No checks recorded.</div>
                    )}
                  </div>
                );
              })}
            </div>
          </Tab>

          <Tab eventKey={3} title={<TabTitleText>Injection Log</TabTitleText>}>
            <div className="tab-content">
              {status?.injectionLog && status.injectionLog.length > 0 ? (
                status.injectionLog.map((entry: { timestamp: string; action: string; target?: string; details?: string }, i: number) => (
                  <div key={i} className="log-entry">
                    <span className="log-time">{new Date(entry.timestamp).toLocaleTimeString()}</span>
                    <strong>{entry.action}</strong>
                    {entry.target && <span> → {entry.target}</span>}
                    {entry.details && <span style={{ color: '#6a6e73' }}> ({entry.details})</span>}
                  </div>
                ))
              ) : (
                <div style={{ color: '#6a6e73' }}>No injection events recorded.</div>
              )}
            </div>
          </Tab>

          <Tab eventKey={4} title={<TabTitleText>Conditions</TabTitleText>}>
            <div className="tab-content">
              {status?.conditions && status.conditions.length > 0 ? (
                <Table aria-label="Conditions" variant="compact">
                  <Thead>
                    <Tr>
                      <Th>Type</Th>
                      <Th>Status</Th>
                      <Th>Reason</Th>
                      <Th>Message</Th>
                      <Th>Last Transition</Th>
                    </Tr>
                  </Thead>
                  <Tbody>
                    {status.conditions.map((c: { type: string; status: string; reason?: string; message?: string; lastTransitionTime?: string }, i: number) => (
                      <Tr key={i}>
                        <Td>{c.type}</Td>
                        <Td>{c.status}</Td>
                        <Td>{c.reason ?? '—'}</Td>
                        <Td>{c.message ?? '—'}</Td>
                        <Td>{formatDate(c.lastTransitionTime)}</Td>
                      </Tr>
                    ))}
                  </Tbody>
                </Table>
              ) : (
                <div style={{ color: '#6a6e73' }}>No conditions.</div>
              )}
            </div>
          </Tab>

          <Tab eventKey={5} title={<TabTitleText>YAML</TabTitleText>}>
            <div className="tab-content">
              <div className="yaml-actions">
                <Button
                  variant="secondary"
                  size="sm"
                  onClick={() => navigator.clipboard.writeText(JSON.stringify({ spec, status }, null, 2))}
                >
                  Copy
                </Button>
              </div>
              <pre className="yaml-block">
                {JSON.stringify({ spec, status }, null, 2)}
              </pre>
            </div>
          </Tab>

          <Tab eventKey={6} title={<TabTitleText>Debug</TabTitleText>}>
            <div className="tab-content">
              <table className="kv-table">
                <tbody>
                  <tr><td>Observed Generation</td><td>{status?.observedGeneration ?? '—'}</td></tr>
                  <tr><td>Cleanup Error</td><td>{exp.cleanupError || '—'}</td></tr>
                  <tr><td>Created At</td><td>{formatDate(exp.createdAt)}</td></tr>
                  <tr><td>Updated At</td><td>{formatDate(exp.updatedAt)}</td></tr>
                </tbody>
              </table>
              <h3 style={{ marginTop: 24 }}>Raw Status JSON</h3>
              <details>
                <summary>Expand</summary>
                <pre className="yaml-block">{JSON.stringify(status, null, 2)}</pre>
              </details>
            </div>
          </Tab>
        </Tabs>
      </div>
    </>
  );
}
```

- [ ] **Step 5: Wire ExperimentDetail into App.tsx**

```tsx
// In App.tsx, add import:
import { ExperimentDetail } from './pages/ExperimentDetail';

// Replace: <Route path="experiments/:namespace/:name" element={<Placeholder title="Experiment Detail" />} />
// With:    <Route path="experiments/:namespace/:name" element={<ExperimentDetail />} />
```

- [ ] **Step 6: Run test to verify it passes**

```bash
cd dashboard/ui && npx vitest run src/pages/ExperimentDetail.test.tsx
```

Expected: PASS

- [ ] **Step 7: Commit**

```bash
git add dashboard/ui/src/pages/ExperimentDetail.tsx dashboard/ui/src/pages/ExperimentDetail.css dashboard/ui/src/App.tsx
git commit -m "feat(dashboard): add Experiment Detail page with 7 tabs"
```

---

### Task 9: Go Embed Integration

**Files:**
- Create: `dashboard/embed.go`
- Modify: `dashboard/cmd/dashboard/main.go`

**Context:** The Go binary embeds the built frontend assets and serves them at `/`. The `embed.go` file uses `//go:embed` to include `ui-dist/` contents. The server falls back to `index.html` for client-side routing (SPA). This must be tested by building the frontend first, then the Go binary.

**Important:** The `ui-dist/` directory must exist before `go build`. The CI/build process should run `cd dashboard/ui && npm run build` first.

- [ ] **Step 1: Create embed.go**

```go
// dashboard/embed.go
package dashboard

import "embed"

//go:embed ui-dist
var UIAssets embed.FS
```

- [ ] **Step 2: Add static file serving to main.go**

In `dashboard/cmd/dashboard/main.go`, add the embedded UI serving after the API routes. The server should serve files from the embedded FS and fall back to `index.html` for SPA routing.

Add import for the dashboard package and `io/fs`:

```go
import (
    // ... existing imports ...
    "io/fs"

    dashboard "github.com/opendatahub-io/odh-platform-chaos/dashboard"
)
```

After `srv := api.NewServer(...)` and before starting the HTTP server, create a file server handler and wrap `srv.Handler()`:

```go
// Serve embedded UI assets
uiFS, err := fs.Sub(dashboard.UIAssets, "ui-dist")
if err != nil {
    log.Fatalf("embedded ui assets: %v", err)
}
fileServer := http.FileServer(http.FS(uiFS))

// Combine API + static file serving
mux := http.NewServeMux()
mux.Handle("/api/", srv.Handler())
mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
    // Try to serve the file; fall back to index.html for SPA routing
    path := r.URL.Path
    if path == "/" {
        path = "/index.html"
    }
    if _, err := fs.Stat(uiFS, path[1:]); err != nil {
        // File not found — serve index.html for client-side routing
        r.URL.Path = "/"
        fileServer.ServeHTTP(w, r)
        return
    }
    fileServer.ServeHTTP(w, r)
})
```

Update `httpServer.Handler` to use `mux` instead of `srv.Handler()`.

- [ ] **Step 3: Build frontend then Go binary**

```bash
cd dashboard/ui && npm run build
cd ../.. && go build ./dashboard/cmd/dashboard/
```

Expected: Both build successfully. The Go binary includes the embedded UI.

- [ ] **Step 4: Verify with a manual smoke test**

```bash
# Start the dashboard (will fail connecting to K8s, but UI should serve)
./dashboard --db=:memory: 2>&1 &
sleep 1
curl -s http://localhost:8080/ | head -5
# Should show the HTML page (index.html)
curl -s http://localhost:8080/experiments | head -5
# Should show the HTML page (SPA fallback)
kill %1
```

- [ ] **Step 5: Commit**

```bash
git add dashboard/embed.go dashboard/cmd/dashboard/main.go
git commit -m "feat(dashboard): embed and serve frontend assets from Go binary"
```

---

### Task 10: Run All Tests and Final Verification

**Files:** None (verification only)

**Context:** Final task to ensure everything works together. Run all frontend tests and backend tests. Verify the build pipeline.

- [ ] **Step 1: Run all frontend tests**

```bash
cd dashboard/ui && npm test
```

Expected: All tests pass (types, hooks, App, components, Overview, ExperimentsList, ExperimentDetail)

- [ ] **Step 2: Run all backend tests**

```bash
cd /path/to/odh-platform-chaos && go test ./dashboard/...
```

Expected: All tests pass (api, convert, store, watcher)

- [ ] **Step 3: Full build pipeline**

```bash
cd dashboard/ui && npm run build && cd ../.. && go build ./dashboard/cmd/dashboard/
```

Expected: Clean build

- [ ] **Step 4: Commit any remaining changes**

```bash
git status
# If any unstaged changes, commit them
```

---

## Phase 2 (Deferred)

The following views are planned for a separate Phase 2 plan:

1. **Live Monitoring** (`/live`) — SSE connection, real-time experiment cards, phase stepper with animations
2. **Suites** (`/suites`) — Suite run cards, expandable experiment tables, version comparison
3. **Operators** (`/operators`) — Operator health cards, component accordions, injection coverage matrix
4. **Knowledge** (`/knowledge`) — Interactive SVG dependency graph, zoom/pan, coverage overlays
