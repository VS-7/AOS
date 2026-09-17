import {
  DropdownMenuItem,
  DropdownMenuLabel,
} from "@/components/ui/dropdown-menu";
import { Avatar, AvatarAgentFallback } from "@/components/ui/avatar";
import { aos } from "@/app/aos";
import { t } from "@/lib/i18n";

interface SetRoutineAgentDropdownProps {
  currentAgent: string;
  onAgentChange: (agent: string) => void;
}

/**
 * The agents a new routine can belong to.
 *
 * Only real agents: Go resolves no "orchestrator" or "all agents" target, and
 * offering them made Create fail with AOS_ROUTINE_NO_SUCH_AGENT. It is offered
 * only while creating — a routine lives in its agent's directory, and Go has
 * no move, so choosing another agent for a saved one could only be refused.
 */
export function SetRoutineAgentDropdown({
  currentAgent,
  onAgentChange,
}: SetRoutineAgentDropdownProps) {
  const agents = aos.stores.agent.useState((state) => state.items);

  return (
    <div className="flex flex-col gap-1">
      <DropdownMenuLabel className="text-xs font-medium text-muted-foreground">
        {t("Agents")}
      </DropdownMenuLabel>

      {agents.length === 0 ? (
        <div className="px-2 py-1 text-xs text-muted-foreground">
          {t("No agents available.")}
        </div>
      ) : (
        agents.map((agent) => (
          <DropdownMenuItem
            key={agent.id}
            onClick={() => onAgentChange(agent.id)}
            className="flex items-center gap-2"
          >
            <Avatar size="sm">
              <AvatarAgentFallback name={agent.name.toLowerCase()} />
            </Avatar>
            <span className="truncate">{agent.name}</span>
            {currentAgent === agent.id && (
              <span className="ml-auto text-xs text-muted-foreground">✓</span>
            )}
          </DropdownMenuItem>
        ))
      )}
    </div>
  );
}
