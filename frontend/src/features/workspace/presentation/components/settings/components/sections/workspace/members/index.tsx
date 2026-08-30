import * as React from "react";
import { Add01Icon, Delete02Icon } from "@hugeicons/core-free-icons";
import { HugeiconsIcon } from "@hugeicons/react";
import { toast } from "sonner";

import { aos } from "@/app/aos";
import { api } from "@/lib/aos-facade";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Label } from "@/components/ui/label";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
  AlertDialogTrigger,
} from "@/components/ui/alert-dialog";
import {
  FormSection,
  FormSectionContent,
  FormSectionDescription,
  FormSectionHeader,
  FormSectionItem,
  FormSectionTitle,
} from "@/components/ui/form-section";
import { SettingsSectionShell } from "../../../section-shell";
import type { UserPublic } from "@/features/auth/interfaces/user.interfaces";
import type { WorkspaceMember } from "@/features/workspace/interfaces/workspace.interfaces";
import { t } from "@/lib/i18n";

export function WorkspaceMembersSection() {
  const workspaceId = aos.stores.workspace.useState((state) => state.current?.id);
  const [createOpen, setCreateOpen] = React.useState(false);
  const [selectedUserId, setSelectedUserId] = React.useState("");
  const [selectedRole, setSelectedRole] = React.useState<"owner" | "member">("member");
  const [isCreating, setIsCreating] = React.useState(false);

  const membersQuery = api.workspace.listMembers.useQuery({
    params: { workspace: workspaceId ?? "" },
    enabled: Boolean(workspaceId),
  });

  const usersQuery = api.user.list.useQuery({
    query: {},
    enabled: Boolean(workspaceId),
  });

  // The daemon has no workspace-membership commands yet, so this query comes
  // back dormant. Saying "no members yet" for that is the wrong sentence in
  // the wrong place: this workspace may well have members, and the screen was
  // reporting on a capability that does not exist as though it had asked and
  // been told nothing.
  const membersUnavailable = membersQuery.isDormant;
  const members = membersUnavailable
    ? []
    : ((membersQuery.data as WorkspaceMember[] | undefined) ?? []);
  const users = (usersQuery.data as UserPublic[] | undefined) ?? [];

  const usersById = React.useMemo(
    () => new Map(users.map((user) => [user.id, user])),
    [users],
  );

  const availableUsers = users.filter(
    (user) => !members.some((member) => member.userId === user.id),
  );

  const refreshMembers = React.useCallback(async () => {
    await membersQuery.refetch();
  }, [membersQuery]);

  const handleCreate = async (event: React.FormEvent) => {
    event.preventDefault();
    if (!workspaceId || !selectedUserId) {
      toast.error(t("Select a user to add"));
      return;
    }

    setIsCreating(true);
    try {
      const result = await api.workspace.addMember.mutate({
        params: { workspace: workspaceId },
        body: { userId: selectedUserId, role: selectedRole },
      });

      if (result.error) {
        const message =
          result.error instanceof Error ? result.error.message : "Failed to add member";
        toast.error(message);
        return;
      }

      toast.success(t("Member added"));
      setCreateOpen(false);
      setSelectedUserId("");
      setSelectedRole("member");
      await refreshMembers();
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "Failed to add member");
    } finally {
      setIsCreating(false);
    }
  };

  const handleRoleChange = async (userId: string, role: "owner" | "member") => {
    if (!workspaceId) return;

    try {
      const result = await api.workspace.updateMember.mutate({
        params: { workspace: workspaceId, userId },
        body: { role },
      });

      if (result.error) {
        const message =
          result.error instanceof Error ? result.error.message : "Failed to update member";
        toast.error(message);
        return;
      }

      toast.success(t("Member updated"));
      await refreshMembers();
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "Failed to update member");
    }
  };

  const handleRemove = async (userId: string) => {
    if (!workspaceId) return;

    try {
      const result = await api.workspace.removeMember.mutate({
        params: { workspace: workspaceId, userId },
      });

      if (result.error) {
        const message =
          result.error instanceof Error ? result.error.message : "Failed to remove member";
        toast.error(message);
        return;
      }

      toast.success(t("Member removed"));
      await refreshMembers();
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "Failed to remove member");
    }
  };

  if (!workspaceId) {
    return (
      <SettingsSectionShell>
        <p className="p-4 text-sm text-muted-foreground">{t("No workspace selected.")}</p>
      </SettingsSectionShell>
    );
  }

  return (
    <SettingsSectionShell>
      <FormSection>
        <FormSectionHeader>
          <FormSectionTitle>{t("Workspace Members")}</FormSectionTitle>
          <FormSectionDescription>
            {t("Control which accounts can access this workspace and their membership role.")}
          </FormSectionDescription>
        </FormSectionHeader>

        <FormSectionContent>
          <FormSectionItem>
            <div className="min-w-0">
              <p className="text-sm font-medium text-foreground">{t("Members")}</p>
              <p className="text-sm text-muted-foreground">
                {t("Owners can manage membership. Members can collaborate in the workspace.")}
              </p>
            </div>
            <Button
              type="button"
              size="sm"
              // Nothing to add a member to yet. Left enabled, the click showed
              // the dormant error verbatim — "the workspace domain does not
              // exist in the Go backend yet" — which is a sentence for whoever
              // is writing the backend, not for whoever is using it.
              disabled={membersUnavailable}
              onClick={() => setCreateOpen(true)}
            >
              <HugeiconsIcon icon={Add01Icon} className="size-3.5" />
              {t("Add Member")}
            </Button>
          </FormSectionItem>

          <div className="divide-y divide-border">
            {membersQuery.isLoading ? (
              <p className="p-4 text-sm text-muted-foreground">{t("Loading members...")}</p>
            ) : membersUnavailable ? (
              <p className="p-4 text-sm text-muted-foreground">
                {t("Workspace membership is not available in this build yet.")}
              </p>
            ) : members.length === 0 ? (
              <p className="p-4 text-sm text-muted-foreground">{t("No members yet.")}</p>
            ) : (
              members.map((member) => {
                const user = usersById.get(member.userId);
                const label = user?.name || user?.email || member.userId;

                return (
                  <div
                    key={member.userId}
                    className="flex flex-wrap items-center justify-between gap-3 p-4"
                  >
                    <div className="min-w-0">
                      <p className="text-sm font-medium text-foreground">{label}</p>
                      <p className="text-sm text-muted-foreground">
                        {user?.email ?? member.userId}
                      </p>
                    </div>

                    <div className="flex items-center gap-2">
                      <Select
                        value={member.role}
                        onValueChange={(value) =>
                          void handleRoleChange(member.userId, value as "owner" | "member")
                        }
                      >
                        <SelectTrigger className="h-8 w-[120px]">
                          <SelectValue />
                        </SelectTrigger>
                        <SelectContent>
                          <SelectItem value="owner">{t("Owner")}</SelectItem>
                          <SelectItem value="member">{t("Member")}</SelectItem>
                        </SelectContent>
                      </Select>

                      <AlertDialog>
                        <AlertDialogTrigger asChild>
                          <Button
                            type="button"
                            variant="ghost"
                            size="icon"
                            className="size-8 text-destructive"
                            aria-label={`Remove ${label}`}
                          >
                            <HugeiconsIcon icon={Delete02Icon} className="size-3.5" />
                          </Button>
                        </AlertDialogTrigger>
                        <AlertDialogContent size="sm">
                          <AlertDialogHeader>
                            <AlertDialogTitle>{t("Remove member?")}</AlertDialogTitle>
                            <AlertDialogDescription>
                              {label} {t("will lose access to this workspace.")}
                            </AlertDialogDescription>
                          </AlertDialogHeader>
                          <AlertDialogFooter>
                            <AlertDialogCancel>{t("Cancel")}</AlertDialogCancel>
                            <AlertDialogAction
                              variant="destructive"
                              onClick={() => void handleRemove(member.userId)}
                            >
                              {t("Remove")}
                            </AlertDialogAction>
                          </AlertDialogFooter>
                        </AlertDialogContent>
                      </AlertDialog>
                    </div>
                  </div>
                );
              })
            )}
          </div>
        </FormSectionContent>
      </FormSection>

      <Dialog open={createOpen} onOpenChange={setCreateOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{t("Add Member")}</DialogTitle>
            <DialogDescription>
              {t("Grant an existing instance account access to this workspace.")}
            </DialogDescription>
          </DialogHeader>

          <form onSubmit={(event) => void handleCreate(event)} className="space-y-4">
            <div className="space-y-2">
              <Label>{t("User")}</Label>
              <Select value={selectedUserId} onValueChange={setSelectedUserId}>
                <SelectTrigger>
                  <SelectValue placeholder={t("Select a user")} />
                </SelectTrigger>
                <SelectContent>
                  {availableUsers.map((user) => (
                    <SelectItem key={user.id} value={user.id}>
                      {user.name} ({user.email})
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>

            <div className="space-y-2">
              <Label>{t("Role")}</Label>
              <Select
                value={selectedRole}
                onValueChange={(value) => setSelectedRole(value as "owner" | "member")}
              >
                <SelectTrigger>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="owner">{t("Owner")}</SelectItem>
                  <SelectItem value="member">{t("Member")}</SelectItem>
                </SelectContent>
              </Select>
            </div>

            <DialogFooter>
              <Button type="button" variant="secondary" onClick={() => setCreateOpen(false)}>
                {t("Cancel")}
              </Button>
              <Button type="submit" disabled={isCreating || !selectedUserId}>
                {isCreating ? "Adding..." : "Add Member"}
              </Button>
            </DialogFooter>
          </form>
        </DialogContent>
      </Dialog>
    </SettingsSectionShell>
  );
}
