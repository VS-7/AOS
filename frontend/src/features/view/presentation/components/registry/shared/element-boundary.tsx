import * as React from "react";
import type { BaseComponentProps } from "@json-render/react";
import { toast } from "sonner";
import { t } from "@/lib/i18n";
import { errorMessage } from "@/lib/aos-facade";

type RegistryComponent = (props: BaseComponentProps<any>) => React.ReactNode;

/**
 * Catches one element's render failure and says so where the element was.
 *
 * json-render has a boundary of its own around every element, and it renders
 * `null` — the error goes to the console and the page shows nothing. A table
 * handed rows it could not map left a view entirely blank with no word of
 * why. This one sits inside that boundary, so it catches first.
 */
class ElementBoundary extends React.Component<
  { name: string; children: React.ReactNode },
  { error: Error | null }
> {
  state: { error: Error | null } = { error: null };

  static getDerivedStateFromError(error: Error) {
    return { error };
  }

  render() {
    if (!this.state.error) return this.props.children;
    return (
      <div
        role="alert"
        className="rounded-md border border-destructive/40 bg-destructive/5 px-3 py-2 text-xs text-destructive"
      >
        {t("This {{component}} could not be displayed: {{reason}}", {
          component: this.props.name,
          reason: this.state.error.message,
        })}
      </div>
    );
  }
}

/**
 * What json-render rejects an event with when the person answers Cancel to an
 * action's confirmation — an answer, not a failure.
 */
const CANCELLED = "Action cancelled";

/**
 * Fires an element's event and settles whatever it answers.
 *
 * json-render's `emit` is async and rejects when a confirmation is cancelled
 * or a handler throws; the components that call it (Button, Link, and every
 * shadcn one) never await it. So each Cancel on a confirmed action was an
 * uncaught "Action cancelled", and a handler that threw failed silently. A
 * cancellation now settles quietly, and anything else is said out loud.
 */
function settled(fire: () => unknown): Promise<void> {
  const report = (error: unknown) => {
    if (error instanceof Error && error.message === CANCELLED) return;
    toast.error(t("The action failed"), { description: errorMessage(error) ?? String(error) });
  };
  // Fired in the same tick as the click, not a microtask later: a handler
  // that needs the click's user activation still has it.
  try {
    return Promise.resolve(fire()).then(() => undefined, report);
  } catch (error) {
    report(error);
    return Promise.resolve();
  }
}

/**
 * Wraps every registry component so a failure renders a visible notice.
 *
 * The component is rendered as an element of its own, not called: `defineRegistry`
 * calls a registry entry as a plain function inside json-render's wrapper, so
 * a throw there happens outside any boundary this wrapper could place.
 */
export function withElementBoundaries<T extends Record<string, RegistryComponent>>(components: T): T {
  return Object.fromEntries(
    Object.entries(components).map(([name, Component]) => {
      const Inner = (props: BaseComponentProps<any>) => <>{Component(props)}</>;
      Inner.displayName = `View${name}`;
      const Wrapped = (props: BaseComponentProps<any>) => {
        // Typed as returning void; returning the settled promise costs the
        // callers nothing and lets whoever wants to wait on it.
        const emit = (event: string) => settled(() => props.emit(event)) as unknown as void;
        const on = (event: string) => {
          const handle = props.on(event);
          return { ...handle, emit: () => settled(() => handle.emit()) as unknown as void };
        };
        return (
          <ElementBoundary name={name}>
            <Inner {...props} emit={emit} on={on} />
          </ElementBoundary>
        );
      };
      return [name, Wrapped];
    }),
  ) as T;
}
