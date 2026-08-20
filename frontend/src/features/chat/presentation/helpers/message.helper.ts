import { type FileUIPart, type UIMessage, isTextUIPart } from "ai";
import { COMPOSER_PROMPT_PART_PREFIX } from "./composer.helper";
import { ChatInlineMarkupHelper } from "./chat-inline-markup.helper";

export class MessageHelper {
  public static getMessageTextParts(message: UIMessage): string[] {
    if (!message.parts?.length) return [];

    return message.parts
      .filter(isTextUIPart)
      .filter((part) => !part.text.trimStart().startsWith(COMPOSER_PROMPT_PART_PREFIX))
      .map((part) => part.text);
  }

  public static getMessageText(message: UIMessage): string {
    return MessageHelper.getMessageTextParts(message).join("");
  }

  public static getMentionIds(message: UIMessage): string[] {
    return ChatInlineMarkupHelper.getMentionIds(MessageHelper.getMessageText(message))
  }

  public static getMessageFiles(message: UIMessage): FileUIPart[] {
    if (!message.parts?.length) return []

    return message.parts.filter((part): part is FileUIPart => part.type === "file")
  }
}
