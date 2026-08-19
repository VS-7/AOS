import React from "react";
import { motion, AnimatePresence } from "framer-motion";
// `ResizableHandle`/`ResizablePanel`/`ResizablePanelGroup` were imported
// but never referenced anywhere in this file (same dead-import pattern as
// `IgniterResponse` and `project.helper.ts`'s `FractalProject`). Dropped
// rather than adding a `resizable.tsx` wrapper (and the `react-resizable-
// panels` dependency it needs) for code that never runs — `components/ui/
// resizable.tsx` does not exist anywhere in the extraction this was ported
// from, so there was nothing to copy even if it had been used.
import {
  Accordion,
  AccordionContent,
  AccordionItem,
  AccordionTrigger,
} from "@/components/ui/accordion";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { ScrollArea } from "@/components/ui/scroll-area";
import { ArrowLeft, ListFilter, Search } from "lucide-react";
import { cn } from "@/lib/utils";
import type { LucideIcon } from "lucide-react";
import { aos } from "@/app/aos";
import { useIsMobile } from "@/hooks/use-mobile";

// ─── Interfaces ───────────────────────────────────────────────────────────────

interface SplitPageLayoutProps {
  children: React.ReactNode;
  className?: string;
  variant?: "default" | "stacked";
  activeItemId?: string | null;
}

interface SidebarProps {
  children: React.ReactNode;
  className?: string;
}

interface SidebarHeaderProps {
  children: React.ReactNode;
  className?: string;
}

interface SearchInputProps extends React.InputHTMLAttributes<HTMLInputElement> {
  className?: string;
}

interface SidebarHeaderActionsProps {
  children: React.ReactNode;
  className?: string;
}

interface SidebarContentProps {
  children: React.ReactNode;
  className?: string;
}

interface SidebarGroupProps {
  id: string;
  children: React.ReactNode;
  defaultOpen?: boolean;
  className?: string;
}

interface SidebarGroupHeaderProps {
  label?: string;
  count?: number;
  icon?: LucideIcon;
  iconClassName?: string;
  children?: React.ReactNode;
}

interface SidebarGroupContentProps {
  children: React.ReactNode;
  variant?: "grouped" | "spaced";
  className?: string;
}

interface SidebarContentTriggerProps {
  children: React.ReactNode;
  className?: string;
}

interface SidebarItemCardProps {
  children: React.ReactNode;
  isActive?: boolean;
  onClick?: () => void;
  className?: string;
}

interface SidebarFooterProps {
  children: React.ReactNode;
  className?: string;
}

interface ContentProps {
  children: React.ReactNode;
  className?: string;
}

interface ContentHeaderProps {
  children: React.ReactNode;
  className?: string;
}

interface ContentTitleProps {
  children: React.ReactNode;
  className?: string;
}

interface ContentDescriptionProps {
  children: React.ReactNode;
  className?: string;
}

interface ContentHeaderActionsProps {
  children: React.ReactNode;
  className?: string;
}

interface ContentBodyProps {
  children: React.ReactNode;
  className?: string;
}

interface DetailProps {
  children: React.ReactNode;
  className?: string;
}

interface DetailTabsProps {
  children: React.ReactNode;
  defaultValue: string;
  className?: string;
}

interface DetailTabProps {
  value: string;
  label: string;
  badge?: number;
  icon?: LucideIcon;
  children: React.ReactNode;
  className?: string;
}

interface WidgetProps {
  children: React.ReactNode;
  className?: string;
}

interface WidgetHeaderProps {
  children: React.ReactNode;
  className?: string;
}

interface WidgetTitleProps {
  children: React.ReactNode;
  className?: string;
}

interface WidgetContentProps {
  children: React.ReactNode;
  className?: string;
}

interface WidgetItemProps {
  children: React.ReactNode;
  className?: string;
}

type SplitPageLayoutVariant = "default" | "stacked";

interface SplitPageLayoutNavigationContextValue {
  showBackButton: boolean;
  onBack: () => void;
  onSidebarItemSelect?: () => void;
}

// ─── Helpers ──────────────────────────────────────────────────────────────────

type SlotComponent<T> = React.ComponentType<T> & {
  slotKey?: string;
};

function getSlotKey<T>(type: React.ComponentType<T>) {
  return (type as SlotComponent<T>).slotKey;
}

