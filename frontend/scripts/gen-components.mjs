// Generates internal/domain/view/components.json from the React component
// catalog.
//
// This is the only generator in the project that runs from TypeScript to Go.
// The other three are Go programs: gencatalog reads Go, gentokens reads CSS as
// text, genschema writes TypeScript from the Go registry. This one has to
// *evaluate* TypeScript, because what `z.object({...})` declares is not
// readable as text — which is why it lives here, in Node, rather than in
// tools/.
//
// Run through tsx: `npx tsx frontend/scripts/gen-components.mjs`.
import { writeFileSync } from "node:fs";
import { dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";
import { zodToJsonSchema } from "zod-to-json-schema";

import { catalogDefinitions } from "../src/features/view/presentation/components/registry/definitions/catalog.definitions.ts";

const here = dirname(fileURLToPath(import.meta.url));
const out = resolve(here, "../../internal/domain/view/components.json");

// category is what the agent filters by when it composes a screen. The
// definitions do not carry one, so it is derived from the name the same way a
// person would group them — and an unknown component lands in "other" rather
// than being dropped, because a component missing from the catalog is a
// component the agent cannot use.
const CATEGORY = {
  layout: ["Stack", "Grid", "Box", "Card", "Separator", "ScrollArea", "SplitPageLayout",
    "SplitPageSidebar", "SplitPageSidebarHeader", "SplitPageSidebarContent",
    "SplitPageSidebarItem", "SplitPageContent", "SplitPageContentHeader",
    "SplitPageContentBody", "DetailSection"],
  data: ["Table", "Stat", "Progress", "Badge", "Avatar", "Image", "MarkdownContent",
    "DiffStats", "DiffView", "ActivityItem", "ActivityList", "Pagination"],
  input: ["Input", "Textarea", "Select", "Checkbox", "Radio", "Switch", "Slider",
    "Button", "Toggle", "ToggleGroup", "ButtonGroup", "SearchInput", "Link",
    "DropdownMenu"],
  feedback: ["Alert", "Skeleton", "Spinner", "Tooltip", "Popover", "Dialog", "Drawer"],
  navigation: ["Tabs", "TabsSubtle", "Accordion", "Collapsible", "Carousel"],
  typography: ["Heading", "Text", "Icon"],
};

function categoryOf(name) {
  for (const [category, names] of Object.entries(CATEGORY)) {
    if (names.includes(name)) return category;
  }
  return "other";
}

const specs = [];
const failures = [];

for (const [name, def] of Object.entries(catalogDefinitions)) {
  let props = {};
  if (def?.props) {
    try {
      props = zodToJsonSchema(def.props, { target: "jsonSchema7", $refStrategy: "none" });
    } catch (err) {
      // A component whose schema cannot be converted is worse present than
      // absent: present and permissive means the Go side validates nothing and
      // the agent finds out on a blank screen. Fail loud, naming it.
      failures.push(`${name}: ${err.message}`);
      continue;
    }
  }
  const slots = Array.isArray(def?.slots) ? def.slots : [];
  specs.push({
    name,
    description: typeof def?.description === "string" ? def.description : "",
    category: categoryOf(name),
    props,
    slots,
    acceptsChildren: slots.length > 0,
  });
}

if (failures.length > 0) {
  console.error("gen-components: could not convert these component schemas:");
  for (const f of failures) console.error("  " + f);
  process.exit(1);
}

specs.sort((a, b) => a.name.localeCompare(b.name));
writeFileSync(out, JSON.stringify(specs, null, 2) + "\n");
console.log(`gen-components: ${specs.length} components → ${out}`);
