import { AosTriggerGroup } from "@/app/builders/trigger";
import type { FractalCustomCollection } from "../../interfaces/collection.interfaces";
import { String } from "@/core/helpers/string";

export const collectionGroup = AosTriggerGroup.create("Collections")
  .withOrder(4)
  .withLoader(({ query, stores }) => {
    const collections: FractalCustomCollection[] = stores.collections.state.items;

    const items = !query
      ? collections
      : collections.filter((collection) =>
        collection.name.toLowerCase().includes(query.toLowerCase()),
      );

    return items.map((collection) => ({
      id: `collection.navigate.${collection.name}` as const,
      label: String.capitalizeFirst(collection.name),
      icon: "Database" as const,
      group: "Collections",
      handler: ({ response }: { response: import("@/app/builders/response").AosResponse }) => {
        response.redirect(`/collections/${collection.name}`);
      },
    }));
  })
  .build();