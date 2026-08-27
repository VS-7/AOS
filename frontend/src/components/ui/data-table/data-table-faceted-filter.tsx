"use client";

import React from "react";
import { CheckIcon, ChevronDown } from "lucide-react";
import { Button } from "../button";
import { Badge } from "../badge";
import { Separator } from "../separator";
import { Popover, PopoverContent, PopoverTrigger } from "../popover";
import {
  Command,
  CommandEmpty,
  CommandGroup,
  CommandInput,
  CommandItem,
  CommandList,
  CommandSeparator,
} from "../command";
import { useDataTable } from "./data-table-provider";
import { cn } from "@/lib/utils";
import { t } from "@/lib/i18n";

interface DataTableFacetedFilterProps {
  id: string;
  accessorKey?: string;
  label: string;
  icon?: React.ReactNode;
  options: { label: string; value: string }[];
}

export function DataTableFacetedFilter({
  id,
  accessorKey,
  label,
  icon,
  options,
}: DataTableFacetedFilterProps) {
  const { table, onCustomFilterChange, customFilters } = useDataTable();
  const column = accessorKey ? table.getColumn(accessorKey) : null;
  const filterValue = column ? column.getFilterValue() : customFilters[id];
  const selectedValues = React.useMemo(
    () => (filterValue as string[]) || [],
    [filterValue],
  );

  const handleToggle = React.useCallback(
    (value: string) => {
      const current = selectedValues;
      const next = current.includes(value)
        ? current.filter((v) => v !== value)
        : [...current, value];

      if (column) {
        column.setFilterValue(next.length ? next : undefined);
      }
      onCustomFilterChange(id, next);
    },
    [column, id, selectedValues, onCustomFilterChange],
  );

  const handleClear = React.useCallback(() => {
    if (column) {
      column.setFilterValue(undefined);
    }
    onCustomFilterChange(id, []);
  }, [column, id, onCustomFilterChange]);

  return (
    <Popover>
      <PopoverTrigger asChild>
        <Button
          variant="outline"
          size="sm"
          className={cn(
            "h-8 border-dashed bg-background",
            selectedValues.length > 0 && "border-solid",
          )}
        >
          {icon && <span className="mr-2">{icon}</span>}
          {label}
          {selectedValues.length > 0 && (
            <>
              <Separator orientation="vertical" className="mx-2 h-4" />
              <Badge
                variant="secondary"
                className="rounded-sm px-1 font-normal lg:hidden"
              >
                {selectedValues.length}
              </Badge>
              <div className="hidden space-x-1 lg:flex">
                {selectedValues.length > 2 ? (
                  <Badge
                    variant="secondary"
                    className="rounded-sm px-1 font-normal"
                  >
                    {selectedValues.length} selected
                  </Badge>
                ) : (
                  options
                    .filter((option) => selectedValues.includes(option.value))
                    .map((option) => (
                      <Badge
                        key={option.value}
                        variant="secondary"
                        className="rounded-sm px-1 font-normal"
                      >
                        {option.label}
                      </Badge>
                    ))
                )}
              </div>
            </>
          )}
          <ChevronDown className="ml-2 h-4 w-4" />
        </Button>
      </PopoverTrigger>
      <PopoverContent className="w-[200px] p-0" align="start">
        <Command>
          <CommandInput placeholder={label} />
          <CommandList>
            <CommandEmpty>{t("No results found.")}</CommandEmpty>
            <CommandGroup>
              {options.map((option) => {
                const isSelected = selectedValues.includes(option.value);
                return (
                  <CommandItem
                    key={option.value}
                    onSelect={() => handleToggle(option.value)}
                  >
                    <div
                      className={cn(
                        "mr-2 flex h-4 w-4 items-center justify-center rounded-sm border border-primary",
                        isSelected
                          ? "bg-primary text-primary-foreground"
                          : "opacity-50 [&_svg]:invisible",
                      )}
                    >
                      <CheckIcon className={cn("h-4 w-4")} />
                    </div>
                    <span>{option.label}</span>
                  </CommandItem>
                );
              })}
            </CommandGroup>
            {selectedValues.length > 0 && (
              <>
                <CommandSeparator />
                <CommandGroup>
                  <CommandItem
                    onSelect={handleClear}
                    className="justify-center text-center"
                  >
                    {t("Clear filters")}
                  </CommandItem>
                </CommandGroup>
              </>
            )}
          </CommandList>
        </Command>
      </PopoverContent>
    </Popover>
  );
}
