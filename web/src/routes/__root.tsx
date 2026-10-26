import { createRootRoute, Link, Outlet, useRouterState } from '@tanstack/react-router'
import { BookOpenCheck, MessageCircleQuestion, Search, ShieldCheck } from 'lucide-react'

export const Route = createRootRoute({ component: Shell })

export const MENU = [
  { to: '/', label: '질문하기', icon: MessageCircleQuestion },
  { to: '/find', label: '데이터 찾기', icon: Search },
  { to: '/quality', label: '품질 점검', icon: ShieldCheck },
  { to: '/how', label: '확인법', icon: BookOpenCheck },
] as const

function Shell() {
  const path = useRouterState({ select: (s) => s.location.pathname })
  const cur = MENU.find((m) => (m.to === '/' ? path === '/' : path.startsWith(m.to))) ?? MENU[0]
  return (
    <div className="relative min-h-screen lg:grid lg:grid-cols-[232px_1fr]">
      <div aria-hidden className="pointer-events-none absolute inset-y-0 left-0 hidden w-[232px] border-r-[1.5px] border-line bg-surface lg:block" />
      <aside className="relative border-line bg-surface lg:sticky lg:top-0 lg:flex lg:h-screen lg:flex-col lg:bg-transparent">
        <div className="flex items-center gap-2.5 px-5 pb-3 pt-5">
          <img src="/favicon.svg" alt="" className="size-8" />
          <div className="text-[21px] font-black tracking-[-0.04em]">물으멍</div>
        </div>
        <nav className="flex gap-1 overflow-x-auto px-3 pb-3 lg:block lg:space-y-1 lg:overflow-visible">
          {MENU.map((m) => {
            const on = m === cur
            return (
              <Link key={m.to} to={m.to}
                className={`flex shrink-0 items-center gap-1.5 rounded-[12px] px-2 py-2 text-[13px] font-bold sm:gap-2.5 sm:px-3 sm:py-2.5 sm:text-[14px] ${on ? 'bg-accent-soft text-accent' : 'text-ink-2 hover:bg-canvas'}`}>
                <m.icon className="size-[18px] shrink-0" strokeWidth={2.2} />{m.label}
              </Link>
            )
          })}
        </nav>
      </aside>
      <div className="min-w-0">
        <header className="sticky top-0 z-10 flex items-center gap-3 border-b-[1.5px] border-line bg-surface/95 px-5 py-3 backdrop-blur lg:px-8">
          <h1 className="mr-auto text-[20px] font-extrabold tracking-[-0.03em]">{cur.label}</h1>
        </header>
        <main className="mx-auto max-w-[1360px] p-4 sm:p-5 lg:p-8"><Outlet /></main>
        <footer className="mx-auto flex max-w-[1360px] flex-wrap items-center gap-3 px-5 pb-8 text-[12px] leading-[1.6] text-ink-3 lg:px-8">
          <span className="min-w-0 flex-1">데이터: 제주데이터허브(jejudatahub.net) 공개 목록 1,259종과 로그인 없이 받을 수 있는 원본, 같은 기관이 공공데이터포털(data.go.kr)에 공개한 파일 36종. 열 사전을 갖춰 계산하는 데이터는 71종(같은 형식 묶음 포함)입니다. 숫자는 모두 코드가 원본에서 계산하고, 문장만 이 PC에서 도는 소형 AI가 다듬습니다.</span>
        </footer>
      </div>
    </div>
  )
}