function unwrapSlotChild(child: React.ReactNode): React.ReactNode {
  if (!React.isValidElement(child)) return child;

  const props = child.props as {
    children?: React.ReactNode;
    style?: React.CSSProperties;
    "data-jr-key"?: string;
  };

  // json-render devtools wrap direct children in display:contents spans.
  if (
    child.type === "span" &&
    props["data-jr-key"] &&
    props.style?.display === "contents" &&
    props.children
  ) {
    return props.children;
  }

  return child;
}

function hasSlotKey<T>(
  child: React.ReactNode,
  type: React.ComponentType<T>,
): child is React.ReactElement<T> {
  const unwrapped = unwrapSlotChild(child);
  if (!React.isValidElement(unwrapped)) return false;

  const expectedSlotKey = getSlotKey(type);
  const childSlotKey = getSlotKey(unwrapped.type as React.ComponentType<T>);

  return Boolean(expectedSlotKey) && childSlotKey === expectedSlotKey;
}

function findSlot<T>(children: React.ReactNode, type: React.ComponentType<T>) {
  return React.Children.toArray(children).find(
    (child) => hasSlotKey(child, type),
  ) as React.ReactElement<T> | undefined;
}

function findSlots<T>(children: React.ReactNode, type: React.ComponentType<T>) {
  return React.Children.toArray(children).filter(
    (child) => hasSlotKey(child, type),
  ) as React.ReactElement<T>[];
}

const SplitPageLayoutNavigationContext = React.createContext<SplitPageLayoutNavigationContextValue | null>(null);

const STACKED_PANEL_VARIANTS = {
  hidden: (direction: number) => ({
    x: direction > 0 ? 32 : -32,
    opacity: 0,
  }),
  visible: {
    x: 0,
    opacity: 1,
  },
  exit: (direction: number) => ({
    x: direction > 0 ? -32 : 32,
    opacity: 0,
  }),
};

// ─── Sidebar Components ───────────────────────────────────────────────────────

function Sidebar({ children, className }: SidebarProps) {
  return (
    <div
      className={cn(
        "flex h-full w-88 min-h-0 flex-col overflow-hidden",
        className,
      )}
    >
      {children}
    </div>
  );
}

function SidebarHeader({ children, className }: SidebarHeaderProps) {
  return (
    <div
      className={cn(
        "z-20 flex shrink-0 flex-col gap-2.5 border-b border-border/25 bg-background px-3 py-3",
        className,
      )}
    >
      {children}
    </div>
  );
}

function SearchInput({ className, placeholder = "Search...", ...props }: SearchInputProps) {
  return (
    <div className={cn("flex w-full items-center gap-2", className)}>
      <div className="relative min-w-0 flex-1">
        <Search className="pointer-events-none absolute left-3 top-1/2 size-3.5 -translate-y-1/2 text-muted-foreground" />
        <Input
          placeholder={placeholder}
          className={cn(
            "h-9 w-full rounded-full border-0 bg-muted/35 pl-9 text-[13px] shadow-none",
            "placeholder:text-muted-foreground/80",
            "focus-visible:bg-muted/45 focus-visible:ring-1 focus-visible:ring-border/60",
          )}
          {...props}
        />
      </div>
      <button
        type="button"
        aria-label="Filters"
        className={cn(
          "flex size-9 shrink-0 items-center justify-center rounded-full",
          "bg-muted/35 text-muted-foreground transition-colors",
          "hover:bg-muted/50 hover:text-foreground",
        )}
      >
        <ListFilter className="size-3.5" strokeWidth={1.75} />
      </button>
    </div>
  );
}

function SidebarHeaderActions({ children, className }: SidebarHeaderActionsProps) {
  return <div className={cn("flex shrink-0 items-center gap-1", className)}>{children}</div>;
}

function SidebarContent({ children, className }: SidebarContentProps) {
  return (
    <div
      className={cn(
        "flex min-h-0 flex-1 flex-col gap-0.5 overflow-y-auto px-2.5 py-2",
        className,
      )}
    >
      {children}
    </div>
  );
}

function SidebarGroup({ id, children, defaultOpen = true, className }: SidebarGroupProps) {
  const headerChild = findSlot(children, SidebarGroupHeader);
  const contentChild = findSlot(children, SidebarGroupContent);

  return (
    <Accordion
      type="multiple"
      defaultValue={defaultOpen ? [id] : []}
      className={cn("w-full", className)}
    >
      <AccordionItem value={id} className="border-b-0">
        <AccordionTrigger className="hover:no-underline py-2 text-sm font-medium text-muted-foreground">
          {headerChild}
        </AccordionTrigger>
        <AccordionContent className="pt-1 pb-3">{contentChild}</AccordionContent>
      </AccordionItem>
    </Accordion>
  );
}

