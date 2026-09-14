"use client";

import * as React from "react";
import {
  ArrowDown01Icon,
  ArrowRight01Icon,
  MoreHorizontalIcon,
  Search01Icon,
} from "@hugeicons/core-free-icons";
import { HugeiconsIcon } from "@hugeicons/react";

import { Button } from "@/components/ui/button";
import { ButtonGroup } from "@/components/ui/button-group";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuPortal,
  DropdownMenuRadioGroup,
  DropdownMenuRadioItem,
  DropdownMenuSeparator,
  DropdownMenuSub,
  DropdownMenuSubContent,
  DropdownMenuSubTrigger,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { Input } from "@/components/ui/input";
import { cn } from "@/lib/utils";
import { t } from "@/lib/i18n";

/**
 * Minimal shape every selectable provider must expose to the picker.
 */
export interface AgentModelSelectProvider {
  /** Provider identifier (e.g. "openai", "anthropic", "opencode"). */
  id: string;
  /** Human-readable provider name (e.g. "Anthropic", "OpenAI"). */
  name: string;
  /** Whether the provider has been configured with credentials. */
  configured?: boolean;
  /** Available model IDs for this provider. */
  models?: { id: string; name?: string }[];
  /**
   * Optional logo renderer. Returning `null` hides the logo block.
   * Defaults to a 1×1 transparent placeholder.
   */
  renderLogo?: () => React.ReactNode;
}

/**
 * The levels the runtime honours (`internal/runtime/agentloop`'s `levelOf`).
 * The picker used to offer "Provider default", "Minimal" and "Extra high" as
 * well, and the runtime read each of those as medium — a choice that saved and
 * then did nothing.
 */
export type AgentModelReasoning = "none" | "low" | "medium" | "high";

export interface AgentModelSelectValue {
  provider: string;
  model: string;
  reasoning?: AgentModelReasoning;
}

interface AgentModelSelectProps {
  /** Providers to surface in the picker. Only providers with `configured === true` are selectable. */
  providers: AgentModelSelectProvider[];
  /** Optional callback used to fetch the catalog of a specific provider. */
  onLoadModels?: (providerId: string) => Promise<{ id: string; name?: string }[]>;
  /** Current selection. */
  value: AgentModelSelectValue;
  /** Change handler. */
  onChange: (next: AgentModelSelectValue) => void;
  /** When `true` the selector is rendered as a `ButtonGroup` of two buttons; otherwise just the model picker. */
  showReasoning?: boolean;
  /** What the menu says when there is no provider to offer at all. */
  emptyLabel?: string;
  /** What the trigger says while nothing is selected. */
  placeholder?: string;
  /** When given, the menu offers to clear the selection (e.g. back to a default). */
  onClear?: () => void;
  clearLabel?: string;
  /** Opens the menu on mount. */
  defaultOpen?: boolean;
  className?: string;
  disabled?: boolean;
}

function reasoningOptions(): { value: AgentModelReasoning; label: string }[] {
  return [
    { value: "none", label: t("None") },
    { value: "low", label: t("Low") },
    { value: "medium", label: t("Medium") },
    { value: "high", label: t("High") },
  ];
}

function isProviderSelectable(provider: AgentModelSelectProvider) {
  return provider.configured !== false;
}

/**
 * Keys the search box keeps for itself.
 *
 * A Radix menu runs typeahead on every printable key that reaches it, moving
 * focus to the first item whose label starts with that letter — so the second
 * letter of a search landed on a menu item instead of in the box. The arrows,
 * Tab and Escape still travel: they are how a person leaves the box for the
 * list, or closes the menu.
 */
function keepKeyInSearch(event: React.KeyboardEvent<HTMLInputElement>) {
  if (!["ArrowDown", "ArrowUp", "Tab", "Escape"].includes(event.key)) {
    event.stopPropagation();
  }
}

/** The models matching `term` by model id, model name or provider name, per provider. */
export function matchModels(
  providers: AgentModelSelectProvider[],
  term: string,
): { provider: AgentModelSelectProvider; models: { id: string; name?: string }[] }[] {
  const needle = term.trim().toLowerCase();
  return providers
    .map((provider) => {
      const providerHit =
        provider.name.toLowerCase().includes(needle) || provider.id.toLowerCase().includes(needle);
      const models = (provider.models ?? []).filter(
        (m) =>
          providerHit ||
          m.id.toLowerCase().includes(needle) ||
          (m.name ?? "").toLowerCase().includes(needle),
      );
      return { provider, models };
    })
    .filter((entry) => entry.models.length > 0);
}

