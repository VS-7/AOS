"use client";

import React from "react";
import { Switch } from "../switch";
import { Label } from "../label";
import { useDataTable } from "./data-table-provider";

interface DataTableBooleanFilterProps {
  id: string;
  accessorKey?: string;
  label: string;
}

export function DataTableBooleanFilter({
  id,
  accessorKey,
  label,
}: DataTableBooleanFilterProps) {
  const { table, onCustomFilterChange, customFilters } = useDataTable();
  const column = accessorKey ? table.getColumn(accessorKey) : null;
  const isChecked = column ? !!column.getFilterValue() : !!customFilters[id];

  const handleToggle = React.useCallback(
    (checked: boolean) => {
      if (column) {
        column.setFilterValue(checked || undefined);
      }
      onCustomFilterChange(id, checked);
    },
    [column, id, onCustomFilterChange],
  );

  return (
    <div className="flex items-center space-x-2 border rounded-md px-3 h-8 bg-background">
      <Switch
        id={id}
        checked={isChecked}
        onCheckedChange={handleToggle}
        className="scale-75"
      />
      <Label htmlFor={id} className="text-xs font-medium cursor-pointer">
        {label}
      </Label>
    </div>
  );
}