function SidebarGroupHeader({
  label,
  count,
  icon: Icon,
  iconClassName,
  children,
}: SidebarGroupHeaderProps) {
  if (children) return <>{children}</>;
  return (
    <div className="flex items-center gap-2">
      {Icon && <Icon className={cn("size-4", iconClassName)} />}
      {label && <span>{label}</span>}
      {count !== undefined && (
        <Badge variant="secondary" className="ml-1 h-5 px-1.5 text-xs font-normal">
          {count}
        </Badge>
      )}
    </div>
  );
}

function SidebarGroupContent({ children, variant = "grouped", className }: SidebarGroupContentProps) {
  return (
    <div
      className={cn(
        'w-full',
        variant === "grouped" && "flex flex-col bg-card/50 divide-y border rounded-md overflow-hidden",
        variant === "spaced" && "flex flex-col gap-1.5",
        className,
      )}
    >
      {children}
    </div>
  );
}

function SidebarContentTrigger({ children, className }: SidebarContentTriggerProps) {
  return (
    <div
      className={cn(
        "flex items-center gap-2 px-1 py-1.5 text-xs font-medium text-muted-foreground",
        className,
      )}
    >
      {children}
    </div>
  );
}

function SidebarItemCard({ children, isActive, onClick, className }: SidebarItemCardProps) {
  const navigation = React.useContext(SplitPageLayoutNavigationContext);

  const handleClick = React.useCallback(() => {
    onClick?.();
    navigation?.onSidebarItemSelect?.();
  }, [navigation, onClick]);

  return (
    <button
      type="button"
      onClick={handleClick}
      className={cn(
        "flex w-full items-start gap-2.5 rounded-lg px-2.5 py-2.5 text-left text-[13px] transition-colors",
        isActive &&
          "bg-accent/55 text-accent-foreground shadow-[inset_2px_0_0_0_hsl(var(--foreground))]",
        !isActive && "hover:bg-muted/40",
        className,
      )}
    >
      {children}
    </button>
  );
}

function SidebarFooter({ children, className }: SidebarFooterProps) {
  return (
    <div
      className={cn(
        "flex shrink-0 items-center mt-auto px-5 h-12 text-xs text-muted-foreground",
        className,
      )}
    >
      {children}
    </div>
  );
}

// ─── Content Panel ────────────────────────────────────────────────────────────

function Content({ children, className }: ContentProps) {
  return (
    <div className={cn("flex h-full flex-col overflow-hidden", className)}>
      {children}
    </div>
  );
}

function ContentHeader({ children, className }: ContentHeaderProps) {
  const navigation = React.useContext(SplitPageLayoutNavigationContext);

  return (
    <div className={cn("flex min-h-12 items-center gap-3 border-b border-border/40 px-5 py-3", className)}>
      {navigation?.showBackButton && (
        <Button
          type="button"
          variant="ghost"
          size="icon"
          onClick={navigation.onBack}
          className="h-8 px-2 text-xs font-medium text-muted-foreground"
        >
          <ArrowLeft className="size-4" />
        </Button>
      )}
      {children}
    </div>
  );
}


function ContentHeaderMain({ children, className }: ContentHeaderProps) {
  return (
    <div className={cn("flex gap-x-4 items-center min-w-0 flex-1", className)}>
      {children}
    </div>
  );
}

function ContentTitle({ children, className }: ContentTitleProps) {
  return (
    <span className={cn("truncate text-sm font-semibold leading-tight", className)}>
      {children}
    </span>
  );
}

function ContentDescription({ children, className }: ContentDescriptionProps) {
  return (
    <span className={cn("truncate text-sm text-muted-foreground leading-tight max-w-xl", className)}>
      {children}
    </span>
  );
}

function ContentHeaderActions({ children, className }: ContentHeaderActionsProps) {
  return (
    <div className={cn("flex shrink-0 items-center gap-1", className)}>
      {children}
    </div>
  );
}

function ContentBody({ children, className }: ContentBodyProps) {
  return (
    <div className={cn("flex flex-1 flex-col min-h-0 overflow-y-auto scroll-fade overflow-y-auto scroll-fade-14", className)}>
      {children}
    </div>
  );
}

// ─── Detail Panel ─────────────────────────────────────────────────────────────

function Detail({ children, className }: DetailProps) {
  return (
    <div className={cn("flex h-full flex-col overflow-hidden w-88", className)}>
      {children}
    </div>
  );
}

