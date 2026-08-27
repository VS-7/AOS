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
import { Input } from "@/components/ui/input";
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
import { t } from "@/lib/i18n";

export function UserUsersSection() {
  const [createOpen, setCreateOpen] = React.useState(false);
  const [name, setName] = React.useState("");
  const [email, setEmail] = React.useState("");
  const [password, setPassword] = React.useState("");
  const [role, setRole] = React.useState<"member" | "super">("member");
  const [isCreating, setIsCreating] = React.useState(false);

  const usersQuery = api.user.list.useQuery({ query: {} });

  const users = (usersQuery.data as UserPublic[] | undefined) ?? [];

  const refreshUsers = React.useCallback(async () => {
    await usersQuery.refetch();
  }, [usersQuery]);

  const handleCreate = async (event: React.FormEvent) => {
    event.preventDefault();
    setIsCreating(true);
    try {
      const result = await api.user.create.mutate({
        body: { name, email, password, role },
      });

      if (result.error) {
        const message =
          result.error instanceof Error
            ? result.error.message
            : "Failed to create user";
        toast.error(message);
        return;
      }

      toast.success(t("User created successfully"));
      setCreateOpen(false);
      setName("");
      setEmail("");
      setPassword("");
      setRole("member");
      await refreshUsers();
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "Failed to create user");
    } finally {
      setIsCreating(false);
    }
  };

  const handleRoleChange = async (userId: string, nextRole: "member" | "super") => {
    try {
      const result = await api.user.update.mutate({
        params: { user: userId },
        body: { role: nextRole },
      });

      if (result.error) {
        const message =
          result.error instanceof Error
            ? result.error.message
            : "Failed to update user";
        toast.error(message);
        return;
      }

      toast.success(t("User updated"));
      await refreshUsers();
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "Failed to update user");
    }
  };

  const handleDelete = async (userId: string) => {
    try {
      const result = await api.user.delete.mutate({ params: { user: userId } });

      if (result.error) {
        const message =
          result.error instanceof Error
            ? result.error.message
            : "Failed to delete user";
        toast.error(message);
        return;
      }

      toast.success(t("User deleted"));
      await refreshUsers();
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "Failed to delete user");
    }
  };

  return (
    <SettingsSectionShell>
      <FormSection>
        <FormSectionHeader>
          <FormSectionTitle>{t("Users")}</FormSectionTitle>
          <FormSectionDescription>
            {t("Manage accounts that can sign in to this AOS installation.")}
          </FormSectionDescription>
        </FormSectionHeader>

        <FormSectionContent>
          <FormSectionItem>
            <div className="min-w-0">
              <p className="text-sm font-medium text-foreground">{t("Accounts")}</p>
              <p className="text-sm text-muted-foreground">
                {t("Super users can create, update, and remove instance accounts.")}
              </p>
            </div>
            <Button type="button" size="sm" onClick={() => setCreateOpen(true)}>
              <HugeiconsIcon icon={Add01Icon} className="size-3.5" />
              {t("Add User")}
            </Button>
          </FormSectionItem>

          <div className="divide-y divide-border">
            {usersQuery.isLoading ? (
              <p className="p-4 text-sm text-muted-foreground">{t("Loading users...")}</p>
            ) : users.length === 0 ? (
              <p className="p-4 text-sm text-muted-foreground">{t("No users found.")}</p>
            ) : (
              users.map((user) => (
                <div
                  key={user.id}
                  className="flex flex-wrap items-center justify-between gap-3 p-4"
                >
                  <div className="min-w-0">
                    <p className="text-sm font-medium text-foreground">{user.name}</p>
                    <p className="text-sm text-muted-foreground">{user.email}</p>
                  </div>

                  <div className="flex items-center gap-2">
                    <Select
                      value={user.role}
                      onValueChange={(value) =>
                        void handleRoleChange(user.id, value as "member" | "super")
                      }
                    >
                      <SelectTrigger className="h-8 w-[120px]">
                        <SelectValue />
                      </SelectTrigger>
                      <SelectContent>
                        <SelectItem value="member">{t("Member")}</SelectItem>
                        <SelectItem value="super">{t("Super")}</SelectItem>
                      </SelectContent>
                    </Select>

                    <AlertDialog>
                      <AlertDialogTrigger asChild>
                        <Button
                          type="button"
                          variant="ghost"
                          size="icon"
                          className="size-8 text-destructive"
                          aria-label={`Delete ${user.name}`}
                        >
                          <HugeiconsIcon icon={Delete02Icon} className="size-3.5" />
                        </Button>
                      </AlertDialogTrigger>
                      <AlertDialogContent size="sm">
                        <AlertDialogHeader>
                          <AlertDialogTitle>{t("Delete user?")}</AlertDialogTitle>
                          <AlertDialogDescription>
                            {t("This permanently removes")} {user.email} {t("from this instance.")}
                          </AlertDialogDescription>
                        </AlertDialogHeader>
                        <AlertDialogFooter>
                          <AlertDialogCancel>{t("Cancel")}</AlertDialogCancel>
                          <AlertDialogAction
                            variant="destructive"
                            onClick={() => void handleDelete(user.id)}
                          >
                            {t("Delete")}
                          </AlertDialogAction>
                        </AlertDialogFooter>
                      </AlertDialogContent>
                    </AlertDialog>
                  </div>
                </div>
              ))
            )}
          </div>
        </FormSectionContent>
      </FormSection>

      <Dialog open={createOpen} onOpenChange={setCreateOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{t("Add User")}</DialogTitle>
            <DialogDescription>
              {t("Create a new account for this AOS instance.")}
            </DialogDescription>
          </DialogHeader>

          <form onSubmit={(event) => void handleCreate(event)} className="space-y-4">
            <div className="space-y-2">
              <Label htmlFor="user-name">{t("Name")}</Label>
              <Input
                id="user-name"
                value={name}
                onChange={(event) => setName(event.target.value)}
                required
              />
            </div>

            <div className="space-y-2">
              <Label htmlFor="user-email">{t("Email")}</Label>
              <Input
                id="user-email"
                type="email"
                value={email}
                onChange={(event) => setEmail(event.target.value)}
                required
              />
            </div>

            <div className="space-y-2">
              <Label htmlFor="user-password">{t("Password")}</Label>
              <Input
                id="user-password"
                type="password"
                minLength={6}
                value={password}
                onChange={(event) => setPassword(event.target.value)}
                required
              />
            </div>

            <div className="space-y-2">
              <Label>{t("Role")}</Label>
              <Select
                value={role}
                onValueChange={(value) => setRole(value as "member" | "super")}
              >
                <SelectTrigger>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="member">{t("Member")}</SelectItem>
                  <SelectItem value="super">{t("Super")}</SelectItem>
                </SelectContent>
              </Select>
            </div>

            <DialogFooter>
              <Button type="button" variant="secondary" onClick={() => setCreateOpen(false)}>
                {t("Cancel")}
              </Button>
              <Button type="submit" disabled={isCreating}>
                {isCreating ? "Creating..." : "Create User"}
              </Button>
            </DialogFooter>
          </form>
        </DialogContent>
      </Dialog>
    </SettingsSectionShell>
  );
}
