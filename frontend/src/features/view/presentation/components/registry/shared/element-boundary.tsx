import * as React from "react";
import type { BaseComponentProps } from "@json-render/react";
import { t } from "@/lib/i18n";

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
      const Wrapped = (props: BaseComponentProps<any>) => (
        <ElementBoundary name={name}>
          <Inner {...props} />
        </ElementBoundary>
      );
      return [name, Wrapped];
    }),
  ) as T;
}
