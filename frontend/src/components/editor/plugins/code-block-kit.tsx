"use client";

import { CodeBlockRules } from "@platejs/code-block";
import {
  CodeBlockPlugin,
  CodeLinePlugin,
  CodeSyntaxPlugin,
} from "@platejs/code-block/react";
import { common, createLowlight } from "lowlight";

import {
  CodeBlockElement,
  CodeLineElement,
  CodeSyntaxLeaf,
} from "@/components/ui/code-block-node";

/**
 * Syntax highlighting for code blocks in the editor.
 *
 * `common` rather than `all`: lowlight's `all` is every grammar highlight.js
 * ships — 190-odd languages, and 1.5 MB of source — and this module is
 * reached from the editor kit, which is loaded at startup. Every one of those
 * grammars was downloaded, parsed and compiled before the window could be
 * used, so that a code block in some language nobody in this workspace writes
 * could be coloured without a further request.
 *
 * `common` is the ~40 languages highlight.js itself considers worth shipping
 * by default, and covers everything an agent actually emits here: the
 * shells, the JavaScript/TypeScript family, Python, Go, Rust, Java, C/C++/C#,
 * SQL, JSON, YAML, XML/HTML, CSS, Markdown and diff. A language outside that
 * set still renders — as an uncoloured code block, which is the same thing an
 * unrecognised language tag has always produced.
 */
const lowlight = createLowlight(common);

export const CodeBlockKit = [
  CodeBlockPlugin.configure({
    inputRules: [CodeBlockRules.markdown({ on: "match" })],
    node: { component: CodeBlockElement },
    options: { lowlight },
    shortcuts: { toggle: { keys: "mod+alt+8" } },
  }),
  CodeLinePlugin.withComponent(CodeLineElement),
  CodeSyntaxPlugin.withComponent(CodeSyntaxLeaf),
];
