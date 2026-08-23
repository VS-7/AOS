/**
 * Absent from the reconstructed source (_extracted/v401/web/src/ has no
 * composer.types.ts, though chat-inline-markup.helper.ts and
 * composer.helper.ts both import from it) — reconstructed from how those two
 * files actually use each type. Same situation as
 * data-table-toolbar-component.tsx and resizable.tsx: a small, faithful
 * reconstruction from usage evidence, not a new design.
 */

import type { Agent } from "@/features/agent/interfaces/agent.interfaces";
import type { Chat } from "@/features/chat/interfaces/chat.interfaces";

/**
 * What `<ChatComposer>` needs from its page. The original also threaded a
 * `userId` through here to resolve message authorship client-side; AOS's
 * chats_send derives the author from the authenticated caller server-side
 * (see internal/domain/chat/service.go's Send), so there is nothing for a
 * client-supplied id to do — kept optional and unused rather than dropped,
 * since `presentation/components/composer/hooks/use-chat-composer.ts`
 * (Task 9's pristine copy) destructures it from this same props type.
 *
 * `onSent`/`onConfirmed`/`onFailed` are the local-echo lifecycle: `onSent`
 * fires the instant a message is submitted (with a client-only id, before
 * the server has seen it) so the sender's own message shows up without
 * waiting on a refetch; `onConfirmed` fires when `chats_send` resolves,
 * swapping that local echo for the server-persisted message (Go always
 * assigns its own id — see `Service.Send` — so this is a replace, not an
 * update in place); `onFailed` fires on mutation error so the unsent echo
 * doesn't linger looking delivered. `ChatContent` wires all three to the
 * matching `useChat` actions.
 */
export interface ChatComposerProps {
  agents: Agent[];
  chat: Chat;
  isDirectMessage?: boolean;
  onSent?: (message: import("@/features/chat/interfaces/chat.interfaces").ChatMessage) => void;
  onConfirmed?: (
    previousId: string,
    message: import("@/features/chat/interfaces/chat.interfaces").ChatMessage,
  ) => void;
  onFailed?: (messageId?: string) => void;
  userId?: string;
}

export type ComposerReferenceKind = "file" | "folder" | "skill" | "instruction";

/** An attachment inserted into the composer: `@mention` a file, folder, skill, or instruction. */
export interface ComposerReference {
  id: string;
  kind: ComposerReferenceKind;
  label: string;
  /** The `[system-reminder]:`-prefixed text sent to the agent alongside the message. */
  prompt: string;
  searchableValue: string;
  /** Filesystem path, for file/folder references. */
  targetPath?: string;
  /** Slug used in the rendered `<source type="skill" name="...">` tag. */
  tagName?: string;
}

/** The `@` trigger's current match: what's typed so far, and where to replace it. */
export interface ActiveMention {
  query: string;
  range: { start: number; end: number };
}

/** The `/` trigger's current match — same shape as {@link ActiveMention}, different key. */
export type ActiveSkillTrigger = ActiveMention;

export interface ComposerMentionTarget {
  kind: "agent" | "user";
  key: string;
  mentionId: string;
  label: string;
  image?: string;
}

/** A local file attached to the composer, kept separate from inline references. */
export interface ComposerAttachedFile {
  filename?: string;
  mediaType: string;
  url: string;
}

/**
 * A recorded-but-not-yet-sent voice note, held in the composer while the
 * user reviews/confirms it. Shape read off `hooks/use-chat-composer-
 * audio.ts`'s own `setPendingAudioClip` calls, this type's origin.
 */
export interface PendingAudioClip {
  blob: Blob;
  durationMs: number;
  filename: string;
  url: string;
}

/** What the composer hands to the send mutation. */
export interface ChatComposerSubmitMessage {
  text: string;
  files: ComposerAttachedFile[];
}
