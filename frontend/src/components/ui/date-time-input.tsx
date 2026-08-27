"use client";

import * as React from "react";
import { ChevronDown } from "lucide-react";

import { Calendar } from "@/components/ui/calendar";
import { type ButtonProps } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { t } from "@/lib/i18n";
import {
  Popover,
  PopoverContent,
  PopoverTrigger,
} from "@/components/ui/popover";

const CalendarInput = Calendar as React.ComponentType<any>;

interface DateTimeInputProps {
  value?: string | null;
  onValueChange: (value: string | undefined) => void;
  variant?: ButtonProps["variant"];
  size?: ButtonProps["size"];
  showTime?: boolean;
  placeholder?: string;
  disabled?: boolean;
  className?: string;
}

function isValidDate(value: Date) {
  return !Number.isNaN(value.getTime());
}

function toUtcDateOnlyIso(date: Date) {
  return new Date(
    Date.UTC(date.getFullYear(), date.getMonth(), date.getDate()),
  ).toISOString();
}

function toLocalDateTimeIso(date: Date, hours: number, minutes: number) {
  return new Date(
    date.getFullYear(),
    date.getMonth(),
    date.getDate(),
    hours,
    minutes,
    0,
    0,
  ).toISOString();
}

function parseTimeValue(value?: string | null) {
  if (!value) return { hours: 0, minutes: 0 };

  const [hoursPart = "0", minutesPart = "0"] = value.split(":");
  const hours = Number.parseInt(hoursPart, 10);
  const minutes = Number.parseInt(minutesPart, 10);

  return {
    hours: Number.isFinite(hours) ? hours : 0,
    minutes: Number.isFinite(minutes) ? minutes : 0,
  };
}

function getDateFromValue(value?: string | null, showTime = false) {
  if (!value) return null;

  const parsed = new Date(value);
  if (!isValidDate(parsed)) return null;

  if (showTime) return parsed;

  return new Date(
    parsed.getUTCFullYear(),
    parsed.getUTCMonth(),
    parsed.getUTCDate(),
  );
}

function formatLabel(value?: string | null, showTime = false) {
  const date = getDateFromValue(value, showTime);
  if (!date) return null;

  return new Intl.DateTimeFormat(
    undefined,
    showTime
      ? {
          dateStyle: "medium",
          timeStyle: "short",
        }
      : {
          dateStyle: "medium",
        },
  ).format(date);
}

export function DateTimeInput({
  value,
  onValueChange,
  variant: _variant = "outline",
  size: _size = "default",
  showTime = false,
  placeholder = showTime ? "Pick a date and time" : "Pick a date",
  disabled,
  className: _className,
}: DateTimeInputProps) {
  const [open, setOpen] = React.useState(false);
  const selectedDate = getDateFromValue(value, showTime);
  const selectedTime = showTime && value ? new Date(value) : null;
  const label = formatLabel(value, showTime);

  function handleDateChange(date?: Date) {
    if (!date) {
      onValueChange(undefined);
      if (!showTime) setOpen(false);
      return;
    }

    if (showTime) {
      const currentTime = selectedTime
        ? {
            hours: selectedTime.getHours(),
            minutes: selectedTime.getMinutes(),
          }
        : { hours: 0, minutes: 0 };

      onValueChange(
        toLocalDateTimeIso(date, currentTime.hours, currentTime.minutes),
      );
      return;
    }

    onValueChange(toUtcDateOnlyIso(date));
    setOpen(false);
  }

  function handleTimeChange(event: React.ChangeEvent<HTMLInputElement>) {
    if (!showTime) return;

    const { hours, minutes } = parseTimeValue(event.target.value);
    const date = selectedDate ?? new Date();

    onValueChange(toLocalDateTimeIso(date, hours, minutes));
  }

  return (
    <Popover open={open} onOpenChange={setOpen}>
      <PopoverTrigger asChild>
        <button
          className="flex items-center gap-1 rounded px-1.5 py-0.5 text-xs hover:bg-accent"
          type="button"
          disabled={disabled}
        >
          {label || placeholder}
          <ChevronDown className="ml-auto size-3 shrink-0 opacity-60" />
        </button>
      </PopoverTrigger>
      <PopoverContent className="p-0" align="start">
        <div className="p-1.5 w-full">
          <CalendarInput
            selected={selectedDate ?? undefined}
            onSelect={handleDateChange}
            initialFocus
            className="w-full"
          />
        </div>

        {showTime ? (
          <div className="border-t p-3">
            <label className="mb-1 block text-xs text-muted-foreground">
              {t("Time")}
            </label>
            <Input
              type="time"
              value={
                selectedTime
                  ? `${String(selectedTime.getHours()).padStart(2, "0")}:${String(
                      selectedTime.getMinutes(),
                    ).padStart(2, "0")}`
                  : ""
              }
              onChange={handleTimeChange}
              className="h-8"
            />
          </div>
        ) : null}
      </PopoverContent>
    </Popover>
  );
}
