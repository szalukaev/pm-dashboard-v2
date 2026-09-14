import Swal from 'sweetalert2'

// UI_GUIDE-compliant SweetAlert2 defaults
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
      confirmButtonText: 'Да',
      cancelButtonText: 'Отмена',
    })
  }

  function showChange(oldValue: string, newValue: string) {
    return fire({
      icon: 'info',
      title: 'Изменение',
      html: `<span style="color:var(--text-muted)">${oldValue}</span> → <span style="color:var(--success)">${newValue}</span>`,
      timer: 3000,
      timerProgressBar: true,
    })
  }

  return { fire, success, error, confirm, showChange, Swal }
}
