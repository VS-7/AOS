import React, { useState } from "react";
import { Plus, ChevronDown, Paperclip } from "lucide-react";
import { Button } from "@/components/ui/button";
import {
  Collapsible,
  CollapsibleTrigger,
  CollapsibleContent,
} from "@/components/ui/collapsible";
import type { TaskAttachment } from "@/features/task/interfaces/comment.interfaces";
import { AttachmentItem } from "./components/item.component";
import { Badge } from "@/components/ui/badge";
import { t } from "@/lib/i18n";

interface TaskAttachmentsProps {
  attachments: TaskAttachment[];
}

export function TaskAttachments({ attachments }: TaskAttachmentsProps) {
  const [isOpen, setIsOpen] = useState(false);

  return (
    <Collapsible onOpenChange={setIsOpen} className="space-y-4">
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-2">
          <CollapsibleTrigger className="text-sm font-semibold flex items-center gap-1.5 text-foreground/70 hover:text-foreground transition-colors">
            <ChevronDown
              className={`size-4 transition-transform duration-200 ${isOpen ? "rotate-0" : "rotate-180"}`}
            />
            {t("Attachments")}{" "}
            <Badge
              className="rounded-md flex items-center gap-1 px-2"
              variant="outline"
            >
              <Paperclip className="size-3" /> {attachments.length}
            </Badge>
          </CollapsibleTrigger>
        </div>
        <Button
          variant="ghost"
          size="icon"
          className="size-8 border rounded-md"
        >
          <Plus className="size-4" />
        </Button>
      </div>

      <CollapsibleContent>
        {attachments.length === 0 ? (
          <div className="p-3 border rounded-md border-dotted text-sm text-muted-foreground">
            {t("No attachments yet.")}
          </div>
        ) : (
          <div className="divide-y rounded-md border bg-card">
            {attachments.map((attachment, index) => (
              <AttachmentItem key={index} attachment={attachment} />
            ))}
          </div>
        )}
      </CollapsibleContent>
    </Collapsible>
  );
}
