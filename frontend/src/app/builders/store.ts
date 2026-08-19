import { DeepPartial } from "ai";
import { useEffect, useRef } from "react";
import { useSyncExternalStoreWithSelector } from "use-sync-external-store/with-selector";

const IGNITER_REGISTRY_SYMBOL = "__aos_registry__";

function isPlainObject(value: unknown): value is Record<string, unknown> {
  if (typeof value !== "object" || value === null || Array.isArray(value)) {
    return false;
  }

  if (value instanceof Map || value instanceof Set) {
    return false;
  }

  if (isRegistryObject(value)) {
    return false;
  }

  const prototype = Object.getPrototypeOf(value);
  return prototype === Object.prototype || prototype === null;
}

export type AosRegistry<TItem> = {
  [IGNITER_REGISTRY_SYMBOL]: true;
  data: Record<string, TItem>;
};

type AosPrimitive = bigint | boolean | null | number | string | symbol | undefined;

export type AosSerialized<TValue> = TValue extends (...args: any[]) => any
  ? never
  : TValue extends Date
  ? string
  : TValue extends AosPrimitive
  ? TValue
  : TValue extends Array<infer TItem>
  ? AosSerialized<TItem>[]
  : TValue extends object
  ? {
    [K in keyof TValue as AosSerialized<TValue[K]> extends never ? never : K]:
    AosSerialized<TValue[K]>;
  }
  : never;

function isRegistryObject(value: unknown): value is AosRegistry<any> {
  return (
    typeof value === "object" &&
    value !== null &&
    IGNITER_REGISTRY_SYMBOL in value &&
    "data" in value
  );
}

export type AosDeepPartial<T> = T extends AosRegistry<any>
  ? T
  : T extends (infer U)[]
  ? AosDeepPartial<U>[]
  : T extends object
  ? { [K in keyof T]?: AosDeepPartial<T[K]> }
  : T;

function mergeStoreState<TState>(currentState: TState, partialState: AosDeepPartial<TState>): TState {
  if (Array.isArray(partialState)) {
    return partialState as TState;
  }

  if (!isPlainObject(currentState) || !isPlainObject(partialState)) {
    return (partialState ?? currentState) as TState;
  }

  const nextState: Record<string, unknown> = { ...currentState };

  for (const [key, nextValue] of Object.entries(partialState)) {
    const currentValue = (currentState as Record<string, unknown>)[key];

    if (Array.isArray(nextValue)) {
      nextState[key] = nextValue;
      continue;
    }

    if (isPlainObject(currentValue) && isPlainObject(nextValue)) {
      nextState[key] = mergeStoreState(currentValue, nextValue);
      continue;
    }

    // If the key exists in a patch, apply it even when value is undefined.
    // This allows explicit state clearing (e.g. set({ current: undefined })).
    nextState[key] = nextValue;
  }

  return nextState as TState;
}

export type SetStateAction<TState> = AosDeepPartial<TState> | ((state: TState) => AosDeepPartial<TState>);

export type AosStoreNamespaceValues = Record<string, string | undefined>;

export type AosStoreNamespaceStrategy = "memory-partition" | "rehydrate";

export interface AosNamespaceManager {
  get: () => AosStoreNamespaceValues;
  set: (
    next:
      | AosStoreNamespaceValues
      | ((current: AosStoreNamespaceValues) => AosStoreNamespaceValues)
  ) => Promise<AosStoreNamespaceValues>;
}

export interface RegistryHelper<TItem> {
  count: () => number;
  entries: () => Array<[string, TItem]>;
  first: () => TItem | undefined;
  get: (id: string) => TItem | undefined;
  has: (id: string) => boolean;
  keys: () => string[];
  list: {
    (): AosSerialized<TItem>[];
    <TOutput>(mapper: (value: TItem, id: string) => TOutput): TOutput[];
  };
  remove: (id: string) => AosRegistry<TItem>;
  set: (id: string, value: TItem) => AosRegistry<TItem>;
  update: (id: string, updater: (current: TItem | undefined) => TItem) => AosRegistry<TItem>;
  upsert: (
    id: string,
    create: () => TItem,
    update?: (current: TItem) => TItem
  ) => AosRegistry<TItem>;
  values: () => TItem[];
}

type RegistryItem<TRegistry> = TRegistry extends AosRegistry<infer TItem> ? TItem : never;

