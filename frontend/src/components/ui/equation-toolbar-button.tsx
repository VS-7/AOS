'use client';

import * as React from 'react';

import { insertInlineEquation } from '@platejs/math';
import { RadicalIcon } from 'lucide-react';
import { useEditorRef } from 'platejs/react';

import { ToolbarButton } from './toolbar';
import { t } from "@/lib/i18n";

export function InlineEquationToolbarButton(
  props: React.ComponentProps<typeof ToolbarButton>
) {
  const editor = useEditorRef();

  return (
    <ToolbarButton
      {...props}
      onClick={() => {
        insertInlineEquation(editor);
      }}
      tooltip={t("Mark as equation")}
    >
      <RadicalIcon />
    </ToolbarButton>
  );
}
