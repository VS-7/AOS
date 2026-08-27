import { t } from "@/lib/i18n";
import * as React from "react"
import {
  PromptInputCommand,
  PromptInputCommandEmpty,
  PromptInputCommandGroup,
  PromptInputCommandInput,
  PromptInputCommandItem,
  PromptInputCommandList,
} from "@/components/ui/prompt-input"
import { PopoverContent } from "@/components/ui/popover"
import { EraserIcon, PlusSignIcon, SparklesIcon, SquareIcon } from "@hugeicons/core-free-icons"
import { HugeiconsIcon } from "@hugeicons/react"
import type { ActiveSkillTrigger, ComposerReference } from "../../composer.types"

export interface ChatComposerSlashCommandItem {
  description: string
  disabled?: boolean
  id: string
  label: string
  searchValue: string
}

interface ChatComposerSkillsCommandMenuProps {
  commandRef: React.RefObject<HTMLDivElement | null>
  onCommandSelect: (item: ChatComposerSlashCommandItem) => void
  onQueryChange: (value: string) => void
  onReferenceSelect: (reference: ComposerReference) => void
  query: string
  slashCommands: ChatComposerSlashCommandItem[]
  selectableSkills: ComposerReference[]
  trigger: ActiveSkillTrigger | null
}

export function ChatComposerSkillsCommandMenu({
  commandRef,
  onCommandSelect,
  onQueryChange,
  onReferenceSelect,
  query,
  slashCommands,
  selectableSkills,
  trigger,
}: ChatComposerSkillsCommandMenuProps) {
  return (
    <PopoverContent
      align="start"
      className="w-[420px] overflow-hidden p-0"
      ref={commandRef}
      side="top"
      sideOffset={12}
    >
      <PromptInputCommand loop shouldFilter>
        <PromptInputCommandInput
          autoFocus
          onValueChange={onQueryChange}
          placeholder={t("Search skills and commands…")}
          value={query}
        />
        <PromptInputCommandList className="max-h-[360px]">
          <PromptInputCommandEmpty>{t("No skills or commands match this search.")}</PromptInputCommandEmpty>

          {slashCommands.length > 0 ? (
            <PromptInputCommandGroup heading={t("Commands")}>
              {slashCommands.map((item) => {
                const Icon = item.id === "stop" ? SquareIcon : EraserIcon
                const badge = item.id === "stop" ? "Stop" : "Clear"

                return (
                  <PromptInputCommandItem
                    disabled={item.disabled}
                    key={item.id}
                    onSelect={() => onCommandSelect(item)}
                    value={item.searchValue}
                  >
                    <HugeiconsIcon icon={Icon} className="size-4 text-muted-foreground" />
                    <span className="min-w-0 flex-1 truncate text-sm text-foreground">
                      {item.label}
                    </span>
                    <span className="rounded-full border px-1.5 py-0.5 text-[10px] uppercase tracking-[0.14em] text-muted-foreground">
                      {badge}
                    </span>
                  </PromptInputCommandItem>
                )
              })}
            </PromptInputCommandGroup>
          ) : null}

          <PromptInputCommandGroup heading={t("Skills")}>
            {selectableSkills.map((reference) => (
              <PromptInputCommandItem
                key={reference.id}
                onSelect={() => onReferenceSelect(reference)}
                value={reference.searchableValue}
              >
                <HugeiconsIcon icon={SparklesIcon} className="size-4 text-muted-foreground" />
                <span className="min-w-0 flex-1 truncate text-sm text-foreground">{reference.label}</span>
                <span className="rounded-full border px-1.5 py-0.5 text-[10px] uppercase tracking-[0.14em] text-muted-foreground">
                  {t("Skill")}
                </span>
                <HugeiconsIcon icon={PlusSignIcon} className="size-4 text-muted-foreground" />
              </PromptInputCommandItem>
            ))}
          </PromptInputCommandGroup>

          {trigger ? (
            <div className="border-t px-3 py-2 text-[11px] text-muted-foreground">
              {t("Type a space to insert a literal slash, or keep filtering skills and commands.")}
            </div>
          ) : null}
        </PromptInputCommandList>
      </PromptInputCommand>
    </PopoverContent>
  )
}

export const CHAT_COMPOSER_SLASH_COMMANDS = {
  CLEAR: {
    description: "Clear the current chat context and start a fresh thread.",
    id: "clear",
    label: "Clear chat context",
    searchValue: "clear reset context chat thread conversation",
  },
  STOP: {
    description: "Stop the agent's current execution and discard its in-flight work.",
    id: "stop",
    label: "Stop current execution",
    searchValue: "stop halt cancel abort current execution agent",
  },
} as const satisfies Record<string, ChatComposerSlashCommandItem>
