import * as React from "react";

import { MarkdownEditor } from "@/components/ui/markdown-editor";
import { t } from "@/lib/i18n";

interface FilesMarkdownViewerProps {
  content: string;
  readOnly?: boolean;
  onChange: (value: string) => void;
}

export function FilesMarkdownViewer({
  content,
  readOnly = false,
  onChange,
}: FilesMarkdownViewerProps) {
  return (
    <div className="mx-auto w-full max-w-3xl px-6 py-6">
      <MarkdownEditor
        value={content}
        onValueChange={onChange}
        disabled={readOnly}
        frontMatterEditor
        placeholder={t("Write markdown...")}
      />
    </div>
  );
}
