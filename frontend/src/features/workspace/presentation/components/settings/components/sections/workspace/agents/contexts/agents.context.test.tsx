import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, render, waitFor } from "@testing-library/react";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import type { Agent } from "@/features/agent/interfaces/agent.interfaces";

/**
 * The agent editor's behaviour lives in this context, and every defect it
 * shipped was about *when* it talks to the daemon rather than what it
 * renders: a store refresh reloading the form under the person's typing, a
 * delete reading back the agent it had just removed, a cleared field that
 * never reached the wire. So the daemon is the mock here, and the assertions
 * are on the calls and on the form.
 */

const daemon = vi.hoisted(() => ({
  records: {} as Record<string, Record<string, unknown>>,
  getById: vi.fn(),
  update: vi.fn(),
  create: vi.fn(),
  remove: vi.fn(),
  refresh: vi.fn(),
  toasts: [] as string[],
}));

vi.mock("sonner", () => ({
  toast: Object.assign((m: string) => daemon.toasts.push(m), {
    success: (m: string) => daemon.toasts.push(`success:${m}`),
    error: (m: string) => daemon.toasts.push(`error:${m}`),
    info: (m: string) => daemon.toasts.push(`info:${m}`),
  }),
}));

// The real `aos.useForm` (app/builders/app.tsx) is react-hook-form with a
// `submit` that runs `onSubmit` and resets the form to what it returned, and
// that leaves the form untouched when `onSubmit` throws. That is the whole
// contract this context relies on, so that is all this stands in for.
vi.mock("@/app/aos", () => ({
  aos: {
    useForm: ({ schema, values, onSubmit }: any) => {
      const form = useForm({ resolver: zodResolver(schema), defaultValues: values });
      return Object.assign(form, {
        isLoading: false,
        submit: form.handleSubmit(async (data: any) => {
          try {
            const next = await onSubmit(data);
            form.reset(next ?? data);
          } catch {
            // A refusal: the form keeps what the person typed.
          }
        }),
      });
    },
    client: {
      agent: {
        getById: { query: daemon.getById },
        update: { mutate: daemon.update },
        create: { mutate: daemon.create },
        delete: {
          useMutation: ({ onSuccess, onError }: any) => ({
            loading: false,
            mutate: async (input: any) => {
              const out = await daemon.remove(input);
              if (out?.error) onError?.(out.error);
              else await onSuccess?.(out?.data);
            },
          }),
        },
      },
    },
    stores: { agent: { actions: { refresh: daemon.refresh } } },
  },
}));

const { AgentsProvider, useAgents } = await import("./agents.context");

type Ctx = ReturnType<typeof useAgents>;

function mount(agents: Agent[]) {
  const ref: { current: Ctx | null } = { current: null };
  function Probe() {
    ref.current = useAgents();
    return null;
  }
  const view = render(
    <AgentsProvider agents={agents}>
      <Probe />
    </AgentsProvider>,
  );
  const rerender = (next: Agent[]) =>
    view.rerender(
      <AgentsProvider agents={next}>
        <Probe />
      </AgentsProvider>,
    );
  return { ctx: () => ref.current!, rerender };
}

const builder = {
  id: "api-builder",
  name: "API Builder",
  description: "Executor",
  role: "Feature Engineer",
  provider: "codex",
  model: "gpt-5.6-terra",
  leader: "luara",
  reasoning: "medium",
  orchestrator: false,
  updatedAt: "2026-08-30T13:47:06Z",
} satisfies Agent;

const luara = {
  id: "luara",
  name: "Luara",
  orchestrator: true,
  updatedAt: "2026-08-30T11:28:53Z",
} satisfies Agent;

const INSTRUCTIONS = "# Mission\n\n<rules>\n- item\n</rules>\n<!-- note -->\n{{ var }} \\ end\n";

beforeEach(() => {
  daemon.toasts = [];
  daemon.records = {
    "api-builder": { ...builder, content: INSTRUCTIONS },
    luara: { ...luara, content: "hi" },
  };
  daemon.getById.mockReset().mockImplementation(async ({ params }: any) => {
    const found = daemon.records[params.agent];
    return found
      ? { data: { agent: found }, error: null }
      : { data: undefined, error: { code: "AOS_COLLECTION_NOT_FOUND", message: "not found" } };
  });
  daemon.update.mockReset().mockImplementation(async ({ params, body }: any) => {
    const next = { ...daemon.records[params.agent], ...body, updatedAt: "2026-09-13T00:00:00Z" };
    daemon.records[params.agent] = next;
    return { data: next, error: null };
  });
  daemon.create.mockReset();
  daemon.remove.mockReset().mockResolvedValue({ data: { deleted: true }, error: null });
  daemon.refresh.mockReset().mockResolvedValue(undefined);
});

