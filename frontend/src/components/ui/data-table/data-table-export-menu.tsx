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
import { t } from "@/lib/i18n";

export function DataTableExportMenu() {
  const { handleExport } = useDataTable();

  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <Button variant="outline" size="sm" className="h-8">
          <Download className="mr-2 h-4 w-4" />
          {t("Export")}
          <ChevronDown className="ml-2 h-4 w-4" />
        </Button>
      </DropdownMenuTrigger>
      <DropdownMenuPortal>
        <DropdownMenuContent align="end" className="w-[260px]">
          <DropdownMenuLabel>{t("Export as")}</DropdownMenuLabel>
          <DropdownMenuSeparator />
          <DropdownMenuGroup>
            <DropdownMenuItem onClick={() => handleExport({ format: "csv" })}>
              <FileText className="mr-2 h-4 w-4" />
              CSV
            </DropdownMenuItem>

            <DropdownMenuItem disabled>
              <FileSpreadsheet className="mr-2 h-4 w-4" />
              {t("Excel")}
              <Badge variant="outline" className="ml-auto">
                {t("Soon")}
              </Badge>
            </DropdownMenuItem>

            <DropdownMenuItem disabled>
              <FileIcon className="mr-2 h-4 w-4" />
              PDF
              <Badge variant="outline" className="ml-auto">
                {t("Soon")}
              </Badge>
            </DropdownMenuItem>
          </DropdownMenuGroup>
        </DropdownMenuContent>
      </DropdownMenuPortal>
    </DropdownMenu>
  );
}
