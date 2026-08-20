import { shadcnComponents } from "@json-render/shadcn";
import {
  BoxComponent,
  GridComponent,
  StackComponent,
} from "./components/layout.components";
import {
  ActivityItemComponent,
  ActivityListComponent,
  CardComponent,
  DetailSectionComponent,
  ScrollAreaComponent,
  SeparatorComponent,
} from "./components/surface.components";
import {
  HeadingComponent,
  LinkComponent,
  MarkdownContentComponent,
  TextComponent,
} from "./components/typography.components";
import {
  BadgeComponent,
  DiffStatsComponent,
  DiffViewComponent,
  IconComponent,
  StatComponent,
} from "./components/data.components";
import { TabsSubtleComponent } from "./components/navigation.components";
import {
  ButtonComponent,
  InputComponent,
  SearchInputComponent,
  SplitPageContentBodyComponent,
  SplitPageContentComponent,
  SplitPageContentHeaderComponent,
  SplitPageLayoutComponent,
  SplitPageSidebarComponent,
  SplitPageSidebarContentComponent,
  SplitPageSidebarHeaderComponent,
  SplitPageSidebarItemComponent,
} from "./components/actions.components";

/**
 * AOS View component registry — custom @app implementations layered over shadcn fallbacks.
 */
export const viewComponents = {
  ...shadcnComponents,

  // Layout
  Stack: StackComponent,
  Grid: GridComponent,
  Box: BoxComponent,
  Card: CardComponent,
  ScrollArea: ScrollAreaComponent,
  Separator: SeparatorComponent,
  DetailSection: DetailSectionComponent,
  ActivityItem: ActivityItemComponent,
  ActivityList: ActivityListComponent,

  // Typography
  Heading: HeadingComponent,
  Text: TextComponent,
  Link: LinkComponent,
  MarkdownContent: MarkdownContentComponent,

  // Data
  Badge: BadgeComponent,
  Icon: IconComponent,
  DiffStats: DiffStatsComponent,
  DiffView: DiffViewComponent,
  Stat: StatComponent,

  // Navigation
  TabsSubtle: TabsSubtleComponent,

  // Actions / forms
  Button: ButtonComponent,
  Input: InputComponent,
  SearchInput: SearchInputComponent,

  // Split page layout
  SplitPageLayout: SplitPageLayoutComponent,
  SplitPageSidebar: SplitPageSidebarComponent,
  SplitPageSidebarHeader: SplitPageSidebarHeaderComponent,
  SplitPageSidebarContent: SplitPageSidebarContentComponent,
  SplitPageSidebarItem: SplitPageSidebarItemComponent,
  SplitPageContent: SplitPageContentComponent,
  SplitPageContentHeader: SplitPageContentHeaderComponent,
  SplitPageContentBody: SplitPageContentBodyComponent,
};

export {
  BoxComponent,
  GridComponent,
  StackComponent,
  CardComponent,
  ScrollAreaComponent,
  SeparatorComponent,
  DetailSectionComponent,
  ActivityItemComponent,
  ActivityListComponent,
  HeadingComponent,
  TextComponent,
  LinkComponent,
  MarkdownContentComponent,
  BadgeComponent,
  IconComponent,
  DiffStatsComponent,
  DiffViewComponent,
  StatComponent,
  TabsSubtleComponent,
  ButtonComponent,
  InputComponent,
  SearchInputComponent,
  SplitPageLayoutComponent,
  SplitPageSidebarComponent,
  SplitPageSidebarHeaderComponent,
  SplitPageSidebarContentComponent,
  SplitPageSidebarItemComponent,
  SplitPageContentComponent,
  SplitPageContentHeaderComponent,
  SplitPageContentBodyComponent,
};
