import { DefaultContext, AosAppTriggerOnSearchCallback, AosTriggerDef, IAosTriggerBuilt, IAosTriggerGroupBuilt, AosTriggerAPI, AosAppConfig, AosTriggerHookResult } from "./types";
import z from "zod";
import { useCallback, useEffect, useState } from "react";
import { AosResponse } from "./response";

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
            if (group.loader) {
              const dynamicTriggers = await group.loader({
                client: config.client as TClient,
                context: {} as TContext,
                stores: config.stores as TStores,
                query: params.query
              });
              allTriggers.push(...dynamicTriggers.map(cmd => ({ ...cmd, group: cmd.group || group.id })));
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
              const keys = keybind!.toLowerCase().split('+').map(k => k.trim());
              const requiresMod = keys.includes('mod');
              const requiresCtrl = keys.includes('ctrl');
              const requiresAlt = keys.includes('alt');
              const requiresShift = keys.includes('shift');

              let mainKey = keys.find(k => !['mod', 'ctrl', 'alt', 'shift'].includes(k));
              if (mainKey === 'left') mainKey = 'arrowleft';
              if (mainKey === 'right') mainKey = 'arrowright';
              if (mainKey === 'up') mainKey = 'arrowup';
              if (mainKey === 'down') mainKey = 'arrowdown';
              if (mainKey === 'esc') mainKey = 'escape';
              if (mainKey === 'space') mainKey = ' ';

              const isModPressed = e.metaKey || e.ctrlKey;

              if (requiresMod && !isModPressed) return;
              if (requiresCtrl && !e.ctrlKey) return;
              if (requiresAlt && !e.altKey) return;
              if (requiresShift && !e.shiftKey) return;

              if (mainKey && e.key.toLowerCase() === mainKey) {
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
