/* eslint-disable @typescript-eslint/no-explicit-any */
"use client";

import React from "react";
import {
  ColumnDef,
  ColumnFiltersState,
  Row,
  SortingState,
  VisibilityState,
  getCoreRowModel,
  getFilteredRowModel,
  getPaginationRowModel,
  getSortedRowModel,
  useReactTable,
  type RowSelectionState,
} from "@tanstack/react-table";
import { Checkbox } from "../checkbox";
import {
  DataTableContextValue,
  DataTableProviderProps,
  NumberCondition,
} from "./data-table.types";

// Context
const DataTableContext = React.createContext<
  DataTableContextValue<any> | undefined
>(undefined);

// Provider
export function DataTableProvider<TData>({
  children,
  columns,
  data,
  filters = [],
  hasExportOption = true,
  enableRowSelection = false,
  onExport,
  onRowClick,
  onRowHover,
  onRowSelect,
  onRowUnselect,
  onFilter,
  initialColumnVisibility = {},
  initialColumnFilters = [],
  initialCustomFilters = {},
}: DataTableProviderProps<TData>) {
  const [sorting, setSorting] = React.useState<SortingState>([]);
  const [columnFilters, setColumnFilters] =
    React.useState<ColumnFiltersState>(initialColumnFilters);
  const [customFilters, setCustomFilters] =
    React.useState<
      Record<string, boolean | string | string[] | NumberCondition[]>
    >(initialCustomFilters);

  const isFirstRender = React.useRef(true);
  const prevColumnFiltersRef =
    React.useRef<ColumnFiltersState>(initialColumnFilters);
  const prevCustomFiltersRef =
    React.useRef<
      Record<string, boolean | string | string[] | NumberCondition[]>
    >(initialCustomFilters);
  const onFilterRef = React.useRef(onFilter);

  // Keep ref up to date
  React.useEffect(() => {
    onFilterRef.current = onFilter;
  }, [onFilter]);

  // Apply default values on mount
  React.useEffect(() => {
    if (isFirstRender.current && filters.length > 0) {
      const defaultColumnFilters: ColumnFiltersState = [];
      const defaultCustomFilters: Record<
        string,
        boolean | string | string[] | NumberCondition[]
      > = {};

      filters.forEach((filter) => {
        if (filter.defaultValue !== undefined) {
          if (filter.accessorKey) {
            defaultColumnFilters.push({
              id: filter.accessorKey,
              value: filter.defaultValue,
            });
          } else {
            defaultCustomFilters[filter.id] = filter.defaultValue;
          }
        }
      });

      if (defaultColumnFilters.length > 0) {
        setColumnFilters(defaultColumnFilters);
      }
      if (Object.keys(defaultCustomFilters).length > 0) {
        setCustomFilters((prev) => ({ ...prev, ...defaultCustomFilters }));
      }
    }
  }, [filters]);

  // Notify parent about filter changes
  React.useEffect(() => {
    if (isFirstRender.current) {
      isFirstRender.current = false;
      return;
    }

    const columnFiltersChanged =
      JSON.stringify(prevColumnFiltersRef.current) !==
      JSON.stringify(columnFilters);
    const customFiltersChanged =
      JSON.stringify(prevCustomFiltersRef.current) !==
      JSON.stringify(customFilters);

    if (columnFiltersChanged || customFiltersChanged) {
      prevColumnFiltersRef.current = columnFilters;
      prevCustomFiltersRef.current = customFilters;
      onFilterRef.current?.({
        columnFilters,
        customFilters,
      });
    }
  }, [columnFilters, customFilters]);

  const [columnVisibility, setColumnVisibility] =
    React.useState<VisibilityState>(initialColumnVisibility);
  const [rowSelection, setRowSelection] = React.useState<RowSelectionState>({});
  const [columnOrder, setColumnOrder] = React.useState<string[]>([]);

  /**
   * 0) Injeção automática de coluna de checkbox se onRowSelect ou onRowUnselect for fornecido
   *    E injeção de filterFn para filtros múltiplos
   */
  const finalColumns = React.useMemo(() => {
    let baseColumns = [...columns];

    // Data Transform: Add filterFn for multiple or boolean filters
    baseColumns = baseColumns.map((col) => {
      const accessor = (col as any).accessorKey;
      const filterDef = filters.find((f) => f.accessorKey === accessor);
      if (filterDef?.type === "multi") {
        return {
          ...col,
          filterFn: "arrIncludesSome",
        };
      }
      if (filterDef?.type === "boolean") {
        return {
          ...col,
          // Business Rule: Boolean filter checks truthiness of value (e.g. > 0)
          filterFn: (
            row: Row<TData>,
            columnId: string,
            filterValue: unknown,
          ) => {
            if (!filterValue) return true;
            const value = row.getValue(columnId);
            return !!value;
          },
        };
      }
      return col;
    });

    // Conditional: Prepend select column if selection callbacks are present or explicitly enabled
    if (!enableRowSelection && !onRowSelect && !onRowUnselect) return baseColumns;

    const checkboxColumn: ColumnDef<TData, any> = {
      id: "select",
      header: ({ table }: any) => (
        <Checkbox
          checked={
            table.getIsAllPageRowsSelected() ||
            (table.getIsSomePageRowsSelected() && "indeterminate")
          }
          onCheckedChange={(value) => table.toggleAllPageRowsSelected(!!value)}
          aria-label="Select all"
          className="translate-y-[2px]"
        />
      ),
      cell: ({ row }: any) => (
        <Checkbox
          checked={row.getIsSelected()}
          onCheckedChange={(value) => row.toggleSelected(!!value)}
          aria-label="Select row"
          className="translate-y-[2px]"
        />
      ),
      enableSorting: false,
      enableHiding: false,
      size: 40,
    };

    return [checkboxColumn, ...baseColumns];
  }, [columns, enableRowSelection, onRowSelect, onRowUnselect, filters]);

  // Sync columnOrder state when finalColumns changes
  React.useEffect(() => {
    setColumnOrder(finalColumns.map((c) => c.id || (c as any).accessorKey || ""));
  }, [finalColumns]);

  /**
   * 1) Geração das opções de filtro via useMemo
   *    Evita recomputar se `data` ou `filters` não mudarem
   */
  const filtersWithOptions = React.useMemo(() => {
    return filters.map((filter) => {
      // If options are already provided (Master Data), use them
      if (filter.options?.length) {
        return filter;
      }

      // Fallback: Generate options dynamically from current data
      // Only for filters with accessorKey
      if (!filter.accessorKey) {
        return filter;
      }

      const uniqueValues = new Set<string>();

      data.forEach((row) => {
        const value = row[filter.accessorKey as keyof TData];
        if (value !== undefined && value !== null) {
          uniqueValues.add(value.toString());
        }
      });

      const options = Array.from(uniqueValues)
        .sort()
        .map((value) => ({
          label: value,
          value,
        }));

      return {
        ...filter,
        options,
      };
    });
  }, [data, filters]);

  /**
   * 2) Função de export, usando useCallback para evitar recriação a cada render.
   *    Se for usada apenas internamente, e não repassada via props, poderia ser
   *    um simples function. Mas useCallback aqui é seguro.
   */
  const handleExport = React.useCallback(
    ({ format }: { format: "csv" | "excel" | "pdf" }) => {
      if (!hasExportOption) return;

      // Monta CSV
      const csvContent = [
        // Headers
        columns.map((col) => {
          // Se o header for uma função (ReactElement), converter para string ou algo custom
          if (typeof col.header === "function") {
            return "[Function Header]"; // ou alguma lógica pra extrair o texto
          }
          return col.header?.toString() || "";
        }),
        // Linhas de dados
        ...data.map((row) =>
          columns.map((col) => {
            // Tenta acessar accessorKey
            // Note: Se col.accessorKey for string, funciona direto:
            const accessor = (col as any).accessorKey;
            if (!accessor) return "";
            const value = row[accessor as keyof TData];
            return value?.toString() || "";
          }),
        ),
      ]
        .map((row) => row.join(","))
        .join("\n");

      // Cria Blob para download
      const blob = new Blob([csvContent], { type: "text/csv;charset=utf-8;" });
      const link = document.createElement("a");
      link.href = URL.createObjectURL(blob);
      link.download = `export.${format}`;
      link.click();

      URL.revokeObjectURL(link.href);

      // Converte blob em File
      const file = new File([blob], `export.${format}`, {
        type: "text/csv;charset=utf-8;",
      });

      onExport?.({ format, file });
    },
    [columns, data, hasExportOption, onExport],
  );

  /**
   * 3) useReactTable: passe as callbacks onSortingChange, onRowSelectionChange,
   *    onColumnFiltersChange etc., se quiser que a tabela atualize o estado automaticamente.
   */
  const table = useReactTable<TData>({
    data,
    columns: finalColumns,
    // Funções de rowModel
    getCoreRowModel: getCoreRowModel(),
    getPaginationRowModel: getPaginationRowModel(),
    getSortedRowModel: getSortedRowModel(),
    getFilteredRowModel: getFilteredRowModel(),
    // Callbacks
    onSortingChange: setSorting, // se quiser permitir que a tabela gerencie sorting
    onColumnFiltersChange: setColumnFilters,
    onColumnVisibilityChange: setColumnVisibility,
    onRowSelectionChange: setRowSelection, // se quiser permitir que a tabela gerencie row selection
    onColumnOrderChange: setColumnOrder,
    // Estados controlados
    state: {
      sorting,
      columnFilters,
      columnVisibility,
      rowSelection,
      columnOrder,
    },
  });

  // Side Effect: Trigger selection callbacks when rowSelection changes
  const prevRowSelection = React.useRef<RowSelectionState>(rowSelection);

  React.useEffect(() => {
    const currentSelection = table.getState().rowSelection;
    const prevSelection = prevRowSelection.current;

    if (onRowSelect || onRowUnselect) {
      // Business Rule: Find newly selected rows
      Object.keys(currentSelection).forEach((id) => {
        if (!prevSelection[id] && currentSelection[id]) {
          const row = table.getRow(id);
          onRowSelect?.(row);
        }
      });

      // Business Rule: Find newly unselected rows
      Object.keys(prevSelection).forEach((id) => {
        if (prevSelection[id] && !currentSelection[id]) {
          const row = table.getRow(id);
          onRowUnselect?.(row);
        }
      });
    }

    prevRowSelection.current = currentSelection;
  }, [rowSelection, table, onRowSelect, onRowUnselect]);

  const handleCustomFilterChange = React.useCallback(
    (id: string, value: boolean | string | string[] | NumberCondition[]) => {
      setCustomFilters((prev) => {
        const next = { ...prev, [id]: value };
        // Remove empty values
        if (Array.isArray(value) && value.length === 0) {
          delete next[id];
        } else if (
          (typeof value === "string" && value === "") ||
          value === undefined ||
          value === null
        ) {
          delete next[id];
        }
        return next;
      });
    },
    [],
  );

  // Monta o objeto de activeFilters (only from column filters)
  const activeFilters = columnFilters
    .map((filter: any) => {
      const filterOption = filtersWithOptions.find(
        (f) => f.accessorKey === filter.id,
      );

      let displayValue = "";

      // Data Transform: Format display value for multiple or single filters
      if (Array.isArray(filter.value)) {
        displayValue = filter.value
          .map((val: any) => {
            const option = filterOption?.options?.find((o) => o.value === val);
            return option?.label || val;
          })
          .join(", ");
      } else if (filterOption?.type === "boolean") {
        // UI Update: Only show label if boolean is true
        return filter.value
          ? {
              id: filter.id,
              label: filterOption.label,
              value: "",
            }
          : null;
      } else {
        const option = filterOption?.options?.find(
          (o) => o.value === filter.value,
        );
        displayValue = option?.label || (filter.value as string);
      }

      return {
        id: filter.id,
        label: filterOption?.label || filter.id,
        value: displayValue,
      };
    })
    .filter(Boolean) as { id: string; label: string; value: string }[];

  // Add custom filters to active filters
  const allActiveFilters = React.useMemo(() => {
    const customFilterEntries = Object.entries(customFilters)
      .filter(([, value]) => {
        if (Array.isArray(value) && value.length === 0) return false;
        if (typeof value === "string" && value === "") return false;
        if (value === undefined || value === null) return false;
        return true;
      })
      .map(([id, value]) => {
        const filterOption = filtersWithOptions.find((f) => f.id === id);

        let displayValue = "";
        if (Array.isArray(value)) {
          if (
            value.length > 0 &&
            typeof value[0] === "object" &&
            value[0] !== null &&
            "operator" in value[0]
          ) {
            // Number conditions
            displayValue = value
              .map((v) => {
                const numV = v as NumberCondition;
                const op = filterOption?.operators?.find(
                  (o) => o.value === numV.operator,
                );
                return `${op?.symbol || numV.operator} ${numV.value}`;
              })
              .join(", ");
          } else {
            // String array
            displayValue = (value as string[])
              .map((val) => {
                const option = filterOption?.options?.find(
                  (o) => o.value === val,
                );
                return option?.label || val;
              })
              .join(", ");
          }
        } else if (typeof value === "boolean") {
          displayValue = "";
        } else {
          displayValue = String(value);
        }

        return {
          id,
          label: filterOption?.label || id,
          value: displayValue,
        };
      });

    return [...activeFilters, ...customFilterEntries];
  }, [activeFilters, customFilters, filtersWithOptions]);

  // Context Value
  const contextValue: DataTableContextValue<TData> = {
    table,
    sorting,
    columnFilters,
    columnVisibility,
    rowSelection,
    columnOrder,
    setColumnOrder,
    data,
    filters: filtersWithOptions,
    customFilters,
    setSorting,
    setColumnFilters,
    setColumnVisibility,
    setRowSelection,
    setCustomFilters,
    onCustomFilterChange: handleCustomFilterChange,
    onRowClick,
    onRowHover,
    onRowSelect,
    onRowUnselect,
    handleExport,
    activeFilters: allActiveFilters,
  };

  return (
    <DataTableContext.Provider value={contextValue}>
      {children}
    </DataTableContext.Provider>
  );
}

// Custom Hook
export function useDataTable<TData>() {
  const context = React.useContext(DataTableContext);
  if (!context) {
    throw new Error("useDataTable must be used within a DataTableProvider");
  }
  return context as DataTableContextValue<TData>;
}
