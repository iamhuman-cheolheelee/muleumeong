package agent

// 질문 속 시점 읽기 — 「2021년」「작년 3월」「22년 1월 말~2월 중순」「2023년 상반기」「2015년 말이랑 2019년 말」.
// 시점 말은 찾은 자리를 지워(공백) 낱말 대조에서 빠지게 한다.

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

type TMention struct {
	S, E  int64 // 시작·끝 시점
	G     int   // 1 연 · 2 월 · 3 일
	Rel   bool  // 작년·올해 같은 상대 시점
	Text  string
	pos   int
	Since bool // 「이후·부터」만 있고 끝이 없다
	Last  bool // 「최근·대비」 — 데이터의 마지막 시점
}

type QTime struct {
	Ms      []TMention
	Kind    string // "" | point | range | pair | since
	S, E    int64
	G       int
	Rel     bool
	Now     bool   // 지금·요즘·현재·최근
	Refuse  string // 데이터가 있을 수 없는 시점(실시간·오늘·다음 주)
	Blanked string // 시점 말을 지운 질문
	LastN   int    // 「최근 5년」의 5
	LastG   int    // 1 해, 2 달
}

var (
	reLastN    = regexp.MustCompile(`(?:최근|지난)\s*(\d+|한|두|세|네|다섯|여섯|일곱|여덟|아홉|열)\s*(년|해|개월|달)`)
	reTNow     = regexp.MustCompile(`지금|현재|요즘|최근|최신|요새`)
	reTRealtim = regexp.MustCompile(`(?:지금|현재) ?\S* ?\S* ?(?:몇 ?명|사람).*(?:있어|있나|있니|붐벼|붐비)|실시간|이 ?시각|오늘|어제|그저께|내일|다음 ?주|이번 ?주|지난 ?주|다음 ?달|이번 ?달|지난 ?달|지난달|이번달|모레|방금`)
	reTVsNow   = regexp.MustCompile(`^\S*\s*(?:대비|에 비해|보다)\s.*(?:늘|줄|몇 ?배|퍼센트|%|떨어|감소율|증가율|변했|변화|차이|올랐|내렸|많아|적어|증가|감소)`)
	reTCmpNow = regexp.MustCompile(`(?:이랑|랑|과|와|하고|보다|대비|에 비해)\s*(?:비교해\S*\s*|견줘\S*\s*)?(?:최근|요즘|지금|현재|최신|요새)`)
	reTFuture  = regexp.MustCompile(`내년|후년|앞으로|다음 ?해`)
	// 하나의 시점 말: (연)? (상/하반기|분기)? (월 (일|초|중순|말))? — 연·월 중 하나는 있어야 한다
	reTOne = regexp.MustCompile(`(?:(재작년|작년|지난해|올해|금년|(\d)\s*년\s*전|((?:19|20)\d{2})\s*년(?:도)?|((?:19|20)\d{2})(?:\s|$|[~\-까부이에의엔은는도]|년)|(\d{2})\s*년(?:도)?(?:\s|[~에의엔은는도이랑과와부까보말상하초]|$))\s*)` +
		`(?:(상반기|하반기|([1-4])\s*분기|말|초|연말|연초)\s*)?` +
		`(?:(\d{1,2})\s*월\s*(?:(\d{1,2})\s*일|(초순|초|중순|말|하순))?)?` +
		`|(\d{1,2})\s*월\s*(?:(\d{1,2})\s*일|(초순|초|중순|말|하순))?`)
	reTRange = regexp.MustCompile(`~|부터|에서\s*\S*\s*까지|까지|사이`)
	reTSince = regexp.MustCompile(`^\s*(이후|부터|이래|후로|들어|들어서|이후로)`)
)

func atoi(s string) int { v, _ := strconv.Atoi(s); return v }

