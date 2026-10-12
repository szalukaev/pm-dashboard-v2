import axios from 'axios'

// Saves a blob as a file.
export function saveBlob(blob: Blob, name: string) {
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  a.download = name
  a.click()
  URL.revokeObjectURL(url)
}

// Downloads a file from the server under the name the server gives it
// (the name may be in Russian and comes encoded), or under fallbackName.
export async function downloadFile(url: string, fallbackName: string, params?: Record<string, string>) {
  const response = await axios.get(url, { params, responseType: 'blob' })
  const header = String(response.headers['content-disposition'] || '')
  const encoded = /filename\*=UTF-8''([^;]+)/i.exec(header)
  saveBlob(response.data, encoded ? decodeURIComponent(encoded[1]) : fallbackName)
}
