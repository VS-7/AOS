import { api } from "@/lib/aos-facade";
import { Badge } from "@/components/ui/badge";
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

/**
 * The accounts that can sign in to this installation.
 *
 * Read-only, on purpose. The roster is published (`/api/auth/users`), but
 * nothing publishes creating an account, changing its role or removing it —
 * the identity domain has those operations and no surface exposes them yet
 * (see `user.*` in lib/command-map.ts). The section used to sit behind the
 * developer panel "Domain not available yet — the Go backend does not publish
 * this domain", hiding a roster that was there to read; ungated, it offered an
 * Add User form and role and delete controls that could only fail. It shows
 * what is true instead: who has an account, and that changing that is not
 * done from here.
 */
export function UserUsersSection() {
  const usersQuery = api.user.list.useQuery({ query: {} });

  // `{users: [...]}`, not a bare list: reading it as an array made every
  // render of a populated roster throw.
  const users = (usersQuery.data as { users?: UserPublic[] } | undefined)?.users ?? [];

  return (
    <SettingsSectionShell>
      <FormSection>
        <FormSectionHeader>
          <FormSectionTitle>{t("Users")}</FormSectionTitle>
          <FormSectionDescription>
            {t("Accounts that can sign in to this AOS installation.")}
          </FormSectionDescription>
        </FormSectionHeader>

        <FormSectionContent>
          <FormSectionItem>
            <p className="text-sm text-muted-foreground">
              {t("The first account is created during onboarding. Accounts cannot be added, changed or removed from here in this version.")}
            </p>
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
                  <Badge variant="outline">
                    {user.role === "super" ? t("Super") : t("Member")}
                  </Badge>
                </div>
              ))
            )}
          </div>
        </FormSectionContent>
      </FormSection>
    </SettingsSectionShell>
  );
}
