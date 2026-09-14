import React, { useState } from "react";
import { toast } from "sonner";
import { Button } from "@/components/ui/button";
import { Avatar, AvatarAgentFallback, AvatarFallback } from "@/components/ui/avatar";
import type { TaskComment } from "@/features/task/interfaces/comment.interfaces";
import { Badge } from "@/components/ui/badge";
import { Textarea } from "@/components/ui/textarea";
import { useAlert } from "@/components/ui/alert-provider";
import { cn, timeAgo } from "@/lib/utils";
import { MarkdownRenderer } from "@/components/ui/markdown-content";
import { ChevronDown, CornerDownRight, Paperclip, Pencil, Trash2 } from "lucide-react";
import { AttachmentItem } from "../../attachments/components/item.component";
import { assigneeInitials } from "@/features/task/presentation/helpers/assignee.helper";
import { aos } from "@/app/aos";
import { errorMessage } from "@/lib/aos-facade";
import { t } from "@/lib/i18n";

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
  taskId: string;
  node: CommentThreadNode;
  onReply: (node: CommentThreadNode) => void;
  onChanged: () => void;
  replyTargetId?: string;
  /** The display name for an author id: an agent's name, a person's name. */
  authorName: (author: string) => string;
  /** The signed-in person, who may edit and delete what they wrote. */
  selfId?: string;
}

export function CommentItem({ taskId, node, onReply, onChanged, replyTargetId, authorName, selfId }: CommentItemProps) {
  const [isExpanded, setIsExpanded] = useState(false);
  const [editing, setEditing] = useState(false);
  const [draft, setDraft] = useState("");
  const [busy, setBusy] = useState(false);
  const { confirm } = useAlert();
  const { comment, depth, mentionedAuthor, children } = node;
  // The source read `comment.role`; the actual field (both the Go side's
  // `Comment.AuthorType` and this port's `CommentSchema`) is
  // `authorType`, already `"user" | "agent"` — same purpose, real name.
  const role = comment.authorType ?? "user";
  const isAgent = role === "agent";
  const isReplying = replyTargetId === comment.id;
  const name = authorName(comment.author);
  const timestamp = timeAgo(comment.createdAt);
  const renderedBody = getRenderedBody(comment, mentionedAuthor ? authorName(mentionedAuthor) : undefined);
  const isCollapsible = shouldCollapseComment(renderedBody);
  const hasNestedChildren = children.some((child) => child.depth > depth);
  // The daemon lets an actor edit only what it wrote (comment.Service's
  // guardOwnership), so the controls go only where they would be accepted.
  const isOwn = !isAgent && Boolean(selfId) && comment.author === selfId;
  // Go's `Comment` has no attachments; the item read `.length` off the
  // missing field and the whole task page fell to its error boundary.
  const attachments = comment.attachments ?? [];

  async function save() {
    setBusy(true);
    try {
      await aos.client.comment.update.mutateOrThrow({
        params: { taskId, id: comment.id },
        body: { body: draft },
      });
      toast.success(t("Comment updated"));
      setEditing(false);
      onChanged();
    } catch (error) {
      toast.error(t("Failed to update comment"), { description: errorMessage(error) });
    } finally {
      setBusy(false);
    }
  }

  async function remove() {
    const confirmed = await confirm({
      title: t("Delete this comment?"),
      description: t("Replies to it stay in the thread."),
      confirmText: t("Delete"),
      cancelText: t("Cancel"),
      variant: "destructive",
    });
    if (!confirmed) return;
    try {
      await aos.client.comment.delete.mutateOrThrow({ params: { taskId, id: comment.id } });
      toast.success(t("Comment deleted"));
      onChanged();
    } catch (error) {
      toast.error(t("Failed to delete comment"), { description: errorMessage(error) });
    }
  }

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
              <AvatarAgentFallback name={name.toLowerCase()} />
            )}

            {!isAgent && (
              <AvatarFallback className={cn(
                "text-sm"
              )}>
                {assigneeInitials(name)}
              </AvatarFallback>
            )}
          </Avatar>

          <div className="min-w-0 flex-1 space-y-3">
            <div className="flex flex-wrap items-center gap-2">
              <span className="text-sm font-medium">{name}</span>
              {isAgent && (
                <Badge variant="secondary">
                  {t("Agent")}
                </Badge>
              )}
              {depth > 0 && (
                <Badge variant="outline">
                  {t("Reply")}
                </Badge>
              )}
              <span className="text-xs text-muted-foreground">{timestamp}</span>
            </div>

            {editing ? (
              <div className="space-y-2">
                <Textarea
                  value={draft}
                  onChange={(event) => setDraft(event.target.value)}
                  className="min-h-20 text-sm"
                  autoFocus
                />
                <div className="flex items-center justify-end gap-2">
                  <Button variant="ghost" size="sm" type="button" onClick={() => setEditing(false)} disabled={busy}>
                    {t("Cancel")}
                  </Button>
                  <Button size="sm" type="button" onClick={() => void save()} disabled={busy || !draft.trim()}>
                    {busy ? t("Saving...") : t("Save")}
                  </Button>
                </div>
              </div>
            ) : (
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
                    {isExpanded ? t("Show less") : t("Read more")}
                  </Button>
                )}
              </div>
            )}

            {attachments.length > 0 && (
              <div className="space-y-2">
                <div className="flex items-center gap-2 text-xs font-medium uppercase tracking-[0.18em] text-muted-foreground">
                  <Paperclip className="size-3.5" />
                  {t("Attachments")}
                </div>
                <div className="divide-y rounded-xl border bg-card">
                  {attachments.map((attachment, index) => (
                    <AttachmentItem key={`${attachment.uri}-${index}`} attachment={attachment} />
                  ))}
                </div>
              </div>
            )}

            {!editing && (
              <div className="flex items-center gap-2">
                <Button
                  variant="ghost"
                  size="sm"
                  className="h-8 rounded-md px-2 text-xs text-muted-foreground hover:text-foreground"
                  type="button"
                  onClick={() => onReply(node)}
                >
                  <CornerDownRight className="mr-1 size-3.5" />
                  {t("Reply")}
                </Button>
                {isOwn && (
                  <>
                    <Button
                      variant="ghost"
                      size="sm"
                      className="h-8 rounded-md px-2 text-xs text-muted-foreground hover:text-foreground"
                      type="button"
                      onClick={() => {
                        setDraft(comment.content);
                        setEditing(true);
                      }}
                    >
                      <Pencil className="mr-1 size-3.5" />
                      {t("Edit")}
                    </Button>
                    <Button
                      variant="ghost"
                      size="sm"
                      className="h-8 rounded-md px-2 text-xs text-muted-foreground hover:text-destructive"
                      type="button"
                      onClick={() => void remove()}
                    >
                      <Trash2 className="mr-1 size-3.5" />
                      {t("Delete")}
                    </Button>
                  </>
                )}
              </div>
            )}
          </div>
        </div>
      </div>

      {children.length > 0 && (
        <div className={cn("grid gap-3", hasNestedChildren && "border-l border-border/70 pl-4 md:pl-6")}>
          {children.map((child) => (
            <CommentItem
              key={child.comment.id}
              taskId={taskId}
              node={child}
              onReply={onReply}
              onChanged={onChanged}
              replyTargetId={replyTargetId}
              authorName={authorName}
              selfId={selfId}
            />
          ))}
        </div>
      )}
    </div>
  );
}
