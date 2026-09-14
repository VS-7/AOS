import { describe, expect, it } from "vitest";
import {
  filterListings,
  groupListingsByCategory,
  installedSkillDetail,
  mergeEnv,
  pickFeaturedListings,
  toMarketplaceListing,
} from "./marketplace.helper";

const github = {
  registry: "local",
  source: "acme/github",
  name: "github",
  description: "GitHub issues, PRs and repos from your agents.",
  version: "1.2.0",
  tags: ["development", "git"],
  stars: 120,
  updatedAt: "2026-09-01T10:00:00Z",
  permissions: { network: ["api.github.com"] },
};

describe("a registry listing on screen", () => {
  // marketplace_discovery answers Go's Listing, and the page read fields it
  // does not have: `displayName.slice` crashed the page into "Something went
  // wrong", and a search threw "listing.keywords is not iterable".
  it("carries every field the cards and filters read", () => {
    const listing = toMarketplaceListing(github);
    expect(listing).toMatchObject({
      name: "acme/github",
      displayName: "github",
      shortDescription: github.description,
      description: github.description,
      category: "Development",
      keywords: ["development", "git"],
      capabilities: [],
      logo: null,
      brandColor: null,
      author: "acme",
      source: "acme/github",
      registry: "local",
      version: "1.2.0",
    });
    expect(() => filterListings([listing], { query: "git" })).not.toThrow();
    expect(filterListings([listing], { query: "issues" })).toHaveLength(1);
  });

  it("is routed by its source, and falls under Other without a known category", () => {
    const bare = toMarketplaceListing({ registry: "r", source: "solo/tool", name: "" });
    expect(bare.name).toBe("solo/tool");
    expect(bare.displayName).toBe("solo/tool");
    expect(bare.category).toBe("Other");
    expect(bare.keywords).toEqual([]);
    expect(groupListingsByCategory([bare]).Other).toHaveLength(1);
  });

  it("is featured by its short name", () => {
    expect(pickFeaturedListings([toMarketplaceListing(github)]).map((l) => l.name)).toEqual(["acme/github"]);
  });
});

describe("an installed skill's page", () => {
  it("lists what the skill installed, by kind", () => {
    const detail = installedSkillDetail({
      id: "demo-crm",
      name: "Demo CRM",
      description: "Contacts and deals.",
      active: true,
      source: "acme/crm",
      version: "0.3.1",
      metadata: {
        toolsets: [{ id: "crm-api", type: "rest-api" }],
        collections: [{ id: "contacts" }],
        views: [{ id: "contacts-table" }],
      },
    });
    expect(detail.plugin).toMatchObject({ name: "demo-crm", displayName: "Demo CRM", source: "acme/crm", version: "0.3.1" });
    expect(detail.installedSkill).toMatchObject({ id: "demo-crm", active: true, skillMdPath: ".aos/skills/demo-crm/SKILL.md" });
    expect(detail.inventory.toolsets).toEqual([
      expect.objectContaining({ id: "crm-api", kind: "toolsets", connectionType: "rest-api" }),
    ]);
    expect(detail.inventory.collections.map((item) => item.id)).toEqual(["contacts"]);
    expect(detail.inventory.views.map((item) => item.id)).toEqual(["contacts-table"]);
    expect(detail.inventory.routines).toEqual([]);
  });
});

describe("saving a toolset's variables", () => {
  // toolsets_update-config replaces Env wholesale: sending only the fields
  // typed this time erased every other variable the toolset had.
  it("keeps what was already there and only adds what was typed", () => {
    expect(
      mergeEnv({ GH_TOKEN: "${env.GITHUB_TOKEN}", REGION: "us" }, { API_KEY: " secret ", REGION: "" }),
    ).toEqual({ GH_TOKEN: "${env.GITHUB_TOKEN}", REGION: "us", API_KEY: "secret" });
    expect(mergeEnv(undefined, { API_KEY: "x" })).toEqual({ API_KEY: "x" });
  });
});

describe("the marketplace's category names", () => {
  // Section titles render through t(category) — a variable the catalogue
  // test's scan for t("…") literals cannot see.
  it("are in both catalogues", async () => {
    const en = (await import("@/lib/i18n/locales/en.json")).default as Record<string, string>;
    const ptBR = (await import("@/lib/i18n/locales/pt-BR.json")).default as Record<string, string>;
    const { MARKETPLACE_ALLOWED_CATEGORIES } = await import("@/features/marketplace/presentation/consts/marketplace");
    const { MARKETPLACE_OTHER_CATEGORY } = await import("./marketplace.helper");
    const missing = [...MARKETPLACE_ALLOWED_CATEGORIES, MARKETPLACE_OTHER_CATEGORY].filter(
      (category) => !(category in en) || !(category in ptBR),
    );
    expect(missing).toEqual([]);
  });
});
