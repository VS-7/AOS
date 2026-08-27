import React, { useState } from "react";
import { Button } from "@/components/ui/button";
import {
  ArrowUp,
  ChevronDown,
  CornerDownRight,
  MessageSquare,
  Paperclip,
  X,
} from "lucide-react";
import { Badge } from "@/components/ui/badge";
import {
  Collapsible,
  CollapsibleTrigger,
  CollapsibleContent,
} from "@/components/ui/collapsible";
import { Skeleton } from "@/components/ui/skeleton";
import { aos } from "@/app/aos";
import * as z from "zod";
import { toast } from "sonner";
import { Form, FormControl, FormField } from "@/components/ui/form";
import { cn } from "@/lib/utils";
import type { TaskComment } from "@/features/task/interfaces/comment.interfaces";
import { t } from "@/lib/i18n";
import {
  CommentItem,
  type CommentThreadNode,
} from "./components/item.component";

const commentSchema = z.object({
  body: z.string().min(1, "Comment cannot be empty"),
});

function getMentionLabel(author: string) {
  return `@${author.trim().replace(/^@+/, "").replace(/\s+/g, "_")}`;
}

function ensureMentionPrefix(body: string, author: string) {
  const mention = getMentionLabel(author);

  if (body.trimStart().startsWith(mention)) {
    return body;
  }

  return `${mention} ${body}`;
}

function sortCommentsByCreatedAt(comments: TaskComment[]) {
  return [...comments].sort((left, right) => {
    return (
      new Date(left.createdAt).getTime() - new Date(right.createdAt).getTime()
    );
  });
}

function buildCommentThread(comments: TaskComment[]) {
  const sortedComments = sortCommentsByCreatedAt(comments);
  const childrenByParentId = new Map<string, TaskComment[]>();
  const commentsById = new Map(
    sortedComments.map((comment) => [comment.id, comment]),
  );

  for (const comment of sortedComments) {
    if (!comment.parentId || !commentsById.has(comment.parentId)) {
      continue;
    }

    const siblings = childrenByParentId.get(comment.parentId) || [];
    siblings.push(comment);
    childrenByParentId.set(comment.parentId, siblings);
  }

  function createNode(
    comment: TaskComment,
    depth: number,
    mentionedAuthor?: string,
  ): CommentThreadNode {
    const childComments = childrenByParentId.get(comment.id) || [];

    return {
      comment,
      depth,
      mentionedAuthor,
      children: childComments.map((childComment) => {
        if (depth >= 2) {
          return createNode(childComment, 2, comment.author);
        }

        return createNode(childComment, depth + 1);
      }),
    };
  }

  return sortedComments
    .filter(
      (comment) => !comment.parentId || !commentsById.has(comment.parentId),
    )
    .map((comment) => createNode(comment, 0));
}

function getReplyTargetParentId(node: CommentThreadNode) {
  if (node.depth >= 2) {
    return node.comment.parentId || node.comment.id;
  }

  return node.comment.id;
}

interface TaskCommentsProps {
  taskId: string;
}

