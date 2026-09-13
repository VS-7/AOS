import * as React from "react";

import { api } from "@/lib/aos-facade";
import type { ApiTokenInfo } from "@/lib/auth";
import { SettingsSectionShell } from "../../../section-shell";
import { DevelopersCliSection } from "./components/cli-section";
import { DevelopersRestApiSection } from "./components/rest-api-section";
import { DevelopersSkillMcpSection } from "./components/skill-mcp-section";

/**
 * UserDevelopersSection — consolidated CLI, REST API, and MCP settings.
 */
export function UserDevelopersSection() {
  // The account's API token as the daemon describes it — a prefix, never the
  // value. This read `hasToken`/`tokenMasked` off the account, which no
  // account carries, so the page always said "No token configured".
  const tokenQuery = api.token.get.useQuery();
  const current = (tokenQuery.data as { token?: ApiTokenInfo | null } | undefined)?.token ?? null;
  const [apiToken, setApiToken] = React.useState<string | null>(null);

  const maskedToken = current ? `${current.prefix}…` : null;
  const hasToken = current !== null || apiToken !== null;

  const handleTokenRevealed = React.useCallback(
    (full: string) => {
      setApiToken(full);
      void tokenQuery.refetch();
    },
    [tokenQuery],
  );

  return (
    <SettingsSectionShell>
      <DevelopersCliSection />
      <DevelopersRestApiSection
        apiToken={apiToken}
        maskedToken={maskedToken}
        hasToken={hasToken}
        onTokenRevealed={handleTokenRevealed}
      />
      <DevelopersSkillMcpSection apiToken={apiToken} maskedToken={maskedToken} />
    </SettingsSectionShell>
  );
}
