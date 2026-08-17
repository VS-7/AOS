"use client";

import {
  FileText,
  FileSpreadsheet,
  FileIcon,
  ChevronDown,
  Download,
} from "lucide-react";
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
  DropdownMenuTrigger,
} from "../dropdown-menu";
import { useDataTable } from "./data-table-provider";

export function DataTableExportMenu() {
  const { handleExport } = useDataTable();

  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <Button variant="outline" size="sm" className="h-8">
          <Download className="mr-2 h-4 w-4" />
          Export
          <ChevronDown className="ml-2 h-4 w-4" />
        </Button>
      </DropdownMenuTrigger>
      <DropdownMenuPortal>
        <DropdownMenuContent align="end" className="w-[260px]">
          <DropdownMenuLabel>Export as</DropdownMenuLabel>
          <DropdownMenuSeparator />
          <DropdownMenuGroup>
            <DropdownMenuItem onClick={() => handleExport({ format: "csv" })}>
              <FileText className="mr-2 h-4 w-4" />
              CSV
            </DropdownMenuItem>

            <DropdownMenuItem disabled>
              <FileSpreadsheet className="mr-2 h-4 w-4" />
              Excel
              <Badge variant="outline" className="ml-auto">
                Soon
              </Badge>
            </DropdownMenuItem>

            <DropdownMenuItem disabled>
              <FileIcon className="mr-2 h-4 w-4" />
              PDF
              <Badge variant="outline" className="ml-auto">
                Soon
              </Badge>
            </DropdownMenuItem>
          </DropdownMenuGroup>
        </DropdownMenuContent>
      </DropdownMenuPortal>
    </DropdownMenu>
  );
}
