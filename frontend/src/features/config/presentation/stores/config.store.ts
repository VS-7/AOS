import { AosStore } from "@/app/builders/store";
import {
  FractalConfig,
  FRACTAL_DEFAULT_CONFIG,
  FractalConfigUpdateInput,
} from "@/features/config/interfaces/config.interfaces";
import { api } from "@/lib/aos-facade";

function unwrapConfig(data: unknown): FractalConfig | null {
  if (!data || typeof data !== "object") return null;
  if ("config" in data && data.config && typeof data.config === "object") {
    return data.config as FractalConfig;
  }
  return data as FractalConfig;
}

export const ConfigStore = AosStore.create("config")
  .withState<FractalConfig>(FRACTAL_DEFAULT_CONFIG)
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
  .addAction("update", (ctx) => async (params: FractalConfigUpdateInput) => {
    const response = await api.config.update.mutate({ body: params });
    const data = unwrapConfig(response.data);
    if (data) {
      ctx.state.set(data);
      return data;
    }
    return ctx.state.get();
  })
  .build();
