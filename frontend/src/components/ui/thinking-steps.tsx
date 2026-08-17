"use client";

import {
  forwardRef,
  type ReactNode,
  type HTMLAttributes,
  type ComponentProps,
  type CSSProperties,
} from "react";
import { motion } from "framer-motion";
import { XCircleIcon } from "lucide-react";
import { cn } from "@/lib/utils";
import { useIcon } from "@/lib/icon-context";
import type { IconName } from "@/lib/icon-context";
import { springs } from "@/lib/springs";
import { fontWeights } from "@/lib/font-weight";
import { useShape } from "@/lib/shape-context";
import { Shimmer } from "@/components/ui/shimmer";
import {
  Accordion,
  AccordionItem,
  AccordionTrigger,
  AccordionContent,
} from "@/components/ui/accordion";
import { Badge } from "@/components/ui/badge";
import type { BadgeColor } from "@/components/ui/badge";

// ─── ThinkingSteps (root) ───────────────────────────────────────────────────

type MotionDivProps = ComponentProps<typeof motion.div>;

interface ThinkingStepsProps extends HTMLAttributes<HTMLDivElement> {
  defaultOpen?: boolean;
  open?: boolean;
  onOpenChange?: (open: boolean) => void;
  children: ReactNode;
}

const ThinkingSteps = forwardRef<HTMLDivElement, ThinkingStepsProps>(
  // eslint-disable-next-line @typescript-eslint/no-unused-vars
  (
    {
      defaultOpen = true,
      open,
      onOpenChange,
      children,
      className,
      defaultValue: _,
      ...props
    },
    ref,
  ) => {
    const controlled = open !== undefined;
    return (
      <div ref={ref} className={cn("w-80 max-w-full", className)} {...props}>
        <motion.div
          layout="position"
          initial={{ opacity: 0, y: -1 }}
          animate={{ opacity: 1, y: 0 }}
          transition={{ duration: 0.22, ease: [0.22, 1, 0.36, 1] }}
          className="w-full max-w-full"
        >
          <Accordion
            type="single"
            collapsible
            {...(controlled
              ? { value: open ? "thinking" : "" }
              : { defaultValue: defaultOpen ? "thinking" : "" })}
            onValueChange={
              onOpenChange
                ? (v: string) => onOpenChange(v === "thinking")
                : undefined
            }
            className="w-full max-w-full"
          >
            {/* Hide standalone accordion expanded bg */}
            <AccordionItem value="thinking" className="[&>.absolute]:hidden">
              {children}
            </AccordionItem>
          </Accordion>
        </motion.div>
      </div>
    );
  },
);
ThinkingSteps.displayName = "ThinkingSteps";

// ─── ThinkingStepsHeader ────────────────────────────────────────────────────

interface ThinkingStepsHeaderProps extends HTMLAttributes<HTMLButtonElement> {
  children?: ReactNode;
}

const ThinkingStepsHeader = forwardRef<
  HTMLButtonElement,
  ThinkingStepsHeaderProps
>(({ children = "Thinking", className, ...props }, ref) => {
  return (
    <div className="w-fit">
      <AccordionTrigger
        ref={ref}
        className={cn(
          "[&>span:first-child]:flex-none w-auto transition-colors duration-500",
          className,
        )}
        {...props}
      >
        {children}
      </AccordionTrigger>
    </div>
  );
});
ThinkingStepsHeader.displayName = "ThinkingStepsHeader";

// ─── ThinkingStepsContent ───────────────────────────────────────────────────

interface ThinkingStepsContentProps extends HTMLAttributes<HTMLDivElement> {
  children: ReactNode;
}

const ThinkingStepsContent = forwardRef<
  HTMLDivElement,
  ThinkingStepsContentProps
>(({ children, className, ...props }, ref) => {
  return (
    <AccordionContent>
      <motion.div
        ref={ref}
        layout="position"
        initial={{ opacity: 0, y: -2 }}
        animate={{ opacity: 1, y: 0 }}
        exit={{ opacity: 0, y: -2 }}
        transition={{ duration: 0.24, ease: [0.22, 1, 0.36, 1] }}
        className={cn("flex flex-col", className)}
        {...(props as MotionDivProps)}
      >
        {children}
      </motion.div>
    </AccordionContent>
  );
});
ThinkingStepsContent.displayName = "ThinkingStepsContent";

// ─── ThinkingStep ───────────────────────────────────────────────────────────

type StepStatus = "complete" | "active" | "pending" | "error";

interface ThinkingStepProps {
  icon?: IconName;
  showIcon?: boolean;
  label: string;
  description?: string;
  descriptionFade?: boolean;
  trailing?: ReactNode;
  trailingContent?: ReactNode;
  status?: StepStatus;
  index: number;
  delay?: number;
  isLast?: boolean;
  children?: ReactNode;
  className?: string;
}

