"use client";

import * as React from "react";
import {
  DndContext,
  DragOverlay,
  PointerSensor,
  closestCenter,
  useSensor,
  useSensors,
  type DragEndEvent,
  type DragStartEvent,
} from "@dnd-kit/core";
import {
  SortableContext,
  horizontalListSortingStrategy,
} from "@dnd-kit/sortable";
import { aos } from "@/app/aos";
import type { ViewportTabState } from "@/features/workspace/presentation/stores/viewport.store";
import {
  WorkspaceTabItem,
  WorkspaceSortableTabItem,
} from "./workspace-tab-item";

const AOS_TAB_ID = "aos";

interface WorkspaceTabStripProps {
  tabs: ViewportTabState[];
  activeTabId: string;
  onSelect: (tabId: string) => void;
}

export function WorkspaceTabStrip({
  tabs,
  activeTabId,
  onSelect,
}: WorkspaceTabStripProps) {
  const [activeDragTab, setActiveDragTab] =
    React.useState<ViewportTabState | null>(null);

  const anchorTab = tabs.find((tab) => tab.id === AOS_TAB_ID);
  const movableTabs = tabs.filter((tab) => tab.id !== AOS_TAB_ID);
  const sortableIds = React.useMemo(
    () => movableTabs.map((tab) => tab.id),
    [movableTabs],
  );

  const sensors = useSensors(
    useSensor(PointerSensor, {
      activationConstraint: {
        distance: 8,
      },
    }),
  );

  const handleDragStart = (event: DragStartEvent) => {
    const tab = tabs.find((item) => item.id === event.active.id);
    setActiveDragTab(tab ?? null);
  };

  const handleDragEnd = (event: DragEndEvent) => {
    const { active, over } = event;
    setActiveDragTab(null);

    if (!over || active.id === over.id) return;

    aos.stores.viewport.actions.reorderTabs(
      String(active.id),
      String(over.id),
    );
  };

  const handleDragCancel = () => {
    setActiveDragTab(null);
  };

  return (
    <DndContext
      sensors={sensors}
      collisionDetection={closestCenter}
      onDragStart={handleDragStart}
      onDragEnd={handleDragEnd}
      onDragCancel={handleDragCancel}
    >
      <div className="flex items-center gap-1 h-12 w-max shrink-0">
        {anchorTab ? (
          <WorkspaceTabItem
            tab={anchorTab}
            isActive={anchorTab.id === activeTabId}
            onSelect={onSelect}
            variant="static"
          />
        ) : null}

        <SortableContext
          items={sortableIds}
          strategy={horizontalListSortingStrategy}
        >
          {movableTabs.map((tab) => (
            <WorkspaceSortableTabItem
              key={tab.id}
              tab={tab}
              isActive={tab.id === activeTabId}
              onSelect={onSelect}
            />
          ))}
        </SortableContext>
      </div>

      <DragOverlay
        dropAnimation={{
          duration: 150,
          easing: "ease-out",
        }}
      >
        {activeDragTab ? (
          <WorkspaceTabItem
            tab={activeDragTab}
            isActive={activeDragTab.id === activeTabId}
            onSelect={onSelect}
            variant="overlay"
          />
        ) : null}
      </DragOverlay>
    </DndContext>
  );
}
