package agent

// 계산 엔진 — 열 사전으로 펼친 긴 표 위에서만 계산한다.
// ① 질문 낱말을 모든 데이터셋의 열 사전에 대어 보고(남는 내용어가 없는 것만) ② 차원 값·지표·시점을 정하고
// ③ 연산(값·합·평균·최대·비교·변화·순위·추이·개수)을 고른 뒤 ④ 계산 결과 숫자만 정해진 틀에 넣는다.
// 하나라도 정할 수 없으면 숫자를 내지 않고 이유를 말한다.

import (
	"strconv"
	"fmt"
	"math"
	"regexp"
	"sort"
	"strings"
	"time"
)

var (
	reSumQ    = regexp.MustCompile(`합쳐|합친|합치|합해|합하면|합계|더해|더하면|통틀어|모두 합|전부 합`)
	reDirQ    = regexp.MustCompile(`증가했|감소했|늘었|줄었|올랐|내렸|높아졌|낮아졌`)
	reSuper   = regexp.MustCompile(`제일|가장|최고|최대|최저|최소|1위|바닥|최다|으뜸|젤`)
	reMinW    = regexp.MustCompile(`적은|적었|적게|적던|낮|최저|최소|추웠|추운|춥|가까|가깝|싼|싸|조용|바닥|덜|짧`)
	reMaxW    = regexp.MustCompile(`많|높|최고|최대|먼 |멀|비싼|비싸|더웠|더운|덥|시끄|길었|최다|자주|인기`)
	reWhich   = regexp.MustCompile(`어디|뭐|무슨|어느|어떤|누구|무엇`)
	reWhenDay = regexp.MustCompile(`날|며칠|일자|날짜`)
	reWhenMon = regexp.MustCompile(`달|몇 ?월|어느 ?월|월은|월이`)
	reWhenYr  = regexp.MustCompile(`해|몇 ?년|연도|년도|언제`)
	reTrendE   = regexp.MustCompile(`변했|변화|추이|추세|해마다|매년|흐름|바뀌|달라졌|어떻게 ?됐|증감`)
	reRankE    = regexp.MustCompile(`몇 ?위|순위|등수|다른 ?시도|다른 ?지역|어느 ?정도|다른 ?곳`)
	reBreak   = regexp.MustCompile(`별로|각각|따로따로`)
	reSign    = regexp.MustCompile(`플러스|마이너스|양수|음수`)
	reMeanW   = regexp.MustCompile(`평균`)
	reCntCue  = regexp.MustCompile(`몇 ?(곳|군데|개|개소|가지|집|호)`)
)

var reGyeonggi = regexp.MustCompile(`경기도|경기 ?지역|경기권|경기 ?(?:와|과|랑|하고|보다)|(?:와|과|랑|하고) 경기`)

var totalForm = setOf("계", "합계", "소계", "전체", "총계", "총합")

type pick struct {
	lt *LT
	g  *Ground
}

var engineReady bool

// lastTies 는 choose 가 하나로 못 정한 동점 후보(키가 서로 다른 것)를 담는다. askEngine 이 계산해 보고
// 모두 같은 답(같은 숫자·같은 1위)을 내면 그 답을 쓴다 — 어느 자료를 골라도 답이 같으면 되묻지 않는다.
type tieSet []pick

func initEngine() {
	if err := loadSchemas(); err != nil {
		fmt.Println("schema:", err)
		return
	}
	engineReady = true
	go func() {
		for _, id := range SchemaIDs() {
			LoadLT(id)
		}
	}()
}

// choose 는 질문 낱말이 모두 닿는 데이터셋 하나를 고른다.
func choose(qt QTime) (*pick, string, []Cand) {
	p, why, c, _ := chooseT(qt)
	return p, why, c
}

func chooseT(qt QTime) (*pick, string, []Cand, tieSet) {
	type cs struct {
		p   pick
		key string
	}
	ok := []cs{}
	near := []Cand{}
	nearSc := []int{}
	for _, id := range SchemaIDs() {
		lt, err := LoadLT(id)
		if err != nil || len(lt.Rows) == 0 {
			continue
		}
		g := GroundQ(qt.Blanked, lt)
		if len(g.Anchor) == 0 {
			continue
		}
		if len(g.Miss) > 0 {
			if d := lt.D; d != nil {
				mw := make([]string, len(g.Miss))
				for i, w := range g.Miss {
					if mw[i] = stripJosa(w); mw[i] == "" {
						mw[i] = w
					}
				}
				near = append(near, Cand{Hit: hitOf(d), Why: "질문의 " + quoteJo(mw, "이", "가") + " 이 데이터에 없어요"})
				nearSc = append(nearSc, g.Score*10-len(g.Miss))
			}
			continue
		}
		ok = append(ok, cs{pick{lt, g}, lt.S.Key})
	}
	if len(ok) == 0 {
		// 가장 가깝게 닿은 데이터를 앞에 둔다(거절 이유를 그 데이터 기준으로 말한다)
		ix := make([]int, len(near))
		for i := range ix {
			ix[i] = i
		}
		sort.SliceStable(ix, func(a, b int) bool { return nearSc[ix[a]] > nearSc[ix[b]] })
		sorted := make([]Cand, len(near))
		for i, k := range ix {
			sorted[i] = near[k]
		}
		return nil, "", sorted, nil
	}
	sort.SliceStable(ok, func(a, b int) bool { return ok[a].p.g.Score > ok[b].p.g.Score })
	best := ok[0].p.g.Score
	top := []cs{}
	keys := map[string]bool{}
	for _, c := range ok {
		// 한 점 차이라도 자료 고유 이름(strong)이 질문에 나온 자료는 함께 놓고 값이 갈리면 되묻는다
		if c.p.g.Score == best || (c.p.g.Score == best-1 && strongHit(c.p)) {
			top = append(top, c)
			keys[c.key] = true
		}
	}
	cands := []Cand{}
	for _, c := range top {
		cands = append(cands, Cand{Hit: hitOf(c.p.lt.D), Why: "질문과 똑같이 맞는 데이터가 여럿"})
	}
	if len(keys) > 1 && qt.S > 0 {
		// 물은 시점을 담은 데이터만 남긴다(2019년을 물었는데 2013~2018년 자료는 뺀다)
		kept := []cs{}
		kk := map[string]bool{}
		for _, c := range top {
			if ltCovers(c.p.lt, qt) {
				kept = append(kept, c)
				kk[c.key] = true
			}
		}
		// 그래도 여럿이면 한 시점 자료보다 시점이 이어지는 자료를 쓴다
		if len(kk) > 1 {
			ts := []cs{}
			tk := map[string]bool{}
			for _, c := range kept {
				if c.p.lt.Gran > 0 {
					ts = append(ts, c)
					tk[c.key] = true
				}
			}
			if len(ts) > 0 {
				kept, kk = ts, tk
			}
		}
		if len(kept) > 0 {
			top, keys = kept, kk
			cands = cands[:0]
			for _, c := range top {
				cands = append(cands, Cand{Hit: hitOf(c.p.lt.D), Why: "질문과 똑같이 맞는 데이터가 여럿"})
			}
		}
	}
	if len(keys) > 1 {
		// 원래 계산하던 데이터와 새로 더한 공공데이터포털 사본이 똑같이 맞으면 원래 것을 쓴다
		kept := []cs{}
		kk := map[string]bool{}
		for _, c := range top {
			if !isPub(c.p.lt.D.ID) {
				kept = append(kept, c)
				kk[c.key] = true
			}
		}
		if len(kept) > 0 && len(kept) < len(top) {
			top, keys = kept, kk
			cands = cands[:0]
			for _, c := range top {
				cands = append(cands, Cand{Hit: hitOf(c.p.lt.D), Why: "질문과 똑같이 맞는 데이터가 여럿"})
			}
		}
	}
	if len(keys) > 1 {
		// 같은 주제의 기본 자료(「인구」면 주민등록 인구)가 하나뿐이면 그것으로 답하고, 다른 자료는 안내만 한다
		var prim *pick
		np := 0
		for i := range top {
			if top[i].p.lt.S.Primary {
				if prim == nil || top[i].key != prim.lt.S.Key {
					np++
				}
				prim = &top[i].p
			}
		}
		if np == 1 {
			return prim, "", nil, nil
		}
	}
	if len(keys) > 1 {
		names := []string{}
		seen := map[string]bool{}
		for _, c := range top {
			if !seen[c.key] {
				seen[c.key] = true
				names = append(names, c.p.lt.S.Subject)
			}
		}
		ties := tieSet{}
		seen2 := map[string]bool{}
		for _, c := range top {
			if !seen2[c.key] {
				seen2[c.key] = true
				ties = append(ties, c.p)
			}
		}
		return nil, "질문이 「" + strings.Join(names, "」「") + "」 중 어느 자료를 물으신 건지 정할 수 없어요. 자료 이름에 든 낱말을 하나 더 적어 주세요", cands, ties
	}
	if len(top) == 1 {
		return &top[0].p, "", nil, nil
	}
	// 같은 묶음(안심코드 4주치·무장애 코스) 안에서 고르기
	s := top[0].p.lt.S
	if s.TitlePeriod && qt.Kind != "" {
		var bp *pick
		bj := 0.0
		for i := range top {
			lt := top[i].p.lt
			j := jaccard(qt.S, qt.E, lt.PS, lt.PE)
			if j > bj {
				bj, bp = j, &top[i].p
			}
		}
		if bp != nil && bj >= 0.6 {
			return bp, "", nil, nil
		}
		return nil, "그 기간과 맞는 4주 묶음 자료가 없어요(자료는 4주 단위로 나뉘어 있어요)", cands, nil
	}
	if s.TitlePeriod {
		return nil, "이 자료는 4주 단위로 나뉘어 있어요. 기간(예: 22년 1월 말~2월 중순)을 같이 적어 주세요", cands, nil
	}
	return nil, "같은 이름의 코스·자료가 여럿이라 하나로 정할 수 없어요. 코스 이름을 더 자세히 적어 주세요", cands, nil
}

func hasHangul(s string) bool {
	for _, r := range s {
		if r >= 0xAC00 && r <= 0xD7A3 {
			return true
		}
	}
	return false
}

// ro 는 「~로/~으로」를 붙인다.
func ro(w string) string {
	r := []rune(strings.TrimSpace(w))
	if len(r) > 0 {
		c := r[len(r)-1]
		if c >= 0xAC00 && c <= 0xD7A3 && (c-0xAC00)%28 != 0 && (c-0xAC00)%28 != 8 {
			return w + "으로"
		}
	}
	return w + "로"
}

func hitOf(d *Dataset) Hit {
	return Hit{ID: d.ID, Title: d.Title, Cat: d.Cat, Owner: d.Owner, Updated: d.Updated, Open: d.Open(), API: d.API, Header: d.Header, URL: d.URL()}
}

func tTime(t int64) time.Time {
	y, m, d := int(t/1000000), int(t/10000%100), int(t/100%100)
	if m == 0 {
		m = 1
	}
	if d == 0 {
		d = 1
	}
	return time.Date(y, time.Month(m), d, int(t%100), 0, 0, 0, time.UTC)
}

func days(a, b int64) float64 { return tTime(b).Sub(tTime(a)).Hours()/24 + 1 }

func jaccard(a1, a2, b1, b2 int64) float64 {
	s, e := a1, a2
	if b1 > s {
		s = b1
	}
	if b2 < e {
		e = b2
	}
	if e < s {
		return 0
	}
	inter := days(s, e)
	u1, u2 := a1, a2
	if b1 < u1 {
		u1 = b1
	}
	if b2 > u2 {
		u2 = b2
	}
	return inter / days(u1, u2)
}

// ── 계산 ─────────────────────────────────────────────

type eng struct {
	lt     *LT
	g      *Ground
	qt     QTime
	q      string // 시점 지운 질문(정규화)
	cue    string
	sel    map[int][]string
	notes  []string
	why    []string
	filt   []string
	mi     int // 지표 번호, -1 = 개수
	ci     int // 개수 번호
	gdim   int // 묶는 차원, -1 없음
	tgrp   int // 시점으로 묶기(1 연 2 월 3 일)
	two    []int // 두 지표를 맞대는 질문의 지표 번호
	S, E   int64
	period string
	used   map[int]bool
	sumAll bool // 행 값 지표를 질문대로 모두 더한다
	plus   []int  // 질문이 합치라고 한 다른 지표
	lbl    string // 합친 지표 이름
	lastN  bool   // 「최근 N년」으로 기간을 잡았다
}

