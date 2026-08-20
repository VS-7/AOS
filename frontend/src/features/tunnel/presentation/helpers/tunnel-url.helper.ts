/**
 * Pure Cloudflare Tunnel URL helpers for presentation surfaces.
 *
 * Hostname normalization and public HTTPS URL construction for settings UI
 * and other tunnel presentation consumers. Backend services keep equivalent
 * private `_snake_case` methods — do not import this helper from services.
 */
export class TunnelUrlHelper {
  /**
   * Normalizes a tunnel hostname to `hostname[:port]` without protocol or trailing slash.
   *
   * @param hostname - Raw hostname or URL from config.
   * @returns Normalized hostname string (empty when input is blank).
   */
  public static normalizeHostname(hostname: string): string {
    // [Data Transformation]: Strip protocol / trailing slashes into hostname:port.
    const trimmed = hostname.trim();
    if (!trimmed) return "";

    try {
      const parsed = trimmed.includes("://")
        ? new URL(trimmed)
        : new URL(`https://${trimmed}`);
      return parsed.hostname + (parsed.port ? `:${parsed.port}` : "");
    } catch {
      return trimmed.replace(/^https?:\/\//i, "").replace(/\/+$/, "");
    }
  }

  /**
   * Builds a public HTTPS URL from a hostname.
   *
   * @param hostname - Hostname (may already include a protocol).
   * @returns Absolute public URL with no trailing slash.
   */
  public static buildPublicUrl(hostname: string): string {
    // [Data Transformation]: Ensure https:// prefix for public tunnel URLs.
    const normalized = TunnelUrlHelper.normalizeHostname(hostname);
    if (!normalized) return "";

    if (/^https?:\/\//i.test(normalized)) {
      return normalized.replace(/\/+$/, "");
    }

    return `https://${normalized}`;
  }

  /**
   * Builds a config-change fingerprint from hostname + token.
   *
   * @param hostname - Tunnel hostname.
   * @param token - Cloudflare tunnel token.
   * @returns Fingerprint string used to detect config drift.
   */
  public static fingerprint(hostname: string, token: string): string {
    // [Data Transformation]: Concatenate normalized hostname with trimmed token.
    return `${TunnelUrlHelper.normalizeHostname(hostname)}::${token.trim()}`;
  }
}
