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
import { Avatar, AvatarAgentFallback } from "@/components/ui/avatar";
import { AtSignIcon } from "lucide-react";
import type { ActiveMention, ComposerMentionTarget } from "../../composer.types";

/**
 * Trimmed from the original: that menu also offered file/folder references
 * (a workspace file search that doesn't exist here) and a local-file upload
 * row. chats_send only accepts text (see internal/domain/chat/schema.go's
 * SendInput), so a file the person attached here would have nowhere to go —
 * offering that button would silently lose their file rather than send it.
 * Agent mentions are real (agents_list) and AOS's Go router parses this
 * markup natively (see internal/domain/chat/routing.go's inlineMention),
 * so that part is unchanged.
 */
interface ChatComposerCommandMenuProps {
  commandMentionTargets: ComposerMentionTarget[];
  commandQuery: string;
  commandRef: React.RefObject<HTMLDivElement | null>;
  isDirectMessage: boolean;
  mentionState: ActiveMention | null;
  onMentionSelect: (target: ComposerMentionTarget) => void;
  onQueryChange: (value: string) => void;
}

export function ChatComposerCommandMenu({
  commandMentionTargets,
  commandQuery,
  commandRef,
  isDirectMessage,
  mentionState,
  onMentionSelect,
  onQueryChange,
}: ChatComposerCommandMenuProps) {
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
          placeholder="Search agents to mention..."
          value={commandQuery}
        />
        <PromptInputCommandList className="max-h-[360px]">
          <PromptInputCommandEmpty>
            No matches found for this search.
          </PromptInputCommandEmpty>

          {!isDirectMessage && commandMentionTargets.length > 0 ? (
            <PromptInputCommandGroup heading="Agents">
              {commandMentionTargets.slice(0, mentionLimit).map((target) => (
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
                  <AtSignIcon className="size-4 text-muted-foreground" />
                </PromptInputCommandItem>
              ))}
            </PromptInputCommandGroup>
          ) : null}
        </PromptInputCommandList>
      </PromptInputCommand>
    </PopoverContent>
  );
}
