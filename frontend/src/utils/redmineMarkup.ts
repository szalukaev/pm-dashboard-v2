import textile from 'textile-js'
import { marked } from 'marked'
import DOMPurify from 'dompurify'

export interface MarkupAttachment {
  id: number
  filename: string
}

export interface MarkupOptions {
  attachments: MarkupAttachment[]
  // Адрес, с которого браузер загружает файл (через backend).
  attachmentUrl: (id: number) => string
  // Базовый адрес Redmine для ссылок на задачи вида #123; пустой — ссылок нет.
  redmineUrl: string
}

// Redmine хранит текст в Textile (по умолчанию) или в Markdown. Через API
// эта настройка не отдаётся, поэтому формат определяется по самому тексту.
const markdownSignals = [
  /^#{1,6}\s/m, // # заголовок
  /^```/m, // блок кода
  /!\[[^\]]*\]\([^)]+\)/, // ![alt](image)
  /\[[^\]]+\]\([^)\s]+\)/, // [text](url)
  /\*\*[^*\n]+\*\*/, // **полужирный**
  /^\s*[-*]\s+\[[ xX]\]/m, // список задач
]
const textileSignals = [
  /^h[1-6]\.\s/m, // h2. заголовок
  /^(bc|bq|p|pre)\.\s/m, // признаки блоков
  /(^|\s)![^!\s][^!\n]*!(\s|$|[.,:;])/m, // !image.png!
  /"[^"\n]+":\S+/, // "text":url
  /(^|\s)@[^@\n]+@/, // @code@
  /^\s*#\s/m, // # нумерованный список (заголовку Markdown тоже нужно слово после #, но списки Textile в Redmine встречаются часто)
]

export function isMarkdown(source: string): boolean {
  const score = (signals: RegExp[]) => signals.filter(re => re.test(source)).length
  return score(markdownSignals) > score(textileSignals)
}

function renderRaw(source: string): string {
  if (isMarkdown(source)) {
    return marked.parse(source, { gfm: true, breaks: true, async: false }) as string
  }
  return textile(source, { breaks: true })
}

function findAttachment(ref: string, attachments: MarkupAttachment[]): MarkupAttachment | undefined {
  let name = ref
  try {
    name = decodeURIComponent(ref)
  } catch {}
  name = name.split(/[?#]/)[0].toLowerCase()
  // По имени Redmine берёт самый свежий файл с таким именем.
  const byName = [...attachments].reverse().find(a => a.filename.toLowerCase() === name)
  if (byName) return byName
  // Абсолютные ссылки на файлы Redmine: /attachments/download/123/x.png
  const m = ref.match(/\/attachments\/(?:download\/|thumbnail\/)?(\d+)/)
  if (m) return attachments.find(a => a.id === Number(m[1]))
  return undefined
}

const issueRef = /(^|[\s(,;:])#(\d+)\b/g

function linkIssueRefs(root: ParentNode, redmineUrl: string) {
  const walker = document.createTreeWalker(root, NodeFilter.SHOW_TEXT)
  const nodes: Text[] = []
  while (walker.nextNode()) {
    const node = walker.currentNode as Text
    if (node.parentElement?.closest('a, code, pre')) continue
    issueRef.lastIndex = 0
    if (issueRef.test(node.data)) nodes.push(node)
  }
  for (const node of nodes) {
    const frag = document.createDocumentFragment()
    let last = 0
    node.data.replace(issueRef, (match, prefix: string, id: string, offset: number) => {
      frag.append(node.data.slice(last, offset) + prefix)
      const a = document.createElement('a')
      a.href = `${redmineUrl}/issues/${id}`
      a.textContent = `#${id}`
      a.target = '_blank'
      a.rel = 'noopener noreferrer'
      frag.append(a)
      last = offset + match.length
      return match
    })
    frag.append(node.data.slice(last))
    node.replaceWith(frag)
  }
}

/** Превращает текст Redmine (Textile или Markdown) в безопасный HTML. */
export function renderRedmineMarkup(source: string | null | undefined, opts: MarkupOptions): string {
  if (!source || !source.trim()) return ''

  const fragment = DOMPurify.sanitize(renderRaw(source), { RETURN_DOM_FRAGMENT: true }) as DocumentFragment

  fragment.querySelectorAll('img').forEach(img => {
    const src = img.getAttribute('src') || ''
    const att = findAttachment(src, opts.attachments)
    if (att) {
      img.setAttribute('src', opts.attachmentUrl(att.id))
      if (!img.getAttribute('alt')) img.setAttribute('alt', att.filename)
    }
    img.setAttribute('loading', 'lazy')
  })

  fragment.querySelectorAll('a').forEach(a => {
    const href = a.getAttribute('href') || ''
    if (!/^[a-z][a-z0-9+.-]*:|^\/|^#/i.test(href)) {
      const att = findAttachment(href, opts.attachments)
      if (att) a.setAttribute('href', opts.attachmentUrl(att.id))
    }
    a.setAttribute('target', '_blank')
    a.setAttribute('rel', 'noopener noreferrer')
  })

  if (opts.redmineUrl) linkIssueRefs(fragment, opts.redmineUrl.replace(/\/$/, ''))

  const container = document.createElement('div')
  container.append(fragment)
  return container.innerHTML
}