func (e *eng) meas() *SMeas {
	if e.mi < 0 {
		return nil
	}
	return &e.lt.Meas[e.mi]
}

func (e *eng) unit() string {
	if m := e.meas(); m != nil {
		return m.Unit
	}
	return e.lt.S.Counts[e.ci].Unit
}

func (e *eng) label() string {
	if e.lbl != "" {
		return e.lbl
	}
	if m := e.meas(); m != nil {
		return m.Label
	}
	return e.lt.S.Counts[e.ci].Label
}

func (e *eng) dec() int {
	if m := e.meas(); m != nil {
		if m.Dec > 0 {
			return m.Dec
		}
		if m.Agg == "avg" {
			return 1
		}
	}
	return 0
}

func (e *eng) fv(v float64) string {
	d := e.dec()
	p := math.Pow(10, float64(d))
	v = math.Round(v*p) / p
	if d == 0 || v == math.Trunc(v) {
		return Fmt(v)
	}
	neg := v < 0
	ip := math.Trunc(math.Abs(v))
	s := Fmt(ip) + strings.TrimLeft(fmt.Sprintf("%.*f", d, math.Abs(v)-ip), "0")
	if neg {
		s = "-" + s
	}
	return s
}

func (e *eng) fd(v float64) string {
	if e.unit() == "%" {
		return e.fv(v) + "%p"
	}
	return e.fu(v)
}

func (e *eng) fu(v float64) string { return e.fv(v) + unitSp(e.unit()) }

func unitSp(u string) string {
	switch u {
	case "", "℃", "%":
		return u
	}
	if strings.ContainsAny(u[:1], "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ㎡") {
		return " " + u
	}
	return u
}

// 받침 있는 말 뒤엔 a, 없으면 b
// quoteJo 는 「가」「나」 꼴로 묶고 마지막 낱말에 맞는 조사를 붙인다.
func quoteJo(ws []string, a, b string) string {
	if len(ws) == 0 {
		return ""
	}
	last := ws[len(ws)-1]
	return "「" + strings.Join(ws, "」「") + "」" + strings.TrimPrefix(jo(last, a, b), last)
}

func jo(w, a, b string) string {
	r := []rune(strings.TrimSpace(w))
	if len(r) == 0 {
		return w + b
	}
	// 「여객 수(국내선+국제선)」처럼 괄호로 끝나면 괄호 앞 낱말로 정한다
	if r[len(r)-1] == ')' {
		if i := strings.LastIndex(string(r), "("); i > 0 {
			if k := []rune(strings.TrimSpace(string(r)[:i])); len(k) > 0 {
				r = k
			}
		}
	}
	c := r[len(r)-1]
	if c >= 0xAC00 && c <= 0xD7A3 {
		if (c-0xAC00)%28 != 0 {
			return w + a
		}
		return w + b
	}
	if c >= '0' && c <= '9' {
		if strings.ContainsRune("013678", c) {
			return w + a
		}
		return w + b
	}
	return w + a
}

func ansRefuse(msg string) (*Result, string) { return nil, msg }

