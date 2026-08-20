import type { FileViewer } from "@/features/file/interfaces/file.interfaces";

/**
 * File viewer resolution for the Files panel.
 *
 * Strategy (deny-binary, default-text):
 * 1. Specialized in-app viewers win (markdown, json, image, pdf, …).
 * 2. Known binary / opaque formats stay external (`other` or media kinds).
 * 3. Everything else opens as Monaco `text` — including unknown source
 *    extensions (`.py`, `.go`, …) and dotted text names (`.env.sample`).
 *
 * This avoids an endless text allowlist while keeping custom editors intact.
 */

const IMAGE_EXTENSIONS = new Set([
  "png",
  "jpg",
  "jpeg",
  "gif",
  "svg",
  "webp",
  "avif",
  "bmp",
  "ico",
  "icns",
  "heic",
  "heif",
  "tif",
  "tiff",
]);

const VIDEO_EXTENSIONS = new Set([
  "mp4",
  "mov",
  "webm",
  "avi",
  "mkv",
  "ogg",
  "m4v",
  "wmv",
  "flv",
]);

const AUDIO_EXTENSIONS = new Set([
  "mp3",
  "wav",
  "m4a",
  "flac",
  "aac",
  "wma",
  "aiff",
]);

const ARCHIVE_EXTENSIONS = new Set([
  "zip",
  "rar",
  "tar",
  "gz",
  "tgz",
  "bz2",
  "xz",
  "7z",
  "dmg",
  "iso",
  "jar",
  "war",
]);

/** Binary / opaque formats that must never open in Monaco. */
const BINARY_EXTENSIONS = new Set([
  ...IMAGE_EXTENSIONS,
  ...VIDEO_EXTENSIONS,
  ...AUDIO_EXTENSIONS,
  ...ARCHIVE_EXTENSIONS,
  "pdf",
  "docx",
  "doc",
  "xlsx",
  "xls",
  "xlsm",
  "pptx",
  "ppt",
  "csv",
  "tsv",
  "exe",
  "dll",
  "so",
  "dylib",
  "bin",
  "dat",
  "wasm",
  "class",
  "o",
  "a",
  "obj",
  "pyc",
  "pyo",
  "pickle",
  "pkl",
  "sqlite",
  "sqlite3",
  "db",
  "parquet",
  "woff",
  "woff2",
  "ttf",
  "otf",
  "eot",
  "node",
  "lockb",
]);

/** Monaco language ids keyed by lowercase file extension. */
export const MONACO_LANGUAGE_BY_EXTENSION: Record<string, string> = {
  bat: "bat",
  c: "c",
  cc: "cpp",
  cfg: "ini",
  cj: "clojure",
  clj: "clojure",
  cljs: "clojure",
  cmd: "bat",
  coffee: "coffee",
  conf: "ini",
  cpp: "cpp",
  cs: "csharp",
  css: "css",
  cxx: "cpp",
  dart: "dart",
  dockerfile: "dockerfile",
  env: "shell",
  ex: "plaintext",
  exs: "plaintext",
  fs: "fsharp",
  fsi: "fsharp",
  fsx: "fsharp",
  go: "go",
  graphql: "graphql",
  groovy: "groovy",
  h: "c",
  handlebars: "handlebars",
  hbs: "handlebars",
  hpp: "cpp",
  htm: "html",
  html: "html",
  ini: "ini",
  java: "java",
  jl: "julia",
  js: "javascript",
  json: "json",
  jsonc: "json",
  jsx: "javascript",
  kt: "kotlin",
  kts: "kotlin",
  less: "less",
  lock: "json",
  log: "plaintext",
  lua: "lua",
  m: "objective-c",
  md: "markdown",
  mdx: "markdown",
  mjs: "javascript",
  mm: "objective-c",
  php: "php",
  pl: "plaintext",
  pm: "plaintext",
  properties: "ini",
  ps1: "powershell",
  psm1: "powershell",
  py: "python",
  pyi: "python",
  pyw: "python",
  r: "r",
  rb: "ruby",
  rs: "rust",
  sample: "shell",
  sass: "scss",
  scala: "scala",
  scss: "scss",
  sh: "shell",
  sql: "sql",
  svg: "xml",
  swift: "swift",
  toml: "ini",
  ts: "typescript",
  tsx: "typescript",
  txt: "plaintext",
  vue: "html",
  xml: "xml",
  yaml: "yaml",
  yml: "yaml",
  zig: "plaintext",
  zsh: "shell",
};

export function getFileExtension(fileName: string): string {
  if (!fileName.includes(".")) return "";
  return fileName.split(".").at(-1)?.toLowerCase() ?? "";
}

function normalizeBaseName(fileName: string): string {
  return fileName.split("/").at(-1)?.toLowerCase() ?? fileName.toLowerCase();
}

/**
 * Resolve which Files panel viewer should handle a path/name.
 */
export function resolveFileViewer(fileName: string): FileViewer {
  const baseName = normalizeBaseName(fileName);
  const extension = getFileExtension(baseName);

  if (["md", "mdx"].includes(extension)) return "markdown";
  if (extension === "json" || extension === "jsonc") return "json";
  if (extension === "excalidraw" || baseName.endsWith(".excalidraw")) {
    return "excalidraw";
  }

  if (IMAGE_EXTENSIONS.has(extension)) return "image";
  if (extension === "pdf") return "pdf";
  if (extension === "docx" || extension === "doc") return "docx";
  if (extension === "xlsx" || extension === "xls" || extension === "xlsm") {
    return "xlsx";
  }
  if (extension === "csv" || extension === "tsv") return "csv";
  if (VIDEO_EXTENSIONS.has(extension)) return "video";
  if (AUDIO_EXTENSIONS.has(extension)) return "audio";
  if (ARCHIVE_EXTENSIONS.has(extension)) return "archive";

  // Known opaque binaries stay external; everything else opens in Monaco.
  if (BINARY_EXTENSIONS.has(extension)) return "other";

  return "text";
}

/**
 * Determines whether a viewer renders text content in Monaco.
 *
 * Returns `true` for `text`, `markdown`, `json`, and `excalidraw` viewers.
 *
 * @param viewer - Resolved viewer kind for a file (see {@link resolveFileViewer}).
 * @returns `true` when the viewer is backed by Monaco text rendering.
 */
export function isTextContentViewer(viewer: FileViewer): boolean {
  return (
    viewer === "text" ||
    viewer === "markdown" ||
    viewer === "json" ||
    viewer === "excalidraw"
  );
}

/**
 * Determines whether a viewer renders editable text content in Monaco.
 *
 * Returns `true` for `text`, `markdown`, `json`, and `excalidraw` viewers —
 * the same set as {@link isTextContentViewer} since all text viewers are editable.
 *
 * @param viewer - Resolved viewer kind for a file (see {@link resolveFileViewer}).
 * @returns `true` when the viewer is backed by Monaco text editing.
 */
export function isEditableFileViewer(viewer: FileViewer): boolean {
  return isTextContentViewer(viewer);
}

export function resolveMonacoLanguageFromFileName(
  fileName: string,
  extension = getFileExtension(fileName),
): string {
  const baseName = normalizeBaseName(fileName);

  if (baseName === ".env" || baseName.startsWith(".env.")) return "shell";
  if (baseName === ".gitignore" || baseName === ".editorconfig") {
    return "plaintext";
  }
  if (baseName.endsWith(".theme.json")) return "json";
  if (baseName.startsWith("dockerfile")) return "dockerfile";
  if (baseName.startsWith("makefile")) return "plaintext";

  return MONACO_LANGUAGE_BY_EXTENSION[extension] ?? "plaintext";
}
