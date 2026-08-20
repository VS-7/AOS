import { useState } from "react";
import type { Task } from "@/features/task/interfaces/task.interfaces";

interface TaskStatusTransitionState {
  open: boolean;
  task: Task | null;
  status: Task["status"] | null;
}

export function useTasksStatusTransition() {
  const [state, setState] = useState<TaskStatusTransitionState>({
    open: false,
    task: null,
    status: null,
  });

  function open(task: Task, status: Task["status"]) {
    setState({
      open: true,
      task,
      status,
    });
  }

  function close() {
    setState({
      open: false,
      task: null,
      status: null,
    });
  }

  return {
    state,
    open,
    close,
  };
}
