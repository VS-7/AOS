import * as React from "react";
import {
  JsonEditor,
  type JsonSchema,
  type JsonValue,
} from "@visual-json/react";
import { resolveSchema } from "@visual-json/core";
import { AlertTriangle } from "lucide-react";

import { cn } from "@/lib/utils";
import { FilesTextViewer } from "./files-text-viewer.component";
import type { FractalFile } from "@/features/file/interfaces/file.interfaces";

interface FilesJsonViewerProps {
  content: string;
  file: FractalFile;
  readOnly?: boolean;
  onChange: (value: string) => void;
}

type ParseResult =
  | { ok: true; value: JsonValue }
  | { ok: false; error: string };

function parseJsonContent(content: string): ParseResult {
  const trimmed = content.trim();
  if (!trimmed) {
    return { ok: true, value: {} };
  }

  try {
    return { ok: true, value: JSON.parse(trimmed) as JsonValue };
  } catch (error) {
    return {
      ok: false,
      error: error instanceof Error ? error.message : "Invalid JSON",
    };
  }
}

function serializeJson(value: JsonValue, previousContent: string): string {
  const body = JSON.stringify(value, null, 2);
  return previousContent.endsWith("\n") ? `${body}\n` : body;
}

const VISUAL_JSON_THEME = {
  "--vj-bg": "var(--background)",
  "--vj-bg-panel": "var(--background)",
  "--vj-bg-hover": "var(--muted)",
  "--vj-bg-selected": "var(--accent)",
  "--vj-bg-selected-muted": "var(--muted)",
  "--vj-bg-match": "var(--muted)",
  "--vj-bg-match-active": "var(--accent)",
  "--vj-text": "var(--foreground)",
  "--vj-text-muted": "var(--muted-foreground)",
  "--vj-text-dim": "var(--muted-foreground)",
  "--vj-text-dimmer": "var(--muted-foreground)",
  "--vj-text-selected": "var(--accent-foreground)",
  "--vj-border": "var(--border)",
  "--vj-border-subtle": "var(--border)",
  "--vj-accent": "var(--primary)",
  "--vj-accent-muted": "var(--secondary)",
  "--vj-input-bg": "var(--background)",
  "--vj-input-border": "var(--input)",
  "--vj-input-font-size": "12px",
  "--vj-font": "var(--font-mono, ui-monospace, SFMono-Regular, Menlo, monospace)",
  "--vj-string": "oklch(0.62 0.12 45)",
  "--vj-number": "oklch(0.68 0.1 145)",
  "--vj-boolean": "oklch(0.65 0.12 250)",
  "--vj-error": "var(--destructive)",
} as React.CSSProperties;

export function FilesJsonViewer({
  content,
  file,
  readOnly = false,
  onChange,
}: FilesJsonViewerProps) {
  const [parseError, setParseError] = React.useState<string | null>(() => {
    const parsed = parseJsonContent(content);
    return parsed.ok ? null : parsed.error;
  });
  const [value, setValue] = React.useState<JsonValue>(() => {
    const parsed = parseJsonContent(content);
    return parsed.ok ? parsed.value : {};
  });
  const [schema, setSchema] = React.useState<JsonSchema | null>(null);
  const [forceRaw, setForceRaw] = React.useState(() => {
    const parsed = parseJsonContent(content);
    return !parsed.ok;
  });
  const lastSerializedRef = React.useRef(content);

  React.useEffect(() => {
    if (content === lastSerializedRef.current) return;

    lastSerializedRef.current = content;
    const parsed = parseJsonContent(content);

    if (!parsed.ok) {
      setParseError(parsed.error);
      setForceRaw(true);
      return;
    }

    setParseError(null);
    setForceRaw(false);
    setValue(parsed.value);
  }, [content]);

  React.useEffect(() => {
    const parsed = parseJsonContent(content);
    if (!parsed.ok) {
      setSchema(null);
      return;
    }

    let cancelled = false;

    void resolveSchema(parsed.value, file.name).then((nextSchema: any) => {
      if (!cancelled) {
        setSchema(nextSchema);
      }
    });

    return () => {
      cancelled = true;
    };
  }, [content, file.name]);

  const handleChange = React.useCallback(
    (nextValue: JsonValue) => {
      setValue(nextValue);

      const serialized = serializeJson(nextValue, lastSerializedRef.current);
      if (serialized === lastSerializedRef.current) return;

      lastSerializedRef.current = serialized;
      onChange(serialized);
    },
    [onChange],
  );

  if (forceRaw || parseError) {
    return (
      <div className="flex h-full min-h-0 flex-col">
        <div className="flex items-start gap-2 border-b border-border/60 bg-destructive/5 px-4 py-3 text-xs text-destructive">
          <AlertTriangle className="mt-0.5 size-3.5 shrink-0" />
          <div className="min-w-0 flex-1">
            <p className="font-medium">Invalid JSON — editing as text</p>
            <p className="mt-0.5 text-destructive/80">
              {parseError ?? "Could not parse this file as JSON."}
            </p>
          </div>
        </div>
        <div className="min-h-0 flex-1">
          <FilesTextViewer
            content={content}
            file={file}
            readOnly={readOnly}
            onChange={onChange}
          />
        </div>
      </div>
    );
  }

  return (
    <div className={cn("h-full min-h-0 overflow-hidden")}>
      <JsonEditor
        value={value}
        onChange={handleChange}
        schema={schema}
        readOnly={readOnly}
        height="100%"
        width="100%"
        treeShowValues
        editorShowDescriptions
        style={VISUAL_JSON_THEME}
      />
    </div>
  );
}
