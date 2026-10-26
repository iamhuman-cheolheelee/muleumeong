import { createFileRoute, Link, useNavigate } from '@tanstack/react-router'
import { useState } from 'react'
import { useGold, useSearchCat } from '@/lib/api'
import { Card, Pill } from '@/lib/ui'

export const Route = createFileRoute('/find')({
  component: Find,
  validateSearch: (s: Record<string, unknown>): { q?: string } => ({ q: typeof s.q === 'string' ? s.q : undefined }),
})

function Find() {
  const { q = '버스 승객 얼마나 타요' } = Route.useSearch()
  const nav = useNavigate({ from: '/find' })
  const [d, setD] = useState(q)
  const s = useSearchCat(q)
  const g = useGold()
  return (
    <div className="grid gap-5 lg:grid-cols-[2fr_1fr]">
      <Card title="낱말로 1,259종 찾기">
        <form onSubmit={(e) => { e.preventDefault(); nav({ search: { q: d } }) }}>
          <input value={d} onChange={(e) => setD(e.target.value)} className="w-full rounded-[12px] border-[1.5px] border-line bg-canvas px-4 py-3 text-[15px] outline-none focus:border-accent" />
        </form>
        <div className="mt-2 flex flex-wrap gap-1 text-[12px]">찾은 낱말: {s.data?.terms.map((t) => <Pill key={t} tone="blue">{t}</Pill>)}</div>
        <div className="mt-3 overflow-x-auto">
          <table className="w-full min-w-[560px] text-[13px]">
            <thead><tr className="text-left text-[12px] text-ink-3"><th className="py-1.5">순위</th><th>데이터</th><th>원본</th></tr></thead>
            <tbody>
              {s.data?.hits.map((h, i) => (
                <tr key={h.id} className="border-t border-line">
                  <td className="py-2 font-extrabold">{i + 1}</td>
                  <td className="py-2"><a href={h.url} target="_blank" rel="noreferrer" className="font-bold hover:text-accent">{h.title}</a><div className="text-[11.5px] text-ink-3">{h.cat} · {h.owner} · {h.updated} · 겹친 낱말 {h.matched?.join(', ') || '없음'}</div></td>
                  <td>{h.open || (h.header?.length ?? 0) >= 2 ? <Link to="/" search={{ q, pin: h.id }} className="font-bold text-accent">{h.open ? '계산하기' : '미리보기로 계산'}</Link> : <span className="text-ink-3">기관 신청 필요</span>}</td>
                  
                </tr>
              ))}
            </tbody>
          </table>
        </div>
        <p className="mt-3 text-[12px] text-ink-3">글자 조각이 겹치는 정도와 핵심 낱말이 들어 있는 수, 두 가지 순위를 합쳐 고릅니다. 「감귤생산량」처럼 띄어 쓰지 않은 말도 글자 조각으로 잡힙니다.</p>
      </Card>
      <Card kicker={g.data ? `시험 질문 ${g.data.n}개 중 정답 데이터를 1위로 찾음 ${g.data.r1}개 · 5위 안 ${g.data.r5}개` : ''} title="찾기 정답표">
        <ul className="divide-y divide-line text-[12.5px]">
          {g.data?.rows.map((r) => (
            <li key={r.q} className="flex items-center gap-2 py-1.5">
              <span className={`w-9 shrink-0 text-center font-black ${r.pos === 1 ? 'text-ok' : r.pos ? 'text-warn' : 'text-problem'}`}>{r.pos ? `${r.pos}위` : '없음'}</span>
              <button onClick={() => { setD(r.q); nav({ search: { q: r.q } }) }} className="text-left hover:text-accent">{r.q}</button>
            </li>
          ))}
        </ul>
        <p className="mt-2 text-[12px] text-ink-3">도민 말투 질문 20개의 정답 데이터를 사람이 먼저 정해 두고, 검색 규칙을 바꿀 때마다 다시 잽니다.</p>
      </Card>
    </div>
  )
}
