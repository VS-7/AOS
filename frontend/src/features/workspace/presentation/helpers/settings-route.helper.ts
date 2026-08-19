import {
  DEFAULT_SETTINGS_SECTION,
  SETTINGS_SECTION_MAP,
  type SettingsSectionId,
} from "@/features/workspace/presentation/components/settings/constants";

/**
 * Legacy settings paths redirected into consolidated sections.
 */
const LEGACY_SETTINGS_REDIRECTS: Record<string, SettingsSectionId> = {
  "user.apiKeys": "user.developers",
  "user.security": "user.profile",
  "workspace.mcp": "user.developers",
};

/**
 * Settings route helpers — map section IDs to URL paths and back.
 */
export class SettingsRouteHelper {
  /**
   * Converts a settings section id into a deep-link path.
   *
   * @param sectionId - Canonical section id (e.g. `user.general`).
   * @returns Path like `/settings/user/general`.
   *
   * @example
   * ```typescript
   * SettingsRouteHelper.sectionIdToPath("workspace.agents");
   * // "/settings/workspace/agents"
   * ```
   */
  public static sectionIdToPath(sectionId: SettingsSectionId): string {
    const [group, section] = sectionId.split(".");
    return `/settings/${group}/${section}`;
  }

  /**
   * Resolves legacy settings paths to their current section id.
   *
   * @param group - URL group segment (`user` | `workspace`).
   * @param section - URL section segment (e.g. `apiKeys`, `mcp`).
   * @returns Redirect target section id, or `null` when the path is not legacy.
   *
   * @example
   * ```typescript
   * SettingsRouteHelper.resolveLegacyRedirect("user", "apiKeys");
   * // "user.developers"
   * ```
   */
  public static resolveLegacyRedirect(
    group: string,
    section: string,
  ): SettingsSectionId | null {
    return LEGACY_SETTINGS_REDIRECTS[`${group}.${section}`] ?? null;
  }

  /**
   * Converts a settings section id into TanStack navigate args.
   *
   * @param sectionId - Canonical section id (e.g. `user.general`).
   * @returns `to` + `params` for `router.navigate`.
   *
   * @example
   * ```typescript
   * const args = SettingsRouteHelper.sectionIdToNavigateArgs("workspace.agents");
   * await router.navigate(args);
   * ```
   */
  public static sectionIdToNavigateArgs(sectionId: SettingsSectionId): {
    to: "/settings/$group/$section";
    params: { group: string; section: string };
  } {
    const [group, section] = sectionId.split(".");
    return {
      to: "/settings/$group/$section",
      params: { group, section },
    };
  }

  /**
   * Parses route params into a settings section id when valid.
   *
   * @param group - URL group segment (`user` | `workspace`).
   * @param section - URL section segment (e.g. `general`, `apiKeys`).
   * @returns Valid {@link SettingsSectionId}, or `null` when unknown.
   *
   * @example
   * ```typescript
   * SettingsRouteHelper.pathToSectionId("user", "general");
   * // "user.general"
   * ```
   */
  public static pathToSectionId(
    group: string,
    section: string,
  ): SettingsSectionId | null {
    const sectionId = `${group}.${section}` as SettingsSectionId;
    if (!SETTINGS_SECTION_MAP[sectionId]) {
      return null;
    }
    return sectionId;
  }

  /**
   * Returns the default settings deep-link path.
   *
   * @returns Path for {@link DEFAULT_SETTINGS_SECTION}.
   *
   * @example
   * ```typescript
   * SettingsRouteHelper.defaultPath();
   * // "/settings/user/general"
   * ```
   */
  public static defaultPath(): string {
    return SettingsRouteHelper.sectionIdToPath(DEFAULT_SETTINGS_SECTION);
  }

  /**
   * Sections that render as full-bleed split layouts (no ScrollArea wrapper).
   */
  public static readonly FULL_BLEED_SECTIONS: readonly SettingsSectionId[] = [
    "workspace.agents",
    "workspace.instructions",
    "workspace.templates",
  ];

  /**
   * Whether a section uses the full-bleed split layout.
   *
   * @param sectionId - Section to inspect.
   * @returns `true` when the section skips ScrollArea wrapping.
   */
  public static isFullBleed(sectionId: SettingsSectionId): boolean {
    return SettingsRouteHelper.FULL_BLEED_SECTIONS.includes(sectionId);
  }
}
