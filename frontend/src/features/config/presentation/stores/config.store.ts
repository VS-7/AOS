import { AosStore } from "@/app/builders/store";
import {
  Config,
  AOS_DEFAULT_CONFIG,
  ConfigUpdateInput,
} from "@/features/config/interfaces/config.interfaces";
import { api } from "@/lib/aos-facade";

function unwrapConfig(data: unknown): Config | null {
  if (!data || typeof data !== "object") return null;
  if ("config" in data && data.config && typeof data.config === "object") {
    return data.config as Config;
  }
  return data as Config;
}

export const ConfigStore = AosStore.create("config")
  .withState<Config>(AOS_DEFAULT_CONFIG)
  .withPreload(async (ctx) => {
    const response = await api.config.get.query();
    const data = unwrapConfig(response.data);
    if (data) return data;
    return ctx.state.get();
  })
  .addAction("refresh", (ctx) => async () => {
    const response = await api.config.get.query();
    const data = unwrapConfig(response.data);
    if (data) {
      ctx.state.set(data);
      return data;
    }
    return ctx.state.get();
  })
  .addAction("update", (ctx) => async (params: ConfigUpdateInput) => {
    const response = await api.config.update.mutate({ body: params });
    const data = unwrapConfig(response.data);
    if (data) {
      ctx.state.set(data);
      return data;
    }
    return ctx.state.get();
  })
  .build();
