package agent

// 의미 검산 — 숫자가 맞아도 「다른 것」을 센 답이면 틀린 답이다.
// 계산을 마친 뒤, 답을 내보내기 전에 다섯 가지를 확인한다. 하나라도 어긋나면 답을 내지 않고 거절한다.
//   ① 주제: 질문의 핵심 낱말이 데이터 제목·태그·설명에 있는가
//   ② 지표: 계산한 열이 질문이 묻는 것(관광객·인구·생산량…)과 같은 것인가
//   ③ 연산: 열 역할에 맞는 연산인가(평균형은 평균, 쌓인 값은 마지막 시점, 번호·시간 열은 측정값 금지)
//   ④ 대상: 질문의 지역과 데이터의 지역이 같은가(자매도시·다른 시도·다른 시를 제주시로 답하지 않는다)
//   ⑤ 시점·단위: 「작년」「2022년」 「몇 명」「몇 대」가 계산과 맞는가

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

type CheckItem struct {
	Name   string `json:"name"`
	OK     bool   `json:"ok"`
	Detail string `json:"detail"`
}

type Meaning struct {
	Pass  bool        `json:"pass"`
	Items []CheckItem `json:"items"`
}

func (m Meaning) Fails() []string {
	out := []string{}
	for _, it := range m.Items {
		if !it.OK {
			out = append(out, it.Name+": "+it.Detail)
		}
	}
	return out
}

var (
	// 질문 속 낱말 중 주제를 가리키지 않는 말
	generic = setOf("몇", "명", "개", "대", "곳", "얼마", "가장", "많은", "많아요", "적은", "어디", "어디예요", "어느", "순위", "변화", "추이", "흐름",
		"전체", "모두", "총", "기준", "통계", "자료", "데이터", "수", "숫자", "규모", "평균", "작년", "올해", "재작년", "지난해", "해마다", "매년",
		"늘었어", "줄었어", "늘었나요", "줄었나요", "많나요", "있나요", "있어요", "됐어요", "되나요", "되요", "돼요", "인가요", "이에요", "예요",
		"제주시", "서귀포시", "서귀포", "제주", "도내", "우리", "동네", "지역", "시도", "다른", "비교", "높아요", "낮아요", "많이", "정도", "대략", "알려줘요")
	reForeignSubject = regexp.MustCompile(`자매|결연|우호|교류도시|해외|국외|외국 ?도시|국제도시`)
	reAskForeign     = regexp.MustCompile(`자매|결연|우호|교류|해외|외국|국제`)
	sido             = []string{"서울", "부산", "대구", "인천", "광주", "대전", "울산", "세종", "경기", "강원", "충북", "충남", "전북", "전남", "경북", "경남"}
	reWordAll        = regexp.MustCompile(`[\p{Hangul}A-Za-z0-9]+`)
	reAfterMyeot     = regexp.MustCompile(`몇\s*([\p{Hangul}]+)`)
	reDateWord       = regexp.MustCompile(`^(\d+|(19|20)?\d{2})(년|월|일|년도|년엔|년에|월에|일에|년이랑|년보다|년과|년부터|년까지|년말|말|분기|주)?$|^\d+(년|월)`)
	// 말끝(서술어·의문)으로 끝나는 낱말 — 주제가 아니다
	rePredicate = regexp.MustCompile(`(었어|았어|였어|됐어|왔어|했어|했나|었나|았나|였나|됐나|었던|았던|였던|더웠|많았|적었|높았|낮았|이야|어요|에요|예요|나요|니까|는지|인지|이고|어디고|뭐야|언제야|뭐였어|였어|이었|거야|건가|일까|할까|있는|없는|있어|없어|하는|되는|된|한|짓는|찍힌|나온|생산됐|비교해서|비교하면|해서|하면|보다|이랑|이후|이전|중에|정도|만큼)$`)
)

func isPredicate(w string) bool {
	if rePredicate.MatchString(w) {
		return true
	}
	return predicateWords[w]
}

var fillerWords = setOf("얼마", "어디", "무슨", "내면", "나는", "나오", "달", "우리나라", "상반기", "하반기", "한 해", "동안", "자주", "언제", "몇", "얼마치")

