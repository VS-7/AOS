"use client";

import * as React from "react";
import YAML from "yaml";
import {
  AlignLeft,
  Calendar,
  CalendarClock,
  CheckSquare,
  Hash,
  Plus,
  Tags,
  X,
} from "lucide-react";
import { MarkdownPlugin } from "@platejs/markdown";
import { Plate, usePlateEditor } from "platejs/react";

import { AutoformatKit } from "@/components/editor/plugins/autoformat-kit";
import { BasicBlocksKit } from "@/components/editor/plugins/basic-blocks-kit";
import { BasicMarksKit } from "@/components/editor/plugins/basic-marks-kit";
import { CodeBlockKit } from "@/components/editor/plugins/code-block-kit";
import { FloatingToolbarKit } from "@/components/editor/plugins/floating-toolbar-kit";
import { LinkKit } from "@/components/editor/plugins/link-kit";
import { ListKit } from "@/components/editor/plugins/list-kit";
import { MarkdownKit } from "@/components/editor/plugins/markdown-kit";
import { MediaKit } from "@/components/editor/plugins/media-kit";
import { SlashKit } from "@/components/editor/plugins/slash-kit";
import { TableKit } from "@/components/editor/plugins/table-kit";
import { Editor } from "@/components/ui/editor";
import { Input } from "@/components/ui/input";
import { Switch } from "@/components/ui/switch";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuSub,
  DropdownMenuSubContent,
  DropdownMenuSubTrigger,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { cn } from "@/lib/utils";
import { t } from "@/lib/i18n";

// ---------------------------------------------------------------------------
// Front matter — types & parsing
// ---------------------------------------------------------------------------

type FrontMatterFieldType =
  | "text"
  | "number"
  | "checkbox"
  | "tags"
  | "date"
  | "datetime";

interface FrontMatterField {
  id: string;
  key: string;
  type: FrontMatterFieldType;
  value: string | number | boolean | string[];
}

interface ParsedFrontMatter {
  hasFrontMatter: boolean;
  parseError: boolean;
  body: string;
  fields: FrontMatterField[];
}

const FRONT_MATTER_PATTERN = /^---\r?\n([\s\S]*?)\r?\n---\r?\n?([\s\S]*)$/;

const FIELD_TYPE_OPTIONS: Array<{
  type: FrontMatterFieldType;
  label: string;
  icon: React.ComponentType<{ className?: string }>;
}> = [
  { type: "text", label: "Text", icon: AlignLeft },
  { type: "number", label: "Number", icon: Hash },
  { type: "checkbox", label: "Checkbox", icon: CheckSquare },
  { type: "tags", label: "Tags", icon: Tags },
  { type: "date", label: "Date", icon: Calendar },
  { type: "datetime", label: "Date with time", icon: CalendarClock },
];

function inferFieldType(value: unknown): FrontMatterFieldType {
  if (typeof value === "boolean") return "checkbox";
  if (typeof value === "number") return "number";
  if (Array.isArray(value)) return "tags";

  if (typeof value === "string") {
    if (/^\d{4}-\d{2}-\d{2}$/.test(value)) return "date";
    if (/^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}/.test(value)) return "datetime";
    return "text";
  }

  return "text";
}

function normalizeFieldValue(
  type: FrontMatterFieldType,
  value: unknown,
): FrontMatterField["value"] {
  switch (type) {
    case "checkbox":
      return Boolean(value);
    case "number":
      return typeof value === "number" ? value : Number(value) || 0;
    case "tags":
      if (Array.isArray(value)) {
        return value.map((item) => String(item));
      }
      if (typeof value === "string" && value.trim()) {
        return [value];
      }
      return [];
    case "date":
    case "datetime":
    case "text":
      if (value === null || value === undefined) return "";
      if (typeof value === "object") return JSON.stringify(value, null, 2);
      return String(value);
  }
}