// Run 은 고른 데이터셋으로 계산한다. 답할 수 없으면 (nil, 이유).
func Run(p *pick, qt QTime) (*Result, string) {
	lt, g := p.lt, p.g
	if n := len(qt.Ms); n > 0 && qt.Ms[n-1].Last && p.lt.Gran > 0 {
		ms := append([]TMention{}, qt.Ms...)
		gr := min(ms[n-1].G, lt.Gran)
		ms[n-1].S, ms[n-1].E = span(tYear(lt.TMax), tMonth(lt.TMax), int(lt.TMax/100%100), 0, gr)
		ms[n-1].G = gr
		qt.Ms, qt.E = ms, ms[n-1].E
	}
	e := &eng{lt: lt, g: g, qt: qt, q: string(g.Q), cue: normQ(g.Cue), sel: map[int][]string{}, mi: -1, ci: -1, gdim: -1, used: map[int]bool{}}
	s := lt.S
	if s.Year > 0 && qt.Kind != "" && !(tYear(qt.S) <= s.Year && s.Year <= tYear(qt.E)) {
		return ansRefuse(fmt.Sprintf("이 자료는 %d년 한 해 분석 자료라 그 시점 값은 없어요", s.Year))
	}
	// ── 차원 값
	qualBy := map[int]string{}
	termDims := map[int]bool{}
	termPos := map[int]int{}
	for _, o := range g.Occs {
		switch o.p.kind {
		case pVal:
			for _, v := range o.p.vals {
				if !contains(e.sel[o.p.dim], v) {
					e.sel[o.p.dim] = append(e.sel[o.p.dim], v)
				}
			}
		case pQual:
			qualBy[o.p.dim] = o.p.qual
		case pDimTerm:
			termDims[o.p.dim] = true
			for _, sp := range o.spans {
				if sp[1]-sp[0] >= 2 && sp[0] > termPos[o.p.dim] { // 「뭐」 같은 물음말은 자리를 보지 않는다
					termPos[o.p.dim] = sp[0]
				}
			}
		}
	}
	for d, k := range qualBy {
		vs := []string{}
		for _, v := range e.sel[d] {
			if strings.HasSuffix(v, "-"+k) {
				vs = append(vs, v)
			}
		}
		if len(vs) > 0 {
			e.sel[d] = vs
		}
	}
	// 같은 차원에서 꼬리만 다른 값(양유(더덕)-경작지·-임야)이 함께 잡히면 꼬리 없는 값은 합친다
	for d, vs := range e.sel {
		if len(lt.Dims[d].Qual) > 0 && len(vs) > 1 {
			bases := map[string]bool{}
			for _, v := range vs {
				b := v
				for k := range lt.Dims[d].Qual {
					b = strings.TrimSuffix(b, "-"+k)
				}
				bases[b] = true
			}
			if len(bases) == 1 {
				e.notes = append(e.notes, "재배지(밭·임야)를 따로 적지 않아 두 곳을 합쳤어요")
				e.sel[d] = vs
			}
		}
	}
	// ── 지역
	rdim := -1
	for i, d := range lt.Dims {
		if d.Region {
			rdim = i
			break
		}
	}
	regs := g.Regions
	if rdim >= 0 {
		for _, v := range e.sel[rdim] {
			regs = append(regs, canonRegion(v))
		}
	}
	mentioned := map[string]bool{}
	for _, r := range regs {
		mentioned[r] = true
	}
	switch s.Scope {
	case "서귀포":
		if mentioned["제주시"] {
			return ansRefuse("이 자료는 서귀포시만 담고 있어 제주시 값은 없어요")
		}
		other := false
		for r := range mentioned {
			if r != "제주" && r != "서귀포" && rdim >= 0 && !e.isRegionVal(rdim, r) {
				other = true
			}
		}
		if other {
			return ansRefuse("이 자료는 서귀포시 자료라 그 지역 값은 없어요")
		}
		if mentioned["제주"] && !mentioned["서귀포"] && len(e.sel[rdim]) == 0 {
			return ansRefuse("이 자료는 서귀포시만 담고 있어 제주 전체 값은 낼 수 없어요")
		}
	case "전국":
		if mentioned["제주"] || mentioned["서귀포"] || mentioned["제주시"] {
			return ansRefuse("이 자료는 우리나라 전체 통계라 제주만 따로 나뉘어 있지 않아요")
		}
		for r := range mentioned {
			if r != "전국" {
				return ansRefuse("이 자료는 우리나라 전체 통계라 지역별 값은 없어요")
			}
		}
	default:
		for r := range mentioned {
			if r == "제주" || r == "도내" {
				if rdim >= 0 {
					for _, v := range e.regionVals(rdim, "제주") {
						if !contains(e.sel[rdim], v) {
							e.sel[rdim] = append(e.sel[rdim], v)
						}
					}
				}
				continue
			}
			if rdim < 0 {
				if s.Scope == "제주" && (r == "서귀포" || r == "제주시") {
					return ansRefuse("이 자료는 제주 전체 값만 있어 행정시별로 나눌 수 없어요")
				}
				return ansRefuse("이 자료에는 「" + r + "」 값이 없어요(" + s.Subject + ")")
			}
			if !e.isRegionVal(rdim, r) {
				return ansRefuse("이 자료에는 「" + r + "」 값이 없어요")
			}
			vs := e.regionVals(rdim, r)
			for _, v := range vs {
				if !contains(e.sel[rdim], v) {
					e.sel[rdim] = append(e.sel[rdim], v)
				}
			}
		}
	}
	// 「장사 경기」의 경기는 지역(경기도)이 아니다
	if !reGyeonggi.MatchString(e.q) {
		for d, vs := range e.sel {
			if !lt.Dims[d].Region {
				continue
			}
			kept := vs[:0]
			for _, v := range vs {
				if canonRegion(v) != "경기" {
					kept = append(kept, v)
				}
			}
			if len(kept) == 0 {
				delete(e.sel, d)
			} else {
				e.sel[d] = kept
			}
		}
	}
	// 「1수원감시정1」처럼 자리표시가 아닌 모든 차원 — 값 안에 든 낱말
	for d, ws := range g.Contain {
		vs := []string{}
		for _, v := range lt.Vals[d] {
			lv := strings.ToLower(v)
			all := true
			for _, w := range ws {
				if !strings.Contains(lv, w) {
					all = false
				}
			}
			if all {
				vs = append(vs, v)
			}
		}
		// 「안내 지점이 몇 개」처럼 낱말이 차원 이름 바로 앞에서 꾸미는 말로 쓰였으면
		// 값 이름 속 낱말로 걸러야 할지(1곳) 차원 전체를 말한 것인지(전부) 정할 수 없다
		for _, w := range ws {
			for _, t := range append([]string{lt.Dims[d].Label}, lt.Dims[d].Terms...) {
				if t != "" && (strings.Contains(e.q, w+" "+t) || strings.Contains(e.q, w+t)) && len(vs) < len(lt.Vals[d]) {
					return ansRefuse("「" + w + " " + t + "」이 이름에 「" + w + "」가 든 " + t + "인지, " + t + " 전체인지 정할 수 없어요. 「이름에 " + w + "가 든」처럼 적어 주세요")
				}
			}
		}
		e.sel[d] = vs
		e.filt = append(e.filt, lt.Dims[d].Label+"에 「"+strings.Join(ws, "·")+"」")
	}
	// ── 지표
	if r := e.pickMeasure(termDims); r != "" {
		return ansRefuse(r)
	}
	if r := e.checkKind(); r != "" {
		return ansRefuse(r)
	}
	// ── 연산 정하기
	super := reSuper.MatchString(e.cue) || reTopN.MatchString(e.q)
	which := reWhich.MatchString(e.cue)
	// 「많이 오는 나라 순위」 — 지역을 하나 고르지 않았으면 순위는 묶음별 줄 세우기다
	if !super && reRankList.MatchString(e.cue) && (rdim < 0 || len(e.sel[rdim]) == 0) && !reRankE2.MatchString(e.cue) {
		super, which = true, true
	}
	// 「교통사고 많이 나는 해는」 — 제일이 없어도 많은 해·달을 물으면 가장 많은 때다
	if !super && lt.Gran > 0 && reManyWhen.MatchString(e.cue) {
		super = true
	}
	for d := range termDims {
		if lt.Dims[d].Flag != "" {
			e.sel[d] = []string{lt.Dims[d].Flag}
			e.filt = append(e.filt, lt.Dims[d].Label+" 있음")
			delete(termDims, d)
		}
	}
	gdim := -1
	// 차원 말이 여럿이면 물음말에 가장 가까운(질문 뒤쪽) 말로 묶는다 — 「관광지 상품 … 관광지 유형은 뭐」는 유형
	for d := range termDims {
		if e.headOfFilter(d) {
			continue
		}
		if len(e.sel[d]) <= 1 && (gdim < 0 || termPos[d] > termPos[gdim] || (termPos[d] == termPos[gdim] && d < gdim)) {
			gdim = d
		}
	}
	// 「가장 많았던 해·달」 — 시점을 묻는 질문은 기본 묶음(국적·관광지)보다 시점으로 묶는다
	timeAsk := super && lt.Gran > 0 && reTimeAsk.MatchString(e.cue)
	if gdim < 0 && !timeAsk && (which || super) && (super || reBreak.MatchString(e.cue)) {
		if s.Group != "" {
			for i, d := range lt.Dims {
				if d.Col == s.Group || d.Label == s.Group {
					gdim = i
				}
			}
		}
	}
	if gdim < 0 && which && super && !timeAsk {
		for i, d := range lt.Dims {
			if d.Entity || d.Attr {
				gdim = i
				break
			}
		}
	}
	// 「가장 많은 특용작물은 뭐」 — 주제 말 안에 차원 말(작물)이 있으면 그 차원으로 묶는다
	if gdim < 0 && which && super && !timeAsk {
		for _, o := range e.g.Occs {
			if o.p.kind != pSubj && o.p.kind != pTitle {
				continue
			}
			for i, d := range lt.Dims {
				if d.Region || d.Flag != "" || len(e.sel[i]) > 0 || gdim >= 0 {
					continue
				}
				for _, t := range d.Terms {
					if len([]rune(t)) >= 2 && strings.Contains(string(o.p.text), t) {
						gdim = i
						break
					}
				}
			}
		}
	}
	// 「가장 많은 달」인데 자료가 해 단위면 다른 값(한 해 합계)을 내지 않는다
	if super && gdim < 0 && reTimeAsk.MatchString(e.cue) {
		if reWhenDay.MatchString(e.cue) && lt.Gran < 3 && !reWhenMon.MatchString(e.cue) {
			return ansRefuse("이 자료는 날짜별 값이 없어 어느 날이 가장 많은지 알 수 없어요")
		}
		if reWhenMon.MatchString(e.cue) && lt.Gran < 2 {
			return ansRefuse("이 자료는 한 해 단위 값만 있어 어느 달이 가장 많은지 알 수 없어요")
		}
	}
	// 시점으로 묶기
	if super && gdim < 0 && lt.Gran > 0 {
		switch {
		case reWhenDay.MatchString(e.cue) && lt.Gran >= 3:
			e.tgrp = 3
		case reWhenMon.MatchString(e.cue) && lt.Gran >= 2:
			e.tgrp = 2
		case reWhenYr.MatchString(e.cue):
			e.tgrp = 1
		}
	}
	// 묶는 차원에 한 값만 고른 경우(「제일 많은 시도는 어디고 제주는」) — 묶음은 전체로, 고른 값은 따로 알려 준다
	var also []string
	// 「베트남 국적 외국인 몇 명」 — 어디·제일을 묻지 않았는데 한 값만 골랐으면 묶지 않고 그 값으로 거른다
	if gdim >= 0 && len(e.sel[gdim]) == 1 && !which && !super {
		gdim = -1
	}
	// 「베트남 사람이 제일 많은 곳은 제주시야 서귀포시야」 — 다른 차원에서 두 값을 견주면 한 값 고른 차원은 거르기
	if gdim >= 0 && len(e.sel[gdim]) == 1 {
		for d, vs := range e.sel {
			if d != gdim && len(vs) >= 2 {
				gdim = -1
				break
			}
		}
	}
	if gdim >= 0 && len(e.sel[gdim]) > 0 {
		also = e.sel[gdim]
		delete(e.sel, gdim)
	}
	e.gdim = gdim
	// 비교할 차원
	cmpDim := -1
	for d, vs := range e.sel {
		if len(vs) >= 2 && d != gdim {
			cmpDim = d
		}
	}
	// 「도착·출발 합쳐서」 — 고른 값들을 견주지 않고 더한다
	if cmpDim >= 0 && reSumQ.MatchString(e.q) {
		cmpDim = -1
	}
	// Must 차원
	for i, d := range lt.Dims {
		if d.Must && len(e.sel[i]) == 0 && i != gdim {
			opts := []string{}
			for _, v := range lt.Vals[i] {
				if len(opts) < 6 {
					opts = append(opts, v)
				}
			}
			return ansRefuse(fmt.Sprintf("어느 %s인지 정해 주세요(%s 중 하나). 서로 더할 수 없는 값이라 정하지 않으면 계산하지 않아요", d.Label, strings.Join(opts, "·")))
		}
	}
	if m := e.meas(); m != nil {
		for _, nd := range m.Need {
			for i, d := range lt.Dims {
				if (d.Label == nd || d.Col == nd) && len(e.sel[i]) == 0 && i != gdim {
					return ansRefuse("어느 " + d.Label + "인지 정해 주세요")
				}
			}
		}
	}
	// ── 시점
	if r := e.window(super, gdim); r != "" {
		return ansRefuse(r)
	}
	// 「최근 5년」 — 자료의 마지막 시점에서 거꾸로 N해(N달)를 잡는다
	if qt.LastN > 0 && lt.Gran > 0 && lt.TMax > 0 {
		y, mo := int(lt.TMax/1000000), int(lt.TMax/10000%100)
		var s0, e0 int64
		if qt.LastG == 2 && lt.Gran >= 2 {
			k := y*12 + mo - 1 - (qt.LastN - 1)
			s0, _ = span(k/12, k%12+1, 1, 0, 2)
			_, e0 = span(y, mo, 1, 0, 2)
		} else {
			s0, _ = span(y-qt.LastN+1, 1, 1, 0, 1)
			_, e0 = span(y, 1, 1, 0, 1)
		}
		e.S, e.E = s0, e0
		e.lastN = true
		e.period = fmtT(s0, lt.Gran) + "~" + fmtT(lt.TMax, lt.Gran)
		kept := e.notes[:0]
		for _, n := range e.notes {
			if !strings.HasPrefix(n, "시점을 적지 않") {
				kept = append(kept, n)
			}
		}
		e.notes = append(kept, fmt.Sprintf("「최근 %d%s」은 자료의 마지막 시점(%s)에서 거꾸로 잡았어요", qt.LastN, map[int]string{1: "년", 2: "개월"}[qt.LastG], fmtT(lt.TMax, lt.Gran)))
	}
	res := &Result{Measure: e.label(), Unit: e.unit(), Notes: []string{}, Filters: []string{}, Facts: []Fact{}}
	if m := e.meas(); m != nil {
		res.Agg = m.Agg
	} else {
		res.Agg = "count"
	}
	var r string
	switch {
	case len(e.two) == 2:
		r = e.opTwoMeas(res)
	case qt.Kind == "pair" && cmpDim >= 0:
		// 「2015년 이후 중국은 일본의 몇 배? 2019년 기준」 — 두 값을 한 시점(기준 또는 나중 시점)에서 견준다
		if !(reRatioQ.MatchString(e.q) || reDiffQ.MatchString(e.q) || strings.Contains(e.q, "기준")) {
			r = "두 시점과 두 값을 한꺼번에 견주진 못해요. 시점이나 값을 하나로 줄여 물어 주세요"
			break
		}
		z := qt.Ms[len(qt.Ms)-1]
		for _, m := range qt.Ms {
			if m.S > z.S {
				z = m
			}
		}
		e.S, e.E = z.S, z.E
		e.period = periodText(z.S, z.E, z.G, lt.Gran)
		r = e.opCompare(res, cmpDim)
	case qt.Kind == "pair", qt.Kind == "range" && reDirQ.MatchString(e.q) && !super && gdim < 0:
		r = e.opChange(res)
	case cmpDim >= 0:
		r = e.opCompare(res, cmpDim)
	case rdim >= 0 && gdim == rdim && reRankE.MatchString(e.cue) && len(also) == 1 && !super:
		e.sel[rdim] = also
		r = e.opRank(res, rdim)
	case gdim >= 0 && (super || which):
		r = e.opArg(res, gdim, also)
	case e.tgrp > 0:
		r = e.opArgTime(res)
	case super && e.meas() != nil && e.meas().Row:
		r = e.opRowPeak(res)
	case rdim >= 0 && reRankE.MatchString(e.cue) && len(e.sel[rdim]) == 1:
		r = e.opRank(res, rdim)
	case gdim >= 0 && reBreak.MatchString(e.cue):
		r = e.opArg(res, gdim, also)
	case qt.Kind == "point" && len(qt.Ms) == 1 && qt.Ms[0].G == 1 && lt.Gran > 0 && reHowMuchChg.MatchString(e.q) && !super && gdim < 0 && cmpDim < 0:
		// 「물가 작년에 얼마나 올랐어」 — 한 해만 말하고 얼마나 올랐냐 물으면 그 전해와 견준다
		z := qt.Ms[0]
		if e.S > 0 {
			z.S, z.E = e.S, e.E
		}
		a := z
		a.S, a.E, a.Text = z.S-1000000, z.E-1000000, ""
		e.qt.Ms = []TMention{a, z}
		r = e.opChange(res)
	case qt.Kind == "point" && len(qt.Ms) == 1 && lt.Gran > 0 && strings.Contains(e.q, "대비") && !strings.Contains(e.q, "전년") && reDirAsk.MatchString(e.q) && !super && gdim < 0 && cmpDim < 0 && qt.Ms[0].E < lt.TMax:
		a := qt.Ms[0]
		z := a
		z.S, z.E, z.Text = lt.TMax-(a.E-a.S), lt.TMax, ""
		e.qt.Ms = []TMention{a, z}
		r = e.opChange(res)
	case rankDim(e, lt) >= 0 && reRankOnly.MatchString(e.q) && !super:
		r = e.opRank(res, rankDim(e, lt))
	case reDirAsk.MatchString(e.q) && lt.Gran > 0 && (qt.Kind == "" || reTNow.MatchString(e.q)) && !super && gdim < 0 && cmpDim < 0:
		// 「출생아 줄었나요」「인구 요즘 늘어요 줄어요」 — 시점이 없으면 자료 첫해부터 마지막 해까지 흐름으로 답한다
		if !e.lastN {
			e.S, e.E = 0, 0
		}
		r = e.opTrend(res)
	case reTrendE.MatchString(e.cue) && lt.Gran > 0 && qt.Kind != "point", lt.Gran >= 2 && reMonthly.MatchString(e.q) && !reMonSum.MatchString(e.q) && !super && gdim < 0:
		r = e.opTrend(res)
	default:
		r = e.opValue(res)
	}
	if r != "" {
		return ansRefuse(r)
	}
	res.Period = e.period
	if lt.Gran <= 1 && reMonthly.MatchString(e.q) {
		e.notes = append(e.notes, "이 데이터는 해마다 한 값만 있어 월별로 나눌 수 없어요. 해 단위 값으로 답했습니다.")
	}
	res.Text = dedupWords(res.Text)
	res.Notes = append(res.Notes, e.notes...)
	res.Filters = append(res.Filters, e.filters()...)
	res.RowsTotal = len(lt.Raw)
	res.RowsUsed = len(e.used)
	e.evidence(res)
	res.Cols = Profile(lt.T, ColDict)
	res.Plan.Why = append(e.why, res.Plan.Why...)
	res.Code = e.code(res)
	return res, ""
}

func (e *eng) isRegionVal(d int, r string) bool { return len(e.regionVals(d, r)) > 0 }

func (e *eng) regionVals(d int, r string) []string {
	out := []string{}
	for _, v := range e.lt.Vals[d] {
		c := canonRegion(v)
		if c == r || (r == "서귀포" && strings.HasPrefix(v, "서귀포")) {
			out = append(out, v)
		}
	}
	return out
}

var reRankList = regexp.MustCompile(`순위|랭킹|순서대로|많은 ?순|많이 \S{0,4} ?(?:나라|곳|데)`)
var reRankE2 = regexp.MustCompile(`몇 ?위|등수`)
var reManyWhen = regexp.MustCompile(`(?:많|높|적|낮|붐)\S*\s+\S*\s*(?:해|달|연도|년도|때)(?:는|가|은|야|예요|에요|$|\?|\s)`)
var reDirAsk = regexp.MustCompile(`늘었|줄었|늘어|줄어|늘고|줄고|올랐|내렸|오르|떨어졌|많아졌|적어졌|증가했|감소했|높아졌|낮아졌`)

var reMonthly = regexp.MustCompile(`월별|달별|매달|월마다|달마다|월간 추이`)

var reMonSum = regexp.MustCompile(`합계|합쳐|합치|모두|총|한 해|전체|더하`)

var reRatioQ = regexp.MustCompile(`몇 ?배|배나|배 ?많|배 ?높|회복률`)
var reDiffQ = regexp.MustCompile(`차이|얼마나 더|몇 명 더|더 많아\?|격차`)

var reTimeAsk = regexp.MustCompile(`(?:많|높|적|낮|컸|크|작|더웠|추웠|붐볐)\S*\s*(?:해|달|날|연도|년도|때|시기)(?:$|[는은가이야요?\s])|몇 ?년|언제|어느 ?해|어느 ?달|몇 ?월|어느 ?연도`)

