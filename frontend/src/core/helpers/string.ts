/**
 * A utility class providing string manipulation and transformation methods.
 * Contains various static methods for common string operations with TypeScript support.
 *
 * @remarks
 * This class offers a collection of methods for string case conversion, manipulation,
 * and validation. All methods are static and can be used without instantiation.
 */
export class String {
  /**
   * Converts a string to title case by capitalizing the first letter of each word.
   *
   * @param input - The string to convert
   * @returns The string with the first letter of each word capitalized
   *
   * @example
   * ```typescript
   * String.capitalize('hello world'); // 'Hello World'
   * String.capitalize('john doe'); // 'John Doe'
   * ```
   */
  static capitalize(input: string): string {
    return input
      .toLowerCase()
      .replace(/\b\w/g, (letter) => letter.toUpperCase())
  }

  /**
   * Extracts the initials from a given string by taking the first letter of each word.
   *
   * @param input - The string to extract initials from
   * @returns A string containing the initials
   *
   * @example
   * ```typescript
   * String.getInitials('John Doe'); // 'JD'
   * String.getInitials('User Interface Design'); // 'UID'
   * ```
   */
  static getInitials(input: string): string {
    const words = input.split(' ')
    return words.map((word) => word.charAt(0)).join('')
  }

  /**
   * Converts a string to camelCase format.
   *
   * @param input - The string to convert
   * @returns The string in camelCase format
   *
   * @example
   * ```typescript
   * String.toCamelCase('hello world'); // 'helloWorld'
   * String.toCamelCase('user-interface'); // 'userInterface'
   * ```
   */
  static toCamelCase(input: string): string {
    return input
      .toLowerCase()
      .replace(/(?:^\w|[A-Z]|\b\w|\s+)/g, (match, index) =>
        index === 0 ? match.toLowerCase() : match.toUpperCase(),
      )
      .replace(/\s+/g, '')
  }

  /**
   * Converts a string to snake_case format.
   *
   * @param input - The string to convert
   * @returns The string in snake_case format
   *
   * @example
   * ```typescript
   * String.toSnakeCase('hello world'); // 'hello_world'
   * String.toSnakeCase('userInterface'); // 'user_interface'
   * ```
   */
  static toSnakeCase(input: string): string {
    return input
      .replace(/\W+/g, ' ')
      .split(/ |\B(?=[A-Z])/)
      .map((word) => word.toLowerCase())
      .join('_')
  }

  /**
   * Converts a string to kebab-case format.
   *
   * @param input - The string to convert
   * @returns The string in kebab-case format
   *
   * @example
   * ```typescript
   * String.toKebabCase('hello world'); // 'hello-world'
   * String.toKebabCase('userInterface'); // 'user-interface'
   * ```
   */
  static toKebabCase(input: string): string {
    return input
      .replace(/\W+/g, ' ')
      .split(/ |\B(?=[A-Z])/)
      .map((word) => word.toLowerCase())
      .join('-')
  }

  /**
   * Checks if a string is a palindrome (reads the same forwards and backwards).
   *
   * @param input - The string to check
   * @returns True if the string is a palindrome, false otherwise
   *
   * @example
   * ```typescript
   * String.isPalindrome('racecar'); // true
   * String.isPalindrome('hello'); // false
   * String.isPalindrome('A man a plan a canal Panama'); // true
   * ```
   */
  static isPalindrome(input: string): boolean {
    const normalized = input.toLowerCase().replace(/[\W_]/g, '')
    return normalized === normalized.split('').reverse().join('')
  }

  /**
   * Truncates a string to a specified length and adds an ellipsis if truncated.
   *
   * @param input - The string to truncate
   * @param length - The maximum length of the truncated string
   * @returns The truncated string with an ellipsis if necessary
   *
   * @example
   * ```typescript
   * String.truncate('hello world', 5); // 'hello...'
   * String.truncate('short', 10); // 'short'
   * ```
   */
  static truncate(input: string, length: number): string {
    return input.length > length ? `${input.substring(0, length)}...` : input
  }

  /**
   * Capitalizes the first character of a string.
   *
   * @param input - The string to capitalize
   * @returns The string with its first character capitalized
   *
   * @example
   * ```typescript
   * String.capitalizeFirst('hello'); // 'Hello'
   * String.capitalizeFirst('world'); // 'World'
   * ```
   */
  static capitalizeFirst(input: string): string {
    if (input.length === 0) return input
    return input.charAt(0).toUpperCase() + input.slice(1)
  }

  /**
   * Converts the first character of a string to lowercase.
   *
   * @param input - The string to convert
   * @returns The string with its first character in lowercase
   *
   * @example
   * ```typescript
   * String.decapitalizeFirst('Hello'); // 'hello'
   * String.decapitalizeFirst('World'); // 'world'
   * ```
   */
  static decapitalizeFirst(input: string): string {
    if (input.length === 0) return input
    return input.charAt(0).toLowerCase() + input.slice(1)
  }

  /**
   * Replaces all occurrences of a substring within a string.
   *
   * @param input - The string to search within
   * @param search - The substring to search for
   * @param replacement - The string to replace the substring with
   * @returns The string with all occurrences of the substring replaced
   *
   * @example
   * ```typescript
   * String.replaceAll('hello world world', 'world', 'there'); // 'hello there there'
   * String.replaceAll('a-b-c', '-', '_'); // 'a_b_c'
   * ```
   */
  static replaceAll(
    input: string,
    search: string,
    replacement: string,
  ): string {
    return input.split(search).join(replacement)
  }

  /**
   * Generates a URL-friendly slug from a string
   * @param text - The text to convert into a slug
   * @returns The generated slug string
   */
  static toSlug(text: string): string {
    if (!text) return ''

    return text
      .toString()
      .toLowerCase()
      .normalize('NFD')
      .replace(/[\u0300-\u036f]/g, '')
      .replace(/[^a-z0-9\s-]/g, '')
      .replace(/\s+/g, '-')
      .replace(/-+/g, '-')
      .trim()
  }

  /**
   * Converts a category slug to a human-readable label by replacing hyphens with spaces
   * and capitalizing each word.
   *
   * @param slug - The category slug to convert (e.g., "account-management")
   * @returns The formatted label (e.g., "Account Management")
   *
   * @example
   * ```typescript
   * String.formatCategoryLabel('account-management'); // 'Account Management'
   * String.formatCategoryLabel('api-keys'); // 'Api Keys'
   * String.formatCategoryLabel('getting-started'); // 'Getting Started'
   * ```
   */
  static formatCategoryLabel(slug: string): string {
    if (!slug) return ''

    return slug
      .split('-')
      .map((word) => this.capitalizeFirst(word))
      .join(' ')
  }
}
