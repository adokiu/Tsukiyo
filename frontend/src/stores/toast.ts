import { create } from 'zustand'

export type ToastType = 'success' | 'error' | 'warning' | 'info'

export interface Toast {
  id: string
  type: ToastType
  message: string
  duration?: number
}

interface ToastState {
  toasts: Toast[]
  add: (type: ToastType, message: string, duration?: number) => void
  remove: (id: string) => void
  success: (message: string) => void
  error: (message: string) => void
  warning: (message: string) => void
  info: (message: string) => void
}

let counter = 0

export const useToastStore = create<ToastState>((set, get) => ({
  toasts: [],

  add: (type, message, duration = 4000) => {
    const id = `toast_${++counter}_${Date.now()}`
    const toast: Toast = { id, type, message, duration }
    set((s) => ({ toasts: [...s.toasts, toast] }))

    if (duration > 0) {
      setTimeout(() => get().remove(id), duration)
    }
  },

  remove: (id) => {
    set((s) => ({ toasts: s.toasts.filter((t) => t.id !== id) }))
  },

  success: (msg) => get().add('success', msg),
  error: (msg) => get().add('error', msg, 6000),
  warning: (msg) => get().add('warning', msg),
  info: (msg) => get().add('info', msg),
}))
