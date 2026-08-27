import { t } from "@/lib/i18n";
import * as React from "react"
import { z } from "zod"
import { useRouter } from "@tanstack/react-router"
import {
  GitBranch,
  TagIcon,
  UserIcon,
  ChevronDown,
  Check
} from "lucide-react"

import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle
} from "@/components/ui/dialog"
import {
  FormControl,
  FormField,
  Field,
  FieldGroup,
  FieldLabel,
  Form
} from "@/components/ui/form"
import { Input } from "@/components/ui/input"
import { Textarea } from "@/components/ui/textarea"
import { Button } from "@/components/ui/button"
import {
  Popover,
  PopoverContent,
  PopoverTrigger,
} from "@/components/ui/popover"
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu"
import { Avatar, AvatarFallback, AvatarAgentFallback, AvatarImage } from "@/components/ui/avatar"
import { SetPriorityDropdown } from "@/features/task/presentation/components/dropdowns/set-priority.dropdown"
import { SetAssigneeDropdown } from "@/features/task/presentation/components/dropdowns/set-assignee.dropdown"
import { SetTypeDropdown } from "@/features/task/presentation/components/dropdowns/set-type.dropdown"
import { SetStatusDropdown } from "@/features/task/presentation/components/dropdowns/set-status.dropdown"
import { TASK_PRIORITY_CONFIG, TASK_STATUS_CONFIG } from "@/features/task/presentation/consts/task"
import { assigneeInitials, resolveAssignee } from "@/features/task/presentation/helpers/assignee.helper"
import { aos } from "@/app/aos"
import { cn } from "@/lib/utils"
import type { TaskPriority, TaskStatus } from "@/features/task/interfaces/task.interfaces"

const worktreeSchema = z.object({
  enabled: z.boolean().default(false),
  base: z.string().optional(),
  branch: z.string().optional(),
})

const formSchema = z.object({
  name: z.string().min(1, "Name is required"),
  summary: z.string().optional(),
  type: z.string().default("task"),
  priority: z.enum(["no_priority", "urgent", "high", "medium", "low"]).default("no_priority"),
  status: z.enum([
    "suggestion",
    "backlog",
    "planning",
    "todo",
    "in_progress",
    "stopped",
    "in_review",
    "finished",
  ]).default("backlog"),
  assigned: z.string().optional(),
  worktree: worktreeSchema.default({ enabled: false }),
})

function generateSlug(value: string): string {
  return value
    .normalize("NFD")
    .replace(/[\u0300-\u036f]/g, "")
    .toLowerCase()
    .trim()
    .replace(/\s+/g, "-")
    .replace(/[^\w-]+/g, "")
    .replace(/--+/g, "-")
    .replace(/^-+/, "")
    .replace(/-+$/, "");
}

