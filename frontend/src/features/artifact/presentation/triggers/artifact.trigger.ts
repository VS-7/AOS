import { AosTriggerGroup } from "@/app/builders/trigger";
import type { FractalArtifactListItem } from "@/features/artifact/interfaces/artifact.interfaces";
import { ArtifactHelper } from "@/features/artifact/presentation/helpers/artifact.helper";
import { String } from "@/core/helpers/string";

export const artifactGroup = AosTriggerGroup.create("Artifacts")
  .withOrder(6)
  .withLoader(({ query, stores }) => {
    const artifacts: FractalArtifactListItem[] = stores.artifacts.state.items;

    const items = !query
      ? artifacts
      : artifacts.filter((artifact) => {
          const name = artifact.name.toLowerCase();
          const id = artifact.id.toLowerCase();
          const q = query.toLowerCase();
          return name.includes(q) || id.includes(q);
        });

    const triggerItems = items.map((artifact) => ({
      id: `artifact.open.${artifact.id}`,
      label: String.capitalizeFirst(artifact.name),
      icon: ArtifactHelper.getIcon(artifact.icon) as "AppWindow",
      group: "Artifacts",
      handler: () => {
        ArtifactHelper.openInBrowserTab(artifact);
      },
    }));

    return triggerItems as never;
  })
  .build();