function ThinkingStep({
  icon = "dot",
  showIcon = true,
  label,
  description,
  descriptionFade = false,
  trailing,
  trailingContent,
  status = "complete",
  index,
  delay = 0,
  isLast = false,
  children,
  className,
}: ThinkingStepProps) {
  const Icon = useIcon(icon);
  const shape = useShape();

  if (status === "pending") return null;

  const isActive = status === "active";
  const isError = status === "error";

  return (
    <motion.div
      layout="position"
      className={cn("relative z-10 overflow-hidden", className)}
      initial={{ height: 0, opacity: 0, y: -2 }}
      animate={{ height: "auto", opacity: 1, y: 0 }}
      exit={{ height: 0, opacity: 0, y: -2 }}
      transition={{
        height: { ...springs.slow, duration: 0.28, bounce: 0.08 },
        opacity: {
          duration: 0.22,
          delay: Math.min(index * 0.028 + delay, 0.14),
          ease: [0.22, 1, 0.36, 1],
        },
        y: {
          duration: 0.22,
          delay: Math.min(index * 0.028 + delay, 0.14),
          ease: [0.22, 1, 0.36, 1],
        },
      }}
    >
      <motion.div
        layout="position"
        animate={{
          backgroundColor: isActive
            ? "color-mix(in srgb, var(--accent) 18%, transparent)"
            : isError
              ? "color-mix(in srgb, var(--destructive) 8%, transparent)"
              : "transparent",
        }}
        transition={{ duration: 0.32, ease: [0.22, 1, 0.36, 1] }}
        className={cn("flex gap-2.5 px-2 py-1.5", shape.item)}
      >
        <div className="flex flex-col items-center shrink-0 w-[14px]">
          <motion.div
            className="pt-0.5"
            animate={isActive ? { scale: [1, 1.03, 1] } : { scale: 1 }}
            transition={{
              duration: 2.2,
              repeat: isActive ? Number.POSITIVE_INFINITY : 0,
              ease: [0.42, 0, 0.58, 1],
            }}
          >
            {showIcon ? (
              <Icon
                size={14}
                strokeWidth={1.5}
                className={cn(
                  "transition-colors duration-500",
                  isError
                    ? "text-destructive"
                    : isActive
                      ? "text-foreground"
                      : "text-muted-foreground",
                )}
              />
            ) : (
              <div className="w-[14px] h-[14px] flex items-center justify-center">
                <motion.div
                  className={cn(
                    "w-1.5 h-1.5 rounded-md",
                    isError
                      ? "bg-destructive"
                      : isActive
                        ? "bg-foreground"
                        : "bg-muted-foreground/60",
                  )}
                  animate={
                    isActive ? { opacity: [0.6, 1, 0.6] } : { opacity: 1 }
                  }
                  transition={{
                    duration: 1.8,
                    repeat: isActive ? Number.POSITIVE_INFINITY : 0,
                    ease: [0.42, 0, 0.58, 1],
                  }}
                />
              </div>
            )}
          </motion.div>
          {!isLast && (
            <motion.div
              className={cn(
                "flex-1 w-px mt-1",
                isActive ? "bg-border" : "bg-border/60",
              )}
              initial={{ scaleY: 0, transformOrigin: "top" }}
              animate={{ scaleY: 1 }}
              transition={{
                duration: 0.26,
                ease: [0.22, 1, 0.36, 1],
                delay: 0.05,
              }}
            />
          )}
        </div>

        <div className="flex-1 flex min-w-0 flex-col gap-1">
          <div className="flex min-w-0 items-center gap-2">
            <motion.span
              initial={false}
              animate={{ opacity: 1 }}
              transition={{ duration: 0.16, ease: [0.22, 1, 0.36, 1] }}
              className="shrink-0 text-[13px] leading-tight text-foreground"
              style={{ fontVariationSettings: fontWeights.medium }}
            >
              {isActive ? (
                <Shimmer as="span" className="text-foreground">
                  {`${label}...`}
                </Shimmer>
              ) : (
                label
              )}
            </motion.span>
            {description ? (
              <motion.div
                initial={false}
                animate={{ opacity: 1 }}
                transition={{ duration: 0.16, ease: [0.22, 1, 0.36, 1] }}
                className="relative min-w-0 flex-1"
              >
                <span className="line-clamp-1 block text-[13px] leading-snug text-muted-foreground">
                  {description}
                </span>
                {descriptionFade ? (
                  <span
                    aria-hidden="true"
                    className="pointer-events-none absolute inset-y-0 right-0 w-12"
                    style={
                      {
                        background:
                          "linear-gradient(to right, transparent, var(--background) 72%)",
                      } satisfies CSSProperties
                    }
                  />
                ) : null}
              </motion.div>
            ) : null}
            {isError ? (
              <motion.span
                initial={false}
                animate={{ opacity: 1 }}
                transition={{ duration: 0.16, ease: [0.22, 1, 0.36, 1] }}
                className="flex shrink-0 items-center gap-1 text-destructive/90"
                title="Error"
              >
                <XCircleIcon className="size-3.5" />
              </motion.span>
            ) : null}
            {trailing ? (
              <motion.div
                initial={false}
                animate={{ opacity: 1 }}
                transition={{ duration: 0.16, ease: [0.22, 1, 0.36, 1] }}
                className="ml-auto shrink-0"
              >
                {trailing}
              </motion.div>
            ) : null}
          </div>
          {trailingContent ? (
            <div className="min-w-0">{trailingContent}</div>
          ) : null}
          {children}
        </div>
      </motion.div>
    </motion.div>
  );
}