type Compute<T> = { [K in keyof T]: T[K] } & {};

type TRegistriesFromState<TState> = {
  [K in keyof TState]: TState[K] extends AosRegistry<infer TItem> | undefined ? TItem : never;
};

export interface StoreFactoryContext<TState, TActions> {
  actions: TActions;
  name: string;
  namespace: {
    bind: (namespaceKey?: string) => StoreFactoryContext<TState, TActions>;
    current: () => string | undefined;
  };
  notify: () => void;
  registry: <K extends keyof TState & string>(key: TState[K] extends AosRegistry<any> | undefined ? K : never) => RegistryHelper<TRegistriesFromState<TState>[K]>;
  state: {
    get: (namespaceKey?: string) => TState;
    set: (action: SetStateAction<TState>, namespaceKey?: string) => void;
  };
}

export interface AosStoreNamespaceContext {
  appPrefix: string;
  namespaces: AosStoreNamespaceValues;
  storeName: string;
}

export interface AosStoreNamespaceConfig {
  resolver: (ctx: AosStoreNamespaceContext) => string | undefined;
  strategy?: AosStoreNamespaceStrategy;
}

function serializePlainValue<TValue>(value: TValue): AosSerialized<TValue> | undefined {
  if (
    value === null ||
    value === undefined ||
    typeof value === "string" ||
    typeof value === "number" ||
    typeof value === "boolean" ||
    typeof value === "bigint" ||
    typeof value === "symbol"
  ) {
    return value as AosSerialized<TValue>;
  }

  if (typeof value === "function") {
    return undefined;
  }

  if (value instanceof Date) {
    return value.toISOString() as AosSerialized<TValue>;
  }

  if (Array.isArray(value)) {
    return value
      .map((item) => serializePlainValue(item))
      .filter((item) => item !== undefined) as AosSerialized<TValue>;
  }

  if (!isPlainObject(value)) {
    return undefined;
  }

  const serialized: Record<string, unknown> = {};

  for (const [key, entryValue] of Object.entries(value)) {
    const nextValue = serializePlainValue(entryValue);

    if (nextValue !== undefined) {
      serialized[key] = nextValue;
    }
  }

  return serialized as AosSerialized<TValue>;
}

function createRegistryHelper<TItem>(getSource: () => Record<string, TItem>, onUpdate?: (next: AosRegistry<TItem>) => void): RegistryHelper<TItem> {
  const wrap = (next: Record<string, TItem>): AosRegistry<TItem> => {
    const registry: AosRegistry<TItem> = {
      [IGNITER_REGISTRY_SYMBOL]: true,
      data: next,
    };
    onUpdate?.(registry);
    return registry;
  };

  return {
    count: () => Object.keys(getSource()).length,
    entries: () => Object.entries(getSource()),
    first: () => Object.values(getSource())[0],
    get: (id: string) => getSource()[id],
    has: (id: string) => id in getSource(),
    keys: () => Object.keys(getSource()),
    list: ((mapper?: (value: TItem, id: string) => unknown) => {
      const entries = Object.entries(getSource());

      if (mapper) {
        return entries.map(([id, value]) => mapper(value, id));
      }

      return entries
        .map(([, value]) => serializePlainValue(value))
        .filter((value) => value !== undefined);
    }) as RegistryHelper<TItem>["list"],
    remove: (id: string) => {
      const source = getSource();
      if (!(id in source)) return wrap(source);

      const next = { ...source };
      delete next[id];
      return wrap(next);
    },
    set: (id: string, value: TItem) =>
      wrap({
        ...getSource(),
        [id]: value,
      }),
    update: (id: string, updater: (current: TItem | undefined) => TItem) => {
      const source = getSource();
      return wrap({
        ...source,
        [id]: updater(source[id]),
      });
    },
    upsert: (
      id: string,
      create: () => TItem,
      update?: (current: TItem) => TItem
    ) => {
      const source = getSource();
      return wrap({
        ...source,
        [id]: source[id] !== undefined
          ? (update ? update(source[id]) : source[id])
          : create(),
      });
    },
    values: () => Object.values(getSource()),
  };
}

export interface AosStoreConfig<TState> {
  name: string;
  initialState: TState;
  namespace?: AosStoreNamespaceConfig;
  preload?: (ctx: StoreFactoryContext<TState, any>) => Promise<TState> | TState
  persistence?: { enabled: boolean; storage?: "localstorage" | "sessionstorage"; key?: string; pick?: (state: TState) => DeepPartial<TState> };
  broadcast?: { enabled: boolean; key?: string; pick?: (state: TState) => DeepPartial<TState> };
}

