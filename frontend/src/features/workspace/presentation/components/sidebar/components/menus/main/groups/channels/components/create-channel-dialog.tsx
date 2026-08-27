import * as React from "react";
import { AnimatePresence, motion } from "motion/react";
import { HugeiconsIcon } from "@hugeicons/react";
import {
  FolderAddIcon,
  TextNumberSignIcon,
  Loading03Icon,
} from "@hugeicons/core-free-icons";
import { toast } from "sonner";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { cn } from "@/lib/utils";
import { aos } from "@/app/aos";
import { Slug } from "@/core/helpers/slug.helper";
import { getChannelTitleSuggestions } from "@/features/chat/presentation/consts/title-suggestions";
import { openChatTab } from "@/features/chat/presentation/helpers/open-chat-tab.helper";
import { SidebarGroupAction } from "@/components/ui/sidebar";
import { t } from "@/lib/i18n";

const MAX_CHANNEL_LENGTH = 80;

interface CreateChannelDialogProps {
  onCreated?: () => void;
  /** `icon` for inline headers; `group-action` for absolute SidebarGroupAction. */
  triggerVariant?: "icon" | "group-action";
}

function clampSlug(value: string) {
  return Slug.generate(value).slice(0, MAX_CHANNEL_LENGTH).replace(/-+$/, "");
}

