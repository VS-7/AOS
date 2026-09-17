import { DefaultContext, AosAppTriggerOnSearchCallback, AosTriggerDef, IAosTriggerBuilt, IAosTriggerGroupBuilt, AosTriggerAPI, AosAppConfig, AosTriggerHookResult } from "./types";
import z from "zod";
import { useCallback, useEffect, useRef, useState } from "react";
import { AosResponse } from "./response";
import { commandKeyIsMeta } from "@/lib/command-key";

/**
 * Triggers whose keybind a mounted component has taken, by how many of them.
 *
 * A component that calls `use()` decides for itself when the key is live and
 * what it does (the browser panel only closes a tab when one is open), so the
 * application-wide binding below has to stand aside while it is mounted — or
 * a toggle pressed once would toggle twice. Module scope, not per registry:
 * `aos.triggers` and `aos.commands` are two `initialize()` calls over the same
 * groups, and a claim made through one has to be seen by the other.
 */
const claimedKeybinds = new Map<string, number>();

const MODIFIERS = ["mod", "ctrl", "alt", "shift"];
const KEY_ALIASES: Record<string, string> = {
  left: "arrowleft",
  right: "arrowright",
  up: "arrowup",
  down: "arrowdown",
  esc: "escape",
  space: " ",
};

/** Keys a text field moves its caret with, whatever modifier comes along. */
const CARET_KEYS = new Set(["arrowleft", "arrowright", "arrowup", "arrowdown", "home", "end", "backspace", "delete"]);

/**
 * Whether a key press is exactly this keybind (`mod+shift+g`).
 *
 * Exactly: a modifier the keybind does not name must not be held. Matching
 * only the named ones meant ⌘⇧N — New Chat — also fired ⌘N, New Task,
 * because nothing checked that Shift was *not* part of it.
 *
 * `mod` is the platform's command key: ⌘ on macOS, Control elsewhere. It
 * used to be either, everywhere — and on macOS Control is text editing (^N
 * next line, ^K delete to the end, ^B back a character), so typing in a field
 * and pressing ^N opened Create Task.
 */
export function keybindMatches(
  keybind: string,
  event: Pick<KeyboardEvent, "key" | "metaKey" | "ctrlKey" | "altKey" | "shiftKey">,
  mac: boolean = commandKeyIsMeta(),
): boolean {
  const keys = keybind.toLowerCase().split("+").map((k) => k.trim());
  const main = keys.find((k) => !MODIFIERS.includes(k));
  if (!main) return false;

  const wantsMeta = keys.includes("mod") && mac;
  const wantsCtrl = keys.includes("ctrl") || (keys.includes("mod") && !mac);
  if (wantsMeta !== event.metaKey || wantsCtrl !== event.ctrlKey) return false;
  if (keys.includes("alt") !== event.altKey) return false;
  if (keys.includes("shift") !== event.shiftKey) return false;

  return event.key.toLowerCase() === (KEY_ALIASES[main] ?? main);
}

/**
 * Whether this key press belongs to the text field it was typed in: a
 * keybind on a caret key (⌘← is the line's start, Control← a word back) moves
 * the caret there, not the tab's history.
 */
function belongsToTextField(keybind: string, event: KeyboardEvent): boolean {
  const target = event.target;
  if (!(target instanceof Element)) return false;
  const editable =
    target.matches("input, textarea, select, [role='textbox']") ||
    target.closest("[contenteditable]:not([contenteditable='false'])") !== null;
  if (!editable) return false;
  const main = keybind.toLowerCase().split("+").map((k) => k.trim()).find((k) => !MODIFIERS.includes(k));
  return main !== undefined && CARET_KEYS.has(KEY_ALIASES[main] ?? main);
}

/**
 * Listens for every keybind the palette shows, for the whole application.
 *
 * Keybinds used to be attached only while some component called `use()` for
 * that trigger, and nothing did for ⌘N, ⌘⇧R, ⌘⇧G, ⌘⇧P or ⌘⇧N — so the palette
 * advertised shortcuts that did nothing. Mounted once, at the layout; `run`
 * is what executes a trigger there (the palette's own runner, which knows how
 * to follow a redirect and say when a command fails).
 *
 * Left alone: hidden triggers (they are not advertised), triggers a mounted
 * component has claimed through `use()`, and triggers marked
 * `globalKeybind: false` because something outside this registry already
 * answers their key.
 */