export function TaskDialog() {
  const router = useRouter();
  const open = aos.stores.viewport.useState(state => state.tasks.dialog.visible);
  const directory = aos.stores.workspace.useState((state) => state.directory);
  const self = aos.stores.auth.useState((state) => state.user);
  const [worktreeOpen, setWorktreeOpen] = React.useState(false)
  const initialValues = React.useMemo(() => ({
    name: "",
    summary: "",
    type: "task",
    priority: "no_priority" as const,
    status: "backlog" as const,
    assigned: undefined as string | undefined,
    worktree: { enabled: false, base: "", branch: "" },
  }), [])

  const form = aos.useForm({
    schema: formSchema,
    mutation: "task.create",
    values: initialValues,
    onSubmit: (values) => ({
      body: {
        name: values.name,
        slug: generateSlug(values.name),
        summary: values.summary,
        type: values.type,
        priority: values.priority,
        status: values.status,
        assigned: values.assigned,
        worktree: values.worktree.enabled
          ? {
            enabled: true,
            ...(values.worktree.base?.trim() ? { base: values.worktree.base.trim() } : {}),
            ...(values.worktree.branch?.trim() ? { branch: values.worktree.branch.trim() } : {}),
          }
          : { enabled: false },
      }
    }),
    onResponse: ({ error }) => {
      if (!error) {
        // Was `'tasks.dialog'` (flat) — but the read above (`state.tasks.
        // dialog.visible`) expects a nested path, a pre-existing
        // inconsistency in the source. Corrected to match.
        aos.stores.viewport.actions.toggle('tasks.dialog.visible', false);
        form.reset();
        // Invalidate all loaders to refresh the task list
        router.invalidate();
      }
    }
  });

  function onOpenChange(isOpen: boolean) {
    aos.stores.viewport.actions.toggle('tasks.dialog.visible', isOpen)
    if (!isOpen) form.reset()
  }

  const worktreeValues = form.watch("worktree")
  const selectedPriority = form.watch("priority") as TaskPriority
  const selectedType = form.watch("type")
  const selectedAssignee = form.watch("assigned")
  const priorityCfg = TASK_PRIORITY_CONFIG[selectedPriority]
  const PriorityIcon = priorityCfg.icon
  const assignee = resolveAssignee({ ...directory, self }, selectedAssignee)
  const isAgent = assignee?.type === "agent"
  const assigneeLabel = assignee?.name || "Unassigned"

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-2xl p-0 gap-0 overflow-hidden">
        <DialogHeader className="sr-only">
          <DialogTitle>{t("Create Task")}</DialogTitle>
        </DialogHeader>
        <Form form={form} className="flex flex-col">
          <div className="p-6 pb-2">
            <FieldGroup>
              <FormField
                control={form.control}
                name="status"
                render={({ field }) => {
                  const statusCfg = TASK_STATUS_CONFIG[field.value as TaskStatus]
                  const StatusIcon = statusCfg.icon

                  return (
                    <Field>
                      <FieldLabel>{t("Status")}</FieldLabel>
                      <FormControl>
                        <DropdownMenu>
                          <DropdownMenuTrigger asChild>
                            <Button
                              variant="outline"
                              size="sm"
                              type="button"
                              className="h-8 w-fit gap-2 px-3 font-normal"
                            >
                              <StatusIcon className={cn("size-4", statusCfg.color)} />
                              <span>{statusCfg.label}</span>
                              <ChevronDown className="size-3 opacity-60" />
                            </Button>
                          </DropdownMenuTrigger>
                          <DropdownMenuContent align="start">
                            <SetStatusDropdown
                              currentStatus={field.value}
                              onStatusChange={(status) => field.onChange(status as TaskStatus)}
                            />
                          </DropdownMenuContent>
                        </DropdownMenu>
                      </FormControl>
                    </Field>
                  )
                }}
              />
              <FormField
                control={form.control}
                name="name"
                render={({ field }) => (
                  <Field>
                    <FormControl>
                      <Input
                        {...field}
                        placeholder={t("Task title")}
                        className="text-xl font-medium border-none shadow-none focus-visible:ring-0 px-0 h-auto"
                        autoFocus
                      />
                    </FormControl>
                  </Field>
                )}
              />
              <FormField
                control={form.control}
                name="summary"
                render={({ field }) => (
                  <Field>
                    <FormControl>
                      <Textarea
                        {...field}
                        placeholder={t("Add description...")}
                        className="border-none shadow-none focus-visible:ring-0 px-0 min-h-25 resize-none text-muted-foreground"
                      />
                    </FormControl>
                  </Field>
                )}
              />
            </FieldGroup>
          </div>

          <div className="flex items-center justify-between px-4 py-3 bg-muted/30 border-t">
            <div className="flex items-center gap-2">
              <DropdownMenu>
                <DropdownMenuTrigger asChild>
                  <Button
                    variant="ghost"
                    size="sm"
                    type="button"
                    className="h-8 gap-2 px-2 text-muted-foreground hover:text-foreground"
                  >
                    <PriorityIcon data-icon="inline-start" className={cn("size-4", priorityCfg.colorClass)} />
                    <span>{priorityCfg.label}</span>
                    <ChevronDown data-icon="inline-end" className="size-3" />
                  </Button>
                </DropdownMenuTrigger>
                <DropdownMenuContent align="start">
                  <SetPriorityDropdown
                    currentPriority={selectedPriority}
                    onPriorityChange={(priority) => form.setValue("priority", priority)}
                  />
                </DropdownMenuContent>
              </DropdownMenu>

              <DropdownMenu>
                <DropdownMenuTrigger asChild>
                  <Button
                    variant="ghost"
                    size="sm"
                    type="button"
                    className="h-8 gap-2 px-2 text-muted-foreground hover:text-foreground capitalize"
                  >
                    <TagIcon data-icon="inline-start" className="size-4" />
                    <span>{selectedType}</span>
                    <ChevronDown data-icon="inline-end" className="size-3" />
                  </Button>
                </DropdownMenuTrigger>
                <DropdownMenuContent align="start">
                  <SetTypeDropdown
                    currentType={selectedType}
                    onTypeChange={(type) => form.setValue("type", type)}
                  />
                </DropdownMenuContent>
              </DropdownMenu>

              <DropdownMenu>
                <DropdownMenuTrigger asChild>
                  <Button
                    variant="ghost"
                    size="sm"
                    type="button"
                    className="h-8 gap-2 px-2 text-muted-foreground hover:text-foreground"
                  >
                    {isAgent ? (
                      <Avatar className="size-4">
                        <AvatarAgentFallback name={(assignee?.name || "").toLowerCase()} />
                      </Avatar>
                    ) : assignee ? (
                      <Avatar className="size-4">
                        {assignee.image ? (
                          <AvatarImage src={assignee.image} alt={assignee.name} />
                        ) : (
                          <AvatarFallback>{assigneeInitials(assignee.name)}</AvatarFallback>
                        )}
                      </Avatar>
                    ) : (
                      <UserIcon data-icon="inline-start" className="size-4" />
                    )}
                    <span className="max-w-28 truncate">{assigneeLabel}</span>
                    <ChevronDown data-icon="inline-end" className="size-3" />
                  </Button>
                </DropdownMenuTrigger>
                <DropdownMenuContent align="start" className="w-64">
                  <SetAssigneeDropdown
                    currentAssignee={selectedAssignee}
                    onAssigneeChange={(assignee) => form.setValue("assigned", assignee)}
                  />
                </DropdownMenuContent>
              </DropdownMenu>

              <Popover open={worktreeOpen} onOpenChange={setWorktreeOpen}>
                <PopoverTrigger asChild>
                  <Button
                    variant="ghost"
                    size="sm"
                    type="button"
                    className={cn(
                      "h-8 gap-2 px-2 hover:text-foreground",
                      worktreeValues.enabled ? "text-green-600" : "text-muted-foreground"
                    )}
                  >
                    <GitBranch data-icon="inline-start" className="size-4" />
                    <span>{t("Worktree")}</span>
                    <ChevronDown data-icon="inline-end" className="size-3" />
                  </Button>
                </PopoverTrigger>
                <PopoverContent className="w-80 p-4" align="start">
                  <div className="space-y-4">
                    <div className="flex items-center justify-between">
                      <div className="text-sm font-medium">{t("Enable Worktree")}</div>
                      <FormField
                        control={form.control}
                        name="worktree.enabled"
                        render={({ field }) => (
                          <Field>
                            <FormControl>
                              <button
                                type="button"
                                onClick={() => field.onChange(!field.value)}
                                className={cn(
                                  "relative inline-flex h-6 w-11 items-center rounded-full transition-colors",
                                  field.value ? "bg-green-600" : "bg-muted"
                                )}
                              >
                                <span
                                  className={cn(
                                    "inline-block h-4 w-4 transform rounded-full bg-white transition-transform",
                                    field.value ? "translate-x-6" : "translate-x-1"
                                  )}
                                />
                              </button>
                            </FormControl>
                          </Field>
                        )}
                      />
                    </div>

                    {worktreeValues.enabled && (
                      <>
                        <FormField
                          control={form.control}
                          name="worktree.base"
                          render={({ field }) => (
                            <Field>
                              <label className="text-xs text-muted-foreground mb-1 block">{t("Base Branch")}</label>
                              <FormControl>
                                <Input
                                  {...field}
                                  placeholder={t("e.g., develop, main")}
                                  className="h-8"
                                />
                              </FormControl>
                            </Field>
                          )}
                        />
                        <FormField
                          control={form.control}
                          name="worktree.branch"
                          render={({ field }) => (
                            <Field>
                              <label className="text-xs text-muted-foreground mb-1 block">{t("Branch Name")}</label>
                              <FormControl>
                                <Input
                                  {...field}
                                  placeholder={t("e.g., feature/my-task")}
                                  className="h-8"
                                />
                              </FormControl>
                            </Field>
                          )}
                        />
                        <div className="text-xs text-muted-foreground">
                          <Check className="inline-block size-3 mr-1" />
                          {t("Worktree will be created when task starts")}
                        </div>
                      </>
                    )}
                  </div>
                </PopoverContent>
              </Popover>
            </div>
            <div className="flex items-center gap-2">
              <Button variant="ghost" size="sm" type="button" onClick={() => onOpenChange(false)}>
                {t("Cancel")}
              </Button>
              <Button size="sm" type="submit" disabled={form.isLoading}>
                {form.isLoading ? "Creating..." : "Create"}
              </Button>
            </div>
          </div>
        </Form>
      </DialogContent>
    </Dialog>
  )
}
