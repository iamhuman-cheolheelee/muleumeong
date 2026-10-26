import { createFileRoute, useNavigate } from '@tanstack/react-router'
import { useState } from 'react'
import { ArrowRight, CheckCircle2, CircleAlert, ExternalLink, SearchX, XCircle } from 'lucide-react'
import { type Answer, useAsk, useSeeds } from '@/lib/api'
import { Card, Pill, Series, writerTone } from '@/lib/ui'

type S = { q?: string; pin?: number }
export const Route = createFileRoute('/')({
  component: Ask,
  validateSearch: (s: Record<string, unknown>): S => ({ q: typeof s.q === 'string' ? s.q : undefined, pin: Number(s.pin) || undefined }),
})

function Ask() {
  const { q, pin } = Route.useSearch()
  const nav = useNavigate({ from: '/' })
  const seeds = useSeeds()
  const first = seeds.data?.[0]
  const cur = q ?? first?.q ?? ''
  const live = useAsk(q && (q !== first?.q || pin) ? q : '', pin ?? 0)
  const a: Answer | undefined = q && (q !== first?.q || pin) ? live.data : first
  const [draft, setDraft] = useState('')
  const go = (x: string, p?: number) => nav({ search: { q: x, pin: p } })

  return (
    <div className="space-y-5">
      <p className="text-[17px] font-extrabold text-ink">제주 공공데이터에 말로 물어보세요. 원본 파일을 계산해 숫자로 답하고, 맞는 데이터가 없으면 솔직히 모른다고 합니다.</p>
      <Card>
        <form className="flex flex-col gap-2 sm:flex-row" onSubmit={(e) => { e.preventDefault(); if (draft.trim()) go(draft.trim()) }}>
          <input value={draft} onChange={(e) => setDraft(e.target.value)} maxLength={200} placeholder="예) 서귀포 귤 생산량 요즘 어때요?"
            className="min-w-0 flex-1 rounded-[12px] border-[1.5px] border-line bg-canvas px-4 py-3 text-[15px] outline-none focus:border-accent" />
          <button className="flex items-center justify-center gap-1.5 rounded-[12px] bg-accent px-5 py-3 text-[15px] font-bold text-white">물어보기 <ArrowRight className="size-4" /></button>
        </form>
        <div className="mt-3 flex flex-wrap gap-1.5">
          {seeds.data?.filter((_, i) => [0, 1, 3, 6].includes(i)).map((s) => (
            <button key={s.q} onClick={() => go(s.q)} className={`rounded-full border-[1.5px] px-3 py-1 text-[12.5px] font-semibold ${s.q === cur && !pin ? 'border-accent bg-accent-soft text-accent' : 'border-line text-ink-2 hover:bg-canvas'}`}>{s.q}</button>
          ))}
        </div>
      </Card>

      {!a ? <div className="text-ink-3">{live.isFetching ? '원본을 찾아 계산하는 중… (보통 2~10초)' : '불러오는 중…'}</div> : <Result a={a} onPin={(id) => go(a.q, id)} />}
    </div>
  )
}

function Badge({ ok, label, why }: { ok: boolean; label: string; why: string }) {
  return (
    <span title={why} className={`inline-flex items-center gap-1 rounded-full px-3 py-1 text-[13px] font-bold ${ok ? 'bg-ok/15 text-ok' : 'bg-problem/10 text-problem'}`}>
      {ok ? <CheckCircle2 className="size-4" /> : <XCircle className="size-4" />}{label} {ok ? '✓' : '✗'}
    </span>
  )
}

function Refused({ a, onPin }: { a: Answer; onPin: (id: number) => void }) {
  return (
    <Card kicker={`질문 · ${a.q}`} title="맞는 데이터를 찾지 못했어요">
      <p className="flex items-start gap-2 text-[15.5px] leading-[1.7]"><SearchX className="mt-1 size-5 shrink-0 text-warn" />
        {a.error && a.error !== '맞는 데이터를 찾지 못했어요' ? a.error : '틀린 숫자를 드리느니 답하지 않습니다. 가장 가까운 데이터 3개와, 왜 맞지 않는지 적었어요.'}</p>
      {(a.cands?.length ?? 0) > 0 && (
        <ol className="mt-4 space-y-2">
          {a.cands!.map((c, i) => (
            <li key={c.id} className="rounded-[12px] border-[1.5px] border-line p-3">
              <div className="text-[14px] font-bold">{i + 1}. {c.title}</div>
              <div className="mt-1 text-[13px] text-ink-2">맞지 않는 이유: {c.why}</div>
              <div className="mt-2 flex gap-3 text-[12.5px]">
                {c.open && <button onClick={() => onPin(c.id)} className="font-bold text-accent">이 데이터로 그래도 계산해 보기</button>}
                <a href={c.url} target="_blank" rel="noreferrer" className="inline-flex items-center gap-1 text-ink-3">원본 페이지 <ExternalLink className="size-3" /></a>
              </div>
            </li>
          ))}
        </ol>
      )}
      <p className="mt-3 text-[12.5px] text-ink-3">낱말을 바꾸거나 해·지역을 넣어 다시 물어보세요.</p>
    </Card>
  )
}

