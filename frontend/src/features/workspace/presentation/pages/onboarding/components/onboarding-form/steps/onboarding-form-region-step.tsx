'use client'

import { Clock, MapPin } from 'lucide-react'
import { UseFormReturn } from 'react-hook-form'
import { FormField, FormItem, FormLabel, FormControl, FormMessage } from '@/components/ui/form'
import { Input } from '@/components/ui/input'
import { Select, SelectTrigger, SelectValue, SelectContent, SelectItem } from '@/components/ui/select'

interface OnboardingFormRegionStepProps {
  form: UseFormReturn<any>
}

const LANGUAGES = [
  { value: 'en', label: 'English (US)' },
  { value: 'en-GB', label: 'English (UK)' },
  { value: 'pt-BR', label: 'Português (Brasil)' },
  { value: 'pt', label: 'Português (Portugal)' },
  { value: 'es', label: 'Español' },
  { value: 'fr', label: 'Français' },
  { value: 'de', label: 'Deutsch' },
  { value: 'it', label: 'Italiano' },
  { value: 'ja', label: '日本語' },
  { value: 'ko', label: '한국어' },
  { value: 'zh', label: '中文' },
  { value: 'ru', label: 'Русский' },
]

export function OnboardingFormRegionStep({ form }: OnboardingFormRegionStepProps) {
  return (
    <div className="flex flex-col items-center justify-center h-full px-6 py-10">
      <div className="w-full max-w-md space-y-8">
        <div>
          <h2 className="text-xl font-semibold tracking-tight text-foreground">Language & Region</h2>
          <p className="text-sm text-muted-foreground leading-relaxed mt-1">
            Pick your language and local timezone so your copilot speaks to you naturally.
          </p>
        </div>

        <div className="space-y-5 bg-muted/20 border border-border/40 p-6 rounded-2xl backdrop-blur-sm">
          <FormField
            control={form.control}
            name="region.language"
            render={({ field }) => (
              <FormItem className="space-y-2">
                <FormLabel className="text-xs font-medium uppercase tracking-wider text-muted-foreground">Primary Language</FormLabel>
                <Select onValueChange={field.onChange} value={field.value}>
                  <FormControl>
                    <SelectTrigger className="h-11 bg-background/60 border-border/60 rounded-xl focus:ring-primary/20 transition-all">
                      <SelectValue placeholder="Select primary language" />
                    </SelectTrigger>
                  </FormControl>
                  <SelectContent className="rounded-xl border-border/60 max-h-56">
                    {LANGUAGES.map((lang) => (
                      <SelectItem key={lang.value} value={lang.value} className="rounded-lg text-sm">
                        {lang.label}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
                <FormMessage />
              </FormItem>
            )}
          />

          <FormField
            control={form.control}
            name="region.timezone"
            render={({ field }) => (
              <FormItem className="space-y-2">
                <FormLabel className="text-xs font-medium uppercase tracking-wider text-muted-foreground">Timezone</FormLabel>
                <FormControl>
                  <div className="relative">
                    <Clock className="w-4 h-4 text-muted-foreground/60 absolute left-3.5 top-1/2 -translate-y-1/2" />
                    <Input
                      placeholder="e.g. America/Sao_Paulo"
                      className="pl-10 h-11 bg-background/60 border-border/60 rounded-xl focus-visible:ring-primary/20 transition-all"
                      {...field}
                    />
                  </div>
                </FormControl>
                <FormMessage />
              </FormItem>
            )}
          />

          <div className="grid grid-cols-2 gap-3 pt-1">
            <FormField
              control={form.control}
              name="region.city"
              render={({ field }) => (
                <FormItem className="space-y-2">
                  <FormLabel className="text-xs font-medium uppercase tracking-wider text-muted-foreground">
                    City <span className="normal-case text-muted-foreground/60 font-normal">(optional)</span>
                  </FormLabel>
                  <FormControl>
                    <div className="relative">
                      <MapPin className="w-3.5 h-3.5 text-muted-foreground/60 absolute left-3 top-1/2 -translate-y-1/2" />
                      <Input
                        placeholder="São Paulo"
                        className="pl-8 h-10 bg-background/60 border-border/60 rounded-xl text-sm focus-visible:ring-primary/20 transition-all"
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
              name="region.country"
              render={({ field }) => (
                <FormItem className="space-y-2">
                  <FormLabel className="text-xs font-medium uppercase tracking-wider text-muted-foreground">
                    Country <span className="normal-case text-muted-foreground/60 font-normal">(optional)</span>
                  </FormLabel>
                  <FormControl>
                    <Input
                      placeholder="Brazil"
                      className="h-10 bg-background/60 border-border/60 rounded-xl text-sm focus-visible:ring-primary/20 transition-all"
                      {...field}
                    />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />
          </div>
        </div>
      </div>
    </div>
  )
}
