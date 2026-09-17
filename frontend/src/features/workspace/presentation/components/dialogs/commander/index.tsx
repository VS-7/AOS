import { useTranslation } from "@/lib/i18n";
import * as React from "react"
import { AppWindow, Globe, Check, FileText } from "lucide-react"
import { toast } from "sonner"
import { iconByName, loadIcons } from "@/lib/icon-registry"
import { aos } from "@/app/aos"
import { triggers } from "@/app/lib/triggers"
import { useGlobalKeybindings } from "@/app/builders/trigger"
import { errorMessage } from "@/lib/aos-facade"
import {
  CommandDialog,
  CommandEmpty,
  CommandGroup,
  CommandInput,
  CommandItem,
  CommandList,
} from "@/components/ui/command"
import { Kbd } from "@/components/ui/kbd"
import { isRedirect, useRouter } from '@tanstack/react-router'
import type { AosTriggerDef } from "@/app/builders/types"

type Dispatch = { dispatch: (id: string, input?: unknown) => Promise<unknown> }

/**
 * Runs a trigger the way a person asked for it — from the palette or its
 * keybind — and follows through on what it answers.
 *
 * A handler that navigates does so by throwing a redirect, which only the
 * router can act on. Everything else it throws is a failure the person has
 * to hear about: the palette closes the moment a command is picked, so a
 * swallowed error meant choosing a command and watching nothing happen.
 */
function useRunTrigger() {
  const router = useRouter()
  const { t } = useTranslation()
  return React.useCallback(
    (triggerId: string) => {
      ;(aos.triggers as Dispatch).dispatch(triggerId).catch((error: unknown) => {
        if (isRedirect(error)) {
          void router.navigate({ to: error.options.to })
          return
        }
        console.error(`[commander] ${triggerId} failed`, error)
        toast.error(t("That command could not run."), { description: errorMessage(error) })
      })
    },
    [router, t],
  )
}

