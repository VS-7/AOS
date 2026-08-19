import { AosStore } from "@/app/builders/store";
import type { FractalAgentRealtimeSessionState } from "../../interfaces/realtime.interfaces";

/**
 * Global store managing the realtime voice call session state.
 */
export const RealtimeStore = AosStore.create("realtime")
  .withState<FractalAgentRealtimeSessionState>({
    status: "idle",
    agentId: null,
    sessionDuration: 0,
    isMuted: false,
    error: null,
  })
  .withPersistence({
    enabled: false,
  })
  .withNamespace({
    resolver: ({ namespaces }) => namespaces.workspaceId,
    strategy: "memory-partition",
  })
  .addAction("startCall", (ctx) => (agentId: string) => {
    console.log("[Realtime:Store] startCall → connecting", { agentId });
    ctx.state.set({
      status: "connecting",
      agentId,
      sessionDuration: 0,
      isMuted: false,
      error: null,
    });
  })
  .addAction("endCall", (ctx) => () => {
    console.log("[Realtime:Store] endCall → idle");
    ctx.state.set({
      status: "idle",
      agentId: null,
      sessionDuration: 0,
      isMuted: false,
      error: null,
    });
  })
  .addAction("setConnected", (ctx) => () => {
    console.log("[Realtime:Store] setConnected → connected");
    ctx.state.set((state) => ({
      ...state,
      status: "connected",
    }));
  })
  .addAction("toggleMute", (ctx) => () => {
    const next = !ctx.state.get().isMuted;
    console.log("[Realtime:Store] toggleMute →", { isMuted: next });
    ctx.state.set((state) => ({
      ...state,
      isMuted: next,
    }));
  })
  .addAction("tickDuration", (ctx) => () => {
    ctx.state.set((state) => ({
      ...state,
      sessionDuration: state.sessionDuration + 1,
    }));
  })
  .addAction("setError", (ctx) => (error: string | null) => {
    console.log("[Realtime:Store] setError", { error });
    ctx.state.set((state) => ({
      ...state,
      error,
    }));
  })
  .build();

export type RealtimeStoreType = typeof RealtimeStore;
