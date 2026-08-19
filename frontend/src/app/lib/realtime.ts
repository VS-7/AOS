/**
 * RealtimeClient
 * 
 * A singleton WebSocket client to manage real-time communication 
 * between the Fractal frontend and backend.
 */
class RealtimeClient {
  private socket: WebSocket | null = null;
  private listeners: Map<string, Set<(data: any) => void>> = new Map();
  private reconnectTimeout: any = null;

  constructor() {
    if (typeof window !== "undefined") {
      this.connect();
    }
  }

  private connect() {
    const protocol = window.location.protocol === "https:" ? "wss:" : "ws:";
    const host = window.location.host;
    
    this.socket = new WebSocket(`${protocol}//${host}/ws`);

    this.socket.onmessage = (event) => {
      try {
        const { type, data } = JSON.parse(event.data);
        this.listeners.get(type)?.forEach((callback) => callback(data));
      } catch (error) {
        console.error("[Realtime] Failed to parse message", error);
      }
    };

    this.socket.onclose = () => {
      console.warn("[Realtime] Connection closed. Reconnecting...");
      this.reconnect();
    };

    this.socket.onerror = (error) => {
      console.error("[Realtime] WebSocket error", error);
    };
  }

  private reconnect() {
    if (this.reconnectTimeout) return;
    this.reconnectTimeout = setTimeout(() => {
      this.reconnectTimeout = null;
      this.connect();
    }, 3000);
  }

  /**
   * Subscribes to a specific event type.
   */
  public on(event: string, callback: (data: any) => void) {
    if (!this.listeners.has(event)) {
      this.listeners.set(event, new Set());
    }

    this.listeners.get(event)!.add(callback);

    return () => {
      this.listeners.get(event)?.delete(callback);
    };
  }
}

export const realtime = new RealtimeClient();
