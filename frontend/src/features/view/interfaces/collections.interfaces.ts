/**
 * O que `features/view` usava de @igniter-js/collections, declarado aqui.
 *
 * O domínio view está dormente — o renderizador declarativo não tem backend
 * Go ainda. Quando tiver, este contrato passa a ser verificado contra ele.
 *
 * `Spec` now defers to the real type from @json-render/core (installed).
 * Result wrapper types below may contain partially-formed or corrupted specs
 * (e.g., circular refs serialized as "[Circular]" strings), so they are
 * typed loosely to match the server-side shape from Igniter.
 */
export type { Spec } from "@json-render/core";

export interface CollectionViewRenderResult {
  view?: unknown;
  spec?: {
    root?: string;
    elements?: Record<string, unknown>;
    state?: Record<string, unknown>;
    [key: string]: unknown;
  };
  renderedAt?: string;
  [key: string]: unknown;
}

export interface CollectionViewActionResult {
  success?: boolean;
  result?: unknown;
  [key: string]: unknown;
}
