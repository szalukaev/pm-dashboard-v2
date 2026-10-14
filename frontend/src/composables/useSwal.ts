import Swal from 'sweetalert2'
import i18n from '../i18n'
import { useToast, type ToastType } from './useToast'

// Настройки SweetAlert2 по умолчанию, соответствующие UI-гайду
const swalDefaults = {
  customClass: {
    popup: 'swal-popup',
    title: 'swal-title',
    htmlContainer: 'swal-html',
    confirmButton: 'swal-confirm',
    cancelButton: 'swal-cancel',
  },
  buttonsStyling: false,
} as const

export function useSwal() {
  function fire(options: Record<string, any>) {
    return Swal.fire({ ...swalDefaults, ...options })
  }

  function success(title: string, text?: string) {
    return fire({ icon: 'success', title, text, timer: 3000, timerProgressBar: true })
  }

  function error(title: string, text?: string) {
    return fire({ icon: 'error', title, text })
  }

  function confirm(title: string, text?: string) {
    return fire({
      title,
      text,
      showCancelButton: true,
      confirmButtonText: i18n.global.t('common.yes'),
      cancelButtonText: i18n.global.t('common.cancel'),
    })
  }

  // Всплывающие уведомления показывает своя стопка (AppToast), а не SweetAlert2:
  // она берёт цвета из токенов и меняется вместе с темой
  const toasts = useToast()

  function showChange(oldValue: string, newValue: string, field?: string, taskId?: string | number) {
    toasts.showChange({ oldValue, newValue, field, taskId })
  }

  function toast(message: string, type: ToastType = 'success') {
    toasts.show(message, type)
  }

  return { fire, success, error, confirm, showChange, toast, Swal }
}
