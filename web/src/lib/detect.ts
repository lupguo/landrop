// No `m` flag: tested per already-split line, so `^\s*` is anchored once.
// (With /m, `\s*` spans newlines from every line start: quadratic.)
const PROMPT = /^\s*(\$|#|>|PS [A-Z]:\\)/
/** Detection only looks at this many leading chars (bounds every regex). */
export const DETECT_LIMIT = 16 * 1024
const KEYWORD = /\b(func|def|class|import|const|let|var|return|select|from|kubectl|docker|git)\b/
const SINGLE = /^(\$ |kubectl |git |docker )/

/**
 * Heuristic: does this text look like code or a shell session?
 * Multi-line text needs at least two independent signals, so prose with one
 * keyword ("from", "let", "return") does not qualify.
 */
export function looksLikeCode(text: string): boolean {
  const lines = text.slice(0, DETECT_LIMIT).trimEnd().split(/\r?\n/)
  if (lines.length < 2) return SINGLE.test(lines[0].trimStart())
  const indented = lines.filter((l) => /^( {2,}|\t)/.test(l)).length
  const signals = [
    indented / lines.length >= 0.3,
    lines.some((l) => /[{};]$/.test(l.trimEnd())),
    lines.some((l) => PROMPT.test(l)),
    lines.some((l) => KEYWORD.test(l)),
  ].filter(Boolean).length
  return signals >= 2
}

const LEVELS = 'error|warn|warning|info|debug|trace|fatal|notice|crit'
const LOG_LINE = new RegExp(
  '^\\s*(' +
    [
      '\\[?\\d{4}[-/]\\d{2}[-/]\\d{2}', // 2026-11-09 / 2026/11/09 / [2026-11-09T…
      '\\[?\\d{2}:\\d{2}:\\d{2}', // 10:20:00(.123)
      '[A-Z][a-z]{2} +\\d{1,2} \\d{2}:\\d{2}:\\d{2}', // syslog: Nov 09 10:20:01
      '[IWEF]\\d{4}\\b', // glog: I1109
      `(${LEVELS.toUpperCase()})\\b`, // ERROR / WARN / INFO / DEBUG first
      `\\[(${LEVELS}|${LEVELS.toUpperCase()})\\]`, // [error] / [ERROR]
      '[VDIWEF]/[\\w.$-]+(\\(\\s*\\d+\\))?:', // android: E/Tag:
    ].join('|') +
    ')',
)

/** ≥2 non-empty lines and ≥50% of them start with a timestamp or log level. */
export function looksLikeLog(text: string): boolean {
  const lines = text.slice(0, DETECT_LIMIT).split(/\r?\n/).filter((l) => l.trim() !== '')
  if (lines.length < 2) return false
  return lines.filter((l) => LOG_LINE.test(l)).length / lines.length >= 0.5
}
