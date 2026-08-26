"use client";

import * as React from "react";
import { Button } from "@/components/ui/button";
import { cn } from "@/lib/utils";
import { saveBlob } from "@/lib/save-file";
import type { UIMessage } from "ai";
import { ArrowDownIcon, DownloadIcon } from "lucide-react";
import type { ComponentProps } from "react";
import { Virtuoso, type VirtuosoHandle } from "react-virtuoso";

export interface ConversationProps<T> extends Omit<
  ComponentProps<"div">,
  "children"
> {
  data: T[];
  itemContent: (index: number, item: T) => React.ReactNode;
  computeItemKey?: (index: number, item: T) => React.Key;
  followOutput?: "smooth" | "auto" | boolean;
  footerHeight?: number;
  initialIndex?: number | "last";
  scrollButtonClassName?: string;
}

export function Conversation<T>({
  data,
  itemContent,
  computeItemKey,
  className,
  followOutput = "smooth",
  footerHeight = 256,
  initialIndex = "last",
  scrollButtonClassName,
  ...props
}: ConversationProps<T>) {
  const virtuosoRef = React.useRef<VirtuosoHandle>(null);
  const [isAtBottom, setIsAtBottom] = React.useState(true);

  const initialTopMostItemIndex = React.useMemo(() => {
    if (data.length === 0) {
      return undefined;
    }

    if (initialIndex === "last") {
      return data.length - 1;
    }

    return Math.min(Math.max(initialIndex, 0), data.length - 1);
  }, [data.length, initialIndex]);

  const scrollToBottom = React.useCallback(() => {
    if (data.length === 0) {
      return;
    }

    virtuosoRef.current?.scrollToIndex({
      index: data.length - 1,
      align: "end",
      behavior: "smooth",
    });
  }, [data.length]);

  return (
    <div
      className={cn("relative flex-1 min-h-0 overflow-hidden", className)}
      role="log"
      {...props}
    >
      <Virtuoso
        className="size-full min-h-0"
        computeItemKey={computeItemKey}
        data={data}
        followOutput={followOutput}
        atBottomStateChange={setIsAtBottom}
        initialTopMostItemIndex={initialTopMostItemIndex}
        itemContent={itemContent}
        ref={virtuosoRef}
        components={{
          Footer: () => <div style={{ height: footerHeight }} />,
        }}
      />

      {!isAtBottom ? (
        <Button
          className={cn(
            "absolute bottom-4 left-[50%] translate-x-[-50%] rounded-md dark:bg-background dark:hover:bg-muted",
            scrollButtonClassName,
          )}
          onClick={scrollToBottom}
          size="icon"
          type="button"
          variant="outline"
        >
          <ArrowDownIcon className="size-4" />
        </Button>
      ) : null}
    </div>
  );
}

export type ConversationEmptyStateProps = ComponentProps<"div"> & {
  title?: string;
  description?: string;
  icon?: React.ReactNode;
};

export const ConversationEmptyState = ({
  className,
  title = "No messages yet",
  description = "Start a conversation to see messages here",
  icon,
  children,
  ...props
}: ConversationEmptyStateProps) => (
  <div
    className={cn(
      "flex size-full flex-col items-center justify-center gap-3 p-8 text-center",
      className,
    )}
    {...props}
  >
    {children ?? (
      <>
        {icon && <div className="text-muted-foreground">{icon}</div>}
        <div className="space-y-1">
          <h3 className="font-medium text-sm">{title}</h3>
          {description && (
            <p className="text-muted-foreground text-sm">{description}</p>
          )}
        </div>
      </>
    )}
  </div>
);

const getMessageText = (message: UIMessage): string =>
  message.parts
    .filter((part) => part.type === "text")
    .map((part) => part.text)
    .join("");

export type ConversationDownloadProps = Omit<
  ComponentProps<typeof Button>,
  "onClick"
> & {
  messages: UIMessage[];
  filename?: string;
  formatMessage?: (message: UIMessage, index: number) => string;
};

const defaultFormatMessage = (message: UIMessage): string => {
  const roleLabel =
    message.role.charAt(0).toUpperCase() + message.role.slice(1);
  return `**${roleLabel}:** ${getMessageText(message)}`;
};

export const messagesToMarkdown = (
  messages: UIMessage[],
  formatMessage: (
    message: UIMessage,
    index: number,
  ) => string = defaultFormatMessage,
): string => messages.map((msg, i) => formatMessage(msg, i)).join("\n\n");

export const ConversationDownload = ({
  messages,
  filename = "conversation.md",
  formatMessage = defaultFormatMessage,
  className,
  children,
  ...props
}: ConversationDownloadProps) => {
  const handleDownload = React.useCallback(() => {
    const markdown = messagesToMarkdown(messages, formatMessage);
    // See lib/save-file.ts: `<a download>` is inert in the desktop window.
    const blob = new Blob([markdown], { type: "text/markdown" });
    void saveBlob(blob, filename);
  }, [messages, filename, formatMessage]);

  return (
    <Button
      className={cn(
        "absolute top-4 right-4 rounded-md dark:bg-background dark:hover:bg-muted",
        className,
      )}
      onClick={handleDownload}
      size="icon"
      type="button"
      variant="outline"
      {...props}
    >
      {children ?? <DownloadIcon className="size-4" />}
    </Button>
  );
};
