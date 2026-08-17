import { useEffect, useMemo, useRef, useState } from "react";
import type { JSX } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { client } from "@/lib/client";
import type { StreamingAnswer } from "@/lib/realtime";
import { Failure } from "@/components/Failure";

/** A part of a message, as the transcript stores it. */
interface Part {
  type: "text" | "reasoning" | "tool_call" | "tool_result" | "file";
  text?: string;
  toolName?: string;
  toolCallId?: string;
  input?: unknown;
  output?: unknown;
}

interface Message {
  id: string;
  role: "user" | "assistant" | "system";
  parts?: Part[];
  author?: { type: string; id: string };
  createdAt: string;
}

interface Chat {
  id: string;
  title: string;
  messages?: Message[];
}

/**
 * The conversation.
 *
 * Parts are rendered by type rather than concatenated: a tool call is a
 * different thing from an answer, and flattening them is how an interface ends
 * up showing a person a JSON payload in the middle of a sentence.
 */
export function ChatScreen({ chatId }: { chatId: string }): JSX.Element {
  const queryClient = useQueryClient();
  const [draft, setDraft] = useState("");
  const bottom = useRef<HTMLDivElement | null>(null);

  const conversation = useQuery({
    queryKey: ["chat", chatId],
    queryFn: async () =>
      (await client.invoke("chats_get", {
        chat: chatId,
        _reasoning: "the chat screen is open",
      })) as Chat,
    enabled: chatId !== "",
  });

  // What is arriving right now, accumulated by the realtime channel. It is
  // separate from the stored transcript so a turn in progress does not require
  // refetching the conversation once per token.
  const streaming = useQuery<StreamingAnswer>({
    queryKey: ["chat", chatId, "streaming"],
    queryFn: () => ({ text: "", reasoning: "" }),
    enabled: chatId !== "",
    staleTime: Infinity,
  });

  const send = useMutation({
    mutationFn: async (text: string) =>
      client.invoke("chats_send", {
        chat: chatId,
        text,
        _reasoning: "the person wrote a message",
      }),
    onSuccess: () => {
      setDraft("");
      queryClient.setQueryData<StreamingAnswer>(["chat", chatId, "streaming"], {
        text: "",
        reasoning: "",
      });
      void queryClient.invalidateQueries({ queryKey: ["chat", chatId] });
    },
  });

  const messages = useMemo(() => conversation.data?.messages ?? [], [conversation.data]);

  useEffect(() => {
    bottom.current?.scrollIntoView({ behavior: "smooth" });
  }, [messages.length, streaming.data?.text]);

  if (chatId === "") {
    return <p className="empty">No conversation is open.</p>;
  }
  if (conversation.isLoading) {
    return <p className="empty">Reading the conversation…</p>;
  }
  if (conversation.error) {
    return <Failure error={conversation.error} />;
  }

  const live = streaming.data?.text ?? "";

  return (
    <div className="chat">
      <div className="messages" role="log" aria-live="polite" aria-label="Conversation">
        {messages.length === 0 && <p className="empty">Nothing has been said yet.</p>}
        {messages.map((message) => (
          <MessageView key={message.id} message={message} />
        ))}
        {live !== "" && (
          <div className="message" data-role="assistant">
            <span className="who">answering</span>
            <div className="body">{live}</div>
          </div>
        )}
        <div ref={bottom} />
      </div>

      <form
        className="composer"
        onSubmit={(e) => {
          e.preventDefault();
          const text = draft.trim();
          if (text !== "") send.mutate(text);
        }}
      >
        <textarea
          value={draft}
          onChange={(e) => setDraft(e.target.value)}
          placeholder="Ask something. Address an agent with @slug."
          aria-label="Message"
          onKeyDown={(e) => {
            // Enter sends, shift-enter breaks the line. A composer where enter
            // inserts a newline is one where every message needs a mouse.
            if (e.key === "Enter" && !e.shiftKey) {
              e.preventDefault();
              const text = draft.trim();
              if (text !== "") send.mutate(text);
            }
          }}
        />
        <button type="submit" disabled={send.isPending || draft.trim() === ""}>
          {send.isPending ? "Sending…" : "Send"}
        </button>
      </form>
      {send.error && <Failure error={send.error} />}
    </div>
  );
}

function MessageView({ message }: { message: Message }): JSX.Element {
  const parts = message.parts ?? [];
  return (
    <article className="message" data-role={message.role}>
      <span className="who">{message.author?.id ?? message.role}</span>
      {parts.map((part, index) => (
        <PartView key={index} part={part} />
      ))}
    </article>
  );
}

function PartView({ part }: { part: Part }): JSX.Element | null {
  switch (part.type) {
    case "text":
      return <div className="body">{part.text}</div>;

    case "reasoning":
      // Collapsed by default. The master prompt asks the agent for the decision
      // rather than the transcript of arriving at it, and the interface agrees.
      return (
        <details className="reasoning">
          <summary>Reasoning</summary>
          <div className="body">{part.text}</div>
        </details>
      );

    case "tool_call":
      return (
        <details className="toolcall">
          <summary>{part.toolName ?? "tool"}</summary>
          <pre>{stringify(part.input)}</pre>
        </details>
      );

    case "tool_result": {
      const rendered = stringify(part.output);
      const truncated = rendered.includes("_truncated");
      return (
        <details className="toolcall">
          <summary>{part.toolName ?? "tool"} → result</summary>
          <pre>{rendered}</pre>
          {truncated && (
            <p className="truncated">
              Part of this output did not fit and was written to a file the agent can read.
            </p>
          )}
        </details>
      );
    }

    default:
      return null;
  }
}

function stringify(value: unknown): string {
  if (value === undefined || value === null) return "";
  if (typeof value === "string") return value;
  try {
    return JSON.stringify(value, null, 2);
  } catch {
    return String(value);
  }
}
