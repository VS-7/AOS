#!/usr/bin/env node
/**
 * Valida o grafo do vault de reconstrução.
 *
 * Verifica:
 *   1. Zero wikilinks quebrados — todo [[alvo]] resolve para uma nota ou alias
 *   2. Zero notas órfãs — toda nota (exceto o MOC) é alcançável por algum link
 *   3. Frontmatter obrigatório — tags, aliases, fase, status, origem
 *   4. Status válido — especificado | em-construcao | pronto
 *
 * O escopo de resolução inclui o vault de engenharia reversa, porque as notas
 * de reconstrução linkam para suas origens com `[[Nota]]`.
 *
 * Uso:  node docs/_scripts/validate-graph.mjs [--json]
 * Saída: 0 se válido, 1 se houver problema.
 */

import { readdirSync, readFileSync, statSync, existsSync } from "node:fs";
import { join, relative, basename, extname, dirname } from "node:path";
import { fileURLToPath } from "node:url";

const HERE = dirname(fileURLToPath(import.meta.url));
const ROOT = join(HERE, "..", "..");
const DOCS = join(ROOT, "docs");

/**
 * O vault de engenharia reversa não vive no repositório do AOS: é material
 * proprietário de terceiro e o PROMPT manda copiar apenas `docs/`. Procura-se
 * em três lugares; se nenhum existir, os links para fora de `docs/` viram
 * "não verificados" (aviso) em vez de "quebrados" (erro).
 */
const RE_VAULT_CANDIDATES = [
  process.env.AOS_RE_VAULT,
  join(ROOT, "Fractal Vault"),
  join(ROOT, "..", "Fractal Reverse Enginner", "Fractal Vault"),
].filter(Boolean);

const RE_VAULT = RE_VAULT_CANDIDATES.find((p) => existsSync(p)) ?? null;
const reVaultAvailable = RE_VAULT !== null;

const MOC = "AOS";
const VALID_STATUS = new Set(["especificado", "em-construcao", "pronto"]);
const REQUIRED_FIELDS = ["tags", "aliases", "fase", "status", "origem"];

const jsonOutput = process.argv.includes("--json");

function walk(dir, acc = []) {
  let entries;
  try {
    entries = readdirSync(dir);
  } catch {
    return acc;
  }
  for (const entry of entries) {
    // `superpowers/` holds SDD specs and plans — process artifacts of one
    // branch, not notes of the specification vault. They carry no vault
    // frontmatter and link to nothing, which is correct for what they are;
    // walking them only produces orphans and missing-field noise.
    if (entry.startsWith(".") || entry === "_scripts" || entry === "superpowers") continue;
    const full = join(dir, entry);
    if (statSync(full).isDirectory()) walk(full, acc);
    else if (extname(entry) === ".md") acc.push(full);
  }
  return acc;
}

function parseFrontmatter(text) {
  if (!text.startsWith("---")) return null;
  const end = text.indexOf("\n---", 3);
  if (end === -1) return null;
  const raw = text.slice(4, end);
  const fm = {};
  for (const line of raw.split("\n")) {
    const m = line.match(/^([a-zA-Z_]+):\s*(.*)$/);
    if (!m) continue;
    const [, key, value] = m;
    fm[key] = value.trim();
  }
  return fm;
}

