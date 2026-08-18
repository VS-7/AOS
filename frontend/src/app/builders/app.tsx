import { createRootRouteWithContext, RouteComponent, AnyRoute, useBlocker } from "@tanstack/react-router";
import { DefaultContext, AosAppBuilt, AosAppConfig, IAosAppBuilder, InferContextIn, IAosTriggerBuilt, MutationPath, AosUseFormOptions, AosFormReturn, AosStoresCollection } from "./types";
import { AosLayout } from "./layout";
import { AosMiddleware } from "./middleware";
import { AosPage } from "./page";
import z from "zod";
import React, { useEffect, useState, useCallback, useRef } from "react";
import { useQueryClient } from "@tanstack/react-query";
import { useForm as useHookForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { isDeepEqualData } from "ai";
import { AosStoreBuilt } from "./store";


/**
 * Orchestrator for building the Aos Application structure.
 * 
 * @template TClient - The API client type.
 * @template TContext - The application context type.
 * 
 * @example
 * ```typescript
 * const app = AosApp.create()
 *   .withClient(client)
 *   .withContext(async () => ({ user: null }))
 *   .build();
 * ```
 */
export class AosApp<
  TClient = any,
  TContext = DefaultContext,
  TStores extends AosStoresCollection<any> = AosStoresCollection<any>,
  TTrigger extends IAosTriggerBuilt<TClient, TContext, TStores> = IAosTriggerBuilt<TClient, TContext, TStores>
> implements IAosAppBuilder<TClient, TContext, TStores, TTrigger> {
  private _config: AosAppConfig<TClient, TContext, TStores, TTrigger> = {};

  /**
   * Creates a new instance of the AosApp builder.
   * @returns A new {@link IAosAppBuilder}.
   */
  static create<TNewClient = any, TNewContext = DefaultContext, TNewStores extends AosStoresCollection<any> = AosStoresCollection<any>, TNewTrigger extends IAosTriggerBuilt<TNewClient, TNewContext, TNewStores> = IAosTriggerBuilt<TNewClient, TNewContext, TNewStores>>(): IAosAppBuilder<TNewClient, TNewContext, TNewStores, TNewTrigger> {
    return new AosApp<TNewClient, TNewContext, TNewStores>();
  }

  /** {@inheritDoc IAosAppBuilder.withLayout} */
  withLayout(component: RouteComponent): IAosAppBuilder<TClient, TContext, TStores, TTrigger> {
    this._config.layout = component;
    return this;
  }

  /** {@inheritDoc IAosAppBuilder.withNotFoundComponent} */
  withNotFoundComponent(component: RouteComponent): IAosAppBuilder<TClient, TContext, TStores, TTrigger> {
    this._config.notFound = component;
    return this;
  }

  /** {@inheritDoc IAosAppBuilder.withNotAuthorizedComponent} */
  withNotAuthorizedComponent(component: RouteComponent): IAosAppBuilder<TClient, TContext, TStores, TTrigger> {
    this._config.notAuthorized = component;
    return this;
  }

  /** {@inheritDoc IAosAppBuilder.withDefaultPreload} */
  withDefaultPreload(intent: AosAppConfig<TClient, TContext, TStores>["defaultPreload"]): IAosAppBuilder<TClient, TContext, TStores, TTrigger> {
    this._config.defaultPreload = intent;
    return this;
  }

  /** {@inheritDoc IAosAppBuilder.withClient} */
  withClient<TNewClient>(client: TNewClient): IAosAppBuilder<TNewClient, TContext, TStores, IAosTriggerBuilt<TNewClient, TContext, TStores>> {
    this._config = { ...this._config };
    this._config.client = client as any;
    return this as unknown as IAosAppBuilder<TNewClient, TContext, TStores>;
  }

  /** {@inheritDoc IAosAppBuilder.withContext} */
  withContext<TNewContext>(fn: (args: { client: TClient; stores: TStores; request: any }) => Promise<TNewContext> | TNewContext): IAosAppBuilder<TClient, TNewContext, TStores, IAosTriggerBuilt<TClient, TNewContext, TStores>> {
    this._config = { ...this._config };
    this._config.contextFn = fn as any;
    return this as unknown as IAosAppBuilder<TClient, TNewContext, TStores>;
  }

  /** {@inheritDoc IAosAppBuilder.withStores} */
  withStores<TNewStores extends AosStoresCollection<any> = AosStoresCollection<any>>(
    stores: TNewStores,
    handler?: (args: { client: TClient; stores: TNewStores; request: any }) => Promise<void> | void
  ): IAosAppBuilder<TClient, TContext, TNewStores, IAosTriggerBuilt<TClient, TContext, TNewStores>> {
    this._config = { ...this._config };
    this._config.stores = stores as any;
    this._config.storesInitHandler = handler as any;
    return this as unknown as IAosAppBuilder<TClient, TContext, TNewStores>;
  }

  /** {@inheritDoc IAosAppBuilder.withTriggers} */
  withTriggers<TNewTrigger extends IAosTriggerBuilt<TClient, TContext, TStores>>(trigger: TNewTrigger): IAosAppBuilder<TClient, TContext, TStores, TNewTrigger> {
    this._config = { ...this._config };
    this._config.triggers = trigger as any;
    return this as unknown as IAosAppBuilder<TClient, TContext, TStores, TNewTrigger>;
  }

  /** {@inheritDoc IAosAppBuilder.withBeforePageLoader} */
  withBeforePageLoader(fn: AosAppConfig<TClient, TContext, TStores, TTrigger>["beforePageLoad"]): IAosAppBuilder<TClient, TContext, TStores, TTrigger> {
    this._config.beforePageLoad = fn;
    return this;
  }

  /** {@inheritDoc IAosAppBuilder.withPageLoader} */
  withPageLoader(fn: AosAppConfig<TClient, TContext, TStores, TTrigger>["onPageLoad"]): IAosAppBuilder<TClient, TContext, TStores, TTrigger> {
    this._config.onPageLoad = fn;
    return this;
  }

  /** {@inheritDoc IAosAppBuilder.build} */
  build(): AosAppBuilt<TClient, TContext, TStores, any> {
    const config = this._config;

    // [Business Rule]: Initialize the root route with a beforeLoad hook to populate global context.
    const rootRoute = createRootRouteWithContext<{
      client: TClient;
      context: TContext;
    }>()({
      component: this._config.layout,
      notFoundComponent: this._config.notFound,
      staleTime: 60000,
      beforeLoad: async (ctx) => {
        if (config.stores) {
          const storesCollection = config.stores as AosStoresCollection<any>;
          const request = {
            url: ctx.location.href,
            query: ctx.search,
          };

          if (config.storesInitHandler) {
            await config.storesInitHandler({
              client: config.client!,
              stores: storesCollection,
              request,
            });
          }

          for (const [key, store] of Object.entries(storesCollection as Record<string, AosStoreBuilt<any, any>>)) {
            if (key === "namespace") {
              continue;
            }

            if (store.isInitialized) {
              continue;
            }

            if (!(store instanceof AosStoreBuilt)) {
              continue;
            }

            await store.init();
          }
        }

        if (config.contextFn) {
          const context = await config.contextFn({
            client: config.client!,
            stores: config.stores as TStores,
            request: {
              url: ctx.location.href,
              query: ctx.search,
            },
          });
          return { context };
        }
        return { context: {} as TContext };
      },
    });

    // [Return]: Return the built application factories with proper type inference.
    // IMPORTANT: We cast to preserve the exact typeof rootRoute for full TypeScript inference.
    return {
      rootRoute,
      config: this._config as AosAppConfig<TClient, TContext, TStores>,
      client: this._config.client as TClient,
      stores: this._config.stores as TStores,

      layout: () => {
        return new AosLayout<typeof rootRoute, TClient, TContext, TStores>(
          rootRoute,
          this._config
        );
      },

      middleware: () => {
        return new AosMiddleware<TClient, TContext, TStores, any>(this._config);
      },

      page: <TNewPath extends string>(path: TNewPath): AosPage<TNewPath, typeof rootRoute, TClient, TContext, TStores, z.ZodTypeAny, any> => {
        return new AosPage<TNewPath, typeof rootRoute, TClient, TContext, TStores, z.ZodTypeAny, any>(
          path,
          rootRoute,
          this._config
        );
      },

      useContext: () => {
        // Obtenção síncrona do contexto global resolvido pelo beforeLoad da rota raiz
        const { context } = rootRoute.useRouteContext();
        return context;
      },

      triggers: this._config.triggers?.initialize(this._config as any) || {
        list: async () => [],
        trigger: async () => { },
        use: () => ({ mutate: async () => { }, data: undefined, error: null, isLoading: false })
      },
      commands: this._config.triggers?.initialize(this._config as any) || {
        list: async () => [],
        trigger: async () => { },
        use: () => ({ mutate: async () => { }, data: undefined, error: null, isLoading: false })
      },

      useQueryClient: useQueryClient<unknown>,

      useForm: <TPath extends MutationPath<TClient>, TSchema extends z.ZodTypeAny>(
        options: AosUseFormOptions<TClient, TPath, TSchema>
      ): AosFormReturn<TSchema> => {
        const {
          schema,
          values,
          mutation,
          params,
          query,
          mode = "onSubmit",
          debounce = 1200,
          preventNavigation = false,
          onSubmit,
          onResponse
        } = options;

        const [isLoading, setIsLoading] = useState(false);
        const hasSubmittedSuccessfullyRef = useRef(false);
        const { client } = this._config;

        const form = useHookForm<z.infer<TSchema> & Record<string, any>>({
          resolver: zodResolver(schema as any) as any,
          defaultValues: values as any,
          shouldUnregister: false,
          mode: mode === "onChange" ? "onChange" : mode === "onBlur" ? "onBlur" : "onSubmit",
        });

        // Block navigation if form is dirty and not submitting
        useBlocker({
          disabled: !preventNavigation,
          shouldBlockFn: () => {
            if (!preventNavigation) return false;
            return form.formState.isDirty && !form.formState.isSubmitting;
          },
        });

        const executeMutation = useCallback(
          async (data: z.infer<TSchema> & Record<string, any>) => {
            setIsLoading(true);
            try {
              let result: any = null;

              if (mutation) {
                // Parse mutation path "controller.action"
                const [controller, action] = (mutation as string).split(".");
                const clientController = (client as any)[controller];

                if (!clientController || !clientController[action] || !clientController[action].mutate) {
                  throw new Error(`Mutation path ${mutation as string} is invalid or not found on the client.`);
                }

                // Build the input payload
                let payload: any = {};

                if (onSubmit) {
                  payload = await onSubmit(data);
                } else {
                  // Default heuristic: map form values to 'body' and merge with 'params'
                  payload = {
                    body: data,
                    params: params || {},
                    query: query || {}
                  };
                }

                // Call the mutate function
                const mutateFn = clientController[action].mutate;
                result = await mutateFn(payload);
              } else if (onSubmit) {
                // If no mutation path, just execute the custom onSubmit
                result = await onSubmit(data);
              } else {
                throw new Error("You must provide either a 'mutation' path or an 'onSubmit' handler.");
              }

              if (onResponse) {
                onResponse({ data: result });
              }

              // Prefer the authoritative mutation result over the submitted payload.
              // Submitted data can be stale when derived fields change server-side
              // (e.g. theme preset swaps that reload accent/surface/ink from API).
              const nextValues =
                result !== undefined && result !== null ? result : data;

              hasSubmittedSuccessfullyRef.current = true;

              form.reset(nextValues, {
                keepErrors: true,
                keepIsSubmitted: true,
                keepSubmitCount: true,
              });
            } catch (err: any) {
              // Handle validation errors or normal errors
              if (err.code === "ERR_BAD_REQUEST" && err.data?.issues) {
                err.data.issues.forEach((issue: any) => {
                  const path = Array.isArray(issue.path) ? issue.path.join(".") : String(issue.path || "");
                  if (path) {
                    form.setError(path as any, { message: issue.message });
                  }
                });
              }

              if (onResponse) {
                onResponse({ error: err instanceof Error ? err : new Error(String(err)) });
              }
            } finally {
              setIsLoading(false);
            }
          },
          [client, mutation, params, query, onSubmit, onResponse, form]
        );

        const executeMutationRef = useRef(executeMutation);
        const previousExternalValuesRef = useRef(values);
        useEffect(() => {
          executeMutationRef.current = executeMutation;
        }, [executeMutation]);

        useEffect(() => {
          if (!values) return;
          if (
            previousExternalValuesRef.current &&
            isDeepEqualData(values, previousExternalValuesRef.current)
          ) {
            return;
          }

          previousExternalValuesRef.current = values;

          const currentValues = form.getValues();
          if (isDeepEqualData(values, currentValues)) {
            hasSubmittedSuccessfullyRef.current = false;
            return;
          }

          if (form.formState.isDirty) {
            // #endregion
            return;
          }

          if (hasSubmittedSuccessfullyRef.current) {
            return;
          }

          // #endregion
          form.reset(values as any, {
            keepErrors: true,
            keepTouched: true,
            keepIsSubmitted: form.formState.isSubmitted,
            keepSubmitCount: true,
          });
        }, [values, form, mode]);

        useEffect(() => {
          if (mode !== "onChange" && mode !== "onBlur") return;

          let timer: ReturnType<typeof setTimeout>;

          const subscription = form.watch((value, { name, type, values }) => {
            // Ignora eventos sem nome (como reset do form) para evitar loops infinitos
            if (!name) return;

            if (mode === "onChange" || mode === "onBlur") {
              clearTimeout(timer);
              timer = setTimeout(() => {
                // #endregion
                form.handleSubmit(executeMutationRef.current)();
              }, debounce);
            }
          });

          return () => {
            clearTimeout(timer);
            subscription.unsubscribe();
          };
        }, [mode, debounce, form]);

        const submit = form.handleSubmit(executeMutation);

        // Keep the public form object referentially stable across renders.
        // Consumers often place `form` in effect dependencies, and returning a
        // freshly spread object causes those effects to run on every field edit.
        return Object.assign(form, {
          isLoading,
          submit,
        }) as AosFormReturn<TSchema>;
      }

    } as unknown as AosAppBuilt<TClient, TContext, TStores, any>;
  }
}