export function TaskComments({ taskId }: TaskCommentsProps) {
  const [isOpen, setIsOpen] = useState(false);
  const [replyTarget, setReplyTarget] = useState<CommentThreadNode | null>(
    null,
  );

  const { data: commentsData, isLoading: isLoadingComments } =
    aos.client.comment.list.useQuery({
      params: { taskId },
      enabled: !!taskId,
    });

  // `useQuery`'s unwrapped payload is `unknown` too (see the facade's own
  // comment on why) — same cast pattern as the loaders.
  const comments = (commentsData as { comments: TaskComment[] } | null | undefined)?.comments || [];
  const thread = buildCommentThread(comments);

  // `aos.useQueryClient()` is TanStack's real `useQueryClient()` (see
  // `app/builders/app.tsx`), which has no `.invalidate()` shorthand — the
  // source assumed a AOS-only convenience this port doesn't have.
  // `invalidateQueries` with the facade's actual query key
  // (`[feature, action, flattenArgs(opts)]`, see `lib/aos-facade.ts`) is
  // the direct equivalent.
  const queryClient = aos.useQueryClient();

  const form = aos.useForm({
    schema: commentSchema,
    mutation: "comment.create",
    values: {
      body: "",
    },
    onSubmit: (values) => ({
      params: { taskId },
      body: {
        body:
          replyTarget && replyTarget.depth >= 2
            ? ensureMentionPrefix(values.body, replyTarget.comment.author)
            : values.body,
        attachments: [],
        ...(replyTarget
          ? { replyToId: getReplyTargetParentId(replyTarget) }
          : {}),
      },
    }),
    onResponse: ({ error }) => {
      if (error) {
        toast.error(error.message || "Failed to add comment");
        return;
      }
      toast.success(replyTarget ? "Reply added" : "Comment added");
      form.reset();
      setReplyTarget(null);
      // Query key shape is the facade's own: `[feature, action,
      // flattenArgs(opts)]` (`lib/aos-facade.ts`) — the *unrenamed*
      // payload the ported code passed, matching the `useQuery` call
      // above (`params: { taskId }`) exactly. The `task`/`taskId` rename
      // itself lives in `command-map.ts` now, not here.
      void queryClient.invalidateQueries({ queryKey: ["comment", "list", { taskId }] });
    },
  });

  return (
    <Collapsible onOpenChange={setIsOpen} className="space-y-4">
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-2">
          <CollapsibleTrigger className="text-sm font-semibold flex items-center gap-1.5 text-foreground/70 hover:text-foreground transition-colors">
            <ChevronDown
              className={`size-4 transition-transform duration-200 ${isOpen ? "rotate-0" : "rotate-180"}`}
            />
            {t("Comments")}{" "}
            <Badge
              className="rounded-md flex items-center gap-1 px-2"
              variant="outline"
            >
              <MessageSquare className="size-3" /> {comments.length}
            </Badge>
          </CollapsibleTrigger>
        </div>
      </div>

      <CollapsibleContent>
        <div className="bg-card divide-y border rounded-md">
          {isLoadingComments && (
            <div className="space-y-4">
              <Skeleton className="h-4 w-3/4" />
              <Skeleton className="h-4 w-1/2" />
              <Skeleton className="h-4 w-2/3" />
            </div>
          )}

          {!isLoadingComments && thread.length === 0 && (
            <div className="px-4 py-6 text-sm text-muted-foreground">
              {t("No comments yet. Start the thread with context, decisions, or next steps.")}
            </div>
          )}

          <div className="divide-y">
            {thread.map((node) => (
              <CommentItem
                key={node.comment.id}
                node={node}
                onReply={setReplyTarget}
                replyTargetId={replyTarget?.comment.id}
              />
            ))}
          </div>

          <Form form={form}>
            <FormField
              control={form.control}
              name="body"
              render={({ field }) => (
                <div className="relative bg-secondary/25 transition-all focus-within:bg-secondary ease-in-out overflow-hidden">
                  {replyTarget && (
                    <div className="flex items-center justify-between text-sm border-b bg-background px-4 py-3">
                      <div className="flex items-center gap-2 font-medium text-foreground">
                        <CornerDownRight className="size-4 text-muted-foreground" />
                        {t("Replying to")} {replyTarget.comment.author}
                      </div>
                      <Button
                        variant="ghost"
                        size="icon"
                        className="size-7 shrink-0 rounded-md"
                        type="button"
                        onClick={() => setReplyTarget(null)}
                      >
                        <X className="size-4" />
                        <span className="sr-only">{t("Cancel reply")}</span>
                      </Button>
                    </div>
                  )}

                  {replyTarget && replyTarget.depth >= 2 && (
                    <div className="flex items-center justify-between gap-2">
                      <Badge
                        variant="secondary"
                        className="rounded-md px-2 py-0.5 text-[11px]"
                      >
                        {t("Auto-mention enabled")}
                      </Badge>
                    </div>
                  )}

                  <div className="p-4">
                    <FormControl>
                      <textarea
                        {...field}
                        placeholder={
                          replyTarget
                            ? "Write a reply..."
                            : "Leave a comment..."
                        }
                        className={cn(
                          "min-h-16 w-full resize-none bg-transparent text-sm placeholder:text-muted-foreground focus:outline-none",
                          "shadow-inner",
                        )}
                        onKeyDown={(e) => {
                          if (e.key === "Enter" && (e.metaKey || e.ctrlKey)) {
                            e.preventDefault();
                            form.submit();
                          }
                        }}
                      />
                    </FormControl>
                    <div className="mt-3 flex items-center justify-end gap-2">
                      <div className="flex items-center gap-2">
                        <Button
                          variant="ghost"
                          size="icon"
                          className="size-8 text-muted-foreground"
                          type="button"
                          disabled
                        >
                          <Paperclip className="size-4" />
                          <span className="sr-only">{t("Attach file")}</span>
                        </Button>
                        <Button
                          size="icon"
                          className="size-8 rounded-lg"
                          type="submit"
                          disabled={!field.value.trim() || form.isLoading}
                        >
                          <ArrowUp className="size-4" />
                        </Button>
                      </div>
                    </div>
                  </div>
                </div>
              )}
            />
          </Form>
        </div>
      </CollapsibleContent>
    </Collapsible>
  );
}
