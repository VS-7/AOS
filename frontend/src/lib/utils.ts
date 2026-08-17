import { clsx, type ClassValue } from "clsx";
import { twMerge } from "tailwind-merge";
import { Link, File, FileText, Image, Code, Archive, Music, Video } from "lucide-react";

export function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs));
}

export function hexToOklch(hex: string): string {
  if (!hex || !hex.startsWith('#')) return hex;

  // 1. Hex to RGB
  let r = parseInt(hex.slice(1, 3), 16);
  let g = parseInt(hex.slice(3, 5), 16);
  let b = parseInt(hex.slice(5, 7), 16);

  if (hex.length === 4) {
    r = parseInt(hex.slice(1, 2).repeat(2), 16);
    g = parseInt(hex.slice(2, 3).repeat(2), 16);
    b = parseInt(hex.slice(3, 4).repeat(2), 16);
  }

  // 2. RGB to Linear sRGB
  const linear = (c: number) => {
    c /= 255;
    return c <= 0.04045 ? c / 12.92 : Math.pow((c + 0.055) / 1.055, 2.4);
  };
  const lr = linear(r);
  const lg = linear(g);
  const lb = linear(b);

  // 3. Linear sRGB to XYZ (D65)
  const l = 0.4122214708 * lr + 0.5363325363 * lg + 0.0514459929 * lb;
  const m = 0.2119034982 * lr + 0.6806995451 * lg + 0.1073969566 * lb;
  const s = 0.0883024619 * lr + 0.2817188376 * lg + 0.6299787005 * lb;

  // 4. XYZ to Oklab
  const l_ = Math.cbrt(l);
  const m_ = Math.cbrt(m);
  const s_ = Math.cbrt(s);

  const L = 0.2104542553 * l_ + 0.7936177850 * m_ - 0.0040720468 * s_;
  const a = 1.9779984951 * l_ - 2.4285922050 * m_ + 0.4505937099 * s_;
  const b_ok = 0.0259040371 * l_ + 0.7827717662 * m_ - 0.8086757660 * s_;

  // 5. Oklab to Oklch
  const C = Math.sqrt(a * a + b_ok * b_ok);
  let H = Math.atan2(b_ok, a) * (180 / Math.PI);
  if (H < 0) H += 360;

  // Handle gray colors where hue is undefined (or 0)
  if (isNaN(H)) H = 0;

  return `${L.toFixed(3)} ${C.toFixed(4)} ${H.toFixed(2)}`;
}

export function getDisplayName(uri: string, observation?: string): string {
  if (uri.startsWith('uri://')) return observation || uri
  if (uri.startsWith('/')) uri = `file:/${uri}`
  const pathname = new URL(uri).pathname;
  const filename = pathname.split('/').pop() || '';
  return filename;
}

export function getIconForExtension(uri: string) {
  if (!uri.startsWith('file://')) return Link;
  const pathname = new URL(uri).pathname;
  const filename = pathname.split('/').pop() || '';
  const extension = filename.split('.').pop()?.toLowerCase();
  switch (extension) {
    case 'pdf':
    case 'txt':
    case 'md':
      return FileText;
    case 'jpg':
    case 'jpeg':
    case 'png':
    case 'gif':
    case 'svg':
      return Image;
    case 'js':
    case 'ts':
    case 'tsx':
    case 'jsx':
      return Code;
    case 'zip':
    case 'rar':
    case 'tar':
    case 'gz':
      return Archive;
    case 'mp3':
    case 'wav':
      return Music;
    case 'mp4':
    case 'avi':
      return Video;
    default:
      return File;
  }
}

export function timeAgo(date: string | Date): string {
  const now = new Date();
  const diffInSeconds = Math.floor((now.getTime() - new Date(date).getTime()) / 1000);

  if (diffInSeconds < 60) return "just now";
  const diffInMinutes = Math.floor(diffInSeconds / 60);
  if (diffInMinutes < 60) return `${diffInMinutes}m ago`;
  const diffInHours = Math.floor(diffInMinutes / 60);
  if (diffInHours < 24) return `${diffInHours}h ago`;
  const diffInDays = Math.floor(diffInHours / 24);
  if (diffInDays < 7) return `${diffInDays}d ago`;
  const diffInWeeks = Math.floor(diffInDays / 7);
  if (diffInWeeks < 4) return `${diffInWeeks}w ago`;
  const diffInMonths = Math.floor(diffInDays / 30);
  if (diffInMonths < 12) return `${diffInMonths}mo ago`;
  const diffInYears = Math.floor(diffInDays / 365);
  return `${diffInYears}y ago`;
}

/**
 * Compact relative time for dense UIs (sidebar stamps): `now`, `5m`, `1h`, `4d`.
 */
export function timeAgoCompact(date: string | Date): string {
  const now = new Date();
  const diffInSeconds = Math.floor(
    (now.getTime() - new Date(date).getTime()) / 1000,
  );

  if (Number.isNaN(diffInSeconds) || diffInSeconds < 60) return "now";
  const diffInMinutes = Math.floor(diffInSeconds / 60);
  if (diffInMinutes < 60) return `${diffInMinutes}m`;
  const diffInHours = Math.floor(diffInMinutes / 60);
  if (diffInHours < 24) return `${diffInHours}h`;
  const diffInDays = Math.floor(diffInHours / 24);
  if (diffInDays < 7) return `${diffInDays}d`;
  const diffInWeeks = Math.floor(diffInDays / 7);
  if (diffInWeeks < 5) return `${diffInWeeks}w`;
  const diffInMonths = Math.floor(diffInDays / 30);
  if (diffInMonths < 12) return `${diffInMonths}mo`;
  const diffInYears = Math.floor(diffInDays / 365);
  return `${diffInYears}y`;
}