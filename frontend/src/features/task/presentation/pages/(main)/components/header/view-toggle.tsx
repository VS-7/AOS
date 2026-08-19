import { ToggleGroup, ToggleGroupItem } from "@/components/ui/toggle-group";
import { Columns3, LayoutList } from "lucide-react";
import { useTasksContext } from "@/features/task/presentation/pages/(main)/context";

export function TasksViewToggle() {
  const { currentView, handleViewChange } = useTasksContext();

  return (
    <ToggleGroup type="single" variant="outline" className="h-9! rounded-md" value={currentView} onValueChange={handleViewChange}>
      <ToggleGroupItem value="list" className="rounded-l-md!">
        <LayoutList data-icon="inline-start" />
      </ToggleGroupItem>
      <ToggleGroupItem value="kanban" className="rounded-r-md!">
        <Columns3 data-icon="inline-start" />
      </ToggleGroupItem>
    </ToggleGroup>
  );
}
