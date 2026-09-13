import * as React from "react";
import { Plus, Trash2 } from "lucide-react";
import { toast } from "sonner";
import { Button } from "@/components/ui/button";
import { Checkbox } from "@/components/ui/checkbox";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { aos } from "@/app/aos";
import { errorMessage } from "@/lib/aos-facade";
import { t } from "@/lib/i18n";
import type { CollectionField } from "@/features/collection/presentation/helpers/collection-fields.helper";

const FIELD_TYPES: CollectionField["type"][] = ["string", "number", "boolean", "date", "enum", "ref", "list"];

type DraftField = { name: string; type: CollectionField["type"]; required: boolean; options: string };

const emptyField = (): DraftField => ({ name: "", type: "string", required: false, options: "" });

/**
 * The id a collection's name suggests: the directory name rules the daemon
 * enforces — lowercase, digits, hyphen and underscore.
 */
export function slugifyCollectionId(name: string): string {
  return name
    .normalize("NFD")
    .replace(/[\u0300-\u036f]/g, "")
    .toLowerCase()
    .replace(/[^a-z0-9_-]+/g, "-")
    .replace(/^-+|-+$/g, "");
}

/** The fields as `collections_create` takes them, from the rows the person filled. */
export function draftToFields(drafts: DraftField[]): CollectionField[] {
  return drafts
    .filter((draft) => draft.name.trim())
    .map((draft) => {
      const field: CollectionField = { name: draft.name.trim(), type: draft.type };
      if (draft.required) field.required = true;
      if (draft.type === "enum") {
        field.enum = draft.options.split(",").map((option) => option.trim()).filter(Boolean);
      }
      if (draft.type === "ref") field.ref = draft.options.trim();
      return field;
    });
}

interface CreateCollectionDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  onCreated: (id: string) => void;
}

/**
 * Declares a collection from the sidebar. There was no way to do it without
 * an agent: the Collections group only listed what already existed.
 *
 * What it validates is only what makes the request well-formed; the daemon's
 * own schema check (a ref naming a collection that exists, an enum with
 * values, no field twice) answers for the rest, and its message is shown.
 */
export function CreateCollectionDialog({ open, onOpenChange, onCreated }: CreateCollectionDialogProps) {
  const [name, setName] = React.useState("");
  const [id, setId] = React.useState("");
  const [idTouched, setIdTouched] = React.useState(false);
  const [format, setFormat] = React.useState<"json" | "md">("json");
  const [fields, setFields] = React.useState<DraftField[]>([{ ...emptyField(), name: "name", required: true }]);
  const [saving, setSaving] = React.useState(false);

  React.useEffect(() => {
    if (!open) return;
    setName("");
    setId("");
    setIdTouched(false);
    setFormat("json");
    setFields([{ ...emptyField(), name: "name", required: true }]);
  }, [open]);

  const effectiveId = idTouched ? id : slugifyCollectionId(name);
  const declared = draftToFields(fields);
  const canSubmit = Boolean(name.trim() && effectiveId && declared.length > 0) && !saving;

  function updateField(index: number, patch: Partial<DraftField>) {
    setFields((current) => current.map((field, at) => (at === index ? { ...field, ...patch } : field)));
  }

  async function submit(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!canSubmit) return;
    setSaving(true);
    try {
      await aos.client.collection.create.mutateOrThrow({
        body: { id: effectiveId, name: name.trim(), format, fields: declared },
      });
      await aos.stores.collections.actions.refresh();
      toast.success(t("Collection created."));
      onOpenChange(false);
      onCreated(effectiveId);
    } catch (error) {
      toast.error(errorMessage(error) ?? t("Unable to create the collection."));
    } finally {
      setSaving(false);
    }
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-lg">
        <DialogHeader>
          <DialogTitle>{t("New collection")}</DialogTitle>
          <DialogDescription>
            {t("A table of records with the fields you declare. Agents and views read and write it.")}
          </DialogDescription>
        </DialogHeader>
        <form className="flex flex-col gap-4" onSubmit={(event) => void submit(event)}>
          <div className="grid grid-cols-2 gap-3">
            <div className="flex flex-col gap-2">
              <Label htmlFor="collection-name">{t("Name")}</Label>
              <Input
                id="collection-name"
                autoFocus
                value={name}
                placeholder={t("Contacts")}
                onChange={(event) => setName(event.target.value)}
              />
            </div>
            <div className="flex flex-col gap-2">
              <Label htmlFor="collection-id">{t("Identifier")}</Label>
              <Input
                id="collection-id"
                value={effectiveId}
                placeholder="contacts"
                onChange={(event) => {
                  setIdTouched(true);
                  setId(slugifyCollectionId(event.target.value));
                }}
              />
            </div>
          </div>

          <div className="flex flex-col gap-2">
            <Label htmlFor="collection-format">{t("Format")}</Label>
            <Select value={format} onValueChange={(value) => setFormat(value as "json" | "md")}>
              <SelectTrigger id="collection-format" className="w-full">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="json">{t("JSON — fields only")}</SelectItem>
                <SelectItem value="md">{t("Markdown — fields and a text body")}</SelectItem>
              </SelectContent>
            </Select>
          </div>

          <div className="flex flex-col gap-2">
            <Label>{t("Fields")}</Label>
            {fields.map((field, index) => (
              <div key={index} className="flex items-center gap-2">
                <Input
                  aria-label={t("Field name")}
                  className="h-8 flex-1"
                  value={field.name}
                  placeholder={t("Field name")}
                  onChange={(event) => updateField(index, { name: event.target.value })}
                />
                <Select value={field.type} onValueChange={(value) => updateField(index, { type: value as DraftField["type"] })}>
                  <SelectTrigger size="sm" className="w-28" aria-label={t("Field type")}>
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    {FIELD_TYPES.map((type) => (
                      <SelectItem key={type} value={type}>
                        {type}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
                {field.type === "enum" || field.type === "ref" ? (
                  <Input
                    aria-label={field.type === "enum" ? t("Allowed values") : t("Referenced collection")}
                    className="h-8 w-32"
                    value={field.options}
                    placeholder={field.type === "enum" ? t("a, b, c") : t("collection id")}
                    onChange={(event) => updateField(index, { options: event.target.value })}
                  />
                ) : null}
                <label className="flex items-center gap-1 text-xs text-muted-foreground">
                  <Checkbox
                    checked={field.required}
                    onCheckedChange={(checked) => updateField(index, { required: checked === true })}
                  />
                  {t("Required")}
                </label>
                <Button
                  type="button"
                  variant="ghost"
                  size="icon"
                  className="size-8"
                  aria-label={t("Remove field")}
                  disabled={fields.length === 1}
                  onClick={() => setFields((current) => current.filter((_, at) => at !== index))}
                >
                  <Trash2 className="size-3.5" />
                </Button>
              </div>
            ))}
            <Button
              type="button"
              variant="outline"
              size="sm"
              className="self-start"
              onClick={() => setFields((current) => [...current, emptyField()])}
            >
              <Plus className="size-3.5" />
              {t("Add field")}
            </Button>
          </div>

          <DialogFooter>
            <Button type="button" variant="ghost" size="sm" onClick={() => onOpenChange(false)}>
              {t("Cancel")}
            </Button>
            <Button type="submit" size="sm" disabled={!canSubmit}>
              {t("Create")}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}