export function CreateChannelDialog({
  onCreated,
  triggerVariant = "icon",
}: CreateChannelDialogProps) {
  const [open, setOpen] = React.useState(false);
  const [channelName, setChannelName] = React.useState("");

  const normalizedName = React.useMemo(
    () => clampSlug(channelName),
    [channelName],
  );
  const suggestions = React.useMemo(
    () => getChannelTitleSuggestions(normalizedName, 3),
    [normalizedName],
  );

  const { mutate: createChat, loading: isCreating } =
    aos.client.chat.create.useMutation({
      onSuccess: (response) => {
        // `onSuccess` receives the full `Envelope` — see `aos-facade.ts`'s
        // `useMutation` doc comment.
        const chat = response?.data?.chat;

        if (!chat) {
          toast.error(t("Unable to create channel."));
          return;
        }

        setChannelName("");
        setOpen(false);
        onCreated?.();
        openChatTab({
          chatId: chat.id,
          title: chat.title || chat.id,
        });
        toast.success(t("Channel created."));
      },
      onError: (error: any) => {
        toast.error(
          error?.error?.message ||
            error?.message ||
            "Unable to create channel.",
        );
      },
    });

  function handleOpenChange(nextOpen: boolean) {
    setOpen(nextOpen);

    if (!nextOpen) {
      setChannelName("");
    }
  }

  function handleSubmit(event?: React.FormEvent<HTMLFormElement>) {
    event?.preventDefault();

    if (!normalizedName) {
      return;
    }

    createChat({
      body: {
        title: normalizedName,
      },
    });
  }

  function applySuggestion(name: string) {
    setChannelName(name);
  }

  return (
    <Dialog open={open} onOpenChange={handleOpenChange}>
      <DialogTrigger asChild>
        {triggerVariant === "group-action" ? (
          <SidebarGroupAction
            className="mr-1.5"
            aria-label={t("Create channel")}
            title={t("Create channel")}
          >
            <HugeiconsIcon icon={FolderAddIcon} className="size-4" />
          </SidebarGroupAction>
        ) : (
          <Button
            type="button"
            size="icon-xs"
            variant="ghost"
            className="text-sidebar-foreground/60 hover:text-sidebar-foreground"
            aria-label={t("Create channel")}
            title={t("Create channel")}
          >
            <HugeiconsIcon icon={FolderAddIcon} className="size-3.5" />
          </Button>
        )}
      </DialogTrigger>

      <DialogContent className="gap-0 overflow-hidden border-border/80 bg-background p-0 sm:max-w-xl">
        <motion.div
          initial={{ opacity: 0, y: 10, scale: 0.985 }}
          animate={{ opacity: 1, y: 0, scale: 1 }}
          transition={{ duration: 0.22, ease: "easeOut" }}
        >
          <DialogHeader className="border-b px-6 py-5 text-left">
            <motion.div
              initial={{ opacity: 0, y: -6 }}
              animate={{ opacity: 1, y: 0 }}
              transition={{ duration: 0.18, delay: 0.04, ease: "easeOut" }}
            >
              <DialogTitle>{t("Create channel")}</DialogTitle>
            </motion.div>
            <motion.div
              initial={{ opacity: 0, y: -4 }}
              animate={{ opacity: 1, y: 0 }}
              transition={{ duration: 0.18, delay: 0.08, ease: "easeOut" }}
            >
              <DialogDescription>
                {t("Choose a short, readable name. We'll slugify it automatically for consistency.")}
              </DialogDescription>
            </motion.div>
          </DialogHeader>

          <form className="flex flex-col" onSubmit={handleSubmit}>
            <motion.div
              className="space-y-3 px-6 py-5"
              initial={{ opacity: 0, y: 8 }}
              animate={{ opacity: 1, y: 0 }}
              transition={{ duration: 0.2, delay: 0.08, ease: "easeOut" }}
            >
              <label
                className="text-sm font-medium text-foreground"
                htmlFor="channel-name"
              >
                {t("Name")}
              </label>

              <motion.div
                layout
                whileFocus={{ scale: 1.005 }}
                className={cn(
                  "flex items-center gap-3 rounded-md border bg-card px-4 py-3 shadow-xs transition-colors",
                  "focus-within:border-ring focus-within:ring-1 focus-within:ring-ring/50",
                )}
              >
                <motion.div
                  whileHover={{ rotate: -8 }}
                  transition={{ duration: 0.16 }}
                >
                  <HugeiconsIcon
                    icon={TextNumberSignIcon}
                    className="size-4 shrink-0 text-muted-foreground"
                  />
                </motion.div>
                <Input
                  id="channel-name"
                  autoFocus
                  value={channelName}
                  onChange={(event) =>
                    setChannelName(clampSlug(event.target.value))
                  }
                  placeholder={t("For example, plano-orcamento")}
                  maxLength={MAX_CHANNEL_LENGTH}
                  disabled={isCreating}
                  className="h-auto border-0 bg-transparent px-0 py-0 rounded-md text-base shadow-none focus-visible:ring-0"
                />
                <motion.span
                  key={normalizedName.length}
                  initial={{ opacity: 0.5, scale: 0.92 }}
                  animate={{ opacity: 1, scale: 1 }}
                  transition={{ duration: 0.16, ease: "easeOut" }}
                  className="shrink-0 text-sm text-muted-foreground"
                >
                  {normalizedName.length}
                </motion.span>
              </motion.div>

              <AnimatePresence initial={false}>
                {suggestions.length > 0 ? (
                  <motion.div
                    key="suggestions"
                    initial={{ opacity: 0, y: -6, height: 0 }}
                    animate={{ opacity: 1, y: 0, height: "auto" }}
                    exit={{ opacity: 0, y: -4, height: 0 }}
                    transition={{ duration: 0.18, ease: "easeOut" }}
                    className="overflow-hidden rounded-md border bg-card/70 py-2 shadow-xs"
                  >
                    {suggestions.map((suggestion, index) => (
                      <motion.button
                        key={suggestion.id}
                        type="button"
                        onClick={() => applySuggestion(suggestion.name)}
                        initial={{ opacity: 0, x: -6 }}
                        animate={{ opacity: 1, x: 0 }}
                        transition={{
                          duration: 0.16,
                          delay: index * 0.03,
                          ease: "easeOut",
                        }}
                        whileHover={{
                          x: 3,
                          backgroundColor: "rgba(255,255,255,0.03)",
                        }}
                        whileTap={{ scale: 0.995 }}
                        className="flex w-full items-start gap-3 px-4 py-2 text-left transition-colors"
                      >
                        <span className="mt-0.5 rounded-md bg-secondary px-1.5 py-0.5 font-mono text-[11px] text-secondary-foreground">
                          {suggestion.name}
                        </span>
                        <span className="min-w-0 text-sm text-muted-foreground">
                          <span className="text-foreground">
                            {suggestion.name}
                          </span>
                          {" - "}
                          {suggestion.description}
                        </span>
                      </motion.button>
                    ))}
                  </motion.div>
                ) : null}
              </AnimatePresence>
            </motion.div>

            <motion.div
              className="flex items-center justify-between border-t bg-muted/20 px-6 py-4"
              initial={{ opacity: 0, y: 8 }}
              animate={{ opacity: 1, y: 0 }}
              transition={{ duration: 0.2, delay: 0.12, ease: "easeOut" }}
            >
              <motion.p
                key={normalizedName || "empty"}
                initial={{ opacity: 0.65, y: 3 }}
                animate={{ opacity: 1, y: 0 }}
                transition={{ duration: 0.16, ease: "easeOut" }}
                className="text-xs text-muted-foreground"
              >
                {t("Channel URL:")}{" "}
                {normalizedName ? `#${normalizedName}` : "waiting for a name"}
              </motion.p>

              <div className="flex items-center gap-2">
                <motion.div whileHover={{ y: -1 }} whileTap={{ y: 0 }}>
                  <Button
                    type="button"
                    variant="ghost"
                    size="sm"
                    onClick={() => handleOpenChange(false)}
                  >
                    {t("Cancel")}
                  </Button>
                </motion.div>
                <motion.div whileHover={{ y: -1 }} whileTap={{ y: 0 }}>
                  <Button
                    type="submit"
                    size="sm"
                    disabled={isCreating || !normalizedName}
                  >
                    {isCreating ? (
                      <HugeiconsIcon
                        icon={Loading03Icon}
                        className="size-4 animate-spin"
                      />
                    ) : null}
                    {t("Create")}
                  </Button>
                </motion.div>
              </div>
            </motion.div>
          </form>
        </motion.div>
      </DialogContent>
    </Dialog>
  );
}
