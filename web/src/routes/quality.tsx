import { createFileRoute } from '@tanstack/react-router'
import { useQuality } from '@/lib/api'
import { Card, Kpi, Series } from '@/lib/ui'

export const Route = createFileRoute('/quality')({ component: Quality })

function Quality() {
  const q = useQuality().data
  if (!q) return <div className="text-ink-3">불러오는 중…</div>
  const nm: Record<string, string> = { csv: 'CSV', hwp: '한글(hwp)', zip: '압축(zip)', xlsx: '엑셀', xls: '엑셀' }
  const agg: Record<string, number> = {}
  for (const [k, v] of Object.entries(q.ByFile)) agg[nm[k] ?? '기타'] = (agg[nm[k] ?? '기타'] ?? 0) + v
  const nFiles = Object.values(q.ByFile).reduce((s, v) => s + v, 0)
  const files = Object.entries(agg).sort((a, b) => b[1] - a[1]).map(([k, v]) => ({ k, v }))
  const ex = (k: string, t: string) => q.Examples[k]?.length ? (
    <div><div className="mb-1 text-[13px] font-bold">{t}</div><ul className="space-y-1 text-[12.5px]">{q.Examples[k].map((h) => <li key={h.id}><a href={h.url} target="_blank" rel="noreferrer" className="hover:text-accent">#{h.id} {h.title}</a></li>)}</ul></div>
  ) : null
  return (
    <div className="space-y-5">
      <div className="grid grid-cols-2 gap-3 sm:gap-4 lg:grid-cols-4">
        <Kpi v={q.Scanned.toLocaleString()} unit="개" label="열어 본 미리보기 파일" sub={`목록 ${q.Total.toLocaleString()}종 중`} />
        <Kpi v={q.CP949} unit="개" label="한글이 깨져 보이는 옛 방식 파일" sub={`자동으로 풀어 읽음 · 요즘 방식 ${q.UTF8}개`} accent />
        <Kpi v={q.WithTotal} unit="개" label="중간에 「합계」 줄이 끼어 있는 표" sub="그대로 더하면 숫자가 두 배가 됨 → 그 줄을 빼고 계산" />
        <Kpi v={q.Open} unit="종" label="로그인 없이 바로 받는 원본" sub={`목록 ${q.Total.toLocaleString()}종 = 바로 받음 ${q.Open} + 로그인·신청 ${q.Login} + 내려받을 파일 없음 ${q.Total - q.Open - q.Login}`} />
      </div>
      <div className="grid gap-5 lg:grid-cols-[2fr_1fr]">
        <Card title={`첨부 파일이 있는 ${nFiles}종의 파일 형식`}>
          <Series data={files} kind="bar" />
          <p className="mt-3 text-[12px] text-ink-3">hwp·zip 은 표로 바로 읽지 못해 계산 후보에서 빼고, 이유를 「일한 순서」에 적습니다. 물으멍은 확장자 대신 파일 내용을 직접 보고 엑셀인지, 옛 한글 방식 CSV인지 가립니다.</p>
        </Card>
        <Card title="분야별 데이터 수"><Series data={q.ByCat.map(([k, v]) => ({ k, v }))} kind="bar" /></Card>
      </div>
      <Card title="자동으로 바로잡는 함정과 실제 예">
        <div className="grid gap-5 md:grid-cols-2">
          {ex('total', '「계·합계·전국」 행이 섞인 표 → 빼고 계산, 뺀 줄 수를 답에 적음')}
          {ex('cp949', '옛 한글 방식으로 저장된 CSV → 자동으로 한글로 풀어 읽음')}
        </div>
        <p className="mt-4 text-[12.5px] leading-[1.6] text-ink-2">예: 「제주도 자동차 등록 현황」(#915) 원본은 시트 2개짜리 엑셀이고, 달마다 「계」 행이 끼어 있습니다. 그대로 더하면 등록 대수가 두 배로 잡힙니다. 등록 대수처럼 쌓이는 값은 12개월을 더하지 않고 가장 최근 달 값만 씁니다.</p>
      </Card>
    </div>
  )
}
