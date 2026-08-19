import { AosApp } from "./builders";
import { AosTrigger } from "./builders/trigger";
import { api } from "@/lib/aos-facade";
import { stores } from "./stores";
import { tasksGroup } from "@/features/task/presentation/triggers/tasks.trigger";

/**
 * The instance the ported pages consume as `aos.page(...)` and
 * `aos.client.task.list.useQuery()`.
 *
 * `client` here is the facade, not `lib/client.ts`: the pages were written
 * against the `client.<feature>.<action>.<method>` shape, and the facade is
 * what presents that.
 *
 * The Fractal original registered 17 stores on this instance; this one
 * registers only the 5 the `task` feature actually reads (see
 * `app/stores.ts`), and the rest arrive in Task 10.
 *
 * `.withTriggers(...)` registers `task`'s own `tasksGroup` (copied
 * verbatim as part of the feature — see `presentation/triggers/tasks.
 * trigger.ts`) so that `aos.triggers.dispatch("tasks.new")` (the "Add
 * task" button in `inner.tsx`) resolves to a real handler instead of the
 * app's no-op fallback. An app shell reaching into one feature's trigger
 * file like this is a shortcut, not the intended shape — later tasks that
 * add more features' triggers should probably have each feature register
 * itself instead of `aos.tsx` importing every one directly.
 */
export const aos = AosApp.create()
  .withClient(api)
  .withStores(stores)
  .withTriggers(AosTrigger.create().addGroup(tasksGroup).build())
  .withDefaultPreload("intent")
  .build();