function parseAliases(value) {
  if (!value) return [];
  const inner = value.replace(/^\[|\]$/g, "");
  return inner
    .split(",")
    .map((s) => s.trim().replace(/^["']|["']$/g, ""))
    .filter(Boolean);
}

/** Extrai wikilinks, ignorando blocos de código cercados e código inline. */
function extractLinks(text) {
  const withoutFences = text.replace(/```[\s\S]*?```/g, "");
  const withoutInline = withoutFences.replace(/`[^`\n]*`/g, "");
  const links = [];
  const re = /\[\[([^\]|#]+)(?:[|#][^\]]*)?\]\]/g;
  let m;
  while ((m = re.exec(withoutInline)) !== null) {
    links.push(m[1].trim());
  }
  return links;
}

// ── Índice de alvos resolvíveis ───────────────────────────────────────────────

const docsFiles = walk(DOCS);
const reFiles = reVaultAvailable ? walk(RE_VAULT) : [];
const allFiles = [...docsFiles, ...reFiles];

/** nome resolvível (lowercase) → caminho canônico */
const resolvable = new Map();
/** caminho → { name, aliases, fm, links } */
const notes = new Map();

for (const file of allFiles) {
  const text = readFileSync(file, "utf8");
  const name = basename(file, ".md");
  const fm = parseFrontmatter(text);
  const aliases = parseAliases(fm?.aliases);

  resolvable.set(name.toLowerCase(), file);
  for (const alias of aliases) {
    if (!resolvable.has(alias.toLowerCase())) resolvable.set(alias.toLowerCase(), file);
  }

  notes.set(file, { name, aliases, fm, links: extractLinks(text), text });
}

// ── Verificações ──────────────────────────────────────────────────────────────

const broken = [];
const unverified = [];
const missingFrontmatter = [];
const badStatus = [];
const linkedTo = new Set();

for (const file of docsFiles) {
  const note = notes.get(file);
  const rel = relative(ROOT, file);

  for (const field of REQUIRED_FIELDS) {
    if (!note.fm || !note.fm[field]) {
      missingFrontmatter.push({ file: rel, field });
    }
  }

  if (note.fm?.status && !VALID_STATUS.has(note.fm.status)) {
    badStatus.push({ file: rel, status: note.fm.status });
  }

  for (const link of note.links) {
    const target = resolvable.get(link.toLowerCase());
    if (!target) {
      (reVaultAvailable ? broken : unverified).push({ file: rel, link });
    } else {
      linkedTo.add(target);
    }
  }
}

// Links vindos do vault de RE também contam para alcançabilidade.
for (const file of reFiles) {
  for (const link of notes.get(file).links) {
    const target = resolvable.get(link.toLowerCase());
    if (target) linkedTo.add(target);
  }
}

const orphans = docsFiles
  .filter((f) => basename(f, ".md") !== MOC && !linkedTo.has(f))
  .map((f) => relative(ROOT, f));

// ── Relatório ─────────────────────────────────────────────────────────────────

const report = {
  notes: docsFiles.length,
  links: docsFiles.reduce((n, f) => n + notes.get(f).links.length, 0),
  broken,
  unverified,
  reVault: RE_VAULT,
  orphans,
  missingFrontmatter,
  badStatus,
};

if (jsonOutput) {
  console.log(JSON.stringify(report, null, 2));
} else {
  console.log(`Notas em docs/: ${report.notes}`);
  console.log(`Wikilinks: ${report.links}`);
  console.log(
    reVaultAvailable
      ? `Vault de RE: ${relative(ROOT, RE_VAULT)}`
      : "Vault de RE: ausente — links externos não verificados",
  );
  console.log("");

  const section = (title, items, render) => {
    if (items.length === 0) {
      console.log(`✓ ${title}: nenhum`);
      return;
    }
    console.log(`✗ ${title}: ${items.length}`);
    for (const item of items) console.log(`    ${render(item)}`);
  };

  section("Links quebrados", broken, (b) => `${b.file} → [[${b.link}]]`);
  if (!reVaultAvailable && unverified.length > 0) {
    const alvos = new Set(unverified.map((u) => u.link));
    console.log(
      `! Links não verificados: ${unverified.length} (${alvos.size} alvos distintos no vault de RE)`,
    );
  }
  section("Notas órfãs", orphans, (o) => o);
  section("Frontmatter faltando", missingFrontmatter, (m) => `${m.file} → ${m.field}`);
  section("Status inválido", badStatus, (s) => `${s.file} → ${s.status}`);
}

const failed =
  broken.length + orphans.length + missingFrontmatter.length + badStatus.length;

process.exit(failed > 0 ? 1 : 0);
