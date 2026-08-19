import { useState } from "react";
import type { FractalTask } from "@/features/task/interfaces/task.interfaces";

interface TaskStatusTransitionState {
  open: boolean;
  task: FractalTask | null;
  status: FractalTask["status"] | null;
}

export function useTasksStatusTransition() {
  const [state, setState] = useState<TaskStatusTransitionState>({
    open: false,
    task: null,
    status: null,
  });

  function open(task: FractalTask, status: FractalTask["status"]) {
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
