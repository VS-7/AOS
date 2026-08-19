import type { BaseComponentProps } from "@json-render/react";
import {
  TabsSubtle,
  TabsSubtleItem,
} from "@/components/ui/tabs-subtle";
import { cn } from "@/lib/utils";
import { resolveLucideIcon } from "../shared/resolve-lucide-icon";
import { useBoundValue } from "../shared/use-bound-value";

type TabItem = {
  label: string;
  value: string;
  icon?: string | null;
  count?: string | number | null;
};

type TabsSubtleProps = {
  items: TabItem[];
  value?: string | null;
  activeLabel?: boolean | null;
  className?: string | null;
};

/**
 * Fractal subtle tabs with animated selection pill.
 * With `activeLabel` + icon: inactive tabs collapse to icon (+ count); active expands the label.
 */
export function TabsSubtleComponent({
  props,
  bindings,
  emit,
}: BaseComponentProps<TabsSubtleProps>) {
  const [value, setValue] = useBoundValue(
    bindings,
    "value",
    props.value ?? props.items[0]?.value ?? "",
  );

  const selectedIndex = Math.max(
    0,
    props.items.findIndex((item: any) => item.value === value),
  );

  return (
    <TabsSubtle
      selectedIndex={selectedIndex}
      activeLabel={props.activeLabel ?? false}
      className={cn(props.className)}
      onSelect={(index) => {
        const next = props.items[index]?.value;
        if (next !== undefined) {
          setValue(next);
          emit("change");
        }
      }}
    >
      {props.items.map((item: any, index: number) => {
        const IconComp = item.icon ? resolveLucideIcon(item.icon) : undefined;
        return (
          <TabsSubtleItem
            key={item.value}
            index={index}
            label={item.label}
            icon={IconComp ?? undefined}
            count={item.count ?? undefined}
          />
        );
      })}
    </TabsSubtle>
  );
}
