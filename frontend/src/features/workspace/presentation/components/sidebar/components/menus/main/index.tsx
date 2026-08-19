import { SidebarContent } from "@/components/ui/sidebar";
import { WorkspaceSidebarNavigationGroup } from "./groups/navigation";
import { WorkspaceSidebarManagementGroup } from "./groups/management";
import { WorkspaceSidebarWorkspaceGroup } from "./groups/workspace";
import { WorkspaceSidebarChatGroupMenu } from "./groups/chat";

export function WorkspaceSidebarMainMenu() {
  return (
    <SidebarContent className="scroll-fade scroll-fade-10 gap-0">
      <WorkspaceSidebarWorkspaceGroup />
      <WorkspaceSidebarNavigationGroup />
      <WorkspaceSidebarManagementGroup />
      <WorkspaceSidebarChatGroupMenu />
    </SidebarContent>
  );
}
