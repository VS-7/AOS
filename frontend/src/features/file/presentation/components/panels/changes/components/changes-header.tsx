import * as React from "react";
import {
  ChevronDown,
  GitBranch,
  ListTodo,
  Monitor,
  MoreHorizontal,
  RefreshCw,
  Search,
} from "lucide-react";
import { Button } from "@/components/ui/button";
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
import { Switch } from "@/components/ui/switch";
import { Input } from "@/components/ui/input";
import { Kbd, KbdGroup } from "@/components/ui/kbd";
import { cn } from "@/lib/utils";
import type {
  ChangesDiffStyle,
  ChangesPanelPreferences,
} from "@/features/file/presentation/helpers/changes.helper";
import {
  formatChangesContextRef,
  formatChangesContextScope,
  formatChangesCountLabel,
} from "@/features/file/presentation/helpers/changes.helper";
import type { FractalFileExplorerContext } from "@/features/file/interfaces/file.interfaces";

interface ChangesHeaderProps {
  explorerContext: FractalFileExplorerContext;
  fileCount: number;
  additions: number;
  deletions: number;
  readOnly: boolean;
  isRefreshing: boolean;
  preferences: ChangesPanelPreferences;
  findOpen: boolean;
  findQuery: string;
  onRefresh: () => void;
  onPreferencesChange: (next: Partial<ChangesPanelPreferences>) => void;
  onFindOpenChange: (open: boolean) => void;
  onFindQueryChange: (query: string) => void;
}

export function ChangesHeader({
  explorerContext,
  fileCount,
  additions,
  deletions,
  readOnly,
  isRefreshing,
  preferences,
  findOpen,
  findQuery,
  onRefresh,
  onPreferencesChange,
  onFindOpenChange,
  onFindQueryChange,
}: ChangesHeaderProps) {
  const findInputRef = React.useRef<HTMLInputElement>(null);
  const scopeLabel = formatChangesContextScope(explorerContext);
  const refLabel = formatChangesContextRef(explorerContext);
  const countLabel = formatChangesCountLabel({ fileCount, readOnly });
  const ScopeIcon =
    explorerContext.type === "task"
      ? ListTodo
      : explorerContext.type === "branch"
        ? GitBranch
        : Monitor;

  React.useEffect(() => {
    if (findOpen) {
      findInputRef.current?.focus();
      findInputRef.current?.select();
    }
  }, [findOpen]);

  function setDiffStyle(diffStyle: ChangesDiffStyle) {
    onPreferencesChange({ diffStyle });
  }

  return (
    <div className="flex shrink-0 flex-col gap-2 border-b px-4 py-3 mb-2">
      <div className="flex items-start gap-2">
        <div className="min-w-0 flex-1 space-y-4">
          <div className="flex min-w-0 items-center gap-2">
            <span className="inline-flex h-6 shrink-0 items-center gap-1.5 rounded-md border border-border/50 bg-muted/50 px-2 text-xs font-medium text-foreground/85">
              <ScopeIcon className="size-3.5 text-muted-foreground" />
              {scopeLabel}
            </span>
            <span className="truncate text-[13px] text-muted-foreground">
              {refLabel}
            </span>
          </div>

          <div className="flex min-w-0 items-center gap-1.5 text-[13px] px-3">
            <span className="truncate text-muted-foreground">{countLabel}</span>
            {(additions > 0 || deletions > 0) && (
              <span className="inline-flex items-center gap-2 font-mono text-[12px] tabular-nums">
                {additions > 0 ? (
                  <span className="text-emerald-500 dark:text-emerald-400">
                    +{additions}
                  </span>
                ) : null}
                {deletions > 0 ? (
                  <span className="text-rose-400/90 dark:text-rose-400/80">
                    −{deletions}
                  </span>
                ) : null}
              </span>
            )}
          </div>
        </div>

        <div className="flex shrink-0 items-center gap-0.5">
          <Button
            type="button"
            variant="ghost"
            size="icon"
            className="size-8 text-muted-foreground"
            onClick={onRefresh}
            disabled={isRefreshing}
            aria-label="Refresh changes"
          >
            <RefreshCw
              className={cn("size-3.5", isRefreshing && "animate-spin")}
            />
          </Button>

          <DropdownMenu>
            <DropdownMenuTrigger asChild>
              <Button
                type="button"
                variant="ghost"
                size="icon"
                className="size-8 text-muted-foreground"
                aria-label="Changes options"
              >
                <MoreHorizontal className="size-4" />
              </Button>
            </DropdownMenuTrigger>
            <DropdownMenuContent align="end" className="w-56">
              <DropdownMenuSub>
                <DropdownMenuSubTrigger>
                  <span className="flex-1">Layout</span>
                  <span className="ml-2 capitalize text-muted-foreground">
                    {preferences.diffStyle}
                  </span>
                </DropdownMenuSubTrigger>
                <DropdownMenuSubContent>
                  <DropdownMenuItem onClick={() => setDiffStyle("unified")}>
                    Unified
                    {preferences.diffStyle === "unified" ? " ✓" : ""}
                  </DropdownMenuItem>
                  <DropdownMenuItem onClick={() => setDiffStyle("split")}>
                    Split
                    {preferences.diffStyle === "split" ? " ✓" : ""}
                  </DropdownMenuItem>
                </DropdownMenuSubContent>
              </DropdownMenuSub>

              <DropdownMenuItem
                className="justify-between gap-3"
                onSelect={(event) => event.preventDefault()}
              >
                Ignore Whitespace
                <Switch
                  checked={preferences.ignoreWhitespace}
                  onCheckedChange={(checked) =>
                    onPreferencesChange({ ignoreWhitespace: checked })
                  }
                />
              </DropdownMenuItem>

              <DropdownMenuItem
                className="justify-between gap-3"
                onSelect={(event) => event.preventDefault()}
              >
                Word Wrap
                <Switch
                  checked={preferences.wordWrap}
                  onCheckedChange={(checked) =>
                    onPreferencesChange({ wordWrap: checked })
                  }
                />
              </DropdownMenuItem>

              <DropdownMenuSeparator />

              <DropdownMenuItem onClick={() => onFindOpenChange(true)}>
                <Search className="size-4" />
                Find in Changes
                <KbdGroup className="ml-auto">
                  <Kbd>⌘</Kbd>
                  <Kbd>F</Kbd>
                </KbdGroup>
              </DropdownMenuItem>
              <DropdownMenuItem onClick={onRefresh}>
                <RefreshCw className="size-4" />
                Refresh Changes
                <KbdGroup className="ml-auto">
                  <Kbd>⌘</Kbd>
                  <Kbd>R</Kbd>
                </KbdGroup>
              </DropdownMenuItem>
            </DropdownMenuContent>
          </DropdownMenu>
        </div>
      </div>

      {findOpen ? (
        <div className="flex items-center gap-2">
          <Input
            ref={findInputRef}
            value={findQuery}
            onChange={(event) => onFindQueryChange(event.target.value)}
            placeholder="Filter changed files…"
            className="h-8"
            onKeyDown={(event) => {
              if (event.key === "Escape") {
                onFindOpenChange(false);
                onFindQueryChange("");
              }
            }}
          />
          <Button
            type="button"
            variant="ghost"
            size="sm"
            className="h-8"
            onClick={() => {
              onFindOpenChange(false);
              onFindQueryChange("");
            }}
          >
            Done
          </Button>
        </div>
      ) : null}
    </div>
  );
}
