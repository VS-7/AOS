"use client";

import * as React from "react";
import { ChevronDown, X } from "lucide-react";
import { enUS, ptBR } from "react-day-picker/locale";

import { Calendar } from "@/components/ui/calendar";
import { Button, type ButtonProps } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { calendarDayOf, dayAsUtcMidnight } from "@/lib/calendar-day";
import { getLocale, t } from "@/lib/i18n";
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
  /**
   * The label of the action that removes the date, shown under the calendar
   * while one is set. Without it a date, once picked, could only be replaced.
   */
  clearLabel?: string;
}

function isValidDate(value: Date) {
  return !Number.isNaN(value.getTime());
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

  if (!showTime) return calendarDayOf(value);

  const parsed = new Date(value);
  return isValidDate(parsed) ? parsed : null;
}

function formatLabel(value?: string | null, showTime = false) {
  const date = getDateFromValue(value, showTime);
  if (!date) return null;

  // The interface's language, not the browser's: inside the desktop window
  // the browser default is the system's, which is how an English interface
  // came to show "20 de set. de 2026".
  return new Intl.DateTimeFormat(
    getLocale(),
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
  placeholder = showTime ? t("Pick a date and time") : t("Pick a date"),
  disabled,
  className: _className,
  clearLabel,
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

    onValueChange(dayAsUtcMidnight(date));
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
          {/* mode="single": react-day-picker makes a day clickable only when
              a selection mode (or onDayClick) is given — without it every day
              was plain text and nothing could be picked. `required` keeps a
              click on the selected day from deselecting it into a silent
              "no date"; removing the date is the explicit action below. */}
          <CalendarInput
            mode="single"
            required
            locale={getLocale() === "pt-BR" ? ptBR : enUS}
            selected={selectedDate ?? undefined}
            defaultMonth={selectedDate ?? undefined}
            onSelect={handleDateChange}
            initialFocus
            className="w-full"
          />
        </div>

        {clearLabel && value ? (
          <div className="border-t p-1">
            <Button
              type="button"
              variant="ghost"
              size="sm"
              className="w-full justify-start gap-2 text-muted-foreground"
              onClick={() => {
                onValueChange(undefined);
                setOpen(false);
              }}
            >
              <X className="size-3.5" />
              {clearLabel}
            </Button>
          </div>
        ) : null}

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
