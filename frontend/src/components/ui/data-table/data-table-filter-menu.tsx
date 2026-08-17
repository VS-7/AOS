"use client";

import React from "react";
import { CheckIcon, ChevronDown, Filter, X } from "lucide-react";
import { Button } from "../button";
import { Badge } from "../badge";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuGroup,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuPortal,
  DropdownMenuSeparator,
  DropdownMenuSub,
  DropdownMenuSubContent,
  DropdownMenuSubTrigger,
  DropdownMenuTrigger,
} from "../dropdown-menu";
import { useDataTable } from "./data-table-provider";
import { DataTableBooleanFilter } from "./data-table-boolean-filter";
import { DataTableSelectFilter } from "./data-table-select-filter";
import { DataTableFacetedFilter } from "./data-table-faceted-filter";
import { DataTableTextFilter } from "./data-table-text-filter";
import { DataTableNumberFilter } from "./data-table-number-filter";
import { cn } from "@/lib/utils";
import { ButtonGroup } from "../button-group";

type DataTableFilterMenuProps = {
  className?: string;
  variant?:
    | "default"
    | "destructive"
    | "outline"
    | "secondary"
    | "ghost"
    | "link";
  size?: "default" | "sm" | "lg" | "icon";
};

export function DataTableFilterMenu({
  className,
  variant = "ghost",
  size = "sm",
}: DataTableFilterMenuProps) {
  const {
    table,
    filters: filterOptions,
    onCustomFilterChange,
    customFilters,
  } = useDataTable();

  // Memoize detached and menu filters to avoid recalculation on every render
  const { detachedFilters, menuFilters, totalActiveCount } =
    React.useMemo(() => {
      if (!filterOptions) {
        return { detachedFilters: [], menuFilters: [], totalActiveCount: 0 };
      }

      const detached = filterOptions.filter((f) => f.placement === "detached");
      const menu = filterOptions.filter((f) => f.placement !== "detached");

      // Calculate total active filters from menu filters only (detached have their own UI)
      const activeCount = menu.reduce((count, filter) => {
        const value = filter.accessorKey
          ? table.getColumn(filter.accessorKey)?.getFilterValue()
          : customFilters[filter.id];

        if (filter.type === "multi" && Array.isArray(value)) {
          return count + value.length;
        }
        if (value !== undefined && value !== null && value !== "") {
          return count + 1;
        }
        return count;
      }, 0);

      return {
        detachedFilters: detached,
        menuFilters: menu,
        totalActiveCount: activeCount,
      };
    }, [filterOptions, table, customFilters]);

  const handleClearAllFilters = React.useCallback(() => {
    filterOptions?.forEach((filter) => {
      if (filter.accessorKey) {
        const column = table.getColumn(filter.accessorKey);
        column?.setFilterValue(undefined);
      }
      
      const defaultValue = filter.type === "multi" || filter.type === "number" ? [] : "";
      onCustomFilterChange(filter.id, defaultValue);
    });
  }, [filterOptions, table, onCustomFilterChange]);

  if (!filterOptions?.length) return null;

  return (
    <ButtonGroup className="flex items-center">
      {/* Render detached filters using separate components */}
      {detachedFilters.map((filter) => {
        switch (filter.type) {
          case "boolean":
            return (
              <DataTableBooleanFilter
                key={filter.id}
                id={filter.id}
                accessorKey={filter.accessorKey}
                label={filter.label}
              />
            );
          case "select":
            return (
              <DataTableSelectFilter
                key={filter.id}
                id={filter.id}
                accessorKey={filter.accessorKey}
                label={filter.label}
                options={filter.options || []}
              />
            );
          case "multi":
            return (
              <DataTableFacetedFilter
                key={filter.id}
                id={filter.id}
                accessorKey={filter.accessorKey}
                label={filter.label}
                icon={filter.icon}
                options={filter.options || []}
              />
            );
          case "text":
            return (
              <DataTableTextFilter
                key={filter.id}
                id={filter.id}
                accessorKey={filter.accessorKey}
                label={filter.label}
                icon={filter.icon}
                placeholder={filter.placeholder}
                placement="detached"
              />
            );
          case "number":
            return (
              <DataTableNumberFilter
                key={filter.id}
                id={filter.id}
                accessorKey={filter.accessorKey}
                label={filter.label}
                icon={filter.icon}
                operators={filter.operators}
                defaultOperator={filter.defaultOperator}
                placeholder={filter.placeholder}
                placement="detached"
              />
            );
          default:
            return null;
        }
      })}

      {/* Menu filters in dropdown */}
      {menuFilters.length > 0 && (
        <DropdownMenu>
          <DropdownMenuTrigger asChild>
            <Button
              variant={variant}
              size={size}
              className={cn("h-8 rounded-md", className)}
            >
              <Filter className="mr-2 h-4 w-4" />
              Filters
              {totalActiveCount > 0 && (
                <Badge
                  variant="secondary"
                  className="ml-2 h-5 min-w-[20px] justify-center"
                >
                  {totalActiveCount}
                </Badge>
              )}
              <ChevronDown className="ml-2 h-4 w-4" />
            </Button>
          </DropdownMenuTrigger>
          <DropdownMenuPortal>
            <DropdownMenuContent align="end" className="w-[220px]">
              <DropdownMenuLabel className="flex items-center justify-between">
                <span>Filter by</span>
                {totalActiveCount > 0 && (
                  <Button
                    variant="ghost"
                    size="sm"
                    className="h-6 px-2 text-xs"
                    onClick={handleClearAllFilters}
                  >
                    <X className="mr-1 h-3 w-3" />
                    Clear all
                  </Button>
                )}
              </DropdownMenuLabel>
              <DropdownMenuSeparator />
              <DropdownMenuGroup>
                {menuFilters.map((filter) => {
                  // For custom filters without accessorKey, use customFilters from context
                  const column = filter.accessorKey
                    ? table.getColumn(filter.accessorKey)
                    : null;
                  const value = column ? column.getFilterValue() : customFilters[filter.id];

                  // Boolean filter
                  if (filter.type === "boolean") {
                    const isChecked = !!value;
                    return (
                      <DropdownMenuItem
                        key={filter.id}
                        onClick={(e) => {
                          e.preventDefault();
                          if (column) {
                            column.setFilterValue(isChecked ? undefined : true);
                          }
                          onCustomFilterChange(filter.id, !isChecked);
                        }}
                      >
                        {filter.icon && (
                          <span className="mr-2">{filter.icon}</span>
                        )}
                        {filter.label}
                        {isChecked && <CheckIcon className="ml-auto h-4 w-4" />}
                      </DropdownMenuItem>
                    );
                  }

                  // Select/Multi filter
                  if (filter.type === "select" || filter.type === "multi") {
                    if (!filter.options?.length) return null;

                    const selectedCount =
                      filter.type === "multi" && Array.isArray(value)
                        ? value.length
                        : value
                          ? 1
                          : 0;

                    return (
                      <DropdownMenuSub key={filter.id}>
                        <DropdownMenuSubTrigger className="pr-2">
                          <div className="flex items-center justify-between flex-1">
                            <div className="flex items-center">
                              {filter.icon && (
                                <span className="mr-2">{filter.icon}</span>
                              )}
                              <span>{filter.label}</span>
                            </div>
                            {selectedCount > 0 && (
                              <Badge
                                variant="secondary"
                                className="ml-2 h-5 min-w-[20px] justify-center text-xs"
                              >
                                {selectedCount}
                              </Badge>
                            )}
                          </div>
                        </DropdownMenuSubTrigger>
                        <DropdownMenuPortal>
                          <DropdownMenuSubContent className="w-[200px]">
                            {filter.options.map((option) => {
                              const isSelected =
                                filter.type === "multi"
                                  ? (value as string[])?.includes(option.value)
                                  : value === option.value;

                              return (
                                <DropdownMenuItem
                                  key={option.value}
                                  className="capitalize"
                                  onClick={(e) => {
                                    if (filter.type === "multi") {
                                      e.preventDefault();
                                      const current = (
                                        (value || []) as string[]
                                      ).includes(option.value)
                                        ? (value as string[])?.filter(
                                            (v) => v !== option.value,
                                          )
                                        : [
                                            ...((value as string[]) || []),
                                            option.value,
                                          ];
                                      if (column) {
                                        column.setFilterValue(
                                          current.length ? current : undefined,
                                        );
                                      }
                                      onCustomFilterChange(filter.id, current);
                                    } else {
                                      const newValue =
                                        value === option.value
                                          ? ""
                                          : option.value;
                                      if (column) {
                                        column.setFilterValue(
                                          newValue || undefined,
                                        );
                                      }
                                      onCustomFilterChange(filter.id, newValue);
                                    }
                                  }}
                                >
                                  <span className="flex-1">{option.label}</span>
                                  {isSelected && (
                                    <CheckIcon className="h-4 w-4 ml-2" />
                                  )}
                                </DropdownMenuItem>
                              );
                            })}
                          </DropdownMenuSubContent>
                        </DropdownMenuPortal>
                      </DropdownMenuSub>
                    );
                  }

                  // Text filter in menu
                  if (filter.type === "text") {
                    return (
                      <DropdownMenuSub key={filter.id}>
                        <DropdownMenuSubTrigger className="pr-2">
                          <div className="flex items-center justify-between flex-1">
                            <div className="flex items-center">
                              {filter.icon && (
                                <span className="mr-2">{filter.icon}</span>
                              )}
                              <span>{filter.label}</span>
                            </div>
                            {!!value && (
                              <Badge
                                variant="secondary"
                                className="ml-2 h-5 px-1 font-normal max-w-[80px] truncate"
                              >
                                "{String(value)}"
                              </Badge>
                            )}
                          </div>
                        </DropdownMenuSubTrigger>
                        <DropdownMenuPortal>
                          <DropdownMenuSubContent className="w-[240px] p-3">
                            <span className="text-[10px] font-medium text-muted-foreground block border-b pb-1 mb-2 uppercase tracking-wider">
                              Filter by {filter.label}
                            </span>
                            <DataTableTextFilter
                              id={filter.id}
                              accessorKey={filter.accessorKey}
                              label={filter.label}
                              icon={filter.icon}
                              placeholder={filter.placeholder}
                              placement="menu"
                            />
                          </DropdownMenuSubContent>
                        </DropdownMenuPortal>
                      </DropdownMenuSub>
                    );
                  }

                  // Number filter in menu
                  if (filter.type === "number") {
                    const activeCount = Array.isArray(value) ? value.length : 0;
                    return (
                      <DropdownMenuSub key={filter.id}>
                        <DropdownMenuSubTrigger className="pr-2">
                          <div className="flex items-center justify-between flex-1">
                            <div className="flex items-center">
                              {filter.icon && (
                                <span className="mr-2">{filter.icon}</span>
                              )}
                              <span>{filter.label}</span>
                            </div>
                            {activeCount > 0 && (
                              <Badge
                                variant="secondary"
                                className="ml-2 h-5 min-w-[20px] justify-center text-xs"
                              >
                                {activeCount}
                              </Badge>
                            )}
                          </div>
                        </DropdownMenuSubTrigger>
                        <DropdownMenuPortal>
                          <DropdownMenuSubContent className="w-[280px] p-3">
                            <span className="text-[10px] font-medium text-muted-foreground block border-b pb-1 mb-2 uppercase tracking-wider">
                              Filter by {filter.label}
                            </span>
                            <DataTableNumberFilter
                              id={filter.id}
                              accessorKey={filter.accessorKey}
                              label={filter.label}
                              icon={filter.icon}
                              operators={filter.operators}
                              defaultOperator={filter.defaultOperator}
                              placeholder={filter.placeholder}
                              placement="menu"
                            />
                          </DropdownMenuSubContent>
                        </DropdownMenuPortal>
                      </DropdownMenuSub>
                    );
                  }

                  return null;
                })}
              </DropdownMenuGroup>
            </DropdownMenuContent>
          </DropdownMenuPortal>
        </DropdownMenu>
      )}
    </ButtonGroup>
  );
}
