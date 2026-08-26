import { Component, type ErrorInfo, type ReactNode } from "react";
import { openExternal } from "@/lib/wails";

// ─── Types ──────────────────────────────────────────────────────────────────

interface ErrorBoundaryProps {
  /** Subtree to protect. */
  children: ReactNode;
  /**
   * Optional custom fallback rendered instead of the default crash screen.
   * Receives the error and a reset callback.
   */
  fallback?: (error: Error, reset: () => void) => ReactNode;
  /**
   * Called whenever a render error is caught. Use for logging integrations.
   * The default implementation logs to console.error (there is no native
   * crash-report bridge exposed from the Go backend yet).
   */
  onError?: (error: Error, info: ErrorInfo) => void;
  /** Section label shown in the crash screen subtitle (e.g. "Tasks", "Chat"). */
  section?: string;
}

interface ErrorBoundaryState {
  hasError: boolean;
  error: Error | null;
  errorInfo: ErrorInfo | null;
  copied: boolean;
}

// ─── Component ───────────────────────────────────────────────────────────────

/**
 * ErrorBoundary — React class-based error boundary.
 *
 * Catches uncaught render errors inside its subtree and renders a branded
 * crash-recovery screen instead of going blank (the Electron white-screen crash).
 *
 * Usage:
 * ```tsx
 * // Global (in app.tsx)
 * <ErrorBoundary>
 *   <App />
 * </ErrorBoundary>
 *
 * // Feature-level (in router.tsx)
 * <ErrorBoundary section="Tasks">
 *   <TasksPage />
 * </ErrorBoundary>
 * ```
 */
export class ErrorBoundary extends Component<
  ErrorBoundaryProps,
  ErrorBoundaryState
> {
  constructor(props: ErrorBoundaryProps) {
    super(props);
    this.state = {
      hasError: false,
      error: null,
      errorInfo: null,
      copied: false,
    };
    this.reset = this.reset.bind(this);
    this.copyError = this.copyError.bind(this);
  }

  static getDerivedStateFromError(error: Error): Partial<ErrorBoundaryState> {
    return { hasError: true, error };
  }

  componentDidCatch(error: Error, info: ErrorInfo): void {
    this.setState({ errorInfo: info });

    // No native crash-report bridge exists yet (nothing like
    // window.go.SystemService.ReportCrash on the Go side), so we just fall
    // through to the console.error below.

    // Custom onError hook (allows callers to plug in a server-side logger).
    if (this.props.onError) {
      try {
        this.props.onError(error, info);
      } catch {
        // Swallow errors from the error handler.
      }
    }

    // Fallback: emit to console so DevTools / native logs still capture it.
    // We intentionally use console.error here because the AOS logger may
    // itself be part of the crashed subtree.
    console.error("[ErrorBoundary] Uncaught render error", {
      section: this.props.section ?? "global",
      error,
      componentStack: info.componentStack,
    });
  }

  reset(): void {
    if (typeof window !== "undefined") {
      window.location.reload();
    } else {
      this.setState({ hasError: false, error: null, errorInfo: null, copied: false });
    }
  }

  copyError(): void {
    const { error, errorInfo } = this.state;
    const text = [
      `Error: ${error?.message ?? "Unknown"}`,
      `Stack: ${error?.stack ?? ""}`,
      `Component stack: ${errorInfo?.componentStack ?? ""}`,
    ].join("\n\n");

    navigator.clipboard
      .writeText(text)
      .then(() => {
        this.setState({ copied: true });
        setTimeout(() => this.setState({ copied: false }), 2000);
      })
      .catch(() => {
        // If clipboard fails, silently swallow — we are already on the error screen.
      });
  }

  render(): ReactNode {
    if (!this.state.hasError) {
      return this.props.children;
    }

    // Custom fallback provided by the caller.
    if (this.props.fallback && this.state.error) {
      return this.props.fallback(this.state.error, this.reset);
    }

    return (
      <ErrorFallback
        error={this.state.error}
        section={this.props.section}
        copied={this.state.copied}
        onReset={this.reset}
        onCopy={this.copyError}
      />
    );
  }
}

// ─── Default crash recovery screen ───────────────────────────────────────────

interface ErrorFallbackProps {
  error: Error | null;
  section?: string;
  copied: boolean;
  onReset: () => void;
  onCopy: () => void;
}

