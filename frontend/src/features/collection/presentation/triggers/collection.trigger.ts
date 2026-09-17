import { AosTriggerGroup } from "@/app/builders/trigger";
import type { CollectionDefinition } from "../helpers/collection-fields.helper";
import { String } from "@/core/helpers/string";

export const collectionGroup = AosTriggerGroup.create("Collections")
  .withOrder(4)
  .withLoader(({ query, stores }) => {
    const collections: CollectionDefinition[] = stores.collections?.state.items ?? [];

    const items = !query
      ? collections
      : collections.filter((collection) =>
        `${collection.name} ${collection.id}`.toLowerCase().includes(query.toLowerCase()),
      );

    return items.map((collection) => ({
      // By id: `/collections/$id` resolves the collection by id, and a name
      // ("Meeting notes") landed on "Page not found" — or, differing only in
      // case, on an empty table whose records could not be listed.
      id: `collection.navigate.${collection.id}` as const,
      label: String.capitalizeFirst(collection.name),
      icon: "Database" as const,
      group: "Collections",
      handler: ({ response }: { response: import("@/app/builders/response").AosResponse }) => {
        response.redirect(`/collections/${encodeURIComponent(collection.id)}`);
      },
    }));
  })
  .build();