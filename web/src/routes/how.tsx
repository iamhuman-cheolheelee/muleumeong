import { createFileRoute } from '@tanstack/react-router'
import { Card } from '@/lib/ui'

export const Route = createFileRoute('/how')({ component: How })

const STEPS = [
  ['질문 이해', '조사·말투를 떼고 데이터를 가리키는 낱말만 남깁니다. 「장사」→소상공인, 「귤」→감귤 같은 도민 말 사전을 거치고, 소형 모델이 낱말을 2~4개 더합니다.'],
  ['데이터 찾기', '제주데이터허브 1,259종의 제목·설명·태그·미리보기 열 이름을 색인해 두고, 글자 조각이 겹치는 정도와 낱말 포함 수를 합쳐 순위를 매깁니다. 로그인·신청이 필요하거나 hwp·zip 인 후보는 이유를 적고 건너뜁니다.'],
  ['원본 불러오기', '받아 둔 사본 → 데이터허브 실시간 내려받기 → 미리보기 순으로 시도합니다. 확장자 대신 파일 첫머리를 보고 엑셀인지, 옛 한글 방식 CSV인지 가립니다.'],
  ['코드로 계산', '질문 말투로 흐름·순위·비교를 고르고, 열 이름과 질문의 글자 조각 겹침으로 측정 열을 정합니다. 합계 행을 빼고, 12개월이 다 차지 않은 해는 연도 비교에서 뺍니다.'],
  ['의미 검산', '고른 열이 질문의 주제·지표·대상과 맞는지, 평균을 더하거나 쌓인 값을 합치지 않았는지 코드로 한 번 더 봅니다. 하나라도 어긋나면 답하지 않습니다.'],
  ['답 쓰기·숫자 검산', '모델에게는 사실표와 규칙 문장만 건넵니다. 돌아온 문장의 숫자·단위가 사실표와 하나라도 다르면 버리고 규칙 문장을 씁니다. 모델이 꺼져 있어도 같은 답이 나옵니다.'],
]

function How() {
  return (
    <div className="grid gap-5 lg:grid-cols-[2fr_1fr]">
      <Card title="물으멍이 질문을 처리하는 순서">
        <ol className="space-y-4">
          {STEPS.map(([t, d], i) => (
            <li key={t} className="grid grid-cols-[28px_1fr] gap-3">
              <span className="grid size-7 place-items-center rounded-full bg-accent text-[13px] font-black text-white">{i + 1}</span>
              <div><div className="text-[15px] font-extrabold">{t}</div><p className="text-[13.5px] leading-[1.65] text-ink-2">{d}</p></div>
            </li>
          ))}
        </ol>
      </Card>
      <div className="space-y-5">
        <Card title="직접 다시 계산하기">
          <ol className="list-decimal space-y-2 pl-4 text-[13px] leading-[1.6] text-ink-2">
            <li>답 화면의 「원본 페이지」에서 같은 파일을 내려받습니다.</li>
            <li>「재현 코드」를 복사해 파일과 같은 폴더에서 파이썬으로 실행합니다.</li>
            <li>출력값을 답의 숫자와 견줍니다. 「숫자의 출처」 표의 행을 원본 파일에서 찾아 같은 값인지도 확인할 수 있습니다.</li>
          </ol>
        </Card>
        <Card title="돌아가는 곳">
          <ul className="space-y-1.5 text-[13px] leading-[1.6] text-ink-2">
            <li>설치 없이 실행 파일 하나(화면 포함)</li>
            <li>문장 다듬기: PC 안의 소형 공개 AI(일반 사무용 PC에서 동작)</li>
            <li>돈 내는 외부 AI 서비스 없음 · 질문이 PC 밖으로 나가지 않음(원본 내려받기만 데이터허브에 접속)</li>
          </ul>
        </Card>
      </div>
    </div>
  )
}
