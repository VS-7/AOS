"use client";

import { ColumnsIcon, CheckIcon } from "lucide-react";
import { Button } from "../button";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuGroup,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "../dropdown-menu";
import { useDataTable } from "./data-table-provider";
import { capitalizeLabel } from "./data-table.utils";
import { t } from "@/lib/i18n";

export function DataTableViewOptions() {
  const { table } = useDataTable();

  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <Button variant="outline" size="sm" className="h-8 gap-2">
          <ColumnsIcon className="size-4" />
          {t("Columns")}
        </Button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end" className="w-[180px]">
        <DropdownMenuLabel>{t("Toggle columns")}</DropdownMenuLabel>
        <DropdownMenuSeparator />
        <DropdownMenuGroup>
          {table
            .getAllColumns()
            .filter((column: any) => column.getCanHide())
            .map((column: any) => {
              const rawLabel = typeof column.columnDef.header === "string"
                ? column.columnDef.header
                : column.id;
              const label = capitalizeLabel(rawLabel);
              
              return (
                <DropdownMenuItem
                  key={column.id}
                  onClick={() => column.toggleVisibility(!column.getIsVisible())}
                >
                  {label}
                  {column.getIsVisible() && (
                    <CheckIcon className="ml-auto h-3 w-3" />
                  )}
                </DropdownMenuItem>
              );
            })}
        </DropdownMenuGroup>
      </DropdownMenuContent>
    </DropdownMenu>
  );
}
