import { afterEach, describe, expect, it, vi } from "vitest";
import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { createPortal } from "react-dom";
import { useForm } from "react-hook-form";
import type { ReactNode } from "react";

import { Form } from "./form";

afterEach(cleanup);

// `aos.useForm` hands <Form> a react-hook-form object with `submit` bolted
// on (app/builders/app.tsx). The spy stands in for that submit: what is
// under test is whether a submit event reaches it, not what it does next.
function Harness({ submit, children }: { submit: () => void; children: ReactNode }) {
  const form = Object.assign(useForm(), { submit });
  return <Form form={form}>{children}</Form>;
}

// Records whether the browser would have gone on to submit natively. A
// native submission of these forms is a GET that navigates the window away
// (field values in the URL, the `?daemon=` address lost), so every submit
// event has to leave here cancelled.
// Captured on the way down and read after dispatch, so a handler that stops
// propagation cannot hide the event from the check.
function watchNativeSubmissions() {
  const seen: Event[] = [];
  const listener = (event: Event) => seen.push(event);
  window.addEventListener("submit", listener, true);
  return {
    get cancelled() {
      return seen.length > 0 && seen.every((event) => event.defaultPrevented);
    },
    stop: () => window.removeEventListener("submit", listener, true),
  };
}

describe("Form", () => {
  it("runs the form's own submit when a submit button is clicked", () => {
    const submit = vi.fn();
    const native = watchNativeSubmissions();
    render(
      <Harness submit={submit}>
        <input aria-label="title" />
        <button type="submit">Create</button>
      </Harness>,
    );

    fireEvent.click(screen.getByRole("button", { name: "Create" }));

    expect(submit).toHaveBeenCalledTimes(1);
    expect(native.cancelled).toBe(true);
    native.stop();
  });

  it("submits when the form is asked to without a submitter, if it has a submit button", () => {
    const submit = vi.fn();
    const { container } = render(
      <Harness submit={submit}>
        <input aria-label="title" />
        <button type="submit">Create</button>
      </Harness>,
    );

    container.querySelector("form")!.requestSubmit();

    expect(submit).toHaveBeenCalledTimes(1);
  });

  // A <button> with no type is a submit button to the browser. Most of the
  // ones inside these forms are toolbar, "add row" or tab buttons whose
  // author never meant them to save anything; while Form swallowed every
  // submit that went unnoticed, and wiring submission up must not turn
  // each of them into a save.
  it("ignores a button that never said it submits", () => {
    const submit = vi.fn();
    const onClick = vi.fn();
    const native = watchNativeSubmissions();
    render(
      <Harness submit={submit}>
        <button onClick={onClick}>Add id</button>
        <button type="submit">Save</button>
      </Harness>,
    );

    fireEvent.click(screen.getByRole("button", { name: "Add id" }));

    expect(onClick).toHaveBeenCalledTimes(1);
    expect(submit).not.toHaveBeenCalled();
    expect(native.cancelled).toBe(true);
    native.stop();
  });

  // Pages that save from a header button (goals, agents, instructions) have
  // no submit button, and their search box is often the only text field in
  // the form. Enter there is an implicit submission with no submitter; it
  // must not save, or validate, an entity the person was not editing.
  it("does not submit a form that has no submit button", () => {
    const submit = vi.fn();
    const native = watchNativeSubmissions();
    const { container } = render(
      <Harness submit={submit}>
        <input aria-label="search" />
        <button type="button" onClick={() => {}}>Save changes</button>
      </Harness>,
    );

    container.querySelector("form")!.requestSubmit();

    expect(submit).not.toHaveBeenCalled();
    expect(native.cancelled).toBe(true);
    native.stop();
  });

  // Blink and WebKit stop a submit event that bubbles out of a nested form
  // at the outer <form>, so a handler that waits for the bubble phase never
  // runs and the browser submits natively. The listener on the outer form
  // below does what those engines do; jsdom does not do it by itself.
  it("still submits a nested form when the engine stops the event at the outer form", () => {
    const outer = vi.fn();
    const inner = vi.fn();
    const native = watchNativeSubmissions();
    const { container } = render(
      <Harness submit={outer}>
        <Harness submit={inner}>
          <input aria-label="label" />
          <button type="submit">Create task type</button>
        </Harness>
      </Harness>,
    );
    const [outerForm] = container.querySelectorAll("form");
    let stoppedAtOuterForm = false;
    outerForm.addEventListener("submit", (event) => {
      if (event.target !== outerForm) {
        stoppedAtOuterForm = true;
        event.stopPropagation();
      }
    });

    fireEvent.click(screen.getByRole("button", { name: "Create task type" }));

    expect(stoppedAtOuterForm).toBe(true);
    expect(inner).toHaveBeenCalledTimes(1);
    expect(outer).not.toHaveBeenCalled();
    native.stop();
  });

  // React events bubble through portals along the component tree, so a
  // dialog's own <form> rendered from inside a page's <Form> reaches the
  // page's handler even though the DOM does not nest them.
  it("leaves a portalled child form's submission to that form", () => {
    const pageSubmit = vi.fn();
    const dialogSubmit = vi.fn((event: { preventDefault: () => void }) => event.preventDefault());
    render(
      <Harness submit={pageSubmit}>
        <button type="submit">Save page</button>
        {createPortal(
          <form onSubmit={dialogSubmit}>
            <button type="submit">Create file</button>
          </form>,
          document.body,
        )}
      </Harness>,
    );

    fireEvent.click(screen.getByRole("button", { name: "Create file" }));

    expect(dialogSubmit).toHaveBeenCalledTimes(1);
    expect(pageSubmit).not.toHaveBeenCalled();
  });
});