function DetailTabs({ children, defaultValue, className }: DetailTabsProps) {
  const tabs = findSlots(children, DetailTab);

  return (
    <Tabs
      defaultValue={defaultValue}
      className={cn("flex h-full w-full flex-col overflow-hidden max-w-full", className)}
    >
      <div className="flex h-14 shrink-0 items-center px-4">
        <TabsList variant="text" className="h-9 w-fit justify-start gap-4 p-0">
          {tabs.map((tab) => {
            const { value, label, badge, icon: Icon } = tab.props;
            return (
              <TabsTrigger key={value} value={value} className="flex items-center gap-1.5">
                {Icon && <Icon className="size-3.5" />}
                {label}
                {badge !== undefined && (
                  <Badge
                    variant="secondary"
                    className="ml-1 flex h-4 min-w-4 items-center justify-center rounded-md px-1 text-sm"
                  >
                    {badge}
                  </Badge>
                )}
              </TabsTrigger>
            );
          })}
        </TabsList>
      </div>

      {tabs.map((tab) => (
        <TabsContent
          key={tab.props.value}
          value={tab.props.value}
          className={cn("m-0 flex min-h-0 flex-1 flex-col outline-none", tab.props.className)}
        >
          <div className="flex flex-col gap-3 p-4 overflow-y-auto">{tab.props.children}</div>
        </TabsContent>
      ))}
    </Tabs>
  );
}

function DetailTab(_props: DetailTabProps) {
  return null;
}

// ─── Widget Components ────────────────────────────────────────────────────────

function Widget({ children, className }: WidgetProps) {
  return (
    <div
      className={cn(
        "flex flex-col overflow-hidden text-sm",
        className,
      )}
    >
      {children}
    </div>
  );
}

function WidgetHeader({ children, className }: WidgetHeaderProps) {
  return (
    <div className={cn("flex items-center px-3 py-2", className)}>{children}</div>
  );
}

function WidgetTitle({ children, className }: WidgetTitleProps) {
  return (
    <span
      className={cn(
        "text-xs text-muted-foreground",
        className,
      )}
    >
      {children}
    </span>
  );
}

function WidgetContent({ children, className }: WidgetContentProps) {
  return (
    <div className={cn("flex flex-col divide-y divide-border/50 rounded-md border bg-muted/20 max-w-full", className)}>{children}</div>
  );
}

function WidgetItem({ children, className }: WidgetItemProps) {
  return <div className={cn("flex items-center gap-2 p-3", className)}>{children}</div>;
}

(Sidebar as SlotComponent<SidebarProps>).slotKey = "split-page-layout.sidebar";
(Content as SlotComponent<ContentProps>).slotKey = "split-page-layout.content";
(Detail as SlotComponent<DetailProps>).slotKey = "split-page-layout.detail";
(SidebarGroupHeader as SlotComponent<SidebarGroupHeaderProps>).slotKey =
  "split-page-layout.sidebar-group-header";
(SidebarGroupContent as SlotComponent<SidebarGroupContentProps>).slotKey =
  "split-page-layout.sidebar-group-content";
(DetailTab as SlotComponent<DetailTabProps>).slotKey = "split-page-layout.detail-tab";

// ─── Root ─────────────────────────────────────────────────────────────────────

