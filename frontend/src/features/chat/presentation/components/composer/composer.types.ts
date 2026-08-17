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
 * client-supplied id to do.
 */
export interface ChatComposerProps {
  agents: Agent[];
  chat: Chat;
  isDirectMessage?: boolean;
  onSent?: () => void;
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

/** What the composer hands to the send mutation. */
export interface ChatComposerSubmitMessage {
  text: string;
  files: ComposerAttachedFile[];
}
