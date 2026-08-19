import { DropdownMenuItem } from "@/components/ui/dropdown-menu";
import type { FractalRoutine } from "@/features/routine/interfaces/routine.interfaces";
import { ROUTINE_STATUS_CONFIG, ROUTINE_STATUS_ORDER } from "@/features/routine/presentation/consts/routine";

interface SetRoutineStatusDropdownProps {
  currentStatus: FractalRoutine["status"];
  onStatusChange: (status: FractalRoutine["status"]) => void;
}

export function SetRoutineStatusDropdown({
  currentStatus,
  onStatusChange,
}: SetRoutineStatusDropdownProps) {
  return (
    <div className="flex flex-col gap-1">
      {ROUTINE_STATUS_ORDER.map((status) => {
        const config = ROUTINE_STATUS_CONFIG[status];
        const Icon = config.icon;

        return (
          <DropdownMenuItem
            key={status}
            onClick={() => onStatusChange(status)}
            className="flex items-center gap-2"
          >
            <Icon className={`size-4 ${config.color}`} />
            <span>{config.label}</span>
            {currentStatus === status && (
              <span className="ml-auto text-xs text-muted-foreground">✓</span>
            )}
          </DropdownMenuItem>
        );
      })}
    </div>
  );
}
