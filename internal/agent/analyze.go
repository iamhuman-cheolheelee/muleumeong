package agent

// 계산 — 숫자는 전부 여기서 나온다. 모델은 이 결과를 문장으로 옮길 뿐이다.
//   ① 계획: 질문 낱말과 열 이름을 맞춰 「무엇을(측정 열)·언제(시간 열)·어디(필터)·어떻게(추이/순위/비교)」를 정한다
//   ② 집계: 합계 행(계·전국)을 빼고, 비율 열은 평균·건수 열은 합계로 묶는다
//   ③ 사실표: 화면·문장·재현 코드가 모두 같은 사실표(Facts)를 쓴다

import (
	"fmt"
	"math"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

type Plan struct {
	Op       string            `json:"op"` // trend | rank | compare | value | count
	Role     string            `json:"role"` // 측정 열 역할
	Stock    bool              `json:"stock"`
	MeasureScore float64       `json:"measureScore"`
	OpWhy    string            `json:"opWhy"`
	Measure  []int             `json:"measure"`
	Wide     bool              `json:"wide"` // 열 하나하나가 비교 대상(예: 관광지별 열)
	TimeCol  int               `json:"timeCol"`
	MonthCol int               `json:"monthCol"`
	Group    int               `json:"group"`
	Target   string            `json:"target"`
	Filters  map[int]string    `json:"-"`
	Agg      string            `json:"agg"` // sum | mean
	Monthly  bool              `json:"monthly"`
	Why      []string          `json:"why"`
	Score    map[string]float64 `json:"-"`
}

type Point struct {
	K   string  `json:"k"`
	V   float64 `json:"v"`
	Hot bool    `json:"hot,omitempty"`
	N   int     `json:"n"` // 묶인 원본 행 수
}

type Fact struct {
	Key   string `json:"key"`
	Label string `json:"label"`
	Value string `json:"value"`
}

type Result struct {
	Op        string     `json:"op"`
	OpLabel   string     `json:"opLabel"`
	Measure   string     `json:"measure"`
	Unit      string     `json:"unit"`
	Agg       string     `json:"agg"`
	Series    []Point    `json:"series"`
	Breakdown []Point    `json:"breakdown"`
	BreakBy   string     `json:"breakBy"`
	Facts     []Fact     `json:"facts"`
	Filters   []string   `json:"filters"`
	Period    string     `json:"period"`
	RowsUsed  int        `json:"rowsUsed"`
	RowsTotal int        `json:"rowsTotal"`
	Dropped   int        `json:"dropped"` // 합계 행으로 빼낸 수
	Notes     []string   `json:"notes"`
	Evidence  [][]string `json:"evidence"`
	EvHeader  []string   `json:"evHeader"`
	Plan      Plan       `json:"plan"`
	Cols      []Col      `json:"cols"`
	Code      string     `json:"code"` // 같은 계산을 pandas 로 다시 하는 코드
	YearNote  string     `json:"yearNote,omitempty"`
	YearMiss  string     `json:"yearMiss,omitempty"`
	Text      string     `json:"-"`
	YearGap   int        `json:"yearGap,omitempty"`
}

var (
	reRank    = regexp.MustCompile(`가장|제일|순위|어디|어느 곳|어느곳|많은 곳|top|TOP|상위|1위`)
	reCompare = regexp.MustCompile(`비교|다른 (시도|지역|곳)|어느 정도|몇 위|전국에서|다른 지역`)
	rePeak    = regexp.MustCompile(`최고|최대|최저|최소|(가장|제일) ?(높|낮|더웠|추웠|뜨거|차가|컸|작았|비싼|싼|비쌌)`)
	reNameCol = regexp.MustCompile(`(명|이름|_NM|NM|name|Name)(\(|$)`)
	reWhere   = regexp.MustCompile(`어디|어느|곳|지역|순위|1위`)
	reTrend   = regexp.MustCompile(`변했|변화|추이|바뀌|줄었|늘었|증가|감소|흐름|몇 년|최근|요즘|그동안|해마다|매년`)
	reRecent  = regexp.MustCompile(`요즘|최근 몇 달|이번 달|지난달|올해`)
	reRate    = regexp.MustCompile(`률|율|지수|체감|전망|평균|BSI|소음도|기온|비율|%|온도|습도`)
	reStock   = regexp.MustCompile(`등록|보유|인구|누적|재고|대수|세대수`)
	reNotMeasure = regexp.MustCompile(`좌표|위도|경도|코드|번호|우편|전화|lat|lon|LAT|LON`)
	reUnit    = regexp.MustCompile(`\(([^()]+)\)\s*$`)
)

func region(q string) []string {
	out := []string{}
	for _, r := range []string{"제주", "서귀포", "서울", "부산", "대구", "인천", "광주", "대전", "울산", "세종", "경기", "강원", "충북", "충남", "전북", "전남", "경북", "경남"} {
		if r == "경기" && !regexp.MustCompile(`경기도|경기 ?지역|경기권`).MatchString(q) {
			continue
		}
		if strings.Contains(q, r) {
			out = append(out, r)
		}
	}
	return out
}

func overlap(a, b string) float64 {
	ga, gb := bigrams(a), map[string]bool{}
	for _, g := range bigrams(b) {
		gb[g] = true
	}
	s := 0.0
	seen := map[string]bool{}
	for _, g := range ga {
		if gb[g] && !seen[g] {
			seen[g] = true
			s++
		}
	}
	return s
}

// MakePlan 은 질문과 열 정보로 계산 계획을 세운다.
func MakePlan(q string, t *Table, cols []Col, in Intent) Plan {
	p := Plan{TimeCol: -1, MonthCol: -1, Group: -1, Filters: map[int]string{}, Agg: "sum", Why: []string{}, Score: map[string]float64{}}
	terms := Terms(q)
	tq := q + " " + strings.Join(terms, " ")
	regs := region(q)

	for _, c := range cols {
		if c.Kind == "time" && p.TimeCol < 0 && c.Distinct > 1 {
			p.TimeCol = c.I
		}
		if c.Kind == "month" && p.MonthCol < 0 {
			p.MonthCol = c.I
		}
	}
	switch {
	case in.FromYear > 0 && in.Year > 0 && in.Year != in.FromYear:
		p.Op, p.OpWhy = "value", fmt.Sprintf("%d년과 %d년을 견줌 → 두 해의 값과 차이", in.FromYear, in.Year)
	case rePeak.MatchString(q) && !reWhere.MatchString(q):
		p.Op, p.OpWhy = "peak", "「"+rePeak.FindString(q)+"」 → 원본 행 가운데 가장 큰(작은) 값 하나"
	case reCompare.MatchString(q):
		p.Op, p.OpWhy = "compare", "「"+reCompare.FindString(q)+"」 → 다른 곳과 비교"
	case reRank.MatchString(q):
		p.Op, p.OpWhy = "rank", "「"+reRank.FindString(q)+"」 → 순위"
	case reTrend.MatchString(q) && !(in.Year > 0 && !strings.Contains(q, "부터")):
		p.Op, p.OpWhy = "trend", "「"+reTrend.FindString(q)+"」 → 기간별 흐름"
	case in.Value || in.Year > 0:
		p.Op, p.OpWhy = "value", "「"+firstNonEmpty(in.YearText, reValueQ.FindString(q))+"」 → 값 하나"
	case p.TimeCol >= 0:
		p.Op, p.OpWhy = "trend", "시간 열 있음 → 기간별 흐름"
	default:
		p.Op, p.OpWhy = "rank", "시간 열 없음 → 순위"
	}

	// 측정 열: 질문 낱말과 열 이름의 2-gram 겹침 + 지역 이름으로 시작하는 열 가산
	best, bestS := -1, 0.0
	nums := []int{}
	for _, c := range cols {
		if c.Kind != "num" || c.Role == "id" || c.Role == "time" || c.Role == "text" {
			continue
		}
		if reNotMeasure.MatchString(c.Label) {
			continue
		}
		nums = append(nums, c.I)
		s := overlap(tq, c.Label)
		if in.Unit != "" && unitFits(in.Unit, c.Label) && s > 0 {
			s += 1.5
		}
		for _, r := range regs {
			if strings.HasPrefix(c.Label, r) {
				s += 3
			}
		}
		p.Score[c.Label] = s
		if s > bestS {
			best, bestS = c.I, s
		}
	}
	ties := 0
	for _, ci := range nums {
		if bestS > 0 && p.Score[cols[ci].Label] == bestS {
			ties++
		}
	}
	if ties >= 3 {
		for _, ci := range nums {
			if p.Score[cols[ci].Label] == bestS && regexp.MustCompile(`\((계|소계|합계)\)`).MatchString(cols[ci].Label) {
				best, ties = ci, 0
				p.Why = append(p.Why, "같은 점수 열이 여럿이라 합계 열 「"+cols[ci].Label+"」을 씀")
				break
			}
		}
	}
	p.MeasureScore = bestS
	if in.Count && p.Op != "trend" && (bestS == 0 || best >= 0 && regexp.MustCompile(`건수|횟수|인원|금액|면적|량`).MatchString(cols[best].Label)) {
		p.Op, p.OpWhy = "count", "「몇 곳·몇 개」 + 맞는 숫자 열 없음 → 행 세기"
		best, ties = -1, 0
	}
	if p.Op == "count" {
		// 행 세기는 측정 열이 필요 없다
	} else if (best < 0 || ties >= 3) && len(nums) >= 3 && (p.Op == "rank" || p.Op == "compare") {
		p.Wide, p.Measure = true, nums
		p.Why = append(p.Why, fmt.Sprintf("질문과 맞는 열 이름이 없고 숫자 열이 %d개 → 열 하나하나를 비교 대상으로 봄", len(nums)))
	} else {
		if best < 0 && len(nums) > 0 {
			best = nums[0]
			p.Why = append(p.Why, "질문과 겹치는 열 이름이 없어 첫 숫자 열을 씀")
		}
		if best >= 0 {
			p.Measure = []int{best}
			p.Why = append(p.Why, fmt.Sprintf("측정 열 「%s」(질문과 겹친 글자 조각 %.0f개)", cols[best].Label, bestS))
		}
	}
	if len(p.Measure) >= 1 {
		c := cols[p.Measure[0]]
		p.Role = c.Role
		if c.Role == "avg" || reRate.MatchString(c.Label) {
			p.Agg, p.Role = "mean", "avg"
			p.Why = append(p.Why, "「"+c.Label+"」은 평균형 값 → 더하지 않고 평균")
		}
		if in.Mean && p.Agg != "mean" {
			p.Agg = "mean"
			p.Why = append(p.Why, "질문이 「평균」을 물음 → 평균")
		}
		if c.Role != "stock" && p.Agg == "sum" && regexp.MustCompile(`등록|보유|현재|지금`).MatchString(q) && reStock.MatchString(q+" "+c.Label) {
			c.Role = "stock"
			p.Why = append(p.Why, "「등록·현재」를 물음 → 쌓여 있는 값으로 봄")
		}
		if c.Role == "stock" {
			p.Stock = true
			p.Why = append(p.Why, "「"+c.Label+"」은 쌓여 있는 값 → 기간을 더하지 않고 각 해의 마지막 시점")
		}
	}

	// 필터·묶음 열: 질문 속 지역·낱말과 같은 값을 가진 범주 열
	for _, c := range cols {
		if c.Kind != "cat" {
			continue
		}
		vals := map[string]bool{}
		for _, r := range t.Rows {
			vals[r[c.I]] = true
		}
		hit := ""
		for v := range vals {
			if v == "" || reTotal.MatchString(v) {
				continue
			}
			for _, w := range append(regs, terms...) {
				// 「제주」는 제주 전체를 뜻하므로 「제주시」·「제주우체국」을 고르지 않는다
				if w == "제주" && v != "제주" && v != "제주도" && v != "제주특별자치도" {
					continue
				}
				if v == w || len([]rune(w)) >= 2 && strings.HasPrefix(v, w) {
					if hit == "" || len(v) < len(hit) {
						hit = v
					}
				}
			}
		}
		if hit == "" {
			continue
		}
		if p.Op == "compare" && p.Group < 0 && len(vals) >= 3 {
			p.Group, p.Target = c.I, hit
			p.Why = append(p.Why, fmt.Sprintf("「%s」 열에서 「%s」을 다른 값과 비교", c.Label, hit))
		} else {
			p.Filters[c.I] = hit
			p.Why = append(p.Why, fmt.Sprintf("「%s」 = 「%s」 만 남김", c.Label, hit))
		}
	}
	if p.Op == "rank" && p.Group < 0 && !p.Wide {
		bestD := 0
		for _, c := range cols {
			if c.Kind == "cat" && c.Distinct >= 2 && c.Distinct <= 40 && p.Filters[c.I] == "" {
				if bestD == 0 || c.Distinct < bestD {
					p.Group, bestD = c.I, c.Distinct
				}
			}
		}
		if p.Group >= 0 {
			p.Why = append(p.Why, fmt.Sprintf("「%s」별로 순위", cols[p.Group].Label))
		}
	}
	if p.TimeCol >= 0 {
		monthly := p.MonthCol >= 0
		if !monthly {
			for _, r := range t.Rows {
				if _, m, ok := ParseTime(r[p.TimeCol]); ok && m > 0 {
					monthly = true
				}
				break
			}
		}
		if monthly && in.Month > 0 && (p.Op == "value" || p.Op == "peak") {
			p.Monthly = true
			p.Why = append(p.Why, fmt.Sprintf("「%d월」을 물음 → 그 달 값", in.Month))
		}
		p.Monthly = p.Monthly || monthly && p.Op == "trend" && (reRecent.MatchString(q) || p.Agg == "mean" && reRecent.MatchString(q))
		if monthly && p.Op == "trend" && !p.Monthly {
			p.Why = append(p.Why, "월별 자료를 연도별로 묶음(12개월이 다 찬 해만)")
		}
		if p.Monthly && p.Op == "trend" {
			p.Why = append(p.Why, "「요즘」 → 최근 24개월을 월별로")
		}
	}
	p.Why = append([]string{p.OpWhy}, p.Why...)
	return p
}

func firstNonEmpty(a, b string) string {
	if a != "" {
		return a
	}
	return b
}

type key struct {
	y, m int
}

func Analyze(q string, t *Table, cols []Col, in Intent) Result {
	p := MakePlan(q, t, cols, in)
	res := Result{Op: p.Op, Plan: p, Cols: cols, Notes: append([]string{}, t.Notes...), RowsTotal: len(t.Rows), Filters: []string{}, Agg: p.Agg}
	res.OpLabel = map[string]string{"trend": "기간별 흐름", "rank": "순위", "compare": "다른 곳과 비교", "value": "값 하나", "count": "개수 세기", "peak": "가장 큰·작은 값"}[p.Op]
	if p.Op == "count" {
		countRows(&res, t, cols, p, in)
		return res
	}
	if p.Op == "peak" && len(p.Measure) == 1 {
		peakRow(&res, q, t, cols, p, in)
		return res
	}
	if len(p.Measure) == 0 {
		res.Notes = append(res.Notes, "숫자로 된 열이 없어 계산할 수 없습니다.")
		return res
	}
	if p.Wide {
		res.Measure = "열별 합계"
		if p.Agg == "mean" {
			res.Measure = "열별 평균"
		}
	} else {
		res.Measure = cols[p.Measure[0]].Label
		if m := reUnit.FindStringSubmatch(res.Measure); m != nil && regexp.MustCompile(`^(명|호|대|천명|천원|백만원|원|%|ha|톤|kg|건|개|㎡|천대|가구|세대|°C|점)$`).MatchString(m[1]) {
			res.Unit = m[1]
		}
	}
	for ci, v := range p.Filters {
		res.Filters = append(res.Filters, cols[ci].Label+" = "+v)
	}
	sort.Strings(res.Filters)

	tot := TotalRows(t, cols)
	// 비교 대상 열 자체가 「전국」 같은 합계 값을 갖고 있으면 그 행은 빼되 기록한다
	rows := []int{}
	for ri, r := range t.Rows {
		ok := true
		for ci, v := range p.Filters {
			if r[ci] != v {
				ok = false
			}
		}
		if !ok {
			continue
		}
		if tot[ri] {
			res.Dropped++
			continue
		}
		rows = append(rows, ri)
	}
	if res.Dropped > 0 {
		res.Notes = append(res.Notes, fmt.Sprintf("「계·합계·전국」 같은 합계 행 %d개를 뺐습니다. 그대로 더하면 같은 숫자를 두 번 셉니다.", res.Dropped))
	}

	tkey := func(r []string) (key, bool) {
		if p.TimeCol < 0 {
			return key{}, false
		}
		y, m, ok := ParseTime(r[p.TimeCol])
		if !ok {
			return key{}, false
		}
		if p.MonthCol >= 0 {
			if mv, ok := Num(r[p.MonthCol]); ok {
				m = int(mv)
			}
		}
		return key{y, m}, true
	}
	val := func(r []string) (float64, bool) { return Num(r[p.Measure[0]]) }
	agg := func(xs []float64) float64 {
		s := 0.0
		for _, x := range xs {
			s += x
		}
		if p.Agg == "mean" && len(xs) > 0 {
			return s / float64(len(xs))
		}
		return s
	}

	used := map[int]bool{}
	switch {
	case p.Op == "value" && p.TimeCol < 0:
		xs := []float64{}
		for _, ri := range rows {
			if v, ok := val(t.Rows[ri]); ok {
				xs = append(xs, v)
				used[ri] = true
			}
		}
		if len(xs) > 0 {
			res.Series = []Point{{K: "전체", V: round(agg(xs)), N: len(xs), Hot: true}}
			res.Period = "자료 전체"
			valueFacts(&res, in, 0)
		}
	case p.Op == "trend" || p.Op == "value":
		if !p.Stock && p.Agg == "sum" && cumulative(t, rows, tkey, val) {
			p.Stock = true
			res.Plan.Stock = true
			res.Notes = append(res.Notes, "달마다 1월부터 쌓아 올린 누계 값이라, 해마다 마지막 달 값을 그 해 값으로 썼습니다.")
		}
		byT := map[key][]float64{}
		months := map[int]map[int]bool{}
		// 쌓여 있는 값(인구·등록 대수·누계)은 해마다 마지막 달만 쓴다 — 12개월을 더하면 12배가 된다
		lastM := map[int]int{}
		if p.Stock && !p.Monthly {
			for _, ri := range rows {
				if k, ok := tkey(t.Rows[ri]); ok && k.m > lastM[k.y] {
					lastM[k.y] = k.m
				}
			}
		}
		for _, ri := range rows {
			k, ok := tkey(t.Rows[ri])
			v, ok2 := val(t.Rows[ri])
			if !ok || !ok2 {
				continue
			}
			if p.Stock && !p.Monthly && k.m != lastM[k.y] {
				continue
			}
			used[ri] = true
			if !p.Monthly {
				if months[k.y] == nil {
					months[k.y] = map[int]bool{}
				}
				months[k.y][k.m] = true
				k.m = 0
			}
			byT[k] = append(byT[k], v)
		}
		ks := make([]key, 0, len(byT))
		for k := range byT {
			ks = append(ks, k)
		}
		sort.Slice(ks, func(a, b int) bool { return ks[a].y*100+ks[a].m < ks[b].y*100+ks[b].m })
		partial := []string{}
		for _, k := range ks {
			if !p.Monthly && !p.Stock && p.Agg == "sum" && len(months[k.y]) > 1 && len(months[k.y]) < 12 && !(p.Op == "value" && k.y == in.Year) {
				partial = append(partial, strconv.Itoa(k.y))
				continue
			}
			lab := strconv.Itoa(k.y)
			if p.Monthly {
				lab = fmt.Sprintf("%d.%02d", k.y, k.m)
			}
			res.Series = append(res.Series, Point{K: lab, V: round(agg(byT[k])), N: len(byT[k])})
		}
		if len(partial) > 0 {
			res.Notes = append(res.Notes, fmt.Sprintf("%s년은 12개월이 다 차지 않아 연도 비교에서 뺐습니다.", strings.Join(partial, "·")))
		}
		if p.Monthly && p.Op == "trend" && len(res.Series) > 24 {
			res.Series = res.Series[len(res.Series)-24:]
		}
		if p.Stock && !p.Monthly && len(lastM) > 0 {
			res.Notes = append(res.Notes, "쌓여 있는 값이라 달마다 더하지 않고, 해마다 마지막 달 값을 그 해 값으로 썼습니다.")
		}
		if p.Op == "value" {
			valueFacts(&res, in, 0)
			break
		}
		trendFacts(&res)
		// 마지막 시점 구성(범주 열 하나, 12개 이하)
		if len(res.Series) > 0 && !p.Monthly {
			last := res.Series[len(res.Series)-1].K
			bc, bd := -1, 0
			for _, c := range cols {
				if c.Kind == "cat" && p.Filters[c.I] == "" && c.Distinct >= 2 && c.Distinct <= 12 && (bd == 0 || c.Distinct < bd) {
					bc, bd = c.I, c.Distinct
				}
			}
			if bc >= 0 {
				m := map[string][]float64{}
				for ri := range used {
					r := t.Rows[ri]
					if k, _ := tkey(r); strconv.Itoa(k.y) == last {
						if v, ok := val(r); ok {
							m[r[bc]] = append(m[r[bc]], v)
						}
					}
				}
				for _, k := range sortedKeys(m) {
					res.Breakdown = append(res.Breakdown, Point{K: k, V: round(agg(m[k])), N: len(m[k])})
				}
				sort.SliceStable(res.Breakdown, func(a, b int) bool { return res.Breakdown[a].V > res.Breakdown[b].V })
				res.BreakBy = fmt.Sprintf("%s년 %s별", last, cols[bc].Label)
			}
		}
	default: // rank · compare — 가장 최근 시점(월별이면 최근 12개월)으로 자른다
		latest := map[int]bool{}
		if p.TimeCol >= 0 {
			type ym struct{ y, m int }
			all := []int{}
			for _, ri := range rows {
				if k, ok := tkey(t.Rows[ri]); ok {
					all = append(all, k.y*100+k.m)
				}
			}
			sort.Ints(all)
			yearSel := 0
			if in.Year > 0 && len(all) > 0 {
				inYear := []int{}
				for _, v := range all {
					if v/100 == in.Year {
						inYear = append(inYear, v)
					}
				}
				if len(inYear) > 0 {
					all, yearSel = inYear, in.Year
				} else {
					yearMissing(&res, in, all[len(all)-1]/100)
				}
			}
			if len(all) > 0 {
				mx := all[len(all)-1]
				monthly := mx%100 > 0
				// 등록 대수·인구처럼 쌓인 값(잔량)은 12개월을 더하면 안 된다 → 최근 한 달만
				stock := monthly && !p.Wide && len(p.Measure) == 1 && (p.Stock || reStock.MatchString(cols[p.Measure[0]].Label))
				for _, ri := range rows {
					k, ok := tkey(t.Rows[ri])
					if !ok {
						continue
					}
					v := k.y*100 + k.m
					if !monthly && k.y == mx/100 || monthly && !stock && yearSel > 0 && k.y == yearSel || monthly && !stock && yearSel == 0 && monthsBetween(v, mx) < 12 || stock && v == mx {
						latest[ri] = true
					}
				}
				if stock {
					res.Period = ymLabel(mx) + " 시점"
					res.Notes = append(res.Notes, "등록 대수처럼 쌓여 가는 값이라 12개월을 더하지 않고 가장 최근 달 값만 썼습니다.")
				} else if monthly && yearSel > 0 {
					res.Period = fmt.Sprintf("%d년", yearSel)
				} else if monthly {
					res.Period = fmt.Sprintf("최근 12개월(%s까지)", ymLabel(mx))
				} else {
					res.Period = fmt.Sprintf("%d년", mx/100)
				}
			}
		}
		inScope := func(ri int) bool { return p.TimeCol < 0 || latest[ri] }
		m := map[string][]float64{}
		if p.Wide {
			for _, ri := range rows {
				if !inScope(ri) {
					continue
				}
				for _, ci := range p.Measure {
					if v, ok := Num(t.Rows[ri][ci]); ok {
						m[cols[ci].Label] = append(m[cols[ci].Label], v)
						used[ri] = true
					}
				}
			}
		} else if p.Group >= 0 {
			for _, ri := range rows {
				if !inScope(ri) {
					continue
				}
				if v, ok := val(t.Rows[ri]); ok {
					m[t.Rows[ri][p.Group]] = append(m[t.Rows[ri][p.Group]], v)
					used[ri] = true
				}
			}
		}
		for _, k := range sortedKeys(m) {
			s := 0.0
			for _, x := range m[k] {
				s += x
			}
			if p.Agg == "mean" {
				s /= float64(len(m[k]))
			}
			res.Series = append(res.Series, Point{K: k, V: round(s), N: len(m[k]), Hot: k == p.Target})
		}
		sort.SliceStable(res.Series, func(a, b int) bool { return res.Series[a].V > res.Series[b].V })
		rankFacts(&res, p.Target)
	}
	res.RowsUsed = len(used)
	// 근거 행: 계산에 실제로 쓴 원본 행 앞 8줄
	res.EvHeader = t.Header
	idxs := make([]int, 0, len(used))
	for ri := range used {
		idxs = append(idxs, ri)
	}
	sort.Ints(idxs)
	if p.Op != "trend" && len(idxs) > 8 {
		// 순위·비교면 1위와 대상 행이 보이도록 앞쪽 행 대신 값이 큰 행을 고른다
		sort.SliceStable(idxs, func(a, b int) bool {
			va, _ := Num(t.Rows[idxs[a]][p.Measure[0]])
			vb, _ := Num(t.Rows[idxs[b]][p.Measure[0]])
			return va > vb
		})
	}
	for i, ri := range idxs {
		if i >= 8 {
			break
		}
		res.Evidence = append(res.Evidence, append([]string{strconv.Itoa(ri + 2)}, t.Rows[ri]...))
	}
	res.EvHeader = append([]string{"원본 줄"}, t.Header...)
	return res
}

func monthsBetween(a, b int) int { return (b/100-a/100)*12 + (b%100 - a%100) }
func ymLabel(v int) string     { return fmt.Sprintf("%d년 %d월", v/100, v%100) }

func round(v float64) float64 {
	if math.Abs(v) >= 100 {
		return math.Round(v)
	}
	return math.Round(v*10) / 10
}

func Fmt(v float64) string {
	neg := v < 0
	v = math.Abs(v)
	var s string
	if v == math.Trunc(v) {
		s = strconv.FormatInt(int64(v), 10)
		for i := len(s) - 3; i > 0; i -= 3 {
			s = s[:i] + "," + s[i:]
		}
	} else {
		s = strconv.FormatFloat(v, 'f', 1, 64)
	}
	if neg {
		return "-" + s
	}
	return s
}

func pct(a, b float64) float64 {
	if a == 0 {
		return 0
	}
	return math.Round((b-a)/math.Abs(a)*1000) / 10
}

func trendFacts(r *Result) {
	s := r.Series
	if len(s) == 0 {
		return
	}
	f, l := s[0], s[len(s)-1]
	mx, mn := f, f
	for _, x := range s {
		if x.V > mx.V {
			mx = x
		}
		if x.V < mn.V {
			mn = x
		}
	}
	u := r.Unit
	add := func(k, lab, v string) { r.Facts = append(r.Facts, Fact{k, lab, v}) }
	add("first", f.K+" 값", Fmt(f.V)+u)
	add("last", l.K+" 값", Fmt(l.V)+u)
	if len(s) > 1 {
		c := pct(f.V, l.V)
		dir := "늘었다"
		if c < 0 {
			dir = "줄었다"
		}
		if r.Agg == "mean" {
			add("change", f.K+"→"+l.K+" 차이", fmt.Sprintf("%s포인트 %s", Fmt(round(math.Abs(l.V-f.V))), map[bool]string{true: "높아졌다", false: "낮아졌다"}[l.V >= f.V]))
		} else {
			add("change", f.K+"→"+l.K+" 변화율", fmt.Sprintf("%s%% %s", Fmt(math.Abs(c)), dir))
		}
		p := s[len(s)-2]
		add("prev", "직전("+p.K+") 대비", fmt.Sprintf("%s%%", Fmt(pct(p.V, l.V))))
	}
	add("max", "가장 높았던 때", mx.K+" "+Fmt(mx.V)+u)
	add("min", "가장 낮았던 때", mn.K+" "+Fmt(mn.V)+u)
	r.Period = f.K + "~" + l.K
}

func rankFacts(r *Result, target string) {
	s := r.Series
	if len(s) == 0 {
		return
	}
	add := func(k, lab, v string) { r.Facts = append(r.Facts, Fact{k, lab, v}) }
	u := r.Unit
	add("top1", "1위", s[0].K+" "+Fmt(s[0].V)+u)
	if len(s) > 1 {
		add("top2", "2위", s[1].K+" "+Fmt(s[1].V)+u)
	}
	total := 0.0
	for _, x := range s {
		total += x.V
	}
	if r.Agg != "mean" && total > 0 {
		add("top1share", "1위 비중", Fmt(round(s[0].V/total*1000)/10)+"%")
	}
	add("count", "비교한 수", strconv.Itoa(len(s))+"곳")
	if target != "" {
		for i, x := range s {
			if x.K == target {
				add("target", target+" 순위", fmt.Sprintf("%d곳 중 %d위", len(s), i+1))
				add("targetv", target+" 값", Fmt(x.V)+u)
				if r.Agg != "mean" && total > 0 {
					add("targetshare", target+" 비중", Fmt(round(x.V/total*1000)/10)+"%")
				}
				vs := make([]float64, len(s))
				for j, y := range s {
					vs[j] = y.V
				}
				sort.Float64s(vs)
				med := vs[len(vs)/2]
				if med > 0 {
					add("vsmed", "중간값 대비", Fmt(round(x.V/med*10)/10)+"배")
				}
			}
		}
	}
}

// yearMissing 은 질문이 가리킨 해가 자료에 없을 때다. 「작년」처럼 상대 시점이면 가장 최근 해로 답하되 밝히고,
// 「2019년」처럼 콕 집은 해면 다른 해 값을 내놓지 않는다(의미 검산에서 거절).
func yearMissing(r *Result, in Intent, latest int) {
	r.YearGap = in.Year - latest
	if in.Relative {
		r.Notes = append(r.Notes, fmt.Sprintf("「%s」(%d년) 자료가 아직 없어, 가장 최근인 %d년 값으로 답했습니다.", in.YearText, in.Year, latest))
		r.YearNote = fmt.Sprintf("%s(%d년) 자료 없음 → %d년 값", in.YearText, in.Year, latest)
	} else {
		r.YearMiss = fmt.Sprintf("질문한 %d년 자료가 없습니다(가장 최근 %d년)", in.Year, latest)
	}
}

// valueFacts 는 값 하나를 묻는 질문의 사실표다. 질문한 해(없으면 가장 최근 해)의 값을 고른다.
func valueFacts(r *Result, in Intent, _ int) {
	s := r.Series
	if len(s) == 0 {
		return
	}
	pick := len(s) - 1
	if in.Year > 0 && s[0].K != "전체" {
		found := -1
		want := strconv.Itoa(in.Year)
		if in.Month > 0 {
			want = fmt.Sprintf("%d.%02d", in.Year, in.Month)
		}
		for i, x := range s {
			if strings.HasPrefix(x.K, want) {
				found = i
			}
		}
		if found >= 0 {
			pick = found
		} else {
			latest, _ := strconv.Atoi(s[len(s)-1].K[:4])
			yearMissing(r, in, latest)
		}
	}
	for i := range s {
		s[i].Hot = i == pick
	}
	x := s[pick]
	u := r.Unit
	lab := x.K
	if len(lab) == 4 {
		lab += "년"
	}
	r.Period = lab
	add := func(k, l, v string) { r.Facts = append(r.Facts, Fact{k, l, v}) }
	add("value", lab+" "+r.Measure, Fmt(x.V)+u)
	if in.Sign {
		add("sign", "0보다", map[bool]string{true: "커서 플러스(늘어남)", false: "작아서 마이너스(줄어듦)"}[x.V > 0])
	}
	if in.FromYear > 0 {
		for _, f := range s {
			if strings.HasPrefix(f.K, strconv.Itoa(in.FromYear)) {
				c := pct(f.V, x.V)
				add("from", f.K+"년 값", Fmt(f.V)+u)
				add("change", f.K+"년→"+lab+" 변화", fmt.Sprintf("%s%s (%s%%)", map[bool]string{true: "+", false: ""}[x.V >= f.V], Fmt(round(x.V-f.V))+u, Fmt(c)))
				add("dir", "방향", map[bool]string{true: "늘었습니다", false: "줄었습니다"}[x.V >= f.V])
			}
		}
	}
	if pick > 0 && s[0].K != "전체" {
		p := s[pick-1]
		add("prev", "직전("+p.K+") 대비", fmt.Sprintf("%s%%", Fmt(pct(p.V, x.V))))
	}
	if r.Agg == "mean" {
		add("agg", "계산 방법", fmt.Sprintf("원본 %d행의 평균", x.N))
	} else if r.Plan.Stock {
		add("agg", "계산 방법", "그 해 마지막 시점 값")
	} else {
		add("agg", "계산 방법", fmt.Sprintf("원본 %d행의 합계", x.N))
	}
}

// countRows 는 「몇 곳·몇 개」 질문 — 걸러진 행 수를 센다(합계 행 제외).
func countRows(r *Result, t *Table, cols []Col, p Plan, in Intent) {
	tot := TotalRows(t, cols)
	// 같은 이름이 여러 줄(지점·기간별)로 나오면 이름 기준으로 한 번만 센다
	nameCol := -1
	for _, c := range cols {
		if c.Kind == "cat" && reNameCol.MatchString(c.Label) && c.Distinct < len(t.Rows) {
			nameCol = c.I
			break
		}
	}
	seen := map[string]bool{}
	n := 0
	for ri, row := range t.Rows {
		ok := true
		for ci, v := range p.Filters {
			if row[ci] != v {
				ok = false
			}
		}
		if !ok || tot[ri] {
			continue
		}
		if nameCol >= 0 {
			if row[nameCol] == "" || seen[row[nameCol]] {
				continue
			}
			seen[row[nameCol]] = true
		}
		n++
		if len(r.Evidence) < 8 {
			r.Evidence = append(r.Evidence, append([]string{strconv.Itoa(ri + 2)}, row...))
		}
	}
	for ci, v := range p.Filters {
		r.Filters = append(r.Filters, cols[ci].Label+" = "+v)
	}
	sort.Strings(r.Filters)
	r.EvHeader = append([]string{"원본 줄"}, t.Header...)
	r.Measure, r.Unit, r.RowsUsed = "행 수(목록 개수)", "곳", n
	r.Period = "자료 전체"
	if n > 0 {
		r.Series = []Point{{K: "전체", V: float64(n), N: n, Hot: true}}
		r.Facts = append(r.Facts, Fact{"value", "목록에 있는 수", Fmt(float64(n)) + "곳"})
		how := "조건에 맞는 원본 행을 하나씩 셈"
		if nameCol >= 0 {
			how = "「" + cols[nameCol].Label + "」이 같은 줄은 한 번만 셈"
		}
		r.Facts = append(r.Facts, Fact{"agg", "계산 방법", how})
	}
}

// cumulative 는 월별 값이 해마다 1월부터 쌓이는 누계인지 본다(해 안에서 줄지 않고, 해가 바뀌면 다시 작아진다).
func cumulative(t *Table, rows []int, tkey func([]string) (key, bool), val func([]string) (float64, bool)) bool {
	sum := map[key]float64{}
	for _, ri := range rows {
		k, ok := tkey(t.Rows[ri])
		v, ok2 := val(t.Rows[ri])
		if ok && ok2 && k.m > 0 {
			sum[k] += v
		}
	}
	byY := map[int][]int{}
	for k := range sum {
		byY[k.y] = append(byY[k.y], k.m)
	}
	years, rises, resets := 0, 0, 0
	prevLast := -1.0
	ys := []int{}
	for y := range byY {
		ys = append(ys, y)
	}
	sort.Ints(ys)
	for _, y := range ys {
		ms := byY[y]
		sort.Ints(ms)
		if len(ms) < 4 {
			continue
		}
		years++
		up := true
		for i := 1; i < len(ms); i++ {
			if sum[key{y, ms[i]}] < sum[key{y, ms[i-1]}] {
				up = false
			}
		}
		// 마지막 달이 첫 달의 2배 이상이어야 「쌓였다」고 본다
		if up && sum[key{y, ms[len(ms)-1]}] >= 2*sum[key{y, ms[0]}] {
			rises++
		}
		if prevLast >= 0 && sum[key{y, ms[0]}] < prevLast/2 {
			resets++
		}
		prevLast = sum[key{y, ms[len(ms)-1]}]
	}
	return years >= 2 && rises == years && resets >= years-1
}

// peakRow 는 「제일 더웠던 날」처럼 원본 행 하나의 최댓값·최솟값을 찾는다(더하거나 평균 내지 않는다).
func peakRow(r *Result, q string, t *Table, cols []Col, p Plan, in Intent) {
	ci := p.Measure[0]
	r.Measure = cols[ci].Label
	low := regexp.MustCompile(`최저|최소|낮|추웠|차가|작았`).MatchString(rePeak.FindString(q))
	tot := TotalRows(t, cols)
	best, bv, n := -1, 0.0, 0
	for ri, row := range t.Rows {
		ok := !tot[ri]
		for fc, v := range p.Filters {
			if row[fc] != v {
				ok = false
			}
		}
		if ok && in.Year > 0 && p.TimeCol >= 0 {
			if y, _, okT := ParseTime(row[p.TimeCol]); !okT || y != in.Year {
				ok = false
			}
		}
		if ok && in.FromYear > 0 && p.TimeCol >= 0 {
			if y, _, okT := ParseTime(row[p.TimeCol]); !okT || y < in.FromYear {
				ok = false
			}
		}
		v, okV := Num(row[ci])
		if !ok || !okV {
			continue
		}
		n++
		if best < 0 || (!low && v > bv) || (low && v < bv) {
			best, bv = ri, v
		}
	}
	for fc, v := range p.Filters {
		r.Filters = append(r.Filters, cols[fc].Label+" = "+v)
	}
	sort.Strings(r.Filters)
	if best < 0 {
		r.Notes = append(r.Notes, "조건에 맞는 숫자 행이 없습니다.")
		return
	}
	when := ""
	for _, c := range cols {
		if c.Kind == "cat" && reNameCol.MatchString(c.Label) && regexp.MustCompile(`뭐|무엇|어떤|이름`).MatchString(q) {
			when = t.Rows[best][c.I]
			break
		}
	}
	if when != "" {
	} else if p.TimeCol >= 0 {
		when = t.Rows[best][p.TimeCol]
	} else {
		for _, c := range cols {
			if c.Kind == "cat" {
				when = t.Rows[best][c.I]
				break
			}
		}
	}
	r.Period = when
	r.RowsUsed = n
	word := map[bool]string{true: "가장 작은 값", false: "가장 큰 값"}[low]
	r.Series = []Point{{K: when, V: round(bv), N: 1, Hot: true}}
	r.Facts = append(r.Facts, Fact{"value", r.Measure + " " + word, Fmt(round(bv))}, Fact{"when", "그 값이 나온 행", when},
		Fact{"agg", "계산 방법", fmt.Sprintf("원본 %d행 가운데 %s 하나(더하지 않음)", n, word)})
	r.EvHeader = t.Header
	r.Evidence = [][]string{append([]string{strconv.Itoa(best + 2)}, t.Rows[best]...)}
	r.Code = fmt.Sprintf("df[%q].%s()", cols[ci].Label, map[bool]string{true: "idxmin", false: "idxmax"}[low])
}
