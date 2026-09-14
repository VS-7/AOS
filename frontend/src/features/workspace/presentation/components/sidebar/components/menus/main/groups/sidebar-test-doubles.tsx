import * as React from "react";

/**
 * Light stand-ins for the sidebar's UI primitives, for the group tests beside
 * this file: every menu rendered open, so a test reads a row's actions the way
 * a person sees them once the menu is opened, without Radix's pointer events.
 */
type Props = React.PropsWithChildren<Record<string, any>>;

const Pass = ({ children }: Props) => <>{children}</>;
const Block = ({ children }: Props) => <div>{children}</div>;
const AsButton = ({ children, onClick, isActive, "aria-label": label, asChild }: Props) =>
  asChild ? <>{children}</> : (
    <button type="button" onClick={onClick} aria-label={label} data-active={isActive ? "true" : undefined}>
      {children}
    </button>
  );

export const sidebar = {
  SidebarMenuAction: AsButton,
  SidebarMenuButton: AsButton,
  SidebarMenuItem: Block,
  SidebarMenuMotionItem: Block,
  SidebarMenuSub: Block,
  SidebarMenuSubButton: AsButton,
  SidebarMenuSubItem: Block,
};

export const dropdown = {
  DropdownMenu: Block,
  DropdownMenuTrigger: Pass,
  DropdownMenuContent: ({ children }: Props) => <div role="menu">{children}</div>,
  DropdownMenuItem: ({ children, onClick }: Props) => (
    <button type="button" role="menuitem" onClick={onClick}>
      {children}
    </button>
  ),
};

export const collapsible = { Collapsible: Pass, CollapsibleContent: Pass, CollapsibleTrigger: Pass };