export function useGlobalKeybindings(
  registry: { groups?: Record<string, { triggers: Record<string, AosTriggerDef<any, any, any, any, any, any, any>> }> },
  run: (triggerId: string) => void,
): void {
  const runRef = useRef(run);
  runRef.current = run;

  useEffect(() => {
    const bound = Object.values(registry.groups ?? {})
      .flatMap((group) => Object.values(group.triggers))
      .filter((def) => def.keybind && !def.hidden && def.globalKeybind !== false);

    const handleKeyDown = (event: KeyboardEvent) => {
      for (const def of bound) {
        if (claimedKeybinds.has(def.id)) continue;
        if (!keybindMatches(def.keybind!, event)) continue;
        if (belongsToTextField(def.keybind!, event)) return;
        event.preventDefault();
        runRef.current(def.id);
        return;
      }
    };

    window.addEventListener("keydown", handleKeyDown);
    return () => window.removeEventListener("keydown", handleKeyDown);
  }, [registry]);
}

export class AosTriggerGroup<
  TMetadataSchema extends z.ZodTypeAny = z.ZodTypeAny,
  TClient = any,
  TContext = DefaultContext,
  TStores = any
> {
  private _id: string;
  private _order: number = 0;
  private _metadataSchema?: TMetadataSchema;
  private _triggers: Record<string, AosTriggerDef<any, any, any, TClient, TContext, TStores, any>> = {};
  private _loader?: AosAppTriggerOnSearchCallback<TClient, TContext, TStores>;

  constructor(id: string) {
    this._id = id;
  }

  static create<TMetadataSchema extends z.ZodTypeAny = z.ZodTypeAny>(id: string) {
    return new AosTriggerGroup<TMetadataSchema>(id);
  }

  withOrder(order: number): this {
    this._order = order;
    return this;
  }

  withMetadataSchema<TSchema extends z.ZodTypeAny>(schema: TSchema): AosTriggerGroup<TSchema, TClient, TContext, TStores> {
    const group = new AosTriggerGroup<TSchema, TClient, TContext, TStores>(this._id);
    group._order = this._order;
    group._metadataSchema = schema;
    group._triggers = this._triggers;
    group._loader = this._loader;
    return group;
  }

  addTrigger<
    TId extends string,
    TSchema extends z.ZodTypeAny = z.ZodTypeAny,
    TResult = any
  >(trigger: AosTriggerDef<TId, TSchema, TResult, TClient, TContext, TStores, z.infer<TMetadataSchema>>): this {
    this._triggers[trigger.id] = {
      ...trigger,
      group: trigger.group || this._id
    };
    return this;
  }

  withLoader(loader: AosAppTriggerOnSearchCallback<TClient, TContext, TStores>): this {
    this._loader = loader;
    return this;
  }

  build(): IAosTriggerGroupBuilt<TClient, TContext, TStores> {
    return {
      id: this._id,
      order: this._order,
      metadataSchema: this._metadataSchema,
      triggers: this._triggers,
      loader: this._loader
    };
  }
}

export class AosTrigger<
  TClient = any,
  TContext = DefaultContext,
  TStores = any
