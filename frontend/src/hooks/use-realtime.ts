import { useEffect, useRef } from "react";
// The Fractal source's own realtime singleton (`RealtimeClient`, copied
// into `app/lib/realtime.ts` in this task's Step 2) — NOT `@/lib/realtime`,
// an unrelated, already-built websocket-connection hook of AOS's own
// (`useRealtime(queryClient)`, invoked once in `App.tsx`) that happens to
// share this file's exported hook's name but is a different mechanism.
import { realtime } from "@/app/lib/realtime";

/**
 * Escuta um evento do canal realtime.
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

    // Stable handler that calls the latest callback from the ref
    const handler = (payload: any) => {
      callbackRef.current(payload);
    };

    // Register listener via the singleton client
    const unsubscribe = realtime.on(event, handler);
    
    return () => {
      unsubscribe();
    };
    // Re-subscribe ONLY if the event name or provided dependencies change
  }, [event, enabled, ...deps]);
}
