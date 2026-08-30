import { useCallback, useEffect, useMemo, useState } from "react";
import type { JSX } from "react";
import { toast } from "sonner";

import { client } from "@/lib/client";
import { t } from "@/lib/i18n";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Skeleton } from "@/components/ui/skeleton";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";

/** The thirteen the daemon accepts, in the order it declares them. */
const CATEGORIES = [
  "decision",
  "intent",
  "commitment",
  "relationship",
  "event",
  "observation",
  "error",
  "learning",
  "fact",
  "reference",
  "instruction",
  "preference",
  "context",
] as const;

type Category = (typeof CATEGORIES)[number];

interface MemoryRow {
  id: string;
  title: string;
  description: string;
  category: Category;
  tags?: string[];
  content?: string;
  confidence?: number;
  status?: string;
  createdAt?: string;
}

/**
 * What an agent knows, and the four things a person can do about it.
 *
 * Only `memories_graph` was ever wired into the desktop, so the window could
 * draw the shape of an agent's knowledge — dots and lines — and could not read
 * one memory, write one, or retire one that had stopped being true. Memory is
 * the part of this system the agent is told to consult before anything else;
 * it was the part the person had no access to at all.
 *
 * Forgetting is deliberately not deleting. `memories_forget` deprecates and
 * keeps the trace along with the reason it stopped applying, which is why the
 * reason is asked for here rather than defaulted — a lineage that says "gone,
 * no idea why" is worse than no lineage.
 */
