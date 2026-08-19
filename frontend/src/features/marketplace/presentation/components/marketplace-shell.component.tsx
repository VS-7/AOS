import type { ReactNode } from "react";

import { Page, PageBody } from "@/components/ui/page";
import { cn } from "@/lib/utils";

/** Shared left-rail width/spacing so list ↔ detail navigation does not shift. */
export function MarketplaceRail({
  children,
  className,
}: {
  children: ReactNode;
  className?: string;
}) {
  return (
    <aside
      className={cn(
        "sticky top-12 z-10 hidden w-44 shrink-0 self-start lg:block xl:w-52",
        className,
      )}
    >
      {children}
    </aside>
  );
}

export function MarketplaceShell({
  rail,
  children,
}: {
  rail: ReactNode;
  children: ReactNode;
}) {
  return (
    <Page className="h-full overflow-hidden">
      <PageBody className="overflow-y-auto [scrollbar-gutter:stable]">
        <div className="mx-auto w-full max-w-6xl px-6 pb-10 pt-4 md:pb-14">
          <div className="flex items-start gap-10 xl:gap-16">
            {rail}
            <div className="flex min-w-0 flex-1 flex-col gap-8 pt-8 md:pt-10">
              {children}
            </div>
          </div>
        </div>
      </PageBody>
    </Page>
  );
}
