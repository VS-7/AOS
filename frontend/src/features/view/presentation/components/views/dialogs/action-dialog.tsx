import * as React from "react";
import { X } from "lucide-react";
import { cn } from "@/lib/utils";
import { FormSchemaHelper } from "../../../helpers/form-schema.helper";
import type { ViewActionParams, ViewActionParamProperty } from "../../../helpers";

export type ActionDialogProps = {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  actionId: string;
  actionDescription?: string;
  params?: ViewActionParams;
  onSubmit: (values: Record<string, unknown>) => void;
  isSubmitting?: boolean;
};

export function ActionDialog({
  open,
  onOpenChange,
  actionId,
  actionDescription,
  params,
  onSubmit,
  isSubmitting = false,
}: ActionDialogProps) {
  const [values, setValues] = React.useState<Record<string, unknown>>({});
  const [errors, setErrors] = React.useState<Record<string, string>>({});

  React.useEffect(() => {
    if (open) {
      setValues({});
      setErrors({});

      if (params?.properties) {
        const initial: Record<string, unknown> = {};
        for (const [key, prop] of Object.entries(params.properties)) {
          if (prop.default !== undefined) {
            initial[key] = prop.default;
          }
        }
        setValues(initial);
      }
    }
  }, [open, params?.properties]);

  const handleSubmit = React.useCallback(
    (e: React.FormEvent) => {
      e.preventDefault();

      if (!params) {
        onSubmit(values);
        return;
      }

      const validationErrors = FormSchemaHelper.validateValue(
        {
          type: "object",
          properties: params.properties ?? {},
          required: params.required,
        } as Record<string, unknown>,
        values,
      );

      if (Object.keys(validationErrors).length > 0) {
        setErrors(validationErrors);
        return;
      }

      onSubmit(values);
    },
    [params, values, onSubmit],
  );

  const requiredFields = React.useMemo(
    () => new Set(params?.required ?? []),
    [params?.required],
  );

  if (!open) return null;

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center">
      <div
        className="fixed inset-0 bg-black/50"
        onClick={() => onOpenChange(false)}
      />
      <div className="relative z-50 w-full max-w-lg rounded-lg bg-background p-6 shadow-lg">
        <div className="space-y-4">
          <div className="space-y-1.5">
            <h2 className="text-lg font-semibold leading-none tracking-tight capitalize">
              {actionId.replace(/-/g, " ")}
            </h2>
            {actionDescription && (
              <p className="text-sm text-muted-foreground">{actionDescription}</p>
            )}
          </div>

          <form onSubmit={handleSubmit} className="space-y-4">
            {params?.properties && (
              <div className="space-y-4">
                {Object.entries(params.properties).map(([key, prop]) => (
                  <FormField
                    key={key}
                    name={key}
                    label={FormSchemaHelper.prettifyLabel(key)}
                    required={requiredFields.has(key)}
                    error={errors[key]}
                  >
                    <FieldInput
                      name={key}
                      prop={prop}
                      value={values[key]}
                      onChange={(val) =>
                        setValues((prev) => ({ ...prev, [key]: val }))
                      }
                    />
                  </FormField>
                ))}
              </div>
            )}

            <div className="flex justify-end gap-2 pt-4">
              <button
                type="button"
                onClick={() => onOpenChange(false)}
                disabled={isSubmitting}
                className="inline-flex items-center justify-center rounded-md text-sm font-medium ring-offset-background transition-colors hover:bg-accent hover:text-accent-foreground h-10 px-4 py-2 border border-input bg-background"
              >
                Cancel
              </button>
              <button
                type="submit"
                disabled={isSubmitting}
                className="inline-flex items-center justify-center rounded-md text-sm font-medium ring-offset-background transition-colors bg-primary text-primary-foreground hover:bg-primary/90 h-10 px-4 py-2"
              >
                {isSubmitting ? "Submitting..." : "Submit"}
              </button>
            </div>
          </form>
        </div>
      </div>
    </div>
  );
}

type FieldInputProps = {
  name: string;
  prop: ViewActionParamProperty;
  value: unknown;
  onChange: (value: unknown) => void;
};

function FieldInput({ name, prop, value, onChange }: FieldInputProps) {
  const inputId = `field-${name}`;

  if (prop.enum) {
    return (
      <select
        id={inputId}
        value={String(value ?? "")}
        onChange={(e) => onChange(e.target.value)}
        className="flex h-10 w-full rounded-md border border-input bg-background px-3 py-2 text-sm ring-offset-background placeholder:text-muted-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 disabled:cursor-not-allowed disabled:opacity-50"
      >
        <option value="">Select an option</option>
        {prop.enum.map((option: unknown) => (
          <option key={String(option)} value={String(option)}>
            {String(option)}
          </option>
        ))}
      </select>
    );
  }

  switch (prop.type) {
    case "number":
    case "integer":
      return (
        <input
          id={inputId}
          type="number"
          value={value != null ? String(value) : ""}
          onChange={(e) =>
            onChange(e.target.value ? Number(e.target.value) : undefined)
          }
          min={prop.minimum}
          max={prop.maximum}
          placeholder={prop.description}
          className="flex h-10 w-full rounded-md border border-input bg-background px-3 py-2 text-sm ring-offset-background placeholder:text-muted-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 disabled:cursor-not-allowed disabled:opacity-50"
        />
      );

    case "boolean":
      return (
        <input
          id={inputId}
          type="checkbox"
          checked={Boolean(value)}
          onChange={(e) => onChange(e.target.checked)}
          className="h-4 w-4 rounded border-gray-300 text-primary focus:ring-2 focus:ring-primary"
        />
      );

    case "array":
      return (
        <input
          id={inputId}
          type="text"
          value={Array.isArray(value) ? value.join(", ") : ""}
          onChange={(e) =>
            onChange(
              e.target.value
                ? e.target.value.split(",").map((s: string) => s.trim())
                : [],
            )
          }
          placeholder="Comma-separated values"
          className="flex h-10 w-full rounded-md border border-input bg-background px-3 py-2 text-sm ring-offset-background placeholder:text-muted-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 disabled:cursor-not-allowed disabled:opacity-50"
        />
      );

    case "string":
    default:
      return (
        <input
          id={inputId}
          type={prop.format === "email" ? "email" : "text"}
          value={String(value ?? "")}
          onChange={(e) => onChange(e.target.value)}
          placeholder={prop.description}
          className="flex h-10 w-full rounded-md border border-input bg-background px-3 py-2 text-sm ring-offset-background placeholder:text-muted-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 disabled:cursor-not-allowed disabled:opacity-50"
        />
      );
  }
}

type FormFieldProps = {
  name: string;
  label: string;
  required?: boolean;
  error?: string;
  children: React.ReactNode;
};

function FormField({
  name,
  label,
  required,
  error,
  children,
}: FormFieldProps) {
  const inputId = `field-${name}`;

  return (
    <div className="space-y-2">
      <label
        htmlFor={inputId}
        className="text-sm font-medium leading-none peer-disabled:cursor-not-allowed peer-disabled:opacity-70"
      >
        {label}
        {required && <span className="text-destructive ml-1">*</span>}
      </label>
      {children}
      {error && <p className="text-xs text-destructive">{error}</p>}
    </div>
  );
}
