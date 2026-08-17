import type { JSONContent } from "@tiptap/core"
import type { Node as ProseMirrorNode } from "@tiptap/pm/model"
import type { EditorState } from "@tiptap/pm/state"
import { ChatInlineMarkupHelper } from "./chat-inline-markup.helper"

export const CHAT_COMPOSER_MENTION_NODE = "chatMention"
export const CHAT_COMPOSER_SOURCE_NODE = "chatSource"

function createTextContent(value: string): JSONContent[] {
  if (!value) {
    return []
  }

  const segments = value.split("\n")
  const content: JSONContent[] = []

  segments.forEach((segment, index) => {
    if (segment.length > 0) {
      content.push({
        text: segment,
        type: "text",
      })
    }

    if (index < segments.length - 1) {
      content.push({
        type: "hardBreak",
      })
    }
  })

  return content
}

export function createChatComposerDocument(value: string): JSONContent {
  const content = ChatInlineMarkupHelper.parse(value).flatMap((token) => {
    if (token.type === "text") {
      return createTextContent(token.value)
    }

    if (token.type === "mention") {
      return [{
        attrs: {
          id: token.id,
        },
        type: CHAT_COMPOSER_MENTION_NODE,
      }]
    }

    return [{
      attrs: {
        name: token.name,
        path: token.path,
        sourceType: token.sourceType,
      },
      type: CHAT_COMPOSER_SOURCE_NODE,
    }]
  })

  return {
    content: [{
      content,
      type: "paragraph",
    }],
    type: "doc",
  }
}

function escapeAttribute(value: string) {
  return value.replaceAll('"', "&quot;")
}

function serializeInlineNode(node: ProseMirrorNode) {
  if (node.type.name === "text") {
    return node.text ?? ""
  }

  if (node.type.name === "hardBreak") {
    return "\n"
  }

  if (node.type.name === CHAT_COMPOSER_MENTION_NODE) {
    return ChatInlineMarkupHelper.buildMentionTag({
      id: String(node.attrs.id ?? ""),
    })
  }

  if (node.type.name === CHAT_COMPOSER_SOURCE_NODE) {
    return `<source type="${String(node.attrs.sourceType ?? "file")}" name="${escapeAttribute(String(node.attrs.name ?? ""))}" path="${escapeAttribute(String(node.attrs.path ?? ""))}">`
  }

  return ""
}

function getEditorLength(node: ProseMirrorNode) {
  if (node.type.name === "text") {
    return node.text?.length ?? 0
  }

  if (
    node.type.name === "hardBreak"
    || node.type.name === CHAT_COMPOSER_MENTION_NODE
    || node.type.name === CHAT_COMPOSER_SOURCE_NODE
  ) {
    return 1
  }

  return 0
}

function getSerializedLength(node: ProseMirrorNode) {
  return serializeInlineNode(node).length
}

export function serializeChatComposerDocument(doc: ProseMirrorNode) {
  let serialized = ""

  doc.forEach((block, _, index) => {
    if (block.type.name !== "paragraph") {
      return
    }

    block.forEach((child) => {
      serialized += serializeInlineNode(child)
    })

    if (index < doc.childCount - 1) {
      serialized += "\n"
    }
  })

  return serialized
}

function getFirstParagraph(doc: ProseMirrorNode) {
  const firstChild = doc.firstChild
  return firstChild?.type.name === "paragraph" ? firstChild : null
}

function getFirstParagraphStart(doc: ProseMirrorNode) {
  let start = 1

  doc.descendants((node, pos) => {
    if (node.type.name === "paragraph") {
      start = pos + 1
      return false
    }

    return undefined
  })

  return start
}

function parentOffsetToSerializedOffset(paragraph: ProseMirrorNode, parentOffset: number) {
  let remaining = Math.max(0, parentOffset)
  let serializedOffset = 0

  paragraph.forEach((child) => {
    if (remaining <= 0) {
      return false
    }

    const editorLength = getEditorLength(child)
    const serializedLength = getSerializedLength(child)

    if (child.type.name === "text") {
      const consumed = Math.min(remaining, editorLength)
      serializedOffset += consumed
      remaining -= consumed
      return undefined
    }

    if (remaining >= editorLength) {
      serializedOffset += serializedLength
      remaining -= editorLength
    }

    return undefined
  })

  return serializedOffset
}

function serializedOffsetToParentOffset(paragraph: ProseMirrorNode, serializedOffset: number) {
  let remaining = Math.max(0, serializedOffset)
  let parentOffset = 0

  paragraph.forEach((child) => {
    if (remaining <= 0) {
      return false
    }

    const editorLength = getEditorLength(child)
    const serializedLength = getSerializedLength(child)

    if (child.type.name === "text") {
      const consumed = Math.min(remaining, editorLength)
      parentOffset += consumed
      remaining -= consumed
      return undefined
    }

    if (remaining >= serializedLength) {
      parentOffset += editorLength
      remaining -= serializedLength
      return undefined
    }

    parentOffset += editorLength
    remaining = 0
    return false
  })

  return parentOffset
}

export function getChatComposerSelectionOffset(state: EditorState) {
  const paragraph = state.selection.$from.parent

  if (paragraph.type.name !== "paragraph") {
    return 0
  }

  return parentOffsetToSerializedOffset(paragraph, state.selection.$from.parentOffset)
}

export function getChatComposerSelectionPosition(state: EditorState, serializedOffset: number) {
  const paragraph = getFirstParagraph(state.doc)

  if (!paragraph) {
    return 1
  }

  return getFirstParagraphStart(state.doc) + serializedOffsetToParentOffset(paragraph, serializedOffset)
}
