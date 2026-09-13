import * as React from "react"
import type { Label as LabelPrimitive } from "radix-ui"
import { Slot } from "radix-ui"
import {
  Controller,
  FormProvider,
  useFormContext,
  useFormState,
  type ControllerProps,
  type FieldPath,
  type FieldValues,
  type UseFormReturn,
} from "react-hook-form"

import { cn } from "@/lib/utils"
import { Label } from "@/components/ui/label"
import { t } from "@/lib/i18n"

type FormProps = {
  /**
   * The object `aos.useForm` returns. Renders a <form> that disables its
   * fields while a submission is in flight (see `disableLoadingState`); a
   * submit button inside it runs `form.submit`, validation included.
   */
  form: UseFormReturn<any> & { submit?: (...args: any[]) => unknown };
  /** When true and `form` is provided, form fields are NOT automatically disabled during submission */
  disableLoadingState?: boolean;
  children?: React.ReactNode;
  className?: string;
}

/**
 * Whether a submit event is this form asking to be submitted.
 *
 * Only a control that declares `type="submit"` counts. A <button> with no
 * type is a submit button to the browser, and these forms are full of them
 * — toolbar, "add row" and tab buttons whose authors never meant them to
 * save — which went unnoticed while every submit was swallowed. A
 * submission with no submitter (Enter in a lone text field, `requestSubmit()`)
 * counts only in a form that has a submit button: a page that saves from a
 * header button has none, and Enter in its search box must not save the
 * entity next to it.
 */
function isOwnSubmission(event: React.FormEvent<HTMLFormElement>): boolean {
  // A <form> nested inside this one, or rendered in a portal whose React
  // parent is this one, reaches this handler with its own submission.
  if (event.target !== event.currentTarget) return false;

  const submitter = (event.nativeEvent as SubmitEvent).submitter;
  if (submitter) return submitter.getAttribute("type") === "submit";
  return event.currentTarget.querySelector('[type="submit"]') !== null;
}

const Form = React.forwardRef<React.ElementRef<typeof FormProvider>, FormProps>(
  ({ form, disableLoadingState = false, children, className, ...props }, _ref) => {
    return (
      <FormProvider {...form}>
        <form
          // Capture, not bubble. Blink and WebKit stop a submit event that
          // bubbles out of a nested <form> at the outer one, so a bubble
          // handler on the inner form never runs and the browser submits
          // natively: a GET that puts the field values (a bot token, once)
          // in the URL and reloads the window without its `?daemon=`
          // address. The capture phase reaches every form on the path
          // before that happens.
          onSubmitCapture={(event) => {
            event.preventDefault();
            if (!isOwnSubmission(event)) return;
            void form.submit?.();
          }}
          className={className}
          {...props}
        >
          {disableLoadingState ? (
            children
          ) : (
            <fieldset disabled={form.formState.isSubmitting} className="contents border-0 p-0 m-0 min-w-0">
              {children}
            </fieldset>
          )}
        </form>
      </FormProvider>
    );
  }
);

Form.displayName = "Form";

type FormFieldContextValue<
  TFieldValues extends FieldValues = FieldValues,
  TName extends FieldPath<TFieldValues> = FieldPath<TFieldValues>,
> = {
  name: TName
}

const FormFieldContext = React.createContext<FormFieldContextValue>(
  {} as FormFieldContextValue
)

const FormField = <
  TFieldValues extends FieldValues = FieldValues,
  TName extends FieldPath<TFieldValues> = FieldPath<TFieldValues>,
>({
  ...props
}: ControllerProps<TFieldValues, TName>) => {
  return (
    <FormFieldContext.Provider value={{ name: props.name }}>
      <Controller {...props} />
    </FormFieldContext.Provider>
  )
}

const useFormField = () => {
  const fieldContext = React.useContext(FormFieldContext)
  const itemContext = React.useContext(FormItemContext)
  const { getFieldState } = useFormContext()
  const formState = useFormState({ name: fieldContext.name })
  const fieldState = getFieldState(fieldContext.name, formState)

  if (!fieldContext) {
    throw new Error("useFormField should be used within <FormField>")
  }

  const { id } = itemContext

  return {
    id,
    name: fieldContext.name,
    formItemId: `${id}-form-item`,
    formDescriptionId: `${id}-form-item-description`,
    formMessageId: `${id}-form-item-message`,
    ...fieldState,
  }
}

type FormItemContextValue = {
  id: string
}

const FormItemContext = React.createContext<FormItemContextValue>(
  {} as FormItemContextValue
)

function FormItem({ className, ...props }: React.ComponentProps<"div">) {
  const id = React.useId()

  return (
    <FormItemContext.Provider value={{ id }}>
      <div
        data-slot="form-item"
        className={cn("grid gap-2", className)}
        {...props}
      />
    </FormItemContext.Provider>
  )
}

function FormLabel({
  className,
  ...props
}: React.ComponentProps<typeof LabelPrimitive.Root>) {
  const { error, formItemId } = useFormField()

  return (
    <Label
      data-slot="form-label"
      data-error={!!error}
      className={cn("data-[error=true]:text-destructive text-xs", className)}
      htmlFor={formItemId}
      {...props}
    />
  )
}

function FormControl({ ...props }: React.ComponentProps<typeof Slot.Root>) {
  const { error, formItemId, formDescriptionId, formMessageId } = useFormField()

  return (
    <Slot.Root
      data-slot="form-control"
      id={formItemId}
      aria-describedby={
        !error
          ? `${formDescriptionId}`
          : `${formDescriptionId} ${formMessageId}`
      }
      aria-invalid={!!error}
      {...props}
    />
  )
}

function FormDescription({ className, ...props }: React.ComponentProps<"p">) {
  const { formDescriptionId } = useFormField()

  return (
    <p
      data-slot="form-description"
      id={formDescriptionId}
      className={cn("text-sm text-muted-foreground", className)}
      {...props}
    />
  )
}

function FormMessage({ className, ...props }: React.ComponentProps<"p">) {
  const { error, formMessageId } = useFormField()
  // A validation message is written in English in a zod schema, far from
  // any t(). Translating it here is what lets a catalogued message reach a
  // Portuguese screen; an uncatalogued one falls back to itself.
  const body = error ? t(String(error?.message ?? "")) : props.children

  if (!body) {
    return null
  }

  return (
    <p
      data-slot="form-message"
      id={formMessageId}
      className={cn("text-sm text-destructive", className)}
      {...props}
    >
      {body}
    </p>
  )
}

function FieldGroup({ className, ...props }: React.ComponentProps<"div">) {
  return (
    <div
      data-slot="field-group"
      className={cn("flex flex-col gap-4", className)}
      {...props}
    />
  )
}

function Field({ className, ...props }: React.ComponentProps<"div">) {
  return (
    <div
      data-slot="field"
      className={cn("grid gap-2", className)}
      {...props}
    />
  )
}

const FieldLabel = FormLabel
const FieldDescription = FormDescription
const FieldMessage = FormMessage

export {
  useFormField,
  Form,
  FormItem,
  FormLabel,
  FormControl,
  FormDescription,
  FormMessage,
  FormField,
  FieldGroup,
  Field,
  FieldLabel,
  FieldDescription,
  FieldMessage,
}
