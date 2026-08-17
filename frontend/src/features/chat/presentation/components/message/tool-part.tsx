import { Tool, ToolHeader, ToolContent, ToolInput, ToolOutput, isErrorOutput, type ToolState } from "@/components/ui/tool";
import type { Part } from "@/features/chat/interfaces/chat.interfaces";

/** Renders one tool invocation: its call part, and its result part once one arrives. */
export function ToolBlock({ call, result }: { call: Part; result?: Part }) {
  const toolName = call.toolName ?? "Tool";
  const reasoning =
    call.input && typeof call.input === "object"
      ? (call.input as Record<string, unknown>)["_reasoning"]
      : undefined;

  const state: ToolState = !result ? "running" : isErrorOutput(result.output) ? "error" : "done";

  return (
    <Tool className="mb-1.5">
      <ToolHeader
        toolName={toolName}
        state={state}
        reasoning={typeof reasoning === "string" ? reasoning : undefined}
      />
      <ToolContent>
        <ToolInput input={call.input} />
        {result && <ToolOutput output={result.output} />}
      </ToolContent>
    </Tool>
  );
}
