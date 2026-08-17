import { useState } from "react";
import type { JSX } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { client } from "@/lib/client";
import { Failure } from "@/components/Failure";

/** The eight states, in the order the board shows them. */
const COLUMNS = [
  "suggestion",
  "backlog",
  "planning",
  "todo",
  "in_progress",
  "stopped",
  "in_review",
  "finished",
] as const;

type Status = (typeof COLUMNS)[number];

interface TaskView {
  id: string;
  name: string;
  type?: string;
  status: Status;
  priority?: string;
  assignee?: { id: string; type: string; name?: string };
  progress?: { completed: number; total: number };
  blocked?: string[];
}

/**
 * The board.
 *
 * Dragging a card calls tasks_set-status, which is the same command the CLI and
 * the agent call, with the same guards behind it. A board that moved the card
 * by writing the field would be a board that can put a task in review with an
 * unfinished plan — which is precisely the rule the domain exists to hold.
 */
export function TaskBoard(): JSX.Element {
  const queryClient = useQueryClient();
  const [dragging, setDragging] = useState<string | null>(null);
  const [over, setOver] = useState<Status | null>(null);
  const [refused, setRefused] = useState<unknown>(null);

  const tasks = useQuery({
    queryKey: ["task"],
    queryFn: async () =>
      (await client.invoke("tasks_list", {
        limit: 500,
        _reasoning: "the board is open",
      })) as { tasks: TaskView[] },
  });

  const move = useMutation({
    mutationFn: async (input: { id: string; status: Status }) =>
      client.invoke("tasks_set-status", {
        id: input.id,
        status: input.status,
        _reasoning: "the person moved the card on the board",
      }),
    onSuccess: () => {
      setRefused(null);
      void queryClient.invalidateQueries({ queryKey: ["task"] });
    },
    // A refusal is shown rather than swallowed: the card snapping back with no
    // explanation is the worst version of this interaction.
    onError: (error) => setRefused(error),
  });

  if (tasks.isLoading) return <p className="empty">Reading the board…</p>;
  if (tasks.error) return <Failure error={tasks.error} />;

  const all = tasks.data?.tasks ?? [];

  return (
    <>
      {refused !== null && <Failure error={refused} />}
      <div className="board">
        {COLUMNS.map((status) => {
          const inColumn = all.filter((t) => t.status === status);
          return (
            <section className="column" key={status}>
              <h3>
                {label(status)} <span aria-label="count">({inColumn.length})</span>
              </h3>
              <div
                className="cards"
                data-over={over === status}
                onDragOver={(e) => {
                  e.preventDefault();
                  setOver(status);
                }}
                onDragLeave={() => setOver((current) => (current === status ? null : current))}
                onDrop={(e) => {
                  e.preventDefault();
                  setOver(null);
                  if (dragging) move.mutate({ id: dragging, status });
                  setDragging(null);
                }}
              >
                {inColumn.map((task) => (
                  <TaskCard
                    key={task.id}
                    task={task}
                    onDragStart={() => setDragging(task.id)}
                    onMove={(to) => move.mutate({ id: task.id, status: to })}
                  />
                ))}
              </div>
            </section>
          );
        })}
      </div>
    </>
  );
}

function TaskCard({
  task,
  onDragStart,
  onMove,
}: {
  task: TaskView;
  onDragStart: () => void;
  onMove: (to: Status) => void;
}): JSX.Element {
  const done = task.progress?.completed ?? 0;
  const total = task.progress?.total ?? 0;

  return (
    <article className="card task" draggable onDragStart={onDragStart}>
      <h4>{task.name}</h4>
      <div className="meta">
        {task.type && <span>{task.type}</span>}
        {task.priority && task.priority !== "no_priority" && <span>{task.priority}</span>}
        {task.assignee?.id && (
          <span>
            {task.assignee.name ?? task.assignee.id}
            {task.assignee.type !== "agent" && " (not dispatched)"}
          </span>
        )}
        {task.blocked && task.blocked.length > 0 && (
          <span className="error">blocked by {task.blocked.length}</span>
        )}
      </div>
      {total > 0 && (
        <>
          <div className="progress" role="progressbar" aria-valuenow={done} aria-valuemax={total}>
            <span style={{ width: `${(done / total) * 100}%` }} />
          </div>
          <span className="meta">
            {done} of {total} steps
          </span>
        </>
      )}

      {/*
        Dragging is not reachable from a keyboard, so the same move is a select.
        A board that can only be used with a mouse is a board half the people
        who need it cannot use.
      */}
      <label className="meta">
        Move to{" "}
        <select
          value={task.status}
          aria-label={`Move ${task.name}`}
          onChange={(e) => onMove(e.target.value as Status)}
        >
          {COLUMNS.map((status) => (
            <option key={status} value={status}>
              {label(status)}
            </option>
          ))}
        </select>
      </label>
    </article>
  );
}

function label(status: Status): string {
  return status.replaceAll("_", " ");
}
