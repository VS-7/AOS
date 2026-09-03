import { describe, expect, it } from "vitest";
import { isToolError, toLiveUiMessage, toUiChat, toUiMessage } from "./chat-message";

/**
 * The fixtures here are the shapes the daemon actually answers with:
 * `internal/domain/chat.Message`, with a `tool-call` and a `tool-result` as
 * two flat parts sharing a `toolCallId`, and the execution attempt recorded
 * on the message that *asked* rather than on the answer.
 */

const call = (id: string, name: string, input: unknown = {}) => ({
  type: "tool-call",
  toolName: name,
  toolCallId: id,
  input,
});

const result = (id: string, name: string, output: unknown) => ({
  type: "tool-result",
  toolName: name,
  toolCallId: id,
  output,
});

describe("tool parts", () => {
  // Reading Go's two parts as two AI-SDK tool parts made every tool appear
  // twice in the thinking timeline and in the header counters — "2 reads" for
  // one read — and always as complete, because `state` was undefined.
  it("folds a call and its result into one part", () => {
    const message = toUiMessage({
      id: "m-1",
      role: "assistant",
      author: { type: "agent", id: "atlas" },
      parts: [
        { type: "text", text: "done" },
        call("c-1", "Read", { file_path: "README.md" }),
        result("c-1", "Read", { data: "# Title" }),
      ],
      createdAt: "2026-09-01T10:00:00Z",
    }) as { parts: Array<Record<string, unknown>> };

    const tools = message.parts.filter((part) => String(part["type"]).startsWith("tool-"));
    expect(tools).toHaveLength(1);
    expect(tools[0]["type"]).toBe("tool-Read");
    expect(tools[0]["state"]).toBe("output-available");
    expect(tools[0]["input"]).toEqual({ file_path: "README.md" });
    expect(tools[0]["output"]).toEqual({ data: "# Title" });
  });

  // The state that makes "Running…" reachable at all.
  it("marks a call with no result yet as running", () => {
    const message = toUiMessage({
      id: "m-1",
      role: "assistant",
      parts: [call("c-1", "Bash", { command: "go", args: ["test"] })],
    }) as { parts: Array<Record<string, unknown>> };

    expect(message.parts[0]["state"]).toBe("input-available");
    expect(message.parts[0]).not.toHaveProperty("output");
  });

  it("marks a failed call as failed, and carries its message", () => {
    const message = toUiMessage({
      id: "m-1",
      role: "assistant",
      parts: [
        call("c-1", "Bash", { command: "rm" }),
        result("c-1", "Bash", {
          code: "AOS_SANDBOX_EXEC_NOT_ALLOWED",
          message: '"rm" is not in this agent\'s allowlist',
        }),
      ],
    }) as { parts: Array<Record<string, unknown>> };

    expect(message.parts[0]["state"]).toBe("output-error");
    expect(message.parts[0]["errorText"]).toBe('"rm" is not in this agent\'s allowlist');
  });

  it("marks a denied call as failed", () => {
    const message = toUiMessage({
      id: "m-1",
      role: "assistant",
      parts: [
        call("c-1", "Write", {}),
        result("c-1", "Write", { denied: true, reason: "a person said no" }),
      ],
    }) as { parts: Array<Record<string, unknown>> };

    expect(message.parts[0]["state"]).toBe("output-error");
    expect(message.parts[0]["errorText"]).toBe("a person said no");
  });

  it("keeps text, reasoning and tool parts in the order they happened", () => {
    const message = toUiMessage({
      id: "m-1",
      role: "assistant",
      parts: [
        { type: "text", text: "answer" },
        { type: "reasoning", text: "first I looked" },
        call("c-1", "Read"),
        result("c-1", "Read", { data: "x" }),
        call("c-2", "Grep"),
        result("c-2", "Grep", { data: [] }),
      ],
    }) as { parts: Array<Record<string, unknown>> };

    expect(message.parts.map((part) => part["type"])).toEqual([
      "text",
      "reasoning",
      "tool-Read",
      "tool-Grep",
    ]);
  });

  // A transcript truncated between a call and its result should still show
  // the work that was done, not hide it.
  it("renders a result whose call is missing rather than dropping it", () => {
    const message = toUiMessage({
      id: "m-1",
      role: "assistant",
      parts: [result("c-9", "Read", { data: "x" })],
    }) as { parts: Array<Record<string, unknown>> };

    expect(message.parts).toHaveLength(1);
    expect(message.parts[0]["type"]).toBe("tool-Read");
    expect(message.parts[0]["state"]).toBe("output-available");
  });
});

