'use client'

import { type HTMLMotionProps, motion } from 'motion/react'
import * as React from 'react'
import { Badge } from './badge'
import { cn } from '@/lib/utils'
import { Button, type ButtonProps } from './button'

// Interfaces base para cada componente
type AnimatedEmptyStateRootProps = HTMLMotionProps<'div'>

type AnimatedEmptyStateCarouselProps = HTMLMotionProps<'div'>

type AnimatedCardProps = HTMLMotionProps<'div'>

type AnimatedEmptyStatePillProps = Omit<HTMLMotionProps<'div'>, 'variant'> & {
  variant?: 'default' | 'secondary' | 'outline' | 'destructive'
}

type AnimatedEmptyStateTitleProps = HTMLMotionProps<'span'>

type AnimatedEmptyStateDescriptionProps = HTMLMotionProps<'p'>

type AnimatedEmptyStateActionsProps = HTMLMotionProps<'div'>

type AnimatedEmptyStateActionProps = ButtonProps

// Componente principal - AnimatedEmptyState
const AnimatedEmptyStateRoot = React.forwardRef<
  HTMLDivElement,
  AnimatedEmptyStateRootProps
>(({ className, children, initial, animate, ...props }, ref) => {
  return (
    <motion.div
      ref={ref}
      className={cn(
        'flex flex-col items-center justify-center gap-6 rounded-md border border-border px-4 py-10 md:min-h-[350px]',
        className
      )}
      initial={initial || { opacity: 0 }}
      animate={
        animate || {
          opacity: 1,
          transition: { staggerChildren: 0.2, delayChildren: 0.1 }
        }
      }
      {...props}
    >
      {children}
    </motion.div>
  )
})
AnimatedEmptyStateRoot.displayName = 'AnimatedEmptyState'

// Componente para o carrossel de cards animados
const AnimatedEmptyStateCarousel = React.forwardRef<
  HTMLDivElement,
  AnimatedEmptyStateCarouselProps
>(({ className, children, initial, animate, transition, ...props }, ref) => {
  return (
    <motion.div
      ref={ref}
      className={cn(
        'mask-[linear-gradient(transparent,black_10%,black_90%,transparent)] h-36 w-full max-w-64 overflow-hidden px-4',
        className
      )}
      initial={initial || { opacity: 0, y: 20 }}
      animate={animate || { opacity: 1, y: 0 }}
      transition={transition || { duration: 0.5 }}
      {...props}
    >
      <motion.div
        className='flex flex-col'
        animate={{ y: ['-0%', '-50%'] }}
        transition={{
          repeat: Infinity,
          repeatType: 'loop',
          duration: 15,
          ease: 'linear'
        }}
      >
        {[...Array(6)].map((_, idx) => (
          <AnimatedCard key={idx}>
            {React.isValidElement(children)
              ? children
              : typeof children === 'function'
                ? React.createElement(children as any, { index: idx % 3 })
                : children}
          </AnimatedCard>
        ))}
      </motion.div>
    </motion.div>
  )
})
AnimatedEmptyStateCarousel.displayName = 'AnimatedEmptyStateCarousel'

// Componente para card animado
const AnimatedCard = React.forwardRef<HTMLDivElement, AnimatedCardProps>(
  (
    { children, className, initial, animate, whileHover, transition, ...props },
    ref
  ) => {
    return (
      <motion.div
        ref={ref}
        className={cn(
          'mt-4 flex items-center gap-3 rounded-md border border-border bg-background p-4 shadow-[0_4px_12px_0_#0000000D]',
          className
        )}
        initial={initial || { opacity: 0, y: 20 }}
        animate={animate || { opacity: 1, y: 0 }}
        whileHover={
          whileHover || {
            scale: 1.03,
            y: -5,
            boxShadow: '0 10px 20px rgba(0,0,0,0.1)'
          }
        }
        transition={
          transition || { type: 'spring', stiffness: 400, damping: 20 }
        }
        {...props}
      >
        {children}
      </motion.div>
    )
  }
)
AnimatedCard.displayName = 'AnimatedCard'

// Componente para badge/pill
const AnimatedEmptyStatePill: React.FC<AnimatedEmptyStatePillProps> = ({
  children,
  className,
  variant = 'default',
  initial,
  animate,
  transition,
  whileHover
}) => {
  return (
    <motion.div
      initial={initial || { opacity: 0, y: 20 }}
      animate={animate || { opacity: 1, y: 0 }}
      transition={transition || { duration: 0.5 }}
      whileHover={whileHover || { scale: 1.05 }}
      style={{ display: 'inline-block' }}
    >
      <Badge variant={variant} className={cn(className)}>
        {/* @ts-expect-error - Badge props */}
        {children}
      </Badge>
    </motion.div>
  )
}
AnimatedEmptyStatePill.displayName = 'AnimatedEmptyStatePill'

