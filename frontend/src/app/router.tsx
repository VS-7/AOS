/**
 * Task 10: the AOS route tree (`@app/router.tsx`), ported. The only
 * changes from the pristine original are the two Igniter-specific imports
 * just below (`IgniterRouter` → `AosRouter`, the `igniter` instance →
 * `aos`) — `AosRouter.create(app).addRoute(route).build(options)` mirrors
 * `IgniterRouter`'s own shape exactly, so nothing else in this file
 * changed. Every `*Page` import below is a pre-built TanStack `Route`
 * (the result of a feature file's own `aos.page(...).build()`), already
 * parented to `aos.rootRoute` at construction — see `app/aos.tsx`'s doc
 * comment on `.withLayout(...)` for why that matters.
 *
 * Fourteen of the ported domains (`DORMANT_DOMAINS` in `lib/command-map.
 * ts`) have no Go backend yet. The nine that have a route reachable
 * through this tree — `collection`, `view`, `goal`, `project`,
 * `marketplace` as their own top-level pages, and `tunnel`/`user`/
 * `instruction`/`template` as sections dispatched from `/settings/$group/
 * $section` — render a `<DormantGate feature="…">` panel instead of a
 * blank screen, crash, or infinite spinner. That wrapping happens inside
 * each page file itself (`withComponent(...)`, and for the four
 * dormant loaders, an early return before the loader ever calls a
 * command), not here: `.addRoute(...)` below takes an already-built
 * `Route` object, so there is no JSX to wrap at this call site. The
 * remaining five dormant domains (`artifact`, `model`, `skill`, `token`,
 * `toolset`) have no page of their own at all — they surface only as
 * embedded widgets inside otherwise-live pages (e.g. a model picker
 * inside Settings → AI Providers), so there is no route here to gate.
 */
import { AosRouter } from "@/app/builders";
import { aos } from "@/app/aos";
import { HomePage } from "@/features/workspace/presentation/pages/home";
import { OnboardingPage } from "@/features/workspace/presentation/pages/onboarding";
import { TasksPage } from "@/features/task/presentation/pages/(main)";
import { TaskDetailsPage } from "@/features/task/presentation/pages/($id)";
import { ChatPage } from "@/features/chat/presentation/pages/($id)";
import { CollectionPage } from "@/features/collection/presentation/pages/($id)";
import { CollectionRecordUpsertPage } from "@/features/collection/presentation/pages/($id)/records/($record)";
import { RouterProvider, type ErrorComponentProps } from "@tanstack/react-router";
import { ViewPage } from "@/features/view/presentation/pages/($view)";
import { LoginPage } from "@/features/auth/presentation/pages/login";
import { ActivitiesPage } from "@/features/activity/presentation/pages/(main)";
import { RoutinesPage } from "@/features/routine/presentation/pages/(main)";
import { RoutineUpsertPage } from "@/features/routine/presentation/pages/($id)";
import { GoalsPage } from "@/features/goal/presentation/pages/(main)";
import { GoalDetailsPage } from "@/features/goal/presentation/pages/($id)";
import { ProjectsPage } from "@/features/project/presentation/pages/(main)";
import { ProjectDetailsPage } from "@/features/project/presentation/pages/($id)";
import { MarketplacePage } from "@/features/marketplace/presentation/pages/marketplace";
import { MarketplaceDetailsPage } from "@/features/marketplace/presentation/pages/marketplace/[name]";
import { SettingsIndexPage } from "@/features/workspace/presentation/pages/settings";
import { SettingsSectionPage } from "@/features/workspace/presentation/pages/settings/($group)/($section)";
import { t } from "@/lib/i18n";

// ─── Route-level error fallback ───────────────────────────────────────────────
// TanStack Router renders this inside its own error boundary for loader errors.
// Uses the same visual language as ErrorBoundary for consistency.
// NOTE: TanStack Router can pass non-Error values, so message extraction is safe.

function RouteErrorFallback({ error, reset }: ErrorComponentProps) {
  // Safe extraction: TanStack Router can pass strings, objects, or Error instances.
  const isErrorInstance = error instanceof Error;
  const errorName = isErrorInstance ? error.name : "Error";
  const errorMessage = isErrorInstance
    ? error.message
    : typeof error === "string"
    ? error
    : "An unexpected error occurred in this section.";

  const displayMessage = errorMessage.length > 100
    ? `${errorMessage.slice(0, 100)}…`
    : errorMessage;

  return (
    <div className="eb-root eb-root--section" role="alert">
      <style>{ROUTE_FALLBACK_STYLES}</style>
      <div className="eb-card">
        {/* Grayscale Icon Illustration */}
        <div className="eb-logo" aria-hidden="true">
          <svg width="88" height="88" viewBox="0 0 88 88" fill="none" xmlns="http://www.w3.org/2000/svg">
            <rect x="12" y="12" width="64" height="64" rx="16" stroke="currentColor" strokeWidth="1" strokeDasharray="3 3" opacity="0.15" />
            <rect x="22" y="22" width="44" height="44" rx="10" stroke="currentColor" strokeWidth="1" opacity="0.1" />
            
            {/* Diagonal crosshairs */}
            <path d="M22 22l44 44M66 22L22 66" stroke="currentColor" strokeWidth="1" opacity="0.08" />
            
            {/* Target element showing fault */}
            <circle cx="44" cy="44" r="14" stroke="currentColor" strokeWidth="1.5" strokeDasharray="4 2" opacity="0.4" />
            <circle cx="44" cy="44" r="6" fill="currentColor" opacity="0.15" />
            <circle cx="44" cy="44" r="2" fill="currentColor" opacity="0.7" />
            
            {/* Fault accent mark */}
            <path d="M58 24l6 6M64 24l-6 6" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" opacity="0.6" />
            <line x1="53" y1="35" x2="44" y2="44" stroke="currentColor" strokeWidth="1" strokeDasharray="2 2" opacity="0.3" />
          </svg>
        </div>

        {/* Text */}
        <div className="eb-text">
          <h2 className="eb-title" style={{ fontSize: "1rem" }}>{t("Something went wrong")}</h2>
          <p className="eb-desc">{t("This section encountered an unexpected error and has been isolated.")}</p>
        </div>

        {/* Error pill */}
        <div className="eb-error-pill" title={errorMessage}>
          <span className="eb-error-name">{errorName}</span>
          <span className="eb-error-sep" aria-hidden="true">·</span>
          <span className="eb-error-msg">{displayMessage}</span>
        </div>

        {/* Action */}
        <div className="eb-actions" style={{ maxWidth: "220px" }}>
          <button id="route-error-retry" className="eb-btn eb-btn--primary" onClick={reset}>
            <svg width="13" height="13" viewBox="0 0 14 14" fill="none" aria-hidden="true">
              <path d="M1 7a6 6 0 1 0 .75-2.91M1 1v3.5h3.5" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round" />
            </svg>
            {t("Try again")}
          </button>
        </div>
      </div>
    </div>
  );
}