var predicateWords = setOf("제일", "언제", "어디", "뭐", "누가", "얼마", "수치", "지금", "현재", "기준", "최근", "동안", "한해", "해", "해가", "때", "사람", "이후", "이전",
	"알려줘", "알려", "보여줘", "궁금", "나와", "나와요", "있나", "있어", "있었어", "됐어", "되나", "많이", "많은", "적게", "높은", "낮은", "더", "덜", "가장", "모두", "다", "각각", "총합", "합계", "평균")

// topicTerms 는 질문에서 주제를 가리키는 낱말(조사·말투·지역·일반어 제외)과 그 비슷한 말이다.
func topicTerms(q string) (core []string, expanded map[string][]string) {
	expanded = map[string][]string{}
	// 「몇 대나」·「몇 호야」처럼 「몇」 바로 뒤 낱말은 단위다
	counters := map[string]bool{}
	for _, m := range reAfterMyeot.FindAllStringSubmatch(q, -1) {
		counters[stripJosa(m[1])] = true
		counters[m[1]] = true
	}
	for _, w := range Terms(q) {
		// 말끝(용도야·농가들·달은)을 떼고 본다
		for _, suf := range []string{"이야", "예요", "이에요", "야", "들", "은", "는"} {
			if r := []rune(w); len(r) >= 3 && strings.HasSuffix(w, suf) {
				w = strings.TrimSuffix(w, suf)
				break
			}
		}
		if fillerWords[w] {
			continue
		}
		if generic[w] || stop[w] || counters[w] || isPredicate(w) || reYearBare.MatchString(" "+w+" ") || reDateWord.MatchString(w) {
			continue
		}
		isSyn := false
		for _, syns := range synonym {
			for _, s := range syns {
				if s == w {
					isSyn = true
				}
			}
		}
		raw := false
		for _, x := range reWordAll.FindAllString(q, -1) {
			if strings.Contains(x, w) {
				raw = true
			}
		}
		if !raw && isSyn {
			continue
		}
		core = append(core, w)
		for k, syns := range synonym {
			if strings.Contains(w, k) {
				expanded[w] = append(expanded[w], syns...)
			}
		}
	}
	return
}

// hitText 는 낱말 w(또는 비슷한 말·앞 두 글자 줄기)가 s 에 들어 있는가.
func hitText(w string, syns []string, s string) bool {
	s = strings.ToLower(strings.ReplaceAll(s, " ", ""))
	if strings.Contains(s, strings.ToLower(w)) {
		return true
	}
	for _, x := range syns {
		if strings.Contains(s, strings.ToLower(x)) {
			return true
		}
	}
	// 3글자 이상 낱말은 앞 2글자 줄기로도 본다(관광객 ↔ 관광)
	// 4글자 합성어(교통사고)는 앞·뒤 줄기가 모두 있어야 한다 — 「교통」만 있는 데이터로 사고를 답하지 않게
	if r := []rune(w); len(r) >= 4 {
		return strings.Contains(s, string(r[:2])) && strings.Contains(s, string(r[len(r)-2:]))
	}
	if r := []rune(w); len(r) >= 3 && strings.Contains(s, string(r[:2])) {
		return true
	}
	return false
}