afterEach(cleanup);

async function select(ctx: () => Ctx, id: string) {
  await act(async () => ctx().setSelectedAgentId(id));
  await waitFor(() => expect(ctx().isLoadingContent).toBe(false));
}

describe("AgentsProvider", () => {
  it("loads the selected agent once, instructions included", async () => {
    const { ctx } = mount([builder, luara]);
    await select(ctx, "api-builder");

    expect(daemon.getById).toHaveBeenCalledTimes(1);
    expect(ctx().form.getValues("content")).toBe(INSTRUCTIONS);
    expect(ctx().form.getValues("role")).toBe("Feature Engineer");
  });

  // R-A1 / #175: any store refresh — a realtime agent event, another
  // screen's save — replaced the array and reset the form from it.
  it("keeps unsaved edits when the roster refreshes", async () => {
    const { ctx, rerender } = mount([builder, luara]);
    await select(ctx, "api-builder");

    await act(async () => ctx().form.setValue("role", "UNSAVED ROLE", { shouldDirty: true }));
    // Another agent changed elsewhere: a new array, and this agent changed too.
    await act(async () =>
      rerender([{ ...builder, updatedAt: "2026-09-13T01:00:00Z" }, { ...luara, description: "x" }]),
    );

    expect(ctx().form.getValues("role")).toBe("UNSAVED ROLE");
    expect(daemon.getById).toHaveBeenCalledTimes(1);
  });

  it("does not read the agent again when a refresh brings nothing new for it", async () => {
    const { ctx, rerender } = mount([builder, luara]);
    await select(ctx, "api-builder");

    await act(async () => rerender([{ ...builder }, { ...luara, description: "changed" }]));

    expect(daemon.getById).toHaveBeenCalledTimes(1);
  });

  it("picks up a change made elsewhere when nothing here is unsaved", async () => {
    const { ctx, rerender } = mount([builder, luara]);
    await select(ctx, "api-builder");

    daemon.records["api-builder"] = { ...daemon.records["api-builder"], role: "Changed Elsewhere", updatedAt: "2026-09-13T02:00:00Z" };
    await act(async () => rerender([{ ...builder, role: "Changed Elsewhere", updatedAt: "2026-09-13T02:00:00Z" }, luara]));

    await waitFor(() => expect(ctx().form.getValues("role")).toBe("Changed Elsewhere"));
  });

  // #168 / #172 / #183: blanks were sent as undefined (dropped by JSON, so
  // "leave unchanged"), a skill nobody can set was sent, and the whole
  // instructions body went out on every save.
  it("sends a cleared field as empty and only the fields that changed", async () => {
    const { ctx } = mount([builder, luara]);
    await select(ctx, "api-builder");

    await act(async () => {
      for (const name of ["description", "role", "provider", "model"] as const) {
        ctx().form.setValue(name, "", { shouldDirty: true });
      }
    });
    await act(async () => ctx().form.submit());

    expect(daemon.update).toHaveBeenCalledTimes(1);
    const { body } = daemon.update.mock.calls[0][0];
    expect(body).toEqual({ description: "", role: "", provider: "", model: "" });
    expect(body).not.toHaveProperty("skill");
    expect(body).not.toHaveProperty("content");
    // The update answers with the agent; reading it back again is a waste.
    expect(daemon.getById).toHaveBeenCalledTimes(1);
  });

  it("saves instructions exactly as typed", async () => {
    const { ctx } = mount([builder, luara]);
    await select(ctx, "api-builder");

    const edited = `${INSTRUCTIONS}  \n- another  \n\n`;
    await act(async () => ctx().form.setValue("content", edited, { shouldDirty: true }));
    await act(async () => ctx().form.submit());

    expect(daemon.update.mock.calls[0][0].body).toEqual({ content: edited });
  });

  it("keeps the edits on screen when the daemon refuses the save", async () => {
    daemon.update.mockResolvedValueOnce({ data: undefined, error: { code: "AOS_AGENT_LEADER_CYCLE", message: "that leader would close a loop" } });
    const { ctx } = mount([builder, luara]);
    await select(ctx, "api-builder");

    await act(async () => ctx().form.setValue("role", "Typed", { shouldDirty: true }));
    await act(async () => ctx().form.submit());

    expect(daemon.toasts).toContain("error:that leader would close a loop");
    expect(daemon.toasts.some((m) => m.startsWith("success:"))).toBe(false);
    expect(ctx().form.getValues("role")).toBe("Typed");
    expect(ctx().form.formState.isDirty).toBe(true);
  });

  // #178: onSuccess refreshed the roster while the deleted id was still
  // selected, and the reload effect asked the daemon for it — twice.
  it("never reads back the agent it just deleted", async () => {
    const { ctx, rerender } = mount([builder, luara]);
    await select(ctx, "api-builder");
    // The roster arrives while the refresh is still in flight, the way the
    // network delivers it — not batched into the same render as the delete.
    let settle!: () => void;
    daemon.refresh.mockImplementation(() => new Promise<void>((resolve) => (settle = resolve)));

    await act(async () => void ctx().deleteSelectedAgent());
    await act(async () => rerender([luara]));
    await act(async () => settle());

    expect(daemon.remove).toHaveBeenCalledWith({ params: { agent: "api-builder" } });
    expect(ctx().selectedAgentId).toBeNull();
    expect(daemon.getById).toHaveBeenCalledTimes(1);
  });

  it("lets go of an agent that was deleted elsewhere", async () => {
    const { ctx, rerender } = mount([builder, luara]);
    await select(ctx, "api-builder");

    await act(async () => rerender([luara]));

    expect(ctx().selectedAgentId).toBeNull();
    expect(daemon.getById).toHaveBeenCalledTimes(1);
  });

  it("asks before switching away from unsaved edits", async () => {
    const { ctx } = mount([builder, luara]);
    await select(ctx, "api-builder");
    await act(async () => ctx().form.setValue("role", "UNSAVED", { shouldDirty: true }));

    await act(async () => ctx().setSelectedAgentId("luara"));
    expect(ctx().selectedAgentId).toBe("api-builder");
    expect(ctx().pendingSelection).toBe("luara");

    await act(async () => ctx().confirmPendingSelection());
    await waitFor(() => expect(ctx().selectedAgentId).toBe("luara"));
  });

  // #176 / #177: the daemon's answers were '"" is not a usable agent slug'
  // and a storage sentence about a collection — neither says what to change
  // in a form whose only identity field is the name.
  it("refuses a name that makes no id, before asking the daemon", async () => {
    const { ctx } = mount([builder, luara]);
    await act(async () => ctx().startCreate());
    await act(async () => ctx().form.setValue("name", "!!!", { shouldDirty: true }));
    await act(async () => ctx().form.submit());

    expect(daemon.create).not.toHaveBeenCalled();
    expect(ctx().form.getFieldState("name").error?.message).toMatch(/letters or digits/);
  });

  it("refuses a name whose id another agent already has", async () => {
    const { ctx } = mount([builder, luara]);
    await act(async () => ctx().startCreate());
    await act(async () => ctx().form.setValue("name", "Luára", { shouldDirty: true }));
    await act(async () => ctx().form.submit());

    expect(daemon.create).not.toHaveBeenCalled();
    expect(ctx().form.getFieldState("name").error?.message).toMatch(/luara/);
  });

  it("creates with only the fields that were filled, and opens what it created", async () => {
    daemon.create.mockImplementation(async ({ body }: any) => ({
      data: { id: "revisor", ...body, updatedAt: "2026-09-13T00:00:00Z" },
      error: null,
    }));
    const { ctx } = mount([builder, luara]);
    await act(async () => ctx().startCreate());
    await act(async () => {
      ctx().form.setValue("name", "Revisor", { shouldDirty: true });
      ctx().form.setValue("role", "  Reviewer ", { shouldDirty: true });
    });
    await act(async () => ctx().form.submit());

    expect(daemon.create.mock.calls[0][0].body).toEqual({ name: "Revisor", role: "Reviewer", orchestrator: false });
    expect(ctx().selectedAgentId).toBe("revisor");
    // The create answered with the record; there is nothing to read back.
    expect(daemon.getById).not.toHaveBeenCalled();
  });
});
