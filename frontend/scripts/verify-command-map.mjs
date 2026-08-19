#!/usr/bin/env node
// Mechanical contract verification for lib/command-map.ts against:
//   1. the published Go command registry (lib/schema.ts, generated)
//   2. the Go command registrations themselves (internal/domain/*/commands.go)
//   3. the Go *Input structs' json tags (internal/domain/**/*.go)
//   4. every client call path actually used in src/features/**
//
// Run: node frontend/scripts/verify-command-map.mjs
// (no deps beyond Node's fs/path — deliberately, so it never rots against a
// devDependency bump)
//
// Exit code is non-zero iff a HARD finding exists (see severities below).

import fs from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const FRONTEND_ROOT = path.resolve(__dirname, "..");
const REPO_ROOT = path.resolve(FRONTEND_ROOT, "..");
const DOMAIN_ROOT = path.join(REPO_ROOT, "internal", "domain");
const SRC_ROOT = path.join(FRONTEND_ROOT, "src");

const COMMAND_MAP_PATH = path.join(SRC_ROOT, "lib", "command-map.ts");
const SCHEMA_PATH = path.join(SRC_ROOT, "lib", "schema.ts");

/** ---------------------------------------------------------------------
 * Small comment/string-aware tokenizer utilities, used to parse
 * command-map.ts without pulling in the TS compiler or executing the file
 * (which imports @wailsio/runtime and hits real transport code at module
 * scope in sibling files).
 * ------------------------------------------------------------------- */

function stripComments(src) {
  let out = "";
  let i = 0;
  const n = src.length;
  while (i < n) {
    const c = src[i];
    const c2 = src[i + 1];
    if (c === "/" && c2 === "/") {
      while (i < n && src[i] !== "\n") i++;
      continue;
    }
    if (c === "/" && c2 === "*") {
      i += 2;
      while (i < n && !(src[i] === "*" && src[i + 1] === "/")) i++;
      i += 2;
      continue;
    }
    if (c === '"' || c === "'" || c === "`") {
      const quote = c;
      out += c;
      i++;
      while (i < n && src[i] !== quote) {
        if (src[i] === "\\") {
          out += src[i] + (src[i + 1] ?? "");
          i += 2;
          continue;
        }
        out += src[i];
        i++;
      }
      out += src[i] ?? "";
      i++;
      continue;
    }
    out += c;
    i++;
  }
  return out;
}

/** Splits `body` (the inside of some {...}) into top-level `key: valueText`
 * entries, respecting nested {}, (), [] and string literals. */
function splitTopLevelEntries(body) {
  const entries = [];
  let depth = 0;
  let cur = "";
  let i = 0;
  const n = body.length;
  const push = () => {
    const trimmed = cur.trim();
    if (trimmed) entries.push(trimmed);
    cur = "";
  };
  while (i < n) {
    const c = body[i];
    if (c === '"' || c === "'" || c === "`") {
      const quote = c;
      cur += c;
      i++;
      while (i < n && body[i] !== quote) {
        if (body[i] === "\\") {
          cur += body[i] + (body[i + 1] ?? "");
          i += 2;
          continue;
        }
        cur += body[i];
        i++;
      }
      cur += body[i] ?? "";
      i++;
      continue;
    }
    if (c === "{" || c === "(" || c === "[") depth++;
    if (c === "}" || c === ")" || c === "]") depth--;
    if (c === "," && depth === 0) {
      push();
      i++;
      continue;
    }
    cur += c;
    i++;
  }
  push();
  return entries.map((e) => {
    // Object keys in this file are either quoted string literals
    // (`"activity.list": ...` in COMMAND_MAP itself) or bare identifiers
    // (`key: "..."`, `wrapOut: "..."` inside a CommandDescriptor literal).
    const quoted = e.match(/^"((?:[^"\\]|\\.)*)"\s*:\s*([\s\S]*)$/);
    if (quoted) return { key: quoted[1], valueText: quoted[2].trim() };
    const bare = e.match(/^([A-Za-z_$][A-Za-z0-9_$]*)\s*:\s*([\s\S]*)$/);
    if (bare) return { key: bare[1], valueText: bare[2].trim() };
    return null;
  }).filter(Boolean);
}

