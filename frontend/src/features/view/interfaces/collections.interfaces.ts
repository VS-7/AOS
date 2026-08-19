/**
 * O que `features/view` usava de @igniter-js/collections, declarado aqui.
 *
 * O domínio view está dormente — o renderizador declarativo não tem backend
 * Go ainda. Quando tiver, este contrato passa a ser verificado contra ele;
 * por ora é o mínimo que faz a tela compilar.
 *
 * Shapes derived from how `presentation/helpers/view-data.helper.ts` and
 * `presentation/pages/($view)/index.tsx` actually use them (`result.spec.
 * root`, `result.spec.elements`, `result.view`, `result.renderedAt`), not
 * guessed from the original package's own types (unavailable — the package
 * isn't installed and never will be).
 */
export interface Spec {
  root: string;
  elements: Record<string, { type: string; props?: Record<string, unknown>; [key: string]: unknown }>;
  state?: Record<string, unknown>;
  [key: string]: unknown;
}

export interface CollectionViewRenderResult {
  view?: unknown;
  spec?: Spec;
  renderedAt?: string;
  [key: string]: unknown;
}

export interface CollectionViewActionResult {
  success?: boolean;
  result?: unknown;
  [key: string]: unknown;
}
