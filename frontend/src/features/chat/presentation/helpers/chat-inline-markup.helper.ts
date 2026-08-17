import type { Agent } from "@/features/agent/interfaces/agent.interfaces"
import type { ComposerReference } from "../components/composer/composer.types"

export type ChatInlineSourceType = "file" | "folder" | "skill"

export interface ChatInlineTextToken {
  type: "text"
  value: string
}

export interface ChatInlineSourceToken {
  type: "source"
  sourceType: ChatInlineSourceType
  name: string
  path: string
  markup: string
}

export interface ChatInlineMentionToken {
  type: "mention"
  id: string
  markup: string
}

export type ChatInlineMarkupToken =
  | ChatInlineTextToken
  | ChatInlineSourceToken
  | ChatInlineMentionToken

export class ChatInlineMarkupHelper {
  private static readonly INLINE_TAG_PATTERN = /<source\b[^>]*>|<mention\b[^>]*>/g
  private static readonly ATTRIBUTE_PATTERN = /([a-zA-Z_:][\w:.-]*)="([^"]*)"/g

  public static buildMentionTag(agent: Agent | { id: string }) {
    return `<mention id="${agent.id}">`
  }

  public static buildSourceTag(reference: ComposerReference) {
    if (reference.kind === "skill") {
      return `<source type="skill" name="${ChatInlineMarkupHelper.escapeAttribute(reference.tagName ?? reference.label)}" path="">`
    }

    return `<source type="${reference.kind}" name="${ChatInlineMarkupHelper.escapeAttribute(reference.label)}" path="${ChatInlineMarkupHelper.escapeAttribute(reference.targetPath ?? "")}">`
  }

  public static parse(value: string): ChatInlineMarkupToken[] {
    if (!value) {
      return [{ type: "text", value: "" }]
    }

    const tokens: ChatInlineMarkupToken[] = []
    let cursor = 0

    for (const match of value.matchAll(ChatInlineMarkupHelper.INLINE_TAG_PATTERN)) {
      const markup = match[0]
      const index = match.index ?? 0

      if (index > cursor) {
        tokens.push({
          type: "text",
          value: value.slice(cursor, index),
        })
      }

      const parsedToken = ChatInlineMarkupHelper.parseMarkup(markup)

      if (parsedToken) {
        tokens.push(parsedToken)
      } else {
        tokens.push({
          type: "text",
          value: markup,
        })
      }

      cursor = index + markup.length
    }

    if (cursor < value.length) {
      tokens.push({
        type: "text",
        value: value.slice(cursor),
      })
    }

    return tokens.length > 0 ? tokens : [{ type: "text", value: "" }]
  }

  public static getSourceTokens(value: string) {
    return ChatInlineMarkupHelper.parse(value).filter(
      (token): token is ChatInlineSourceToken => token.type === "source",
    )
  }

  public static getMentionIds(value: string) {
    const ids = new Set<string>()

    ChatInlineMarkupHelper.parse(value).forEach((token) => {
      if (token.type === "mention" && token.id.trim()) {
        ids.add(token.id.trim())
      }
    })

    return [...ids]
  }

  public static toDisplayMarkdown(value: string) {
    return ChatInlineMarkupHelper.parse(value)
      .map((token) => {
        if (token.type === "text") {
          return token.value
        }

        if (token.type === "mention") {
          const id = token.id.trim()
          return `<chat-mention id="${ChatInlineMarkupHelper.escapeAttribute(id)}">${ChatInlineMarkupHelper.escapeHtml(id)}</chat-mention>`
        }

        return `<chat-source type="${ChatInlineMarkupHelper.escapeAttribute(token.sourceType)}" name="${ChatInlineMarkupHelper.escapeAttribute(token.name)}" path="${ChatInlineMarkupHelper.escapeAttribute(token.path)}">${ChatInlineMarkupHelper.escapeHtml(token.name)}</chat-source>`
      })
      .join("")
  }

  public static insertMarkupAtOffset(value: string, markup: string, offset: number) {
    const boundedOffset = Math.max(0, Math.min(offset, value.length))
    const prefixNeedsSpace = boundedOffset > 0 && !/\s/.test(value[boundedOffset - 1] ?? "")
    const suffixNeedsSpace = boundedOffset < value.length && !/\s/.test(value[boundedOffset] ?? "")
    const insertion = `${prefixNeedsSpace ? " " : ""}${markup}${suffixNeedsSpace ? " " : ""}`
    const nextValue = `${value.slice(0, boundedOffset)}${insertion}${value.slice(boundedOffset)}`
    const nextCaret = boundedOffset + insertion.length

    return {
      nextCaret,
      nextValue,
    }
  }

  public static replaceRangeWithMarkup(
    value: string,
    range: { start: number; end: number },
    markup: string,
  ) {
    const prefix = value.slice(0, range.start)
    const suffix = value.slice(range.end)
    const prefixNeedsSpace = prefix.length > 0 && !/\s/.test(prefix[prefix.length - 1] ?? "")
    const suffixNeedsSpace = suffix.length > 0 && !/^\s/.test(suffix)
    const replacement = `${prefixNeedsSpace ? " " : ""}${markup}${suffixNeedsSpace ? " " : ""}`
    const nextValue = `${prefix}${replacement}${suffix}`
    const nextCaret = prefix.length + replacement.length

    return {
      nextCaret,
      nextValue,
    }
  }

  public static serializeEditableValue(root: HTMLElement) {
    return [...root.childNodes]
      .map((node) => ChatInlineMarkupHelper.serializeNode(node))
      .join("")
  }

  public static getSelectionOffset(root: HTMLElement) {
    if (typeof window === "undefined") {
      return 0
    }

    const selection = window.getSelection()

    if (!selection || selection.rangeCount === 0) {
      return 0
    }

    const range = selection.getRangeAt(0)

    if (range.startContainer === root) {
      return ChatInlineMarkupHelper.getOffsetWithinNode(root, range.startOffset)
    }

    const nodeOffset = ChatInlineMarkupHelper.getOffsetBeforeNode(root, range.startContainer)

    return nodeOffset + ChatInlineMarkupHelper.getOffsetWithinNode(range.startContainer, range.startOffset)
  }

  public static setSelectionOffset(root: HTMLElement, offset: number) {
    if (typeof window === "undefined" || typeof document === "undefined") {
      return
    }

    const boundedOffset = Math.max(0, offset)
    const target = ChatInlineMarkupHelper.resolveSelectionTarget(root, boundedOffset)
    const selection = window.getSelection()

    if (!selection) {
      return
    }

    const range = document.createRange()
    range.setStart(target.node, target.offset)
    range.collapse(true)
    selection.removeAllRanges()
    selection.addRange(range)
  }

  private static parseMarkup(markup: string): ChatInlineMarkupToken | null {
    if (markup.startsWith("<mention")) {
      const attributes = ChatInlineMarkupHelper.parseAttributes(markup)
      const id = attributes.id?.trim()

      if (!id) {
        return null
      }

      return {
        type: "mention",
        id,
        markup,
      }
    }

    if (markup.startsWith("<source")) {
      const attributes = ChatInlineMarkupHelper.parseAttributes(markup)
      const sourceType = attributes.type?.trim() as ChatInlineSourceType | undefined
      const name = attributes.name?.trim()
      const path = attributes.path?.trim() ?? ""

      if (!sourceType || !name) {
        return null
      }

      if (!["file", "folder", "skill"].includes(sourceType)) {
        return null
      }

      return {
        type: "source",
        sourceType,
        name,
        path,
        markup,
      }
    }

    return null
  }

  private static parseAttributes(markup: string) {
    const attributes: Record<string, string> = {}

    for (const match of markup.matchAll(ChatInlineMarkupHelper.ATTRIBUTE_PATTERN)) {
      const key = match[1]
      const value = match[2] ?? ""
      if (key) attributes[key] = value
    }

    return attributes
  }

  private static escapeAttribute(value: string) {
    return value.replaceAll('"', "&quot;")
  }

  private static escapeHtml(value: string) {
    return value
      .replaceAll("&", "&amp;")
      .replaceAll("<", "&lt;")
      .replaceAll(">", "&gt;")
  }

  private static serializeNode(node: Node): string {
    if (node.nodeType === Node.TEXT_NODE) {
      return node.textContent ?? ""
    }

    if (node.nodeType !== Node.ELEMENT_NODE) {
      return ""
    }

    const element = node as HTMLElement

    if (element.tagName === "BR") {
      return "\n"
    }

    const inlineMarkup = element.dataset.inlineMarkup

    if (inlineMarkup) {
      return inlineMarkup
    }

    return [...element.childNodes]
      .map((child) => ChatInlineMarkupHelper.serializeNode(child))
      .join("")
  }

  private static getOffsetBeforeNode(root: HTMLElement, targetNode: Node) {
    let offset = 0

    const visit = (node: Node): boolean => {
      if (node === targetNode) {
        return true
      }

      if (node.nodeType === Node.TEXT_NODE) {
        offset += node.textContent?.length ?? 0
        return false
      }

      if (node.nodeType !== Node.ELEMENT_NODE) {
        return false
      }

      const element = node as HTMLElement

      if (element.tagName === "BR") {
        offset += 1
        return false
      }

      if (element.dataset.inlineMarkup) {
        offset += element.dataset.inlineMarkup.length
        return false
      }

      for (const child of [...element.childNodes]) {
        if (visit(child)) {
          return true
        }
      }

      return false
    }

    for (const child of [...root.childNodes]) {
      if (visit(child)) {
        break
      }
    }

    return offset
  }

  private static getOffsetWithinNode(node: Node, nodeOffset: number) {
    if (node.nodeType === Node.TEXT_NODE) {
      return nodeOffset
    }

    if (node.nodeType !== Node.ELEMENT_NODE) {
      return 0
    }

    const element = node as HTMLElement
    const inlineMarkup = element.dataset.inlineMarkup

    if (inlineMarkup) {
      return nodeOffset > 0 ? inlineMarkup.length : 0
    }

    let offset = 0
    const childNodes = [...element.childNodes]

    for (let index = 0; index < Math.min(nodeOffset, childNodes.length); index += 1) {
      const child = childNodes[index]
      if (child) offset += ChatInlineMarkupHelper.getNodeLength(child)
    }

    return offset
  }

  private static getNodeLength(node: Node): number {
    if (node.nodeType === Node.TEXT_NODE) {
      return node.textContent?.length ?? 0
    }

    if (node.nodeType !== Node.ELEMENT_NODE) {
      return 0
    }

    const element = node as HTMLElement

    if (element.tagName === "BR") {
      return 1
    }

    if (element.dataset.inlineMarkup) {
      return element.dataset.inlineMarkup.length
    }

    return [...element.childNodes].reduce(
      (sum, child) => sum + ChatInlineMarkupHelper.getNodeLength(child),
      0,
    )
  }

  private static resolveSelectionTarget(root: HTMLElement, offset: number): { node: Node; offset: number } {
    let remaining = offset

    const visit = (node: Node): { node: Node; offset: number } | null => {
      if (node.nodeType === Node.TEXT_NODE) {
        const length = node.textContent?.length ?? 0
        if (remaining <= length) {
          return { node, offset: remaining }
        }
        remaining -= length
        return null
      }

      if (node.nodeType !== Node.ELEMENT_NODE) {
        return null
      }

      const element = node as HTMLElement

      if (element.tagName === "BR") {
        if (remaining <= 1) {
          return {
            node: element.parentNode ?? root,
            offset: ChatInlineMarkupHelper.getChildIndex(element) + (remaining === 0 ? 0 : 1),
          }
        }
        remaining -= 1
        return null
      }

      if (element.dataset.inlineMarkup) {
        const length = element.dataset.inlineMarkup.length
        if (remaining <= length) {
          return {
            node: element.parentNode ?? root,
            offset: ChatInlineMarkupHelper.getChildIndex(element) + (remaining === 0 ? 0 : 1),
          }
        }
        remaining -= length
        return null
      }

      for (const child of [...element.childNodes]) {
        const result = visit(child)
        if (result) {
          return result
        }
      }

      return null
    }

    for (const child of [...root.childNodes]) {
      const result = visit(child)
      if (result) {
        return result
      }
    }

    return {
      node: root,
      offset: root.childNodes.length,
    }
  }

  private static getChildIndex(node: Node) {
    return Array.prototype.indexOf.call(node.parentNode?.childNodes ?? [], node)
  }
}
