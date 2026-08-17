import * as React from 'react'
import { cn } from '@/lib/utils'

interface FormSectionProps extends React.ComponentPropsWithRef<'div'> { }

const FormSection = ({ className, ref, ...props }: FormSectionProps) => (
  <div
    ref={ref}
    className={cn('flex flex-col', className)}
    {...props}
  />
)

const FormSectionHeader = ({
  className,
  ref,
  ...props
}: React.ComponentPropsWithRef<'div'>) => (
  <div ref={ref} className={cn('px-1 mb-4', className)} {...props} />
)

const FormSectionTitle = ({
  className,
  ref,
  ...props
}: React.ComponentPropsWithRef<'h3'>) => (
  <h3
    ref={ref}
    className={cn('text-sm font-medium leading-6 text-foreground', className)}
    {...props}
  />
)

const FormSectionDescription = ({
  className,
  ref,
  ...props
}: React.ComponentPropsWithRef<'p'>) => (
  <p
    ref={ref}
    className={cn('text-sm text-muted-foreground', className)}
    {...props}
  />
)

const FormSectionContent = ({
  className,
  ref,
  ...props
}: React.ComponentPropsWithRef<'div'>) => (
  <div
    ref={ref}
    className={cn(
      'rounded-xl border border-border bg-secondary/50 text-card-foreground overflow-hidden divide-y divide-border',
      className
    )}
    {...props}
  />
)

const FormSectionItem = ({
  className,
  ref,
  ...props
}: React.ComponentPropsWithRef<'div'>) => (
  <div
    ref={ref}
    className={cn(
      'flex flex-row items-center justify-between min-h-16 p-4 gap-4',
      className
    )}
    {...props}
  />
)

const FormSectionFooter = ({
  className,
  ref,
  ...props
}: React.ComponentPropsWithRef<'div'>) => (
  <div
    ref={ref}
    className={cn('flex items-center p-6', className)}
    {...props}
  />
)

export {
  FormSection,
  FormSectionHeader,
  FormSectionTitle,
  FormSectionDescription,
  FormSectionContent,
  FormSectionItem,
  FormSectionFooter
}