var reTopN = regexp.MustCompile(`\d+ ?위까지|상위 ?\d+|\d+ ?위 ?안|톱 ?\d+|top ?\d+|별 ?순위|순위 ?\d+ ?개|\d+ ?(?:개|곳|위)만`)

var reTopNum = regexp.MustCompile(`(\d+) ?(?:위|곳|개|군데|나라)`)

// topN 은 「3위까지」「상위 5곳」의 숫자(최대 10)를, 없으면 0을 준다.
func topN(q string) int {
	if !reTopN.MatchString(q) && !strings.Contains(q, "상위") {
		return 0
	}
	m := reTopNum.FindStringSubmatch(q)
	if m == nil {
		return 0
	}
	n, _ := strconv.Atoi(m[1])
	if n > 10 {
		n = 10
	}
	return n
}

var reTimesQ = regexp.MustCompile(`몇 ?번|몇 ?회|몇 ?일이나|몇 ?일 ?있|몇 ?날|횟수|일수|날 수`)

var reTwoMeasCmp = regexp.MustCompile(`보다|비교|차이|견줘|대비`)

func (e *eng) pickMeasure(termDims map[int]bool) string {
	lt, g := e.lt, e.g
	type mo struct{ idx, ln int }
	var ms []mo
	cnt, cntLn := -1, 0
	for _, o := range g.Occs {
		switch o.p.kind {
		case pMeas, pPlus:
			ln := len(o.p.text)
			if weakMeas[string(o.p.text)] {
				ln = 0
				if lt.Meas[o.p.idx].Def {
					ln = 1
				}
			}
			ms = append(ms, mo{o.p.idx, ln})
		case pSubj, pTitle:
			for i, m := range lt.Meas {
				for _, t := range m.Terms {
					tr := runesOf(t)
					if len(tr) >= 2 && !weakMeas[t] && !strings.Contains(t, "+") && strings.Contains(string(o.p.text), string(tr)) {
						// 주제 말에 기본 지표 이름이 들어 있으면(「외국인 주민」) 질문의 다른 지표 말(「남자」)이 이긴다
						l := len(tr)
						if m.Def {
							l = 1
						}
						ms = append(ms, mo{i, l})
					}
				}
			}
		case pCount:
			if len(o.p.text) > cntLn {
				cnt, cntLn = o.p.idx, len(o.p.text)
			}
		}
	}
	sort.SliceStable(ms, func(a, b int) bool { return ms[a].ln > ms[b].ln })
	// 질문의 지표 말 여럿을 한 지표 이름이 다 품으면(「전기차」+「등록 대수」→「전기차 등록 대수」) 그 지표
	{
		var txt []string
		for _, o := range g.Occs {
			if (o.p.kind == pMeas || o.p.kind == pPlus) && !weakMeas[string(o.p.text)] {
				txt = append(txt, strings.ReplaceAll(string(o.p.text), " ", ""))
			}
		}
		if len(txt) >= 2 {
			bi, bc, bl := -1, 0, 0
			for i, m := range lt.Meas {
				lb := strings.ReplaceAll(m.Label, " ", "")
				c, cov := 0, 0
				for _, t := range txt {
					if strings.Contains(lb, t) {
						c++
						cov += len(t)
					}
				}
				ll := len([]rune(lb))
				if c >= 2 && (cov > bc || (cov == bc && ll < bl)) {
					bi, bc, bl = i, cov, ll
				}
			}
			if bi >= 0 && bc > ms[0].ln {
				ms = append([]mo{{bi, bc}}, ms...)
			}
		}
	}
	// 「사업용, 비사업용 합쳐서」 — 단위·집계가 같은 지표 여럿을 합치라고 하면 더한다
	if reSumQ.MatchString(e.q) {
		var ids []int
		seen := map[int]bool{}
		for _, o := range g.Occs {
			if o.p.kind == pMeas && !weakMeas[string(o.p.text)] && len([]rune(string(o.p.text))) >= 2 && !seen[o.p.idx] {
				seen[o.p.idx] = true
				ids = append(ids, o.p.idx)
			}
		}
		if len(ids) >= 2 {
			a := lt.Meas[ids[0]]
			same := a.Agg == "sum" || a.Agg == "stock"
			var ls []string
			for _, k := range ids {
				m := lt.Meas[k]
				if m.Unit != a.Unit || m.Agg != a.Agg || m.Row {
					same = false
				}
				ls = append(ls, m.Label)
			}
			if !same {
				return "합치려는 값(「" + strings.Join(ls, "」「") + "」)의 단위나 성격이 달라 더할 수 없어요"
			}
			e.mi, e.plus = ids[0], ids[1:]
			e.lbl = strings.Join(ls, "·") + " 합계"
			e.why = append(e.why, "지표 합: 「"+strings.Join(ls, "」+「")+"」")
			return ""
		}
	}
	// 두 지표를 맞대는 질문(「소비자물가가 생활물가보다 높았어?」)은 한 지표 값만 내면 오답이 된다
	if reTwoMeasCmp.MatchString(e.q) {
		seen := map[int]string{}
		for _, o := range g.Occs {
			if o.p.kind == pMeas && !weakMeas[string(o.p.text)] && len(o.p.text) >= 2 {
				seen[o.p.idx] = lt.Meas[o.p.idx].Label
			}
			if o.p.kind == pSubj || o.p.kind == pTitle {
				for i, m := range lt.Meas {
					for _, t := range m.Terms {
						if len([]rune(t)) >= 3 && !weakMeas[t] && strings.ReplaceAll(t, " ", "") == strings.ReplaceAll(string(o.p.text), " ", "") {
							seen[i] = m.Label
						}
					}
				}
			}
		}
		if len(seen) == 2 && e.qt.Kind != "pair" && e.qt.Kind != "range" {
			var ids []int
			for i := range seen {
				ids = append(ids, i)
			}
			pos := func(i int) int {
				best := 1 << 30
				for _, t := range append([]string{lt.Meas[i].Label}, lt.Meas[i].Terms...) {
					if k := strings.Index(e.q, t); k >= 0 && k < best {
						best = k
					}
				}
				return best
			}
			sort.Slice(ids, func(a, b int) bool { return pos(ids[a]) < pos(ids[b]) })
			if lt.Meas[ids[0]].Unit == lt.Meas[ids[1]].Unit && lt.Meas[ids[0]].Agg == lt.Meas[ids[1]].Agg {
				e.two = ids
				e.mi = ids[0]
				e.why = append(e.why, "지표 둘: 「"+lt.Meas[ids[0]].Label+"」·「"+lt.Meas[ids[1]].Label+"」")
				return ""
			}
		}
		if len(seen) >= 2 {
			var ls []string
			for _, l := range seen {
				ls = append(ls, "「"+l+"」")
			}
			sort.Strings(ls)
			return "두 지표(" + strings.Join(ls, "·") + ")를 맞대는 계산은 아직 못 해요. 지표마다 따로 물어 주세요"
		}
	}
	mi := -1
	if len(ms) > 0 {
		mi = ms[0].idx
		for _, x := range ms[1:] {
			if x.ln == ms[0].ln && x.idx != mi {
				a, b := lt.Meas[mi], lt.Meas[x.idx]
				if b.Strong && !a.Strong {
					mi = x.idx
				} else if a.Strong == b.Strong {
					return "「" + a.Label + "」과 「" + b.Label + "」 중 어느 값을 물으시는지 정할 수 없어요"
				}
			}
		}
		// 낱말이 여러 지표에 걸치면 가장 긴 것
	}
	if cnt >= 0 && (mi < 0 || cntLn > ms[0].ln) {
		e.ci = cnt
		e.why = append(e.why, "「"+lt.S.Counts[cnt].Label+"」를 셉니다")
		return ""
	}
	cue := reCntCue.FindStringSubmatch(e.q)
	if mi < 0 {
		for i, m := range lt.Meas {
			if m.Def {
				mi = i
			}
		}
		if mi < 0 && len(lt.Meas) == 1 {
			mi = 0
		}
		if (mi < 0 || cue != nil) && len(lt.S.Counts) > 0 {
			if mi >= 0 && cue != nil && unitAccepts(lt.Meas[mi].Unit, cue[1]) {
				// 단위가 맞는 지표가 있으면 그대로
			} else {
				for i, c := range lt.S.Counts {
					if c.Def || e.ci < 0 {
						e.ci = i
					}
				}
				if e.ci >= 0 {
					e.mi = -1
					e.why = append(e.why, "「"+lt.S.Counts[e.ci].Label+"」를 셉니다")
					return ""
				}
			}
		}
		if mi < 0 {
			return "무엇을 계산할지(지표) 정할 수 없어요"
		}
	} else if cue != nil && !unitAccepts(lt.Meas[mi].Unit, cue[1]) && len(lt.S.Counts) > 0 {
		for i, c := range lt.S.Counts {
			if c.Def || e.ci < 0 {
				e.ci = i
			}
		}
		e.mi = -1
		return ""
	}
	// 「더운 날 몇 번」 — 날·횟수를 세라는데 지표가 온도·금액이면 다른 값을 내지 않는다
	if reTimesQ.MatchString(e.q) && !strings.ContainsAny(lt.Meas[mi].Unit, "회번건일") {
		return "「몇 번」처럼 날 수·횟수를 세는 계산은 이 데이터로 할 수 없어요"
	}
	e.mi = mi
	e.why = append(e.why, "지표: 「"+lt.Meas[mi].Label+"」("+lt.Meas[mi].Unit+")")
	return ""
}

// 질문이 요구하는 숫자 종류(단위·합계)와 고른 값이 어긋나면 다른 값을 내지 않는다.
// 단위가 맞는 개수·지표가 하나뿐이면 그것으로 바꾸고, 아니면 거절한다.
var reUnitCue = regexp.MustCompile(`몇 ?(건|번|회|명|분|곳|군데|개소|집|호|대|원|점|도|명이|개)`)

func unitFam(u string) string {
	switch u {
	case "건", "번", "회":
		return "event"
	case "명", "분":
		return "person"
	case "곳", "군데", "개소", "집", "호":
		return "place"
	case "원", "천원", "만원", "백만원", "억원":
		return "money"
	}
	return ""
}

func (e *eng) checkKind() string {
	lt := e.lt
	if m := reUnitCue.FindStringSubmatch(e.q); m != nil {
		want := unitFam(m[1])
		have := unitFam(e.unit())
		if want != "" && have != "" && want != have {
			alt := -1
			n := 0
			for i, c := range lt.S.Counts {
				if unitFam(c.Unit) == want {
					alt, n = i, n+1
				}
			}
			if n == 1 {
				e.mi, e.ci = -1, alt
				e.why = append(e.why, "「"+lt.S.Counts[alt].Label+"」를 셉니다(단위 「"+m[1]+"」)")
				return ""
			}
			return "「몇 " + m[1] + "」을 물으셨는데 이 자료에서 고른 값은 「" + e.label() + "」(" + e.unit() + ")이라 맞지 않아요"
		}
	}
	sq := e.q
	if mm := e.meas(); mm != nil {
		for _, t := range append([]string{mm.Label}, mm.Terms...) {
			if t != "" {
				sq = strings.ReplaceAll(sq, t, " ")
			}
		}
	}
	if mm := e.meas(); mm != nil && mm.Agg == "avg" && reSumQ.MatchString(sq) {
		if !mm.Row || e.lt.Meas[e.mi].Unit == "점" || e.lt.Meas[e.mi].Unit == "도" || e.lt.Meas[e.mi].Unit == "%" {
			return "「" + mm.Label + "」은 더해서 의미가 있는 값이 아니에요"
		}
		e.sumAll = true
		e.notes = append(e.notes, "행마다 적힌 「"+mm.Label+"」을 모두 더한 합계예요")
	}
	return ""
}

func unitAccepts(u, cue string) bool {
	switch cue {
	case "곳", "군데", "개", "개소", "가지":
		return u == "곳" || u == "개" || u == "호"
	case "집", "호":
		return u == "호" || u == "곳"
	}
	return true
}