/** Finds the `{ ... }` matching the first `{` in `s`, returns inner text. */
function extractBraced(s) {
  const start = s.indexOf("{");
  if (start === -1) return null;
  let depth = 0;
  for (let i = start; i < s.length; i++) {
    const c = s[i];
    if (c === '"' || c === "'" || c === "`") {
      const quote = c;
      i++;
      while (i < s.length && s[i] !== quote) {
        if (s[i] === "\\") i++;
        i++;
      }
      continue;
    }
    if (c === "{") depth++;
    if (c === "}") {
      depth--;
      if (depth === 0) return s.slice(start + 1, i);
    }
  }
  return null;
}

function unquote(s) {
  const t = s.trim();
  const m = t.match(/^"((?:[^"\\]|\\.)*)"$/) || t.match(/^'((?:[^'\\]|\\.)*)'$/);
  return m ? m[1] : t;
}

/** ---------------------------------------------------------------------
 * 1. Parse lib/schema.ts -> set of published command keys.
 * ------------------------------------------------------------------- */
function parseSchemaKeys(schemaSrc) {
  const keys = new Set();
  const re = /^\s*"([a-z][a-zA-Z0-9_-]*)":\s*\{\s*input:/gm;
  let m;
  while ((m = re.exec(schemaSrc))) keys.add(m[1]);
  return keys;
}

/** ---------------------------------------------------------------------
 * 2 & 3. Parse Go domain packages -> per-command Input/Output type, and per
 * struct -> json tags.
 * ------------------------------------------------------------------- */
function parseGoDomains() {
  const domains = fs.readdirSync(DOMAIN_ROOT).filter((d) =>
    fs.statSync(path.join(DOMAIN_ROOT, d)).isDirectory()
  );

  /** commandKey -> { group, name, inputType, outputType, domain, file, line } */
  const registrations = new Map();
  /** domain -> { structName -> [{field, jsonName, goType}] } */
  const structsByDomain = new Map();

  for (const domain of domains) {
    const dir = path.join(DOMAIN_ROOT, domain);
    const files = fs.readdirSync(dir).filter((f) => f.endsWith(".go") && !f.endsWith("_test.go"));

    // --- registrations, from commands.go (if present) ---
    const cmdFile = files.find((f) => f === "commands.go");
    if (cmdFile) {
      const src = fs.readFileSync(path.join(dir, cmdFile), "utf8");
      const blockRe = /command\.MustRegister\(reg,\s*command\.Command\[([A-Za-z0-9_*]+),\s*([A-Za-z0-9_*]+)\]\{([\s\S]*?)\n\t\}\)/g;
      let bm;
      while ((bm = blockRe.exec(src))) {
        const [, inputType, outputType, body] = bm;
        const gMatch = body.match(/Group:\s*"([^"]+)"/);
        const nMatch = body.match(/Name:\s*"([^"]+)"/);
        if (!gMatch || !nMatch) continue;
        const key = `${gMatch[1]}_${nMatch[1]}`;
        const lineNo = src.slice(0, bm.index).split("\n").length;
        registrations.set(key, { group: gMatch[1], name: nMatch[1], inputType, outputType, domain, file: cmdFile, line: lineNo });
      }
    }

    // --- struct field/json-tag index, from every non-test .go file ---
    const structs = {};
    for (const f of files) {
      const src = fs.readFileSync(path.join(dir, f), "utf8");
      const structRe = /type\s+([A-Za-z0-9_]+)\s+struct\s*\{/g;
      let sm;
      while ((sm = structRe.exec(src))) {
        const name = sm[1];
        // brace-match the struct body
        let depth = 1;
        let i = sm.index + sm[0].length;
        const start = i;
        while (i < src.length && depth > 0) {
          if (src[i] === "{") depth++;
          if (src[i] === "}") depth--;
          i++;
        }
        const body = src.slice(start, i - 1);
        const fields = [];
        for (const rawLine of body.split("\n")) {
          const line = rawLine.trim();
          if (!line || line.startsWith("//")) continue;
          // FieldName Type `json:"name,opts" ...`
          const fm = line.match(/^([A-Za-z0-9_]+)\s+([^\s`]+(?:\s*\[\][^\s`]+)?)\s*`([^`]*)`/);
          if (!fm) continue;
          const [, fieldName, goType, tagStr] = fm;
          const jm = tagStr.match(/json:"([^"]*)"/);
          if (!jm) continue;
          const jsonName = jm[1].split(",")[0];
          if (jsonName === "-" || jsonName === "") continue;
          fields.push({ field: fieldName, jsonName, goType });
        }
        structs[name] = fields;
      }
    }
    structsByDomain.set(domain, structs);
  }

  return { registrations, structsByDomain };
}

/** ---------------------------------------------------------------------
 * 4. Parse lib/command-map.ts -> COMMAND_MAP entries.
 * ------------------------------------------------------------------- */
function parseCommandMap(rawSrc) {
  const src = stripComments(rawSrc);
  const anchor = "export const COMMAND_MAP: Record<string, MapEntry> = {";
  const start = src.indexOf(anchor);
  if (start === -1) throw new Error("could not find COMMAND_MAP in command-map.ts");
  const body = extractBraced(src.slice(start + anchor.length - 1));
  const rawEntries = splitTopLevelEntries(body);

  const entries = {};
  for (const { key, valueText } of rawEntries) {
    if (valueText === "null") {
      entries[key] = { kind: "null" };
      continue;
    }
    if (valueText.startsWith('"') || valueText.startsWith("'")) {
      entries[key] = { kind: "command", goKey: unquote(valueText), renameIn: {}, coerceIn: [], wrapOut: undefined };
      continue;
    }
    if (valueText.startsWith("{")) {
      const inner = extractBraced(valueText);
      const fields = splitTopLevelEntries(inner);
      const rec = { kind: "command", goKey: undefined, renameIn: {}, coerceIn: [], wrapOut: undefined };
      for (const f of fields) {
        if (f.key === "key") rec.goKey = unquote(f.valueText);
        else if (f.key === "wrapOut") rec.wrapOut = unquote(f.valueText);
        else if (f.key === "renameIn") {
          const riInner = extractBraced(f.valueText);
          for (const ri of splitTopLevelEntries(riInner)) {
            rec.renameIn[ri.key] = unquote(ri.valueText);
          }
        } else if (f.key === "coerceIn") {
          const ciInner = extractBraced(f.valueText);
          for (const ci of splitTopLevelEntries(ciInner)) rec.coerceIn.push({ field: ci.key, body: ci.valueText });
        }
      }
      entries[key] = rec;
      continue;
    }
    // Arrow function / other expression -> HTTP handler or unrecognized.
    entries[key] = { kind: "http" };
  }
  return entries;
}

/** ---------------------------------------------------------------------
 * 5. Scan src/features/** (and src/app) for `api.<feature>.<action>` call
 * paths actually used by ported code.
 * ------------------------------------------------------------------- */
function scanCallPaths() {
  const found = new Map(); // "feature.action" -> [{file, line}]
  // Three call shapes the ported code actually uses, all resolving through
  // the same `api` Proxy in the end:
  //   1. `api.<feature>.<action>.(query|mutate|useQuery|useMutation)(...)`
  //      — the direct facade call, most call sites.
  //   2. `aos.client.<feature>.<action>.(...)` — `app/aos.tsx` exposes the
  //      same `api` instance as `aos.client`; pristine pages were written
  //      against that name.
  //   3. `mutation: "<feature>.<action>"` — `AosApp.useForm`'s declarative
  //      path (`app/builders/app.tsx`: `(mutation as string).split(".")`
  //      then indexes into `client[controller][action].mutate`), used by
  //      settings/dialog forms instead of calling `.mutate` directly.
  const patterns = [
    /\b(?:api|aos\.client)\.([a-zA-Z0-9_]+)\.([a-zA-Z0-9_]+)\.(query|mutate|useQuery|useMutation)\b/g,
    /\bmutation:\s*"([a-zA-Z0-9_]+)\.([a-zA-Z0-9_]+)"/g,
  ];

  function walk(dir) {
    for (const entry of fs.readdirSync(dir, { withFileTypes: true })) {
      const p = path.join(dir, entry.name);
      if (entry.isDirectory()) {
        walk(p);
      } else if (/\.(ts|tsx)$/.test(entry.name) && !entry.name.endsWith(".test.ts") && !entry.name.endsWith(".test.tsx")) {
        const src = fs.readFileSync(p, "utf8");
        const rel = path.relative(FRONTEND_ROOT, p);
        for (const re of patterns) {
          let m;
          re.lastIndex = 0;
          while ((m = re.exec(src))) {
            const key = `${m[1]}.${m[2]}`;
            const lineNo = src.slice(0, m.index).split("\n").length;
            if (!found.has(key)) found.set(key, []);
            found.get(key).push(`${rel}:${lineNo}`);
          }
        }
      }
    }
  }
  walk(SRC_ROOT);
  return found;
}

/** ---------------------------------------------------------------------
 * Main
 * ------------------------------------------------------------------- */
function main() {
  const schemaSrc = fs.readFileSync(SCHEMA_PATH, "utf8");
  const schemaKeys = parseSchemaKeys(schemaSrc);

  const { registrations, structsByDomain } = parseGoDomains();

  const cmapSrc = fs.readFileSync(COMMAND_MAP_PATH, "utf8");
  const entries = parseCommandMap(cmapSrc);

  const findings = { hard: [], soft: [], info: [] };

  // --- Check A: every mapped `key` names a real published command ---
  for (const [path_, e] of Object.entries(entries)) {
    if (e.kind !== "command") continue;
    if (!e.goKey) {
      findings.hard.push(`${path_}: command entry has no resolvable "key"`);
      continue;
    }
    if (!schemaKeys.has(e.goKey)) {
      findings.hard.push(`${path_}: key "${e.goKey}" is not in the published registry (lib/schema.ts)`);
    }
  }

  // --- Check B: renameIn targets are real json tags on the Go *Input ---
  // --- Check C: wrapOut correctness against bare-pointer-entity rule ---
  for (const [path_, e] of Object.entries(entries)) {
    if (e.kind !== "command" || !e.goKey) continue;
    const reg = registrations.get(e.goKey);
    if (!reg) {
      findings.soft.push(`${path_} (${e.goKey}): no Go registration found (not in internal/domain/*/commands.go) — cannot check renameIn/wrapOut`);
      continue;
    }
    const structs = structsByDomain.get(reg.domain) ?? {};
    const inputFields = structs[reg.inputType] ?? null;
    if (!inputFields) {
      findings.soft.push(`${path_} (${e.goKey}): could not locate Go struct ${reg.inputType} in domain "${reg.domain}"`);
    } else {
      const jsonNames = new Set(inputFields.map((f) => f.jsonName));
      for (const [uiField, goField] of Object.entries(e.renameIn)) {
        if (!jsonNames.has(goField)) {
          findings.hard.push(
            `${path_} (${e.goKey}): renameIn target "${goField}" (from "${uiField}") is not a json tag on ${reg.domain}.${reg.inputType} — real tags: [${[...jsonNames].join(", ")}]`,
          );
        }
      }
      // coerceIn field names, after renameIn is applied, should also land on
      // a real tag — UNLESS the transform's own body returns an object
      // literal (the "reshape into several top-level Go fields" case the
      // type doc describes), in which case what matters is that object's
      // *own* keys, not the coerced field's original name. Heuristic: an
      // arrow body starting `(` or `{` right after `=>` returning an object
      // literal (`=> ({ ... })` or `=> { return { ... } }`) is read for its
      // own top-level keys via a light regex rather than a full parse.
      for (const { field: uiField, body: coerceBody } of e.coerceIn) {
        const returnsObjectLiteral = /=>\s*\(\{/.test(coerceBody) || /return\s*\{/.test(coerceBody);
        if (returnsObjectLiteral) {
          const objKeys = [...coerceBody.matchAll(/[{,]\s*([A-Za-z_$][A-Za-z0-9_$]*)\s*:/g)].map((m) => m[1]);
          const unknown = objKeys.filter((k) => !jsonNames.has(k) && k !== "value");
          if (unknown.length > 0 && objKeys.length > 0) {
            findings.info.push(
              `${path_} (${e.goKey}): coerceIn.${uiField} returns an object literal with keys [${objKeys.join(", ")}] — could not confirm [${unknown.join(", ")}] against ${reg.domain}.${reg.inputType}'s tags by static regex; read manually.`,
            );
          }
          continue;
        }
        const goField = e.renameIn[uiField] ?? uiField;
        if (!jsonNames.has(goField)) {
          findings.soft.push(
            `${path_} (${e.goKey}): coerceIn touches "${uiField}" (-> "${goField}") which is not a json tag on ${reg.domain}.${reg.inputType} — real tags: [${[...jsonNames].join(", ")}]. May be intentionally dropped (see coerceIn doc on returning undefined).`,
          );
        }
      }
    }

    const isPointerBareEntity = reg.outputType.startsWith("*") && !reg.outputType.slice(1).endsWith("Output");
    const isNamedOutput = !reg.outputType.startsWith("*") && reg.outputType.endsWith("Output");
    const isBareValueType = !reg.outputType.startsWith("*") && !reg.outputType.endsWith("Output");

    if (isPointerBareEntity && !e.wrapOut) {
      findings.hard.push(`${path_} (${e.goKey}): Go returns bare ${reg.outputType} but map entry has no wrapOut`);
    }
    if (isNamedOutput && e.wrapOut) {
      findings.hard.push(`${path_} (${e.goKey}): Go returns named ${reg.outputType} (already shaped) but map entry has wrapOut: "${e.wrapOut}" — double-wrapping`);
    }
    if (isPointerBareEntity && e.wrapOut) {
      findings.info.push(`${path_} (${e.goKey}): wrapOut: "${e.wrapOut}" for bare ${reg.outputType} — confirm call sites actually read .${e.wrapOut}`);
    }
    if (isBareValueType) {
      findings.soft.push(`${path_} (${e.goKey}): Go returns bare non-pointer, non-Output value type "${reg.outputType}" — brief's strict rule doesn't cover this shape; wrapOut=${e.wrapOut ? `"${e.wrapOut}"` : "(none)"} — verify call site manually.`);
    }
  }

  // --- Check D: every call path used in src/features exists in COMMAND_MAP ---
  const callPaths = scanCallPaths();
  for (const [callPath, sites] of callPaths.entries()) {
    if (!(callPath in entries)) {
      findings.hard.push(`${callPath}: used at ${sites.join(", ")} but absent from COMMAND_MAP entirely — call() will throw at runtime`);
    }
  }

  // --- Report ---
  const lines = [];
  lines.push(`# verify-command-map report`);
  lines.push(``);
  lines.push(`Schema keys (published registry): ${schemaKeys.size}`);
  lines.push(`Go registrations found: ${registrations.size}`);
  lines.push(`COMMAND_MAP entries: ${Object.keys(entries).length}`);
  lines.push(`Distinct call paths used in src/features: ${callPaths.size}`);
  lines.push(``);
  lines.push(`## HARD findings (${findings.hard.length}) — contract violations`);
  for (const f of findings.hard) lines.push(`- [HARD] ${f}`);
  lines.push(``);
  lines.push(`## SOFT findings (${findings.soft.length}) — needs a human judgment call`);
  for (const f of findings.soft) lines.push(`- [SOFT] ${f}`);
  lines.push(``);
  lines.push(`## INFO (${findings.info.length})`);
  for (const f of findings.info) lines.push(`- [INFO] ${f}`);
  lines.push(``);

  const report = lines.join("\n");
  console.log(report);

  process.exit(findings.hard.length > 0 ? 1 : 0);
}

main();
