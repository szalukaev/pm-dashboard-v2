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

  function showChange(oldValue: string, newValue: string, header?: string) {
    return Swal.fire({
      toast: true,
      position: 'top-end',
      icon: 'success',
      title: `${header ? `<div style="font-size:11px;color:#94a3b8;margin-bottom:2px">${header}</div>` : ''}<div style="font-size:13px"><span style="color:#999;text-decoration:line-through">${oldValue}</span> <span style="color:#fff;margin:0 4px">→</span> <span style="color:#4ade80;font-weight:600">${newValue}</span></div>`,
      showConfirmButton: false,
      timer: 3000,
      timerProgressBar: true,
      background: '#1e293b',
      color: '#f1f5f9',
      customClass: { popup: 'toast-beautiful' },
    })
  }

  function toast(message: string, type: 'success' | 'error' | 'info' = 'success') {
    return Swal.fire({
      toast: true,
      position: 'top-end',
      icon: type,
      title: `<div style="font-size:13px">${message}</div>`,
      showConfirmButton: false,
      timer: 2500,
      timerProgressBar: true,
      background: type === 'error' ? '#7f1d1d' : '#1e293b',
      color: '#f1f5f9',
      customClass: { popup: 'toast-beautiful' },
    })
  }

  return { fire, success, error, confirm, showChange, toast, Swal }
}
