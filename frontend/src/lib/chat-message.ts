/**
 * One stored message, in the shape the ported chat components read.
 *
 * This is the third translation point, alongside `command-map.ts` for calls
 * and `realtime-event-map.ts` for event names, and it exists because a
 * message is the one payload that arrives through *both* of them: as part of
 * a chat (`chats_get`, `chats_create`, `chats_update`), as the echo of a send
 * (`chats_send`), and as a live snapshot on the realtime channel
 * (`chat.message`). It used to be applied only to the first, so the live
 * answer and the confirmed echo were rendered from a shape nothing understood.
 *
 * Two things are translated here.
 *
 * **Tool calls.** Go stores a turn's tool use as two flat parts — a
 * `tool-call` and a `tool-result` sharing a `toolCallId`
 * (`internal/domain/chat.Part`, a documented divergence from the original's
 * AI-SDK message). The ported UI reads the AI-SDK shape: one part per call,
 * named `tool-<name>`, carrying `input`, `output` and a `state` that moves
 * from `input-available` to `output-available`. Reading Go's two parts
 * through that lens made every tool appear *twice* in the thinking timeline
 * and in the header counters ("2 reads" for one read), always as `complete`
 * — a call still running looked finished, and a call that failed or was
 * denied looked successful, because `state` was `undefined` and `undefined`
 * reads as complete. The pairing happens here.
 *
 * **Execution metadata.** Go records the attempt on the message that *asked*
 * (`Message.Runs`), which is the granularity at which somebody asks why an
 * answer was expensive. The UI reads it off the *answer*. So every finished
 * answer showed "Worked for 0s". The answer now names the message it replies
 * to (`replyTo`), and the run is resolved through it.
 */

/** The lifecycle of one tool call, in the AI SDK's vocabulary. */
export type ToolPartState =
  | "input-streaming"
  | "input-available"
  | "output-available"
  | "output-error";

interface RawPart {
  type?: string;
  text?: string;
  toolName?: string;
  toolCallId?: string;
  input?: unknown;
  output?: unknown;
  mediaType?: string;
  uri?: string;
}

interface RawRun {
  agentId?: string;
  jobId?: string;
  status?: string;
  startedAt?: string;
  completedAt?: string;
  usage?: unknown;
  error?: { code?: string; message?: string };
}

interface RawMessage {
  id?: string;
  role?: string;
  author?: { type?: string; id?: string };
  parts?: RawPart[];
  replyTo?: string;
  runs?: RawRun[];
  reactions?: unknown[];
  createdAt?: string;
  metadata?: Record<string, unknown>;
}

/**
 * Whether a tool's output is the runtime's error encoding.
 *
 * Two shapes reach here. `{error: "..."}` is what the loop writes for a
 * denied call; a failed one carries the whole `apperr` envelope, which is an
 * object with a `code` and a `message`. `components/ui/tool.tsx` only knew
 * the first, so a tool that failed rendered as one that succeeded.
 */
export function isToolError(output: unknown): boolean {
  if (!output || typeof output !== "object" || Array.isArray(output)) return false;
  const record = output as Record<string, unknown>;
  if (typeof record["error"] === "string") return true;
  if (record["denied"] === true) return true;
  return typeof record["code"] === "string" && typeof record["message"] === "string";
}

/** The message the error text should read, for a failed call. */
function errorTextOf(output: unknown): string | undefined {
  if (!output || typeof output !== "object") return undefined;
  const record = output as Record<string, unknown>;
  if (typeof record["error"] === "string") return record["error"];
  if (typeof record["message"] === "string") return record["message"];
  if (record["denied"] === true && typeof record["reason"] === "string") return record["reason"];
  return undefined;
}

/**
 * Folds Go's flat parts into the parts the components read.
 *
 * Order is preserved by walking once: text, reasoning and file parts pass
 * through where they are, a `tool-call` becomes the single tool part at the
 * position it was called, and the matching `tool-result` fills that same part
 * in rather than adding another. A result whose call is missing — which
 * should not happen, but a truncated transcript could produce it — still
 * renders, as a completed call with no input, because dropping it would hide
 * work that was done.
 */
function foldParts(parts: RawPart[]): unknown[] {
  const out: unknown[] = [];
  const toolIndex = new Map<string, number>();

  for (const part of parts) {
    if (part.type === "tool-call") {
      const key = part.toolCallId ?? `${part.toolName}:${out.length}`;
      toolIndex.set(key, out.length);
      out.push({
        type: `tool-${part.toolName ?? "unknown"}`,
        toolCallId: part.toolCallId,
        toolName: part.toolName,
        input: part.input,
        // No result yet: the call is in flight. This is the state that made
        // "Running…" reachable at all — every tool used to read as complete.
        state: "input-available" as ToolPartState,
      });
      continue;
    }

    if (part.type === "tool-result") {
      const key = part.toolCallId ?? "";
      const at = toolIndex.get(key);
      const failed = isToolError(part.output);
      if (at === undefined) {
        out.push({
          type: `tool-${part.toolName ?? "unknown"}`,
          toolCallId: part.toolCallId,
          toolName: part.toolName,
          output: part.output,
          state: (failed ? "output-error" : "output-available") as ToolPartState,
          ...(failed ? { errorText: errorTextOf(part.output) } : {}),
        });
        continue;
      }
      const call = out[at] as Record<string, unknown>;
      call["output"] = part.output;
      call["state"] = failed ? "output-error" : "output-available";
      if (failed) call["errorText"] = errorTextOf(part.output);
      continue;
    }

    out.push(part);
  }

  return out;
}