// rowsFor 는 고른 값·기본값·합계 행 규칙으로 행을 거른다.
func (e *eng) rowsFor(S, E int64, skip int) []int {
	lt := e.lt
	out := []int{}
	allow := make([]map[string]bool, len(lt.Dims))
	deny := make([]map[string]bool, len(lt.Dims))
	for i, d := range lt.Dims {
		if i == skip {
			deny[i] = setOf(d.Totals...)
			for _, v := range d.Exclude {
				deny[i][v] = true
			}
			for _, v := range lt.Vals[i] {
				if totalForm[strings.TrimSpace(v)] || (d.Region && canonRegion(v) == "전국") {
					deny[i][v] = true
				}
			}
			continue
		}
		if vs := e.sel[i]; len(vs) > 0 {
			allow[i] = setOf(vs...)
			continue
		}
		if d.Default != "" && contains(lt.Vals[i], d.Default) {
			allow[i] = setOf(d.Default)
			continue
		}
		if d.Region {
			if vs := e.regionVals(i, "제주"); len(vs) > 0 && e.lt.S.Scope == "시도" {
				allow[i] = setOf(vs...)
				continue
			}
		}
		if len(d.Totals) > 0 {
			has := false
			for _, t := range d.Totals {
				if contains(lt.Vals[i], t) {
					has = true
				}
			}
			if has {
				allow[i] = setOf(d.Totals...)
				continue
			}
		}
		if d.Flag != "" {
			continue
		}
		deny[i] = setOf(d.Exclude...)
		for _, v := range lt.Vals[i] {
			if totalForm[strings.TrimSpace(v)] {
				deny[i][v] = true
			}
		}
	}
	for ri, r := range lt.Rows {
		ok := true
		for i := range lt.Dims {
			v := r.D[i]
			if allow[i] != nil && !allow[i][v] {
				ok = false
				break
			}
			if deny[i] != nil && deny[i][v] {
				ok = false
				break
			}
		}
		if !ok {
			continue
		}
		if S > 0 && r.G > 0 && (r.T < S || r.E > E) {
			continue
		}
		if e.mi >= 0 && math.IsNaN(r.V[e.mi]) {
			continue
		}
		out = append(out, ri)
	}
	return out
}

// agg 는 행 묶음 하나의 값을 낸다.
func (e *eng) agg(ix []int) (float64, bool) {
	lt := e.lt
	if len(ix) == 0 {
		return 0, false
	}
	for _, i := range ix {
		e.used[lt.Rows[i].R] = true
	}
	if e.mi >= 0 && len(e.plus) > 0 {
		mi, pl := e.mi, e.plus
		e.plus = nil
		tot, ok := e.agg(ix)
		for _, k := range pl {
			e.mi = k
			v, ok2 := e.agg(ix)
			tot += v
			ok = ok && ok2
		}
		e.mi, e.plus = mi, pl
		return tot, ok
	}
	if e.mi < 0 {
		c := lt.S.Counts[e.ci]
		if c.Distinct == "" {
			return float64(len(ix)), true
		}
		di := -1
		for i, d := range lt.Dims {
			if d.Col == c.Distinct {
				di = i
			}
		}
		if di < 0 {
			return float64(len(ix)), true
		}
		seen := map[string]bool{}
		for _, i := range ix {
			seen[strings.ReplaceAll(strings.ToLower(lt.Rows[i].D[di]), " ", "")] = true
		}
		return float64(len(seen)), true
	}
	m := lt.Meas[e.mi]
	switch m.Agg {
	case "avg":
		s := 0.0
		for _, i := range ix {
			s += lt.Rows[i].V[e.mi]
		}
		if e.sumAll {
			return s, true
		}
		return s / float64(len(ix)), true
	case "stock", "cum":
		var last int64
		for _, i := range ix {
			if lt.Rows[i].T > last {
				last = lt.Rows[i].T
			}
		}
		s := 0.0
		for _, i := range ix {
			if lt.Rows[i].T == last {
				s += lt.Rows[i].V[e.mi]
			}
		}
		return s, true
	}
	s := 0.0
	for _, i := range ix {
		s += lt.Rows[i].V[e.mi]
	}
	return s, true
}

func (e *eng) isSum() bool {
	m := e.meas()
	return m != nil && m.Agg == "sum"
}

// window 는 계산할 기간을 정한다.
func (e *eng) window(super bool, gdim int) string {
	lt, qt := e.lt, e.qt
	if lt.Gran == 0 || (lt.S.Time == nil && lt.PS > 0) {
		if lt.PS > 0 {
			e.period = fmtT(lt.PS, 3) + "~" + fmtT(lt.PE, 3)
		}
		return ""
	}
	if qt.Kind == "pair" {
		for _, m := range qt.Ms {
			if m.G > lt.Gran {
				return fmt.Sprintf("이 자료는 %s 단위라 그보다 잘게 나눌 수 없어요", granName(lt.Gran))
			}
			if m.E < lt.TMin || m.S > lt.TMax {
				return fmt.Sprintf("「%s」 자료가 없어요(자료 기간: %s~%s)", m.Text, fmtT(lt.TMin, lt.Gran), fmtT(lt.TMax, lt.Gran))
			}
		}
		return ""
	}
	if qt.Kind != "" {
		S, E := qt.S, qt.E
		if qt.Kind == "since" {
			E = lt.TMax
		}
		if qt.G > lt.Gran {
			return fmt.Sprintf("이 자료는 %s 단위라 그보다 잘게 나눈 값은 없어요", granName(lt.Gran))
		}
		if E < lt.TMin || S > lt.TMax {
			if qt.Rel && qt.G == 1 && tYear(qt.S)-tYear(lt.TMax) == 1 {
				y := tYear(lt.TMax)
				S, E = span(y, 0, 0, 0, 1)
				e.notes = append(e.notes, fmt.Sprintf("%d년 자료는 아직 없어 가장 최근인 %d년 값을 알려 드려요", tYear(qt.S), y))
			} else {
				return fmt.Sprintf("그 시점 자료가 없어요(이 데이터는 %s~%s)", fmtT(lt.TMin, lt.Gran), fmtT(lt.TMax, lt.Gran))
			}
		}
		cs, ce := S, E
		if lt.TMin > cs {
			cs = lt.TMin
		}
		if lt.TMax < ce {
			ce = lt.TMax
		}
		cov := days(cs, ce) / days(S, E)
		if cov < 0.97 {
			if e.isSum() && !super && e.tgrp == 0 && e.useCum(S, E) {
				e.S, e.E = S, E
				e.period = periodText(S, E, qt.G, lt.Gran)
				return ""
			}
			if e.isSum() && !super && e.tgrp == 0 {
				return fmt.Sprintf("그 기간 중 %s~%s만 자료가 있어 합계를 낼 수 없어요", fmtT(cs, lt.Gran), fmtT(ce, lt.Gran))
			}
			e.notes = append(e.notes, fmt.Sprintf("자료가 있는 %s~%s만 계산했어요", fmtT(cs, lt.Gran), fmtT(ce, lt.Gran)))
		}
		e.S, e.E = S, E
		e.period = periodText(S, E, qt.G, lt.Gran)
		return ""
	}
	// 시점이 없을 때
	m := e.meas()
	ix := e.rowsFor(0, 0, gdim)
	if len(ix) == 0 {
		return "조건에 맞는 행이 없어요"
	}
	var last int64
	for _, i := range ix {
		if lt.Rows[i].T > last {
			last = lt.Rows[i].T
		}
	}
	switch {
	case reTrendE.MatchString(e.cue) || (reDirAsk.MatchString(e.q) && !super && gdim < 0) || e.tgrp > 0 || (super && m != nil && m.Row) || m == nil:
		e.period = fmtT(lt.TMin, lt.Gran) + "~" + fmtT(lt.TMax, lt.Gran)
		return ""
	case m.Agg == "stock" || m.Agg == "cum" || qt.Now:
		var lr LRow
		for _, i := range ix {
			if lt.Rows[i].T == last {
				lr = lt.Rows[i]
			}
		}
		e.S, e.E = lr.T, lr.E
		e.period = fmtT(lr.T, lr.G)
		if !strings.Contains(e.q, "최근") && !strings.Contains(e.q, "지금") {
			e.notes = append(e.notes, "시점을 적지 않으셔서 가장 최근인 "+e.period+" 값으로 답했어요")
		}
		return ""
	}
	// 「평균이 몇 점」 — 행 값의 평균을 시점 없이 물으면 자료 전체 기간 평균이다
	if m.Agg == "avg" && reMeanW.MatchString(e.q) && e.gdim < 0 {
		e.period = fmtT(lt.TMin, lt.Gran) + "~" + fmtT(lt.TMax, lt.Gran)
		e.notes = append(e.notes, "시점을 적지 않으셔서 자료 전체 기간인 "+e.period+" 평균으로 답했어요")
		return ""
	}
	// 합·평균 — 가장 최근의 온전한 해
	y := e.lastFullYear(ix)
	e.S, e.E = span(y, 0, 0, 0, 1)
	e.period = fmt.Sprintf("%d년", y)
	e.notes = append(e.notes, fmt.Sprintf("시점을 적지 않으셔서 자료가 온전히 있는 가장 최근 해인 %d년 값으로 답했어요", y))
	return ""
}

func (e *eng) lastFullYear(ix []int) int {
	lt := e.lt
	mon := map[int]map[int]bool{}
	maxY := 0
	for _, i := range ix {
		r := lt.Rows[i]
		y := tYear(r.T)
		if mon[y] == nil {
			mon[y] = map[int]bool{}
		}
		for m := tMonth(r.T); m <= tMonth(r.E) && m > 0; m++ {
			mon[y][m] = true
		}
		if y > maxY {
			maxY = y
		}
	}
	for y := maxY; y > maxY-30; y-- {
		if len(mon[y]) == 12 {
			return y
		}
	}
	return maxY
}

func granName(g int) string {
	return map[int]string{1: "연", 2: "월", 3: "일", 4: "시간"}[g]
}

func periodText(S, E int64, qg, dg int) string {
	if tYear(S) == tYear(E) && tMonth(S) == 1 && tMonth(E) == 12 && S%10000 == 100 && E/100%100 == 31 {
		return fmt.Sprintf("%d년", tYear(S))
	}
	g := qg
	if g < 2 && (tMonth(S) != 1 || tMonth(E) != 12) {
		g = 2
	}
	a, b := fmtT(S, g), fmtT(E, g)
	if tYear(S) == tYear(E) && tMonth(S) == 1 && tMonth(E) == 6 && E/100%100 == 30 {
		return fmt.Sprintf("%d년 상반기", tYear(S))
	}
	if tYear(S) == tYear(E) && tMonth(S) == 7 && tMonth(E) == 12 {
		return fmt.Sprintf("%d년 하반기", tYear(S))
	}
	if a == b {
		return a
	}
	return a + "~" + b
}

func (e *eng) filters() []string {
	out := []string{}
	for i, d := range e.lt.Dims {
		if vs := e.sel[i]; len(vs) > 0 && i != e.gdim {
			out = append(out, d.Label+" "+strings.Join(vs, "·"))
		} else if d.Default != "" && i != e.gdim {
			out = append(out, d.Label+" "+d.Default+"(기본값)")
		}
	}
	return append(out, e.filt...)
}

func (e *eng) subj() string {
	parts := []string{}
	for i := range e.lt.Dims {
		if vs := e.sel[i]; len(vs) > 0 && i != e.gdim && e.lt.Dims[i].Flag == "" {
			if len(vs) == 1 {
				parts = append(parts, vs[0])
			}
		}
	}
	for _, f := range e.filt {
		if strings.HasSuffix(f, " 있음") {
			parts = append(parts, strings.TrimSuffix(f, " 있음")+"이 있는")
		}
	}
	return strings.Join(parts, " ")
}

func (e *eng) head() string {
	h := strings.TrimSpace(e.period + " " + e.subj())
	if h != "" {
		h += " "
	}
	return h
}

// ── 연산들 ─────────────────────────────────────────

