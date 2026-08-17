"use client";

import React from "react";
import { Plus, X, ChevronDown, Minus } from "lucide-react";
import { Button } from "../button";
import { Input } from "../input";
import { Badge } from "../badge";
import { Popover, PopoverContent, PopoverTrigger } from "../popover";
import { ToggleGroup, ToggleGroupItem } from "../toggle-group";
import { useDataTable } from "./data-table-provider";
import {
  NumberCondition,
  NumberOperator,
  DEFAULT_NUMBER_OPERATORS,
} from "./data-table.types";
import { cn } from "@/lib/utils";
import { Label } from "../label";
import { ButtonGroup } from "../button-group";

interface DataTableNumberFilterProps {
  id: string;
  accessorKey?: string;
  label: string;
  icon?: React.ReactNode;
  operators?: NumberOperator[];
  defaultOperator?: string;
  placeholder?: string;
  placement?: "detached" | "menu";
  min?: number;
  max?: number;
  step?: number;
}

function DataTableNumberFilter({
  id,
  accessorKey,
  label,
  icon,
  operators = DEFAULT_NUMBER_OPERATORS,
  defaultOperator = "gte",
  placeholder = "Value",
  placement = "detached",
  min,
  max,
  step = 1,
}: DataTableNumberFilterProps) {
  const { table, onCustomFilterChange, customFilters } = useDataTable();
  const column = accessorKey ? table.getColumn(accessorKey) : null;

  const filterValue = column
    ? (column.getFilterValue() as NumberCondition[])
    : (customFilters[id] as NumberCondition[]) || [];

  const existingConditions = React.useMemo<NumberCondition[]>(
    () => (Array.isArray(filterValue) ? filterValue : []),
    [filterValue],
  );

  const [operator, setOperator] = React.useState(defaultOperator);
  const [inputValue, setInputValue] = React.useState("");
  const [open, setOpen] = React.useState(false);

  const handleIncrement = React.useCallback(() => {
    const currentValue = parseFloat(inputValue) || 0;
    let newValue = currentValue + step;
    if (max !== undefined) newValue = Math.min(newValue, max);
    if (min !== undefined) newValue = Math.max(newValue, min);
    setInputValue(String(newValue));
  }, [inputValue, step, min, max]);

  const handleDecrement = React.useCallback(() => {
    const currentValue = parseFloat(inputValue) || 0;
    let newValue = currentValue - step;
    if (max !== undefined) newValue = Math.min(newValue, max);
    if (min !== undefined) newValue = Math.max(newValue, min);
    setInputValue(String(newValue));
  }, [inputValue, step, min, max]);

  const handleAdd = React.useCallback(() => {
    const numValue = parseFloat(inputValue);
    if (isNaN(numValue)) return;

    const newCondition: NumberCondition = {
      operator,
      value: numValue,
    };

    const updatedConditions = [...existingConditions, newCondition];

    if (column) {
      column.setFilterValue(
        updatedConditions.length ? updatedConditions : undefined,
      );
    }
    onCustomFilterChange(id, updatedConditions);

    setInputValue("");
  }, [
    column,
    id,
    operator,
    inputValue,
    existingConditions,
    onCustomFilterChange,
  ]);

  const handleRemove = React.useCallback(
    (index: number) => {
      const updatedConditions = existingConditions.filter(
        (_, i) => i !== index,
      );

      if (column) {
        column.setFilterValue(
          updatedConditions.length ? updatedConditions : undefined,
        );
      }
      onCustomFilterChange(id, updatedConditions);
    },
    [column, id, existingConditions, onCustomFilterChange],
  );

  const handleClear = React.useCallback(() => {
    if (column) {
      column.setFilterValue(undefined);
    }
    onCustomFilterChange(id, []);
    setOpen(false);
  }, [column, id, onCustomFilterChange]);

  const handleKeyDown = React.useCallback(
    (e: React.KeyboardEvent<HTMLInputElement>) => {
      if (e.key === "Enter") {
        handleAdd();
      }
    },
    [handleAdd],
  );

  const isActive = existingConditions.length > 0;

  const FilterContent = () => (
    <div className="flex flex-col gap-2">
      <div>
        <Label>Select operator</Label>

        <ToggleGroup
          type="single"
          value={operator}
          onValueChange={(value) => value && setOperator(value)}
          className="justify-start border divide-x gap-0 w-fit mt-1"
        >
          {operators.map((op) => (
            <ToggleGroupItem
              key={op.value}
              value={op.value}
              className="h-8 px-3 text-xs"
              aria-label={op.label}
            >
              {op.symbol}
            </ToggleGroupItem>
          ))}
        </ToggleGroup>
      </div>

      <div>
        <Label>Enter value</Label>
        <ButtonGroup className="flex items-center mt-1 border">
          <Button
            variant="ghost"
            size="icon"
            className="h-8"
            onClick={handleDecrement}
          >
            <Minus />
          </Button>
          <Input
            type="number"
            placeholder={placeholder}
            value={inputValue}
            onChange={(e) => setInputValue(e.target.value)}
            onKeyDown={handleKeyDown}
            className="h-8 flex-1 text-center"
            min={min}
            max={max}
          />
          <Button
            variant="ghost"
            size="icon"
            className="h-8"
            onClick={handleIncrement}
          >
            <Plus />
          </Button>
        </ButtonGroup>
      </div>
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
              <Badge variant="secondary" className="ml-2 h-5 px-1 font-normal">
                {existingConditions.length}
              </Badge>
            )}
            <ChevronDown className="ml-2 h-4 w-4" />
          </Button>
        </PopoverTrigger>
        <PopoverContent className="w-[280px] p-3" align="start">
          <div className="space-y-3">
            <div className="flex items-center justify-between">
              <span className="text-xs font-medium text-muted-foreground">
                {label}
              </span>

              {isActive && (
                <Button
                  variant="ghost"
                  size="sm"
                  className="h-6 px-2 text-xs"
                  onClick={handleClear}
                >
                  <X className="mr-1 h-3 w-3" />
                  Clear
                </Button>
              )}
            </div>

            <FilterContent />

            {existingConditions.length > 0 && (
              <div className="space-y-1 pt-2 border-t">
                <span className="text-xs text-muted-foreground">
                  Active conditions:
                </span>
                <div className="flex flex-wrap gap-1">
                  {existingConditions.slice(0, 4).map((cond, index) => {
                    const op = operators.find((o) => o.value === cond.operator);
                    return (
                      <Badge
                        key={index}
                        variant="secondary"
                        className="h-6 text-xs flex items-center gap-1"
                      >
                        {op?.symbol || cond.operator} {cond.value}
                        <button
                          onClick={() => handleRemove(index)}
                          className="ml-1 hover:text-destructive"
                        >
                          <X className="h-3 w-3" />
                        </button>
                      </Badge>
                    );
                  })}
                  {existingConditions.length > 4 && (
                    <Badge variant="secondary" className="h-6 text-xs">
                      +{existingConditions.length - 4}
                    </Badge>
                  )}
                </div>
              </div>
            )}

            <div className="flex gap-2 pt-2">
              <Button
                size="sm"
                className="flex-1"
                onClick={() => setOpen(false)}
              >
                Aplicar
              </Button>
            </div>
          </div>
        </PopoverContent>
      </Popover>
    );
  }

  return (
    <div className="space-y-3">
      <FilterContent />
      
      <div className="flex gap-2 pt-1">
        <Button
          size="sm"
          className="flex-1 h-7 text-xs"
          onClick={handleAdd}
        >
          Add Condition
        </Button>
        {isActive && (
          <Button
            variant="outline"
            size="sm"
            className="flex-1 h-7 text-xs"
            onClick={handleClear}
          >
            Clear All
          </Button>
        )}
      </div>

      {existingConditions.length > 0 && (
        <div className="space-y-1 pt-2 border-t">
          <span className="text-[10px] text-muted-foreground block font-medium uppercase tracking-wider">
            Active conditions:
          </span>
          <div className="flex flex-wrap gap-1">
            {existingConditions.map((cond, index) => {
              const op = operators.find((o) => o.value === cond.operator);
              return (
                <Badge
                  key={index}
                  variant="secondary"
                  className="h-6 text-[10px] flex items-center gap-1 font-mono"
                >
                  {op?.symbol || cond.operator} {cond.value}
                  <button
                    onClick={() => handleRemove(index)}
                    className="ml-1 hover:text-destructive"
                  >
                    <X className="h-2.5 w-2.5" />
                  </button>
                </Badge>
              );
            })}
          </div>
        </div>
      )}
    </div>
  );
}

export { DataTableNumberFilter, DEFAULT_NUMBER_OPERATORS };
