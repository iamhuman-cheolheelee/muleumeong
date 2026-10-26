import { useQuery } from '@tanstack/react-query'

export type Col = { i: number; name: string; label: string; kind: string; distinct: number; missing: number; samples: string[]; min: number; max: number }
export type Hit = { id: number; title: string; cat: string; owner: string; updated: string; open: boolean; api: boolean; format?: string; score: number; bm25Rank: number; coverRank: number; matched: string[] | null; header: string[] | null; url: string }
export type Point = { k: string; v: number; n: number }
export type Fact = { key: string; label: string; value: string }
export type Result = {
  op: string; opLabel: string; measure: string; unit: string; agg: string; series: Point[] | null; breakdown: Point[] | null; breakBy: string
  facts: Fact[] | null; filters: string[]; period: string; rowsUsed: number; rowsTotal: number; dropped: number; notes: string[]
  evidence: string[][] | null; evHeader: string[] | null; plan: { why: string[]; opWhy: string }; cols: Col[]; code: string
}
export type Step = { tool: string; name: string; detail: string; ms: number; state: string }
export type CheckItem = { name: string; ok: boolean; detail: string }
export type Cand = { id: number; title: string; url: string; open: boolean; why: string }
export type Answer = {
  meaning?: { pass: boolean; items: CheckItem[] }; numOk: boolean; refused: boolean; cands: Cand[] | null
  q: string; terms: string[]; extra: string[] | null; hits: Hit[]; chosen?: Hit; skipped: string[]; source: string; format: string; rows: number
  result?: Result; text: string; writer: string; check: string; steps: Step[]; totalMs: number; error?: string
}
export type Quality = {
  Total: number; Open: number; API: number; Login: number; ByFile: Record<string, number>; Scanned: number; CP949: number; UTF8: number
  XlsxInCsv: number; WithTotal: number; HTMLPrev: number; Examples: Record<string, Hit[]>; ByCat: [string, number][]
}
export type Gold = { n: number; r1: number; r5: number; rows: { q: string; pos: number; top: string }[] }

export const get = async <T,>(u: string): Promise<T> => {
  const r = await fetch(u)
  if (!r.ok) throw new Error(`${r.status} ${u}`)
  return r.json()
}

export const useSeeds = () => useQuery({ queryKey: ['seeds'], queryFn: () => get<Answer[]>('/api/seeds'), staleTime: Infinity })
export const useAsk = (q: string, pin: number) =>
  useQuery({ queryKey: ['ask', q, pin], queryFn: () => get<Answer>(`/api/ask?q=${encodeURIComponent(q)}&pin=${pin}`), enabled: !!q, staleTime: Infinity })
export const useSearchCat = (q: string) =>
  useQuery({ queryKey: ['search', q], queryFn: () => get<{ hits: Hit[]; terms: string[] }>(`/api/search?q=${encodeURIComponent(q)}`), enabled: !!q, placeholderData: (p) => p })
export const useQuality = () => useQuery({ queryKey: ['quality'], queryFn: () => get<Quality>('/api/quality'), staleTime: Infinity })
export const useGold = () => useQuery({ queryKey: ['gold'], queryFn: () => get<Gold>('/api/golden'), staleTime: Infinity })
export const useHealth = () => useQuery({ queryKey: ['health'], queryFn: () => get<{ ok: boolean; datasets: number; model: string; llm: boolean }>('/api/health'), refetchInterval: 30000 })
export const fmt = (v: number) => (Math.abs(v) >= 100 ? Math.round(v).toLocaleString() : (Math.round(v * 10) / 10).toLocaleString())