export function WorkspaceCommander() {
  const { t } = useTranslation()
  const runTrigger = useRunTrigger()
  const open = aos.stores.viewport.useState(s => s.commander.dialog.visible)
  const [query, setQuery] = React.useState("")
  // `any[]`, not `AosTriggerDef<string>[]`: the local default-generic alias
  // doesn't match what `aos.triggers.list()` actually returns (parameterized
  // over this app's real client/stores types), and this component only
  // reads loosely-typed `.metadata` fields off each entry anyway.
  const [commands, setCommands] = React.useState<any[]>([])

  React.useEffect(() => {
    if (!open) {
      setQuery("")
      return
    }

    // The query is matched here, against what the person reads — the
    // translated label — as well as the English one and the id. Leaving it to
    // `list()` matched only the English label, so a Portuguese interface found
    // nothing for "meta" and everything for "goal".
    let cancelled = false
    const loadCommands = async () => {
      try {
        const list = await aos.triggers.list({ query: "" })
        if (!cancelled) setCommands(list)
      } catch (error) {
        console.error("[commander] the commands could not be listed", error)
        if (!cancelled) setCommands([])
      }
    }

    void loadCommands()
    return () => {
      cancelled = true
    }
  }, [open])

  const visibleCommands = React.useMemo(() => {
    const q = query.trim().toLowerCase()
    if (!q) return commands
    return commands.filter((command) =>
      [t(command.label), command.label, command.id, command.group ? t(command.group) : "", command.group ?? ""]
        .some((text) => text.toLowerCase().includes(q)),
    )
  }, [commands, query, t])

  // The icons the listed commands name, fetched one small chunk each rather
  // than imported at the top of this file: the lookup is by string, so a
  // namespace import was keeping all 1.3 MB of icons in the startup bundle
  // for a dialog behind ⌘K. See lib/icon-registry.
  //
  // `iconTick` exists to re-render once they arrive — `iconByName` reads a
  // module-level cache, which React has no way to notice changing.
  const [iconTick, setIconTick] = React.useState(0)
  React.useEffect(() => {
    if (!open || commands.length === 0) return
    let cancelled = false
    void loadIcons(commands.map((command: any) => command.icon)).then(() => {
      if (!cancelled) setIconTick((tick) => tick + 1)
    })
    return () => {
      cancelled = true
    }
  }, [open, commands])

  const groupedCommands = React.useMemo(() => {
    return visibleCommands.reduce((acc, command) => {
      const group = command.group || "General"
      if (!acc[group]) acc[group] = []
      acc[group].push(command)
      return acc
    }, {} as Record<string, AosTriggerDef<string>[]>) as Record<string, AosTriggerDef<string>[]>
  }, [visibleCommands])

  const handleSelectCommand = (command: AosTriggerDef<string>) => {
    runTrigger(command.id)
    aos.stores.viewport.actions.setCommanderOpen(false)
  }

  // Every shortcut the list below shows, answered for the whole window —
  // see useGlobalKeybindings for which ones a mounted component keeps.
  useGlobalKeybindings(triggers, runTrigger)

  // Format the keybind string to be displayed nicely (e.g. mod+shift+f -> ⌘ ⇧ F)
  const renderShortcut = (keybind?: string) => {
    if (!keybind) return null
    const keys = keybind.split('+').map(k => k.trim().toLowerCase())

    return (
      <div className="ml-auto flex items-center gap-0.5 text-xs tracking-widest text-muted-foreground">
        {keys.map((key, i) => {
          let displayKey = key.toUpperCase()
          if (key === 'mod') displayKey = '⌘'
          if (key === 'shift') displayKey = '⇧'
          if (key === 'alt') displayKey = '⌥'
          if (key === 'ctrl') displayKey = '⌃'
          if (key === 'left') displayKey = '←'
          if (key === 'right') displayKey = '→'
          if (key === 'up') displayKey = '↑'
          if (key === 'down') displayKey = '↓'

          return <Kbd key={i}>{displayKey}</Kbd>
        })}
      </div>
    )
  }

  aos.triggers.use({
    trigger: "app.commander.open"
  })

  return (
    <CommandDialog
      open={open}
      onOpenChange={aos.stores.viewport.actions.setCommanderOpen}
      title={t("Command Palette")}
      description={t("Search for a command to run...")}
    >
      <CommandInput
        placeholder={t("Type a command or search...")}
        value={query}
        onValueChange={setQuery}
      />
      <CommandList>
        <CommandEmpty>{t("No results found.")}</CommandEmpty>

        {Object.entries(groupedCommands).map(([group, groupCommands]) => (
          <CommandGroup key={group} heading={t(group)}>
            {groupCommands.map((command: any) => {
              const isTab = command.id.startsWith("tab:")
              // `iconTick` is read so this recomputes once the icon chunks
              // land; until then a named icon simply is not drawn yet.
              void iconTick
              const Icon = command.icon
                ? iconByName(command.icon)
                : isTab
                  ? (command.metadata?.type === 'file' ? FileText : command.metadata?.type === 'in-app' ? AppWindow : Globe)
                  : null
              const isActiveTab = command.metadata?.active

              return (
                <CommandItem
                  key={command.id}
                  // cmdk filters again on its own; give it the same words the
                  // list above was matched on, or it hides what we kept.
                  value={command.id}
                  keywords={[t(command.label), command.label, t(group), group]}
                  onSelect={() => handleSelectCommand(command)}
                  className={isActiveTab ? "bg-accent/50" : ""}
                >
                  {command.metadata?.favicon ? (
                    <img src={command.metadata.favicon} className="mr-2 h-4 w-4 rounded-sm" alt="" />
                  ) : (
                    Icon && <Icon className="mr-2 h-4 w-4" />
                  )}
                  <span className="line-clamp-1">{t(command.label)}</span>
                  {isActiveTab && <Check className="ml-auto size-3.5! text-primary" />}
                  {renderShortcut(command.keybind)}
                </CommandItem>
              )
            })}
          </CommandGroup>
        ))}
      </CommandList>
    </CommandDialog>
  )
}
