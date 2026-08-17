/**
 * @class Slug
 * @description A static helper class dedicated to string normalization and slug generation, ensuring consistent URL-friendly identifiers across the system.
 */
export class Slug {
  /**
   * @method generate
   * @description Transforms a raw string into a URL-friendly slug. Normalizes Unicode characters and removes non-alphanumeric punctuation.
   * @param {string} text - The raw string to be slugified.
   * @returns {string} The normalized slug.
   * @example
   * Slug.generate("Hello World!") // "hello-world"
   * Slug.generate("Café au Lait") // "cafe-au-lait"
   */
  static generate(text: string): string {
    // [Validation]: Ensure the input is a valid string
    if (!text || typeof text !== "string") {
      return "";
    }

    // [Data Transformation]: Normalize Unicode characters (accents/diacritics)
    const normalized = text.normalize("NFD").replace(/[\u0300-\u036f]/g, "");

    // [Return]: Slugify the resulting string by lowercasing, trimming, and replacing special characters
    return normalized
      .toString()
      .toLowerCase()
      .trim()
      .replace(/\s+/g, "-") // Replace spaces with -
      .replace(/[^\w-]+/g, "") // Remove all non-word chars except hyphens
      .replace(/--+/g, "-") // Replace multiple - with single -
      .replace(/^-+/, "") // Trim - from start of text
      .replace(/-+$/, ""); // Trim - from end of text
  }
}
