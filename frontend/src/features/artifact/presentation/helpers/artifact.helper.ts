import type { FractalArtifactListItem } from "@/features/artifact/interfaces/artifact.interfaces";
import type { ViewportTabState } from "@/features/workspace/presentation/stores/viewport.store";
import { aos } from "@/app/aos";

/**
 * Presentation helpers for workspace artifacts in the sidebar and commander.
 */
export class ArtifactHelper {
  /**
   * Returns the Lucide icon name for an artifact, or the default.
   *
   * @param icon - The icon name from the artifact record.
   * @returns The icon name to render in the UI.
   */
  public static getIcon(icon?: string): string {
    return icon || "AppWindow";
  }

  /**
   * Reads the artifact ID stored on a browser tab, if any.
   *
   * @param tab - Viewport tab state.
   * @returns Artifact ID when the tab was opened from the sidebar.
   */
  public static getTabArtifactId(tab: ViewportTabState): string | undefined {
    const artifactId = tab.metadata?.artifactId;
    return typeof artifactId === "string" ? artifactId : undefined;
  }

  /**
   * Opens an artifact in a browser tab, reusing an existing tab when possible.
   *
   * @param artifact - Artifact list item with resolved URLs.
   */
  public static openInBrowserTab(artifact: FractalArtifactListItem): void {
    const viewportTabs = aos.stores.viewport.state.tabs;

    const existingTab = viewportTabs.items.find(
      (tab) =>
        tab.type === "browser" &&
        ArtifactHelper.getTabArtifactId(tab) === artifact.id,
    );

    if (existingTab) {
      aos.stores.viewport.actions.setActiveTab(existingTab.id);
      return;
    }

    aos.stores.viewport.actions.createTab({
      type: "browser",
      url: artifact.urls.local,
      title: artifact.name,
      metadata: {
        artifactId: artifact.id,
      },
    });
  }
}
