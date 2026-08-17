"use client";

import React from "react";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "../select";
import { useDataTable } from "./data-table-provider";

interface DataTableSelectFilterProps {
  id: string;
  accessorKey?: string;
  label: string;
  options: { label: string; value: string }[];
}

export function DataTableSelectFilter({
  id,
  accessorKey,
  label,
  options,
}: DataTableSelectFilterProps) {
  const { table, onCustomFilterChange, customFilters } = useDataTable();
  const column = accessorKey ? table.getColumn(accessorKey) : null;
  const value = column
    ? (column.getFilterValue() as string) || ""
    : (customFilters[id] as string) || "";

  const handleValueChange = React.useCallback(
    (newValue: string) => {
      const finalValue = newValue === "ALL" ? "" : newValue;
      if (column) {
        column.setFilterValue(finalValue || undefined);
      }
      onCustomFilterChange(id, finalValue);
    },
    [column, id, onCustomFilterChange],
  );

  return (
    <Select value={value} onValueChange={handleValueChange}>
      <SelectTrigger className="h-8 w-[150px] border bg-background">
        <SelectValue placeholder={label} />
      </SelectTrigger>
      <SelectContent>
        <SelectItem value="ALL">Todas</SelectItem>
        {options.map((option) => (
          <SelectItem key={option.value} value={option.value}>
            {option.label}
          </SelectItem>
        ))}
      </SelectContent>
    </Select>
  );
}
