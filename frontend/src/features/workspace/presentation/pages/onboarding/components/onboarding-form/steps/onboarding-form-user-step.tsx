import { t } from "@/lib/i18n";
'use client'

import { User, Mail } from 'lucide-react'
import { UseFormReturn } from 'react-hook-form'
import { FormField, FormItem, FormLabel, FormControl, FormMessage } from '@/components/ui/form'
import { Input } from '@/components/ui/input'

interface OnboardingFormUserStepProps {
  form: UseFormReturn<any>
}

export function OnboardingFormUserStep({ form }: OnboardingFormUserStepProps) {
  return (
    <div className="flex flex-col items-center justify-center h-full px-8 py-8">
      <div className="w-full max-w-md space-y-6">
        <div>
          <h2 className="text-xl font-semibold tracking-tight text-foreground">{t("A bit about you")}</h2>
          <p className="text-sm text-muted-foreground leading-relaxed mt-1">
            {t("This helps your copilot identify you and personalize your local workspace.")}
          </p>
        </div>

        <div className="space-y-4">
          <FormField
            control={form.control}
            name="user.name"
            render={({ field }) => (
              <FormItem className="space-y-2">
                <FormLabel className="text-xs font-medium uppercase tracking-wider text-muted-foreground">{t("Full Name")}</FormLabel>
                <FormControl>
                  <div className="relative">
                    <User className="w-4 h-4 text-muted-foreground/60 absolute left-3.5 top-1/2 -translate-y-1/2" />
                    <Input
                      placeholder={t("e.g. Felipe Barcelos")}
                      className="pl-10 h-11 bg-muted/20 border-border/60 rounded-xl focus-visible:ring-primary/20 transition-all"
                      autoFocus
                      {...field}
                    />
                  </div>
                </FormControl>
                <FormMessage />
              </FormItem>
            )}
          />

          <FormField
            control={form.control}
            name="user.email"
            render={({ field }) => (
              <FormItem className="space-y-2">
                <FormLabel className="text-xs font-medium uppercase tracking-wider text-muted-foreground">{t("Email Address")}</FormLabel>
                <FormControl>
                  <div className="relative">
                    <Mail className="w-4 h-4 text-muted-foreground/60 absolute left-3.5 top-1/2 -translate-y-1/2" />
                    <Input
                      type="email"
                      placeholder={t("you@example.com")}
                      className="pl-10 h-11 bg-muted/20 border-border/60 rounded-xl focus-visible:ring-primary/20 transition-all"
                      {...field}
                    />
                  </div>
                </FormControl>
                <p className="text-[11px] text-muted-foreground/80 leading-normal pl-1">
                  {t("Stored locally for workspace session identity and local commits.")}
                </p>
                <FormMessage />
              </FormItem>
            )}
          />
        </div>
      </div>
    </div>
  )
}