function SplitPageLayoutRoot({
  children,
  className,
  variant = "default",
  activeItemId = null,
}: SplitPageLayoutProps) {
  const sidebar = findSlot(children, Sidebar);
  const content = findSlot(children, Content);
  const detail = findSlot(children, Detail);

  const isMobile = useIsMobile();
  const sidebarVisible = aos.stores.viewport.useState(s => s.page.sidebar.visible);
  const detailsVisible = aos.stores.viewport.useState(s => s.page.details.visible);

  const shouldUseStacked = variant === "stacked" || isMobile;
  const effectiveVariant: SplitPageLayoutVariant = Boolean(sidebar) && shouldUseStacked
    ? "stacked"
    : "default";

  const [showDetail, setShowDetail] = React.useState(false);
  const [transitionDirection, setTransitionDirection] = React.useState<1 | -1>(1);

  React.useEffect(() => {
    if (effectiveVariant !== "stacked") return;

    if (activeItemId) {
      setTransitionDirection(1);
      setShowDetail(true);
      return;
    }

    setTransitionDirection(-1);
    setShowDetail(false);
  }, [activeItemId, effectiveVariant]);

  const handleBack = React.useCallback(() => {
    setTransitionDirection(-1);
    setShowDetail(false);
  }, []);

  const handleSidebarItemSelect = React.useCallback(() => {
    if (effectiveVariant !== "stacked") return;

    setTransitionDirection(1);
    setShowDetail(true);
  }, [effectiveVariant]);

  const navigationContextValue = React.useMemo<SplitPageLayoutNavigationContextValue>(
    () => ({
      showBackButton: effectiveVariant === "stacked" && showDetail,
      onBack: handleBack,
      onSidebarItemSelect: effectiveVariant === "stacked" ? handleSidebarItemSelect : undefined,
    }),
    [effectiveVariant, handleBack, handleSidebarItemSelect, showDetail],
  );

  const stackedSidebar = React.useMemo(() => {
    if (!sidebar) return null;

    return React.cloneElement(sidebar, {
      className: cn(sidebar.props.className, "w-full border-r-0"),
    });
  }, [sidebar]);

  if (effectiveVariant === "stacked") {
    return (
      <SplitPageLayoutNavigationContext.Provider value={navigationContextValue}>
        <div
          className={cn(
            "flex h-full w-full overflow-hidden",
            className,
          )}
        >
          <AnimatePresence initial={false} mode="wait">
            {!showDetail ? (
              <motion.div
                key="split-page-layout.list"
                custom={transitionDirection}
                variants={STACKED_PANEL_VARIANTS}
                initial="hidden"
                animate="visible"
                exit="exit"
                transition={{ type: "spring", bounce: 0, duration: 0.35 }}
                className="h-full w-full overflow-hidden"
              >
                {stackedSidebar}
              </motion.div>
            ) : (
              <motion.div
                key="split-page-layout.detail"
                custom={transitionDirection}
                variants={STACKED_PANEL_VARIANTS}
                initial="hidden"
                animate="visible"
                exit="exit"
                transition={{ type: "spring", bounce: 0, duration: 0.35 }}
                className="flex h-full w-full overflow-hidden"
              >
                <div className="flex-1 min-w-0 h-full overflow-hidden">
                  {content}
                </div>
                {detail && (
                  <div className="shrink-0 h-full overflow-hidden">
                    {detail}
                  </div>
                )}
              </motion.div>
            )}
          </AnimatePresence>
        </div>
      </SplitPageLayoutNavigationContext.Provider>
    );
  }

  return (
    <SplitPageLayoutNavigationContext.Provider value={navigationContextValue}>
      <div
        className={cn(
          "flex h-full w-full overflow-hidden",
          className,
        )}
      >
        <AnimatePresence initial={false} mode="popLayout">
          {sidebar && sidebarVisible && (
            <motion.div
              layout
              initial={{ width: 0, opacity: 0 }}
              animate={{ width: "auto", opacity: 1 }}
              exit={{ width: 0, opacity: 0 }}
              transition={{ type: "spring", bounce: 0, duration: 0.4 }}
              className="overflow-hidden shrink-0 h-full"
            >
              {sidebar}
            </motion.div>
          )}
        </AnimatePresence>

        <motion.div layout className="flex-1 min-w-0 h-full overflow-hidden">
          {content}
        </motion.div>

        <AnimatePresence initial={false} mode="popLayout">
          {detail && detailsVisible && (
            <motion.div
              layout
              initial={{ width: 0, opacity: 0 }}
              animate={{ width: "auto", opacity: 1 }}
              exit={{ width: 0, opacity: 0 }}
              transition={{ type: "spring", bounce: 0, duration: 0.4 }}
              className="overflow-hidden shrink-0 h-full"
            >
              {detail}
            </motion.div>
          )}
        </AnimatePresence>
      </div>
    </SplitPageLayoutNavigationContext.Provider>
  );
}

// ─── Export ───────────────────────────────────────────────────────────────────

export const SplitPageLayout = Object.assign(SplitPageLayoutRoot, {
  Sidebar,
  SidebarHeader,
  SearchInput,
  SidebarHeaderActions,
  SidebarContent,
  SidebarGroup,
  SidebarGroupHeader,
  SidebarGroupContent,
  SidebarContentTrigger,
  SidebarItemCard,
  SidebarFooter,
  Content,
  ContentHeader,
  ContentHeaderMain,
  ContentTitle,
  ContentDescription,
  ContentHeaderActions,
  ContentBody,
  Detail,
  DetailTabs,
  DetailTab,
  Widget,
  WidgetHeader,
  WidgetTitle,
  WidgetContent,
  WidgetItem,
});
