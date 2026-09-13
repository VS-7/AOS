import { describe, it, expect, beforeEach } from "vitest";
import { render, screen, cleanup, act } from "@testing-library/react";
import * as React from "react";
import { AlertProvider, useAlert } from "./alert-provider";
import { setLocale } from "@/lib/i18n";

function Asker({ onReady }: { onReady: (confirm: ReturnType<typeof useAlert>["confirm"]) => void }) {
  const { confirm } = useAlert();
  React.useEffect(() => onReady(confirm), [confirm, onReady]);
  return null;
}

async function ask(options: Parameters<ReturnType<typeof useAlert>["confirm"]>[0]) {
  let confirm!: ReturnType<typeof useAlert>["confirm"];
  render(
    <AlertProvider>
      <Asker onReady={(c) => (confirm = c)} />
    </AlertProvider>,
  );
  await act(async () => {
    void confirm(options);
  });
}

beforeEach(() => {
  cleanup();
  try {
    setLocale("en");
  } catch {
    // no storage in this runner; the locale still switches
  }
});

describe("AlertProvider confirm", () => {
  // "Remove finished jobs?" drew its Remove button solid black: the variant
  // went to an inner Button that AlertDialogAction's own default-variant
  // Button then overrode.
  it("draws a destructive confirmation as destructive", async () => {
    await ask({ title: "Remove finished jobs?", confirmText: "Remove", variant: "destructive" });
    const remove = screen.getByRole("button", { name: "Remove" }).className;
    expect(remove).toMatch(/bg-destructive/);
    expect(remove).not.toMatch(/(^|\s)bg-primary(\s|$)/);
  });

  // Its default Cancel was the literal "Cancel" in every language.
  it("labels the default buttons in the language on screen", async () => {
    setLocale("pt-BR");
    await ask({ title: "Remover?" });
    expect(screen.getByRole("button", { name: "Cancelar" })).toBeTruthy();
  });
});
