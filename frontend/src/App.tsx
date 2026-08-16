import { useEffect, useState } from "react";
import type { JSX } from "react";
import { QueryClient, QueryClientProvider, useQuery } from "@tanstack/react-query";
import { client, DomainError, isDesktop, setWorkspace } from "@/lib/client";
import { useRealtime } from "@/lib/realtime";
import {
  listThemes,
  resolveAppearance,
  selectTheme,
  storedChoice,
  watchSystemAppearance,
  type AppearancePreference,
  type ThemeSummary,
} from "@/lib/theme";
import { ChatScreen, Failure } from "@/features/chat/ChatScreen";
import { TaskBoard } from "@/features/task/TaskBoard";
import { MemoryGraph } from "@/features/memory/MemoryGraph";
import { ApprovalModal } from "@/components/ApprovalModal";

const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      // Realtime invalidates what changed, so polling would only add load.
      // Retrying a domain refusal would repeat a refusal.
      refetchOnWindowFocus: false,
      retry: (count, error) => !(error instanceof DomainError) && count < 2,
    },
  },
});

type Screen = "chat" | "board" | "graph";

export function App(): JSX.Element {
  return (
    <QueryClientProvider client={queryClient}>
      <Shell />
    </QueryClientProvider>
  );
}

function Shell(): JSX.Element {
  const [screen, setScreen] = useState<Screen>("chat");
  const connection = useRealtime(queryClient);

  const workspace = useQuery({
    queryKey: ["workspace"],
    queryFn: async () => {
      const out = (await client.invoke("workspace_get", {
        _reasoning: "the application is starting",
      })) as { id: string; name: string };
      setWorkspace(out.id);
      return out;
    },
  });

  const chat = useQuery({
    queryKey: ["chats"],
    queryFn: async () =>
      (await client.invoke("chats_list", {
        _reasoning: "the application is starting",
      })) as { chats: Array<{ id: string; title: string }> },
  });

  const firstChat = chat.data?.chats?.[0]?.id ?? "";

  return (
    <div className="app">
      <aside className="sidebar">
        <h1>{workspace.data?.name ?? "AOS"}</h1>
        <nav aria-label="Screens">
          <button aria-current={screen === "chat" ? "page" : undefined} onClick={() => setScreen("chat")}>
            Chat
          </button>
          <button aria-current={screen === "board" ? "page" : undefined} onClick={() => setScreen("board")}>
            Tasks
          </button>
          <button aria-current={screen === "graph" ? "page" : undefined} onClick={() => setScreen("graph")}>
            Memories
          </button>
        </nav>
        <footer>
          <ThemePicker />
          <span className="status" data-state={connection} aria-live="polite">
            <span className="dot" />
            {connection === "open" ? "connected" : connection}
          </span>
          <span className="status">{isDesktop() ? "desktop" : "browser"}</span>
        </footer>
      </aside>

      <main className="main">
        {workspace.error && <Failure error={workspace.error} />}

        {screen === "chat" && (
          <>
            <header>
              <h2>Chat</h2>
              <p className="subtitle">Talk to the agents of this workspace.</p>
            </header>
            <ChatScreen chatId={firstChat} />
          </>
        )}

        {screen === "board" && (
          <>
            <header>
              <h2>Tasks</h2>
              <p className="subtitle">
                Moving a card calls the same command an agent calls, with the same guards behind it.
              </p>
            </header>
            <TaskBoard />
          </>
        )}

        {screen === "graph" && (
          <>
            <header>
              <h2>Memories</h2>
              <p className="subtitle">What the orchestrator knows, and how it connects.</p>
            </header>
            <MemoryGraph agent="atlas" />
          </>
        )}
      </main>

      <ApprovalModal />
    </div>
  );
}

/** The theme picker, and the listener that keeps `auto` following the system. */
function ThemePicker(): JSX.Element {
  const initial = storedChoice();
  const [theme, setTheme] = useState(initial.theme);
  const [preference, setPreference] = useState<AppearancePreference>(initial.appearance);
  const [themes, setThemes] = useState<ThemeSummary[]>([]);
  const [failed, setFailed] = useState(false);

  useEffect(() => {
    listThemes().then(setThemes).catch(() => setFailed(true));
  }, []);

  useEffect(() => {
    selectTheme(theme, preference).catch(() => setFailed(true));
  }, [theme, preference]);

  useEffect(() => {
    // Only while the preference is auto: somebody who chose dark did not ask to
    // be switched to light at sunrise.
    if (preference !== "auto") return;
    return watchSystemAppearance(() => {
      void selectTheme(theme, "auto");
    });
  }, [preference, theme]);

  if (failed) return <span className="status">theme unavailable</span>;

  return (
    <>
      <label className="status">
        Theme
        <select value={theme} onChange={(e) => setTheme(e.target.value)} aria-label="Theme">
          {themes.map((t) => (
            <option key={t.id} value={t.id}>
              {t.name}
            </option>
          ))}
        </select>
      </label>
      <label className="status">
        {resolveAppearance(preference)}
        <select
          value={preference}
          onChange={(e) => setPreference(e.target.value as AppearancePreference)}
          aria-label="Appearance"
        >
          <option value="auto">Follow the system</option>
          <option value="light">Light</option>
          <option value="dark">Dark</option>
        </select>
      </label>
    </>
  );
}