// CheckMeaning 은 계산 결과가 질문과 같은 것을 셌는지 본다.
func CheckMeaning(q string, in Intent, d *Dataset, t *Table, cols []Col, r *Result) Meaning {
	m := Meaning{Pass: true}
	add := func(name string, ok bool, detail string) {
		m.Items = append(m.Items, CheckItem{name, ok, detail})
		if !ok {
			m.Pass = false
		}
	}
	core, exp := topicTerms(q)
	meta := d.Title + " " + d.Tags + " " + d.Cat + " " + d.Desc
	label := r.Measure

	// ① 주제
	hit, miss := []string{}, []string{}
	for _, w := range core {
		if hitText(w, exp[w], meta) || hitText(w, exp[w], strings.Join(t.Header, " ")) && hitText(w, exp[w], d.Title+d.Desc) {
			hit = append(hit, w)
		} else {
			miss = append(miss, w)
		}
	}
	switch {
	case len(core) == 0:
		add("주제", false, "질문에서 무엇을 묻는지 낱말을 찾지 못함")
	case len(hit) == 0:
		add("주제", false, "질문의 "+quoteJo([]string{strings.Join(core, "·")}, "이", "가")+" 데이터 제목·설명에 없음")
	case len(miss) > 0 && len(hit)*2 < len(core):
		add("주제", false, quoteJo([]string{strings.Join(hit, "·")}, "은", "는")+" 맞지만 "+quoteJo([]string{strings.Join(miss, "·")}, "이", "가")+" 데이터에 없음")
	default:
		add("주제", true, quoteJo([]string{strings.Join(hit, "·")}, "이", "가")+" 데이터 제목·설명에 있음")
	}

	// ② 지표 — 계산한 열이 질문의 지표와 같은가
	if r.Op == "count" {
		// 「특수학급 있는 유치원 몇 곳」: 조건이 열 이름이면 그 열로 걸러야 한다 — 거르지 않고 다 세면 틀린 답
		cond := ""
		for _, w := range core {
			for _, c := range cols {
				if hitText(w, exp[w], c.Label) && r.Plan.Filters[c.I] == "" {
					cond = c.Label
				}
			}
		}
		if cond != "" {
			add("지표", false, "조건 열 「"+cond+"」로 거르지 못하고 전체 행을 셈")
		} else {
			add("지표", true, "목록의 행을 셈")
		}
	} else if r.Plan.Wide {
		units := map[string]bool{}
		for _, ci := range r.Plan.Measure {
			u := ""
			if m := reUnit.FindStringSubmatch(cols[ci].Label); m != nil {
				u = m[1]
			}
			units[u] = true
		}
		roles := map[string]bool{}
		for _, ci := range r.Plan.Measure {
			roles[map[bool]string{true: "avg", false: "sum"}[cols[ci].Role == "avg"]] = true
		}
		if len(units) > 1 || len(roles) > 1 {
			add("지표", false, "단위가 다른 열(금액·무게 등)을 한 줄로 견줄 수 없음")
		} else {
			add("지표", true, "열 하나하나가 비교 대상")
		}
	} else if len(r.Plan.Measure) == 0 {
		add("지표", false, "계산할 숫자 열이 없음")
	} else {
		lab := cols[r.Plan.Measure[0]].Label
		ok := r.Plan.MeasureScore > 0
		for _, w := range core {
			if hitText(w, exp[w], lab) {
				ok = true
			}
		}
		// 질문이 지표 이름을 안 썼을 때(「몇 명 왔어요」): 숫자 열이 하나뿐이고 단위가 맞으면 그 열이 곧 지표다
		if !ok {
			n := 0
			for _, c := range cols {
				if c.Kind == "num" && (c.Role == "sum" || c.Role == "avg" || c.Role == "stock") {
					n++
				}
			}
			if n == 1 && in.Unit != "" && unitFits(in.Unit, lab) {
				ok = true
			}
		}
		if q2 := extraQualifier(q, lab); ok && q2 != "" {
			add("지표", false, "열 「"+lab+"」에는 질문에 없는 조건 「"+q2+"」이 붙음라 다른 것을 셈")
		} else if ok {
			add("지표", true, "계산한 열 「"+lab+"」이 질문과 맞음")
		} else {
			add("지표", false, "질문과 맞는 열이 없어 「"+lab+"」을 골랐음는데, 다른 것을 셀 수 있음")
		}
	}

	// ③ 연산과 열 역할
	if r.Op != "count" && len(r.Plan.Measure) > 0 && !r.Plan.Wide {
		c := cols[r.Plan.Measure[0]]
		switch {
		case r.Op == "peak" && (c.Role == "id" || c.Role == "time"):
			add("연산", false, "「"+c.Label+"」은 측정값이 아님")
		case r.Op == "peak":
			add("연산", true, "원본 행 하나의 최댓값·최솟값(더하지 않음)")
		case c.Role == "id" || c.Role == "time":
			add("연산", false, "「"+c.Label+"」은 "+c.RoleWhy+"라 측정값이 아님")
		case c.Role == "avg" && r.Agg != "mean":
			add("연산", false, "「"+c.Label+"」은 평균형인데 더했음")
		case c.Role == "avg":
			add("연산", true, "평균형 열 → 평균")
		case r.Plan.Stock:
			add("연산", true, "쌓여 있는 값 → 기간을 더하지 않음")
		default:
			add("연산", true, "합산 가능한 열 → 합계")
		}
	} else {
		add("연산", true, map[bool]string{true: "행 세기", false: "열별 계산"}[r.Op == "count"])
	}

	// ④ 대상(지역)
	add(subject(q, in, d, t, cols, r))

	// ⑤ 시점·단위
	yy := fmt.Sprintf("%02d년", in.Year%100)
	noYear := in.Year > 0 && r.Plan.TimeCol < 0 && !strings.Contains(d.Title, strconv.Itoa(in.Year)) && !strings.Contains(d.Title, yy) && !strings.Contains(r.Period, strconv.Itoa(in.Year))
	switch {
	case noYear:
		add("시점", false, fmt.Sprintf("데이터에 연도 구분이 없어 %d년 값인지 알 수 없음", in.Year))
	case r.YearMiss != "":
		add("시점", false, r.YearMiss)
	case r.YearNote != "" && (r.YearGap > 1 || in.Month > 0):
		add("시점", false, r.YearNote+". 묻는 때와 너무 멀어 답하지 않음")
	case r.YearNote != "":
		add("시점", true, r.YearNote+"(답에 밝힘)")
	case in.Year > 0:
		add("시점", true, fmt.Sprintf("%s → %d년", in.YearText, in.Year))
	default:
		add("시점", true, "시점 지정 없음 → "+firstNonEmpty(r.Period, "자료 전체"))
	}
	if in.Unit != "" && r.Op != "count" && !r.Plan.Wide {
		u := label + " " + r.Unit
		if unitFits(in.Unit, u) {
			add("단위", true, "「몇 "+in.Unit+"」 ↔ 「"+label+"」")
		} else {
			add("단위", false, "질문은 「"+in.Unit+"」 단위를 묻는데 계산한 값은 「"+label+"」")
		}
	} else if in.Unit != "" && r.Op == "count" && in.Unit != "곳" {
		add("단위", false, "행 수를 셌지만 질문은 「"+in.Unit+"」 단위")
	}
	if len(r.Facts) == 0 {
		add("계산", false, "계산된 숫자가 없음")
	}
	return m
}