function objectToFields(data: Record<string, unknown>): FrontMatterField[] {
  return Object.entries(data).map(([key, value]) => {
    const type = inferFieldType(value);
    return {
      id: key,
      key,
      type,
      value: normalizeFieldValue(type, value),
    };
  });
}

function fieldToYamlValue(field: FrontMatterField): unknown {
  switch (field.type) {
    case "checkbox":
      return Boolean(field.value);
    case "number":
      return typeof field.value === "number"
        ? field.value
        : Number(field.value) || 0;
    case "tags":
      return Array.isArray(field.value)
        ? field.value.map((item) => String(item).trim()).filter(Boolean)
        : [];
    case "date":
    case "datetime":
    case "text":
      return String(field.value ?? "");
  }
}

function fieldsToObject(fields: FrontMatterField[]): Record<string, unknown> {
  const result: Record<string, unknown> = {};

  for (const field of fields) {
    const key = field.key.trim();
    if (!key) continue;
    result[key] = fieldToYamlValue(field);
  }

  return result;
}

function parseFrontMatter(markdown: string): ParsedFrontMatter {
  const match = markdown.match(FRONT_MATTER_PATTERN);

  if (!match) {
    return {
      hasFrontMatter: false,
      parseError: false,
      body: markdown,
      fields: [],
    };
  }

  try {
    const parsed = YAML.parse(match[1] ?? "");

    if (!parsed || typeof parsed !== "object" || Array.isArray(parsed)) {
      return {
        hasFrontMatter: false,
        parseError: false,
        body: markdown,
        fields: [],
      };
    }

    return {
      hasFrontMatter: true,
      parseError: false,
      body: match[2] ?? "",
      fields: objectToFields(parsed as Record<string, unknown>),
    };
  } catch {
    return {
      hasFrontMatter: true,
      parseError: true,
      body: match[2] ?? "",
      fields: [],
    };
  }
}

function composeMarkdown(fields: FrontMatterField[], body: string): string {
  const data = fieldsToObject(fields);
  const yaml = YAML.stringify(data).trim();

  if (!yaml) {
    return body;
  }

  const normalizedBody = body.replace(/^\n/, "");
  return `---\n${yaml}\n---\n${normalizedBody}`;
}

function createUniquePropertyKey(fields: FrontMatterField[]): string {
  const base = "property";
  if (!fields.some((field) => field.key === base)) return base;

  let index = 2;
  while (fields.some((field) => field.key === `${base}_${index}`)) {
    index += 1;
  }

  return `${base}_${index}`;
}

function coerceFieldType(
  field: FrontMatterField,
  nextType: FrontMatterFieldType,
): FrontMatterField {
  if (field.type === nextType) return field;

  if (nextType === "tags") {
    const raw = Array.isArray(field.value)
      ? field.value
      : String(field.value ?? "")
          .split(",")
          .map((item) => item.trim())
          .filter(Boolean);

    return { ...field, type: nextType, value: raw };
  }

  if (nextType === "checkbox") {
    return {
      ...field,
      type: nextType,
      value: Boolean(field.value),
    };
  }

  if (nextType === "number") {
    return {
      ...field,
      type: nextType,
      value: Number(field.value) || 0,
    };
  }

  return {
    ...field,
    type: nextType,
    value: normalizeFieldValue(nextType, field.value),
  };
}

// ---------------------------------------------------------------------------
// Front matter — properties editor UI
// ---------------------------------------------------------------------------

interface FrontMatterPropertiesEditorProps {
  fields: FrontMatterField[];
  disabled?: boolean;
  panelEnabled?: boolean;
  onTogglePanel?: (enabled: boolean) => void;
  onChange: (fields: FrontMatterField[]) => void;
}

function FieldTypeIcon({
  type,
  className,
}: {
  type: FrontMatterFieldType;
  className?: string;
}) {
  const option = FIELD_TYPE_OPTIONS.find((item) => item.type === type);
  const Icon = option?.icon ?? AlignLeft;
  return <Icon className={className} />;
}

