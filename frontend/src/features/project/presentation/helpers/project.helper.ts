import type { FractalProject } from "@/features/project/interfaces/project.interfaces";

/**
 * @class ProjectHelper
 * @description Provides standardized helpers for project management and UI rendering.
 */
export class ProjectHelper {
  /**
   * Whether `icon` stores an image (URL / data URI) instead of a Lucide name.
   *
   * @param icon - Raw project.icon value.
   * @returns True when the value should render as an `<img>`.
   */
  public static isImageIcon(icon?: string | null): boolean {
    if (!icon?.trim()) return false;
    const value = icon.trim();
    return (
      value.startsWith("data:image/") ||
      value.startsWith("http://") ||
      value.startsWith("https://") ||
      value.startsWith("blob:")
    );
  }

  /**
   * Returns the Lucide icon name for a project, or a default.
   * Image values are returned as-is so callers can pass them to {@link Icon}.
   *
   * @param icon - The icon name or image from the project.
   * @returns Lucide name, image URI, or `"Folder"`.
   */
  public static getIcon(icon?: string): string {
    return icon?.trim() || "Folder";
  }

  /**
   * Short label for property panels when `icon` is an image.
   *
   * @param icon - Raw project.icon value.
   * @returns Human-readable label (never a raw data URI).
   */
  public static getIconLabel(icon?: string | null): string {
    if (!icon?.trim()) return "Folder";
    if (ProjectHelper.isImageIcon(icon)) return "Custom image";
    return icon.trim();
  }
}
