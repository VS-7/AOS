import { t } from "@/lib/i18n";
import * as React from "react"
import { AppWindow, Globe, Check, FileText } from "lucide-react"
import { iconByName, loadIcons } from "@/lib/icon-registry"
import { aos } from "@/app/aos"
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

export function WorkspaceCommander() {
  const router = useRouter()
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

    const loadCommands = async () => {
      const list = await aos.triggers.list({ query })
      setCommands(list)
    }

    loadCommands()
  }, [open, query])

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
    return commands.reduce((acc, command) => {
      const group = command.group || "General"
      if (!acc[group]) acc[group] = []
      acc[group].push(command)
      return acc
    }, {} as Record<string, AosTriggerDef<string>[]>) as Record<string, AosTriggerDef<string>[]>
  }, [commands, query])

  const handleSelectCommand = (command: AosTriggerDef<string>) => {
    (aos.triggers as { dispatch: (id: string, input?: unknown) => Promise<unknown> }).dispatch(command.id).catch(error => {
      if (isRedirect(error)) {
        router.navigate({ to: error.options.to })
      }
    })

    aos.stores.viewport.actions.setCommanderOpen(false)
  }

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
    <CommandDialog open={open} onOpenChange={aos.stores.viewport.actions.setCommanderOpen}>
      <CommandInput
        placeholder={t("Type a command or search...")}
        value={query}
        onValueChange={setQuery}
      />
      <CommandList>
        <CommandEmpty>{t("No results found.")}</CommandEmpty>

        {Object.entries(groupedCommands).map(([group, groupCommands]) => (
          <CommandGroup key={group} heading={group}>
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
                  onSelect={() => handleSelectCommand(command)}
                  className={isActiveTab ? "bg-accent/50" : ""}
                >
                  {command.metadata?.favicon ? (
                    <img src={command.metadata.favicon} className="mr-2 h-4 w-4 rounded-sm" alt="" />
                  ) : (
                    Icon && <Icon className="mr-2 h-4 w-4" />
                  )}
                  <span className="line-clamp-1">{command.label}</span>
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