/** The execution block, from the run that produced this answer. */
function executionOf(run: RawRun | undefined, sourceMessageId: string | undefined) {
  if (!run) return undefined;
  return {
    agentId: run.agentId,
    jobId: run.jobId,
    sourceMessageId,
    // Go's Status has five values; this side's execution status has two, and
    // anything that has not completed is still running as far as the header
    // is concerned.
    status: run.completedAt ? (run.error ? "error" : "completed") : "running",
    startedAt: run.startedAt,
    completedAt: run.completedAt,
    usage: run.usage,
    error: run.error,
  };
}

/** How a message was obtained, which is what its execution status can say. */
export interface ToUiMessageOptions {
  /**
   * The runs of the message this one answers, looked up by `replyTo`.
   *
   * Go keeps the attempt on the asking message, so an answer on its own
   * cannot say how long it took — which is why every finished turn read
   * "Worked for 0s".
   */
  sourceRuns?: RawRun[];
  /** The id of that message, which the thinking header reads back. */
  sourceMessageId?: string;
  /**
   * True for a snapshot published while the turn is still being written. It
   * has no run anywhere yet — the run is recorded when the answer is stored —
   * so the status is stated rather than derived.
   */
  live?: boolean;
}

/** One message, translated. */
export function toUiMessage(raw: unknown, options: ToUiMessageOptions = {}): unknown {
  if (raw === null || typeof raw !== "object") return raw;
  const message = raw as RawMessage;

  const role = String(message.role ?? "");
  const ownRuns = Array.isArray(message.runs) ? message.runs : [];
  const sourceRuns = options.sourceRuns ?? [];
  const sourceMessageId = options.sourceMessageId ?? message.replyTo;

  // The attempt that produced *this* message: the newest run of the message
  // it answers. Falling back to this message's own runs keeps a user message
  // — where Go actually records them — describing its own dispatches.
  const attempts = sourceRuns.length > 0 ? sourceRuns : ownRuns;
  const latest = attempts.length > 0 ? attempts[attempts.length - 1] : undefined;

  const execution = options.live
    ? {
        agentId: message.author?.id,
        sourceMessageId,
        status: "running" as const,
        startedAt: message.createdAt,
      }
    : executionOf(latest, sourceMessageId);

  return {
    ...message,
    parts: foldParts(Array.isArray(message.parts) ? message.parts : []),
    metadata: {
      ...(message.metadata ?? {}),
      type: role === "assistant" ? "agent" : role === "system" ? "system" : "user",
      data: { id: String(message.author?.id ?? "") },
      createdAt: message.createdAt,
      // Go has no per-message updatedAt; the newest attempt's end is the
      // closest true answer, and the elapsed calculation falls back past it
      // when there is none.
      updatedAt: latest?.completedAt ?? message.createdAt,
      runs: ownRuns,
      reactions: message.reactions,
      execution,
    },
  };
}

/**
 * Every message of one chat, translated, with each answer's run resolved
 * through the message it replies to.
 */
export function toUiMessages(raw: unknown): unknown {
  if (!Array.isArray(raw)) return raw;
  const messages = raw as RawMessage[];

  const runsByID = new Map<string, RawRun[]>();
  for (const message of messages) {
    if (typeof message?.id === "string" && Array.isArray(message.runs)) {
      runsByID.set(message.id, message.runs);
    }
  }

  return messages.map((message) => {
    if (message === null || typeof message !== "object") return message;
    const replyTo = typeof message.replyTo === "string" ? message.replyTo : undefined;
    return toUiMessage(message, {
      sourceRuns: replyTo ? runsByID.get(replyTo) : undefined,
      sourceMessageId: replyTo,
    });
  });
}

/** A whole chat, with its messages translated. */
export function toUiChat(raw: unknown): unknown {
  if (raw === null || typeof raw !== "object") return raw;
  const chat = raw as Record<string, unknown>;
  if (!Array.isArray(chat["messages"])) return chat;
  return { ...chat, messages: toUiMessages(chat["messages"]) };
}

/**
 * A snapshot of an answer still being written.
 *
 * `chat.message` carries the same message shape the transcript will store,
 * under the same id, but no run exists for it yet — so its status is stated
 * as running rather than derived from an attempt that has not been recorded.
 */
export function toLiveUiMessage(raw: unknown): unknown {
  return toUiMessage(raw, { live: true });
}
