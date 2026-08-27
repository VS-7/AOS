import {
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
} from "@/components/ui/dropdown-menu";
import { Avatar, AvatarAgentFallback } from "@/components/ui/avatar";
import { aos } from "@/app/aos";
import { RoutineHelper } from "@/features/routine/presentation/helpers/routine.helper";
import { ROUTINE_RESERVED_AGENT_CONFIG } from "@/features/routine/presentation/consts/routine";
import { t } from "@/lib/i18n";

interface SetRoutineAgentDropdownProps {
  currentAgent: string;
  onAgentChange: (agent: string) => void;
}

export function SetRoutineAgentDropdown({
  currentAgent,
  onAgentChange,
}: SetRoutineAgentDropdownProps) {
  const agents = aos.stores.agent.useState((state) => state.items);

  return (
    <div className="flex flex-col gap-1">
      <DropdownMenuLabel className="text-xs font-medium text-muted-foreground">
        {t("Workspace targets")}
      </DropdownMenuLabel>

      {RoutineHelper.RESERVED_AGENTS.map((agentId) => (
        <DropdownMenuItem
          key={agentId}
          onClick={() => onAgentChange(agentId)}
          className="flex flex-col items-start gap-0.5"
        >
          <div className="flex w-full items-center gap-2">
            <Avatar size="sm">
              <AvatarAgentFallback name={agentId} />
            </Avatar>
            <span className="truncate">
              {ROUTINE_RESERVED_AGENT_CONFIG[agentId].label}
            </span>
            {currentAgent === agentId && (
              <span className="ml-auto text-xs text-muted-foreground">✓</span>
            )}
          </div>
          <span className="pl-8 text-[10px] text-muted-foreground">
            {ROUTINE_RESERVED_AGENT_CONFIG[agentId].description}
          </span>
        </DropdownMenuItem>
      ))}

      <DropdownMenuSeparator />

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
