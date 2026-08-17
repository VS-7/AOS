"use client";

import React from "react";
import { Search, X } from "lucide-react";
import { AnimatePresence, motion } from "motion/react";
import { useDataTable } from "./data-table-provider";
import {
  InputGroup,
  InputGroupAddon,
  InputGroupInput,
} from "../input-group";

interface DataTableSearchProps {
  placeholder?: string;
  className?: string;
}

export function DataTableSearch({
  placeholder = "Search...",
  className,
  ...props
}: DataTableSearchProps) {
  const { table } = useDataTable();
  const globalFilter = (table.getState().globalFilter as string) ?? "";

  const handleSearch = React.useCallback(
    (val: string) => {
      table.setGlobalFilter(val || undefined);
    },
    [table],
  );

  const handleClear = React.useCallback(() => {
    table.setGlobalFilter(undefined);
  }, [table]);

  return (
    <InputGroup className={className}>
      <InputGroupAddon align="inline-start">
        <Search className="size-4 text-muted-foreground" />
      </InputGroupAddon>
      <InputGroupInput
        placeholder={placeholder}
        value={globalFilter}
        onChange={(event) => handleSearch(event.target.value)}
        {...props}
      />
      <AnimatePresence>
        {globalFilter && (
          <InputGroupAddon align="inline-end">
            <motion.button
              type="button"
              onClick={handleClear}
              initial={{ scale: 0, opacity: 0 }}
              animate={{ scale: 1, opacity: 1 }}
              exit={{ scale: 0, opacity: 0 }}
              transition={{ type: "spring", stiffness: 350, damping: 25 }}
              className="flex size-5 items-center justify-center rounded-md bg-muted hover:bg-muted-foreground/20 text-muted-foreground hover:text-foreground transition-colors cursor-pointer"
            >
              <X className="size-3" />
            </motion.button>
          </InputGroupAddon>
        )}
      </AnimatePresence>
    </InputGroup>
  );
}
