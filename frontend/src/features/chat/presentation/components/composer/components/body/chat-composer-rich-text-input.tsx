import * as React from "react"
import { Node } from "@tiptap/core"
import Placeholder from "@tiptap/extension-placeholder"
import StarterKit from "@tiptap/starter-kit"
import { HugeiconsIcon } from "@hugeicons/react"
import { FileLinkIcon, Folder01Icon, SparklesIcon } from "@hugeicons/core-free-icons"
import { AtSignIcon } from "lucide-react"
import { EditorContent, NodeViewWrapper, ReactNodeViewRenderer, useEditor } from "@tiptap/react"
import { usePromptInputAttachments } from "@/components/ui/prompt-input"
import { cn } from "@/lib/utils"
import { ComposerHelper } from "@/features/chat/presentation/helpers/composer.helper"
import { ChatInlineMarkupHelper } from "@/features/chat/presentation/helpers/chat-inline-markup.helper"
import {
  CHAT_COMPOSER_MENTION_NODE,
  CHAT_COMPOSER_SOURCE_NODE,
  createChatComposerDocument,
  getChatComposerSelectionOffset,
  getChatComposerSelectionPosition,
  serializeChatComposerDocument,
} from "@/features/chat/presentation/helpers/chat-composer-editor.helper"
import type { ComposerReference } from "../../composer.types"

interface ChatComposerRichTextInputProps {
  className?: string
  disabled?: boolean
  onEscape?: () => void
  onSelectionChange: (value: string, caret: number) => void
  placeholder: string
  value: string
}

export interface ChatComposerRichTextInputRef {
  clear: () => void
  focus: () => void
  insertMention: (agentId: string) => void
  insertReference: (reference: ComposerReference) => void
}

function MentionBadgeView(props: { node: { attrs: { id?: string } } }) {
  return (
    <NodeViewWrapper
      as="span"
      className="mx-0.5 align-middle inline-flex h-7 items-center gap-1.5 rounded-full border border-emerald-200/80 bg-emerald-50 px-2.5 text-[12px] font-medium text-emerald-700 dark:border-emerald-500/30 dark:bg-emerald-500/10 dark:text-emerald-200"
      contentEditable={false}
    >
      <AtSignIcon className="size-3.5 shrink-0" />
      <span className="truncate">{props.node.attrs.id ?? ""}</span>
    </NodeViewWrapper>
  )
}

function SourceBadgeView(props: { node: { attrs: { name?: string; sourceType?: "file" | "folder" | "skill" } } }) {
  const sourceType = props.node.attrs.sourceType ?? "file"
  const Icon = sourceType === "skill" ? SparklesIcon : sourceType === "folder" ? Folder01Icon : FileLinkIcon
  const colorClass = sourceType === "skill"
    ? "border-violet-200/80 bg-violet-50 text-violet-700 dark:border-violet-500/30 dark:bg-violet-500/10 dark:text-violet-200"
    : sourceType === "folder"
      ? "border-amber-200/80 bg-amber-50 text-amber-700 dark:border-amber-500/30 dark:bg-amber-500/10 dark:text-amber-200"
      : "border-sky-200/80 bg-sky-50 text-sky-700 dark:border-sky-500/30 dark:bg-sky-500/10 dark:text-sky-200"

  return (
    <NodeViewWrapper
      as="span"
      className={`mx-0.5 align-middle inline-flex h-7 max-w-full items-center gap-1.5 rounded-full border px-2.5 text-[12px] font-medium ${colorClass}`}
      contentEditable={false}
    >
      <HugeiconsIcon icon={Icon} className="size-3.5 shrink-0" />
      <span className="truncate">{props.node.attrs.name ?? ""}</span>
    </NodeViewWrapper>
  )
}

const ChatMentionNode = Node.create({
  addAttributes() {
    return {
      id: {
        default: "",
      },
    }
  },
  atom: true,
  group: "inline",
  inline: true,
  name: CHAT_COMPOSER_MENTION_NODE,
  selectable: false,
  renderHTML({ HTMLAttributes }) {
    return ["span", { "data-chat-mention-node": String(HTMLAttributes.id ?? "") }]
  },
  addNodeView() {
    return ReactNodeViewRenderer(MentionBadgeView)
  },
})

