export const SYSTEM_FONT_VALUE = "__system__";

export function toStoredFont(font?: string): string | null | undefined {
  if (font === undefined) return undefined;
  if (!font || font === SYSTEM_FONT_VALUE) return null;
  return font;
}

export function fromStoredFont(font?: string | null): string {
  if (font == null || font === "" || font === SYSTEM_FONT_VALUE) {
    return SYSTEM_FONT_VALUE;
  }
  return font;
}