func ParseQTime(q string) QTime {
	qt := QTime{}
	if reTRealtim.MatchString(q) {
		qt.Refuse = "그 시점(오늘·이번 달·실시간)의 자료는 공개 데이터에 없어요"
	}
	if reTFuture.MatchString(q) {
		qt.Refuse = "아직 오지 않은 시점은 계산할 수 없어요"
	}
	qt.Now = reTNow.MatchString(q)
	if m := reLastN.FindStringSubmatch(q); m != nil {
		n := map[string]int{"한": 1, "두": 2, "세": 3, "네": 4, "다섯": 5, "여섯": 6, "일곱": 7, "여덟": 8, "아홉": 9, "열": 10}[m[1]]
		if n == 0 {
			fmt.Sscanf(m[1], "%d", &n)
		}
		if n > 0 {
			qt.LastN, qt.LastG = n, 1
			if m[2] == "개월" || m[2] == "달" {
				qt.LastG = 2
			}
		}
	}
	b := []byte(q)
	lastY := 0
	for _, m := range reTOne.FindAllStringSubmatchIndex(q, -1) {
		g := func(i int) string {
			if m[2*i] < 0 {
				return ""
			}
			return q[m[2*i]:m[2*i+1]]
		}
		whole := strings.TrimSpace(q[m[0]:m[1]])
		if whole == "" {
			continue
		}
		// 「1수원」「12시」처럼 시점이 아닌 숫자는 여기 걸리지 않는다(년·월이 붙어야 한다)
		y, rel := 0, false
		switch {
		case g(1) == "재작년":
			y, rel = ThisYear-2, true
		case g(1) == "작년" || g(1) == "지난해":
			y, rel = ThisYear-1, true
		case g(1) == "올해" || g(1) == "금년":
			y, rel = ThisYear, true
		case g(2) != "":
			y, rel = ThisYear-atoi(g(2)), true
		case g(3) != "":
			y = atoi(g(3))
		case g(4) != "":
			y = atoi(g(4))
		case g(5) != "":
			y = 2000 + atoi(g(5))
		}
		// 「2년 동안」「3년 사이」는 기간 길이지 해가 아니다
		if g(5) != "" {
			after := q[m[1]:]
			if regexp.MustCompile(`^\s*(동안|간|째|차|이상|넘)`).MatchString(after) || atoi(g(5)) < 10 {
				continue
			}
		}
		if g(2) != "" && g(1) == "" {
			// handled
		}
		part, qn := g(6), g(7)
		mo, day, sub := atoi(g(8)), atoi(g(9)), g(10)
		if y == 0 {
			mo, day, sub = atoi(g(11)), atoi(g(12)), g(13)
			if mo == 0 {
				continue
			}
			y = lastY
			if y == 0 {
				// 해 없이 달만 — 해를 알 수 없다
				qt.Ms = append(qt.Ms, TMention{G: 2, Text: whole, pos: m[0]})
				for i := m[0]; i < m[1]; i++ {
					b[i] = ' '
				}
				continue
			}
		}
		if mo > 12 || day > 31 {
			continue
		}
		lastY = y
		t := TMention{Rel: rel, Text: whole, pos: m[0]}
		switch {
		case mo > 0 && day > 0:
			t.S, t.E = span(y, mo, day, 0, 3)
			t.G = 3
		case mo > 0 && sub != "":
			ld := lastDay(y, mo)
			a, z := 1, ld
			switch sub {
			case "초", "초순":
				a, z = 1, 10
			case "중순":
				a, z = 11, 20
			case "말", "하순":
				a, z = 21, ld
			}
			t.S, t.E, t.G = tk(y, mo, a, 0), tk(y, mo, z, 23), 3
		case mo > 0:
			t.S, t.E = span(y, mo, 0, 0, 2)
			t.G = 2
		case part == "상반기":
			t.S, t.E, t.G = tk(y, 1, 1, 0), tk(y, 6, 30, 23), 2
		case part == "하반기":
			t.S, t.E, t.G = tk(y, 7, 1, 0), tk(y, 12, 31, 23), 2
		case qn != "":
			k := atoi(qn)
			t.S, t.E, t.G = tk(y, 3*k-2, 1, 0), tk(y, 3*k, lastDay(y, 3*k), 23), 2
		case part == "말" || part == "연말":
			t.S, t.E, t.G = tk(y, 12, 1, 0), tk(y, 12, 31, 23), 2
		case part == "초" || part == "연초":
			t.S, t.E, t.G = tk(y, 1, 1, 0), tk(y, 1, 31, 23), 2
		default:
			t.S, t.E = span(y, 0, 0, 0, 1)
			t.G = 1
		}
		if reTSince.MatchString(q[m[1]:]) {
			t.Since = true
		}
		qt.Ms = append(qt.Ms, t)
		for i := m[0]; i < m[1]; i++ {
			b[i] = ' '
		}
	}
	qt.Blanked = string(b)
	if qt.LastN > 0 {
		qt.Blanked = reLastN.ReplaceAllStringFunc(qt.Blanked, func(x string) string { return strings.Repeat(" ", len(x)) })
	}
	ms := qt.Ms
	for _, m := range ms {
		if m.S > 0 && tTime(m.S).Year() > ThisYear {
			qt.Refuse = "아직 오지 않은 시점(" + m.Text + ")은 계산할 수 없어요. 공개 데이터는 지난 기록만 담고 있어요"
		}
	}
	for _, m := range ms {
		if m.S == 0 {
			qt.Refuse = "몇 년도의 「" + m.Text + "」인지 알 수 없어요. 해를 같이 적어 주세요"
		}
	}
	switch {
	case len(ms) == 0:
	case len(ms) == 1:
		qt.Kind, qt.S, qt.E, qt.G, qt.Rel = "point", ms[0].S, ms[0].E, ms[0].G, ms[0].Rel
		if ms[0].Since {
			qt.Kind = "since"
		}
		// 「2016년 대비 얼마나 늘었어」「2008년이랑 최근 비교」 — 그해부터 마지막 자료까지 견준다
		if rest := q[ms[0].pos:]; reTVsNow.MatchString(rest) || (qt.Now && reTCmpNow.MatchString(rest)) {
			qt.Kind, qt.E = "pair", tk(ThisYear, 12, 31, 23)
			qt.Ms = append(qt.Ms, TMention{S: qt.E, E: qt.E, G: ms[0].G, Text: "최근", pos: len(q), Last: true})
		}
	default:
		a, z := ms[0], ms[len(ms)-1]
		between := q[a.pos:z.pos]
		qt.S, qt.E, qt.G, qt.Rel = a.S, z.E, max(a.G, z.G), a.Rel || z.Rel
		if reTRange.MatchString(between) || strings.Contains(q[z.pos:], "사이") {
			qt.Kind = "range"
		} else {
			qt.Kind = "pair"
		}
	}
	return qt
}
