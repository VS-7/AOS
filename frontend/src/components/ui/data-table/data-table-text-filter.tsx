"use client";

import React from "react";
import { Search, ChevronDown } from "lucide-react";
import { Button } from "../button";
import { Input } from "../input";
import { Badge } from "../badge";
import { Popover, PopoverContent, PopoverTrigger } from "../popover";
import { useDataTable } from "./data-table-provider";
import { cn } from "@/lib/utils";
import { t } from "@/lib/i18n";

interface DataTableTextFilterProps {
  id: string;
  accessorKey?: string;
  label: string;
  icon?: React.ReactNode;
  placeholder?: string;
  placement?: "detached" | "menu";
}

export function DataTableTextFilter({
  id,
  accessorKey,
  label,
  icon,
  placeholder = "Search...",
  placement = "detached",
}: DataTableTextFilterProps) {
  const { table, onCustomFilterChange, customFilters } = useDataTable();
  const column = accessorKey ? table.getColumn(accessorKey) : null;

  const filterValue = column
    ? (column.getFilterValue() as string) || ""
    : (customFilters[id] as string) || "";

  const [localValue, setLocalValue] = React.useState(filterValue);
  const [open, setOpen] = React.useState(false);

  React.useEffect(() => {
    setLocalValue(filterValue);
  }, [filterValue]);

  const handleApply = React.useCallback(() => {
    if (column) {
      column.setFilterValue(localValue || undefined);
    }
    onCustomFilterChange(id, localValue);
    setOpen(false);
  }, [column, id, localValue, onCustomFilterChange]);

  const handleClear = React.useCallback(() => {
    setLocalValue("");
    if (column) {
      column.setFilterValue(undefined);
    }
    onCustomFilterChange(id, "");
    setOpen(false);
  }, [column, id, onCustomFilterChange]);

  const handleKeyDown = React.useCallback(
    (e: React.KeyboardEvent<HTMLInputElement>) => {
      if (e.key === "Enter") {
        handleApply();
      }
    },
    [handleApply],
  );

  const isActive = filterValue !== "";

  const FilterContent = () => (
    <div className="flex items-center gap-1">
      <Input
        type="text"
        placeholder={placeholder}
        value={localValue}
        onChange={(e) => setLocalValue(e.target.value)}
        onKeyDown={handleKeyDown}
        className="h-8 border-none focus-visible:ring-0 focus-visible:ring-offset-0 text-xs"
      />
      <Button
        variant="ghost"
        size="sm"
        className="h-7 w-7 px-0"
        onClick={handleApply}
      >
        <Search className="h-4 w-4" />
      </Button>
    </div>
  );

  if (placement === "detached") {
    return (
      <Popover open={open} onOpenChange={setOpen}>
        <PopoverTrigger asChild>
          <Button
            variant="outline"
            size="sm"
            className={cn(
              "h-8 border-dashed bg-background",
              isActive && "border-solid",
            )}
          >
            {icon && <span className="mr-2">{icon}</span>}
            {label}
            {isActive && (
              <>
                <Badge
                  variant="secondary"
                  className="ml-2 h-5 px-1 font-normal"
                >
                  "{filterValue}"
                </Badge>
              </>
            )}
            <ChevronDown className="ml-2 h-4 w-4" />
          </Button>
        </PopoverTrigger>
        <PopoverContent className="w-[250px] p-3" align="start">
          <div className="space-y-2">
            <span className="text-xs font-medium text-muted-foreground">
              {label}
            </span>
            <FilterContent />
            <div className="flex gap-2 pt-2">
              <Button
                size="sm"
                variant="outline"
                className="flex-1"
                onClick={handleClear}
              >
                {t("Clear")}
              </Button>
              <Button size="sm" className="flex-1" onClick={handleApply}>
                {t("Filter")}
              </Button>
            </div>
          </div>
        </PopoverContent>
      </Popover>
    );
  }

  return (
    <div className="space-y-2">
      <FilterContent />
      <div className="flex gap-2 pt-1">
        <Button
          size="sm"
          variant="outline"
          className="flex-1 h-7 text-xs"
          onClick={handleClear}
        >
          {t("Clear")}
        </Button>
        <Button size="sm" className="flex-1 h-7 text-xs" onClick={handleApply}>
          {t("Apply")}
        </Button>
      </div>
    </div>
  );
}
