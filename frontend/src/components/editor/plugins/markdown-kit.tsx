import {
  BaseFootnoteDefinitionPlugin,
  BaseFootnoteReferencePlugin,
} from '@platejs/footnote';
import { MarkdownPlugin, remarkMdx, remarkMention, type MdRules } from '@platejs/markdown';
import { KEYS } from 'platejs';
import type { Processor } from 'unified';
import remarkEmoji from 'remark-emoji';
import remarkGfm from 'remark-gfm';
import remarkMath from 'remark-math';

// The editor holds prompts — agent instructions, routine prompts, instruction
// bodies — and the runtime renders `{{ }}` in them as Liquid when a section
// asks for it. MDX stays for what markdown has no syntax for (`<u>`, media),
// but to MDX a `{…}` is a JavaScript expression, and Plate has no node for
// one: "Hi {{ name }}" loaded as "Hi ", and the next save wrote that to disk.
// The two pieces below make braces plain text in both directions.

type MdxExpression = { type: string; value: string };

// Plate rewrites an HTML comment into a JSX one (`{/* … */}`) before parsing.
// Those were dropped before this file kept expressions, and still are: as
// text they would come back as `{/\* … \*/}`, which is worse than nothing.
const isComment = (node: MdxExpression) => /^\s*\/\*[\s\S]*\*\/\s*$/.test(node.value);

// Loading: an expression becomes the text it was written as.
const braceRules = {
  mdxTextExpression: {
    deserialize: (node: MdxExpression, deco: Record<string, unknown>) =>
      isComment(node) ? [] : { ...deco, text: `{${node.value}}` },
  },
  mdxFlowExpression: {
    deserialize: (node: MdxExpression) =>
      isComment(node) ? [] : { type: KEYS.p, children: [{ text: `{${node.value}}` }] },
  },
} as unknown as MdRules;

type ToMarkdownExtension = {
  unsafe?: Array<{ character: string }>;
  extensions?: ToMarkdownExtension[];
};

function withoutBraceEscapes(extension: ToMarkdownExtension): ToMarkdownExtension {
  return {
    ...extension,
    unsafe: extension.unsafe?.filter((pattern) => pattern.character !== '{'),
    extensions: extension.extensions?.map(withoutBraceEscapes),
  };
}

// Saving: remark-mdx tells the serializer to escape every `{` so it cannot
// start an expression, which wrote `\{\{ name }}` — text a Liquid engine and
// a person both read with the backslashes in. Unescaped, the next load parses
// the brace as an expression again, and the rule above turns it back into
// the same text. Must come after remarkMdx, whose extension it edits.
function remarkPlainBraces(this: Processor) {
  const data = this.data() as { toMarkdownExtensions?: ToMarkdownExtension[] };
  data.toMarkdownExtensions = (data.toMarkdownExtensions ?? []).map(withoutBraceEscapes);
}

export const MarkdownKit = [
  BaseFootnoteReferencePlugin,
  BaseFootnoteDefinitionPlugin,
  MarkdownPlugin.configure({
    options: {
      plainMarks: [KEYS.suggestion, KEYS.comment],
      remarkPlugins: [
        remarkMath,
        remarkGfm,
        remarkEmoji as any,
        remarkMdx,
        remarkMention,
        remarkPlainBraces,
      ],
      rules: braceRules,
    },
  }),
];
