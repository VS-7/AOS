import React from "react";
import {
  Tool,
  ToolHeader,
  ToolContent,
  ToolInput,
  ToolOutput,
} from "@/components/ui/tool";
import type { ToolUIPart, DynamicToolUIPart } from "ai";
import { ToolUIHelper } from "../../helpers/tool-ui.helper";

/**
 * @component ToolBlock
 * @description Renders a block representing a tool call and its output.
 */
export function ToolBlock({ part }: { part: ToolUIPart | DynamicToolUIPart }) {
  const toolName = "toolName" in part ? part.toolName : part.type.split("-").slice(1).join("-");
  const config = ToolUIHelper.getConfig(toolName);
  const reasoning = (part.input as any)?._reasoning;

  return (
    <Tool className="mb-1.5">
      <ToolHeader
        // `part.state` is the AI SDK's richer lifecycle union (see
        // `components/ui/tool.tsx`'s `ToolPart` doc comment); `ToolHeader`'s
        // own `state` prop predates that and only accepts the coarser
        // three-way `ToolState`. Cast rather than widen a shared UI
        // primitive's prop for this one caller.
        state={part.state as any}
        toolName={toolName}
        title={config.title}
        reasoning={reasoning}
      />
      <ToolContent>
        <ToolInput input={part.input} />
        {"output" in part && (
          <ToolOutput output={part.output} {...({ errorText: (part as any).errorText } as any)} />
        )}
      </ToolContent>
    </Tool>
  );
}