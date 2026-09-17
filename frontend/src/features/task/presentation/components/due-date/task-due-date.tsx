import * as React from "react";
import { ChevronDown, X } from "lucide-react";
import { enUS, ptBR } from "react-day-picker/locale";
import { Calendar } from "@/components/ui/calendar";
import { Button } from "@/components/ui/button";
import { Popover, PopoverContent, PopoverTrigger } from "@/components/ui/popover";
import { TaskHelper } from "@/features/task/presentation/helpers/task.helper";
import { getLocale, t } from "@/lib/i18n";
import { cn } from "@/lib/utils";

/**
 * The instant a picked day stands for: that day, at the time of day the task
 * was already due, or at local midnight when it had no due date.
 *
 * Sent as RFC3339, which is the only form tasks_update accepts. The row and
 * card menus used to ask for "YYYY-MM-DD" in a browser prompt and send it as
 * typed, which the daemon refused as "not an RFC3339 instant" — when the
 * prompt opened at all inside the desktop window.
 */
export function dueInstant(day: Date, current?: string | null): string {
  const previous = current ? new Date(current) : null;
  const keep = previous && !Number.isNaN(previous.getTime()) ? previous : null;
  return new Date(
    day.getFullYear(),
    day.getMonth(),
    day.getDate(),
    keep?.getHours() ?? 0,
    keep?.getMinutes() ?? 0,
  ).toISOString();
}

interface TaskDueDateCalendarProps {
  value?: string | null;
  onChange: (value: string | undefined) => void;
}

/** A calendar that sets the due date, with the way to remove it beside it. */
export function TaskDueDateCalendar({ value, onChange }: TaskDueDateCalendarProps) {
  const selected = value ? new Date(value) : undefined;
  const valid = selected && !Number.isNaN(selected.getTime()) ? selected : undefined;

  return (
    <div className="flex flex-col">
      {/* `required`: clicking the selected day would otherwise deselect it and
          call back with no day, which read as a click that did nothing.
          Removing the date is the button below. */}
      <Calendar
        mode="single"
        required
        locale={getLocale() === "pt-BR" ? ptBR : enUS}
        selected={valid}
        defaultMonth={valid}
        onSelect={(day: Date) => onChange(dueInstant(day, value))}
      />
      {valid && (
        <Button
          type="button"
          variant="ghost"
          size="sm"
          className="m-1 mt-0 justify-start gap-2 text-muted-foreground"
          onClick={() => onChange(undefined)}
        >
          <X className="size-3.5" />
          {t("Remove due date")}
        </Button>
      )}
    </div>
  );
}

interface TaskDueDatePickerProps extends TaskDueDateCalendarProps {
  className?: string;
}

/** The task page's due date field: its date in the interface's language, and the calendar. */
export function TaskDueDatePicker({ value, onChange, className }: TaskDueDatePickerProps) {
  const [open, setOpen] = React.useState(false);
  const label = TaskHelper.formatDate(value);

  return (
    <Popover open={open} onOpenChange={setOpen}>
      <PopoverTrigger asChild>
        <button
          type="button"
          className={cn("flex items-center gap-1 rounded px-1.5 py-0.5 text-xs hover:bg-accent", className)}
        >
          <span className={cn(!label && "text-muted-foreground")}>{label ?? t("No due date")}</span>
          <ChevronDown className="ml-auto size-3 shrink-0 opacity-60" />
        </button>
      </PopoverTrigger>
      <PopoverContent className="w-auto p-0" align="start">
        <TaskDueDateCalendar
          value={value}
          onChange={(next) => {
            setOpen(false);
            onChange(next);
          }}
        />
      </PopoverContent>
    </Popover>
  );
}
