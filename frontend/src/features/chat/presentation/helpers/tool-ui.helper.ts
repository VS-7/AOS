import type { ToolPart } from "@/components/ui/tool";

export interface ToolUIConfig {
  title: string;
  labels: Record<ToolPart["state"], string>;
}

export class ToolUIHelper {
  private static registry: Record<string, ToolUIConfig> = {
    Read: {
      title: "File content",
      labels: {
        "input-streaming": "Preparing",
        "input-available": "Reading",
        "output-available": "Read complete",
        "output-error": "Read failed",
        "approval-requested": "Confirm",
        "approval-responded": "Approved",
        "output-denied": "Denied",
      },
    },
    Write: {
      title: "File output",
      labels: {
        "input-streaming": "Preparing",
        "input-available": "Writing",
        "output-available": "Write complete",
        "output-error": "Write failed",
        "approval-requested": "Confirm",
        "approval-responded": "Approved",
        "output-denied": "Denied",
      },
    },
    Edit: {
      title: "File modification",
      labels: {
        "input-streaming": "Preparing",
        "input-available": "Editing",
        "output-available": "Edit complete",
        "output-error": "Edit failed",
        "approval-requested": "Confirm",
        "approval-responded": "Approved",
        "output-denied": "Denied",
      },
    },
    Glob: {
      title: "File search",
      labels: {
        "input-streaming": "Searching",
        "input-available": "Searching",
        "output-available": "Search complete",
        "output-error": "Search failed",
        "approval-requested": "Confirm",
        "approval-responded": "Approved",
        "output-denied": "Denied",
      },
    },
    Grep: {
      title: "Content search",
      labels: {
        "input-streaming": "Searching",
        "input-available": "Searching",
        "output-available": "Search complete",
        "output-error": "Search failed",
        "approval-requested": "Confirm",
        "approval-responded": "Approved",
        "output-denied": "Denied",
      },
    },
    Bash: {
      title: "Shell execution",
      labels: {
        "input-streaming": "Preparing",
        "input-available": "Running",
        "output-available": "Execution finished",
        "output-error": "Execution failed",
        "approval-requested": "Confirm",
        "approval-responded": "Approved",
        "output-denied": "Denied",
      },
    },
    Agent: {
      title: "Subagent mission",
      labels: {
        "input-streaming": "Preparing",
        "input-available": "Working",
        "output-available": "Finished",
        "output-error": "Failed",
        "approval-requested": "Confirm",
        "approval-responded": "Approved",
        "output-denied": "Denied",
      },
    },
  };

  public static getConfig(toolName: string): ToolUIConfig {
    return this.registry[toolName] || {
      title: toolName,
      labels: {
        "input-streaming": "Pending...",
        "input-available": "Running...",
        "output-available": "Completed",
        "output-error": "Error",
        "approval-requested": "Waiting",
        "approval-responded": "Approved",
        "output-denied": "Denied",
      },
    };
  }
}
