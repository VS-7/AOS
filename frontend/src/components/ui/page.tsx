'use client'

import { motion, MotionProps } from 'motion/react'
import * as React from 'react'
import { cn } from '@/lib/utils'
import { isDesktop } from '@/lib/client'

// Variantes de animação
const pageVariants = {
  hidden: { opacity: 0 },
  visible: {
    opacity: 1,
    transition: {
      duration: 0.0
    }
  }
}

const headerVariants = {
  hidden: { opacity: 0, y: -20 },
  visible: {
    opacity: 1,
    y: 0,
    transition: {
      duration: 0.0
    }
  }
}

const bodyVariants = {
  hidden: { opacity: 0, y: 20 },
  visible: {
    opacity: 1,
    y: 0,
    transition: {
      duration: 0.015,
    }
  }
}

interface PageContextValue {
  className?: string
  isNative: boolean
}

const PageContext = React.createContext<PageContextValue | null>(null)

export function usePage() {
  const context = React.useContext(PageContext)
  if (!context) {
    throw new Error('usePage must be used within a <Page />')
  }
  return context
}

type MotionDivProps = React.ComponentPropsWithRef<'div'> & MotionProps

export function Page({
  className,
  children,
  ref,
  ...props
}: MotionDivProps & { showInboxPanel?: boolean }) {
  const isNative = isDesktop()

  return (
    <PageContext.Provider value={{ isNative }}>
      <motion.div
        ref={ref}
        variants={pageVariants}
        initial='hidden'
        animate='visible'
        className={cn(
          'flex w-full flex-col max-h-[calc(100vh-3.5rem)] h-screen relative overflow-hidden page',
          className
        )}
        {...props}
      >
        {children}
      </motion.div>
    </PageContext.Provider>
  )
}

type PageHeaderProps = MotionDivProps & {
  hideChatToggle?: boolean
}

export function PageHeader({
  className,
  children,
  ref,
  ...props
}: PageHeaderProps) {
  const { isNative } = usePage()

  return (
    <motion.div
      ref={ref}
      variants={headerVariants}
      initial='hidden'
      animate='visible'
      className={cn(
        'flex h-12 px-6 items-center gap-4 sticky top-0 z-20 border-b',
        className
      )}
      {...props}
    >
      <div className={cn(
        'flex items-center gap-4 flex-1 min-w-0 [&>div]:flex [&>div]:items-center [*]:flex [*]:items-center justify-between',
        isNative && '[-webkit-app-region:no-drag]'
      )}>
        {children}
      </div>
    </motion.div>
  )
}

export function PageMainBar({
  className,
  ref,
  ...props
}: React.ComponentPropsWithRef<'div'>) {
  return (
    <div
      ref={ref}
      className={cn('flex w-full items-center gap-2', className)}
      {...props}
    />
  )
}

export function PageActionsBar({
  className,
  ref,
  ...props
}: React.ComponentPropsWithRef<'div'>) {
  return (
    <div
      ref={ref}
      className={cn('flex items-center gap-2', className)}
      {...props}
    />
  )
}

export function PageSecondaryHeader({
  className,
  ref,
  ...props
}: MotionDivProps) {
  return (
    <motion.div
      ref={ref}
      variants={headerVariants}
      initial='hidden'
      animate='visible'
      className={cn('flex items-center gap-2 px-2 py-2', className)}
      {...props}
    />
  )
}

export function PageBody({
  className,
  ref,
  ...props
}: MotionDivProps) {
  return (
    <motion.div
      ref={ref}
      variants={bodyVariants}
      initial='hidden'
      animate='visible'
      className={cn(
        'flex-1 h-full flex flex-col grow overflow-y-auto scroll-fade overflow-y-auto scroll-fade-14',
        className
      )}
      {...props}
    />
  )
}

export function PageActions({
  className,
  ref,
  ...props
}: MotionDivProps) {
  return (
    <motion.div
      ref={ref}
      initial={{ opacity: 0, y: 20 }}
      animate={{
        opacity: 1,
        y: 0,
        transition: {
          duration: 0.4,
          delay: 0.3
        }
      }}
      className={cn(
        'flex h-14 items-center justify-end gap-4',
        className
      )}
      {...props}
    />
  )
}
