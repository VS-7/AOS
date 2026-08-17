"use client";

import { cn } from "@/lib/utils";
import { DataTableSearch } from "./data-table-search";
import { DataTableFilterMenu } from "./data-table-filter-menu";
import { DataTableExportMenu } from "./data-table-export-menu";
import { DataTableViewOptions } from "./data-table-view-options";

interface DataTableToolbarProps {
  className?: string;
}

/**
 * The row above the table: search, filters, and per-table actions.
 *
 * `index.ts` names this module but it was absent from the reconstructed
 * source — every piece it composes (search, filter menu, export, view
 * options) already exists and reads shared state from `useDataTable()`
 * itself, so this is a small, faithful reconstruction rather than a new
 * design: the same four controls the rest of the folder already implies.
 */
export function DataTableToolbar({ className }: DataTableToolbarProps) {
  return (
    <div className={cn("flex items-center justify-between gap-2 px-1 py-2", className)}>
      <div className="flex flex-1 items-center gap-2">
        <DataTableSearch />
        <DataTableFilterMenu />
      </div>
      <div className="flex items-center gap-2">
        <DataTableExportMenu />
        <DataTableViewOptions />
      </div>
    </div>
  );
}
