import { toast as sonnerToast } from 'vue-sonner'

export interface ToastOptions {
  description?: string
  duration?: number
  action?: {
    label: string
    onClick: () => void
  }
}

/**
 * Composable for displaying toast notifications using vue-sonner
 *
 * Usage:
 * ```ts
 * const { toast } = useToast()
 * toast.success('Operation completed')
 * toast.error('Something went wrong', { description: 'Please try again' })
 * ```
 */
export function useToast() {
  /**
   * Show a success toast
   */
  function success(message: string, options?: ToastOptions) {
    sonnerToast.success(message, {
      description: options?.description,
      duration: options?.duration ?? 4000,
      action: options?.action,
    })
  }

  /**
   * Show an error toast
   */
  function error(message: string, options?: ToastOptions) {
    sonnerToast.error(message, {
      description: options?.description,
      duration: options?.duration ?? 6000,
      action: options?.action,
    })
  }

  /**
   * Show an info toast
   */
  function info(message: string, options?: ToastOptions) {
    sonnerToast.info(message, {
      description: options?.description,
      duration: options?.duration ?? 4000,
      action: options?.action,
    })
  }

  /**
   * Show a warning toast
   */
  function warning(message: string, options?: ToastOptions) {
    sonnerToast.warning(message, {
      description: options?.description,
      duration: options?.duration ?? 5000,
      action: options?.action,
    })
  }

  /**
   * Show a loading toast (dismissible programmatically)
   */
  function loading(message: string, options?: ToastOptions) {
    return sonnerToast.loading(message, {
      description: options?.description,
      duration: options?.duration ?? Infinity,
    })
  }

  /**
   * Dismiss a specific toast or all toasts
   */
  function dismiss(toastId?: string | number) {
    sonnerToast.dismiss(toastId)
  }

  /**
   * Show a promise-based toast
   */
  function promise<T>(
    promise: Promise<T>,
    messages: {
      loading: string
      success: string | ((data: T) => string)
      error: string | ((error: unknown) => string)
    }
  ) {
    return sonnerToast.promise(promise, messages)
  }

  return {
    toast: {
      success,
      error,
      info,
      warning,
      loading,
      dismiss,
      promise,
    },
  }
}

export default useToast
