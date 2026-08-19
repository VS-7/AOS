import * as React from "react";
import { Plus, Trash2 } from "lucide-react";
import { useFieldArray } from "react-hook-form";

import type { AosFormReturn } from "@/app/builders/types";
import { Button } from "@/components/ui/button";
import {
  FormControl,
  FormDescription,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from "@/components/ui/form";
import { Input } from "@/components/ui/input";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { Separator } from "@/components/ui/separator";
import { Switch } from "@/components/ui/switch";
import { Textarea } from "@/components/ui/textarea";
import { cn } from "@/lib/utils";
import {
  type CollectionJsonSchemaNode,
  type CollectionSchemaPathSegment,
  FormSchemaHelper,
} from "../helpers/form-schema.helper";
import { MarkdownEditor } from "@/components/ui/markdown-editor";

type SchemaFormProps = {
  form: AosFormReturn<any>;
  schema?: Record<string, unknown> | null;
  disabled?: boolean;
  rootName?: string;
};

type SchemaFormNodeRenderer = (
  fieldSchema: CollectionJsonSchemaNode,
  path: CollectionSchemaPathSegment[],
  name: string,
  label: string,
  required: boolean,
  options?: { arrayItem?: boolean; inlineObject?: boolean },
) => React.ReactNode;

export function SchemaForm({
  form,
  schema,
  disabled = false,
  rootName = "data",
}: SchemaFormProps) {
  const typedSchema = (schema ?? {}) as CollectionJsonSchemaNode;
  const rootProperties = typedSchema.properties ?? {};
  const rootRequired = new Set(typedSchema.required ?? []);

  const renderNode = React.useCallback<SchemaFormNodeRenderer>(
    (fieldSchema, path, name, label, required, options) => {
      const type = FormSchemaHelper.normalizeType(fieldSchema);
      const description = fieldSchema.description;

      if (type === "object") {
        const requiredSet = new Set(fieldSchema.required ?? []);
        const inlineObject = options?.inlineObject ?? false;

        if (inlineObject) {
          const properties = Object.entries(fieldSchema.properties ?? {});
          return (
            <div key={name} className="space-y-3">
              {properties.length > 0 ? (
                <div className="text-[0.65rem] font-semibold uppercase tracking-widest text-muted-foreground/70">
                  {label}
                </div>
              ) : null}
              <div className="space-y-4">
                {properties.map(([key, childSchema], index, entries) => (
                  <React.Fragment key={`${name}.${key}`}> 
                    {renderNode(
                      childSchema,
                      [...path, key],
                      `${name}.${key}`,
                      childSchema.title ?? FormSchemaHelper.prettifyLabel(key),
                      childSchema.optional === true ? false : requiredSet.has(key),
                      { inlineObject: true },
                    )}

                    {index < entries.length - 1 ? <Separator className="bg-border/60" /> : null}
                  </React.Fragment>
                ))}
              </div>
            </div>
          );
        }

        return (
          <section key={name} className="space-y-6">
            <div className="space-y-2">
              <p className="text-[0.68rem] font-medium uppercase tracking-[0.18em] text-muted-foreground/80">
                Section
              </p>
              <div className="space-y-1">
                <h3 className="text-lg font-semibold tracking-tight text-foreground">{label}</h3>
                {description ? (
                  <p className="max-w-2xl text-sm leading-6 text-muted-foreground">{description}</p>
                ) : null}
              </div>
            </div>

            <div className="space-y-5">
              {Object.entries(fieldSchema.properties ?? {}).map(([key, childSchema], index, entries) => (
                <React.Fragment key={`${name}.${key}`}> 
                  {renderNode(
                    childSchema,
                    [...path, key],
                    `${name}.${key}`,
                    childSchema.title ?? FormSchemaHelper.prettifyLabel(key),
                    childSchema.optional === true ? false : requiredSet.has(key),
                  )}

                  {index < entries.length - 1 ? <Separator className="bg-border/60" /> : null}
                </React.Fragment>
              ))}
            </div>
          </section>
        );
      }

      if (type === "array") {
        return (
          <SchemaArrayField
            key={name}
            form={form}
            fieldSchema={fieldSchema}
            name={name}
            path={path}
            label={label}
            required={required}
            disabled={disabled}
            renderNode={renderNode}
            options={options}
          />
        );
      }

      const isArrayItem = options?.arrayItem ?? false;

      if (fieldSchema.enum?.length) {
        return (
          <FormField
            key={name}
            control={form.control}
            name={name as never}
            render={({ field }) => (
              <FormItem className={isArrayItem ? "space-y-2" : "grid gap-3 sm:grid-cols-[minmax(0,190px)_minmax(0,1fr)] sm:gap-8"}>
                {isArrayItem ? (
                  <FormLabel className="sr-only">{label}</FormLabel>
                ) : (
                  <FieldMeta label={label} description={description} required={required} />
                )}

                <div className="space-y-2">
                  <FormControl>
                    <Select
                      disabled={disabled}
                      value={field.value === undefined ? "" : JSON.stringify(field.value)}
                      onValueChange={(nextValue) => field.onChange(JSON.parse(nextValue))}
                    >
                      <SelectTrigger className="h-11 w-full rounded-2xl border-border/70 bg-background/70 px-4 shadow-none">
                        <SelectValue placeholder={`Select ${label.toLowerCase()}`} />
                      </SelectTrigger>
                      <SelectContent>
                        {fieldSchema.enum?.map((option) => (
                          <SelectItem key={JSON.stringify(option)} value={JSON.stringify(option)}>
                            {String(option)}
                          </SelectItem>
                        ))}
                      </SelectContent>
                    </Select>
                  </FormControl>
                  <FormMessage className="text-xs" />
                </div>
              </FormItem>
            )}
          />
        );
      }

      if (type === "boolean") {
        return (
          <FormField
            key={name}
            control={form.control}
            name={name as never}
            render={({ field }) => (
              <FormItem className={isArrayItem ? "space-y-2" : "grid gap-4 sm:grid-cols-[minmax(0,190px)_minmax(0,1fr)] sm:items-center sm:gap-8"}>
                {isArrayItem ? (
                  <FormLabel className="sr-only">{label}</FormLabel>
                ) : (
                  <FieldMeta label={label} description={description} required={required} />
                )}

                <div className="flex items-center justify-between gap-3 rounded-[20px] border border-border/70 bg-background/70 px-4 py-3">
                  <div className="text-sm text-muted-foreground">
                    {field.value === true ? "Enabled" : "Disabled"}
                  </div>
                  <FormControl>
                    <Switch
                      checked={field.value === true}
                      disabled={disabled}
                      onCheckedChange={(checked) => field.onChange(checked === true)}
                    />
                  </FormControl>
                </div>
                <FormMessage className={isArrayItem ? "text-xs" : "text-xs sm:col-start-2"} />
              </FormItem>
            )}
          />
        );
      }

      if (type === "number" || type === "integer") {
        return (
          <FormField
            key={name}
            control={form.control}
            name={name as never}
            render={({ field }) => (
              <FormItem className={isArrayItem ? "space-y-2" : "grid gap-3 sm:grid-cols-[minmax(0,190px)_minmax(0,1fr)] sm:gap-8"}>
                {isArrayItem ? (
                  <FormLabel className="sr-only">{label}</FormLabel>
                ) : (
                  <FieldMeta label={label} description={description} required={required} />
                )}

                <div className="space-y-2">
                  <FormControl>
                    <Input
                      type="number"
                      value={typeof field.value === "number" ? String(field.value) : ""}
                      placeholder={description ?? `Enter ${label.toLowerCase()}`}
                      disabled={disabled}
                      className="h-11 rounded-2xl border-border/70 bg-background/70 px-4 shadow-none"
                      onChange={(event) => {
                        const nextValue = event.target.value;
                        field.onChange(nextValue === "" ? undefined : Number(nextValue));
                      }}
                    />
                  </FormControl>
                  <FormMessage className="text-xs" />
                </div>
              </FormItem>
            )}
          />
        );
      }

      return (
        <FormField
          key={name}
          control={form.control}
          name={name as never}
          render={({ field }) => {
            const currentValue = typeof field.value === "string" ? field.value : "";
            const shouldUseTextarea =
              fieldSchema.format === "textarea" ||
              fieldSchema.format === "markdown" ||
              currentValue.includes("\n");

            return (
              <FormItem className={isArrayItem ? "space-y-2" : "grid gap-3 sm:grid-cols-[minmax(0,190px)_minmax(0,1fr)] sm:gap-8"}>
                {isArrayItem ? (
                  <FormLabel className="sr-only">{label}</FormLabel>
                ) : (
                  <FieldMeta label={label} description={description} required={required} />
                )}

                <div className="space-y-2">
                  <FormControl>
                    {shouldUseTextarea ? (
                      <MarkdownEditor
                        value={currentValue}
                        placeholder={description ?? `Enter ${label.toLowerCase()}`}
                        disabled={disabled}
                        onValueChange={(value) => field.onChange(value)}
                        className="min-h-32 rounded-[24px] border-border/70 bg-background/70 px-4 py-3 shadow-none"
                      />
                    ) : (
                      <Input
                        value={currentValue}
                        placeholder={description ?? `Enter ${label.toLowerCase()}`}
                        disabled={disabled}
                        onChange={(event) => field.onChange(event.target.value)}
                        className="h-11 rounded-2xl border-border/70 bg-background/70 px-4 shadow-none"
                      />
                    )}
                  </FormControl>
                  <FormMessage className="text-xs" />
                </div>
              </FormItem>
            );
          }}
        />
      );
    },
    [disabled, form],
  );

  if (Object.keys(rootProperties).length === 0) {
    return (
      <div className="rounded-[28px] border border-dashed border-border/70 px-6 py-8 text-sm leading-6 text-muted-foreground">
        This collection schema has no declared properties yet.
      </div>
    );
  }

  return (
    <div className="space-y-10">
      {Object.entries(rootProperties).map(([key, fieldSchema], index, entries) => (
        <React.Fragment key={key}>
          {renderNode(
            fieldSchema,
            [key],
            `${rootName}.${key}`,
            fieldSchema.title ?? FormSchemaHelper.prettifyLabel(key),
            fieldSchema.optional === true ? false : rootRequired.has(key),
          )}

          {index < entries.length - 1 ? <Separator className="bg-border/60" /> : null}
        </React.Fragment>
      ))}
    </div>
  );
}

type SchemaArrayFieldProps = {
  form: AosFormReturn<any>;
  fieldSchema: CollectionJsonSchemaNode;
  name: string;
  path: CollectionSchemaPathSegment[];
  label: string;
  required: boolean;
  disabled: boolean;
  renderNode: SchemaFormNodeRenderer;
  options?: { arrayItem?: boolean; inlineObject?: boolean };
};

function SchemaArrayField({
  form,
  fieldSchema,
  name,
  path,
  label,
  required,
  disabled,
  renderNode,
  options,
}: SchemaArrayFieldProps) {
  const itemSchema = FormSchemaHelper.getArrayItemSchema(fieldSchema);
  const itemLabel = itemSchema.title ?? label;
  const { fields, append, remove } = useFieldArray({
    control: form.control,
    name: name as never,
  });

  const isNested = options?.arrayItem ?? false;
  const itemType = FormSchemaHelper.normalizeType(itemSchema);
  const isObjectItem = itemType === "object";

  return (
    <section className={cn("space-y-5", isNested && "space-y-4")}>
      <div className={cn(
        "flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between",
        isNested && "gap-2"
      )}>
        <div className="space-y-1">
          <div className="flex flex-wrap items-center gap-2">
            <h3 className={cn(
              "text-base font-semibold tracking-tight text-foreground",
              isNested && "text-sm font-medium tracking-normal"
            )}>
              {label}
            </h3>
            {required && !isNested ? (
              <span className="text-xs text-muted-foreground">Required list</span>
            ) : null}
          </div>
          {fieldSchema.description && !isNested ? (
            <p className="max-w-2xl text-sm leading-6 text-muted-foreground">
              {fieldSchema.description}
            </p>
          ) : null}
        </div>

        <Button
          type="button"
          variant="outline"
          size="sm"
          disabled={disabled}
          onClick={() => append(FormSchemaHelper.createEmptyValue(itemSchema) as never)}
        >
          <Plus />
          Add item
        </Button>
      </div>

      {fields.length === 0 ? (
        <div className={cn(
          "rounded-[24px] border border-dashed border-border/70 px-5 py-6 text-sm leading-6 text-muted-foreground",
          isNested && "py-4 text-xs"
        )}>
          No entries yet. Add the first {itemLabel.toLowerCase()} when you&apos;re ready.
        </div>
      ) : (
        <div className="space-y-3">
          {fields.map((field, index) => {
            return (
              <div
                key={field.id}
                className="group relative rounded-2xl border border-border/70 bg-background/70 p-4 shadow-sm transition-all hover:border-border/80"
              >
                <Button
                  type="button"
                  variant="ghost"
                  size="icon-sm"
                  className="absolute -right-1 -top-1 z-10 h-7 w-7 rounded-full bg-background/80 text-muted-foreground opacity-0 shadow-sm transition-all hover:bg-destructive/10 hover:text-destructive group-hover:opacity-100 focus-visible:opacity-100"
                  disabled={disabled}
                  onClick={() => remove(index)}
                >
                  <Trash2 className="size-3.5" />
                  <span className="sr-only">Remove {itemLabel} #{index + 1}</span>
                </Button>

                <div className="flex items-center gap-2 pr-8">
                  <div className="flex h-6 w-6 shrink-0 items-center justify-center rounded-full bg-muted text-[10px] font-mono font-medium text-muted-foreground ring-1 ring-border/50">
                    {index + 1}
                  </div>
                  <div className="text-sm font-medium text-foreground/90">{itemLabel}</div>
                </div>

                <div className={cn("mt-3", (isObjectItem || itemType === "array") && "space-y-3")}>
                  {isObjectItem ? (
                    <div className="space-y-4">
                      {Object.entries(itemSchema.properties ?? {}).map(([key, childSchema], childIndex, entries) => {
                        const childRequired =
                          childSchema.optional === true
                            ? false
                            : new Set(itemSchema.required ?? []).has(key);
                        const childLabel = childSchema.title ?? FormSchemaHelper.prettifyLabel(key);
                        const childOptions =
                          FormSchemaHelper.normalizeType(childSchema) === "object"
                            ? { inlineObject: true }
                            : undefined;

                        return (
                          <React.Fragment key={`${field.id}.${key}`}> 
                            {renderNode(
                              childSchema,
                              [...path, index, key],
                              `${name}.${index}.${key}`,
                              childLabel,
                              childRequired,
                              childOptions,
                            )}

                            {childIndex < entries.length - 1 ? <Separator className="bg-border/60" /> : null}
                          </React.Fragment>
                        );
                      })}
                    </div>
                  ) : (
                    renderNode(
                      itemSchema,
                      [...path, index],
                      `${name}.${index}`,
                      itemLabel,
                      true,
                      { arrayItem: true },
                    )
                  )}
                </div>
              </div>
            );
          })}
        </div>
      )}
    </section>
  );
}

function FieldMeta({
  label,
  description,
  required,
}: {
  label: string;
  description?: string;
  required: boolean;
}) {
  return (
    <div className="space-y-1 pt-1">
      <FormLabel className="text-[0.95rem] font-medium text-foreground">
        {label}
        {required ? <span className="text-muted-foreground"> *</span> : null}
      </FormLabel>
      {description ? (
        <FormDescription className="max-w-xs text-[0.82rem] leading-5 text-muted-foreground">
          {description}
        </FormDescription>
      ) : null}
    </div>
  );
}
