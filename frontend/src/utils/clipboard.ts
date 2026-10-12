// Copies text to the clipboard and reports whether it worked.
//
// The clipboard API exists only on https and localhost; the dashboard is
// often opened by plain http inside a network, so there is a fallback
// through a hidden text field.
export async function copyText(text: string): Promise<boolean> {
  try {
    if (navigator.clipboard && window.isSecureContext) {
      await navigator.clipboard.writeText(text)
      return true
    }
  } catch {}

  const field = document.createElement('textarea')
  field.value = text
  field.setAttribute('readonly', '')
  field.style.position = 'fixed'
  field.style.opacity = '0'
  document.body.appendChild(field)
  field.select()
  let copied = false
  try {
    copied = document.execCommand('copy')
  } catch {}
  field.remove()
  return copied
}