function ErrorFallback({ error, section, copied, onReset, onCopy }: ErrorFallbackProps) {
  const isGlobal = !section;
  const errorCode = error?.name ?? "Error";
  const errorMessage = error?.message ? truncate(error.message, 100) : "An unexpected error occurred.";

  function handleReload() {
    window.location.reload();
  }

  function handleOpenBrowser() {
    // `openExternal` already picks the right mechanism per host: the
    // operating system's browser inside the desktop window, a new tab in a
    // browser. `window.open` on its own opens nothing in a WebView.
    void openExternal(window.location.href);
  }

  return (
    <div
      className={isGlobal ? "eb-root eb-root--global" : "eb-root eb-root--section"}
      role="alert"
      aria-live="assertive"
    >
      <style>{STYLES}</style>

      {/* Subtle animated noise grain background — global only */}
      {isGlobal && <div className="eb-grain" aria-hidden="true" />}

      <div className="eb-card">
        {/* Status pill */}
        <div className="eb-status-row">
          <span className="eb-pulse" aria-hidden="true" />
          <span className="eb-status-label">Runtime Error</span>
        </div>

        {/* Logo */}
        <div className="eb-logo" aria-hidden="true">
          <AOSMark />
        </div>

        {/* Text */}
        <div className="eb-text">
          <h1 className="eb-title">
            {isGlobal ? "Something went wrong" : `Error in ${section}`}
          </h1>
          <p className="eb-desc">
            {isGlobal
              ? "AOS caught a rendering error and stopped to protect your data."
              : "This section encountered an unexpected error and has been isolated."}
          </p>
        </div>

        {/* Error pill */}
        {error && (
          <div className="eb-error-pill" title={error.message}>
            <span className="eb-error-name">{errorCode}</span>
            <span className="eb-error-sep" aria-hidden="true">·</span>
            <span className="eb-error-msg">{errorMessage}</span>
          </div>
        )}

        {/* Actions */}
        <div className="eb-actions">
          {isGlobal ? (
            <button id="eb-reload" className="eb-btn eb-btn--primary" onClick={handleReload}>
              <ReloadIcon />
              Reload AOS
            </button>
          ) : (
            <button id="eb-reset" className="eb-btn eb-btn--primary" onClick={onReset}>
              <ReloadIcon />
              Try again
            </button>
          )}

          <div className="eb-secondary-actions">
            <button id="eb-copy" className="eb-btn eb-btn--ghost" onClick={onCopy}>
              {copied ? <CheckIcon /> : <CopyIcon />}
              {copied ? "Copied" : "Copy details"}
            </button>

            {isGlobal && (
              <button id="eb-browser" className="eb-btn eb-btn--ghost" onClick={handleOpenBrowser}>
                <ExternalIcon />
                Open in browser
              </button>
            )}
          </div>
        </div>
      </div>
    </div>
  );
}

// ─── Helpers ──────────────────────────────────────────────────────────────────

function truncate(str: string, max: number): string {
  return str.length > max ? `${str.slice(0, max)}…` : str;
}

// ─── Icons ────────────────────────────────────────────────────────────────────

function AOSMark() {
  return (
    <svg width="32" height="32" viewBox="0 0 32 32" fill="none" aria-label="AOS">
      {/* Diamond outline */}
      <path
        d="M16 3L29 16L16 29L3 16L16 3Z"
        stroke="currentColor"
        strokeWidth="1.5"
        strokeLinejoin="round"
        fill="none"
        opacity="0.5"
      />
      {/* Inner diamond filled */}
      <path
        d="M16 9L23 16L16 23L9 16L16 9Z"
        stroke="currentColor"
        strokeWidth="1.5"
        strokeLinejoin="round"
        fill="none"
        opacity="0.8"
      />
      {/* Center dot */}
      <circle cx="16" cy="16" r="2" fill="currentColor" />
    </svg>
  );
}

function ReloadIcon() {
  return (
    <svg width="13" height="13" viewBox="0 0 14 14" fill="none" aria-hidden="true">
      <path
        d="M1 7a6 6 0 1 0 .75-2.91M1 1v3.5h3.5"
        stroke="currentColor"
        strokeWidth="1.5"
        strokeLinecap="round"
        strokeLinejoin="round"
      />
    </svg>
  );
}