function cloneStoreState<TValue>(value: TValue): TValue {
  if (value === null || value === undefined) return value;

  if (Array.isArray(value)) {
    return value.map((item) => cloneStoreState(item)) as TValue;
  }

  if (value instanceof Date) {
    return new Date(value.getTime()) as TValue;
  }

  if (!isPlainObject(value)) {
    return value;
  }

  const next: Record<string, unknown> = {};

  for (const [key, entryValue] of Object.entries(value)) {
    next[key] = cloneStoreState(entryValue);
  }

  return next as TValue;
}

export class AosStoreBuilt<TState, TActions> {
  private _activeNamespace?: string;
  private _appPrefix = "aos";
  private _state: TState;
  private _initialState: TState;
  private _listeners = new Set<() => void>();
  private _actions: TActions = {} as TActions;
  private _channel?: BroadcastChannel;
  private _getNamespaces: () => AosStoreNamespaceValues = () => ({});
  private _storageKey?: string;
  private _isInitialized = false;
  private _warnedNeverInitialized = false;
  private _namespaceBuckets = new Map<string, TState>();
  private _setState: (action: SetStateAction<TState>) => void;

  constructor(
    public readonly name: string,
    initialState: TState,
    actionsFactory: Record<string, (ctx: StoreFactoryContext<TState, TActions>) => any>,
    private config: AosStoreConfig<TState>
  ) {
    this._state = cloneStoreState(initialState);
    this._initialState = cloneStoreState(initialState);

    const createContext = (boundNamespaceKey?: string): StoreFactoryContext<TState, TActions> => ({
      actions: this._actions,
      name: this.name,
      namespace: {
        bind: (namespaceKey?: string) => createContext(namespaceKey ?? boundNamespaceKey ?? this._activeNamespace),
        current: () => boundNamespaceKey ?? this._activeNamespace,
      },
      notify: this._notify.bind(this),
      registry: (key: any) => {
        return createRegistryHelper(() => {
          const currentState = this._getStateForNamespace(boundNamespaceKey) as Record<string, any>;
          const currentValue = currentState[key];
          if (currentValue === undefined) return {};
          if (!isRegistryObject(currentValue)) {
            throw new Error(`[AosStore: ${this.name}] State key "${key}" is not an Aos registry.`);
          }
          return currentValue.data;
        }, (next) => this._setStateForNamespace({ [key]: next } as any, boundNamespaceKey));
      },
      state: {
        get: (namespaceKey?: string) => this._getStateForNamespace(namespaceKey ?? boundNamespaceKey),
        set: (action: SetStateAction<TState>, namespaceKey?: string) =>
          this._setStateForNamespace(action, namespaceKey ?? boundNamespaceKey),
      },
    });

    const set = (action: SetStateAction<TState>) => {
      this._setStateForNamespace(action);
    };
    this._setState = set;

    for (const [key, factory] of Object.entries(actionsFactory)) {
      (this._actions as any)[key] = factory(createContext());
    }
  }

  private _computeScopedKey(baseKey: string, namespaceKey?: string) {
    return namespaceKey
      ? `${this._appPrefix}:${namespaceKey}:${baseKey}`
      : `${this._appPrefix}:${baseKey}`;
  }

  private _getPartitionKey(namespaceKey?: string) {
    return namespaceKey ?? "__default__";
  }

  private _resolveNamespace() {
    if (!this.config.namespace) {
      return undefined;
    }

    return this.config.namespace.resolver({
      appPrefix: this._appPrefix,
      namespaces: this._getNamespaces(),
      storeName: this.name,
    });
  }

  private _getStateForNamespace(namespaceKey?: string) {
    if (namespaceKey === undefined || namespaceKey === this._activeNamespace) {
      return this._state;
    }

    return this._namespaceBuckets.get(this._getPartitionKey(namespaceKey)) ?? cloneStoreState(this._initialState);
  }

  private _persistState(state: TState, namespaceKey?: string) {
    const persistence = this.config.persistence;
    if (!persistence?.enabled || typeof window === "undefined") {
      return;
    }

    const storage = persistence.storage === "sessionstorage" ? sessionStorage : localStorage;
    const storageKey = this._computeScopedKey(persistence.key || this.name, namespaceKey);
    const stateToPersist = persistence.pick ? persistence.pick(state) : state;
    storage.setItem(storageKey, JSON.stringify(stateToPersist));
  }