function FrontMatterPropertyValue({
  field,
  disabled,
  onChange,
}: {
  field: FrontMatterField;
  disabled?: boolean;
  onChange: (value: FrontMatterField["value"]) => void;
}) {
  if (field.type === "checkbox") {
    return (
      <Switch
        checked={Boolean(field.value)}
        disabled={disabled}
        onCheckedChange={(checked) => onChange(checked)}
      />
    );
  }

  if (field.type === "tags") {
    const tags = Array.isArray(field.value) ? field.value : [];

    return (
      <div className="flex min-h-8 flex-wrap items-center gap-1.5">
        {tags.map((tag, index) => (
          <span
            key={`${field.id}-${tag}-${index}`}
            className="h-7 inline-flex max-w-full items-center gap-1 rounded-md focus:bg-muted/60 bg-muted px-2 text-xs text-foreground"
          >
            <span className="truncate">{tag}</span>
            {!disabled ? (
              <button
                type="button"
                className="rounded-sm text-muted-foreground hover:text-foreground"
                onClick={() =>
                  onChange(tags.filter((_, tagIndex) => tagIndex !== index))
                }
              >
                <X className="size-3" />
              </button>
            ) : null}
          </span>
        ))}

        {!disabled ? (
          <Input
            placeholder={t("Add...")}
            className="h-7 min-w-24 flex-1 border-0 focus:bg-muted/60 bg-transparent px-2 text-xs shadow-none focus-visible:ring-1 focus-visible:ring-ring/10"
            onKeyDown={(event) => {
              if (event.key !== "Enter") return;
              event.preventDefault();

              const next = event.currentTarget.value.trim();
              if (!next || tags.includes(next)) {
                event.currentTarget.value = "";
                return;
              }

              onChange([...tags, next]);
              event.currentTarget.value = "";
            }}
          />
        ) : null}
      </div>
    );
  }

  if (field.type === "text" && String(field.value ?? "").includes("\n")) {
    return (
      <textarea
        value={String(field.value ?? "")}
        disabled={disabled}
        rows={3}
        onChange={(event) => onChange(event.target.value)}
        className={cn(
          "w-full resize-y rounded-md border-0 focus:bg-muted/60 bg-transparent px-2 text-sm leading-relaxed text-foreground outline-none",
          "focus-visible:ring-1 focus-visible:ring-ring/10",
          disabled && "opacity-80",
        )}
      />
    );
  }

  return (
    <Input
      type={
        field.type === "number"
          ? "number"
          : field.type === "date"
            ? "date"
            : field.type === "datetime"
              ? "datetime-local"
              : "text"
      }
      value={String(field.value ?? "")}
      disabled={disabled}
      onChange={(event) =>
        onChange(
          field.type === "number"
            ? Number(event.target.value)
            : event.target.value,
        )
      }
      className="h-8 rounded-md border-0 focus:bg-muted/60 px-2 text-xs shadow-none focus-visible:ring-1 focus-visible:ring-ring/10"
    />
  );
}