/**
 * Two-button selector for {provider, model} + optional reasoning level.
 *
 * @example
 * ```tsx
 * <AgentModelSelect
 *   providers={providers}
 *   value={value}
 *   onChange={setValue}
 *   showReasoning
 * />
 * ```
 */
export const AgentModelSelect = ({
  providers,
  onLoadModels,
  value,
  onChange,
  showReasoning = true,
  emptyLabel,
  placeholder,
  onClear,
  clearLabel,
  defaultOpen,
  className,
  disabled,
}: AgentModelSelectProps) => {
  const [search, setSearch] = React.useState("");
  const [dynamicModels, setDynamicModels] = React.useState<
    Record<string, { id: string; name?: string }[]>
  >({});
  const [loadingProvider, setLoadingProvider] = React.useState<string | null>(null);

  const visibleProviders = React.useMemo(
    () => providers.filter(isProviderSelectable),
    [providers],
  );

  const currentProvider = React.useMemo(
    () => visibleProviders.find((p) => p.id === value.provider),
    [visibleProviders, value.provider],
  );

  const currentModelLabel = React.useMemo(() => {
    if (!currentProvider) return value.model;
    const allModels = [
      ...(currentProvider.models ?? []),
      ...(dynamicModels[currentProvider.id] ?? []),
    ];
    const match = allModels.find((m) => m.id === value.model);
    return match?.name ?? value.model;
  }, [currentProvider, dynamicModels, value.model]);

  const term = search.trim();
  const matches = React.useMemo(
    () => (term ? matchModels(visibleProviders, term) : []),
    [visibleProviders, term],
  );

  const handleSelectModel = React.useCallback(
    (providerId: string, modelId: string) => {
      onChange({
        provider: providerId,
        model: modelId,
        reasoning: value.reasoning,
      });
    },
    [onChange, value.reasoning],
  );

  const handleSelectProvider = React.useCallback(
    async (providerId: string) => {
      // If we already know a model, use it; otherwise the caller will
      // request the catalog via `onLoadModels` and we close the menu.
      const existing = visibleProviders.find((p) => p.id === providerId);
      const firstModel = existing?.models?.[0]?.id;
      if (firstModel) {
        handleSelectModel(providerId, firstModel);
        return;
      }

      if (onLoadModels) {
        setLoadingProvider(providerId);
        try {
          const fetched = await onLoadModels(providerId);
          setDynamicModels((prev) => ({ ...prev, [providerId]: fetched }));
          const first = fetched[0]?.id;
          if (first) {
            handleSelectModel(providerId, first);
          }
        } finally {
          setLoadingProvider(null);
        }
      }
    },
    [visibleProviders, onLoadModels, handleSelectModel],
  );

  const handleReasoning = React.useCallback(
    (next: string) => {
      onChange({ ...value, reasoning: next as AgentModelReasoning });
    },
    [onChange, value],
  );

  const modelTrigger = (
    <Button
      variant="outline"
      size="sm"
      className={cn("gap-1.5 pr-1.5", showReasoning && "rounded-r-none")}
      disabled={disabled}
    >
      {currentProvider?.renderLogo?.()}
      <span className="max-w-40 truncate font-medium">
        {currentModelLabel || placeholder || t("Select model")}
      </span>
      <HugeiconsIcon icon={ArrowDown01Icon} className="size-3.5 opacity-60" />
    </Button>
  );

  const reasoningButton = (
    <Button
      variant="outline"
      size="icon-sm"
      aria-label={t("Reasoning level")}
      className="rounded-l-none border-l-0"
      disabled={disabled}
    >
      <HugeiconsIcon icon={MoreHorizontalIcon} className="size-4" />
    </Button>
  );

  const emptyState = (text: string) => (
    <div className="px-2 py-3 text-center text-xs text-muted-foreground">{text}</div>
  );

  const browse = () =>
    visibleProviders.map((provider) => {
      const providerModels = provider.models ?? dynamicModels[provider.id] ?? [];
      const isSelected = provider.id === value.provider;
      const isLoading = loadingProvider === provider.id;

      if (providerModels.length === 0 && onLoadModels) {
        return (
          <DropdownMenuItem
            key={provider.id}
            onSelect={() => handleSelectProvider(provider.id)}
            className="cursor-pointer"
          >
            {provider.renderLogo?.()}
            <span className="flex-1">{provider.name}</span>
            <HugeiconsIcon icon={ArrowRight01Icon} className="size-3.5 opacity-50" />
            {isLoading && (
              <span className="text-xs text-muted-foreground">{t("Loading…")}</span>
            )}
          </DropdownMenuItem>
        );
      }

      if (providerModels.length === 0) {
        return (
          <DropdownMenuItem key={provider.id} disabled className="cursor-not-allowed opacity-60">
            {provider.renderLogo?.()}
            <span className="flex-1">{provider.name}</span>
            <span className="text-xs text-muted-foreground">{t("No models")}</span>
          </DropdownMenuItem>
        );
      }

      return (
        <DropdownMenuSub key={provider.id}>
          <DropdownMenuSubTrigger
            className={cn("cursor-pointer", isSelected && "bg-accent text-accent-foreground")}
          >
            {provider.renderLogo?.()}
            <span className="flex-1">{provider.name}</span>
          </DropdownMenuSubTrigger>
          <DropdownMenuPortal>
            <DropdownMenuSubContent className="min-w-56 max-h-80 overflow-y-auto p-1">
              <DropdownMenuLabel className="px-2 py-1 text-[11px]">{provider.name}</DropdownMenuLabel>
              {providerModels.map((m) => modelItem(provider, m))}
            </DropdownMenuSubContent>
          </DropdownMenuPortal>
        </DropdownMenuSub>
      );
    });

  const modelItem = (provider: AgentModelSelectProvider, m: { id: string; name?: string }) => {
    const isModelSelected = provider.id === value.provider && m.id === value.model;
    return (
      <DropdownMenuItem
        key={`${provider.id}/${m.id}`}
        onSelect={() => handleSelectModel(provider.id, m.id)}
        className="cursor-pointer"
      >
        <span className="flex-1 truncate">{m.name ?? m.id}</span>
        {isModelSelected && <span className="text-xs text-muted-foreground">✓</span>}
      </DropdownMenuItem>
    );
  };

  // A search lists the matching models themselves, flat and grouped by
  // provider: a model name is what people search for, and a hit hidden
  // inside a submenu is a hit nobody sees.
  const searchResults = () =>
    matches.map(({ provider, models }) => (
      <React.Fragment key={provider.id}>
        <DropdownMenuLabel className="flex items-center gap-1.5 px-2 py-1 text-[11px]">
          {provider.renderLogo?.()}
          {provider.name}
        </DropdownMenuLabel>
        {models.map((m) => modelItem(provider, m))}
      </React.Fragment>
    ));

  return (
    <ButtonGroup className={cn("w-fit", className)}>
      <DropdownMenu
        defaultOpen={defaultOpen}
        onOpenChange={(open) => {
          if (!open) setSearch("");
        }}
      >
        <DropdownMenuTrigger asChild>{modelTrigger}</DropdownMenuTrigger>
        <DropdownMenuContent align="start" sideOffset={6} className="w-72 p-0">
          <div className="flex items-center gap-2 border-b border-border/60 px-2 py-1.5">
            <HugeiconsIcon icon={Search01Icon} className="size-3.5 text-muted-foreground" />
            <Input
              autoFocus
              value={search}
              onChange={(e) => setSearch(e.target.value)}
              onKeyDown={keepKeyInSearch}
              placeholder={t("Search models")}
              className="h-7 border-0 bg-transparent px-0 text-sm shadow-none focus-visible:ring-0"
            />
          </div>
          <div className="max-h-72 overflow-y-auto p-1">
            {onClear && !term ? (
              <>
                <DropdownMenuItem onSelect={onClear} className="cursor-pointer">
                  <span className="flex-1">{clearLabel ?? t("Clear")}</span>
                  {!value.provider && !value.model ? (
                    <span className="text-xs text-muted-foreground">✓</span>
                  ) : null}
                </DropdownMenuItem>
                {visibleProviders.length > 0 ? <DropdownMenuSeparator /> : null}
              </>
            ) : null}
            {visibleProviders.length === 0
              ? emptyState(emptyLabel ?? t("No providers connected yet."))
              : term
                ? matches.length === 0
                  ? emptyState(t('No model matches "{{term}}".', { term }))
                  : searchResults()
                : browse()}
          </div>
        </DropdownMenuContent>
      </DropdownMenu>

      {showReasoning && (
        <DropdownMenu>
          <DropdownMenuTrigger asChild>{reasoningButton}</DropdownMenuTrigger>
          <DropdownMenuContent align="end" sideOffset={6} className="w-44">
            <DropdownMenuLabel className="px-2 text-[11px]">{t("Reasoning")}</DropdownMenuLabel>
            <DropdownMenuRadioGroup value={value.reasoning ?? "medium"} onValueChange={handleReasoning}>
              {reasoningOptions().map((opt) => (
                <DropdownMenuRadioItem key={opt.value} value={opt.value}>
                  {opt.label}
                </DropdownMenuRadioItem>
              ))}
            </DropdownMenuRadioGroup>
          </DropdownMenuContent>
        </DropdownMenu>
      )}
    </ButtonGroup>
  );
};

export default AgentModelSelect;
