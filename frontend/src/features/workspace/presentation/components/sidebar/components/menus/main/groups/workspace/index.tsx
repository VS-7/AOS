import {
  SidebarGroup,
  SidebarGroupContent,
} from "@/components/ui/sidebar";
import { AppSidebarWorkspaceSelectDropdown } from "../../../../dropdowns/workspace-select.dropdown";
import { AppSidebarWorkspaceHeaderDropdown } from "../../../../dropdowns/workspace-header.dropdown";

/**
 * Mobile-only workspace switcher group rendered above the main navigation.
 */
export function WorkspaceSidebarWorkspaceGroup() {
  return (
    <SidebarGroup>
      <SidebarGroupContent className="grid grid-cols-[1fr_auto] gap-1">
        <AppSidebarWorkspaceHeaderDropdown />
        <AppSidebarWorkspaceSelectDropdown />
      </SidebarGroupContent>
    </SidebarGroup>
  );
}
