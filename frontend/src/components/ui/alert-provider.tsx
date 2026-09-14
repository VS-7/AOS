'use client'

import type React from 'react'
import { createContext, useCallback, useContext, useEffect, useState } from 'react'
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle
} from './alert-dialog'
import { Button } from './button'
import { setUnsavedPromptHandler } from '@/lib/unsaved-prompt.bridge'
import { t } from '@/lib/i18n'

interface AlertOptions {
  title?: string
  description?: string
  confirmText?: string
  cancelText?: string
  variant?: 'default' | 'destructive'
  onConfirm?: () => void | Promise<void>
  onCancel?: () => void
}

interface UnsavedPromptOptions {
  title?: string
  description?: string
  saveText?: string
  discardText?: string
  cancelText?: string
}

type UnsavedPromptResult = 'save' | 'discard' | 'cancel'

interface AlertContextType {
  confirm: (options: AlertOptions) => Promise<boolean>
  alert: (title: string, description?: string) => Promise<void>
  promptUnsaved: (options?: UnsavedPromptOptions) => Promise<UnsavedPromptResult>
}

const AlertContext = createContext<AlertContextType | undefined>(undefined)

interface AlertProviderProps {
  children: React.ReactNode
}

type AlertQueueItem =
  | {
      id: string
      type: 'confirm' | 'alert'
      options: AlertOptions
      resolve: (value: boolean | undefined) => void
    }
  | {
      id: string
      type: 'unsaved'
      options: UnsavedPromptOptions
      resolve: (value: UnsavedPromptResult) => void
    }

export function AlertProvider({ children }: AlertProviderProps) {
  const [alerts, setAlerts] = useState<AlertQueueItem[]>([])

  const confirm = useCallback((options: AlertOptions): Promise<boolean> => {
    return new Promise((resolve) => {
      const id = Math.random().toString(36).substr(2, 9)
      setAlerts((prev) => [
        ...prev,
        {
          id,
          type: 'confirm',
          options,
          resolve: resolve as (value: boolean | undefined) => void
        }
      ])
    })
  }, [])

  const alert = useCallback(
    (title: string, description?: string): Promise<void> => {
      return new Promise((resolve) => {
        const id = Math.random().toString(36).substr(2, 9)
        setAlerts((prev) => [
          ...prev,
          {
            id,
            type: 'alert',
            options: { title, description },
            resolve: resolve as (value: boolean | undefined) => void
          }
        ])
      })
    },
    []
  )

  const promptUnsaved = useCallback(
    (options: UnsavedPromptOptions = {}): Promise<UnsavedPromptResult> => {
      return new Promise((resolve) => {
        const id = Math.random().toString(36).substr(2, 9)
        setAlerts((prev) => [
          ...prev,
          {
            id,
            type: 'unsaved',
            options,
            resolve,
          },
        ])
      })
    },
    [],
  )

  const handleConfirm = useCallback(
    async (id: string, options: AlertOptions) => {
      const alertItem = alerts.find((a) => a.id === id)
      if (!alertItem || alertItem.type === 'unsaved') return

      try {
        if (options.onConfirm) {
          await options.onConfirm()
        }
        alertItem.resolve(true)
      } catch (error) {
        console.error('Error in alert confirm:', error)
        alertItem.resolve(false)
      }

      setAlerts((prev) => prev.filter((a) => a.id !== id))
    },
    [alerts]
  )

  const handleCancel = useCallback(
    (id: string, options: AlertOptions) => {
      const alertItem = alerts.find((a) => a.id === id)
      if (!alertItem || alertItem.type === 'unsaved') return

      if (options.onCancel) {
        options.onCancel()
      }
      alertItem.resolve(false)
      setAlerts((prev) => prev.filter((a) => a.id !== id))
    },
    [alerts]
  )

  const handleAlertClose = useCallback(
    (id: string) => {
      const alertItem = alerts.find((a) => a.id === id)
      if (!alertItem || alertItem.type === 'unsaved') return

      // @ts-expect-error - Expected
      alertItem.resolve()
      setAlerts((prev) => prev.filter((a) => a.id !== id))
    },
    [alerts]
  )

  const handleUnsaved = useCallback((id: string, result: UnsavedPromptResult) => {
    const alertItem = alerts.find((a) => a.id === id)
    if (!alertItem || alertItem.type !== 'unsaved') return
    alertItem.resolve(result)
    setAlerts((prev) => prev.filter((a) => a.id !== id))
  }, [alerts])

  useEffect(() => {
    setUnsavedPromptHandler(promptUnsaved)
    return () => setUnsavedPromptHandler(null)
  }, [promptUnsaved])

  return (
    <AlertContext.Provider value={{ confirm, alert, promptUnsaved }}>
      {children}

      {alerts.map((item) => {
        if (item.type === 'unsaved') {
          const { id, options } = item
          return (
            <AlertDialog key={id} open={true}>
              <AlertDialogContent>
                <AlertDialogHeader>
                  <AlertDialogTitle>
                    {options.title || t('Unsaved changes')}
                  </AlertDialogTitle>
                  <AlertDialogDescription>
                    {options.description ||
                      t('Do you want to save your changes before closing?')}
                  </AlertDialogDescription>
                </AlertDialogHeader>
                <AlertDialogFooter>
                  <AlertDialogCancel asChild>
                    <Button
                      variant='outline'
                      onClick={() => handleUnsaved(id, 'cancel')}
                    >
                      {options.cancelText || t('Cancel')}
                    </Button>
                  </AlertDialogCancel>
                  <Button
                    variant='outline'
                    onClick={() => handleUnsaved(id, 'discard')}
                  >
                    {options.discardText || t("Don't Save")}
                  </Button>
                  <AlertDialogAction asChild>
                    <Button onClick={() => handleUnsaved(id, 'save')}>
                      {options.saveText || t('Save')}
                    </Button>
                  </AlertDialogAction>
                </AlertDialogFooter>
              </AlertDialogContent>
            </AlertDialog>
          )
        }

        const { id, type, options } = item
        return (
          <AlertDialog key={id} open={true}>
            <AlertDialogContent>
              <AlertDialogHeader>
                <AlertDialogTitle>
                  {options.title ||
                    (type === 'confirm' ? t('Confirm Action') : t('Alert'))}
                </AlertDialogTitle>
                {options.description && (
                  <AlertDialogDescription>
                    {options.description}
                  </AlertDialogDescription>
                )}
              </AlertDialogHeader>

              <AlertDialogFooter>
                {type === 'confirm' ? (
                  <>
                    <AlertDialogCancel onClick={() => handleCancel(id, options)}>
                      {options.cancelText || t('Cancel')}
                    </AlertDialogCancel>
                    {/* The variant on the action itself: given to a Button
                        inside it, AlertDialogAction's own default-variant
                        Button won the class merge, and every destructive
                        confirmation drew a solid black button. */}
                    <AlertDialogAction
                      variant={options.variant || 'default'}
                      onClick={() => handleConfirm(id, options)}
                    >
                      {options.confirmText || t('Confirm')}
                    </AlertDialogAction>
                  </>
                ) : (
                  <AlertDialogAction asChild>
                    <Button onClick={() => handleAlertClose(id)}>{t('OK')}</Button>
                  </AlertDialogAction>
                )}
              </AlertDialogFooter>
            </AlertDialogContent>
          </AlertDialog>
        )
      })}
    </AlertContext.Provider>
  )
}

export function useAlert() {
  const context = useContext(AlertContext)

  if (context === undefined) {
    throw new Error('useAlert must be used within an AlertProvider')
  }

  return context
}