function FrontMatterPropertyRow({
  field,
  disabled,
  onChange,
  onRemove,
}: {
  field: FrontMatterField;
  disabled?: boolean;
  onChange: (field: FrontMatterField) => void;
  onRemove: () => void;
}) {
  return (
    <div className="group grid grid-cols-[auto_7.5rem_minmax(0,1fr)] items-start gap-3 py-1.5">
      <DropdownMenu>
        <DropdownMenuTrigger asChild>
          <button
            type="button"
            disabled={disabled}
            className="mt-0.5 flex size-7 items-center justify-center rounded-md text-muted-foreground hover:bg-muted disabled:opacity-50"
          >
            <FieldTypeIcon type={field.type} className="size-3" />
          </button>
        </DropdownMenuTrigger>
        <DropdownMenuContent align="start" className="w-48">
          <DropdownMenuSub>
            <DropdownMenuSubTrigger>{t("Change type")}</DropdownMenuSubTrigger>
            <DropdownMenuSubContent>
              {FIELD_TYPE_OPTIONS.map((option) => {
                const Icon = option.icon;
                return (
                  <DropdownMenuItem
                    key={option.type}
                    onClick={() =>
                      onChange(coerceFieldType(field, option.type))
                    }
                  >
                    <Icon />
                    {option.label}
                  </DropdownMenuItem>
                );
              })}
            </DropdownMenuSubContent>
          </DropdownMenuSub>
          <DropdownMenuSeparator />
          <DropdownMenuItem
            variant="destructive"
            onClick={onRemove}
          >
            {t("Remove")}
          </DropdownMenuItem>
        </DropdownMenuContent>
      </DropdownMenu>

      <Input
        value={field.key}
        disabled={disabled}
        onChange={(event) =>
          onChange({
            ...field,
            key: event.target.value,
          })
        }
        className="h-7 rounded-md border-0 focus:bg-muted/60 px-2 text-xs shadow-none focus-visible:ring-1 focus-visible:ring-ring/10"
      />

      <FrontMatterPropertyValue
        field={field}
        disabled={disabled}
        onChange={(value) => onChange({ ...field, value })}
      />
    </div>
  );
}

function FrontMatterPropertiesEditor({
  fields,
  disabled,
  panelEnabled: _panelEnabled = true,
  onTogglePanel: _onTogglePanel,
  onChange,
}: FrontMatterPropertiesEditorProps) {
  function updateField(index: number, nextField: FrontMatterField) {
    const next = fields.map((field, fieldIndex) =>
      fieldIndex === index ? nextField : field,
    );
    onChange(next);
  }

  function removeField(index: number) {
    onChange(fields.filter((_, fieldIndex) => fieldIndex !== index));
  }

  function addField() {
    const key = createUniquePropertyKey(fields);
    onChange([
      ...fields,
      {
        id: `${key}-${Date.now()}`,
        key,
        type: "text",
        value: "",
      },
    ]);
  }

  return (
    <div className="border-b border-border/40 pb-4">
      <div className="mb-3 flex items-center justify-between gap-3">
        <p className="text-xs text-muted-foreground">{t("Properties")}</p>
      </div>

      <div className="flex flex-col">
        {fields.map((field, index) => (
          <FrontMatterPropertyRow
            key={field.id}
            field={field}
            disabled={disabled}
            onChange={(nextField) => updateField(index, nextField)}
            onRemove={() => removeField(index)}
          />
        ))}
      </div>

      {!disabled ? (
        <button
          type="button"
          onClick={addField}
          className="mt-2 inline-flex items-center ml-2 gap-1.5 text-xs text-muted-foreground hover:text-foreground"
        >
          <Plus className="size-3.5" />
          {t("Add property")}
        </button>
      ) : null}
    </div>
  );
}

// ---------------------------------------------------------------------------
// Markdown editor
// ---------------------------------------------------------------------------

interface MarkdownEditorProps {
  value: string;
  onValueChange: (value: string) => void;
  title?: string;
  placeholder?: string;
  disabled?: boolean;
  className?: string;
  editorClassName?: string;
  /** When true, renders a Notion-style front-matter panel when YAML front-matter is detected. Default: false. */
  frontMatterEditor?: boolean;
}