// Componente para o content wrapper (título e descrição)
const AnimatedEmptyStateContent = React.forwardRef<
  HTMLDivElement,
  HTMLMotionProps<'div'>
>(({ className, children, initial, animate, transition, ...props }, ref) => {
  return (
    <motion.div
      ref={ref}
      className={cn('max-w-xs space-y-4! text-pretty text-center', className)}
      initial={initial || { opacity: 0, y: 20 }}
      animate={animate || { opacity: 1, y: 0 }}
      transition={transition || { duration: 0.5 }}
      {...props}
    >
      {children}
    </motion.div>
  )
})
AnimatedEmptyStateContent.displayName = 'AnimatedEmptyStateContent'

// Componente para o título
const AnimatedEmptyStateTitle = React.forwardRef<
  HTMLSpanElement,
  AnimatedEmptyStateTitleProps
>(({ className, children, initial, animate, transition, ...props }, ref) => {
  return (
    <motion.span
      ref={ref}
      className={cn('text-sm font-medium mb-2', className)}
      initial={initial || { opacity: 0, y: 20 }}
      animate={animate || { opacity: 1, y: 0 }}
      transition={transition || { duration: 0.5 }}
      {...props}
    >
      {children}
    </motion.span>
  )
})
AnimatedEmptyStateTitle.displayName = 'AnimatedEmptyStateTitle'

// Componente para a descrição
const AnimatedEmptyStateDescription = React.forwardRef<
  HTMLParagraphElement,
  AnimatedEmptyStateDescriptionProps
>(({ className, children, initial, animate, transition, ...props }, ref) => {
  return (
    <motion.p
      ref={ref}
      className={cn('text-pretty text-muted-foreground text-sm', className)}
      initial={initial || { opacity: 0, y: 20 }}
      animate={animate || { opacity: 1, y: 0 }}
      transition={transition || { duration: 0.5 }}
      {...props}
    >
      {children}
    </motion.p>
  )
})
AnimatedEmptyStateDescription.displayName = 'AnimatedEmptyStateDescription'

// Componente para o container de ações
const AnimatedEmptyStateActions = React.forwardRef<
  HTMLDivElement,
  AnimatedEmptyStateActionsProps
>(({ className, children, initial, animate, transition, ...props }, ref) => {
  return (
    <motion.div
      ref={ref}
      className={cn('flex items-center gap-2', className)}
      initial={initial || { opacity: 0, y: 20 }}
      animate={animate || { opacity: 1, y: 0 }}
      transition={transition || { duration: 0.5 }}
      {...props}
    >
      {children}
    </motion.div>
  )
})
AnimatedEmptyStateActions.displayName = 'AnimatedEmptyStateActions'

// Componente para o link "Learn more"
const AnimatedEmptyStateAction = React.forwardRef<
  HTMLButtonElement,
  AnimatedEmptyStateActionProps
>(({ children, ...props }, ref) => {
  return (
    <motion.div whileHover={{ scale: 1.05 }} whileTap={{ scale: 0.95 }}>
      <Button ref={ref} {...props}>
        {children}
      </Button>
    </motion.div>
  )
})
AnimatedEmptyStateAction.displayName = 'AnimatedEmptyStateAction'

// Componente composto
const AnimatedEmptyState = Object.assign(AnimatedEmptyStateRoot, {
  Carousel: AnimatedEmptyStateCarousel,
  Card: AnimatedCard,
  Pill: AnimatedEmptyStatePill,
  Content: AnimatedEmptyStateContent,
  Title: AnimatedEmptyStateTitle,
  Description: AnimatedEmptyStateDescription,
  Actions: AnimatedEmptyStateActions,
  Action: AnimatedEmptyStateAction
})

export {
  AnimatedEmptyState,
  AnimatedCard,
  AnimatedEmptyStateRoot,
  AnimatedEmptyStateCarousel,
  AnimatedEmptyStatePill,
  AnimatedEmptyStateContent,
  AnimatedEmptyStateTitle,
  AnimatedEmptyStateDescription,
  AnimatedEmptyStateActions,
  AnimatedEmptyStateAction
}