// subject 는 질문의 지역과 데이터·계산의 지역이 같은지 본다.
func subject(q string, in Intent, d *Dataset, t *Table, cols []Col, r *Result) (string, bool, string) {
	const name = "대상"
	if reForeignSubject.MatchString(d.Title) && !reAskForeign.MatchString(q) {
		return name, false, "데이터의 행은 「" + reForeignSubject.FindString(d.Title) + "」 도시들. 제주 자체의 값이 아님"
	}
	applied := strings.Join(r.Filters, " ") + " " + r.Plan.Target
	title := d.Title + " " + d.Desc
	// 질문이 콕 집은 지역(제주시·서귀포·읍면동·다른 시도)
	for _, rg := range in.Regions {
		if rg == "제주" {
			continue
		}
		if strings.Contains(applied, rg) {
			continue
		}
		// 데이터 제목이 그 지역 전용인가
		if strings.Contains(d.Title, rg) {
			continue
		}
		// 데이터 안에 그 지역 값이 있는데 거르지 않았거나, 아예 없거나
		found := false
		for _, c := range cols {
			if c.Kind != "cat" {
				continue
			}
			for _, row := range t.Rows {
				if strings.HasPrefix(row[c.I], rg) {
					found = true
					break
				}
			}
		}
		if found && (r.Op == "rank" || r.Op == "compare") && r.Plan.Group >= 0 {
			continue // 지역별 순위·비교 — 그 지역이 순위에 들어 있다
		}
		if found {
			return name, false, "「" + rg + "」만 골라 계산하지 못함"
		}
		return name, false, "데이터에 「" + rg + "」 값이 없음"
	}
	// 질문은 제주 전체를 묻는데 데이터는 한 시·한 곳만 다룬다(「제주 관광객」을 서귀포시 관광지 관람객으로 답하지 않는다)
	onlyJeju := len(in.Regions) == 0 || len(in.Regions) == 1 && in.Regions[0] == "제주"
	if onlyJeju {
		for _, part := range []string{"서귀포시", "제주시"} {
			if strings.Contains(d.Title, part) && !strings.Contains(d.Title, "제주특별자치도") && !strings.Contains(d.Title, "제주도") || strings.Contains(d.Title, "제주특별자치도 "+part) {
				return name, false, "질문은 제주 전체인데 데이터는 「" + part + "」만 다룸"
			}
		}
	}
	// 다른 시도 이름이 제목에 있는데 질문은 제주를 묻는다
	for _, s := range sido {
		if strings.Contains(d.Title, s) && !strings.Contains(q, s) && !strings.Contains(d.Title, "시도") && !strings.Contains(d.Title, "전국") {
			return name, false, "데이터는 「" + s + "」 자료"
		}
	}
	// 「제주시」 질문인데 데이터가 서귀포시 전용(또는 반대)
	if contains(in.Regions, "제주시") && strings.Contains(title, "서귀포") && !strings.Contains(title, "제주시") && !strings.Contains(title, "제주특별자치도") {
		return name, false, "데이터는 서귀포시 자료"
	}
	if (contains(in.Regions, "서귀포시") || contains(in.Regions, "서귀포")) && strings.Contains(d.Title, "제주시") && !strings.Contains(d.Title, "서귀포") {
		return name, false, "데이터는 제주시 자료"
	}
	// 시도별 표인데 제주를 고르지 않고 전국을 더함
	for _, c := range cols {
		if c.Kind != "cat" {
			continue
		}
		hasJeju, hasOther := false, false
		for _, row := range t.Rows {
			v := row[c.I]
			if v == "제주" || v == "제주도" || v == "제주특별자치도" || strings.HasPrefix(v, "제주특별") {
				hasJeju = true
			}
			for _, s := range sido {
				if strings.HasPrefix(v, s) {
					hasOther = true
				}
			}
		}
		if hasJeju && hasOther && !strings.Contains(applied, "제주") && !(r.Plan.Group == c.I && (r.Op == "rank" || r.Op == "compare")) {
			return name, false, "시도별 표에서 제주만 고르지 않음"
		}
	}
	if len(in.Regions) == 0 || contains(in.Regions, "제주") && len(in.Regions) == 1 {
		return name, true, "제주 전체"
	}
	return name, true, "질문한 지역 「" + strings.Join(in.Regions, "·") + "」과 맞음"
}

