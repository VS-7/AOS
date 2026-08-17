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
  DropdownMenuSub,
  DropdownMenuSubContent,
  DropdownMenuSubTrigger,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { Input } from "@/components/ui/input";
import { cn } from "@/lib/utils";

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

export type AgentModelReasoning =
  | "provider-default"
  | "none"
  | "minimal"
  | "low"
  | "medium"
  | "high"
  | "xhigh";

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
  className?: string;
  disabled?: boolean;
}

const REASONING_OPTIONS: { value: AgentModelReasoning; label: string }[] = [
  { value: "provider-default", label: "Provider default" },
  { value: "none", label: "None" },
  { value: "minimal", label: "Minimal" },
  { value: "low", label: "Low" },
  { value: "medium", label: "Medium" },
  { value: "high", label: "High" },
  { value: "xhigh", label: "Extra high" },
];

function isProviderSelectable(provider: AgentModelSelectProvider) {
  return provider.configured !== false;
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

  const filteredProviders = React.useMemo(() => {
    const term = search.trim().toLowerCase();
    if (!term) return visibleProviders;
    return visibleProviders.filter((p) => p.name.toLowerCase().includes(term));
  }, [visibleProviders, search]);

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
      className="gap-1.5 rounded-r-none pr-1.5"
      disabled={disabled}
    >
      {currentProvider?.renderLogo?.()}
      <span className="max-w-40 truncate font-medium">
        {currentModelLabel || "Select model"}
      </span>
      <HugeiconsIcon icon={ArrowDown01Icon} className="size-3.5 opacity-60" />
    </Button>
  );

  const reasoningButton = (
    <Button
      variant="outline"
      size="icon-sm"
      aria-label="Reasoning level"
      className="rounded-l-none border-l-0"
      disabled={disabled}
    >
      <HugeiconsIcon icon={MoreHorizontalIcon} className="size-4" />
    </Button>
  );

  return (
    <ButtonGroup className={cn("w-fit", className)}>
      <DropdownMenu
        onOpenChange={(open) => {
          if (!open) setSearch("");
        }}
      >
        <DropdownMenuTrigger asChild>{modelTrigger}</DropdownMenuTrigger>
        <DropdownMenuContent
          align="start"
          sideOffset={6}
          className="w-72 p-0"
        >
          <div className="flex items-center gap-2 border-b border-border/60 px-2 py-1.5">
            <HugeiconsIcon
              icon={Search01Icon}
              className="size-3.5 text-muted-foreground"
            />
            <Input
              autoFocus
              value={search}
              onChange={(e) => setSearch(e.target.value)}
              placeholder="Search models"
              className="h-7 border-0 bg-transparent px-0 text-sm shadow-none focus-visible:ring-0"
            />
          </div>
          <div className="max-h-72 overflow-y-auto p-1">
            {filteredProviders.length === 0 ? (
              <div className="px-2 py-3 text-center text-xs text-muted-foreground">
                No providers configured.
              </div>
            ) : (
              filteredProviders.map((provider) => {
                const providerModels =
                  provider.models ?? dynamicModels[provider.id] ?? [];
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
                      <HugeiconsIcon
                        icon={ArrowRight01Icon}
                        className="size-3.5 opacity-50"
                      />
                      {isLoading && (
                        <span className="text-xs text-muted-foreground">
                          Loading…
                        </span>
                      )}
                    </DropdownMenuItem>
                  );
                }

                if (providerModels.length === 0) {
                  return (
                    <DropdownMenuItem
                      key={provider.id}
                      disabled
                      className="cursor-not-allowed opacity-60"
                    >
                      {provider.renderLogo?.()}
                      <span className="flex-1">{provider.name}</span>
                      <span className="text-xs text-muted-foreground">
                        No models
                      </span>
                    </DropdownMenuItem>
                  );
                }

                return (
                  <DropdownMenuSub key={provider.id}>
                    <DropdownMenuSubTrigger
                      className={cn(
                        "cursor-pointer",
                        isSelected && "bg-accent text-accent-foreground",
                      )}
                    >
                      {provider.renderLogo?.()}
                      <span className="flex-1">{provider.name}</span>
                    </DropdownMenuSubTrigger>
                    <DropdownMenuPortal>
                      <DropdownMenuSubContent className="min-w-56 max-h-80 overflow-y-auto p-1">
                        <DropdownMenuLabel className="px-2 py-1 text-[11px]">
                          {provider.name}
                        </DropdownMenuLabel>
                        {providerModels.map((m) => {
                          const isModelSelected =
                            provider.id === value.provider && m.id === value.model;
                          return (
                            <DropdownMenuItem
                              key={m.id}
                              onSelect={() => handleSelectModel(provider.id, m.id)}
                              className="cursor-pointer"
                            >
                              <span className="flex-1 truncate">
                                {m.name ?? m.id}
                              </span>
                              {isModelSelected && (
                                <span className="text-xs text-muted-foreground">
                                  ✓
                                </span>
                              )}
                            </DropdownMenuItem>
                          );
                        })}
                      </DropdownMenuSubContent>
                    </DropdownMenuPortal>
                  </DropdownMenuSub>
                );
              })
            )}
          </div>
        </DropdownMenuContent>
      </DropdownMenu>

      {showReasoning && (
        <DropdownMenu>
          <DropdownMenuTrigger asChild>{reasoningButton}</DropdownMenuTrigger>
          <DropdownMenuContent align="end" sideOffset={6} className="w-44">
            <DropdownMenuLabel className="px-2 text-[11px]">
              Reasoning
            </DropdownMenuLabel>
            <DropdownMenuRadioGroup
              value={value.reasoning ?? "provider-default"}
              onValueChange={handleReasoning}
            >
              {REASONING_OPTIONS.map((opt) => (
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
