import { t } from "@/lib/i18n";
'use client'

import { Eye, EyeOff, Lock, CheckCircle2 } from 'lucide-react'
import { useState } from 'react'
import { UseFormReturn } from 'react-hook-form'
import { FormField, FormItem, FormLabel, FormControl, FormMessage } from '@/components/ui/form'
import { Input } from '@/components/ui/input'
import { Button } from '@/components/ui/button'
import { cn } from '@/lib/utils'

interface OnboardingFormSecurityStepProps {
  form: UseFormReturn<any>
}

interface Requirement {
  id: string
  label: string
  validator: (pass: string) => boolean
}

const REQUIREMENTS: Requirement[] = [
  { id: 'minLen', label: '8+ chars', validator: (p) => p.length >= 8 },
  { id: 'upper', label: 'Uppercase', validator: (p) => /[A-Z]/.test(p) },
  { id: 'number', label: 'Number', validator: (p) => /[0-9]/.test(p) },
  { id: 'symbol', label: 'Symbol', validator: (p) => /[^A-Za-z0-9]/.test(p) },
]

function getPasswordStrength(password: string) {
  if (!password) return { label: 'Enter a password', score: 0, color: 'bg-muted' }

  let score = 0
  if (password.length >= 6) score++
  if (password.length >= 8) score++
  if (/[A-Z]/.test(password)) score++
  if (/[0-9]/.test(password)) score++
  if (/[^A-Za-z0-9]/.test(password)) score++

  if (score <= 1) return { label: 'Weak', score: 1, color: 'bg-red-500' }
  if (score <= 2) return { label: 'Fair', score: 2, color: 'bg-amber-500' }
  if (score <= 3) return { label: 'Good', score: 3, color: 'bg-yellow-500' }
  if (score <= 4) return { label: 'Strong', score: 4, color: 'bg-green-500' }
  return { label: 'Ultra Secure', score: 5, color: 'bg-emerald-400' }
}

export function OnboardingFormSecurityStep({ form }: OnboardingFormSecurityStepProps) {
  const [showPassword, setShowPassword] = useState(false)
  const password: string = form.watch('security.password') || ''
  const strength = getPasswordStrength(password)

  return (
    <div className="flex flex-col items-center justify-center h-full px-8 py-8">
      <div className="w-full max-w-md space-y-6">
        <div>
          <h2 className="text-xl font-semibold tracking-tight text-foreground">{t("Your space, your security")}</h2>
          <p className="text-sm text-muted-foreground leading-relaxed mt-1">
            {t("Set a master password to encrypt your local workspace, API credentials, and sessions.")}
          </p>
        </div>

        <div className="space-y-4">
          <FormField
            control={form.control}
            name="security.password"
            render={({ field }) => (
              <FormItem className="space-y-2">
                <FormLabel className="text-xs font-medium uppercase tracking-wider text-muted-foreground">{t("Master Password")}</FormLabel>
                <FormControl>
                  <div className="relative">
                    <Lock className="w-4 h-4 text-muted-foreground/60 absolute left-3.5 top-1/2 -translate-y-1/2" />
                    <Input
                      type={showPassword ? 'text' : 'password'}
                      placeholder={t("Enter a strong master password")}
                      className="pl-10 pr-10 h-11 bg-muted/20 border-border/60 rounded-xl focus-visible:ring-primary/20 transition-all"
                      autoFocus
                      {...field}
                    />
                    <Button
                      type="button"
                      variant="ghost"
                      size="icon"
                      className="absolute right-1 top-1/2 -translate-y-1/2 h-8 w-8 text-muted-foreground hover:text-foreground hover:bg-transparent"
                      onClick={() => setShowPassword((s) => !s)}
                      tabIndex={-1}
                    >
                      {showPassword ? (
                        <EyeOff className="w-4 h-4" />
                      ) : (
                        <Eye className="w-4 h-4" />
                      )}
                    </Button>
                  </div>
                </FormControl>
                <FormMessage />
              </FormItem>
            )}
          />

          <div className="space-y-2.5 pt-1">
            <div className="flex items-center justify-between text-xs">
              <span className="text-muted-foreground font-medium">{t("Password Strength")}</span>
              <span className={cn('font-medium transition-colors', password ? 'text-foreground' : 'text-muted-foreground')}>
                {strength.label}
              </span>
            </div>
            <div className="flex gap-1.5">
              {Array.from({ length: 5 }).map((_, i) => (
                <div
                  key={i}
                  className={cn(
                    'h-1.5 flex-1 rounded-full transition-all duration-300',
                    i < strength.score ? strength.color : 'bg-muted/50'
                  )}
                />
              ))}
            </div>
          </div>

          <div className="flex flex-wrap gap-2 pt-1">
            {REQUIREMENTS.map((req) => {
              const isMet = req.validator(password)
              return (
                <span
                  key={req.id}
                  className={cn(
                    'inline-flex items-center gap-1.5 text-xs px-2.5 py-1 rounded-lg border transition-all duration-200',
                    isMet
                      ? 'bg-emerald-500/10 text-emerald-600 dark:text-emerald-400 border-emerald-500/20 font-medium'
                      : 'bg-background/40 text-muted-foreground/70 border-border/40'
                  )}
                >
                  <CheckCircle2 className={cn('w-3.5 h-3.5 transition-colors', isMet ? 'text-emerald-500' : 'text-muted-foreground/30')} />
                  {req.label}
                </span>
              )
            })}
          </div>

          <div className="pt-2 border-t border-border/30 flex items-center gap-2 text-[11px] text-muted-foreground/80">
            <span className="shrink-0">🔒</span>
            <span>{t("Zero-Knowledge — Credentials remain exclusively on your local device.")}</span>
          </div>
        </div>
      </div>
    </div>
  )
}
