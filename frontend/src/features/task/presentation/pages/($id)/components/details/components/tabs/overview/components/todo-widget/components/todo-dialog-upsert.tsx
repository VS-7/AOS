import { t } from "@/lib/i18n";
import * as React from "react"
import { z } from "zod"

import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogFooter,
  DialogTrigger,
} from "@/components/ui/dialog"
import {
  FormControl,
  FormField,
  Field,
  FieldGroup,
  Form,
} from "@/components/ui/form"
import { Input } from "@/components/ui/input"
import { Textarea } from "@/components/ui/textarea"
import { Button } from "@/components/ui/button"
import { Label } from "@/components/ui/label"
import { aos } from "@/app/aos"
import type { Todo } from "@/features/task/interfaces/todo.interfaces"

const formSchema = z.object({
  description: z.string().min(1, "Description is required"),
  agent: z.string().optional(),
  instructions: z.string().optional(),
})

interface TodoDialogUpsertProps {
  taskId: string
  todo?: Todo
  onCreated?: () => void
  children?: React.ReactNode
}

export function TodoDialogUpsert({ taskId, todo, onCreated, children }: TodoDialogUpsertProps) {
  const ref = React.useRef<HTMLDivElement | null>(null)

  const isEdit = !!todo

  // Reads `todo.title`/`todo.content` — Go's real `Todo` field names (see
  // `interfaces/task.interfaces.ts`'s `TaskTodoSchema` doc comment)
  // — while keeping this form's own field names (`description`/
  // `instructions`) unchanged; the outgoing rename back to `title`/
  // `content` happens in `command-map.ts`'s `todo.create`/`todo.update`
  // entries, not here.
  const initialValues = React.useMemo(() => ({
    description: todo?.title || "",
    agent: todo?.agent || "",
    instructions: todo?.content || "",
  }), [todo])

  const form = aos.useForm({
    schema: formSchema,
    mutation: isEdit ? "todo.update" : "todo.create",
    values: initialValues,
    onSubmit: (values) => {
      const body: Record<string, unknown> = {
        description: values.description,
      }
      if (values.agent) body.agent = values.agent
      if (values.instructions) body.instructions = values.instructions

      if (isEdit) {
        return {
          params: { taskId, id: todo!.id },
          body,
        }
      }

      return {
        params: { taskId },
        body,
      }
    },
    onResponse: ({ error }) => {
      if (!error) {
        ref.current?.click()
        form.reset()
        onCreated?.()
      }
    },
  })

  return (
    <Dialog>
      <DialogTrigger asChild>
        <div ref={ref}>
          {children}
        </div>
      </DialogTrigger>
      <DialogContent className="sm:max-w-lg">
        <DialogHeader>
          <DialogTitle>{isEdit ? "Edit Todo" : "Create Todo"}</DialogTitle>
        </DialogHeader>
        <Form form={form}>
          <FieldGroup>
            <FormField
              control={form.control}
              name="description"
              render={({ field }) => (
                <Field>
                  <Label htmlFor="description">{t("Description")}</Label>
                  <FormControl>
                    <Textarea
                      {...field}
                      id="description"
                      placeholder={t("What needs to be done?")}
                      className="min-h-20 resize-none"
                    />
                  </FormControl>
                </Field>
              )}
            />
            <FormField
              control={form.control}
              name="instructions"
              render={({ field }) => (
                <Field>
                  <Label htmlFor="instructions">{t("Instructions (optional)")}</Label>
                  <FormControl>
                    <Textarea
                      {...field}
                      id="instructions"
                      placeholder={t("Instructions for the agent...")}
                      className="min-h-20 resize-none"
                    />
                  </FormControl>
                </Field>
              )}
            />
            {/*
              C5 of the final review ("honest empty state" policy, R26):
              `agent` is in this form's own schema and gets read from
              `todo?.agent` on edit, but there was no control for it at all
              — a user could never see or set it. There is also no Go field
              to hold it (`command-map.ts`'s `todo.create`/`.update` entries:
              "no Go equivalent for `agent` at all... sent and silently
              ignored"). Two silent gaps stacked; this makes both visible
              instead of pretending the field doesn't exist.
            */}
            <Field>
              <Label htmlFor="agent">{t("Agent")}</Label>
              <FormControl>
                <Input id="agent" value={form.watch("agent") ?? ""} disabled placeholder={t("Not assignable yet")} />
              </FormControl>
              <p className="text-xs text-muted-foreground">
                {t("Assigning a todo to a specific agent isn't saved by the backend yet.")}
              </p>
            </Field>
          </FieldGroup>
          <DialogFooter className="mt-6">
            <Button variant="outline" type="button" onClick={() => {
              form.reset()
              ref.current?.click()
            }}>
              {t("Cancel")}
            </Button>
            <Button type="submit" disabled={form.isLoading}>
              {form.isLoading ? "Saving..." : isEdit ? "Update" : "Create"}
            </Button>
          </DialogFooter>
        </Form>
      </DialogContent>
    </Dialog>
  )
}