function Result({ a, onPin }: { a: Answer; onPin: (id: number) => void }) {
  if (a.refused) return <Refused a={a} onPin={onPin} />
  const r = a.result
  const mItems = a.meaning?.items ?? []
  const series = r?.series ?? []
  const isLine = r?.op === 'trend'
  return (
    <div className="grid gap-5">
      <div className="min-w-0 space-y-5">
        <Card kicker={`질문 · ${a.q}`} title={a.chosen ? `「${a.chosen.title}」에서 계산한 답` : '답을 만들지 못했습니다'}
          right={a.writer && <Pill tone={writerTone(a.writer)}>{a.writer === '규칙 문장' ? '숫자·문장 모두 코드가 작성' : 'AI가 다듬은 문장 · 숫자 검산 통과'}</Pill>}>
          {a.error ? <p className="text-[15px] text-problem">{a.error}</p> : (
            <>
              <p className="text-[16.5px] font-semibold leading-[1.7]">{a.text}</p>
              <div className="mt-3 flex flex-wrap gap-2">
                <Badge ok={a.numOk} label="숫자 일치" why={a.check} />
                <Badge ok={!!a.meaning?.pass} label="의미 일치" why={mItems.map((m) => `${m.name}: ${m.detail}`).join('\n')} />
              </div>
              <p className="mt-2 text-[12.5px] text-ink-3">숫자 일치 = 답의 숫자가 원본 계산값과 같음 · 의미 일치 = 질문한 대상·때·지표를 셌음</p>
              {mItems.length > 0 && (
                <details className="mt-2"><summary className="cursor-pointer text-[12.5px] font-bold text-accent">의미 검사 {mItems.length}가지 보기</summary>
                  <ul className="mt-2 space-y-1 text-[12.5px]">{mItems.map((m) => <li key={m.name} className="flex gap-1.5">{m.ok ? <CheckCircle2 className="mt-0.5 size-3.5 shrink-0 text-ok" /> : <XCircle className="mt-0.5 size-3.5 shrink-0 text-problem" />}<b>{m.name}</b> {m.detail}</li>)}</ul>
                </details>
              )}
              {r && (
                <div className="mt-4 flex flex-wrap gap-1.5 text-[12px]">
                  <Pill tone="off">{r.opLabel}</Pill><Pill tone="off">측정: {r.measure}</Pill>{r.period && <Pill tone="off">{r.period}</Pill>}
                  {r.filters.map((f) => <Pill key={f} tone="blue">{f}</Pill>)}
                  <Pill tone="off">원본 {r.rowsTotal.toLocaleString()}행 중 {r.rowsUsed.toLocaleString()}행 사용</Pill>
                </div>
              )}
            </>
          )}
        </Card>
        {r && series.length > 0 && (
          <Card kicker={r.opLabel} title={`${r.measure}${r.agg === 'mean' && r.op !== 'peak' ? ' (평균)' : ''}`}>
            <Series data={series} kind={isLine ? 'line' : 'bar'} unit={r.unit} />
            {r.breakdown && r.breakdown.length > 0 && (
              <div className="mt-5 border-t-[1.5px] border-line pt-4">
                <div className="mb-2 text-[13px] font-bold text-ink-2">마지막 시점 「{r.breakBy}」별</div>
                <Series data={r.breakdown} kind="bar" unit={r.unit} />
              </div>
            )}
          </Card>
        )}
        <details className="rounded-[16px] border-[1.5px] border-line bg-surface p-4">
          <summary className="cursor-pointer text-[14px] font-bold text-accent">출처·계산 과정 보기 (원본 행·재현 코드·일한 순서)</summary>
          <div className="mt-4 grid gap-5 lg:grid-cols-2">
        {r?.evidence && r.evidence.length > 0 && (
          <Card kicker="숫자의 출처" title={`계산에 쓴 원본 행(${r.evidence.length}줄)`} right={a.chosen && <a href={a.chosen.url} target="_blank" rel="noreferrer" className="flex items-center gap-1 text-[13px] font-bold text-accent">원본 페이지 <ExternalLink className="size-3.5" /></a>}>
            <div className="overflow-x-auto">
              <table className="w-full min-w-[520px] text-[12.5px]">
                <thead><tr className="text-left text-ink-3">{(r.evHeader ?? []).map((h, i) => <th key={i} className="whitespace-nowrap border-b-[1.5px] border-line px-2 py-1.5 font-bold">{h}</th>)}</tr></thead>
                <tbody>{r.evidence.map((row, i) => <tr key={i} className="border-b border-line">{row.map((c, j) => <td key={j} className={`whitespace-nowrap px-2 py-1.5 ${j === 0 ? 'font-bold text-ink-3' : ''}`}>{c}</td>)}</tr>)}</tbody>
              </table>
            </div>
            <p className="mt-2 text-[12px] text-ink-3">원본 파일에서 계산에 쓴 행을 그대로 옮겼습니다. 표가 넓으면 옆으로 밀어 보세요. 같은 파일을 열면 같은 값이 있습니다.</p>
          </Card>
        )}
        {r?.code && (
          <Card kicker="재현 코드" title="같은 계산을 코드로 다시 해 보기">
            <pre className="overflow-x-auto rounded-[12px] bg-canvas p-3 text-[12px] leading-[1.55]">{r.code}</pre>
          </Card>
        )}
        <Card title="일한 순서">
          <ol className="space-y-3">
            {a.steps.map((s, i) => (
              <li key={i} className="grid grid-cols-[22px_1fr] gap-2">
                <span className={`grid size-[22px] place-items-center rounded-full text-[11px] font-black text-white ${s.state === 'warn' ? 'bg-warn' : 'bg-accent'}`}>{i + 1}</span>
                <div><div className="text-[13.5px] font-bold">{s.name}</div><div className="text-[12.5px] leading-[1.5] text-ink-2">{s.detail}</div></div>
              </li>
            ))}
          </ol>
        </Card>
        {r && r.notes.length > 0 && (
          <Card kicker="주의" title="계산 기준">
            <ul className="space-y-2">{r.notes.map((n) => <li key={n} className="flex gap-1.5 text-[13px] leading-[1.5]"><CircleAlert className="mt-0.5 size-3.5 shrink-0 text-warn" />{n}</li>)}</ul>
          </Card>
        )}
        {r?.facts && r.facts.length > 0 && (
          <Card title="답의 핵심 숫자">
            <dl className="divide-y divide-line text-[13px]">{r.facts.map((f) => <div key={f.key} className="flex justify-between gap-3 py-1.5"><dt className="text-ink-2">{f.label}</dt><dd className="text-right font-bold">{f.value}</dd></div>)}</dl>
            <p className="mt-2 text-[12px] text-ink-3">AI가 다듬은 문장은 코드가 쓴 기준 문장과 숫자·단위가 하나라도 다르면 버리고 기준 문장을 씁니다.</p>
          </Card>
        )}
        <Card title="찾은 데이터">
          <details open={false}><summary className="cursor-pointer text-[13px] font-bold text-accent">후보 {a.hits.length}개 보기 · 골라서 다시 계산</summary>
          <ul className="mt-2 space-y-1.5">
            {a.hits.map((h) => (
              <li key={h.id}>
                <button onClick={() => onPin(h.id)} disabled={!h.open && (h.header?.length ?? 0) < 2} className={`w-full rounded-[10px] px-2.5 py-2 text-left ${a.chosen?.id === h.id ? 'bg-accent-soft' : h.open ? 'hover:bg-canvas' : 'opacity-60'}`}>
                  <div className="text-[13px] font-bold leading-[1.4]">{h.title}</div>
                  <div className="mt-0.5 flex flex-wrap gap-1 text-[11px]"><Pill tone="off">{h.cat}</Pill>{h.open ? <Pill tone="ok">원본 받기 가능</Pill> : (h.header?.length ?? 0) >= 2 ? <Pill tone="gold">미리보기로 계산</Pill> : <Pill tone="off">로그인·신청 필요</Pill>}{h.format && <Pill tone="off">{h.format}</Pill>}</div>
                </button>
              </li>
            ))}
          </ul></details>
          <p className="mt-2 text-[12px] text-ink-3">다른 후보를 누르면 그 데이터로 다시 계산합니다. 원본 상태: {a.source || '–'} · {a.format}</p>
        </Card>
          </div>
        </details>
      </div>
    </div>
  )
}
