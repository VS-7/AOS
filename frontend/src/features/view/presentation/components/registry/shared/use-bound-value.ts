import { useStateStore } from "@json-render/react";

/**
 * Reads/writes a prop through json-render `$bindState` bindings when present.
 */
export function useBoundValue<T>(
  bindings: Record<string, string> | undefined,
  propKey: string,
  propValue: T,
): [T, (next: T) => void] {
  const { get, set } = useStateStore();
  const path = bindings?.[propKey];

  if (!path) {
    return [propValue, () => undefined];
  }

  const bound = get(path) as T;
  return [
    bound !== undefined ? bound : propValue,
    (next: T) => {
      set(path, next);
    },
  ];
}
