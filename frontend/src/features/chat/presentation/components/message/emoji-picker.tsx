import * as React from "react";
import { EmojiPicker } from "frimousse";
import { Button } from "@/components/ui/button";
import { getLocale, t, useTranslation } from "@/lib/i18n";
import { cn } from "@/lib/utils";

/** The emojibase-data release the application ships, under `public/`. */
export const EMOJIBASE_VERSION = "17.0.0";

/**
 * Where the picker reads its emoji data: the application's own assets.
 *
 * Without it frimousse fetched `emojibase-data@latest` from cdn.jsdelivr.net
 * every time its cache was empty — unpinned, so the data could change under
 * the picker, and unreachable offline or behind a firewall, where the picker
 * said "Loading emojis..." forever and the only trace was a console error. The
 * data is copied into `public/assets/emojibase/<version>/` so it ships inside
 * `dist/assets`, which both the desktop binary and the daemon embed.
 */
export const EMOJIBASE_URL = `/assets/emojibase/${EMOJIBASE_VERSION}`;

/** The emojibase locale for the interface's language: names and search follow it. */
function emojibaseLocale(): "en" | "pt" {
  return getLocale() === "pt-BR" ? "pt" : "en";
}

type LoadState = "checking" | "ready" | "failed";

/**
 * A full emoji picker, or a message saying it cannot be one.
 *
 * frimousse swallows a failed load (it logs and keeps showing its loading
 * state), so the data is read once here first. It is a small file from the
 * application's own assets and the browser serves the picker's own read of it
 * from cache.
 */
export function ChatEmojiPicker({
  onSelectEmoji,
}: {
  onSelectEmoji: (emoji: string) => void;
}) {
  useTranslation();
  const locale = emojibaseLocale();
  const [state, setState] = React.useState<LoadState>("checking");
  const [attempt, setAttempt] = React.useState(0);

  React.useEffect(() => {
    let cancelled = false;
    setState("checking");
    fetch(`${EMOJIBASE_URL}/${locale}/messages.json`)
      .then((response) => {
        if (!cancelled) setState(response.ok ? "ready" : "failed");
      })
      .catch(() => {
        if (!cancelled) setState("failed");
      });
    return () => {
      cancelled = true;
    };
  }, [attempt, locale]);

  if (state === "failed") {
    return (
      <div className="flex h-[25rem] w-full flex-col items-center justify-center gap-3 p-6 text-center">
        <p className="text-sm text-muted-foreground">{t("The emoji list could not be loaded.")}</p>
        <Button type="button" size="sm" variant="outline" onClick={() => setAttempt((n) => n + 1)}>
          {t("Try again")}
        </Button>
      </div>
    );
  }

  if (state === "checking") {
    return (
      <div className="flex h-[25rem] w-full items-center justify-center text-sm text-muted-foreground">
        {t("Loading emojis...")}
      </div>
    );
  }

  return (
    <EmojiPicker.Root
      className="isolate flex h-[25rem] w-full flex-col bg-popover text-popover-foreground"
      emojibaseUrl={EMOJIBASE_URL}
      locale={locale}
      onEmojiSelect={({ emoji }: { emoji: string }) => onSelectEmoji(emoji)}
    >
      <div className="border-b border-border/70 p-3">
        <EmojiPicker.Search
          className="h-9 w-full rounded-lg border border-border/70 bg-background px-3 text-sm outline-hidden transition-colors placeholder:text-muted-foreground focus:border-ring"
          placeholder={t("Search emoji")}
        />
      </div>

      <EmojiPicker.Viewport className="relative flex-1 outline-hidden">
        <EmojiPicker.Loading className="absolute inset-0 flex items-center justify-center text-sm text-muted-foreground">
          {t("Loading emojis...")}
        </EmojiPicker.Loading>
        <EmojiPicker.Empty className="absolute inset-0 flex items-center justify-center text-sm text-muted-foreground">
          {({ search }: { search: string }) => t("No emoji found for “{{search}}”", { search })}
        </EmojiPicker.Empty>
        <EmojiPicker.List
          className="pb-3"
          components={{
            CategoryHeader: ({ category, ...props }: any) => (
              <div
                className="bg-popover/95 px-3 py-2 text-[10px] font-semibold tracking-[0.2em] text-muted-foreground uppercase backdrop-blur-sm"
                {...props}
              >
                {category.label}
              </div>
            ),
            Row: ({ children, ...props }: any) => (
              <div className="grid grid-cols-8 gap-1 px-2 py-0.5" {...props}>
                {children}
              </div>
            ),
            Emoji: ({ emoji, ...props }: any) => (
              <button
                className={cn(
                  "flex size-9 items-center justify-center rounded-lg text-lg transition-colors outline-hidden",
                  emoji.isActive ? "bg-muted" : "hover:bg-muted/70",
                )}
                {...props}
              >
                {emoji.emoji}
              </button>
            ),
          }}
        />
      </EmojiPicker.Viewport>
    </EmojiPicker.Root>
  );
}
