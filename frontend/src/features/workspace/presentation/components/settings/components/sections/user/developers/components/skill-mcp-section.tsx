import * as React from "react";
import {
  Tick01Icon,
  Copy01Icon,
  Download01Icon,
  McpServerIcon,
  PuzzleIcon,
} from "@hugeicons/core-free-icons";
import { HugeiconsIcon } from "@hugeicons/react";
import { toast } from "sonner";

import { Button } from "@/components/ui/button";
import {
  FormSection,
  FormSectionContent,
  FormSectionDescription,
  FormSectionHeader,
  FormSectionItem,
  FormSectionTitle,
} from "@/components/ui/form-section";
import { Switch } from "@/components/ui/switch";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { cn } from "@/lib/utils";

import { AddSkillDropdown } from "./add-skill-dropdown";

const MCP_HTTP_URL = "http://localhost:5326/mcp";
const TOKEN_PLACEHOLDER = "fractal_<your-api-token>";
const TOKEN_PLACEHOLDER_MASKED = "fractal_...xxxx";

type McpTransport = "stdio" | "http";

interface DevelopersSkillMcpSectionProps {
  /** Full API token used when copying/downloading mcp.json. */
  apiToken: string | null;
  /** Masked token preview shown in the JSON viewer. */
  maskedToken: string | null;
}

/**
 * Builds the STDIO mcp.json config for `fractal --mcp`.
 *
 * @param token - API token (full or masked preview).
 * @returns Pretty-printed JSON string.
 */
function buildStdioConfig(token: string): string {
  return JSON.stringify(
    {
      mcpServers: {
        fractal: {
          command: "fractal",
          args: ["--mcp"],
          env: {
            FRACTAL_TOKEN: token,
          },
        },
      },
    },
    null,
    2,
  );
}

/**
 * Builds the HTTP mcp.json config for the local MCP endpoint.
 *
 * @param token - API token (full or masked preview).
 * @returns Pretty-printed JSON string.
 */
function buildHttpConfig(token: string): string {
  return JSON.stringify(
    {
      mcpServers: {
        fractal: {
          type: "http",
          url: MCP_HTTP_URL,
          headers: {
            Authorization: `Bearer ${token}`,
          },
        },
      },
    },
    null,
    2,
  );
}

/**
 * Skill install row + MCP server toggle with STDIO/HTTP mcp.json viewer.
 * Preview uses the masked token; copy/download embeds the full token when available.
 */