// extraQualifier 는 측정 열 이름에 질문에 없는 한정어(「경제활동 인구」의 경제활동)가 붙었는지 본다.
// 「인구」를 물었는데 「경제활동 인구」를 세면 숫자는 원본과 같아도 다른 것을 센 답이다.
func extraQualifier(q, lab string) string {
	core := lab
	if i := strings.Index(core, "("); i > 0 {
		// 영문 열은 괄호 속 한글 풀이를 본다
		if regexp.MustCompile(`^[A-Za-z_0-9]+$`).MatchString(strings.TrimSpace(core[:i])) {
			core = strings.TrimSuffix(core[i+1:], ")")
		} else {
			core = core[:i]
		}
	}
	words := regexp.MustCompile(`[가-힣]{2,}`).FindAllString(core, -1)
	if len(words) < 2 {
		return ""
	}
	qq := strings.ReplaceAll(q, " ", "")
	for _, w := range words {
		if qualifierWords[w] {
			if !hitText(w, nil, qq) {
				return w
			}
		}
	}
	return ""
}

var qualifierWords = setOf("경제활동", "비경제활동", "외국인", "내국인", "등록외국인", "청년", "노인", "고령", "여성", "남성", "장애인", "기초생활", "법인", "개인", "외국", "단체", "전출", "전입", "순이동", "사망", "출생", "혼인", "이혼", "실업", "취업", "주간", "야간", "평일", "주말", "누계", "전년", "전월", "증감", "전망")
