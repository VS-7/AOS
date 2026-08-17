/**
 * Resolves a file to the Monaco language id its syntax highlighting and
 * language services should run under.
 *
 * Extension mapping is the common case; a handful of well-known files are
 * recognised by name instead, since "Dockerfile" and ".gitignore" carry no
 * extension a lookup table could key on.
 */

const EXTENSION_LANGUAGE: Record<string, string> = {
  // Web
  ts: "typescript",
  tsx: "typescript",
  mts: "typescript",
  cts: "typescript",
  js: "javascript",
  jsx: "javascript",
  mjs: "javascript",
  cjs: "javascript",
  html: "html",
  htm: "html",
  css: "css",
  scss: "scss",
  less: "less",
  vue: "html",
  svelte: "html",

  // Data
  json: "json",
  jsonc: "json",
  json5: "json",
  yaml: "yaml",
  yml: "yaml",
  toml: "ini",
  xml: "xml",
  csv: "plaintext",
  sql: "sql",
  graphql: "graphql",
  gql: "graphql",
  proto: "proto",

  // Docs
  md: "markdown",
  mdx: "markdown",
  txt: "plaintext",

  // Systems / backend languages
  go: "go",
  rs: "rust",
  py: "python",
  rb: "ruby",
  php: "php",
  java: "java",
  kt: "kotlin",
  kts: "kotlin",
  swift: "swift",
  c: "c",
  h: "c",
  cpp: "cpp",
  cc: "cpp",
  cxx: "cpp",
  hpp: "cpp",
  cs: "csharp",
  scala: "scala",
  clj: "clojure",
  ex: "elixir",
  exs: "elixir",
  erl: "erlang",
  hs: "haskell",
  lua: "lua",
  r: "r",
  dart: "dart",
  zig: "zig",

  // Shell / config
  sh: "shell",
  bash: "shell",
  zsh: "shell",
  fish: "shell",
  ps1: "powershell",
  bat: "bat",
  cmd: "bat",
  dockerfile: "dockerfile",
  ini: "ini",
  conf: "ini",
  env: "shell",

  // Misc
  diff: "diff",
  patch: "diff",
  vim: "plaintext",
  tex: "latex",
  proto3: "proto",
};

/** Case-insensitive filename matches, for files without a useful extension. */
const FILENAME_LANGUAGE: Record<string, string> = {
  dockerfile: "dockerfile",
  makefile: "makefile",
  gnumakefile: "makefile",
  ".gitignore": "plaintext",
  ".gitattributes": "plaintext",
  ".editorconfig": "ini",
  ".npmrc": "ini",
  ".env": "shell",
};

/** The languages Read's binary detection would refuse before this even runs. */
export function resolveLanguage(path: string): string {
  const name = path.split("/").pop() ?? path;
  const lower = name.toLowerCase();

  if (lower in FILENAME_LANGUAGE) return FILENAME_LANGUAGE[lower] ?? "plaintext";
  if (lower.startsWith("dockerfile.")) return "dockerfile";

  const dot = name.lastIndexOf(".");
  if (dot <= 0) return "plaintext";
  const ext = lower.slice(dot + 1);
  return EXTENSION_LANGUAGE[ext] ?? "plaintext";
}