const ChatSourceNode = Node.create({
  addAttributes() {
    return {
      name: {
        default: "",
      },
      path: {
        default: "",
      },
      sourceType: {
        default: "file",
      },
    }
  },
  atom: true,
  group: "inline",
  inline: true,
  name: CHAT_COMPOSER_SOURCE_NODE,
  selectable: false,
  renderHTML({ HTMLAttributes }) {
    return ["span", {
      "data-chat-source-node": String(HTMLAttributes.name ?? ""),
      "data-source-type": String(HTMLAttributes.sourceType ?? "file"),
    }]
  },
  addNodeView() {
    return ReactNodeViewRenderer(SourceBadgeView)
  },
})

export const ChatComposerRichTextInput = React.forwardRef<ChatComposerRichTextInputRef, ChatComposerRichTextInputProps>(function ChatComposerRichTextInput({
  className,
  disabled,
  onEscape,
  onSelectionChange,
  placeholder,
  value,
}, ref) {
  const attachments = usePromptInputAttachments()
  const lastKnownValueRef = React.useRef(value)

  const editor = useEditor({
    content: createChatComposerDocument(value),
    editorProps: {
      attributes: {
        "aria-label": "Message",
        class: cn(
          "min-h-16 px-4 pb-1 pt-3 text-[14px] leading-6 outline-none w-full",
          "whitespace-pre-wrap break-words",
          disabled && "cursor-not-allowed opacity-60",
          className,
        ),
        "data-chat-composer-editor": "true",
        "data-placeholder": placeholder,
      },
      handleKeyDown(view, event): boolean {
        if (event.key === "Escape") {
          onEscape?.()
          return false
        }

        if (event.key === "Enter" && !event.shiftKey && !event.isComposing) {
          event.preventDefault()
          const form = view.dom.closest("form")
          const submitButton = form?.querySelector('button[type="submit"]') as HTMLButtonElement | null

          if (!submitButton?.disabled) {
            form?.requestSubmit()
          }

          return true
        }

        if (event.key === "Enter" && event.shiftKey) {
          event.preventDefault()
          const hardBreak = view.state.schema.nodes.hardBreak

          if (!hardBreak) {
            return false
          }

          view.dispatch(
            view.state.tr.replaceSelectionWith(hardBreak.create()).scrollIntoView(),
          )
          return true
        }

        if (event.key === "Backspace" && lastKnownValueRef.current.length === 0 && attachments.files.length > 0) {
          const lastAttachment = attachments.files.at(-1)

          if (lastAttachment) {
            event.preventDefault()
            attachments.remove(lastAttachment.id)
            return true
          }
        }

        return false
      },
    },
    editable: !disabled,
    extensions: [
      StarterKit.configure({
        blockquote: false,
        bold: false,
        bulletList: false,
        code: false,
        codeBlock: false,
        dropcursor: false,
        gapcursor: false,
        heading: false,
        horizontalRule: false,
        italic: false,
        listItem: false,
        orderedList: false,
        strike: false,
      }),
      Placeholder.configure({
        emptyEditorClass: "before:pointer-events-none before:text-muted-foreground before:content-[attr(data-placeholder)]",
        placeholder,
      }),
      ChatMentionNode,
      ChatSourceNode,
    ],
    immediatelyRender: false,
    onCreate({ editor }) {
      const nextValue = serializeChatComposerDocument(editor.state.doc)
      lastKnownValueRef.current = nextValue
      onSelectionChange(nextValue, getChatComposerSelectionOffset(editor.state))
    },
    onSelectionUpdate({ editor }) {
      const nextValue = serializeChatComposerDocument(editor.state.doc)
      lastKnownValueRef.current = nextValue
      onSelectionChange(nextValue, getChatComposerSelectionOffset(editor.state))
    },
    onUpdate({ editor }) {
      const nextValue = serializeChatComposerDocument(editor.state.doc)
      lastKnownValueRef.current = nextValue
      onSelectionChange(nextValue, getChatComposerSelectionOffset(editor.state))
    },
  }, [placeholder])

  React.useEffect(() => {
    if (!editor) {
      return
    }

    editor.setEditable(!disabled)
  }, [disabled, editor])

  React.useEffect(() => {
    if (!editor || value === lastKnownValueRef.current) {
      return
    }

    const currentCaret = getChatComposerSelectionOffset(editor.state)
    editor.commands.setContent(createChatComposerDocument(value), false)
    editor.commands.setTextSelection(getChatComposerSelectionPosition(editor.state, currentCaret))
    lastKnownValueRef.current = value
  }, [editor, value])

  const applyNextValue = React.useCallback((nextValue: string, nextCaret: number) => {
    if (!editor) {
      return
    }

    editor.commands.setContent(createChatComposerDocument(nextValue), false)
    editor.commands.setTextSelection(getChatComposerSelectionPosition(editor.state, nextCaret))
    editor.commands.focus()
    lastKnownValueRef.current = nextValue
    onSelectionChange(nextValue, nextCaret)
  }, [editor, onSelectionChange])

  React.useImperativeHandle(ref, () => ({
    clear() {
      if (!editor) {
        return
      }

      editor.commands.setContent(createChatComposerDocument(""), false)
      editor.commands.setTextSelection(1)
      editor.commands.focus()
      lastKnownValueRef.current = ""
    },
    focus() {
      editor?.chain().focus().run()
    },
    insertMention(agentId: string) {
      if (!editor) {
        return
      }

      const currentValue = serializeChatComposerDocument(editor.state.doc)
      const caret = getChatComposerSelectionOffset(editor.state)
      const mentionState = ComposerHelper.getActiveMention(currentValue, caret)
      const markup = ChatInlineMarkupHelper.buildMentionTag({ id: agentId })
      const result = mentionState
        ? replaceRange(currentValue, mentionState.range.start, mentionState.range.end, markup)
        : insertAt(currentValue, caret, markup)

      applyNextValue(result.nextValue, result.nextCaret)
    },
    insertReference(reference: ComposerReference) {
      if (!editor) {
        return
      }

      const currentValue = serializeChatComposerDocument(editor.state.doc)
      const caret = getChatComposerSelectionOffset(editor.state)
      const mentionState = ComposerHelper.getActiveMention(currentValue, caret)
      const markup = ChatInlineMarkupHelper.buildSourceTag(reference)
      const result = mentionState
        ? replaceRange(currentValue, mentionState.range.start, mentionState.range.end, markup)
        : insertAt(currentValue, caret, markup)

      applyNextValue(result.nextValue, result.nextCaret)
    },
  }), [applyNextValue, editor])

  return (
    <EditorContent
      editor={editor}
      className="w-full [&_.ProseMirror]:outline-none [&_.ProseMirror]:whitespace-pre-wrap [&_.ProseMirror]:break-words [&_.ProseMirror_p]:m-0 [&_.ProseMirror_.ProseMirror-selectednode]:outline-none"
      role="textbox"
    />
  )
})

