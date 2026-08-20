import React, { useState } from "react";
import { Button } from "@/components/ui/button";
import { Avatar, AvatarAgentFallback, AvatarFallback } from "@/components/ui/avatar";
import type { TaskComment } from "@/features/task/interfaces/comment.interfaces";
import { Badge } from "@/components/ui/badge";
import { cn, timeAgo } from "@/lib/utils";
import { MarkdownRenderer } from "@/components/ui/markdown-content";
import { ChevronDown, CornerDownRight, Paperclip } from "lucide-react";
import { AttachmentItem } from "../../attachments/components/item.component";

const COLLAPSED_MAX_HEIGHT_CLASS = "max-h-64";

function getMentionLabel(author: string) {
  return `@${author.trim().replace(/^@+/, "").replace(/\s+/g, "_")}`;
}

// Reads `comment.content` — Go's stored `Comment` has no `body` field, only
// `content` (`internal/domain/comment/entity.go`); `body` is a write-side
// name for comment.create/update's input, not what a fetched comment
// carries back. See `interfaces/task.interfaces.ts`'s `CommentSchema`.
function getRenderedBody(comment: TaskComment, mentionedAuthor?: string) {
  if (!mentionedAuthor) {
    return comment.content;
  }

  const mention = getMentionLabel(mentionedAuthor);
  const body = comment.content.trimStart();

  if (body.startsWith(mention)) {
    return comment.content;
  }

  return `${mention} ${comment.content}`;
}

function shouldCollapseComment(body: string) {
  return body.length > 320 || body.split("\n").length > 8;
}

export interface CommentThreadNode {
  comment: TaskComment;
  depth: number;
  mentionedAuthor?: string;
  children: CommentThreadNode[];
}

interface CommentItemProps {
  node: CommentThreadNode;
  onReply: (node: CommentThreadNode) => void;
  replyTargetId?: string;
}

export function CommentItem({ node, onReply, replyTargetId }: CommentItemProps) {
  const [isExpanded, setIsExpanded] = useState(false);
  const { comment, depth, mentionedAuthor, children } = node;
  // The source read `comment.role`; the actual field (both the Go side's
  // `Comment.AuthorType` and this port's `CommentSchema`) is
  // `authorType`, already `"user" | "agent"` — same purpose, real name.
  const role = comment.authorType ?? "user";
  const isAgent = role === "agent";
  const isReplying = replyTargetId === comment.id;
  const initials = comment.author.substring(0, 2).toUpperCase();
  const timestamp = timeAgo(comment.createdAt);
  const renderedBody = getRenderedBody(comment, mentionedAuthor);
  const isCollapsible = shouldCollapseComment(renderedBody);
  const hasNestedChildren = children.some((child) => child.depth > depth);

  return (
    <div className={cn("space-y-3", depth > 0 && "ml-4 md:ml-6")}>
      <div
        className={cn(
          "p-4",
          depth > 0 && "relative before:absolute before:-left-4 before:top-6 before:h-px before:w-4 before:bg-border md:before:-left-6 md:before:w-6",
          isReplying && "bg-card/40",
        )}
      >
        <div className="flex gap-x-3">
          <Avatar className="size-6 shrink-0">
            {isAgent && (
              <AvatarAgentFallback name={comment.author.toLowerCase()} />
            )}

            {!isAgent && (
              <AvatarFallback className={cn(
                "text-sm"
              )}>
                {initials}
              </AvatarFallback>
            )}
          </Avatar>

          <div className="min-w-0 flex-1 space-y-3">
            <div className="flex flex-wrap items-center gap-2">
              <span className="text-sm font-medium">{comment.author}</span>
              {isAgent && (
                <Badge variant="secondary">
                  Agent
                </Badge>
              )}
              {depth > 0 && (
                <Badge variant="outline">
                  Reply
                </Badge>
              )}
              <span className="text-xs text-muted-foreground">{timestamp}</span>
            </div>

            <div className="space-y-2">
              <div className="relative">
                <div className={cn(!isExpanded && isCollapsible && `${COLLAPSED_MAX_HEIGHT_CLASS} overflow-hidden`)}>
                  <MarkdownRenderer content={renderedBody} />
                </div>

                {!isExpanded && isCollapsible && (
                  <div className="pointer-events-none absolute inset-x-0 bottom-0 h-12 bg-linear-to-t from-card via-card/90 to-transparent" />
                )}
              </div>

              {isCollapsible && (
                <Button
                  variant="ghost"
                  size="sm"
                  className="h-7 rounded-md px-2 text-xs text-muted-foreground hover:text-foreground"
                  type="button"
                  onClick={() => setIsExpanded((current) => !current)}
                >
                  <ChevronDown className={cn("mr-1 size-3.5 transition-transform", isExpanded && "rotate-180")} />
                  {isExpanded ? "Show less" : "Read more"}
                </Button>
              )}
            </div>

            {comment.attachments.length > 0 && (
              <div className="space-y-2">
                <div className="flex items-center gap-2 text-xs font-medium uppercase tracking-[0.18em] text-muted-foreground">
                  <Paperclip className="size-3.5" />
                  Attachments
                </div>
                <div className="divide-y rounded-xl border bg-card">
                  {comment.attachments.map((attachment, index) => (
                    <AttachmentItem key={`${attachment.uri}-${index}`} attachment={attachment} />
                  ))}
                </div>
              </div>
            )}

            <div className="flex items-center gap-2">
              <Button
                variant="ghost"
                size="sm"
                className="h-8 rounded-md px-2 text-xs text-muted-foreground hover:text-foreground"
                type="button"
                onClick={() => onReply(node)}
              >
                <CornerDownRight className="mr-1 size-3.5" />
                Reply
              </Button>
            </div>
          </div>
        </div>
      </div>

      {children.length > 0 && (
        <div className={cn("grid gap-3", hasNestedChildren && "border-l border-border/70 pl-4 md:pl-6")}>
          {children.map((child) => (
            <CommentItem
              key={child.comment.id}
              node={child}
              onReply={onReply}
              replyTargetId={replyTargetId}
            />
          ))}
        </div>
      )}
    </div>
  );
}
