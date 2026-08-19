import * as React from "react";

import { aos } from "@/app/aos";
import { SettingsSectionShell } from "../../../section-shell";
import { DevelopersCliSection } from "./components/cli-section";
import { DevelopersRestApiSection } from "./components/rest-api-section";
import { DevelopersSkillMcpSection } from "./components/skill-mcp-section";

/**
 * UserDevelopersSection — consolidated CLI, REST API, and MCP settings.
 */
export function UserDevelopersSection() {
  const authUser = aos.stores.auth.useState((state) => state.user);
  const [apiToken, setApiToken] = React.useState<string | null>(null);

  const maskedToken = authUser?.tokenMasked ?? null;
  const hasToken = authUser?.hasToken ?? false;

  const handleTokenRevealed = React.useCallback(
    (full: string, _masked?: string) => {
      setApiToken(full);
      void aos.stores.auth.actions.refreshUser();
    },
    [],
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