function insertAt(value: string, offset: number, markup: string) {
  const boundedOffset = Math.max(0, Math.min(offset, value.length))
  const prefixNeedsSpace = boundedOffset > 0 && !/\s/.test(value[boundedOffset - 1] ?? "")
  const suffixNeedsSpace = boundedOffset < value.length && !/\s/.test(value[boundedOffset] ?? "")
  const insertion = `${prefixNeedsSpace ? " " : ""}${markup}${suffixNeedsSpace ? " " : ""}`

  return {
    nextCaret: boundedOffset + insertion.length,
    nextValue: `${value.slice(0, boundedOffset)}${insertion}${value.slice(boundedOffset)}`,
  }
}

function replaceRange(value: string, start: number, end: number, markup: string) {
  const prefix = value.slice(0, start)
  const suffix = value.slice(end)
  const prefixNeedsSpace = prefix.length > 0 && !/\s/.test(prefix[prefix.length - 1] ?? "")
  const suffixNeedsSpace = suffix.length > 0 && !/^\s/.test(suffix)
  const replacement = `${prefixNeedsSpace ? " " : ""}${markup}${suffixNeedsSpace ? " " : ""}`

  return {
    nextCaret: prefix.length + replacement.length,
    nextValue: `${prefix}${replacement}${suffix}`,
  }
}