  private _broadcastState(state: TState) {
    if (!this._channel) {
      return;
    }

    const stateToBroadcast = this.config.broadcast?.pick ? this.config.broadcast.pick(state) : state;

    try {
      this._channel.postMessage(stateToBroadcast);
    } catch (e) {
      console.warn(`[AosStore: ${this.name}] Failed to broadcast state. Ensure it does not contain Map, Set, functions, or complex objects like FractalChat.`, e);
    }
  }

  private _setStateForNamespace(action: SetStateAction<TState>, namespaceKey?: string) {
    const targetNamespace = namespaceKey ?? this._activeNamespace;
    const currentState = this._getStateForNamespace(targetNamespace);
    const nextState = typeof action === "function" ? (action as any)(currentState) : action;
    const mergedState = mergeStoreState(currentState, nextState);
    const isActiveNamespace = targetNamespace === this._activeNamespace || targetNamespace === undefined;

    if (this.config.namespace?.strategy === "memory-partition") {
      this._namespaceBuckets.set(this._getPartitionKey(targetNamespace), mergedState);
    }

    if (isActiveNamespace) {
      this._state = mergedState;
      if (this._storageKey) {
        this._persistState(this._state, this._activeNamespace);
      }
      this._broadcastState(this._state);
      this._notify();
      return;
    }

    this._persistState(mergedState, targetNamespace);
  }

  private _disposeBroadcastChannel() {
    if (!this._channel) {
      return;
    }

    this._channel.close();
    this._channel = undefined;
  }

  private async _hydrateActiveNamespace() {
    const namespaceKey = this._activeNamespace;
    const partitionKey = this._getPartitionKey(namespaceKey);
    const strategy = this.config.namespace?.strategy ?? "memory-partition";

    if (strategy === "memory-partition" && this._namespaceBuckets.has(partitionKey)) {
      this._state = this._namespaceBuckets.get(partitionKey)!;
      return;
    }

    this._state = cloneStoreState(this._initialState);

    if (this.config.persistence?.enabled) {
      this._storageKey = this._computeScopedKey(this.config.persistence.key || this.name, namespaceKey);
      if (typeof window !== "undefined") {
        const storage = this.config.persistence.storage === "sessionstorage" ? sessionStorage : localStorage;
        const stored = storage.getItem(this._storageKey);
        if (stored) {
          try {
            this._state = mergeStoreState(this._state, JSON.parse(stored));
          } catch (e) {
            console.error(`Failed to parse stored state for ${this.name}`, e);
          }
        }
      }
    } else {
      this._storageKey = undefined;
    }

    if (this.config.preload) {
      try {
        this._state = await this.config.preload({
          actions: this._actions,
          name: this.name,
          namespace: {
            bind: (boundNamespaceKey?: string) => ({
              ...this._createNamespaceBoundContext(boundNamespaceKey ?? namespaceKey),
            }),
            current: () => namespaceKey,
          },
          notify: this._notify.bind(this),
          registry: this._createNamespaceBoundContext(namespaceKey).registry,
          state: this._createNamespaceBoundContext(namespaceKey).state,
        });
      } catch (e) {
        console.error(`Failed to preload state`, e)
      }
    }

    if (strategy === "memory-partition") {
      this._namespaceBuckets.set(partitionKey, this._state);
    }

    if (this.config.broadcast?.enabled) {
      const channelName = this._computeScopedKey(this.config.broadcast.key || this.name, namespaceKey);
      if (typeof window !== "undefined" && typeof BroadcastChannel !== "undefined") {
        this._channel = new BroadcastChannel(channelName);

        this._channel.onmessage = (event) => {
          this._state = { ...this._state, ...event.data };
          if (strategy === "memory-partition") {
            this._namespaceBuckets.set(partitionKey, this._state);
          }
          this._notify();
        };
      }
    }
  }

