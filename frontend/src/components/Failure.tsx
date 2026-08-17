import type { JSX } from "react";
import { DomainError } from "@/lib/client";

/**
 * A failure, with the call to action the domain attached.
 *
 * Every error the system produces carries a code and a suggestion, and showing
 * the suggestion is the difference between an interface that reports a problem
 * and one that helps with it.
 */
export function Failure({ error }: { error: unknown }): JSX.Element {
  if (error instanceof DomainError) {
    return (
      <div className="error" role="alert">
        <p>{error.message}</p>
        {error.actions.map((action, index) => (
          <p className="cta" key={index}>
            {action.label}
            {action.command ? ` — ${action.command}` : ""}
          </p>
        ))}
      </div>
    );
  }
  return (
    <p className="error" role="alert">
      {error instanceof Error ? error.message : "Something went wrong."}
    </p>
  );
}