// opTwoMeas 는 같은 표의 두 지표(출생아 수·사망자 수, 소비자물가·생활물가)를 같은 조건에서 견준다.
func (e *eng) opTwoMeas(res *Result) string {
	ix := e.rowsFor(e.S, e.E, -1)
	keep := e.mi
	var vs [2]float64
	var ls [2]string
	for k, i := range e.two {
		e.mi = i
		v, ok := e.agg(ix)
		if !ok {
			e.mi = keep
			return "두 지표 중 한쪽 자료가 없어요"
		}
		vs[k], ls[k] = v, e.label()
	}
	e.mi = keep
	res.Op, res.OpLabel = "compare", "두 지표 비교"
	res.Series = []Point{{K: ls[0], V: vs[0]}, {K: ls[1], V: vs[1]}}
	res.Facts = append(res.Facts, Fact{ls[0], ls[0], e.fu(vs[0])}, Fact{ls[1], ls[1], e.fu(vs[1])})
	rel := "두 값이 같아요"
	if vs[0] > vs[1] {
		rel = jo(ls[0], "이", "가") + " " + e.fd(vs[0]-vs[1]) + " 더 " + map[bool]string{true: "높아요", false: "많아요"}[e.unit() == "" || e.unit() == "%"]
	} else if vs[0] < vs[1] {
		rel = jo(ls[1], "이", "가") + " " + e.fd(vs[1]-vs[0]) + " 더 " + map[bool]string{true: "높아요", false: "많아요"}[e.unit() == "" || e.unit() == "%"]
	}
	t := fmt.Sprintf("%s%s %s, %s %s입니다. %s.", e.head(), ls[0], e.fu(vs[0]), ls[1], e.fu(vs[1]), rel)
	if reRatioQ.MatchString(e.q) && vs[1] != 0 {
		t += fmt.Sprintf(" %s %s의 %.2f배입니다.", jo(ls[0], "은", "는"), ls[1], vs[0]/vs[1])
	}
	res.Text = t
	res.Plan.Op, res.Plan.OpWhy = "compare", "한 표의 두 지표를 같은 조건에서 견주는 질문"
	return ""
}

func (e *eng) opValue(res *Result) string {
	ix := e.rowsFor(e.S, e.E, -1)
	v, ok := e.agg(ix)
	if !ok {
		return "조건에 맞는 자료가 없어요(" + strings.TrimSpace(e.head()) + ")"
	}
	res.Op, res.OpLabel = "value", "값"
	lab := e.label()
	if e.meas() != nil && e.meas().Agg == "avg" && e.period != "" && strings.Contains(e.period, "~") || (e.meas() != nil && e.meas().Agg == "avg" && reMeanW.MatchString(e.q)) {
		lab = "평균 " + lab
	}
	res.Facts = append(res.Facts, Fact{"value", lab, e.fu(v)})
	t := fmt.Sprintf("%s%s %s입니다.", e.head(), jo(lab, "은", "는"), e.fu(v))
	if reSign.MatchString(e.cue) {
		if v > 0 {
			t += " 플러스예요(늘어난 쪽)."
		} else if v < 0 {
			t += " 마이너스예요(줄어든 쪽)."
		} else {
			t += " 0이에요."
		}
	}
	res.Plan.Op, res.Plan.OpWhy = "value", "값 하나를 묻는 질문"
	res.Text = t
	return ""
}

func (e *eng) opChange(res *Result) string {
	ms := e.qt.Ms
	a, z := ms[0], ms[len(ms)-1]
	if a.S > z.S { // 「2025년은 2023년의 몇 배」 — 늘고 줄음은 언제나 옛 시점 → 새 시점
		a, z = z, a
	}
	va, ok1 := e.agg(e.rowsFor(a.S, a.E, -1))
	vz, ok2 := e.agg(e.rowsFor(z.S, z.E, -1))
	if !ok1 || !ok2 {
		return "두 시점 중 한쪽 자료가 없어요"
	}
	pa, pz := periodText(a.S, a.E, a.G, e.lt.Gran), periodText(z.S, z.E, z.G, e.lt.Gran)
	if a.G == 2 && a.Text != "" && strings.Contains(a.Text, "말") {
		pa = fmt.Sprintf("%d년 말", tYear(a.S))
	}
	if z.G == 2 && strings.Contains(z.Text, "말") {
		pz = fmt.Sprintf("%d년 말", tYear(z.S))
	}
	d := vz - va
	e.period = pa + " → " + pz
	res.Op, res.OpLabel = "change", "변화"
	res.Series = []Point{{K: pa, V: va}, {K: pz, V: vz}}
	res.Facts = append(res.Facts, Fact{"from", pa, e.fu(va)}, Fact{"to", pz, e.fu(vz)}, Fact{"diff", "변화량", e.fv(d) + unitSp(e.unit())})
	dir := "늘었습니다"
	if d < 0 {
		dir = "줄었습니다"
	} else if d == 0 {
		dir = "그대로입니다"
	}
	pc := ""
	if va != 0 {
		pc = fmt.Sprintf("(%+.1f%%)", (vz-va)/math.Abs(va)*100)
	}
	sj := e.subj()
	if sj != "" {
		sj += " "
	}
	res.Text = fmt.Sprintf("%s%s %s %s, %s %s %s %s%s.", sj, jo(e.label(), "은", "는"), pa, e.fu(va), pz, ro(e.fu(vz)), e.fd(math.Abs(d)), dir, pc)
	if reRatioQ.MatchString(e.q) && va > 0 {
		res.Text += fmt.Sprintf(" %s 값은 %s의 %.2f배입니다.", pz, pa, vz/va)
	}
	res.Plan.Op, res.Plan.OpWhy = "change", "두 시점을 견주는 질문"
	return ""
}

func (e *eng) opCompare(res *Result, d int) string {
	vs := e.sel[d]
	pts := []Point{}
	for _, v := range vs {
		save := e.sel[d]
		e.sel[d] = []string{v}
		x, ok := e.agg(e.rowsFor(e.S, e.E, -1))
		e.sel[d] = save
		if ok {
			pts = append(pts, Point{K: v, V: x})
		}
	}
	if len(pts) < 2 {
		return "견줄 값 중 일부가 자료에 없어요"
	}
	sort.SliceStable(pts, func(a, b int) bool { return pts[a].V > pts[b].V })
	pts[0].Hot = true
	sv := e.sel[d]
	delete(e.sel, d)
	h := e.head()
	e.sel[d] = sv
	parts := []string{}
	for _, p := range pts {
		parts = append(parts, p.K+" "+e.fu(p.V))
		res.Facts = append(res.Facts, Fact{p.K, p.K, e.fu(p.V)})
	}
	word := "많아요"
	if u := e.unit(); u == "%" || u == "℃" || u == "dB" || u == "" || strings.Contains(u, "/") || u == "m" || u == "점" {
		word = "높아요"
	}
	res.Op, res.OpLabel = "compare", "비교"
	res.Series = pts
	res.BreakBy = e.lt.Dims[d].Label
	// 질문에 먼저 나온 값(대개 제주)을 앞에
	first, fp := vs[0], -1
	for _, v := range vs {
		if i := strings.Index(e.q, v); i >= 0 && (fp < 0 || i < fp) {
			first, fp = v, i
		}
	}
	for _, v := range vs {
		if canonRegion(v) == "제주" {
			first = v
		}
	}
	fv, other := 0.0, Point{}
	for _, p := range pts {
		if p.K == first {
			fv = p.V
		} else if other.K == "" {
			other = p
		}
	}
	rel := ""
	if fv > other.V {
		rel = fmt.Sprintf(" %s %s보다 더 %s.", jo(first, "이", "가"), other.K, word)
	} else if fv < other.V {
		low := "적어요"
		if word == "높아요" {
			low = "낮아요"
		}
		rel = fmt.Sprintf(" %s %s보다 %s.", jo(first, "이", "가"), other.K, low)
	} else {
		rel = " 두 값이 같아요."
	}
	if len(pts) == 2 && pts[1].V > 0 && reRatioQ.MatchString(e.q) {
		rel += fmt.Sprintf(" %s %s의 %.1f배입니다.", jo(pts[0].K, "은", "는"), pts[1].K, pts[0].V/pts[1].V)
	} else if len(pts) == 2 && reDiffQ.MatchString(e.q) {
		rel += fmt.Sprintf(" 차이는 %s입니다.", e.fu(pts[0].V-pts[1].V))
	}
	res.Text = fmt.Sprintf("%s%s: %s.%s 가장 큰 쪽은 %s(%s)입니다.", h, e.label(), strings.Join(parts, ", "), rel, pts[0].K, e.fu(pts[0].V))
	res.Plan.Op, res.Plan.OpWhy = "compare", "두 값 이상을 견주는 질문"
	return ""
}

func (e *eng) wantMin() bool {
	mn, mx := reMinW.MatchString(e.q), reMaxW.MatchString(e.q)
	if mn && !mx {
		return true
	}
	if mx && !mn {
		return false
	}
	if m := e.meas(); m != nil && m.Peak == "min" {
		return true
	}
	return mn && mx && reMinW.FindStringIndex(e.cue) != nil && reMaxW.FindStringIndex(e.cue) == nil
}

func (e *eng) dispVal(d int, v string) string {
	dm := e.lt.Dims[d]
	if syn := dm.Syn[v]; len(syn) > 0 && !hasHangul(v) {
		return v + "(" + syn[0] + ")"
	}
	return v
}

func (e *eng) opArg(res *Result, d int, also []string) string {
	lt := e.lt
	dm := lt.Dims[d]
	if m := e.meas(); m != nil && m.Row && (dm.Entity || dm.Attr) {
		return e.opRowPeak(res)
	}
	ix := e.rowsFor(e.S, e.E, d)
	by := map[string][]int{}
	order := []string{}
	for _, i := range ix {
		v := lt.Rows[i].D[d]
		if v == "" {
			continue
		}
		if by[v] == nil {
			order = append(order, v)
		}
		by[v] = append(by[v], i)
	}
	pts := []Point{}
	for _, v := range order {
		x, _ := e.agg(by[v])
		pts = append(pts, Point{K: v, V: x, N: len(by[v])})
	}
	if len(pts) == 0 {
		return "조건에 맞는 자료가 없어요"
	}
	mn := e.wantMin()
	sort.SliceStable(pts, func(a, b int) bool {
		if mn {
			return pts[a].V < pts[b].V
		}
		return pts[a].V > pts[b].V
	})
	pts[0].Hot = true
	res.Op, res.OpLabel = "rank", "순위"
	res.Breakdown, res.BreakBy = pts, dm.Label
	if len(pts) > 12 {
		res.Breakdown = pts[:12]
	}
	ties := []string{}
	for _, p := range pts {
		if p.V == pts[0].V {
			ties = append(ties, e.dispVal(d, p.K))
		}
	}
	word := "가장 많은"
	if mn {
		word = "가장 적은"
	}
	if u := e.unit(); u == "%" || u == "℃" || u == "dB" || u == "" || u == "점" || u == "m" {
		word = map[bool]string{true: "가장 낮은", false: "가장 높은"}[mn]
	}
	if e.meas() != nil && e.meas().Agg == "avg" && reMeanW.MatchString(e.q) == false && e.meas().Unit == "점" {
		word += "(기간 평균)"
	}
	head := e.head()
	top := strings.Join(ties, "·")
	res.Facts = append(res.Facts, Fact{"top", dm.Label, top}, Fact{"topValue", e.label(), e.fu(pts[0].V)})
	t := fmt.Sprintf("%s%s %s %s %s(%s)입니다.", head, jo(e.label(), "이", "가"), word, jo(dm.Label, "은", "는"), top, e.fu(pts[0].V))
	if n := topN(e.q); n >= 3 && len(ties) == 1 && len(pts) > 1 {
		var xs []string
		for i := 1; i < n && i < len(pts); i++ {
			xs = append(xs, fmt.Sprintf("%d위 %s(%s)", i+1, e.dispVal(d, pts[i].K), e.fu(pts[i].V)))
			res.Facts = append(res.Facts, Fact{fmt.Sprintf("r%d", i+1), fmt.Sprintf("%d위 %s", i+1, e.dispVal(d, pts[i].K)), e.fu(pts[i].V)})
		}
		t += " 다음은 " + strings.Join(xs, ", ") + "입니다."
	} else if len(pts) > 1 && len(ties) == 1 {
		t += fmt.Sprintf(" 다음은 %s(%s)입니다.", e.dispVal(d, pts[1].K), e.fu(pts[1].V))
		res.Facts = append(res.Facts, Fact{"r2", "2위 " + e.dispVal(d, pts[1].K), e.fu(pts[1].V)})
	}
	for _, a := range also {
		for _, p := range pts {
			if p.K == a {
				rk := 0
				for i, q := range pts {
					if q.K == a {
						rk = i + 1
					}
				}
				t += fmt.Sprintf(" %s %s(%d위)입니다.", jo(a, "은", "는"), e.fu(p.V), rk)
				res.Facts = append(res.Facts, Fact{a, a, e.fu(p.V)})
			}
		}
	}
	res.Text = t
	res.Plan.Op, res.Plan.OpWhy = "rank", dm.Label+"별로 묶어 "+word+" 곳을 찾는 질문"
	return ""
}