// ─── ThinkingStepDetails (nested accordion) ────────────────────────────────

interface ThinkingStepDetailsProps {
  summary: string;
  details?: string[];
  defaultOpen?: boolean;
  children?: ReactNode;
  className?: string;
  mode?: "full" | "trigger" | "content";
}

function ThinkingStepDetails({
  summary,
  details,
  defaultOpen = false,
  children,
  className,
  mode = "full",
}: ThinkingStepDetailsProps) {
  const shape = useShape();

  const trigger = (
    <div className="w-fit">
      <AccordionTrigger
        className={cn(
          "[&>span:first-child]:flex-none h-auto w-auto gap-1 py-0.5 pl-2 pr-1.5 text-[12px] text-muted-foreground/85 transition-colors duration-200 hover:text-foreground",
          shape.item,
          className,
        )}
      >
        {summary}
      </AccordionTrigger>
    </div>
  );

  const content = (
    <AccordionContent className={cn(className)}>
      <div className="flex flex-col gap-0.5 pt-0.5">
        {details?.map((item, i) => (
          <span
            key={i}
            className="text-[12px] text-muted-foreground leading-snug"
          >
            {item}
          </span>
        ))}
        {children}
      </div>
    </AccordionContent>
  );

  if (mode === "trigger") {
    return trigger;
  }

  if (mode === "content") {
    return content;
  }

  return (
    <Accordion
      type="single"
      collapsible
      defaultValue={defaultOpen ? "details" : ""}
      className="w-fit max-w-none"
    >
      <AccordionItem value="details" className="[&>.absolute]:hidden">
        {trigger}
        {content}
      </AccordionItem>
    </Accordion>
  );
}

// ─── ThinkingStepSources ────────────────────────────────────────────────────

interface ThinkingStepSourcesProps extends HTMLAttributes<HTMLDivElement> {
  children: ReactNode;
}

const ThinkingStepSources = forwardRef<
  HTMLDivElement,
  ThinkingStepSourcesProps
>(({ children, className, ...props }, ref) => {
  return (
    <div
      ref={ref}
      className={cn("flex flex-wrap gap-1.5 mt-1", className)}
      {...props}
    >
      {children}
    </div>
  );
});
ThinkingStepSources.displayName = "ThinkingStepSources";

// ─── ThinkingStepSource ─────────────────────────────────────────────────────

interface ThinkingStepSourceProps {
  color?: BadgeColor;
  delay?: number;
  children: ReactNode;
  className?: string;
}

function ThinkingStepSource({
  color = "gray",
  delay = 0,
  children,
  className,
}: ThinkingStepSourceProps) {
  return (
    <motion.span
      initial={{ opacity: 0, scale: 0.85, filter: "blur(4px)" }}
      animate={{ opacity: 1, scale: 1, filter: "blur(0px)" }}
      transition={{
        ...springs.moderate,
        delay,
        filter: { duration: 0.12, delay },
      }}
    >
      <Badge variant="default" size="sm" color={color} className={className}>
        {children}
      </Badge>
    </motion.span>
  );
}
ThinkingStepSource.displayName = "ThinkingStepSource";

// ─── ThinkingStepImage ──────────────────────────────────────────────────────

interface ThinkingStepImageProps {
  src: string;
  alt?: string;
  caption?: string;
  delay?: number;
  className?: string;
}

function ThinkingStepImage({
  src,
  alt = "",
  caption,
  delay = 0,
  className,
}: ThinkingStepImageProps) {
  const shape = useShape();
  return (
    <motion.div
      className={cn("mt-1.5", className)}
      initial={{ opacity: 0, filter: "blur(4px)" }}
      animate={{ opacity: 1, filter: "blur(0px)" }}
      transition={{
        opacity: { duration: 0.2, delay, ease: "easeOut" },
        filter: { duration: 0.15, delay },
      }}
    >
      <img
        src={src}
        alt={alt}
        className={cn("w-full max-w-[200px] object-cover", shape.container)}
      />
      {caption && (
        <span className="text-[11px] text-muted-foreground mt-1 block">
          {caption}
        </span>
      )}
    </motion.div>
  );
}
ThinkingStepImage.displayName = "ThinkingStepImage";

// ─── Exports ────────────────────────────────────────────────────────────────

export {
  ThinkingSteps,
  ThinkingStepsHeader,
  ThinkingStepsContent,
  ThinkingStep,
  ThinkingStepDetails,
  ThinkingStepSources,
  ThinkingStepSource,
  ThinkingStepImage,
};

export type {
  ThinkingStepsProps,
  ThinkingStepsHeaderProps,
  ThinkingStepsContentProps,
  ThinkingStepProps,
  ThinkingStepDetailsProps,
  ThinkingStepSourcesProps,
  ThinkingStepSourceProps,
  ThinkingStepImageProps,
  StepStatus,
};
