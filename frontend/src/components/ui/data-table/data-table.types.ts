/* eslint-disable @typescript-eslint/no-explicit-any */
"use client";

import React from "react";
import {
  ColumnDef,
  ColumnFiltersState,
  Row,
  SortingState,
  VisibilityState,
  useReactTable,
  type RowSelectionState,
} from "@tanstack/react-table";

type ExtractAccessorKey<T> = {
  [K in keyof T]: K extends string ? K : never;
}[keyof T] &
  string;

export type FilterType = "boolean" | "multi" | "select" | "text" | "number";

export interface NumberCondition {
  operator: string;
  value: number;
}

export interface DataTableFilterOption {
  label: string;
  value: string;
}

export interface NumberOperator {
  label: string;
  value: string;
  symbol: string;
}

export const DEFAULT_NUMBER_OPERATORS: NumberOperator[] = [
  { label: "=", value: "eq", symbol: "=" },
  { label: ">=", value: "gte", symbol: "≥" },
  { label: "<=", value: "lte", symbol: "≤" },
  { label: ">", value: "gt", symbol: ">" },
  { label: "<", value: "lt", symbol: "<" },
  { label: "!=", value: "ne", symbol: "≠" },
];

export type DataTableFilters<TAccessorKey extends string> = {
  id: string;
  accessorKey?: TAccessorKey;
  label: string;
  icon?: React.ReactNode;
  type?: FilterType;
  placement?: "menu" | "detached";
  options?: DataTableFilterOption[];
  operators?: NumberOperator[];
  defaultOperator?: string;
  defaultValue?: boolean | string | string[] | NumberCondition[];
  placeholder?: string;
  min?: number;
  max?: number;
  step?: number;
};

export interface DataTableState<TData> {
  sorting: SortingState;
  columnFilters: ColumnFiltersState;
  columnVisibility: VisibilityState;
  rowSelection: Record<string, boolean>;
  columnOrder: string[];
  data: TData[];
  filters?: DataTableFilters<ExtractAccessorKey<TData>>[];
  customFilters: Record<
    string,
    boolean | string | string[] | NumberCondition[]
  >;
  exportData?: () => void;
}

export interface DataTableContextValue<TData> extends DataTableState<TData> {
  table: ReturnType<typeof useReactTable<TData>>;
  handleExport: ({ format }: { format: "csv" | "excel" | "pdf" }) => void;
  activeFilters: { id: string; label: string; value: string }[];
  setCustomFilters: React.Dispatch<
    React.SetStateAction<
      Record<string, boolean | string | string[] | NumberCondition[]>
    >
  >;
  onCustomFilterChange: (
    id: string,
    value: boolean | string | string[] | NumberCondition[],
  ) => void;

  setSorting: React.Dispatch<React.SetStateAction<SortingState>>;
  setColumnFilters: React.Dispatch<React.SetStateAction<ColumnFiltersState>>;
  setColumnVisibility: React.Dispatch<React.SetStateAction<VisibilityState>>;
  setRowSelection: React.Dispatch<React.SetStateAction<RowSelectionState>>;
  setColumnOrder: React.Dispatch<React.SetStateAction<string[]>>;

  onRowClick?: (row: Row<TData>) => void;
  onRowHover?: (row: Row<TData>) => void;
  onRowSelect?: (row: Row<TData>) => void;
  onRowUnselect?: (row: Row<TData>) => void;
}

export interface DataTableProviderProps<TData> {
  children: React.ReactNode;
  columns: ColumnDef<TData, any>[];
  filters?: DataTableFilters<ExtractAccessorKey<TData>>[];
  data: TData[];
  hasExportOption?: boolean;
  enableRowSelection?: boolean;
  onExport?: (params: { format: "csv" | "excel" | "pdf"; file: File }) => void;
  onRowClick?: (row: Row<TData>) => void;
  onRowHover?: (row: Row<TData>) => void;
  onRowSelect?: (row: Row<TData>) => void;
  onRowUnselect?: (row: Row<TData>) => void;
  onFilter?: (filters: {
    columnFilters: ColumnFiltersState;
    customFilters: Record<
      string,
      boolean | string | string[] | NumberCondition[]
    >;
  }) => void;
  initialColumnVisibility?: VisibilityState;
  initialColumnFilters?: ColumnFiltersState;
  initialCustomFilters?: Record<
    string,
    boolean | string | string[] | NumberCondition[]
  >;
}
