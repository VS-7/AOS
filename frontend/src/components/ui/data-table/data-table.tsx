"use client";

import {
  flexRender,
  type Row,
  type Header,
  type HeaderGroup,
  type Cell,
} from "@tanstack/react-table";

import { useDataTable } from "./data-table-provider";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "../table";
import { cn } from "@/lib/utils";
import {
  ArrowUp,
  ArrowDown,
  ArrowUpDown,
  ChevronDown,
  EyeOff,
} from "lucide-react";
import { Button } from "../button";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "../dropdown-menu";
import { capitalizeLabel } from "./data-table.utils";
import { t } from "@/lib/i18n";

// eslint-disable-next-line @typescript-eslint/no-unused-vars
interface DataTableProps<_TData> {
  className?: string;
}

// Componente principal DataTable
export function DataTable<TData>({ className }: DataTableProps<TData>) {
  const { table, onRowClick, onRowHover } = useDataTable<TData>();

  return (
    <div className={cn("h-full bg-background", className)}>
      <Table>
        <TableHeader className="bg-background border-b">
          {table.getHeaderGroups().map((headerGroup: HeaderGroup<TData>) => (
            <TableRow key={headerGroup.id}>
              {headerGroup.headers.map((header: Header<TData, unknown>, index) => {
                const isSortable = header.column.getCanSort();
                const sortedState = header.column.getIsSorted();
                
                const headerContent = typeof header.column.columnDef.header === "string"
                  ? capitalizeLabel(header.column.columnDef.header)
                  : flexRender(header.column.columnDef.header, header.getContext());

                const isSelectOrActions = header.id === "select" || header.id === "actions";

                return (
                  <TableHead
                    key={header.id}
                    className={cn(
                      "group relative select-none",
                      index === 0 && "pl-6",
                    )}
                  >
                    <div className="flex items-center gap-1.5 h-full w-full">
                      {isSortable && !isSelectOrActions ? (
                        <div
                          className="flex items-center gap-1 cursor-pointer hover:text-foreground transition-colors group/sort"
                          onClick={header.column.getToggleSortingHandler()}
                        >
                          <span className="font-semibold text-xs text-foreground/80">{headerContent}</span>
                          <span className="text-muted-foreground/50 group-hover/sort:text-muted-foreground transition-colors">
                            {sortedState === "asc" ? (
                              <ArrowUp className="size-3.5" />
                            ) : sortedState === "desc" ? (
                              <ArrowDown className="size-3.5" />
                            ) : (
                              <ArrowUpDown className="size-3.5 opacity-0 group-hover/sort:opacity-100 transition-opacity" />
                            )}
                          </span>
                        </div>
                      ) : (
                        <div className="font-semibold text-xs text-foreground/80">{headerContent}</div>
                      )}

                      {!isSelectOrActions && (
                        <DropdownMenu>
                          <DropdownMenuTrigger asChild>
                            <Button
                              variant="ghost"
                              size="icon"
                              className="size-5 p-0 opacity-0 group-hover:opacity-100 data-[state=open]:opacity-100 transition-opacity ml-auto hover:bg-muted"
                            >
                              <ChevronDown className="size-3.5" />
                            </Button>
                          </DropdownMenuTrigger>
                          <DropdownMenuContent align="end" className="w-36">
                            {isSortable && (
                              <>
                                <DropdownMenuItem onClick={() => header.column.toggleSorting(false)}>
                                  <ArrowUp className="mr-2 size-3.5 text-muted-foreground" />
                                  {t("Sort Asc")}
                                </DropdownMenuItem>
                                <DropdownMenuItem onClick={() => header.column.toggleSorting(true)}>
                                  <ArrowDown className="mr-2 size-3.5 text-muted-foreground" />
                                  {t("Sort Desc")}
                                </DropdownMenuItem>
                              </>
                            )}
                            {header.column.getCanHide() && (
                              <DropdownMenuItem onClick={() => header.column.toggleVisibility(false)}>
                                <EyeOff className="mr-2 size-3.5 text-muted-foreground" />
                                {t("Hide Column")}
                              </DropdownMenuItem>
                            )}
                          </DropdownMenuContent>
                        </DropdownMenu>
                      )}
                    </div>
                  </TableHead>
                );
              })}
            </TableRow>
          ))}
        </TableHeader>
        <TableBody>
          {table.getRowModel().rows?.length ? (
            table.getRowModel().rows.map((row: Row<TData>) => (
              <TableRow
                key={row.id}
                data-state={row.getIsSelected() && "selected"}
                onClick={() => onRowClick?.(row)}
                onMouseEnter={() => onRowHover?.(row)}
                className={cn(
                  "transition-colors",
                  onRowClick || onRowHover
                    ? "cursor-pointer hover:bg-muted/50"
                    : "",
                )}
              >
                {row.getVisibleCells().map((cell: Cell<TData, unknown>, index) => (
                  <TableCell
                    key={cell.id}
                    className={cn(
                      index === 0 && "pl-6",
                    )}
                  >
                    {flexRender(cell.column.columnDef.cell, cell.getContext())}
                  </TableCell>
                ))}
              </TableRow>
            ))
          ) : (
            <TableRow>
              <TableCell
                colSpan={table.getAllColumns().length}
                className="h-24 text-center"
              >
                {t("Nothing Here! :-)")}
              </TableCell>
            </TableRow>
          )}
        </TableBody>
      </Table>
    </div>
  );
}