export function MarkdownEditor({
  value,
  onValueChange,
  title,
  placeholder = "Write markdown here...",
  disabled,
  className,
  editorClassName,
  frontMatterEditor = false,
}: MarkdownEditorProps) {
  const parsed = React.useMemo(() => parseFrontMatter(value), [value]);
  const canShowPropertiesPanel =
    frontMatterEditor && parsed.hasFrontMatter && !parsed.parseError;

  const [propertiesPanelEnabled, setPropertiesPanelEnabled] = React.useState(true);
  const [fields, setFields] = React.useState<FrontMatterField[]>(parsed.fields);
  const bodyRef = React.useRef(parsed.body);

  React.useEffect(() => {
    setFields(parsed.fields);
    bodyRef.current = parsed.body;
  }, [parsed.fields, parsed.body, value]);

  const editorMarkdown =
    canShowPropertiesPanel && propertiesPanelEnabled ? parsed.body : value;

  const editor = usePlateEditor(
    {
      id: "markdown-editor",
      plugins: [
        ...AutoformatKit,
        ...BasicBlocksKit,
        ...BasicMarksKit,
        ...CodeBlockKit,
        ...FloatingToolbarKit,
        ...LinkKit,
        ...ListKit,
        ...MarkdownKit,
        ...MediaKit,
        ...SlashKit,
        ...TableKit,
      ],
      value: [],
    },
    [],
  );

  const hasSyncedInitialValueRef = React.useRef(false);
  const isSyncingRef = React.useRef(false);
  const lastValueRef = React.useRef(value);
  const lastEditorMarkdownRef = React.useRef(editorMarkdown);
  const lastSerializedBodyRef = React.useRef<string | null>(null);

  const publishValue = React.useCallback(
    (nextValue: string) => {
      if (nextValue === lastValueRef.current) return;
      lastValueRef.current = nextValue;
      onValueChange(nextValue);
    },
    [onValueChange],
  );

  const syncEditorNodes = React.useCallback(
    (markdown: string) => {
      const nextNodes = markdown.trim()
        ? editor.getApi(MarkdownPlugin).markdown.deserialize(markdown)
        : [];

      // Plate fires onChange for programmatic replaceNodes. Suppress so the
      // deserialize→serialize round-trip does not look like a user edit.
      isSyncingRef.current = true;
      try {
        editor.tf.replaceNodes(nextNodes, {
          at: [],
          children: true,
        });
      } finally {
        isSyncingRef.current = false;
      }

      // Baseline after round-trip — catches deferred onChange from Plate/React.
      lastSerializedBodyRef.current = editor
        .getApi(MarkdownPlugin)
        .markdown.serialize();
    },
    [editor],
  );

  React.useEffect(() => {
    if (!hasSyncedInitialValueRef.current) {
      hasSyncedInitialValueRef.current = true;
      lastValueRef.current = value;
      lastEditorMarkdownRef.current = editorMarkdown;
      syncEditorNodes(editorMarkdown);
      return;
    }

    if (
      value === lastValueRef.current &&
      editorMarkdown === lastEditorMarkdownRef.current
    ) {
      return;
    }

    lastValueRef.current = value;
    lastEditorMarkdownRef.current = editorMarkdown;
    syncEditorNodes(editorMarkdown);
  }, [editorMarkdown, syncEditorNodes, value]);

  const handleEditorChange = React.useCallback(() => {
    if (isSyncingRef.current) return;

    const serializedBody = editor.getApi(MarkdownPlugin).markdown.serialize();

    // Ignore no-op / round-trip echoes (including deferred post-sync onChange).
    if (serializedBody === lastSerializedBodyRef.current) return;
    lastSerializedBodyRef.current = serializedBody;

    if (canShowPropertiesPanel && propertiesPanelEnabled) {
      bodyRef.current = serializedBody;
      publishValue(composeMarkdown(fields, serializedBody));
      return;
    }

    publishValue(serializedBody);
  }, [
    canShowPropertiesPanel,
    editor,
    fields,
    propertiesPanelEnabled,
    publishValue,
  ]);

  const handleFieldsChange = React.useCallback(
    (nextFields: FrontMatterField[]) => {
      setFields(nextFields);
      publishValue(composeMarkdown(nextFields, bodyRef.current));
    },
    [publishValue],
  );

  const editorClassNames = cn(
    "min-h-64 bg-transparent p-0 rounded-none shadow-none focus-visible:ring-0",
    "text-sm text-foreground font-sans",
    "[&_h1]:text-[16px] md:[&_h1]:text-[22px] [&_h1]:font-semibold [&_h1]:my-5 [&_h1]:text-foreground [&_h1]:tracking-tight",
    "[&_h2]:text-[15px] md:[&_h2]:text-[18px] [&_h2]:font-semibold [&_h2]:my-4 [&_h2]:text-foreground [&_h2]:tracking-tight",
    "[&_h3]:text-[14px] md:[&_h3]:text-[16px] [&_h3]:font-semibold [&_h3]:my-3.5 [&_h3]:text-foreground [&_h3]:tracking-tight",
    "[&_h4]:text-[13px] md:[&_h4]:text-[15px] [&_h4]:font-medium [&_h4]:my-3 [&_h4]:text-foreground [&_h4]:tracking-tight",
    "[&_h5]:text-[12px] md:[&_h5]:text-[14px] [&_h5]:font-medium [&_h5]:my-3 [&_h5]:text-foreground [&_h5]:tracking-tight",
    "[&_h6]:text-[12px] md:[&_h6]:text-[13px] [&_h6]:font-medium [&_h6]:my-3 [&_h6]:text-foreground [&_h6]:tracking-tight",
    "[&_p]:text-sm [&_p]:leading-[1.75] [&_p]:text-foreground/85 [&_p]:mt-0 [&_p]:mb-1.5 last:[&_p]:mb-0",
    "[&_blockquote]:my-3 [&_blockquote]:border-l-2 [&_blockquote]:border-border/40 [&_blockquote]:pl-3 [&_blockquote]:text-xs [&_blockquote]:leading-relaxed [&_blockquote]:text-muted-foreground",
    "[&_strong]:font-medium [&_strong]:text-foreground",
    "[&_em]:italic [&_em]:text-foreground/95",
    "[&_code]:text-sm [&_code]:font-mono [&_code]:bg-muted [&_code]:px-1.5 [&_code]:py-0.5 [&_code]:rounded-md",
    "[&_pre]:bg-muted [&_pre]:p-4 [&_pre]:rounded-md [&_pre]:overflow-x-auto [&_pre]:my-4",
    "[&_pre_code]:bg-transparent [&_pre_code]:p-0 [&_pre_code]:rounded-none",
    "[&_table]:w-full [&_table]:border-collapse [&_table]:my-4",
    "[&_th]:text-sm [&_th]:font-semibold [&_th]:p-2 [&_th]:border [&_th]:border-border/10 [&_th]:bg-muted/50",
    "[&_td]:text-sm [&_td]:p-2 [&_td]:border [&_td]:border-border/10",
    "[&_hr]:my-6 [&_hr]:border-border/20",
    disabled && "pointer-events-none opacity-80",
    className,
    editorClassName,
  );

  return (
    <div className="flex flex-col gap-4">
      {canShowPropertiesPanel && !propertiesPanelEnabled ? (
        <div className="flex items-center justify-end gap-2 border-b border-border/40 pb-3">
          <span className="text-xs text-muted-foreground">{t("Properties panel")}</span>
          <Switch
            checked={propertiesPanelEnabled}
            disabled={disabled}
            onCheckedChange={setPropertiesPanelEnabled}
          />
        </div>
      ) : null}

      {canShowPropertiesPanel && propertiesPanelEnabled ? (
        <FrontMatterPropertiesEditor
          fields={fields}
          disabled={disabled}
          panelEnabled={propertiesPanelEnabled}
          onTogglePanel={setPropertiesPanelEnabled}
          onChange={handleFieldsChange}
        />
      ) : null}

      {title ? (
        <p className="text-sm font-medium text-foreground">{title}</p>
      ) : null}

      <Plate readOnly={disabled} editor={editor} onChange={handleEditorChange}>
        <Editor
          variant="none"
          placeholder={placeholder}
          className={editorClassNames}
        />
      </Plate>
    </div>
  );
}
