// 通用小工具：剪贴板复制 + 文件下载，供工具页复用
export async function copyText(text: string): Promise<boolean> {
  try {
    await navigator.clipboard.writeText(text)
    return true
  } catch {
    const ta = document.createElement('textarea')
    ta.value = text
    ta.style.position = 'fixed'
    ta.style.opacity = '0'
    document.body.appendChild(ta)
    ta.select()
    const ok = document.execCommand('copy')
    ta.remove()
    return ok
  }
}

export function downloadText(filename: string, text: string, mime = 'text/plain') {
  const blob = new Blob([text], { type: `${mime};charset=utf-8` })
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  a.download = filename
  a.click()
  URL.revokeObjectURL(url)
}

// JSON 语法高亮：输入必须已是格式化后的 JSON 文本，返回安全 HTML
export function highlightJson(code: string): string {
  const esc = (s: string) =>
    s.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;')
  return esc(code).replace(
    /("(\\u[a-z0-9]{4}|\\[^u]|[^\\"])*")(\s*:)?|\b(true|false|null)\b|-?\d+(\.\d+)?([eE][+-]?\d+)?/g,
    (m, str, _u, colon) => {
      if (str !== undefined) {
        return `<span class="j-${colon ? 'key' : 'str'}">${str}</span>${colon ?? ''}`
      }
      if (/^(true|false)$/.test(m)) return `<span class="j-bool">${m}</span>`
      if (m === 'null') return `<span class="j-null">${m}</span>`
      return `<span class="j-num">${m}</span>`
    }
  )
}
