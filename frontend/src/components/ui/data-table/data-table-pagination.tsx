"use client";

import React from "react";
import {
  ChevronLeft,
  ChevronRight,
  ChevronsLeft,
  ChevronsRight,
} from "lucide-react";
import { Button } from "../button";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "../select";
import { useDataTable } from "./data-table-provider";
import { cn } from "@/lib/utils";
import { t } from "@/lib/i18n";

interface DataTablePaginationProps {
  className?: string;
}

export const DataTablePagination = React.memo(function DataTablePagination<
  TData,
>({ className }: DataTablePaginationProps) {
  const { table } = useDataTable<TData>();

  // Se quiser memorizá-los para prevenir renders, pode usar useCallback:
  const handleSetPageSize = React.useCallback(
    (value: string) => {
      table.setPageSize(Number(value));
    },
    [table],
  );

  const goToFirstPage = React.useCallback(() => {
    table.setPageIndex(0);
  }, [table]);

  const goToPreviousPage = React.useCallback(() => {
    table.previousPage();
  }, [table]);

  const goToNextPage = React.useCallback(() => {
    table.nextPage();
  }, [table]);

  const goToLastPage = React.useCallback(() => {
    table.setPageIndex(table.getPageCount() - 1);
  }, [table]);

  const { pageSize, pageIndex } = table.getState().pagination;
  const canPreviousPage = table.getCanPreviousPage();
  const canNextPage = table.getCanNextPage();
  const pageCount = table.getPageCount();

  return (
    <div
      className={cn(
        "flex items-center justify-between space-x-2 h-8 border-t border-border px-8 mt-auto sticky bottom-0 bg-background",
        className,
      )}
    >
      <div className="flex items-center gap-6">
        {/* -- Rows selected count -- */}
        <div className="flex items-center !text-xs gap-x-3">
          <div className="font-medium text-muted-foreground whitespace-nowrap">
            {table.getFilteredSelectedRowModel().rows.length} of{" "}
            {table.getFilteredRowModel().rows.length} selected
          </div>

          {/* Business Rule: Allow selecting/unselecting all rows across all pages */}
          {table.getFilteredRowModel().rows.length > 0 && (
            <Button
              variant="ghost"
              size="sm"
              className="h-7 px-2 text-xs font-bold hover:bg-muted"
              onClick={() => {
                const allSelected = table.getIsAllRowsSelected();
                table.toggleAllRowsSelected(!allSelected);
              }}
            >
              {table.getIsAllRowsSelected()
                ? "Clear Selection"
                : `Select all ${table.getFilteredRowModel().rows.length}`}
            </Button>
          )}
        </div>

        {/* -- Itens por página -- */}
        <div className="flex items-center space-x-2">
          <p className="text-xs font-medium pr-2">{t("Items")}</p>
          <Select value={`${pageSize}`} onValueChange={handleSetPageSize}>
            <SelectTrigger className="!h-6 w-[70px]">
              <SelectValue placeholder={pageSize} />
            </SelectTrigger>
            <SelectContent side="top">
              {[10, 20, 30, 40, 50].map((size) => (
                <SelectItem key={size} value={`${size}`}>
                  {size}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>
      </div>

      {/* -- Navegação de páginas -- */}
      <div className="space-x-4">
        <div className="flex items-center justify-between px-2">
          <div className="flex items-center space-x-6 lg:space-x-8">
            <div className="flex w-[100px] items-center justify-center text-xs font-medium">
              {t("Page")} {pageIndex + 1} of {pageCount}
            </div>
            <div className="flex items-center space-x-2">
              {/* Botão "Primeira página" (desktop) */}
              <Button
                variant="outline"
                className="hidden size-6 p-0 lg:flex hover:bg-muted/50"
                onClick={goToFirstPage}
                disabled={!canPreviousPage}
              >
                <span className="sr-only">{t("Go to first page")}</span>
                <ChevronsLeft className="h-3 w-3" />
              </Button>

              {/* Botão "Página anterior" */}
              <Button
                variant="outline"
                className="size-6 p-0 hover:bg-muted/50"
                onClick={goToPreviousPage}
                disabled={!canPreviousPage}
              >
                <span className="sr-only">{t("Go to previous page")}</span>
                <ChevronLeft className="h-3 w-3" />
              </Button>

              {/* Botão "Próxima página" */}
              <Button
                variant="outline"
                className="size-6 p-0 hover:bg-muted/50"
                onClick={goToNextPage}
                disabled={!canNextPage}
              >
                <span className="sr-only">{t("Go to next page")}</span>
                <ChevronRight className="h-3 w-3" />
              </Button>

              {/* Botão "Última página" (desktop) */}
              <Button
                variant="outline"
                className="hidden size-6 p-0 lg:flex hover:bg-muted/50"
                onClick={goToLastPage}
                disabled={!canNextPage}
              >
                <span className="sr-only">{t("Go to last page")}</span>
                <ChevronsRight className="h-3 w-3" />
              </Button>
            </div>
          </div>
        </div>
      </div>
    </div>
  );
});
