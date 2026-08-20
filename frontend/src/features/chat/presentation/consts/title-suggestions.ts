import { Slug } from "@/core/helpers/slug.helper"

export interface ChannelTitleSuggestion {
  id: string
  name: string
  description: string
  keywords: string[]
}

export const CHANNEL_TITLE_SUGGESTIONS: ChannelTitleSuggestion[] = [
  {
    id: "help",
    name: "help",
    description: "For questions, assistance, and resources on a topic",
    keywords: ["support", "questions", "docs", "assist", "faq"],
  },
  {
    id: "proj",
    name: "proj",
    description: "For collaboration on and discussion about a project",
    keywords: ["project", "produto", "feature", "squad", "build"],
  },
  {
    id: "team",
    name: "team",
    description: "For updates and work from a department or team",
    keywords: ["people", "department", "group", "org", "crew"],
  },
  {
    id: "ops",
    name: "ops",
    description: "For operations, incidents, and handoff routines",
    keywords: ["incident", "infra", "handoff", "support", "runtime"],
  },
  {
    id: "design",
    name: "design",
    description: "For reviews, explorations, and UI decisions",
    keywords: ["ux", "ui", "brand", "creative", "review"],
  },
  {
    id: "eng",
    name: "eng",
    description: "For engineering updates, architecture, and implementation",
    keywords: ["engineering", "backend", "frontend", "system", "dev"],
  },
  {
    id: "bugs",
    name: "bugs",
    description: "For bug triage, fixes, regressions, and hot spots",
    keywords: ["issue", "fix", "regression", "qa", "debug"],
  },
  {
    id: "research",
    name: "research",
    description: "For investigations, experiments, and references",
    keywords: ["explore", "study", "analysis", "discovery", "learn"],
  },
  {
    id: "launch",
    name: "launch",
    description: "For release planning, rollout coordination, and go-live updates",
    keywords: ["release", "rollout", "deploy", "ship", "launch"],
  },
  {
    id: "sales",
    name: "sales",
    description: "For pipeline updates, deals, and customer follow-up",
    keywords: ["customer", "revenue", "deal", "crm", "account"],
  },
  {
    id: "marketing",
    name: "marketing",
    description: "For campaigns, content, growth experiments, and launches",
    keywords: ["campaign", "content", "growth", "social", "brand"],
  },
  {
    id: "finance",
    name: "finance",
    description: "For budgets, invoices, and spending visibility",
    keywords: ["budget", "invoice", "cost", "expense", "money"],
  },
]

export function getChannelTitleSuggestions(query: string, limit = 3) {
  const normalizedQuery = Slug.generate(query)

  if (!normalizedQuery) {
    return CHANNEL_TITLE_SUGGESTIONS.slice(0, limit)
  }

  return CHANNEL_TITLE_SUGGESTIONS
    .map((suggestion) => {
      let score = 0

      if (suggestion.name.startsWith(normalizedQuery)) score += 6
      if (suggestion.name.includes(normalizedQuery)) score += 4
      if (normalizedQuery.includes(suggestion.name)) score += 3

      for (const keyword of suggestion.keywords) {
        const normalizedKeyword = Slug.generate(keyword)
        if (normalizedKeyword.startsWith(normalizedQuery)) score += 3
        if (normalizedKeyword.includes(normalizedQuery)) score += 2
        if (normalizedQuery.includes(normalizedKeyword)) score += 1
      }

      return { suggestion, score }
    })
    .filter((entry) => entry.score > 0)
    .sort((left, right) => {
      if (right.score !== left.score) return right.score - left.score
      return left.suggestion.name.localeCompare(right.suggestion.name)
    })
    .slice(0, limit)
    .map((entry) => entry.suggestion)
}
