import {
  BotIcon,
  CheckListIcon,
  TextNumberSignIcon,
  RepeatIcon,
} from "@hugeicons/core-free-icons";
import { HugeiconsIcon, type IconSvgElement } from "@hugeicons/react";
import type { FractalChatKind } from "@/features/chat/services/chat/chat-kind.helper";

const KIND_ICON: Record<Exclude<FractalChatKind, "external">, IconSvgElement> = {
  channel: TextNumberSignIcon,
  dm: BotIcon,
  task: CheckListIcon,
  run: RepeatIcon,
};

interface ChatRowKindIconProps {
  kind: FractalChatKind;
  className?: string;
}

/**
 * Leading kind glyph for chat sidebar rows and search hits.
 */
export function ChatRowKindIcon({ kind, className }: ChatRowKindIconProps) {
  const icon =
    kind === "external" ? TextNumberSignIcon : (KIND_ICON[kind] ?? TextNumberSignIcon);

  return (
    <HugeiconsIcon
      icon={icon}
      className={className ?? "size-3.5 shrink-0 text-muted-foreground"}
    />
  );
}
