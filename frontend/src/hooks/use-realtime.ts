import { useEffect, useRef } from "react";
import { onRealtimeEvent, type RealtimeEvent } from "@/lib/realtime";
import { REALTIME_EVENT_MAP } from "@/lib/realtime-event-map";

/**
 * Escuta um evento do canal realtime, traduzido do vocabulário original para
 * o do daemon.
 *
 * Antes desta correção (B1 da revisão final), este hook assinava um
 * `RealtimeClient` próprio copiado do original (`app/lib/realtime.ts`) — um
 * segundo WebSocket, sem o escopo por workspace, falando nomes de evento
 * (`chat:refresh`, `files:changed`, ...) que o daemon nunca emite. Todo
 * `useRealtime(...)` do código portado era inalcançável. Agora este hook
 * assina o único socket real (`lib/realtime.ts`, montado em
 * `root-layout.tsx`) através de `onRealtimeEvent`, e resolve o nome
 * original para o nome real do daemon via `REALTIME_EVENT_MAP`
 * (`lib/realtime-event-map.ts`) — o mesmo ponto único de tradução que
 * `command-map.ts` já é para chamadas, uma camada acima.
 *
 * A tipagem genérica do original derivava os nomes de evento do registry de
 * stores do Igniter. Sem esse pacote, o nome é uma string: o ganho de inferir
 * `"chat:refresh"` de um registry não paga trazer a dependência de volta.
 *
 * @example useRealtime("chat:refresh", (p) => { if (p.chatId === id) refetch(); }, [id])
 */
export function useRealtime(
  event: string,
  callback: (payload: any) => void,
  deps: React.DependencyList = [],
  enabled = true,
) {
  // Use a ref to always point to the latest callback without re-subscribing on every render
  const callbackRef = useRef(callback);

  useEffect(() => {
    callbackRef.current = callback;
  }, [callback]);

  useEffect(() => {
    if (!enabled) {
      return;
    }

    const entry = REALTIME_EVENT_MAP[event];

    if (entry === undefined) {
      // Same "fail loud" stance `command-map.ts` takes on a missing key: a
      // call site using a name nobody translated is a programming error,
      // not backend state — and unlike a failed `call()`, a subscription
      // that silently never fires has no error state for the UI to land
      // in at all.
      console.error(
        `[useRealtime] "${event}" is not in REALTIME_EVENT_MAP — register it in lib/realtime-event-map.ts`,
      );
      return;
    }

    if (entry === null) {
      // Declared, not silent: the daemon has no counterpart for this
      // AOS event today. See this event's own comment in
      // `realtime-event-map.ts` for why.
      return;
    }

    const descriptor = typeof entry === "string" ? { type: entry } : entry;

    const unsubscribe = onRealtimeEvent((raw: RealtimeEvent) => {
      if (raw.type !== descriptor.type) return;
      const payload = descriptor.adapt ? descriptor.adapt(raw) : raw.data;
      callbackRef.current(payload);
    });

    return () => {
      unsubscribe();
    };
    // Re-subscribe ONLY if the event name or provided dependencies change
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [event, enabled, ...deps]);
}