function CopyIcon() {
  return (
    <svg width="13" height="13" viewBox="0 0 14 14" fill="none" aria-hidden="true">
      <rect x="4.5" y="4.5" width="8" height="8" rx="1.5" stroke="currentColor" strokeWidth="1.4" />
      <path
        d="M4.5 9.5H2.5A1.5 1.5 0 0 1 1 8V2.5A1.5 1.5 0 0 1 2.5 1H8A1.5 1.5 0 0 1 9.5 2.5V4.5"
        stroke="currentColor"
        strokeWidth="1.4"
        strokeLinecap="round"
      />
    </svg>
  );
}

function CheckIcon() {
  return (
    <svg width="13" height="13" viewBox="0 0 14 14" fill="none" aria-hidden="true">
      <path d="M2 7l3.5 3.5L12 3" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round" />
    </svg>
  );
}

function ExternalIcon() {
  return (
    <svg width="13" height="13" viewBox="0 0 14 14" fill="none" aria-hidden="true">
      <path
        d="M8.5 1.5H12.5V5.5M12.5 1.5L7 7M6 2.5H2.5A1 1 0 0 0 1.5 3.5V11.5A1 1 0 0 0 2.5 12.5H10.5A1 1 0 0 0 11.5 11.5V8"
        stroke="currentColor"
        strokeWidth="1.4"
        strokeLinecap="round"
        strokeLinejoin="round"
      />
    </svg>
  );
}

// ─── Styles ───────────────────────────────────────────────────────────────────
// Self-contained — no Tailwind required. Adapts to the app's CSS custom
// properties automatically. Gracefully falls back to hardcoded dark values.

