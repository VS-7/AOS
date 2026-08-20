import { type FileUIPart, type SourceDocumentUIPart } from "ai";
import type { WorkspaceFile } from "@/features/file/interfaces/file.interfaces";
import type { Agent } from "@/features/agent/interfaces/agent.interfaces";
import type { Instruction } from "@/features/instruction/interfaces/instruction.interfaces";
import type { Skill } from "@/features/skill/interfaces/skill.interfaces";
import type {
  Chat,
  ChatMessage,
} from "@/features/chat/interfaces/chat.interfaces";
import type { WorkspaceDirectoryUser } from "@/features/workspace/interfaces/directory.interfaces";
import type {
  ActiveMention,
  ActiveSkillTrigger,
  ComposerAttachedFile,
  ComposerMentionTarget,
  ComposerReference,
  ComposerReferenceKind,
  ChatComposerSubmitMessage,
} from "../components/composer/composer.types";
import { ChatInlineMarkupHelper } from "./chat-inline-markup.helper";

export const COMPOSER_PROMPT_PART_PREFIX = "[system-reminder]:";

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
    const mentionStart = match.index + match[1].length;

    return {
      query,
      range: {
        start: mentionStart,
        end: caret,
      },
    };
  }

  /**
   * Detects an active "/" slash trigger in the composer.
   *
   * Mirrors {@link getActiveMention} for the `@` mention flow: a leading `/`
   * (either at the start of input or after whitespace) opens the skills menu
   * so the user can search for and inject a skill reference. The trigger is
   * inactive once the user types whitespace, allowing them to type literal
   * paths (e.g. "src/foo") without invoking the menu.
   *
   * Inline markup tokens (e.g. `<source type="skill" name="…">`) are ignored
   * when scanning for the trigger so they cannot accidentally reactivate the
   * menu when the user navigates past an inserted skill badge.
   *
   * @param value - Serialized composer value (plain text + inline markup).
   * @param caret - Current caret offset inside the serialized value.
   * @returns The active skill trigger, or `null` when no `/` is in scope.
   */
  public static getActiveSkillTrigger(
    value: string,
    caret: number,
  ): ActiveSkillTrigger | null {
    const tail = ComposerHelper.getTextTailBeforeCaret(value, caret);
    const match = tail.match(/(^|\s)\/([a-zA-Z0-9_-]*)$/);

    if (!match || match.index === undefined) {
      return null;
    }

    const query = match[2] ?? "";
    const triggerStart = match.index + match[1].length;

    return {
      query,
      range: {
        start: triggerStart,
        end: caret,
      },
    };
  }

  /**
   * Returns the substring of text before the caret, ignoring inline markup
   * tokens (`<source …>`, `<mention …>`). Markup tags are preserved as zero
   * width so the returned string can be re-aligned with the original offset
   * when computing trigger positions.
   */
  private static getTextTailBeforeCaret(value: string, caret: number) {
    const tokens = ChatInlineMarkupHelper.parse(value);
    let cursor = 0;
    let tail = "";

    for (const token of tokens) {
      if (token.type === "text") {
        const textLength = token.value.length;
        const segmentStart = cursor;
        const segmentEnd = cursor + textLength;

        if (caret > segmentStart) {
          const segmentCaret = Math.min(caret, segmentEnd);
          tail += token.value.slice(0, segmentCaret - segmentStart);
        }

        cursor = segmentEnd;
        continue;
      }

      const markupLength = token.markup.length;
      const segmentStart = cursor;
      const segmentEnd = cursor + markupLength;

      if (caret <= segmentStart) {
        break;
      }

      if (caret < segmentEnd) {
        tail += " ";
      }

      cursor = segmentEnd;

      if (caret <= segmentEnd) {
        break;
      }
    }

    return tail;
  }

  public static toReferenceFromFile(file: WorkspaceFile): ComposerReference {
    const isDirectory = file.type === "directory";

    return {
      id: `${file.type}:${file.absolutePath}`,
      kind: isDirectory ? "folder" : "file",
      label: file.name,
      prompt: isDirectory
        ? `${COMPOSER_PROMPT_PART_PREFIX} Workspace attachment reference: Inspect the folder at "${file.absolutePath}" as context for this request.`
        : `${COMPOSER_PROMPT_PART_PREFIX} Workspace attachment reference: Use the file at "${file.absolutePath}" as context for this request.`,
      searchableValue: `${file.name} ${file.path} ${file.absolutePath} ${file.type}`,
      targetPath: file.absolutePath,
    };
  }

  public static toReferenceFromSkill(skill: Skill): ComposerReference {
    return {
      id: `skill:${skill.id}`,
      kind: "skill",
      label: skill.name,
      prompt: `${COMPOSER_PROMPT_PART_PREFIX} Skill activation request: Activate the AOS skill "${skill.id}" for this request.`,
      searchableValue: `${skill.id} ${skill.name} ${skill.description}`,
      tagName: skill.id,
    };
  }

  public static toReferenceFromInstruction(
    instruction: Instruction,
  ): ComposerReference {
    return {
      id: `instruction:${instruction.id}`,
      kind: "instruction",
      label: instruction.name,
      prompt: `${COMPOSER_PROMPT_PART_PREFIX} Workspace instruction attached: Apply the instruction "${instruction.name}" (id: "${instruction.id}") for this request.`,
      searchableValue: `${instruction.id} ${instruction.name} ${instruction.description ?? ""} ${(instruction.paths ?? []).join(" ")}`,
    };
  }

  public static toSourceDocument(
    reference: ComposerReference,
  ): SourceDocumentUIPart {
    return {
      type: "source-document",
      sourceId: reference.id,
      mediaType: ComposerHelper.getReferenceMediaType(reference.kind),
      title: reference.label,
      filename: reference.targetPath,
    };
  }

  public static formatDuration(durationMs: number) {
    const totalSeconds = Math.max(1, Math.round(durationMs / 1000));
    const minutes = Math.floor(totalSeconds / 60);
    const seconds = totalSeconds % 60;

    return `${minutes}:${seconds.toString().padStart(2, "0")}`;
  }

  public static isWorkspaceWideInstruction(instruction: Instruction) {
    const paths = instruction.paths ?? [];

    if (paths.length === 0) {
      return true;
    }

    return paths.some((pattern) => {
      const normalized = pattern.trim();
      return normalized === "**" || normalized === "**/*" || normalized === "*";
    });
  }

  public static getInstructionMatchesForPath(
    instructions: Instruction[],
    path: string,
  ) {
    const normalizedPath = ComposerHelper.normalizePath(path);

    return instructions.filter((instruction) => {
      const paths = instruction.paths ?? [];

      if (paths.length === 0) {
        return false;
      }

      return paths.some((pattern) =>
        ComposerHelper.matchesInstructionGlob(pattern, normalizedPath),
      );
    });
  }

  public static buildMessageParts(params: {
    autoAttachedInstructionReferences: ComposerReference[];
    submitMessage: ChatComposerSubmitMessage;
  }) {
    const parts: ChatMessage["parts"] = [];
    const mainText = params.submitMessage.text.trim();

    params.autoAttachedInstructionReferences.forEach((reference) => {
      parts.push({
        type: "text",
        text: reference.prompt,
      });
    });

    params.submitMessage.files.forEach((file) => {
      parts.push(ComposerHelper.toFilePart(file));
    });

    if (mainText) {
      parts.push({
        type: "text",
        text: mainText,
      });
    }

    return parts;
  }

  private static normalizePath(value: string) {
    return value.replace(/\\/g, "/");
  }

  private static matchesInstructionGlob(pattern: string, filePath: string) {
    const regexPattern = pattern
      .replace(/\./g, "\\.")
      .replace(/\*\*/g, "(.+)")
      .replace(/\*/g, "([^/]+)")
      .replace(/\?/g, "(.)");

    return new RegExp(`^${regexPattern}$`).test(filePath);
  }

  private static getReferenceMediaType(kind: ComposerReferenceKind) {
    switch (kind) {
      case "file":
      case "folder":
        return "application/vnd.aos.workspace-reference";
      case "skill":
        return "application/vnd.aos.skill-reference";
      case "instruction":
        return "application/vnd.aos.instruction-reference";
    }
  }

  private static toFilePart(
    // `ComposerAttachedFile` (what the composer actually collects, no
    // `.type`), not the AI SDK's `FileUIPart` this signature originally
    // declared — the function body only reads `filename`/`mediaType`/`url`.
    file: ComposerAttachedFile,
  ): ChatMessage["parts"][number] {
    return {
      type: "file",
      filename: file.filename,
      mediaType: file.mediaType || "application/octet-stream",
      url: file.url,
    };
  }

  public static buildInlineMentionTag(agentId: string) {
    return ChatInlineMarkupHelper.buildMentionTag({ id: agentId });
  }

  public static buildInlineSourceTag(reference: ComposerReference) {
    return ChatInlineMarkupHelper.buildSourceTag(reference);
  }

  /**
   * Whether this chat is a private user↔agent DM (mentions of other agents
   * are forbidden — talk in a shared channel instead).
   */
  public static isAgentDirectMessage(
    chat: Pick<Chat, "id" | "kind" | "participants">,
    agents?: Array<Pick<Agent, "id">>,
  ): boolean {
    const participants = chat.participants ?? [];

    if (participants.some((participant) => participant.type === "agent")) {
      return chat.kind === "dm" || !chat.kind;
    }

    // Legacy slug-as-id agent DMs (pre-participants migration).
    if (agents?.some((agent) => agent.id === chat.id)) {
      return true;
    }

    return false;
  }

  /**
   * Builds mention candidates for the composer: chat members (agents + users).
   *
   * - Workspace / channel chats → full roster (agents + directory people).
   *   Agents are rarely listed on channel `participants`, so participant ACL
   *   alone would hide the whole Agents group.
   * - Private chats with participants → those rows only
   * - User↔user DMs also offer workspace agents (wake via @mention)
   * - User↔agent DMs → empty (product rule: no nested agent mentions)
   * - Always excludes the ambient self user
   */
  public static resolveMentionTargets(params: {
    agents: Agent[];
    chat: Pick<Chat, "id" | "kind" | "visibility" | "participants">;
    directoryUsers: WorkspaceDirectoryUser[];
    selfUserId?: string;
    query?: string;
  }): ComposerMentionTarget[] {
    const { agents, chat, directoryUsers, selfUserId } = params;
    const query = (params.query ?? "").trim().toLowerCase();
    const participants = chat.participants ?? [];

    if (ComposerHelper.isAgentDirectMessage(chat, agents)) {
      return [];
    }

    const agentsById = new Map(agents.map((agent) => [agent.id, agent]));
    const usersById = new Map(directoryUsers.map((user) => [user.id, user]));

    const isUserDm =
      chat.kind === "dm" &&
      !participants.some((participant) => participant.type === "agent");

    // Shared workspace surfaces always expose the full team roster — even when
    // `participants` only lists humans (or is empty on legacy channels).
    const isSharedWorkspaceChat =
      chat.visibility === "workspace" ||
      chat.kind === "channel" ||
      participants.length === 0;

    let agentIds: string[] = [];
    let userIds: string[] = [];

    if (isSharedWorkspaceChat || isUserDm) {
      agentIds = agents.map((agent) => agent.id);
      userIds = isUserDm
        ? participants
            .filter((participant) => participant.type === "user")
            .map((participant) => participant.id)
        : directoryUsers.map((user) => user.id);
    } else {
      agentIds = participants
        .filter((participant) => participant.type === "agent")
        .map((participant) => participant.id);
      userIds = participants
        .filter((participant) => participant.type === "user")
        .map((participant) => participant.id);
    }

    if (selfUserId) {
      userIds = userIds.filter((id) => id !== selfUserId);
    }

    const targets: ComposerMentionTarget[] = [];

    for (const agentId of [...new Set(agentIds)]) {
      const agent = agentsById.get(agentId);
      if (!agent) {
        continue;
      }
      targets.push({
        kind: "agent",
        key: `agent:${agent.id}`,
        mentionId: agent.id,
        label: agent.name || agent.id,
        image: agent.image,
      });
    }

    for (const userId of [...new Set(userIds)]) {
      const profile = usersById.get(userId);
      const username = profile?.username?.trim();
      if (!username) {
        // Backend resolves humans by username — skip unresolved peers.
        continue;
      }
      targets.push({
        kind: "user",
        key: `user:${userId}`,
        mentionId: username,
        label: profile?.name?.trim() || username,
        image: profile?.image,
      });
    }

    const filtered = targets.filter((target) => {
      if (!query) {
        return true;
      }
      return (
        target.mentionId.toLowerCase().includes(query) ||
        target.label.toLowerCase().includes(query)
      );
    });

    return filtered
      .sort((left, right) => {
        if (left.kind !== right.kind) {
          return left.kind === "agent" ? -1 : 1;
        }
        return left.label.localeCompare(right.label);
      })
      .slice(0, 12);
  }
}
