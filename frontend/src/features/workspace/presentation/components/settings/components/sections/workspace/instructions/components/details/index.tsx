import React from "react";
import { FileText, MapPin, Tag } from "lucide-react";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import {
  FormControl,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from "@/components/ui/form";
import { Textarea } from "@/components/ui/textarea";
import { SplitPageLayout } from "@/components/ui/split-page-layout";
import { useInstructions } from "../../contexts/instructions.context";

const INSTRUCTION_TYPES = ["standards", "patterns", "workflows"] as const;

export function SelectedInstructionDetail() {
  const { selectedInstruction, selectedInstructionId, isCreateMode, form } =
    useInstructions();

  if (!selectedInstructionId && !isCreateMode) return null;

  const currentType = form.watch("type");
  const currentPaths = form.watch("pathsText");
  const pathCount = currentPaths
    ?.split("\n")
    .map((path: string) => path.trim())
    .filter(Boolean).length;
  const typeOptions = Array.from(
    new Set([
      ...INSTRUCTION_TYPES,
      ...(currentType && !INSTRUCTION_TYPES.includes(currentType)
        ? [currentType]
        : []),
    ]),
  );

  return (
    <SplitPageLayout.DetailTabs defaultValue="overview">
      <SplitPageLayout.DetailTab value="overview" label="Overview">
        <SplitPageLayout.Widget>
          <SplitPageLayout.WidgetHeader>
            <SplitPageLayout.WidgetTitle>
              Properties
            </SplitPageLayout.WidgetTitle>
          </SplitPageLayout.WidgetHeader>
          <SplitPageLayout.WidgetContent>
            <FormField
              control={form.control}
              name="type"
              render={({ field }) => (
                <FormItem className="border-0 p-0">
                  <SplitPageLayout.WidgetItem>
                    <Tag className="size-3.5 shrink-0 text-muted-foreground" />
                    <span className="w-14 shrink-0 text-xs text-muted-foreground">
                      Type
                    </span>
                    <DropdownMenu>
                      <DropdownMenuTrigger asChild>
                        <button
                          type="button"
                          className="rounded px-1.5 py-0.5 text-xs hover:bg-accent"
                        >
                          {field.value || "Select type"}
                        </button>
                      </DropdownMenuTrigger>
                      <DropdownMenuContent align="start">
                        {typeOptions.map((type) => (
                          <DropdownMenuItem
                            key={type}
                            onClick={() => field.onChange(type)}
                          >
                            {type}
                          </DropdownMenuItem>
                        ))}
                      </DropdownMenuContent>
                    </DropdownMenu>
                  </SplitPageLayout.WidgetItem>
                  <FormMessage />
                </FormItem>
              )}
            />

            <SplitPageLayout.WidgetItem>
              <MapPin className="size-3.5 shrink-0 text-muted-foreground" />
              <span className="w-14 shrink-0 text-xs text-muted-foreground">
                Scope
              </span>
              <span className="text-xs text-foreground">
                {pathCount
                  ? `${pathCount} path${pathCount !== 1 ? "s" : ""}`
                  : "Global"}
              </span>
            </SplitPageLayout.WidgetItem>

            {!isCreateMode && selectedInstruction ? (
              <SplitPageLayout.WidgetItem>
                <FileText className="size-3.5 shrink-0 text-muted-foreground" />
                <span className="w-14 shrink-0 text-xs text-muted-foreground">
                  ID
                </span>
                <span className="font-mono text-xs text-muted-foreground">
                  {selectedInstruction.id}
                </span>
              </SplitPageLayout.WidgetItem>
            ) : null}
          </SplitPageLayout.WidgetContent>
        </SplitPageLayout.Widget>

        <SplitPageLayout.Widget>
          <SplitPageLayout.WidgetHeader>
            <SplitPageLayout.WidgetTitle>Paths</SplitPageLayout.WidgetTitle>
          </SplitPageLayout.WidgetHeader>
          <SplitPageLayout.WidgetContent>
            <FormField
              control={form.control}
              name="pathsText"
              render={({ field }) => (
                <FormItem className="w-full space-y-2">
                  <FormLabel className="text-xs text-muted-foreground">
                    One glob per line
                  </FormLabel>
                  <FormControl>
                    <Textarea
                      placeholder={"src/features/**/*.ts\nsrc/@/**/*"}
                      className="min-h-32 resize-none text-sm"
                      {...field}
                      value={field.value ?? ""}
                    />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />
          </SplitPageLayout.WidgetContent>
        </SplitPageLayout.Widget>
      </SplitPageLayout.DetailTab>
    </SplitPageLayout.DetailTabs>
  );
}