> {
  private _groups: Record<string, IAosTriggerGroupBuilt<TClient, TContext, TStores>> = {};

  static create<TNewClient = any, TNewContext = DefaultContext, TNewStores = any>() {
    return new AosTrigger<TNewClient, TNewContext, TNewStores>();
  }

  addGroup(group: AosTriggerGroup<any, TClient, TContext, TStores> | IAosTriggerGroupBuilt<TClient, TContext, TStores>): this {
    const built = group instanceof AosTriggerGroup ? group.build() : group;
    this._groups[built.id] = built;
    return this;
  }

  build(): IAosTriggerBuilt<TClient, TContext, TStores> {
    const groups = this._groups;

    return {
      groups,
      initialize: (config) => {
        const list: AosTriggerAPI<TClient, TContext, TStores>['list'] = async (params) => {
          const allTriggers: AosTriggerDef<any, any, any, TClient, TContext, TStores>[] = [];

          // Sort groups by order
          const sortedGroups = Object.values(groups).sort((a, b) => a.order - b.order);

          for (const group of sortedGroups) {
            // Static triggers
            const staticTriggers = Object.values(group.triggers);
            allTriggers.push(...staticTriggers);

            // Dynamic triggers
            // One group's loader failing costs that group, not the palette:
            // a single loader reading a store that was never registered used
            // to reject the whole list, and every command disappeared.
            if (group.loader) {
              try {
                const dynamicTriggers = await group.loader({
                  client: config.client as TClient,
                  context: {} as TContext,
                  stores: config.stores as TStores,
                  query: params.query
                });
                allTriggers.push(...dynamicTriggers.map(cmd => ({ ...cmd, group: cmd.group || group.id })));
              } catch (error) {
                console.error(`[triggers] the "${group.id}" group could not list its commands`, error);
              }
            }
          }

          // Filter by query if provided (simple label search as fallback)
          if (params.query) {
            const q = params.query.toLowerCase();
            return allTriggers.filter(cmd =>
              !cmd.hidden && (
                cmd.label.toLowerCase().includes(q) ||
                cmd.id.toLowerCase().includes(q) ||
                cmd.group?.toLowerCase().includes(q)
              )
            );
          }

          return allTriggers.filter(cmd => !cmd.hidden);
        };

        const dispatch: AosTriggerAPI<TClient, TContext, TStores>['dispatch'] = async (triggerId, input) => {
          // Find trigger in groups
          let def: AosTriggerDef<any, any, any, TClient, TContext, TStores> | undefined;

          for (const group of Object.values(groups)) {
            if (group.triggers[triggerId]) {
              def = group.triggers[triggerId];
              break;
            }
          }

          if (!def) {
            const all = await list({ query: '' });
            def = all.find(c => c.id === triggerId);
          }

          if (!def) throw new Error(`Trigger ${triggerId} not found`);

          let parsedInput = input;
          if (def.schema) {
            parsedInput = await def.schema.parseAsync(input);
          }

          if (def.handler) {
            const response = new AosResponse();
            const context = config.contextFn ? await config.contextFn({ client: config.client!, request: {}, stores: config.stores }) : {} as TContext;

            return await def.handler({
              input: parsedInput,
              client: config.client!,
              context,
              stores: config.stores as TStores,
              response
            });
          }

          return undefined;
        };

        const use: AosTriggerAPI<TClient, TContext, TStores>['use'] = ({ trigger: triggerId, enabled = true, onPressKey, onSuccess, onError }) => {
          const [data, setData] = useState<any>(undefined);
          const [error, setError] = useState<Error | null>(null);
          const [isLoading, setIsLoading] = useState(false);

          const mutate = useCallback(async (input: any) => {
            setIsLoading(true);
            setError(null);
            try {
              const result = await dispatch(triggerId, input);
              setData(result);
              if (onSuccess) onSuccess({ data: result, input });
              return result;
            } catch (err: any) {
              setError(err);
              if (onError) onError({ error: err, input });
              throw err;
            } finally {
              setIsLoading(false);
            }
          }, [triggerId, onSuccess, onError]);

          // Taken while mounted, enabled or not: whether the key is live is
          // this component's call to make, not the global binding's.
          useEffect(() => {
            claimedKeybinds.set(triggerId, (claimedKeybinds.get(triggerId) ?? 0) + 1);
            return () => {
              const remaining = (claimedKeybinds.get(triggerId) ?? 1) - 1;
              if (remaining > 0) claimedKeybinds.set(triggerId, remaining);
              else claimedKeybinds.delete(triggerId);
            };
          }, [triggerId]);

          useEffect(() => {
            if (!enabled) return;

            let keybind: string | undefined;
            for (const group of Object.values(groups)) {
              if (group.triggers[triggerId]?.keybind) {
                keybind = group.triggers[triggerId].keybind;
                break;
              }
            }

            if (!keybind) return;

            const handleKeyDown = (e: KeyboardEvent) => {
              if (keybindMatches(keybind!, e) && !belongsToTextField(keybind!, e)) {
                e.preventDefault();
                if (onPressKey) {
                  onPressKey(e);
                } else {
                  mutate({}).catch(console.error);
                }
              }
            };

            window.addEventListener('keydown', handleKeyDown);
            return () => window.removeEventListener('keydown', handleKeyDown);
          }, [enabled, triggerId, onPressKey, mutate]);

          return { mutate, data, error, isLoading };
        };

        return { list, dispatch, use };
      }
    };
  }
}
