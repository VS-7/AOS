export const MARKETPLACE_ALLOWED_CATEGORIES = [
  "Productivity",
  "Development",
  "Communication",
  "Data",
  "Design",
  "DevOps",
  "AI",
  "Research",
  "Marketing",
] as const;

export const MARKETPLACE_CATEGORY_PREVIEW_LIMIT = 6;

export const MARKETPLACE_FEATURED_PLUGIN_SLUGS = [
  "fractal",
  "github",
  "slack",
  "linear",
  "notion",
  "playwright",
] as const;

export const MARKETPLACE_FEATURED_SECTION_ID = "featured-plugins";
export const MARKETPLACE_INSTALLED_SECTION_ID = "installed-plugins";

export const MARKETPLACE_INVENTORY_ORDER = [
  "toolsets",
  "collections",
  "views",
  "hooks",
  "instructions",
  "templates",
  "artifacts",
  "routines",
] as const;

export const MARKETPLACE_INVENTORY_FOLDERS: Record<
  (typeof MARKETPLACE_INVENTORY_ORDER)[number],
  string
> = {
  toolsets: "toolsets",
  collections: "collections",
  views: "views",
  hooks: "hooks",
  instructions: "instructions",
  templates: "templates",
  artifacts: "artifacts",
  routines: "routines",
};
