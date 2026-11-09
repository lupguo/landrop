const PROMPT = /^\s*(\$|#|>|PS [A-Z]:\\)/m
const KEYWORD = /\b(func|def|class|import|const|let|var|return|select|from|kubectl|docker|git)\b/
const SINGLE = /^(\$ |kubectl |git |docker )/

/** Heuristic: does this text look like a code snippet, shell command or log? */
export function looksLikeCode(text: string): boolean {
  const lines = text.replace(/\s+$/, '').split(/\r?\n/)
  if (lines.length < 2) return SINGLE.test(lines[0].trimStart())
  const indented = lines.filter((l) => /^( {2,}|\t)/.test(l)).length
  if (indented / lines.length >= 0.3) return true
  if (lines.some((l) => /[{};]\s*$/.test(l))) return true
  return PROMPT.test(text) || KEYWORD.test(text)
}
