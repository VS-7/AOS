import * as React from "react";
import {
  PromptInputCommand,
  PromptInputCommandEmpty,
  PromptInputCommandGroup,
  PromptInputCommandInput,
  PromptInputCommandItem,
  PromptInputCommandList,
} from "@/components/ui/prompt-input";
import { PopoverContent } from "@/components/ui/popover";
import {
  Avatar,
  AvatarAgentFallback,
  AvatarFallback,
  AvatarImage,
} from "@/components/ui/avatar";
import { AtIcon, Attachment01Icon, FileLinkIcon, Folder01Icon, PlusSignIcon } from "@hugeicons/core-free-icons";
import { HugeiconsIcon } from "@hugeicons/react";
import { t } from "@/lib/i18n";
import type {
  ActiveMention,
  ComposerMentionTarget,
  ComposerReference,
} from "../../composer.types";

interface ChatComposerCommandMenuProps {
  commandMentionTargets: ComposerMentionTarget[];
  commandQuery: string;
  commandRef: React.RefObject<HTMLDivElement | null>;
  isDirectMessage: boolean;
  mentionState: ActiveMention | null;
  onMentionSelect: (target: ComposerMentionTarget) => void;
  onQueryChange: (value: string) => void;
  onReferenceSelect: (reference: ComposerReference) => void;
  onUploadSelect: () => void;
  selectableFiles: ComposerReference[];
}

export function ChatComposerCommandMenu({
  commandMentionTargets,
  commandQuery,
  commandRef,
  isDirectMessage,
  mentionState,
  onMentionSelect,
  onQueryChange,
  onReferenceSelect,
  onUploadSelect,
  selectableFiles,
}: ChatComposerCommandMenuProps) {
  const agentTargets = commandMentionTargets.filter(
    (target) => target.kind === "agent",
  );
  const userTargets = commandMentionTargets.filter(
    (target) => target.kind === "user",
  );
  const mentionLimit = mentionState ? 8 : 4;

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
          onValueChange={onQueryChange}
          placeholder={
            mentionState && !isDirectMessage
              ? "Search teammates..."
              : "Search files, folders, and mentions..."
          }
          value={commandQuery}
        />
        <PromptInputCommandList className="max-h-[360px]">
          <PromptInputCommandEmpty>
            {t("No matches found for this search.")}
          </PromptInputCommandEmpty>

          <PromptInputCommandItem
            onSelect={onUploadSelect}
            value="upload photo file attachment"
          >
            <HugeiconsIcon icon={Attachment01Icon} className="size-4 text-muted-foreground" />
            <span className="min-w-0 flex-1 truncate text-sm text-foreground">
              {t("Upload photo or file")}
            </span>
            <HugeiconsIcon icon={PlusSignIcon} className="size-4 text-muted-foreground" />
          </PromptInputCommandItem>

          {selectableFiles.length > 0 ? (
            <PromptInputCommandGroup heading={t("Files & folders")}>
              {selectableFiles.map((reference) => {
                const isFolder = reference.kind === "folder";
                const Icon = isFolder ? Folder01Icon : FileLinkIcon;

                return (
                  <PromptInputCommandItem
                    key={reference.id}
                    onSelect={() => onReferenceSelect(reference)}
                    value={reference.searchableValue}
                  >
                    <HugeiconsIcon icon={Icon} className="size-4 text-muted-foreground" />
                    <span className="min-w-0 flex-1 truncate text-sm text-foreground">
                      {reference.label}
                    </span>
                    <span className="rounded-full border px-1.5 py-0.5 text-[10px] uppercase tracking-[0.14em] text-muted-foreground">
                      {isFolder ? "Folder" : "File"}
                    </span>
                    <HugeiconsIcon icon={PlusSignIcon} className="size-4 text-muted-foreground" />
                  </PromptInputCommandItem>
                );
              })}
            </PromptInputCommandGroup>
          ) : null}

          {!isDirectMessage && agentTargets.length > 0 ? (
            <PromptInputCommandGroup heading={t("Agents")}>
              {agentTargets.slice(0, mentionLimit).map((target) => (
                <PromptInputCommandItem
                  key={target.key}
                  onSelect={() => onMentionSelect(target)}
                  value={`${target.label} ${target.mentionId} mention agent`}
                >
                  <Avatar className="size-5 rounded-full">
                    <AvatarAgentFallback name={target.mentionId} />
                  </Avatar>
                  <span className="min-w-0 flex-1 truncate text-sm text-foreground">
                    {target.label}
                  </span>
                  <span className="text-xs text-muted-foreground">
                    @{target.mentionId}
                  </span>
                  <HugeiconsIcon icon={AtIcon} className="size-4 text-muted-foreground" />
                </PromptInputCommandItem>
              ))}
            </PromptInputCommandGroup>
          ) : null}

          {!isDirectMessage && userTargets.length > 0 ? (
            <PromptInputCommandGroup heading={t("People")}>
              {userTargets.slice(0, mentionLimit).map((target) => {
                const initials = target.label
                  .split(/\s+/)
                  .filter(Boolean)
                  .slice(0, 2)
                  .map((part) => part[0]?.toUpperCase() ?? "")
                  .join("");

                return (
                  <PromptInputCommandItem
                    key={target.key}
                    onSelect={() => onMentionSelect(target)}
                    value={`${target.label} ${target.mentionId} mention user people`}
                  >
                    <Avatar className="size-5 rounded-full overflow-hidden">
                      {target.image ? (
                        <AvatarImage src={target.image} alt="" />
                      ) : (
                        <AvatarFallback className="rounded-full text-[9px] font-medium">
                          {initials || target.mentionId.slice(0, 2).toUpperCase()}
                        </AvatarFallback>
                      )}
                    </Avatar>
                    <span className="min-w-0 flex-1 truncate text-sm text-foreground">
                      {target.label}
                    </span>
                    <span className="text-xs text-muted-foreground">
                      @{target.mentionId}
                    </span>
                    <HugeiconsIcon icon={AtIcon} className="size-4 text-muted-foreground" />
                  </PromptInputCommandItem>
                );
              })}
            </PromptInputCommandGroup>
          ) : null}
        </PromptInputCommandList>
      </PromptInputCommand>
    </PopoverContent>
  );
}