  private _createNamespaceBoundContext(namespaceKey?: string): StoreFactoryContext<TState, TActions> {
    return {
      actions: this._actions,
      name: this.name,
      namespace: {
        bind: (nextNamespaceKey?: string) => this._createNamespaceBoundContext(nextNamespaceKey ?? namespaceKey),
        current: () => namespaceKey ?? this._activeNamespace,
      },
      notify: this._notify.bind(this),
      registry: (key: any) => {
        return createRegistryHelper(() => {
          const currentState = this._getStateForNamespace(namespaceKey) as Record<string, any>;
          const currentValue = currentState[key];
          if (currentValue === undefined) return {};
          if (!isRegistryObject(currentValue)) {
            throw new Error(`[AosStore: ${this.name}] State key "${key}" is not an Aos registry.`);
          }
          return currentValue.data;
        }, (next) => this._setStateForNamespace({ [key]: next } as any, namespaceKey));
      },
      state: {
        get: (nextNamespaceKey?: string) => this._getStateForNamespace(nextNamespaceKey ?? namespaceKey),
        set: (action: SetStateAction<TState>, nextNamespaceKey?: string) =>
          this._setStateForNamespace(action, nextNamespaceKey ?? namespaceKey),
      },
    };
  }

  public _attachRuntime(options: { getNamespaces?: () => AosStoreNamespaceValues; prefix?: string }) {
    this._appPrefix = options.prefix || "aos";
    this._getNamespaces = options.getNamespaces ?? (() => ({}));
  }

  public async init(prefix?: string) {
    if (this._isInitialized) return;
    this._isInitialized = true;
    this._appPrefix = prefix || this._appPrefix;
    this._activeNamespace = this._resolveNamespace();
    await this._hydrateActiveNamespace();
  }

  public async sync() {
    if (!this._isInitialized || !this.config.namespace) {
      return;
    }

    const nextNamespace = this._resolveNamespace();
    if (nextNamespace === this._activeNamespace) {
      return;
    }

    if (this.config.namespace.strategy === "memory-partition") {
      this._namespaceBuckets.set(this._getPartitionKey(this._activeNamespace), this._state);
    }

    this._disposeBroadcastChannel();
    this._activeNamespace = nextNamespace;
    await this._hydrateActiveNamespace();
    this._notify();
  }

  private _notify() {
    this._listeners.forEach((l) => l());
  }

  private _subscribe = (listener: () => void) => {
    this._listeners.add(listener);
    return () => {
      this._listeners.delete(listener);
    };
  };

  get state() {
    return this._state;
  }

  get actions() {
    return this._actions;
  }

  get isInitialized() {
    return this._isInitialized;
  }

  useState(): TState;
  useState<TSelected>(selector: (state: TState) => TSelected, equalityFn?: (a: TSelected, b: TSelected) => boolean): TSelected;
  useState<TSelected>(selector?: (state: TState) => TSelected, equalityFn?: (a: TSelected, b: TSelected) => boolean): TState | TSelected {
    // Class-level fix from the final review's ledger triage: `ViewStore`/
    // `ArtifactStore` were read through `.useState()` as pristine
    // singletons — never passed to `AosStore.router({...})` in `app/
    // stores.ts`, so `.init()` (which runs `withPreload` and resolves the
    // namespace) never ran, and every read just saw the untouched initial
    // state forever. This is the *third* time this exact bug shipped
    // (`workspace`/`auth`/`projects`/`goals` were the first two — see
    // `app/lib/stores.ts`'s own incident writeup). Fixing only the two
    // instances again would leave a fourth store free to repeat it
    // silently; this warns from the class itself, the first time any
    // store's state is read before it was ever registered — independent
    // of which store, or when its module happens to load. Gated on
    // `config.preload` existing: a store with no preload has nothing
    // `.init()` would fetch, so never being registered costs it nothing
    // worth warning about.
    if (this.config.preload && !this._isInitialized && !this._warnedNeverInitialized) {
      this._warnedNeverInitialized = true;
      console.error(
        `[AosStore: ${this.name}] .useState() was called, but this store was never ` +
          `passed to AosStore.router({...}) in app/stores.ts — .init() never ran, so ` +
          `its preload never fetched and its namespace never resolved. It will read as ` +
          `permanently stuck at its initial state. Add "${this.name}" to that registry's ` +
          `"stores" object.`,
      );
    }

    const getSnapshot = () => this._state;

    return useSyncExternalStoreWithSelector(
      this._subscribe,
      getSnapshot,
      getSnapshot,
      selector || ((state: TState) => state as unknown as TSelected),
      equalityFn
    );
  }

  useActions() {
    return this._actions;
  }

