import React from "react";
import { UserIcon } from "lucide-react";
import {
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
} from "@/components/ui/dropdown-menu";
import {
  Avatar,
  AvatarFallback,
  AvatarAgentFallback,
  AvatarImage,
} from "@/components/ui/avatar";
import { aos } from "@/app/aos";
import {
  assigneeAgents,
  assigneeInitials,
  assigneePeople,
} from "@/features/task/presentation/helpers/assignee.helper";

interface SetAssigneeDropdownProps {
  currentAssignee?: string;
  onAssigneeChange: (assignee: string | undefined) => void;
}

export function SetAssigneeDropdown({
  currentAssignee,
  onAssigneeChange,
}: SetAssigneeDropdownProps) {
  const directory = aos.stores.workspace.useState((state) => state.directory);
  const self = aos.stores.auth.useState((state) => state.user);

  const directoryInput = { ...directory, self };
  const people = assigneePeople(directoryInput);
  const agents = assigneeAgents(directoryInput);

  return (
    <div className="flex flex-col gap-1">
      {/* No Assignee Option */}
      <DropdownMenuItem
        onClick={() => onAssigneeChange(undefined)}
        className="flex items-center gap-2"
      >
        <Avatar size="sm">
          <AvatarFallback>
            <UserIcon className="size-3 text-muted-foreground" />
          </AvatarFallback>
        </Avatar>
        <span>No assignee</span>
        {!currentAssignee && <span className="ml-auto text-xs text-muted-foreground">✓</span>}
      </DropdownMenuItem>

      {/* People Section — workspace members + current user */}
      {people.length > 0 && (
        <>
          <DropdownMenuSeparator />
          <DropdownMenuLabel className="text-xs font-medium text-muted-foreground">
            People
          </DropdownMenuLabel>
          {people.map((person) => (
            <DropdownMenuItem
              key={person.id}
              onClick={() => onAssigneeChange(person.id)}
              className="flex items-center gap-2"
            >
              <Avatar size="sm">
                {person.image ? (
                  <AvatarImage src={person.image} alt={person.name} />
                ) : (
                  <AvatarFallback>{assigneeInitials(person.name)}</AvatarFallback>
                )}
              </Avatar>
              <span>{person.name}</span>
              {currentAssignee === person.id && (
                <span className="ml-auto text-xs text-muted-foreground">✓</span>
              )}
            </DropdownMenuItem>
          ))}
        </>
      )}

      <DropdownMenuSeparator />

      {/* Agents Section */}
      <DropdownMenuLabel className="text-xs font-medium text-muted-foreground">
        Agents
      </DropdownMenuLabel>
      {agents.map((agent) => (
        <DropdownMenuItem
          key={agent.id}
          onClick={() => onAssigneeChange(agent.id)}
          className="flex items-center gap-2"
        >
          <Avatar size="sm">
            <AvatarAgentFallback name={agent.name.toLowerCase()} />
          </Avatar>
          <span>{agent.name}</span>
          {currentAssignee === agent.id && (
            <span className="ml-auto text-xs text-muted-foreground">✓</span>
          )}
        </DropdownMenuItem>
      ))}
    </div>
  );
}
