import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import * as React from "react";
import { act, cleanup, renderHook } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";

type MutationOptions = { onSuccess?: (r: unknown) => void; onError?: (e: unknown) => void };
const mutations = vi.hoisted(() => ({} as Record<string, { mutate: ReturnType<typeof vi.fn>; options: MutationOptions }>));
const refresh = vi.hoisted(() => vi.fn());
const confirm = vi.hoisted(() => vi.fn());
const closeChatTab = vi.hoisted(() => vi.fn());
const toast = vi.hoisted(() => ({ success: vi.fn(), error: vi.fn() }));

vi.mock("@/app/aos", () => {
  const node = (name: string) => ({
    useMutation: (options: MutationOptions) => {
      mutations[name] ??= { mutate: vi.fn(), options };
      mutations[name].options = options;
      return { mutate: mutations[name].mutate, loading: false };
    },
  });
  return {
    aos: {
      client: { chat: { update: node("update"), delete: node("delete"), clear: node("clear") } },
      stores: { chat: { actions: { refresh } } },
    },
  };
});
vi.mock("@/components/ui/alert-provider", () => ({ useAlert: () => ({ confirm }) }));
vi.mock("../helpers/open-chat-tab.helper", () => ({ closeChatTab }));
vi.mock("sonner", () => ({ toast }));

import { useChatActions } from "./use-chat-actions";

let queryClient: QueryClient;
function render(chat = { id: "c-1", title: "geral", kind: "channel" }, options = {}) {
  const wrapper = ({ children }: { children: React.ReactNode }) => (
    <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
  );
  return renderHook(() => useChatActions(chat, options), { wrapper });
}

beforeEach(() => {
  queryClient = new QueryClient();
  for (const key of Object.keys(mutations)) delete mutations[key];
  refresh.mockReset();
  confirm.mockReset();
  closeChatTab.mockReset();
  toast.success.mockReset();
  toast.error.mockReset();
});
afterEach(() => cleanup());

describe("useChatActions", () => {
  it("deletes only after the person confirms", async () => {
    confirm.mockResolvedValueOnce(false);
    const { result } = render();
    await act(() => result.current.remove());
    expect(mutations.delete.mutate).not.toHaveBeenCalled();

    expect(closeChatTab).not.toHaveBeenCalled();

    confirm.mockResolvedValueOnce(true);
    await act(() => result.current.remove());
    expect(closeChatTab).toHaveBeenCalledWith("c-1");
    expect(mutations.delete.mutate).toHaveBeenCalledWith({ params: { chat: "c-1" } });
  });

  // The tab used to close only when it was the focused one. An open tab in
  // the background kept the deleted transcript and a working composer, and
  // every later chat write refetched a conversation that no longer existed.
  it("closes the chat's tab and forgets its transcript once deleted, focused or not", async () => {
    queryClient.setQueryData(["chat", "getById", { chat: "c-1" }], { chat: { id: "c-1" } });
    const { result } = render();
    act(() => mutations.delete.options.onSuccess?.({ data: { deleted: true } }));

    expect(closeChatTab).toHaveBeenCalledWith("c-1");
    expect(queryClient.getQueryData(["chat", "getById", { chat: "c-1" }])).toBeUndefined();
    expect(refresh).toHaveBeenCalled();
    expect(toast.success).toHaveBeenCalled();
    expect(result.current).toBeTruthy();
  });

  // `/clear` dropped the whole transcript on Enter, with no way back.
  it("clears only after the person confirms, then hands the empty transcript back", async () => {
    const onCleared = vi.fn();
    confirm.mockResolvedValueOnce(true);
    const { result } = render({ id: "c-2", title: "Luara", kind: "dm" }, { onCleared });

    await act(() => result.current.clear());
    expect(confirm).toHaveBeenCalledWith(expect.objectContaining({ variant: "destructive" }));
    expect(mutations.clear.mutate).toHaveBeenCalledWith({ params: { chat: "c-2" } });

    act(() => mutations.clear.options.onSuccess?.({ data: { removed: 3 } }));
    expect(onCleared).toHaveBeenCalled();
  });

  it("renames with the trimmed title and ignores an unchanged one", () => {
    const { result } = render();
    act(() => result.current.rename("  geral  "));
    expect(mutations.update.mutate).not.toHaveBeenCalled();
    act(() => result.current.rename("  produto "));
    expect(mutations.update.mutate).toHaveBeenCalledWith({ params: { chat: "c-1" }, body: { title: "produto" } });
  });

  it("says why a refusal happened", () => {
    render();
    act(() => mutations.delete.options.onError?.(new Error("the record could not be removed")));
    expect(toast.error).toHaveBeenCalledWith("the record could not be removed");
  });
});