describe("execution metadata", () => {
  // Go records the attempt on the message that asked. The interface reads it
  // off the answer, found nothing, and showed "Worked for 0s" on turns that
  // took minutes.
  it("resolves an answer's run through the message it replies to", () => {
    const chat = toUiChat({
      id: "c-1",
      messages: [
        {
          id: "u-1",
          role: "user",
          parts: [{ type: "text", text: "what time is it?" }],
          createdAt: "2026-09-01T10:00:00Z",
          runs: [
            {
              agentId: "atlas",
              jobId: "j-1",
              status: "completed",
              startedAt: "2026-09-01T10:00:01Z",
              completedAt: "2026-09-01T10:02:30Z",
            },
          ],
        },
        {
          id: "a-1",
          role: "assistant",
          author: { type: "agent", id: "atlas" },
          replyTo: "u-1",
          parts: [{ type: "text", text: "half past ten" }],
          createdAt: "2026-09-01T10:00:01Z",
        },
      ],
    }) as { messages: Array<{ metadata: Record<string, any> }> };

    const answer = chat.messages[1];
    expect(answer.metadata.type).toBe("agent");
    expect(answer.metadata.execution).toBeDefined();
    expect(answer.metadata.execution.status).toBe("completed");
    expect(answer.metadata.execution.agentId).toBe("atlas");
    expect(answer.metadata.execution.sourceMessageId).toBe("u-1");
    expect(answer.metadata.execution.startedAt).toBe("2026-09-01T10:00:01Z");
    // The elapsed header reads this; it was the message's own createdAt, so
    // every turn measured zero.
    expect(answer.metadata.updatedAt).toBe("2026-09-01T10:02:30Z");
  });

  it("reports a failed run as failed", () => {
    const chat = toUiChat({
      messages: [
        {
          id: "u-1",
          role: "user",
          runs: [
            {
              agentId: "atlas",
              status: "error",
              startedAt: "2026-09-01T10:00:01Z",
              completedAt: "2026-09-01T10:00:02Z",
              error: { code: "AOS_AGENT_PROVIDER_FAILED", message: "no answer" },
            },
          ],
        },
        { id: "a-1", role: "assistant", replyTo: "u-1", parts: [] },
      ],
    }) as { messages: Array<{ metadata: Record<string, any> }> };

    expect(chat.messages[1].metadata.execution.status).toBe("error");
    expect(chat.messages[1].metadata.execution.error.code).toBe("AOS_AGENT_PROVIDER_FAILED");
  });

  it("keeps a user message's own dispatches on it", () => {
    const chat = toUiChat({
      messages: [
        { id: "u-1", role: "user", runs: [{ agentId: "atlas", status: "running", startedAt: "x" }] },
      ],
    }) as { messages: Array<{ metadata: Record<string, any> }> };

    expect(chat.messages[0].metadata.type).toBe("user");
    expect(chat.messages[0].metadata.runs).toHaveLength(1);
  });

  // A snapshot published while the turn is still being written has no run
  // anywhere yet — the run is recorded when the answer is stored.
  it("states a live snapshot as running", () => {
    const live = toLiveUiMessage({
      id: "a-1",
      role: "assistant",
      author: { type: "agent", id: "atlas" },
      parts: [{ type: "text", text: "thinking about" }],
      createdAt: "2026-09-01T10:00:01Z",
    }) as { metadata: Record<string, any> };

    expect(live.metadata.type).toBe("agent");
    expect(live.metadata.execution.status).toBe("running");
    expect(live.metadata.execution.agentId).toBe("atlas");
    expect(live.metadata.createdAt).toBe("2026-09-01T10:00:01Z");
  });
});

describe("isToolError", () => {
  it("recognises both shapes the runtime produces", () => {
    expect(isToolError({ error: "denied" })).toBe(true);
    expect(isToolError({ code: "AOS_X", message: "why" })).toBe(true);
    expect(isToolError({ denied: true, reason: "no" })).toBe(true);
  });

  it("does not call a successful result an error", () => {
    expect(isToolError({ data: "content" })).toBe(false);
    expect(isToolError({ data: { error: 0 } })).toBe(false);
    expect(isToolError(null)).toBe(false);
    expect(isToolError("text")).toBe(false);
  });
});

describe("passthrough", () => {
  it("leaves a value that is not a message alone", () => {
    expect(toUiMessage(null)).toBeNull();
    expect(toUiMessage("text")).toBe("text");
    expect(toUiChat({ id: "c-1" })).toEqual({ id: "c-1" });
  });
});
