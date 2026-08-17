import type { Agent } from "@/features/agent/interfaces/agent.interfaces";
import type { Chat } from "@/features/chat/interfaces/chat.interfaces";
import type {
  ActiveMention,
  ComposerMentionTarget,
} from "../components/composer/composer.types";

/**
 * Trimmed from the original: that version also resolved file, folder,
 * skill, and instruction references, and a workspace directory of human
 * users. AOS has no file/skill/instruction command groups and no multi-user
 * directory (see ChatThreadHelper), so the composer this now backs only
 * offers what's real — typed text and `@agent` mentions.
 */
export class ComposerHelper {
  public static getActiveMention(
    value: string,
    caret: number,
  ): ActiveMention | null {
    const beforeCaret = value.slice(0, caret);
    const match = beforeCaret.match(/(^|\s)@([a-zA-Z0-9_-]*)$/);

    if (!match || match.index === undefined) {
      return null;
    }

    const query = match[2] ?? "";
    const mentionStart = match.index + (match[1]?.length ?? 0);

    return {
      query,
      range: {
        start: mentionStart,
        end: caret,
      },
    };
  }

  /**
   * Whether this is a private user<->agent DM — the one agent in it is
   * already who the person is talking to, so offering to @mention it too
   * would be product noise, not a real choice.
   */
  public static isAgentDirectMessage(
    chat: Pick<Chat, "kind" | "participants">,
  ): boolean {
    const participants = chat.participants ?? [];
    return (
      chat.kind === "dm" &&
      participants.some((participant) => participant.type === "agent")
    );
  }

  /**
   * Mention candidates for the composer: workspace agents, filtered by
   * whatever's typed after `@`. Unlike the original there is no "People"
   * group — AOS is single-operator, with no directory of other humans to
   * mention.
   */
  public static resolveMentionTargets(params: {
    agents: Agent[];
    chat: Pick<Chat, "kind" | "participants">;
    query?: string;
  }): ComposerMentionTarget[] {
    const { agents, chat } = params;
    const query = (params.query ?? "").trim().toLowerCase();

    if (ComposerHelper.isAgentDirectMessage(chat)) {
      return [];
    }

    const targets: ComposerMentionTarget[] = agents.map((agent) => ({
      kind: "agent",
      key: `agent:${agent.id}`,
      mentionId: agent.id,
      label: agent.name || agent.id,
      image: agent.image,
    }));

    return targets
      .filter(
        (target) =>
          !query ||
          target.mentionId.toLowerCase().includes(query) ||
          target.label.toLowerCase().includes(query),
      )
      .sort((left, right) => left.label.localeCompare(right.label))
      .slice(0, 12);
  }
}
