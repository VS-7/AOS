/**
 * Absent from every extraction (`_extracted/{index,v401/web,v401/server}`
 * have no `realtime.interfaces.ts` under `features/agent/`) — reconstructed
 * from how `presentation/stores/realtime.store.ts` is the sole consumer of
 * `FractalAgentRealtimeSessionState`, the only type this file needs to
 * export.
 */
export type FractalAgentRealtimeSessionStatus =
  | "idle"
  | "connecting"
  | "connected";

export interface FractalAgentRealtimeSessionState {
  status: FractalAgentRealtimeSessionStatus;
  agentId: string | null;
  sessionDuration: number;
  isMuted: boolean;
  error: string | null;
}
