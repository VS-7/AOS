import * as React from "react";

import { aos } from "@/app/aos";
import type { ArtifactListItem } from "@/features/artifact/interfaces/artifact.interfaces";
import { ArtifactHelper } from "@/features/artifact/presentation/helpers/artifact.helper";
import { ArtifactStore } from "@/features/artifact/presentation/stores/artifact.store";

export function useArtifacts() {
  const activeArtifactId = aos.stores.viewport.useState((state) => {
    const activeTab = state.tabs.items.find((tab) => tab.id === state.tabs.current);

    if (!activeTab || activeTab.type !== "browser") {
      return undefined;
    }

    return ArtifactHelper.getTabArtifactId(activeTab);
  });

  const rawArtifacts = ArtifactStore.useState((state) => state.items);

  const artifacts = React.useMemo(() => {
    return [...rawArtifacts].sort((left, right) =>
      left.name.localeCompare(right.name),
    );
  }, [rawArtifacts]);

  function open(artifact: ArtifactListItem) {
    ArtifactHelper.openInBrowserTab(artifact);
  }

  return {
    artifacts,
    current: activeArtifactId,
    open,
  };
}
