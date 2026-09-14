import { AosStore } from "@/app/builders/store";
import {
  Config,
  AOS_DEFAULT_CONFIG,
  ConfigUpdateInput,
} from "@/features/config/interfaces/config.interfaces";
import { api } from "@/lib/aos-facade";
import { applyConfiguredLocale } from "@/lib/i18n";
import { replacing } from "@/app/builders/replace-state";

function unwrapConfig(data: unknown): Config | null {
  if (!data || typeof data !== "object") return null;
  if ("config" in data && data.config && typeof data.config === "object") {
    return data.config as Config;
  }
  return data as Config;
}

/**
 * Puts the interface in the language the installation was configured with.
 *
 * Onboarding asks for a language and writes it to `region.language`; without
 * this the answer only ever reached the agents, and the screens the person
 * chose it on stayed in English. See `applyConfiguredLocale`.
 */
function adoptLanguage(config: Config | null): void {
  applyConfiguredLocale(config?.region?.language);
}

export const ConfigStore = AosStore.create("config")
  .withState<Config>(AOS_DEFAULT_CONFIG)
  .withPreload(async (ctx) => {
    const response = await api.config.get.query();
    const data = unwrapConfig(response.data);
    adoptLanguage(data);
    if (data) return data;
    return ctx.state.get();
  })
  .addAction("refresh", (ctx) => async () => {
    const response = await api.config.get.query();
    const data = unwrapConfig(response.data);
    adoptLanguage(data);
    if (data) {
      ctx.state.set(replacing(ctx.state.get(), data));
      return data;
    }
    return ctx.state.get();
  })
  /**
   * Takes the configuration a `config_update` answered with as the state,
   * without asking again.
   *
   * The settings forms saved through the facade and never wrote back here,
   * so returning to General or Profile showed the values from before the
   * save, and the next autosave sent them — undoing it. `replacing`, because
   * the answer omits an emptied field and a merge would keep the old one.
   */
  .addAction("adopt", (ctx) => (config: Config | null | undefined) => {
    const data = unwrapConfig(config);
    if (!data) return;
    ctx.state.set(replacing(ctx.state.get(), data));
    adoptLanguage(data);
  })
  .addAction("update", (ctx) => async (params: ConfigUpdateInput) => {
    // A refusal used to fall through to "return the current state", which
    // the caller (the model-slot picker) cannot tell from a save: the slot
    // it had already shown as chosen stayed on screen, unsaved, in silence.
    const data = unwrapConfig(await api.config.update.mutateOrThrow({ body: params }));
    if (data) {
      ctx.state.set(replacing(ctx.state.get(), data));
      return data;
    }
    return ctx.state.get();
  })
  .build();