const STYLES = `
  /* ── Keyframes ─────────────────────────────────────────────── */

  @keyframes eb-fade-up {
    from { opacity: 0; transform: translateY(10px); }
    to   { opacity: 1; transform: translateY(0); }
  }

  @keyframes eb-pulse-ring {
    0%, 100% { opacity: 1; transform: scale(1); }
    50%       { opacity: 0.4; transform: scale(1.6); }
  }

  @keyframes eb-grain {
    0%, 100% { transform: translate(0, 0); }
    10%       { transform: translate(-2%, -3%); }
    20%       { transform: translate(3%, 2%); }
    30%       { transform: translate(-1%, 4%); }
    40%       { transform: translate(4%, -1%); }
    50%       { transform: translate(-3%, 2%); }
    60%       { transform: translate(2%, -4%); }
    70%       { transform: translate(-4%, 1%); }
    80%       { transform: translate(3%, 3%); }
    90%       { transform: translate(-2%, -2%); }
  }

  /* ── Root ──────────────────────────────────────────────────── */

  .eb-root {
    font-family: "Geist Variable", system-ui, -apple-system, sans-serif;
    color: var(--foreground, hsl(210 40% 98%));
  }

  .eb-root--global {
    position: fixed;
    inset: 0;
    display: flex;
    align-items: center;
    justify-content: center;
    background: var(--background, hsl(220 13% 7%));
    z-index: 9999;
    overflow: hidden;
  }

  .eb-root--section {
    position: relative;
    display: flex;
    align-items: center;
    justify-content: center;
    min-height: 280px;
    padding: 24px;
    border-radius: 12px;
    background: var(--card, hsl(220 13% 10%));
    border: 1px solid var(--border, hsl(215 20% 14%));
    overflow: hidden;
  }

  /* ── Grain texture (global only) ─────────────────────────── */

  .eb-grain {
    pointer-events: none;
    position: absolute;
    inset: -60%;
    width: 220%;
    height: 220%;
    opacity: 0.022;
    background-image: url("data:image/svg+xml,%3Csvg viewBox='0 0 256 256' xmlns='http://www.w3.org/2000/svg'%3E%3Cfilter id='noise'%3E%3CfeTurbulence type='fractalNoise' baseFrequency='0.9' numOctaves='4' stitchTiles='stitch'/%3E%3C/filter%3E%3Crect width='100%25' height='100%25' filter='url(%23noise)'/%3E%3C/svg%3E");
    background-size: 180px 180px;
    animation: eb-grain 8s steps(10) infinite;
  }

  /* ── Card ───────────────────────────────────────────────────── */

  .eb-card {
    position: relative;
    z-index: 1;
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 20px;
    width: 100%;
    max-width: 400px;
    text-align: center;
    animation: eb-fade-up 0.35s cubic-bezier(0.22, 1, 0.36, 1) both;
  }

  /* ── Status pill ────────────────────────────────────────────── */

  .eb-status-row {
    display: inline-flex;
    align-items: center;
    gap: 6px;
    padding: 4px 10px 4px 8px;
    border-radius: 100px;
    background: color-mix(in oklab, var(--destructive) 12%, transparent);
    border: 1px solid color-mix(in oklab, var(--destructive) 25%, transparent);
  }

  .eb-pulse {
    position: relative;
    display: block;
    width: 6px;
    height: 6px;
    border-radius: 50%;
    background: var(--destructive, hsl(0 72% 55%));
    flex-shrink: 0;
  }

  .eb-pulse::before {
    content: "";
    position: absolute;
    inset: -3px;
    border-radius: 50%;
    background: color-mix(in oklab, var(--destructive) 35%, transparent);
    animation: eb-pulse-ring 2s ease infinite;
  }

  .eb-status-label {
    font-size: 0.6875rem;
    font-weight: 500;
    letter-spacing: 0.04em;
    text-transform: uppercase;
    color: var(--destructive, hsl(0 72% 60%));
  }

  /* ── Logo ───────────────────────────────────────────────────── */

  .eb-logo {
    color: var(--foreground, hsl(210 40% 98%));
    opacity: 0.55;
    line-height: 0;
  }

  /* ── Text ───────────────────────────────────────────────────── */

  .eb-text {
    display: flex;
    flex-direction: column;
    gap: 6px;
  }

  .eb-title {
    margin: 0;
    font-size: 1.125rem;
    font-weight: 600;
    letter-spacing: -0.025em;
    color: var(--foreground, hsl(210 40% 98%));
    line-height: 1.3;
  }

  .eb-desc {
    margin: 0;
    font-size: 0.8125rem;
    line-height: 1.55;
    color: var(--muted-foreground, hsl(215 16% 47%));
    max-width: 320px;
  }

  /* ── Error pill ─────────────────────────────────────────────── */

  .eb-error-pill {
    display: inline-flex;
    align-items: baseline;
    gap: 6px;
    padding: 6px 12px;
    border-radius: 8px;
    background: var(--muted, hsl(215 20% 12%));
    border: 1px solid var(--border, hsl(215 20% 16%));
    max-width: 100%;
    overflow: hidden;
  }

  .eb-error-name {
    font-size: 0.6875rem;
    font-weight: 600;
    letter-spacing: 0.02em;
    color: var(--destructive, hsl(0 72% 58%));
    white-space: nowrap;
    flex-shrink: 0;
    font-family: ui-monospace, "Cascadia Code", monospace;
  }

  .eb-error-sep {
    color: var(--border, hsl(215 20% 28%));
    font-size: 0.75rem;
    flex-shrink: 0;
  }

  .eb-error-msg {
    font-size: 0.6875rem;
    font-family: ui-monospace, "Cascadia Code", monospace;
    color: var(--muted-foreground, hsl(215 16% 47%));
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
    min-width: 0;
  }

  /* ── Actions ────────────────────────────────────────────────── */

  .eb-actions {
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 8px;
    width: 100%;
    max-width: 280px;
  }

  .eb-secondary-actions {
    display: flex;
    gap: 6px;
    width: 100%;
  }

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
    transition: background 0.15s ease, color 0.15s ease, opacity 0.15s ease, transform 0.1s ease;
    outline: none;
    white-space: nowrap;
  }

  .eb-btn:active {
    transform: scale(0.97);
  }

  .eb-btn--primary {
    width: 100%;
    background: var(--foreground, hsl(210 40% 98%));
    color: var(--background, hsl(220 13% 7%));
  }

  .eb-btn--primary:hover {
    opacity: 0.9;
  }

  .eb-btn--ghost {
    flex: 1;
    background: var(--muted, hsl(215 20% 12%));
    color: var(--muted-foreground, hsl(215 16% 47%));
    border: 1px solid var(--border, hsl(215 20% 16%));
  }

  .eb-btn--ghost:hover {
    background: var(--accent, hsl(215 20% 16%));
    color: var(--foreground, hsl(210 40% 98%));
    border-color: var(--border, hsl(215 20% 22%));
  }
`;
