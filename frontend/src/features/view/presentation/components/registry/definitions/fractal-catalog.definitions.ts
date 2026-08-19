import { z } from "zod";
import { shadcnComponentDefinitions } from "@json-render/shadcn/catalog";
import {
  validationCheckSchema,
  validateOnSchema,
  zClassName,
  zNullableString,
} from "../shared/catalog-zod";

const tabItemSchema = z.object({
  label: z.string(),
  value: z.string(),
  icon: zNullableString.describe("Lucide icon name (PascalCase), e.g. GitPullRequest"),
  count: z.union([z.string(), z.number()]).nullable().optional()
    .describe("Optional count shown beside the tab (works with activeLabel)"),
});

/**
 * Fractal-specific and extended json-render catalog definitions.
 * Spreads the shadcn baseline, overrides key primitives, and adds SplitPageLayout family.
 */
export const fractalCatalogDefinitions = {
  ...shadcnComponentDefinitions,

  Separator: {
    props: z.object({
      orientation: z.enum(["horizontal", "vertical"]).nullable(),
      className: zClassName,
    }),
    description: "Visual separator line",
  },

  Stack: {
    ...shadcnComponentDefinitions.Stack,
    props: z.object({
      direction: z.enum(["horizontal", "vertical"]).nullable(),
      gap: z.enum(["none", "sm", "md", "lg", "xl"]).nullable(),
      align: z.enum(["start", "center", "end", "stretch"]).nullable(),
      justify: z
        .enum(["start", "center", "end", "between", "around"])
        .nullable(),
      wrap: z.boolean().nullable(),
      className: zClassName,
    }),
    slots: ["default"],
    description: "Flex container for layouts",
    example: { direction: "vertical", gap: "md" },
  },

  Card: {
    props: z.object({
      title: zNullableString,
      description: zNullableString,
      maxWidth: z.enum(["sm", "md", "lg", "full"]).nullable(),
      centered: z.boolean().nullable(),
      className: zClassName,
      padding: z.enum(["none", "sm", "md"]).nullable(),
      density: z.enum(["default", "compact"]).nullable(),
    }),
    slots: ["default"],
    description:
      "Container card for content sections. Children render inside CardContent.",
    example: { title: "Overview", description: "Your account summary" },
  },

  Box: {
    props: z.object({
      as: z.enum(["div", "section", "article", "main"]).nullable(),
      className: zClassName,
    }),
    slots: ["default"],
    description: "Generic container — use className for full Tailwind control.",
  },

  ScrollArea: {
    props: z.object({
      className: zClassName,
      orientation: z.enum(["vertical", "horizontal", "both"]).nullable(),
    }),
    slots: ["default"],
    description: "Scrollable region for sidebars and long content.",
  },

  Heading: {
    props: z.object({
      text: z.string(),
      level: z.enum(["h1", "h2", "h3", "h4"]).nullable(),
      className: zClassName,
      truncate: z.boolean().nullable(),
    }),
    description: "Heading text (h1-h4)",
    example: { text: "Welcome", level: "h1" },
  },

  Text: {
    props: z.object({
      text: z.string(),
      variant: z
        .enum(["body", "caption", "muted", "lead", "code"])
        .nullable(),
      weight: z.enum(["normal", "medium", "semibold"]).nullable(),
      align: z.enum(["left", "center", "right"]).nullable(),
      truncate: z.boolean().nullable(),
      lines: z.number().nullable(),
      className: zClassName,
    }),
    description: "Paragraph text with Fractal typography tokens.",
    example: { text: "Hello, world!" },
  },

  Link: {
    props: z.object({
      label: z.string(),
      href: z.string(),
      external: z.boolean().nullable(),
      variant: z.enum(["default", "muted"]).nullable(),
      className: zClassName,
    }),
    events: ["press"],
    description: "Anchor link. Bind on.press for click handler.",
  },

  MarkdownContent: {
    props: z.object({
      content: z.string(),
      isUserMessage: z.boolean().nullable(),
      className: zClassName,
    }),
    description:
      "Renders markdown with Fractal MarkdownContent (tasks/chat parity). Bind content via $state, e.g. { $state: '/items/0/body' }.",
    example: {
      content: "## Overview\n\nMarkdown from GitHub or any source.",
      isUserMessage: false,
    },
  },

  Badge: {
    props: z.object({
      text: z.string(),
      variant: z
        .enum(["default", "secondary", "destructive", "outline"])
        .nullable(),
      size: z.enum(["sm", "md", "lg"]).nullable(),
      color: z
        .enum([
          "gray",
          "red",
          "orange",
          "amber",
          "yellow",
          "lime",
          "green",
          "emerald",
          "teal",
          "cyan",
          "blue",
          "indigo",
          "violet",
          "purple",
          "fuchsia",
          "pink",
          "rose",
        ])
        .nullable(),
      className: zClassName,
    }),
    description: "Status badge with Fractal color palette.",
    example: { text: "Open", variant: "secondary" },
  },

  Icon: {
    props: z.object({
      name: z.string().describe("Lucide icon name in PascalCase, e.g. GitBranch"),
      size: z.number().nullable(),
      className: zClassName,
      fallback: zNullableString,
    }),
    description: "Dynamic Lucide icon resolved by name.",
    example: { name: "GitPullRequest", size: 16 },
  },

  DiffStats: {
    props: z.object({
      additions: z.number(),
      deletions: z.number(),
      size: z.enum(["sm", "md"]).nullable(),
      className: zClassName,
    }),
    description:
      "Colored +/− diff counter (same styling as the Changes panel).",
    example: { additions: 42, deletions: 7 },
  },

  DiffView: {
    props: z.object({
      oldContent: z.string(),
      newContent: z.string(),
      fileName: zNullableString,
      diffStyle: z.enum(["unified", "split"]).nullable(),
      wordWrap: z.boolean().nullable(),
      className: zClassName,
    }),
    description:
      "Full text diff viewer using @pierre/diffs (same engine as Changes panel).",
  },

  Stat: {
    props: z.object({
      icon: zNullableString.describe("Lucide icon name"),
      label: z.string(),
      value: z.string(),
      variant: z.enum(["inline", "tile", "strip", "row"]).nullable(),
      className: zClassName,
    }),
    description:
      "Metadata display — inline, tile, strip KPI cell, or Codex-style labeled row.",
    example: { icon: "GitBranch", label: "Branch", value: "feat/foo → main" },
  },

  TabsSubtle: {
    props: z.object({
      items: z.array(tabItemSchema),
      value: zNullableString,
      activeLabel: z.boolean().nullable(),
      className: zClassName,
    }),
    events: ["change"],
    description:
      "Fractal subtle tabs (animated pill). Use { $bindState } on value for active tab.",
  },

  Button: {
    props: z.object({
      label: z.string(),
      variant: z
        .enum([
          "default",
          "outline",
          "secondary",
          "ghost",
          "destructive",
          "link",
          "primary",
          "danger",
        ])
        .nullable(),
      size: z
        .enum(["xs", "sm", "default", "lg", "icon", "icon-sm", "icon-lg"])
        .nullable(),
      disabled: z.boolean().nullable(),
      className: zClassName,
    }),
    events: ["press"],
    description:
      "Fractal button (shadcn variants). primary→default, danger→destructive.",
    example: { label: "Submit", variant: "default" },
  },

  SearchInput: {
    props: z.object({
      placeholder: zNullableString,
      value: zNullableString,
      name: zNullableString,
      className: zClassName,
    }),
    events: ["change"],
    description:
      "Sidebar search field (SplitPageLayout.SearchInput styling). Use { $bindState } on value.",
  },

  Input: {
    props: z.object({
      label: z.string(),
      name: z.string(),
      type: z.enum(["text", "email", "password", "number"]).nullable(),
      placeholder: zNullableString,
      value: zNullableString,
      checks: validationCheckSchema,
      validateOn: validateOnSchema,
      className: zClassName,
    }),
    events: ["submit", "focus", "blur"],
    description: "Text input with Fractal styling.",
  },

  SplitPageLayout: {
    props: z.object({
      variant: z.enum(["default", "stacked"]).nullable(),
      activeItemId: zNullableString,
      className: zClassName,
    }),
    slots: ["default"],
    description:
      "Master-detail page shell (sidebar + content). Children: SplitPageSidebar + SplitPageContent.",
  },

  SplitPageSidebar: {
    props: z.object({ className: zClassName }),
    slots: ["default"],
    description: "Left sidebar column inside SplitPageLayout.",
  },

  SplitPageSidebarHeader: {
    props: z.object({ className: zClassName }),
    slots: ["default"],
    description: "Sticky header row inside the sidebar (search, tabs, actions).",
  },

  SplitPageSidebarContent: {
    props: z.object({ className: zClassName }),
    slots: ["default"],
    description: "Scrollable list area inside the sidebar.",
  },

  SplitPageSidebarItem: {
    props: z.object({
      itemIndex: z.number().describe("Index used to compare with /ui/selected"),
      title: z.string(),
      meta: zNullableString,
      badge: zNullableString,
      statusIcon: zNullableString.describe("Lucide icon for row status"),
      statusTone: z
        .enum(["success", "warning", "danger", "muted"])
        .nullable(),
      additions: z.number().nullable(),
      deletions: z.number().nullable(),
      className: zClassName,
    }),
    events: ["press"],
    description:
      "Selectable sidebar row with title, meta, optional badge and diff stats.",
  },

  SplitPageContent: {
    props: z.object({ className: zClassName }),
    slots: ["default"],
    description: "Main content / detail column inside SplitPageLayout.",
  },

  SplitPageContentHeader: {
    props: z.object({ className: zClassName }),
    slots: ["default"],
    description: "Detail header toolbar (title row + actions).",
  },

  SplitPageContentBody: {
    props: z.object({ className: zClassName }),
    slots: ["default"],
    description: "Scrollable detail body inside SplitPageContent.",
  },

  DetailSection: {
    props: z.object({
      title: z.string(),
      value: zNullableString.describe("Stable accordion value id"),
      defaultOpen: z.boolean().nullable(),
      className: zClassName,
    }),
    slots: ["default"],
    description:
      "Linear-style collapsible detail section (chevron + quiet title).",
    example: { title: "Description", defaultOpen: true },
  },

  ActivityItem: {
    props: z.object({
      icon: zNullableString.describe("Lucide icon name"),
      tone: z.enum(["success", "warning", "danger", "muted"]).nullable(),
      title: z.string(),
      meta: zNullableString,
      status: zNullableString.describe("Trailing status label"),
      variant: z.enum(["plain", "pill"]).nullable(),
      className: zClassName,
    }),
    description:
      "Quiet activity/check row — icon · title · optional trailing status. Default variant is pill.",
    example: {
      icon: "CheckCircle2",
      tone: "success",
      title: "CI",
      status: "Passed",
    },
  },

  ActivityList: {
    props: z.object({
      items: z
        .array(
          z.object({
            icon: zNullableString,
            tone: z.enum(["success", "warning", "danger", "muted"]).nullable(),
            title: z.string(),
            meta: zNullableString,
            status: zNullableString,
          }),
        )
        .nullable(),
      emptyText: zNullableString,
      className: zClassName,
    }),
    description:
      "Stack of Codex-style pill rows. Bind items via $state to an array.",
  },
} ;

export type FractalCatalogComponentName = keyof typeof fractalCatalogDefinitions;

export const FRACTAL_REGISTRY_COMPONENT_NAMES = Object.keys(
  fractalCatalogDefinitions,
) as FractalCatalogComponentName[];