const ROUTE_FALLBACK_STYLES = `
  @keyframes eb-fade-up {
    from { opacity: 0; transform: translateY(8px); }
    to   { opacity: 1; transform: translateY(0); }
  }
  .eb-root--section {
    position: relative;
    display: flex;
    align-items: center;
    justify-content: center;
    flex: 1;
    min-height: 100%;
    height: 100%;
    width: 100%;
    padding: 24px;
    box-sizing: border-box;
    background: transparent;
    overflow: hidden;
    font-family: "Geist Variable", system-ui, sans-serif;
    color: var(--foreground, hsl(210 40% 98%));
  }
  .eb-card {
    position: relative;
    z-index: 1;
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 16px;
    width: 100%;
    max-width: 360px;
    text-align: center;
    animation: eb-fade-up 0.3s cubic-bezier(0.22, 1, 0.36, 1) both;
  }
  .eb-logo {
    color: var(--foreground, hsl(210 40% 98%));
    opacity: 0.55;
    line-height: 0;
  }
  .eb-text { display: flex; flex-direction: column; gap: 4px; }
  .eb-title { margin: 0; font-weight: 600; letter-spacing: -0.02em; color: var(--foreground, hsl(210 40% 98%)); line-height: 1.3; }
  .eb-desc { margin: 0; font-size: 0.8125rem; line-height: 1.5; color: var(--muted-foreground, hsl(215 16% 47%)); }
  .eb-error-pill {
    display: inline-flex;
    align-items: baseline;
    gap: 6px;
    padding: 5px 10px;
    border-radius: 7px;
    background: var(--muted, hsl(215 20% 12%));
    border: 1px solid var(--border, hsl(215 20% 16%));
    max-width: 100%;
    overflow: hidden;
  }
  .eb-error-name {
    font-size: 0.6875rem;
    font-weight: 600;
    color: var(--destructive, hsl(0 72% 58%));
    white-space: nowrap;
    flex-shrink: 0;
    font-family: ui-monospace, monospace;
  }
  .eb-error-sep { color: var(--border, hsl(215 20% 28%)); font-size: 0.75rem; flex-shrink: 0; }
  .eb-error-msg {
    font-size: 0.6875rem;
    font-family: ui-monospace, monospace;
    color: var(--muted-foreground, hsl(215 16% 47%));
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
    min-width: 0;
  }
  .eb-actions { display: flex; flex-direction: column; align-items: center; gap: 8px; width: 100%; }
  .eb-btn {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    gap: 6px;
    height: 34px;
    padding: 0 14px;
    border-radius: 8px;
    font-size: 0.8125rem;
    font-weight: 500;
    font-family: "Geist Variable", system-ui, sans-serif;
    cursor: pointer;
    border: none;
    transition: opacity 0.15s ease, transform 0.1s ease;
    width: 100%;
  }
  .eb-btn:active { transform: scale(0.97); }
  .eb-btn--primary {
    background: var(--foreground, hsl(210 40% 98%));
    color: var(--background, hsl(220 13% 7%));
  }
  .eb-btn--primary:hover { opacity: 0.88; }
`;

// ─── Router ───────────────────────────────────────────────────────────────────

export const router = AosRouter.create(aos)
  .addRoute(HomePage)
  .addRoute(CollectionPage)
  .addRoute(CollectionRecordUpsertPage)
  .addRoute(ViewPage)
  .addRoute(TasksPage)
  .addRoute(TaskDetailsPage)
  .addRoute(OnboardingPage)
  .addRoute(ChatPage)
  .addRoute(LoginPage)
  .addRoute(ActivitiesPage)
  .addRoute(RoutinesPage)
  .addRoute(RoutineUpsertPage)
  .addRoute(GoalsPage)
  .addRoute(GoalDetailsPage)
  .addRoute(ProjectsPage)
  .addRoute(ProjectDetailsPage)
  .addRoute(MarketplacePage)
  .addRoute(MarketplaceDetailsPage)
  .addRoute(SettingsIndexPage)
  .addRoute(SettingsSectionPage)
  .build({ defaultErrorComponent: RouteErrorFallback });

declare module "@tanstack/react-router" {
  interface Register {
    router: typeof router;
  }
}

export function AppRouter() {
  return <RouterProvider router={router} />;
}