export function MemoryList({ agentId }: { agentId: string }): JSX.Element {
  const [memories, setMemories] = useState<MemoryRow[]>([]);
  const [loading, setLoading] = useState(true);
  const [query, setQuery] = useState("");
  const [category, setCategory] = useState<Category | "all">("all");
  const [open, setOpen] = useState<MemoryRow | null>(null);
  const [forgetting, setForgetting] = useState<MemoryRow | null>(null);
  const [composing, setComposing] = useState(false);

  const recall = useCallback(async () => {
    setLoading(true);
    try {
      const answer = (await client.invoke("memories_recall", {
        agent: agentId,
        ...(query.trim() ? { query: query.trim() } : {}),
        ...(category === "all" ? {} : { category }),
        limit: 100,
        _reasoning: "the person is reading what this agent remembers",
      })) as { memories?: MemoryRow[] } | undefined;
      setMemories(answer?.memories ?? []);
    } catch (error) {
      toast.error(error instanceof Error ? error.message : t("The memories could not be read."));
      setMemories([]);
    } finally {
      setLoading(false);
    }
  }, [agentId, query, category]);

  // Debounced so typing in the search box does not send one recall per
  // keystroke; the daemon answers this from a search index when it has one
  // and by scanning when it does not, and the second is not free.
  useEffect(() => {
    const id = setTimeout(() => void recall(), 250);
    return () => clearTimeout(id);
  }, [recall]);

  const active = useMemo(
    () => memories.filter((memory) => (memory.status ?? "active") === "active"),
    [memories],
  );
  const deprecated = memories.length - active.length;

  return (
    <div className="flex h-full flex-col gap-3">
      <div className="flex flex-wrap items-center gap-2">
        <Input
          value={query}
          onChange={(event) => setQuery(event.target.value)}
          placeholder={t("Search what this agent remembers")}
          className="h-9 max-w-xs"
        />
        <Select value={category} onValueChange={(value) => setCategory(value as Category | "all")}>
          <SelectTrigger className="h-9 w-44">
            <SelectValue placeholder={t("All categories")} />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="all">{t("All categories")}</SelectItem>
            {CATEGORIES.map((name) => (
              <SelectItem key={name} value={name}>
                {name}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
        <div className="ml-auto flex items-center gap-2">
          {deprecated > 0 ? (
            <span className="text-xs text-muted-foreground">
              {t("{count} deprecated").replace("{count}", String(deprecated))}
            </span>
          ) : null}
          <Button size="sm" variant="secondary" onClick={() => setComposing(true)}>
            {t("Write a memory")}
          </Button>
        </div>
      </div>

      <div className="min-h-0 flex-1 overflow-auto rounded-lg border border-border/60">
        {loading ? (
          <div className="space-y-2 p-3">
            <Skeleton className="h-12 w-full" />
            <Skeleton className="h-12 w-full" />
            <Skeleton className="h-12 w-full" />
          </div>
        ) : memories.length === 0 ? (
          <p className="p-6 text-center text-sm text-muted-foreground">
            {query.trim() || category !== "all"
              ? t("Nothing matches this search.")
              : t("This agent hasn't recorded anything yet.")}
          </p>
        ) : (
          <ul className="divide-y divide-border/60">
            {memories.map((memory) => (
              <li key={memory.id} className="flex items-start gap-3 p-3">
                <button
                  type="button"
                  className="min-w-0 flex-1 text-left"
                  onClick={() => void openMemory(memory, agentId, setOpen)}
                >
                  <div className="flex items-center gap-2">
                    <span className="truncate text-sm font-medium">{memory.title}</span>
                    <Badge variant="secondary">{memory.category}</Badge>
                    {(memory.status ?? "active") !== "active" ? (
                      <Badge variant="outline">{memory.status}</Badge>
                    ) : null}
                  </div>
                  <p className="mt-0.5 line-clamp-2 text-xs text-muted-foreground">
                    {memory.description}
                  </p>
                </button>
                {(memory.status ?? "active") === "active" ? (
                  <Button size="sm" variant="ghost" onClick={() => setForgetting(memory)}>
                    {t("Forget")}
                  </Button>
                ) : null}
              </li>
            ))}
          </ul>
        )}
      </div>

      <MemoryDetail memory={open} onClose={() => setOpen(null)} />
      <ForgetDialog
        memory={forgetting}
        agentId={agentId}
        onClose={() => setForgetting(null)}
        onDone={() => {
          setForgetting(null);
          void recall();
        }}
      />
      <ComposeDialog
        open={composing}
        onClose={() => setComposing(false)}
        onDone={() => {
          setComposing(false);
          void recall();
        }}
      />
    </div>
  );
}

/**
 * Reads one memory in full.
 *
 * The list carries the title and the description — the summary written to be
 * found by search — but not the body, which is the memory itself. Reflecting
 * is the call that returns it.
 */
async function openMemory(
  memory: MemoryRow,
  agentId: string,
  show: (memory: MemoryRow) => void,
): Promise<void> {
  show(memory);
  try {
    const full = (await client.invoke("memories_reflect", {
      memory: memory.id,
      agent: agentId,
      _reasoning: "the person opened one memory to read it in full",
    })) as MemoryRow | undefined;
    if (full) show({ ...memory, ...full });
  } catch {
    // The summary is already on screen and is a real answer on its own.
  }
}

function MemoryDetail({
  memory,
  onClose,
}: {
  memory: MemoryRow | null;
  onClose: () => void;
}): JSX.Element | null {
  if (!memory) return null;
  return (
    <Dialog open onOpenChange={onClose}>
      <DialogContent className="sm:max-w-2xl">
        <DialogHeader>
          <DialogTitle>{memory.title}</DialogTitle>
          <DialogDescription>{memory.description}</DialogDescription>
        </DialogHeader>
        <div className="flex flex-wrap items-center gap-2">
          <Badge variant="secondary">{memory.category}</Badge>
          {typeof memory.confidence === "number" ? (
            <Badge variant="outline">
              {t("confidence {value}").replace("{value}", memory.confidence.toFixed(2))}
            </Badge>
          ) : null}
          {(memory.tags ?? []).map((tag) => (
            <Badge key={tag} variant="outline">
              {tag}
            </Badge>
          ))}
        </div>
        {memory.content ? (
          <pre className="max-h-96 overflow-auto whitespace-pre-wrap rounded-lg border border-border/60 bg-muted/40 p-3 text-xs">
            {memory.content}
          </pre>
        ) : (
          <p className="text-sm text-muted-foreground">{t("This memory has no body.")}</p>
        )}
      </DialogContent>
    </Dialog>
  );
}

function ForgetDialog({
  memory,
  agentId,
  onClose,
  onDone,
}: {
  memory: MemoryRow | null;
  agentId: string;
  onClose: () => void;
  onDone: () => void;
}): JSX.Element | null {
  const [reason, setReason] = useState("");
  const [busy, setBusy] = useState(false);
  useEffect(() => setReason(""), [memory?.id]);
  if (!memory) return null;

  // Go refuses a placeholder: the minimum is five characters, because a
  // lineage entry reading "n/a" is worse than none.
  const tooShort = reason.trim().length < 5;

  const forget = async () => {
    setBusy(true);
    try {
      await client.invoke("memories_forget", {
        memory: memory.id,
        agent: agentId,
        reason: reason.trim(),
        _reasoning: "the person is retiring a memory that stopped being true",
      });
      toast.success(t("Memory deprecated."));
      onDone();
    } catch (error) {
      toast.error(error instanceof Error ? error.message : t("The memory could not be forgotten."));
    } finally {
      setBusy(false);
    }
  };

  return (
    <Dialog open onOpenChange={onClose}>
      <DialogContent className="sm:max-w-lg">
        <DialogHeader>
          <DialogTitle>{t("Forget this memory?")}</DialogTitle>
          <DialogDescription>
            {t(
              "It is deprecated, not deleted: the trace stays, with the reason it no longer applies.",
            )}
          </DialogDescription>
        </DialogHeader>
        <div className="space-y-2">
          <Label htmlFor="forget-reason">{t("What changed?")}</Label>
          <Input
            id="forget-reason"
            value={reason}
            onChange={(event) => setReason(event.target.value)}
            placeholder={t("The API this described was replaced in v2.")}
          />
          <p className="text-xs text-muted-foreground">
            {t("If you are unsure, lower its confidence instead of forgetting it.")}
          </p>
        </div>
        <DialogFooter>
          <Button variant="secondary" onClick={onClose}>
            {t("Cancel")}
          </Button>
          <Button variant="destructive" disabled={busy || tooShort} onClick={forget}>
            {t("Forget")}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

function ComposeDialog({
  open,
  onClose,
  onDone,
}: {
  open: boolean;
  onClose: () => void;
  onDone: () => void;
}): JSX.Element | null {
  const [title, setTitle] = useState("");
  const [description, setDescription] = useState("");
  const [category, setCategory] = useState<Category>("fact");
  const [content, setContent] = useState("");
  const [busy, setBusy] = useState(false);

  useEffect(() => {
    if (!open) return;
    setTitle("");
    setDescription("");
    setCategory("fact");
    setContent("");
  }, [open]);

  if (!open) return null;

  const store = async () => {
    setBusy(true);
    try {
      await client.invoke("memories_store", {
        title: title.trim(),
        description: description.trim(),
        category,
        ...(content.trim() ? { content: content.trim() } : {}),
        _reasoning: "a person is recording something they want the agent to remember",
      });
      toast.success(t("Memory recorded."));
      onDone();
    } catch (error) {
      toast.error(error instanceof Error ? error.message : t("The memory could not be recorded."));
    } finally {
      setBusy(false);
    }
  };

  const incomplete = !title.trim() || !description.trim();

  return (
    <Dialog open onOpenChange={onClose}>
      <DialogContent className="sm:max-w-lg">
        <DialogHeader>
          <DialogTitle>{t("Write a memory")}</DialogTitle>
          <DialogDescription>
            {t("It belongs to whoever is signed in — say who you are with --agent to write as one.")}
          </DialogDescription>
        </DialogHeader>
        <div className="space-y-3">
          <div className="space-y-1.5">
            <Label htmlFor="memory-title">{t("Title")}</Label>
            <Input
              id="memory-title"
              value={title}
              onChange={(event) => setTitle(event.target.value)}
              placeholder={t("Sharp and specific — it is what you will scan later.")}
            />
          </div>
          <div className="space-y-1.5">
            <Label htmlFor="memory-description">{t("Description")}</Label>
            <Input
              id="memory-description"
              value={description}
              onChange={(event) => setDescription(event.target.value)}
              placeholder={t("Dense and keyword-rich — this is what search reads.")}
            />
          </div>
          <div className="space-y-1.5">
            <Label>{t("Category")}</Label>
            <Select value={category} onValueChange={(value) => setCategory(value as Category)}>
              <SelectTrigger className="h-9">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                {CATEGORIES.map((name) => (
                  <SelectItem key={name} value={name}>
                    {name}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>
          <div className="space-y-1.5">
            <Label htmlFor="memory-content">{t("Body (optional)")}</Label>
            <textarea
              id="memory-content"
              value={content}
              onChange={(event) => setContent(event.target.value)}
              rows={5}
              className="w-full rounded-lg border border-border/60 bg-background/60 p-2 text-sm"
            />
          </div>
        </div>
        <DialogFooter>
          <Button variant="secondary" onClick={onClose}>
            {t("Cancel")}
          </Button>
          <Button disabled={busy || incomplete} onClick={store}>
            {t("Record")}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