func (e *eng) opArgTime(res *Result) string {
	lt := e.lt
	ix := e.rowsFor(e.S, e.E, -1)
	if e.meas() != nil && e.meas().Row && e.tgrp >= lt.Gran {
		return e.opRowPeak(res)
	}
	by := map[int64][]int{}
	keys := []int64{}
	for _, i := range ix {
		r := lt.Rows[i]
		k := tkey(r.T, e.tgrp)
		if by[k] == nil {
			keys = append(keys, k)
		}
		by[k] = append(by[k], i)
	}
	// 합계 지표는 온전하지 않은 해를 뺀다
	pts := []Point{}
	full := map[int]bool{}
	if e.tgrp == 1 && e.isSum() && lt.Gran >= 2 {
		mon := map[int]map[int]bool{}
		for _, i := range ix {
			y := tYear(lt.Rows[i].T)
			if mon[y] == nil {
				mon[y] = map[int]bool{}
			}
			mon[y][tMonth(lt.Rows[i].T)] = true
		}
		for y, ms := range mon {
			full[y] = len(ms) == 12
		}
	}
	for _, k := range keys {
		if len(full) > 0 && !full[tYear(k)] {
			continue
		}
		x, _ := e.agg(by[k])
		pts = append(pts, Point{K: fmtT(k, e.tgrp), V: x, N: len(by[k])})
	}
	if len(pts) == 0 {
		return "조건에 맞는 자료가 없어요"
	}
	mn := e.wantMin()
	srt := append([]Point{}, pts...)
	sort.SliceStable(srt, func(a, b int) bool {
		if mn {
			return srt[a].V < srt[b].V
		}
		return srt[a].V > srt[b].V
	})
	ties := []string{}
	for _, p := range srt {
		if p.V == srt[0].V {
			ties = append(ties, p.K)
		}
	}
	for i := range pts {
		pts[i].Hot = pts[i].V == srt[0].V
	}
	word := map[bool]string{true: "가장 낮았던", false: "가장 높았던"}[mn]
	if u := e.unit(); u == "명" || u == "대" || u == "건" || u == "톤" || u == "호" || u == "곳" || u == "회" || u == "원" || u == "달러" || u == "가구" || u == "세대" || u == "개" || strings.HasSuffix(u, "원") {
		word = map[bool]string{true: "가장 적었던", false: "가장 많았던"}[mn]
	}
	res.Op, res.OpLabel = "rank", "때 찾기"
	res.Series = pts
	gn := map[int]string{1: "해", 2: "달", 3: "날"}[e.tgrp]
	res.Facts = append(res.Facts, Fact{"top", gn, strings.Join(ties, "·")}, Fact{"topValue", e.label(), e.fu(srt[0].V)})
	sj := e.subj()
	if sj != "" && strings.Contains(e.label(), sj) {
		sj = ""
	}
	if sj != "" {
		sj += " "
	}
	rng := ""
	if e.period != "" {
		rng = e.period + " 중 "
	}
	res.Text = fmt.Sprintf("%s%s%s %s %s %s(%s)입니다.", rng, sj, jo(e.label(), "이", "가"), word, jo(gn, "은", "는"), strings.Join(ties, "·"), e.fu(srt[0].V))
	res.Plan.Op, res.Plan.OpWhy = "rank", gn+" 단위로 묶어 "+word+" 때를 찾는 질문"
	return ""
}

func tkey(t int64, g int) int64 {
	switch g {
	case 1:
		return t / 1000000 * 1000000
	case 2:
		return t / 10000 * 10000
	case 3:
		return t / 100 * 100
	}
	return t
}

func (e *eng) opRowPeak(res *Result) string {
	lt := e.lt
	m := e.meas()
	if m == nil {
		return "무엇을 견줄지 정할 수 없어요"
	}
	ix := e.rowsFor(e.S, e.E, -1)
	if len(ix) == 0 {
		return "조건에 맞는 자료가 없어요"
	}
	mn := e.wantMin()
	best := lt.Rows[ix[0]].V[e.mi]
	for _, i := range ix {
		v := lt.Rows[i].V[e.mi]
		if (mn && v < best) || (!mn && v > best) {
			best = v
		}
	}
	ties := []int{}
	for _, i := range ix {
		if lt.Rows[i].V[e.mi] == best {
			ties = append(ties, i)
			e.used[lt.Rows[i].R] = true
		}
	}
	names := []string{}
	for _, i := range ties {
		r := lt.Rows[i]
		parts := []string{}
		for di, d := range lt.Dims {
			if (d.Entity || d.Attr || (len(lt.Vals[di]) > 1 && len(e.sel[di]) == 0 && d.Default == "")) && r.D[di] != "" {
				parts = append(parts, r.D[di])
			}
		}
		if r.G > 0 && lt.S.Time != nil {
			parts = append(parts, fmtT(r.T, r.G))
		}
		n := strings.Join(parts, " · ")
		if !contains(names, n) {
			names = append(names, n)
		}
	}
	if len(names) > 5 {
		names = append(names[:5], fmt.Sprintf("외 %d건", len(names)-5))
	}
	word := map[bool]string{true: "가장 낮은", false: "가장 높은"}[mn]
	if u := m.Unit; u == "원" || u == "㎡" || u == "명" {
		word = map[bool]string{true: "가장 적은", false: "가장 큰"}[mn]
	}
	if m.Unit == "원" && strings.Contains(m.Label, "가격") {
		word = map[bool]string{true: "가장 싼", false: "가장 비싼"}[mn]
	}
	if m.Unit == "km" {
		word = map[bool]string{true: "가장 가까운", false: "가장 먼"}[mn]
	}
	res.Op, res.OpLabel = "peak", "최댓값 찾기"
	if mn {
		res.OpLabel = "최솟값 찾기"
	}
	res.Facts = append(res.Facts, Fact{"top", "해당 행", strings.Join(names, " / ")}, Fact{"topValue", m.Label, e.fu(best)})
	h := e.head()
	res.Text = fmt.Sprintf("%s%s %s 값은 %s이며, %s의 기록입니다.", h, word, m.Label, e.fu(best), strings.Join(names, " / "))
	res.Plan.Op, res.Plan.OpWhy = "peak", "행 하나하나의 값 중 "+word+" 것을 찾는 질문"
	return ""
}

func isoT(t int64, g int) string {
	y, m, d, h := int(t/1000000), int(t/10000%100), int(t/100%100), int(t%100)
	switch g {
	case 1:
		return fmt.Sprintf("%d", y)
	case 2:
		return fmt.Sprintf("%d-%02d", y, m)
	case 3:
		return fmt.Sprintf("%d-%02d-%02d", y, m, d)
	}
	return fmt.Sprintf("%d-%02d-%02d %02d시", y, m, d, h)
}

func (e *eng) opRank(res *Result, d int) string {
	mine := e.sel[d][0]
	save := e.sel[d]
	delete(e.sel, d)
	ix := e.rowsFor(e.S, e.E, d)
	e.sel[d] = save
	lt := e.lt
	by := map[string][]int{}
	order := []string{}
	for _, i := range ix {
		v := lt.Rows[i].D[d]
		if by[v] == nil {
			order = append(order, v)
		}
		by[v] = append(by[v], i)
	}
	pts := []Point{}
	for _, v := range order {
		x, _ := e.agg(by[v])
		pts = append(pts, Point{K: v, V: x, Hot: v == mine})
	}
	sort.SliceStable(pts, func(a, b int) bool { return pts[a].V > pts[b].V })
	rk, mv := 0, 0.0
	sum := 0.0
	for i, p := range pts {
		sum += p.V
		if p.K == mine {
			rk, mv = i+1, p.V
		}
	}
	if rk == 0 {
		return "견줄 값이 없어요"
	}
	res.Op, res.OpLabel = "rank", "순위"
	res.Breakdown, res.BreakBy = pts, lt.Dims[d].Label
	res.Facts = append(res.Facts, Fact{"value", mine, e.fu(mv)}, Fact{"rank", "순위", fmt.Sprintf("%d개 중 %d위", len(pts), rk)}, Fact{"top", "1위", pts[0].K + " " + e.fu(pts[0].V)})
	avg := sum / float64(len(pts))
	cmp := "평균보다 많아요"
	if mv < avg {
		cmp = "평균보다 적어요"
	}
	res.Text = fmt.Sprintf("%s%s %s, %d개 %s 중 %d위입니다(1위 %s %s, %s 평균 %s, %s).", strings.TrimSpace(e.period+" ")+" ", jo(mine+" "+e.label(), "은", "는"), ro(e.fu(mv)), len(pts), lt.Dims[d].Label, rk, pts[0].K, e.fu(pts[0].V), lt.Dims[d].Label, e.fu(avg), cmp)
	res.Text = strings.TrimSpace(res.Text)
	res.Plan.Op, res.Plan.OpWhy = "rank", "다른 "+lt.Dims[d].Label+"와 견주는 질문"
	return ""
}

func (e *eng) opTrend(res *Result) string {
	lt := e.lt
	ix := e.rowsFor(e.S, e.E, -1)
	g := 1
	if e.S > 0 && tYear(e.S) == tYear(e.E) && lt.Gran >= 2 {
		g = 2
	}
	by := map[int64][]int{}
	keys := []int64{}
	for _, i := range ix {
		k := tkey(lt.Rows[i].T, g)
		if by[k] == nil {
			keys = append(keys, k)
		}
		by[k] = append(by[k], i)
	}
	sort.Slice(keys, func(a, b int) bool { return keys[a] < keys[b] })
	pts := []Point{}
	for _, k := range keys {
		if g == 1 && e.isSum() && lt.Gran >= 2 {
			ms := map[int]bool{}
			for _, i := range by[k] {
				ms[tMonth(lt.Rows[i].T)] = true
			}
			if len(ms) < 12 {
				e.notes = appendU(e.notes, "자료가 1년치가 안 되는 해는 추이에서 뺐어요")
				continue
			}
		}
		x, _ := e.agg(by[k])
		pts = append(pts, Point{K: fmtT(k, g), V: x, N: len(by[k])})
	}
	if len(pts) < 2 {
		return "추이를 볼 만큼 시점이 많지 않아요"
	}
	a, z := pts[0], pts[len(pts)-1]
	mx, mn := pts[0], pts[0]
	for _, p := range pts {
		if p.V > mx.V {
			mx = p
		}
		if p.V < mn.V {
			mn = p
		}
	}
	d := z.V - a.V
	dir := "늘었습니다"
	if d < 0 {
		dir = "줄었습니다"
	}
	res.Op, res.OpLabel = "trend", "추이"
	res.Series = pts
	res.Facts = append(res.Facts, Fact{"from", a.K, e.fu(a.V)}, Fact{"to", z.K, e.fu(z.V)}, Fact{"diff", "변화량", e.fv(d) + unitSp(e.unit())}, Fact{"max", "가장 높았던 때 " + mx.K, e.fu(mx.V)}, Fact{"min", "가장 낮았던 때 " + mn.K, e.fu(mn.V)})
	sj := e.subj()
	if sj != "" {
		sj += " "
	}
	res.Text = fmt.Sprintf("%s%s %s %s에서 %s %s %s %s. 가장 높았던 때는 %s(%s), 가장 낮았던 때는 %s(%s)입니다.", sj, jo(e.label(), "은", "는"), a.K, e.fu(a.V), z.K, ro(e.fu(z.V)), e.fd(math.Abs(d)), dir, mx.K, e.fu(mx.V), mn.K, e.fu(mn.V))
	e.period = a.K + "~" + z.K
	res.Plan.Op, res.Plan.OpWhy = "trend", "시간에 따른 변화를 묻는 질문"
	return ""
}

func appendU(xs []string, x string) []string {
	if contains(xs, x) {
		return xs
	}
	return append(xs, x)
}

func (e *eng) evidence(res *Result) {
	lt := e.lt
	res.EvHeader = lt.Header
	rs := []int{}
	for r := range e.used {
		rs = append(rs, r)
	}
	sort.Ints(rs)
	for _, r := range rs {
		if len(res.Evidence) >= 15 {
			break
		}
		if r < len(lt.Raw) {
			res.Evidence = append(res.Evidence, lt.Raw[r])
		}
	}
}

