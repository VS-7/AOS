"use client";

import { useEffect, useState } from "react";
import { ArrowRightIcon } from "lucide-react";
import { Dialog, DialogContent, DialogTitle } from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";
import { Logo } from "@/components/ui/logo";
import { useRouter, useRouterState } from "@tanstack/react-router";
import { t } from "@/lib/i18n";

export function WelcomeDialog() {
  const router = useRouter();
  const location = useRouterState({ select: (state) => state.location });

  const [open, setOpen] = useState(false);

  const isWelcomeSearchEnabled = (() => {
    const params = new URLSearchParams(location.searchStr);
    return params.get("welcome") === "true";
  })();

  useEffect(() => {
    setOpen(isWelcomeSearchEnabled);
  }, [isWelcomeSearchEnabled]);

  function handleClose() {
    setOpen(false);

    const params = new URLSearchParams(location.searchStr);
    params.delete("welcome");

    const nextQuery = params.toString();
    const nextPath = nextQuery
      ? `${location.pathname}?${nextQuery}`
      : location.pathname;

    router.history.replace(nextPath);
  }

  function handleOpenChange(nextOpen: boolean) {
    setOpen(nextOpen);

    if (!nextOpen && isWelcomeSearchEnabled) {
      handleClose();
    }
  }

  return (
    <Dialog open={open} onOpenChange={handleOpenChange}>
      <DialogContent className="sm:max-w-xl">
        <DialogTitle className="sr-only">{t("Welcome to your workspace")}</DialogTitle>
        <Logo className="size-12" />

        <div className="space-y-2 mt-32">
          <h1 className="font-bold text-xl">{t("Welcome to your workspace")}</h1>
          <p>
            {t("Your workspace is ready. From here, you can organize your projects, manage agents, and start shipping with AOS.")}
          </p>
        </div>

        <div className="flex items-center space-x-4">
          <Button onClick={handleClose} variant="outline" className="w-fit">
            {t("I'm ready to start")}
            <ArrowRightIcon className="ml-2" />
          </Button>
        </div>
      </DialogContent>
    </Dialog>
  );
}
