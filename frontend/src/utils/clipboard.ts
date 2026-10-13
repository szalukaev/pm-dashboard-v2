// Копирует текст в буфер обмена и сообщает, получилось ли.
//
// API буфера обмена есть только на https и localhost; дашборд часто
// открывают по обычному http внутри сети, поэтому есть запасной способ
// через скрытое текстовое поле.
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
