import type { ReactNode } from 'react'

export const KIND_COLOR: Record<string, string> = { 기상: 'var(--color-problem)', 병해충: 'var(--color-gold)', 보조사업: 'var(--color-accent)' }

export function Card({ title, kicker, right, children, className = '' }: { title?: ReactNode; kicker?: ReactNode; right?: ReactNode; children: ReactNode; className?: string }) {
  return (
    <section className={`card p-5 ${className}`}>
      {(title || kicker) && (
        <header className="mb-3 flex items-start justify-between gap-3">
          <div>
            {kicker && <div className="kicker">{kicker}</div>}
            {title && <h3 className="mt-0.5 text-[16px] font-extrabold tracking-[-0.02em]">{title}</h3>}
          </div>
          {right}
        </header>
      )}
      {children}
    </section>
  )
}

export function Kpi({ v, unit, label, sub, accent }: { v: ReactNode; unit?: string; label: string; sub?: string; accent?: boolean }) {
  return (
    <div className="card px-5 py-4">
      <div className={`num text-[34px] ${accent ? 'text-accent' : ''}`}>{v}{unit && <small>{unit}</small>}</div>
      <div className="mt-2 text-[13px] font-bold">{label}</div>
      {sub && <div className="mt-0.5 text-[12px] text-ink-3">{sub}</div>}
    </div>
  )
}

export function Pill({ children, tone = 'ok' }: { children: ReactNode; tone?: 'ok' | 'off' | 'acc' | 'gold' | 'blue' | 'prob' }) {
  const c = {
    ok: 'bg-[#C4EED0] text-[#14532d]', off: 'bg-canvas text-ink-2 border border-line', acc: 'bg-accent-soft text-accent',
    gold: 'bg-gold-soft text-[#7a4b00]', blue: 'bg-blue-soft text-blue-hr', prob: 'bg-problem-soft text-problem',
  }[tone]
  return <span className={`inline-flex items-center gap-1 whitespace-nowrap rounded-full px-2.5 py-0.5 text-[12px] font-bold ${c}`}>{children}</span>
}

export const kindTone = (k: string): 'gold' | 'blue' | 'prob' | 'acc' | 'off' => (k === '기상' ? 'prob' : k === '병해충' ? 'gold' : k === '보조사업' ? 'acc' : 'off')

export function DDay({ d }: { d?: number }) {
  if (d === undefined || d === null) return null
  return <Pill tone={d <= 7 ? 'prob' : d <= 30 ? 'gold' : 'off'}>{d === 0 ? 'D-day' : d > 0 ? `D-${d}` : '마감'}</Pill>
}

// 폰 문자 화면 목업 — 실제로 나가는 글자 그대로
export function Sms({ text, channel, small }: { text: string; channel?: string; small?: boolean }) {
  return (
    <div className={`rounded-[22px] border-[1.5px] border-line bg-canvas p-3 ${small ? '' : 'max-w-[380px]'}`}>
      <div className="mb-2 flex items-center justify-between text-[11px] font-bold text-ink-3"><span>{channel === '음성 안내' ? '음성 안내 원고(75세 이상)' : '문자 미리보기'}</span><span>의성군</span></div>
      <pre className="whitespace-pre-wrap break-keep rounded-[16px] rounded-tl-[4px] bg-surface p-3 font-sans text-[13.5px] leading-[1.55] text-ink">{text || '오늘 보낼 내용이 없습니다.'}</pre>
    </div>
  )
}

export function Bars({ data, colors }: { data: [string, number][]; colors?: Record<string, string> }) {
  const m = Math.max(...data.map((d) => d[1]), 1)
  return (
    <div className="space-y-2">
      {data.map(([k, v]) => (
        <div key={k} className="grid grid-cols-[72px_1fr_36px] items-center gap-2 text-[13px]">
          <span className="font-semibold text-ink-2">{k}</span>
          <div className="h-3 rounded-full bg-canvas"><div className="h-3 rounded-full" style={{ width: `${(v / m) * 100}%`, background: colors?.[k] ?? 'var(--color-accent)' }} /></div>
          <span className="text-right font-extrabold">{v}</span>
        </div>
      ))}
    </div>
  )
}


// 막대·선 겸용 작은 차트 — 시간 흐름은 선, 순위는 막대
export function Series({ data, kind, unit = '' }: { data: { k: string; v: number }[]; kind: 'line' | 'bar'; unit?: string }) {
  if (!data.length) return <div className="text-[13px] text-ink-3">그릴 값이 없습니다.</div>
  if (kind === 'bar') {
    const top = data.slice(0, 10)
    const m = Math.max(...top.map((d) => d.v), 1)
    return (
      <div className="space-y-2">
        {top.map((d, i) => (
          <div key={d.k} className="grid grid-cols-[minmax(84px,148px)_1fr_auto] items-center gap-2 text-[13px]">
            <span className="truncate font-semibold text-ink-2" title={d.k}>{d.k}</span>
            <div className="h-3 rounded-full bg-canvas"><div className="h-3 rounded-full" style={{ width: `${(d.v / m) * 100}%`, background: i === 0 ? 'var(--color-accent)' : 'var(--color-line-2)' }} /></div>
            <span className="text-right font-extrabold tabular-nums">{d.v.toLocaleString()}{unit}</span>
          </div>
        ))}
      </div>
    )
  }
  const W = 640, H = 220, P = 34
  const vs = data.map((d) => d.v)
  const lo = Math.min(...vs), hi = Math.max(...vs)
  const pad = (hi - lo) * 0.15 || 1
  const y = (v: number) => H - P - ((v - (lo - pad)) / (hi - lo + 2 * pad)) * (H - 2 * P)
  const x = (i: number) => P + (i / Math.max(1, data.length - 1)) * (W - 2 * P)
  const iMax = vs.indexOf(hi), iMin = vs.indexOf(lo)
  const every = Math.ceil(data.length / 8)
  return (
    <svg viewBox={`0 0 ${W} ${H}`} className="w-full" role="img" aria-label="기간별 흐름">
      <polyline fill="none" stroke="var(--color-accent)" strokeWidth="2.5" points={data.map((d, i) => `${x(i)},${y(d.v)}`).join(' ')} />
      {data.map((d, i) => (
        <g key={d.k}>
          <circle cx={x(i)} cy={y(d.v)} r={i === iMax || i === iMin || i === data.length - 1 ? 4.5 : 3} fill={i === iMin ? 'var(--color-problem)' : 'var(--color-accent)'} />
          {(i === iMax || i === iMin || i === 0 || i === data.length - 1) && (
            <text x={x(i)} y={i === iMin || (i === 0 && data.length > 1 && data[1].v > d.v) || (i === data.length - 1 && i > 0 && data[i - 1].v > d.v) ? y(d.v) + 24 : y(d.v) - 12} textAnchor={i === 0 ? 'start' : i === data.length - 1 ? 'end' : 'middle'} fontSize="15" fontWeight="800" fill="var(--color-ink)" stroke="#fff" strokeWidth="4" paintOrder="stroke">{d.v.toLocaleString()}{unit}</text>
          )}
          {i % every === 0 || i === data.length - 1 ? <text x={x(i)} y={H - 10} textAnchor="middle" fontSize="13" fill="var(--color-ink-3)">{d.k}</text> : null}
        </g>
      ))}
    </svg>
  )
}

export const writerTone = (w: string): 'ok' | 'acc' | 'gold' => (w.startsWith('Qwen') || w === '로컬 모델' ? 'acc' : w === '캐시' ? 'ok' : 'gold')