  useSubscribe(options: { condition?: (state: TState) => boolean; on: (state: TState) => void }) {
    const optionsRef = useRef(options);
    optionsRef.current = options;

    useEffect(() => {
      const listener = () => {
        const currentState = this._state;
        if (!optionsRef.current.condition || optionsRef.current.condition(currentState)) {
          optionsRef.current.on(currentState);
        }
      };
      this._listeners.add(listener);
      return () => {
        this._listeners.delete(listener);
      };
    }, []);
  }
}

export class AosStoreBuilder<TState = any, TActions = {}> {
  private _config: AosStoreConfig<TState>;
  private _actionsFactory: Record<string, (ctx: any) => any> = {};

  constructor(name: string) {
    this._config = {
      name,
      initialState: {} as TState,
    };
  }

  withState<TNewState>(initialState: TNewState) {
    const builder = new AosStoreBuilder<TNewState, TActions>(this._config.name);
    builder._config = { ...this._config, initialState } as unknown as AosStoreConfig<TNewState>;
    builder._actionsFactory = this._actionsFactory;
    return builder;
  }

  addAction<TKey extends string, TActionFn extends (...args: any[]) => any>(
    name: TKey,
    factory: (ctx: StoreFactoryContext<TState, TActions>) => TActionFn
  ): AosStoreBuilder<TState, TActions & Record<TKey, TActionFn>> {
    this._actionsFactory[name] = factory;
    return this as any;
  }

  withPreload(preload: (ctx: StoreFactoryContext<TState, TActions>) => Promise<TState> | TState) {
    this._config.preload = preload
    return this
  }

  withNamespace(namespace: AosStoreNamespaceConfig) {
    this._config.namespace = namespace;
    return this;
  }

  withPersistence(options: { enabled?: boolean; storage?: "localstorage" | "sessionstorage"; key?: string; pick?: (state: TState) => DeepPartial<TState> }) {
    this._config.persistence = { enabled: options.enabled ?? true, storage: options.storage || "localstorage", key: options.key, pick: options.pick };
    return this;
  }

  withBroadcast(options: { enabled?: boolean; key?: string; pick?: (state: TState) => DeepPartial<TState> }) {
    this._config.broadcast = { enabled: options.enabled ?? true, key: options.key, pick: options.pick };
    return this;
  }

  build(): AosStoreBuilt<TState, TActions> {
    const store = new AosStoreBuilt<TState, TActions>(
      this._config.name,
      this._config.initialState,
      this._actionsFactory,
      this._config
    );
    return store;
  }
}

export const AosStore = {
  create: (name: string) => new AosStoreBuilder(name),
  registry: <TItem>(data?: Record<string, TItem>): AosRegistry<TItem> => ({
    [IGNITER_REGISTRY_SYMBOL]: true,
    data: data ?? {},
  }),
  router: <TStores extends Record<string, AosStoreBuilt<any, any>>>(options: { prefix?: string; stores: TStores }) => {
    if ("namespace" in options.stores) {
      throw new Error(`[AosStore] "namespace" is a reserved store key used by the runtime namespace manager.`);
    }

    const stores = options.stores;
    const prefix = options.prefix || "aos";
    let namespaces: AosStoreNamespaceValues = {};
    let queue = Promise.resolve<AosStoreNamespaceValues>(namespaces);

    const namespaceManager: AosNamespaceManager = {
      get: () => ({ ...namespaces }),
      set: (next) => {
        queue = queue.then(async () => {
          const resolved = typeof next === "function" ? next({ ...namespaces }) : next;
          const previous = namespaces;
          const hasChanged =
            Object.keys({ ...previous, ...resolved }).some((key) => previous[key] !== resolved[key]);

          if (!hasChanged) {
            return { ...namespaces };
          }

          namespaces = { ...resolved };

          await Promise.all(
            Object.values(stores).map((store) => store.sync())
          );

          return { ...namespaces };
        });

        return queue;
      },
    };

    Object.keys(stores).forEach((key) => {
      const store = stores[key];
      store._attachRuntime({
        getNamespaces: namespaceManager.get,
        prefix,
      });
    });

    const storesWithNamespace = stores as TStores & { namespace: AosNamespaceManager };
    Object.defineProperty(storesWithNamespace, "namespace", {
      configurable: false,
      enumerable: false,
      value: namespaceManager,
      writable: false,
    });

    return storesWithNamespace;
  }
}
