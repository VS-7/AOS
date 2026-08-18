import { z } from "zod";
import { AosResponse } from "./response";
import { DefaultContext, AosAppConfig, InferContextIn } from "./types";

/**
 * Builder for creating middleware-like procedures that extend the application context.
 * 
 * @template TClient - The API client type.
 * @template TContext - The application context type.
 * @template TOptions - Zod schema for input options.
 * @template TContextOut - The context produced by this procedure.
 */
export class AosMiddleware<
  TClient = any,
  TContext = DefaultContext,
  TStores = any,
  TOptions extends z.ZodTypeAny = z.ZodTypeAny,
  TContextOut = any
> {
  private _options?: TOptions;
  private _loader?: any;

  constructor(public config: AosAppConfig<TClient, TContext, TStores>) { }

  /**
   * Configures input options for the procedure using Zod.
   */
  withOptions<TSchema extends z.ZodTypeAny>(schema: TSchema) {
    const builder = new AosMiddleware<TClient, TContext, TStores, TSchema, TContextOut>(
      this.config
    );
    builder._loader = this._loader;
    builder._options = schema;
    return builder;
  }

  /**
   * Defines the logic to be executed during the route lifecycle.
   * @param loader - Function that returns context updates.
   */
  withLoader<TOut>(
    loader: (args: {
      context: TContext;
      client: TClient;
      options: z.infer<TOptions>;
      response: AosResponse;
      request: { url: string; query: any; params: Record<string, string> };
      stores: TStores;
    }) => Promise<TOut> | TOut
  ) {
    const builder = new AosMiddleware<TClient, TContext, TStores, TOptions, TOut>(
      this.config
    );
    builder._options = this._options;
    builder._loader = loader;
    return builder;
  }

  /**
   * Finalizes the procedure configuration.
   */
  build() {
    return (options?: z.infer<TOptions>) => {
      return {
        options,
        loader: this._loader,
      };
    };
  }
}
