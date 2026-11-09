import type { DirEntry } from '../api'

export interface Crumb { name: string; path: string }

export function breadcrumbs(path: string): Crumb[] {
  const out: Crumb[] = []
  let acc = ''
  for (const seg of path.split('/').filter(Boolean)) {
    acc = acc ? `${acc}/${seg}` : seg
    out.push({ name: seg, path: acc })
  }
  return out
}

export function joinPath(base: string, name: string): string {
  return base ? `${base}/${name}` : name
}

export function sortEntries(list: DirEntry[]): DirEntry[] {
  return [...list].sort((a, b) =>
    a.is_dir !== b.is_dir ? (a.is_dir ? -1 : 1) : a.name.localeCompare(b.name, undefined, { sensitivity: 'base' }) || (a.name < b.name ? -1 : 1))
}
