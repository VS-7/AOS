import { SidebarContent } from "@/components/ui/sidebar"
import { FilesExplorerGroup } from "./components/files-explorer-group"

export function AppSidebarFilesMenu() {
  return (
    <SidebarContent className="flex h-full min-h-0 flex-1 flex-col gap-0 overflow-hidden p-0">
      <FilesExplorerGroup />
    </SidebarContent>
  )
}