func (e *eng) code(res *Result) string {
	lt := e.lt
	b := &strings.Builder{}
	name := "data.csv"
	if lt.D.File != nil {
		name = lt.D.File.FileName
	}
	if strings.HasSuffix(strings.ToLower(name), ".xlsx") || strings.HasSuffix(strings.ToLower(name), ".xls") {
		fmt.Fprintf(b, "import pandas as pd\ndf = pd.read_excel(%q)  # %s\n", name, lt.D.Title)
	} else {
		fmt.Fprintf(b, "import pandas as pd\ndf = pd.read_csv(%q)  # %s\n", name, lt.D.Title)
	}
	if lt.S.Melt != nil {
		cols := []string{}
		for _, c := range lt.S.Melt.Cols {
			cols = append(cols, fmt.Sprintf("%q", c.Col))
		}
		fmt.Fprintf(b, "df = df.melt(id_vars=[c for c in df.columns if c not in [%s]], var_name='열', value_name='값')  # 넓은 표 → 긴 표\n", strings.Join(cols, ", "))
	}
	for _, f := range res.Filters {
		fmt.Fprintf(b, "# 조건: %s\n", f)
	}
	if e.period != "" {
		fmt.Fprintf(b, "# 기간: %s\n", e.period)
	}
	col := "행 수"
	if m := e.meas(); m != nil {
		col = m.Col
		if col == "" {
			col = "값"
		}
		fn := map[string]string{"sum": "sum", "avg": "mean", "stock": "마지막 시점 값을 더함", "cum": "마지막 시점 값을 더함"}[m.Agg]
		fmt.Fprintf(b, "# %s: 「%s」 열 → %s\n", res.OpLabel, col, fn)
	} else {
		c := lt.S.Counts[e.ci]
		if c.Distinct != "" {
			fmt.Fprintf(b, "# 개수: df[%q].nunique()\n", c.Distinct)
		} else {
			b.WriteString("# 개수: len(df)\n")
		}
	}
	if e.gdim >= 0 {
		fmt.Fprintf(b, "# 묶음: df.groupby(%q)\n", lt.Dims[e.gdim].Col)
	}
	return b.String()
}

func engineMeaning(e *Result, lt *LT, g *Ground) Meaning {
	its := []CheckItem{
		{"주제", true, "질문 낱말 " + quoteJo(g.Anchor, "이", "가") + " 이 데이터 열 사전에 있음"},
		{"지표", true, "「" + e.Measure + "」로 계산"},
		{"연산", true, e.OpLabel},
		{"대상", true, strings.Join(e.Filters, ", ")},
		{"시점", true, e.Period},
	}
	if its[3].Detail == "" {
		its[3].Detail = "전체"
	}
	if its[4].Detail == "" {
		its[4].Detail = "시점 없는 자료"
	}
	return Meaning{Pass: true, Items: its}
}

// askEngine 은 질문을 열 사전 엔진으로만 푼다(옛 분석기로 되돌아가지 않는다).
func askEngine(a *Answer, q string, extra []string, T time.Time) *Answer {
	return askEngineD(a, q, extra, T, 0)
}

// askEngineD 의 depth 가 1이면 AI 낱말 맞추기로 고친 질문이다(다시 고치지 않는다).
func askEngineD(a *Answer, q string, extra []string, T time.Time, depth int) *Answer {
	t0 := time.Now()
	qt := ParseQTime(q)
	hits, _ := Search(q, 8, extra)
	a.Hits = hits
	if qt.Refuse != "" {
		a.refuse(qt.Refuse, []Cand{})
		a.Steps = append(a.Steps, Step{"refuse", "시점 확인", qt.Refuse, since(t0), "warn"})
		a.TotalMs = since(T)
		return a
	}
	p, why, cands, ties := chooseT(qt)
	// 「부산 인구」 — 제주가 아닌 시도만 물었는데 그 시도 값이 든 자료(시도별·전국)가 아니면 제주 자료로 답하지 않는다
	if other := otherSido(a.Intent.Regions); len(other) > 0 && (p == nil || (p.lt.S.Scope != "시도" && p.lt.S.Scope != "전국")) {
		msg := "물으멍은 제주 공공데이터로만 답해요. 「" + strings.Join(other, "」「") + "」 값이 든 자료는 데이터허브에 없어요"
		a.Steps = append(a.Steps, Step{"refuse", "답할 수 있는 질문인지", msg, since(t0), "warn"})
		a.refuse(msg, nil)
		a.TotalMs = since(T)
		return a
	}
	var agreeNote string
	if p == nil && len(ties) > 1 {
		if tp, note := agreeTies(ties, qt); tp != nil {
			p, agreeNote = tp, note
		}
	}
	if p == nil && why == "" && depth == 0 && len(cands) > 0 {
		if w := tryRewrite(q, qt, extra, T); w != nil {
			w.Q, w.Terms, w.Extra, w.Hits = a.Q, a.Terms, a.Extra, a.Hits
			w.Steps = append(append([]Step{}, a.Steps...), w.Steps...)
			w.TotalMs = since(T)
			*a = *w
			return a
		}
	}
	if p == nil {
		if why == "" {
			why = "질문의 낱말을 모두 담은 데이터를 찾지 못했어요"
			if len(cands) > 0 {
				why += ". 가까운 데이터는 있지만 " + strings.TrimPrefix(cands[0].Why, "질문의 ")
			}
		}
		a.Steps = append(a.Steps, Step{"search_catalog", "데이터 찾기", why, since(t0), "warn"})
		a.refuse(why, cands)
		a.TotalMs = since(T)
		return a
	}
	lt := p.lt
	d := lt.D
	h := hitOf(d)
	h.Matched = p.g.Anchor
	a.Steps = append(a.Steps, Step{"search_catalog", "데이터 찾기", fmt.Sprintf("열 사전에 질문 낱말을 대어 봄%.0s → 「%s」(닿은 말: %s)", "", d.Title, strings.Join(p.g.Anchor, ", ")), since(t0), "ok"})
	t0 = time.Now()
	res, rwhy := Run(p, qt)
	if res == nil {
		a.Steps = append(a.Steps, Step{"compute", "코드로 계산", rwhy, since(t0), "warn"})
		a.refuse(rwhy, []Cand{{Hit: h, Why: rwhy}})
		a.TotalMs = since(T)
		return a
	}
	a.Chosen = &h
	a.Source, a.Format, a.Rows = lt.Src, lt.T.Format, len(lt.Raw)
	a.Steps = append(a.Steps, Step{"fetch_file", "원본 불러오기", fmt.Sprintf("%s · %s · %s행 → 긴 표 %s행", lt.Src, lt.T.Format, Fmt(float64(len(lt.Raw))), Fmt(float64(len(lt.Rows)))), 0, "ok"})
	a.Result = res
	a.Steps = append(a.Steps, Step{"compute", "코드로 계산", fmt.Sprintf("%s · 지표 「%s」 · 원본 %s행 사용", res.OpLabel, res.Measure, Fmt(float64(res.RowsUsed))), since(t0), "ok"})
	m := engineMeaning(res, lt, p.g)
	a.Meaning = &m
	a.Steps = append(a.Steps, Step{"check_meaning", "의미 검산", "주제·지표·연산·대상·시점이 모두 열 사전에서 정해짐", 0, "ok"})
	text := res.Text
	if agreeNote != "" {
		res.Notes = append(res.Notes, agreeNote)
	}
	if len(res.Notes) > 0 {
		text += " (" + strings.Join(res.Notes, " · ") + ")"
	}
	a.Text, a.Writer, a.Check, a.NumOK = text, "규칙 문장", "정해진 틀에 계산 결과 숫자만 끼워 넣었습니다", true
	tw := time.Now()
	st := "ok"
	if len(res.Facts) > 0 && res.Text != "" {
		if out, chk := llmPhrase(q, d.Title, res, res.Text, answerBudget-time.Since(T)); out != "" {
			a.Text, a.Writer, a.Check = out, WriterLLM, chk
			if len(res.Notes) > 0 {
				a.Text += " (" + strings.Join(res.Notes, " · ") + ")"
			}
		} else {
			a.Check, st = chk, "warn"
		}
	}
	a.Steps = append(a.Steps, Step{"write_answer", "답 쓰기·숫자 검산", a.Check, since(tw), st})
	a.TotalMs = since(T)
	return a
}

// useCum 은 한 해 합계를 물었는데 달마다 값이 다 없을 때, 같은 해 12월의 「당해 누계」 열로 답한다.
func (e *eng) useCum(S, E int64) bool {
	m := e.meas()
	if m == nil || m.Cum == "" || tMonth(S) != 1 || tMonth(E) != 12 || tYear(S) != tYear(E) {
		return false
	}
	ci := -1
	for i, x := range e.lt.Meas {
		if x.Col == m.Cum {
			ci = i
		}
	}
	if ci < 0 {
		return false
	}
	dec := false
	for _, r := range e.lt.Rows {
		if tYear(r.T) == tYear(S) && tMonth(r.T) == 12 {
			dec = true
			break
		}
	}
	if !dec {
		return false
	}
	e.mi = ci
	e.notes = append(e.notes, fmt.Sprintf("이 자료는 %d년 일부 달만 있어 12월의 「당해 누계」 열로 한 해 합계를 읽었어요", tYear(S)))
	return true
}

// ltCovers 는 데이터가 질문 시점을 담고 있는가.
func ltCovers(lt *LT, qt QTime) bool {
	if lt.Gran == 0 {
		if lt.S.Year > 0 {
			y := tTime(qt.S).Year()
			return y == lt.S.Year
		}
		return lt.PS == 0 || (qt.S <= lt.PE && qt.E >= lt.PS)
	}
	return qt.S <= lt.TMax && qt.E >= lt.TMin
}

// agreeTies 는 동점 후보를 모두 계산해 첫 숫자·첫 항목이 같으면 첫 후보를 돌려준다.
func agreeTies(ties tieSet, qt QTime) (*pick, string) {
	var first *Result
	names := []string{}
	for i := range ties {
		r, _ := Run(&ties[i], qt)
		if r == nil || len(r.Facts) == 0 {
			return nil, ""
		}
		if first == nil {
			first = r
		} else if r.Facts[0].Value != first.Facts[0].Value || r.Unit != first.Unit {
			return nil, ""
		}
		names = append(names, ties[i].lt.S.Subject)
	}
	return &ties[0], "「" + strings.Join(names, "」「") + "」 자료 모두 같은 값이에요"
}

// otherSido 는 질문 지역 중 제주가 없을 때 제주 밖 시도 이름을 돌려준다.
func otherSido(rs []string) []string {
	out := []string{}
	for _, r := range rs {
		switch r {
		case "제주", "제주시", "서귀포", "서귀포시":
			return nil
		}
		for _, x := range sido {
			if r == x {
				out = append(out, r)
			}
		}
	}
	return out
}

var reHowMuchChg = regexp.MustCompile(`얼마나 ?(?:올랐|늘었|줄었|떨어졌|내렸|증가|감소)|(?:비교|대비|전년)\S*\s.*(?:늘었|줄었|올랐|떨어졌|내렸)`)
var reRankOnly = regexp.MustCompile(`몇 ?위|순위|등수`)

// rankDim 은 값 하나만 고른 차원(값이 셋 이상) — 「성산일출봉 입장객 순위 몇위야」
func rankDim(e *eng, lt *LT) int {
	for d, vs := range e.sel {
		if len(vs) == 1 && len(lt.Vals[d]) >= 3 {
			return d
		}
	}
	return -1
}

// dedupWords 「입장객 입장객 수」처럼 같은 낱말이 잇달아 나오면 하나로 줄인다
func dedupWords(t string) string {
	w := strings.Split(t, " ")
	out := w[:0]
	for i, x := range w {
		if i > 0 && x == w[i-1] && x != "" {
			continue
		}
		out = append(out, x)
	}
	return strings.Join(out, " ")
}

func strongHit(p pick) bool {
	for _, a := range p.g.Anchor {
		for _, t := range p.lt.S.Strong {
			if a == t && len([]rune(t)) >= 3 {
				return true
			}
		}
	}
	return false
}

// headOfFilter 는 「숙박 상품」처럼 차원 말이 고른 값 바로 뒤에서 그 값을 받는 이름말로만 쓰였는지 본다.
// 그런 차원은 값을 거르는 데 쓰였으니 묶음(어느 것이 가장 많은지)으로 쓰지 않는다.
func (e *eng) headOfFilter(d int) bool {
	vs := e.sel[d]
	if len(vs) != 1 {
		return false
	}
	dm := e.lt.Dims[d]
	forms := append([]string{vs[0]}, dm.Syn[vs[0]]...)
	terms := append([]string{dm.Label}, dm.Terms...)
	for _, f := range forms {
		for _, t := range terms {
			if f == "" || len([]rune(t)) < 2 {
				continue
			}
			if strings.Contains(e.q, f+" "+t) || strings.Contains(e.q, f+t) {
				return true
			}
		}
	}
	return false
}
