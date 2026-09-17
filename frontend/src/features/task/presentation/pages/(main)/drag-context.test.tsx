import { afterEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, render } from "@testing-library/react";
import type { Task } from "@/features/task/interfaces/task.interfaces";

/**
 * The kanban's drop target is state this provider keeps, not something
 * @dnd-kit derives per frame: a column lights up because `overContainerId`
 * names it. A column the lifecycle does not reach is a disabled droppable and
 * makes `over` null, so the provider has to be told what "over nothing" means
 * — it used to mean "keep the last column", which lit a column the pointer
 * had already left while it hovered one that refuses the card.
 */

vi.mock("@tanstack/react-router", () => ({
  useNavigate: () => vi.fn(),
  useRouter: () => ({ invalidate: vi.fn() }),
}));

vi.mock("@/app/aos", () => ({
  aos: {
    stores: {
      workspace: { useState: () => undefined },
    },
  },
}));

import { TasksProvider, useDragContext } from "./context";

const task = (id: string, status: Task["status"]): Task =>
  ({ id, name: id, status, priority: "no_priority" }) as unknown as Task;

function renderDrag(tasks: Task[]) {
  const seen: { current: ReturnType<typeof useDragContext> | null } = { current: null };
  function Probe() {
    seen.current = useDragContext();
    return null;
  }
  render(
    <TasksProvider tasks={tasks} search={{ view: "kanban" }} client={{}} route={{}}>
      <Probe />
    </TasksProvider>,
  );
  return seen;
}

describe("the kanban's drop target", () => {
  afterEach(cleanup);

  it("clears while the card hovers a column that takes no drop", () => {
    const drag = renderDrag([task("t1", "todo")]);

    act(() => drag.current!.handleDragStart({ active: { id: "t1" } } as never));
    expect(drag.current!.activeDropStatus).toBe("todo");

    act(() => drag.current!.handleDragOver({ over: { id: "in_progress" } } as never));
    expect(drag.current!.activeDropStatus).toBe("in_progress");

    // Over a disabled column: @dnd-kit reports no droppable at all.
    act(() => drag.current!.handleDragOver({ over: null } as never));
    expect(drag.current!.activeDropStatus).toBeNull();
  });
});
