import { useEffect, useRef } from "react";
import type { JSX } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { client, isDesktop } from "@/lib/client";

interface ApprovalRequest {
  id: string;
  tool: string;
  agent?: string;
  input?: unknown;
  reason?: string;
  risk: "low" | "medium" | "high";
  expiresAt: string;
}

/**
 * The interface half of ADR-0007.
 *
 * A policy hook can answer "ask" instead of allow or deny, and when it does the
 * agent loop blocks here until somebody decides. In the original there is
 * nowhere for that question to appear, so "ask" degrades to "deny" and the
 * autonomy matrix in the master prompt cannot be followed. This is the window
 * it appears in.
 */
export function ApprovalModal(): JSX.Element | null {
  const queryClient = useQueryClient();
  const dialog = useRef<HTMLDivElement | null>(null);

  const pending = useQuery({
    queryKey: ["approvals"],
    queryFn: async () => {
      const out = (await client.invoke("approvals_list", {
        _reasoning: "the window is checking what is waiting on a person",
      })) as { pending: ApprovalRequest[] };
      return out.pending ?? [];
    },
    // Realtime pushes an approval.request event, and this is the floor under
    // it: a request that arrived while the window was closed is still blocking
    // a run, and nothing else would surface it.
    refetchInterval: 5_000,
  });

  const decide = useMutation({
    mutationFn: async (input: { id: string; approved: boolean; reason: string }) => {
      if (isDesktop() && window.go?.ApprovalService) {
        return window.go.ApprovalService.Resolve(input.id, {
          approved: input.approved,
          reason: input.reason,
        });
      }
      return client.invoke("approvals_decide", {
        id: input.id,
        approved: input.approved,
        reason: input.reason,
        _reasoning: "the person answered a waiting tool call",
      });
    },
    onSettled: () => queryClient.invalidateQueries({ queryKey: ["approvals"] }),
  });

  const request = pending.data?.[0];

  useEffect(() => {
    // A modal that appears without focus is a modal a keyboard user does not
    // know is there.
    if (request) dialog.current?.focus();
  }, [request?.id]);

  if (!request) return null;

  return (
    <div className="approval-backdrop" role="presentation">
      <div
        className="approval"
        role="alertdialog"
        aria-modal="true"
        aria-labelledby="approval-title"
        tabIndex={-1}
        ref={dialog}
        onKeyDown={(e) => {
          // Escape refuses rather than dismissing. A dialog that can be closed
          // without answering leaves the run blocked until its deadline, and
          // the person believes they said no.
          if (e.key === "Escape") {
            e.preventDefault();
            decide.mutate({ id: request.id, approved: false, reason: "dismissed" });
          }
        }}
      >
        <span className="risk" data-risk={request.risk}>
          {request.risk} risk
        </span>
        <h3 id="approval-title">
          {request.agent ?? "An agent"} wants to run {request.tool}
        </h3>
        {request.reason && <p>{request.reason}</p>}
        <pre>{JSON.stringify(request.input ?? {}, null, 2)}</pre>
        <p className="status">
          Waiting until {new Date(request.expiresAt).toLocaleTimeString()}. Unanswered, it is
          refused.
        </p>
        <div className="actions">
          <button
            onClick={() =>
              decide.mutate({ id: request.id, approved: false, reason: "refused by the person" })
            }
            disabled={decide.isPending}
          >
            Refuse
          </button>
          <button
            onClick={() =>
              decide.mutate({ id: request.id, approved: true, reason: "allowed by the person" })
            }
            disabled={decide.isPending}
          >
            Allow once
          </button>
        </div>
      </div>
    </div>
  );
}