export function DevelopersSkillMcpSection({
  apiToken,
  maskedToken,
}: DevelopersSkillMcpSectionProps) {
  const [mcpEnabled, setMcpEnabled] = React.useState(true);
  const [transport, setTransport] = React.useState<McpTransport>("stdio");
  const [copied, setCopied] = React.useState(false);

  const previewToken =
    maskedToken ?? (apiToken ? `fractal_...${apiToken.slice(-4)}` : TOKEN_PLACEHOLDER_MASKED);
  const exportToken = apiToken ?? TOKEN_PLACEHOLDER;

  const exportJson =
    transport === "stdio"
      ? buildStdioConfig(exportToken)
      : buildHttpConfig(exportToken);

  const handleCopy = async () => {
    try {
      await navigator.clipboard.writeText(exportJson);
      setCopied(true);
      toast.success(
        apiToken
          ? "mcp.json copied with full token"
          : "mcp.json copied (replace the token placeholder)",
      );
      setTimeout(() => setCopied(false), 2000);
    } catch {
      toast.error("Failed to copy");
    }
  };

  const handleDownload = () => {
    const blob = new Blob([exportJson], { type: "application/json" });
    const url = URL.createObjectURL(blob);
    const anchor = document.createElement("a");
    anchor.href = url;
    anchor.download = "mcp.json";
    anchor.click();
    URL.revokeObjectURL(url);
    toast.success("mcp.json downloaded");
  };

  return (
    <FormSection>
      <FormSectionHeader>
        <FormSectionTitle>Skill and MCP</FormSectionTitle>
        <FormSectionDescription>
          Connect agents through Fractal skills and the MCP server.
        </FormSectionDescription>
      </FormSectionHeader>

      <FormSectionContent>
        <FormSectionItem>
          <div className="flex min-w-0 items-center gap-3">
            <div className="flex size-9 shrink-0 items-center justify-center rounded-lg border border-border bg-background">
              <HugeiconsIcon
                icon={PuzzleIcon}
                className="size-4 text-muted-foreground"
              />
            </div>
            <div className="min-w-0">
              <p className="text-sm font-medium text-foreground">
                Fractal Skills
              </p>
              <p className="text-sm text-muted-foreground">
                Allow agents to interact with Fractal through CLI and skills.
              </p>
            </div>
          </div>
          <AddSkillDropdown />
        </FormSectionItem>

        <FormSectionItem>
          <div className="flex min-w-0 items-center gap-3">
            <div className="flex size-9 shrink-0 items-center justify-center rounded-lg border border-border bg-background">
              <HugeiconsIcon
                icon={McpServerIcon}
                className="size-4 text-muted-foreground"
              />
            </div>
            <div className="min-w-0">
              <p className="text-sm font-medium text-foreground">
                Fractal MCP server
              </p>
              <p className="text-sm text-muted-foreground">
                Add this to agents that support{" "}
                <code className="rounded bg-muted px-1 py-0.5 font-mono text-xs">
                  mcp.json
                </code>
                .
              </p>
            </div>
          </div>
          <Switch
            checked={mcpEnabled}
            onCheckedChange={setMcpEnabled}
            aria-label="Toggle Fractal MCP server config"
          />
        </FormSectionItem>

        {mcpEnabled ? (
          <div className="p-4">
            <Tabs
              value={transport}
              onValueChange={(value) => setTransport(value as McpTransport)}
              className="gap-0 overflow-hidden rounded-lg border border-border bg-background"
            >
              <div className="flex items-center justify-between gap-2 border-b border-border bg-muted/60 px-3 py-2">
                <div className="flex items-center gap-3">
                  <span className="font-mono text-xs text-muted-foreground">
                    mcp.json
                  </span>
                  <TabsList variant="default" className="h-7">
                    <TabsTrigger value="stdio" className="px-2 text-xs">
                      STDIO
                    </TabsTrigger>
                    <TabsTrigger value="http" className="px-2 text-xs">
                      HTTP
                    </TabsTrigger>
                  </TabsList>
                </div>
                <div className="flex items-center gap-1">
                  <Button
                    type="button"
                    variant="ghost"
                    size="icon"
                    className="size-7"
                    onClick={handleDownload}
                    aria-label="Download mcp.json"
                  >
                    <HugeiconsIcon icon={Download01Icon} className="size-3.5" />
                  </Button>
                  <Button
                    type="button"
                    variant="ghost"
                    size="icon"
                    className="size-7"
                    onClick={() => void handleCopy()}
                    aria-label="Copy mcp.json with full token"
                  >
                    {copied ? (
                      <HugeiconsIcon icon={Tick01Icon} className="size-3.5" />
                    ) : (
                      <HugeiconsIcon icon={Copy01Icon} className="size-3.5" />
                    )}
                  </Button>
                </div>
              </div>

              <TabsContent value="stdio" className="mt-0">
                <McpJsonBody content={buildStdioConfig(previewToken)} />
              </TabsContent>
              <TabsContent value="http" className="mt-0">
                <McpJsonBody content={buildHttpConfig(previewToken)} />
              </TabsContent>
            </Tabs>
            {!apiToken ? (
              <p className="mt-2 text-xs text-muted-foreground">
                Preview shows a masked token. Generate or load an API token in
                REST API to copy mcp.json with the full value.
              </p>
            ) : null}
          </div>
        ) : null}
      </FormSectionContent>
    </FormSection>
  );
}

/**
 * Renders mcp.json with simple line numbers.
 */
function McpJsonBody({ content }: { content: string }) {
  const lines = content.split("\n");

  return (
    <pre
      className={cn(
        "max-h-72 overflow-auto bg-muted/30 p-3 font-mono text-xs leading-5 text-foreground",
      )}
    >
      {lines.map((line, index) => (
        <div key={`${index}-${line}`} className="flex gap-3">
          <span className="w-5 shrink-0 select-none text-right text-muted-foreground/60">
            {index + 1}
          </span>
          <span className="min-w-0 whitespace-pre-wrap break-all">{line}</span>
        </div>
      ))}
    </pre>
  );
}
