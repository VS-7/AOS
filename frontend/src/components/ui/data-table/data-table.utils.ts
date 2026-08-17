/* eslint-disable @typescript-eslint/no-explicit-any */
"use client";

import { DataTableFilters } from "./data-table.types";
import { DEFAULT_NUMBER_OPERATORS } from "./data-table-number-filter";

export interface ColumnDescriptor {
  key: string;
  label: string;
}

/**
 * Analyzes a dataset dynamically and generates matching DataTable filters
 * for each column based on type detection and cardinality.
 */
export function generateFiltersFromData<TData extends Record<string, any>>(
  data: TData[],
  columns: (string | ColumnDescriptor)[],
): DataTableFilters<any>[] {
  if (!data || data.length === 0) return [];

  const parsedColumns: ColumnDescriptor[] = columns.map((col) => {
    if (typeof col === "string") {
      return { key: col, label: col };
    }
    return col;
  });

  return parsedColumns.map((col) => {
    const key = col.key;
    const label = col.label;

    // Get all defined, non-null values for this key
    const values = data
      .map((item) => item[key])
      .filter((val) => val !== null && val !== undefined);

    if (values.length === 0) {
      return {
        id: key,
        accessorKey: key,
        label,
        type: "text",
      };
    }

    // Determine type by inspecting values
    let isBoolean = true;
    let isNumber = true;
    let isString = true;

    for (const val of values) {
      const t = typeof val;
      if (t !== "boolean") isBoolean = false;
      if (t !== "number") isNumber = false;
      if (t !== "string") isString = false;
    }

    if (isBoolean) {
      return {
        id: key,
        accessorKey: key,
        label,
        type: "boolean",
      };
    }

    if (isNumber) {
      return {
        id: key,
        accessorKey: key,
        label,
        type: "number",
        operators: DEFAULT_NUMBER_OPERATORS,
        defaultOperator: "gte",
      };
    }

    if (isString) {
      const uniqueValues = Array.from(new Set(values.map((v) => String(v))));
      // If unique values are relatively low cardinality (e.g. <= 15 unique options and < 60% of total rows)
      if (
        uniqueValues.length <= 15 &&
        uniqueValues.length > 1 &&
        uniqueValues.length < values.length * 0.6
      ) {
        return {
          id: key,
          accessorKey: key,
          label,
          type: "multi",
          options: uniqueValues.sort().map((v) => ({ label: v, value: v })),
        };
      }
    }

    // Fallback to text search
    return {
      id: key,
      accessorKey: key,
      label,
      type: "text",
    };
  });
}

/**
 * Capitalizes a column label helper (e.g. "createdAt" -> "Created At", "id" -> "ID")
 */
export function capitalizeLabel(str: string): string {
  if (!str) return "";
  if (str.toLowerCase() === "id") return "ID";

  const words = str
    .replace(/([A-Z])/g, " $1")
    .replace(/[_-]+/g, " ")
    .trim()
    .split(/\s+/);

  return words
    .map((word) => word.charAt(0).toUpperCase() + word.slice(1).toLowerCase())
    .join(" ");
}
