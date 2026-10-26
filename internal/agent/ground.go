package agent

// 낱말 대조(grounding) — 질문의 낱말 하나하나가 열 사전의 어디(주제·지표·차원 값·지역·시점)에 닿는지 찾는다.
// 닿지 않는 내용어가 하나라도 남으면 그 데이터로는 답하지 않는다(「해녀」「하와이」「교통사고」).

import (
	"regexp"
	"sort"
	"strings"
	"unicode"
)

const (
	pVal = iota
	pPlus
	pQual
	pMeas
	pCount
	pDimTerm
	pInfo
	pTitle
	pSubj
	pRegion
)

type phrase struct {
	text   []rune
	kind   int
	dim    int
	vals   []string
	idx    int // 지표·세기 번호
	region string
	qual   string
	parts  [][]rune // pPlus
	single bool
}

type occ struct {
	p     *phrase
	spans [][2]int
}

type Ground struct {
	LT      *LT
	Q       []rune
	Occs    []occ
	Miss    []string // 닿지 않은 내용어
	TitleHit []string // 열 사전에는 없지만 데이터 제목에 든 낱말
	Score   int
	Anchor  []string
	Regions []string
	Contain map[int][]string // 값 안에 든 낱말로 거르기
	Cue     string           // 찾은 말을 지운 질문(연산 단서용)
	Covered []bool
}

var (
	reNorm = regexp.MustCompile(`[^\p{Hangul}A-Za-z0-9+ ]+`)
	// 말끝·조사로만 이뤄진 꼬리
	endingRunes = map[rune]bool{}
	ignorable   = setOf(
		"소개", "소개돼", "소개된", "소개되어", "나오", "나온", "나오는", "잰", "재는", "측정한", "들어", "들어있는", "적힌", "올라온", "등록된",
		"몇", "얼마", "얼마나", "얼마야", "얼마고", "얼만", "어디", "어디야", "어디고", "어디예요", "어디에요", "어느", "언제", "언제야", "무슨", "뭐", "뭐야", "뭐고", "뭔가", "뭐였어", "뭐였지", "무엇", "어떤", "어때", "어떻게", "어땠어", "어땠나", "어떤가",
		"가장", "제일", "최고", "최대", "최저", "최소", "많이", "많은", "많았", "많아", "많았던", "많던", "적은", "적게", "적었", "높은", "높았", "낮은", "낮았", "더", "덜", "많", "적", "높", "낮", "큰", "작은",
		"평균", "총", "총합", "전체", "모두", "다", "합쳐서", "합치면", "합해서", "합계", "합", "다합쳐서", "통틀어", "기준", "기준으로", "자료", "데이터", "통계", "정보", "현황", "내용", "기록", "분석", "분석된", "조사", "공개",
		"알려줘", "알려", "알려주세요", "알려줄래", "줘", "주세요", "보여줘", "보여", "말해줘", "궁금", "궁금해", "궁금해요", "해요", "해", "했어", "했나", "나와", "나와요", "있어", "있나", "있는", "있었어", "있니", "있을까", "있지", "됐어", "돼", "되나", "됐나", "되는", "된", "되지",
		"이야", "야", "요", "지금", "현재", "요즘", "최근", "최신", "한", "해", "동안", "1년", "한해", "한", "달", "월", "연도", "년도", "해마다", "매년", "매달", "매월", "그동안", "그간", "사이", "중", "중에", "중에서", "중엔", "별", "별로", "각", "각각", "따로", "만", "번", "부터", "까지", "이후", "이전", "이래",
		"비교", "비교하면", "비교해서", "비교해", "견주면", "대비", "보다", "늘었", "줄었", "늘었어", "줄었어", "늘었나", "줄었나", "늘었는지", "줄었는지", "변했", "변화", "추이", "증가", "감소", "증감", "차이", "흐름", "변했어", "변했나", "바뀌었", "바뀌었어", "달라졌", "어떻게됐",
		"정도", "수", "수치", "값", "숫자", "규모", "우리", "명", "대", "곳", "군데", "개", "건", "톤", "호", "도", "원", "달러", "퍼센트", "프로", "등", "이번", "순위", "위", "1위", "등수", "몇위", "플러스", "마이너스", "양수", "음수", "내면", "내서", "내", "몇이", "나", "혹시", "좀", "대략", "대충", "약", "그리고", "랑", "하고", "또", "고", "및",
		"m", "km", "db", "kg", "ha", "그럼", "그러면", "그", "이", "저", "거", "것", "게", "건가", "인가", "인지", "인데", "때", "적이", "적", "쪽", "얼마치", "데", "개수", "갯수", "미터", "데시벨", "도야", "몇도",
		"말", "초", "중순", "상반기", "하반기", "분기", "연간", "월간", "일간", "월별", "연도별", "해별", "기간", "당시", "그때", "처음", "마지막", "끝", "직전", "이전", "올해", "작년", "재작년", "지난해", "년", "일", "주", "때는", "기준일",
		"제주", "제주도", "도내", "우리 도", "제주특별자치도", "있었나", "봐줘", "찾아줘", "계산해줘", "계산", "내줘", "구해줘", "몇이야", "몇이었어", "몇이에요", "몇인가", "몇개", "몇명", "몇대", "몇곳", "몇톤", "몇건", "몇호",
		"나왔", "됐", "했", "였", "었", "았", "이었", "였어", "이었어", "합쳐", "모아", "모아서", "통째로", "전부", "모든", "다른", "곳이", "곳은", "어딘지", "어딘가", "어딜까", "뭔지", "누구", "몇이나", "얼마큼", "만큼", "만큼이나",
		"포함", "포함해서", "포함한", "포함하면", "떨어졌어", "언제임", "추세", "회복률", "전년", "들어오는", "들어오", "들어온", "있음", "있어", "있나", "줄고", "늘고", "곳만", "개만", "만", "젤", "몇가구", "가구에요", "임", "난", "많이 난", "이상", "세이상", "몇가구에요", "가구에요", "에요", "몇", "줄고있어", "늘고있어", "몇위", "몇위야", "순위", "때", "떨어졌", "떨어졌나", "떨어진", "떨어졌는지", "비교해줘", "비교해봐", "비교해 줘", "감소율", "증가율", "증감률", "변화율", "상승률", "하락률", "상위", "하위", "p", "%p", "포인트", "태우고", "태운", "싣고", "데려온", "이여", "여", "오는", "온", "배나", "배", "늘리려면", "몇 배", "명이여", "다 합치면", "합치면", "비해", "비하면", "비해서", "올랐", "올랐어", "내렸", "내렸어", "높아졌", "높아졌어", "낮아졌", "낮아졌어", "연평균", "월평균", "일평균", "밤", "낮", "환경", "분", "건수", "횟수", "날", "날은", "날이", "날짜", "해", "해는", "해가", "달", "달은", "달이", "때", "때는", "때가", "언제", "언제였어", "일자", "며칠", "몇월", "몇년", "몇년도", "편", "쪽", "어디가", "어디야", "어디예요", "무엇", "얼마나", "얼마였어", "얼마였지", "얼마고", "얼마예요", "얼마에요", "몇이었어", "되나요", "돼요", "인가요", "이에요", "예요", "에요", "있나요", "있어요", "됐어요", "알려줘요", "알려주세요", "궁금해요", "몇개야", "몇곳이야", "온", "해온", "해 온", "측정값", "측정치", "값이", "값은", "수치는", "남녀", "합쳐서", "합쳐", "합친", "합해서", "합하면", "합계", "합계로", "계셔", "계세요", "계신", "빼고", "제외", "제외하고", "치고", "기준으로", "기준", "연말", "말", "말에", "말엔", "가구야", "가구였어", "가구예요", "가구인가요", "명이야", "명이었어", "명이에요", "건이야", "건이었어", "대야", "대였어", "편이야", "회야")
	reNumTok = regexp.MustCompile(`^\d+(\.\d+)?(년|월|일|위|명|대|곳|개|톤|건|호|번째|번|등|시|m|km|kg|%|퍼센트)?$`)
	weakMeas = setOf("몇 명", "관광객", "방문객", "사람", "얼마", "얼마치", "금액", "왔", "찾은", "다녀간", "돈", "비용", "값", "많이", "많이 든", "분", "불었", "자주", "도", "높이", "미터", "점수", "인기", "가격", "집", "곳",
		"짓는", "키우는", "나온", "나왔", "나오는", "나와", "나와요", "나오", "나는", "났", "캤", "수확", "생산", "팔았", "판", "벌었", "등록", "몇 대", "넓은", "먼", "멀리", "km", "킬로", "거리", "가까운", "가깝", "방문", "손님", "많이 검색", "많이 불", "얼마나 나", "톤", "데시벨", "db", "조용", "시끄", "시끄러운", "시끄러웠", "추운", "추웠", "춥", "더운", "더웠", "덥", "뜨거", "결제", "체감")
	sidoFull = map[string][]string{
		"서울": {"서울", "서울시", "서울특별시"}, "부산": {"부산", "부산시", "부산광역시"}, "대구": {"대구", "대구시", "대구광역시"}, "인천": {"인천", "인천시", "인천광역시"},
		"광주": {"광주", "광주시", "광주광역시"}, "대전": {"대전", "대전시", "대전광역시"}, "울산": {"울산", "울산시", "울산광역시"}, "세종": {"세종", "세종시", "세종특별자치시"},
		"경기": {"경기도", "경기 지역", "경기권"}, "강원": {"강원", "강원도", "강원특별자치도"}, "충북": {"충북", "충청북도"}, "충남": {"충남", "충청남도"},
		"전북": {"전북", "전라북도", "전북특별자치도"}, "전남": {"전남", "전라남도"}, "경북": {"경북", "경상북도"}, "경남": {"경남", "경상남도"},
		"제주": {"제주", "제주도", "제주특별자치도", "도내", "우리 도", "제주 지역", "제주지역"}, "제주시": {"제주시"}, "서귀포": {"서귀포", "서귀포시", "서귀포 지역"},
		"전국": {"전국", "우리나라", "한국", "국내", "전국 평균", "대한민국"},
	}
)

func init() {
	for _, r := range "이가은는을를에의도만로으서게한테랑과와하고부터까지보다중별어요야지냐니나까던었았했였됐돼해워져진된죠네군며면인데엔거예요들께씩쯤엔엔간건걸서고된던다냐니죠" {
		endingRunes[r] = true
	}
}

func canonRegion(s string) string {
	s = strings.TrimSpace(s)
	for c, fs := range sidoFull {
		for _, f := range fs {
			if s == f {
				return c
			}
		}
	}
	return s
}

var reMyeot = regexp.MustCompile(`몇(가구|명|건|톤|곳|개|위|배|대|회)`)

// colloq 는 입말을 계산이 읽는 말로 바꾼다
var colloq = [][2]string{
	{"코로나때 얼마나 줄었", "2019년 대비 2021년 얼마나 줄었"}, {"코로나 때 얼마나 줄었", "2019년 대비 2021년 얼마나 줄었"},
	{"코로나때", "2020년부터 2022년까지"}, {"코로나 때", "2020년부터 2022년까지"},
	{"전년대비", "전년 대비"}, {"많은곳", "많은 곳"}, {"많은데", "많은 데"}, {"줄고있", "줄고 있"}, {"늘고있", "늘고 있"},
	{"승객수", "여객 수"}, {"승객", "여객"}, {"내도객", "입도객"},
}

func normQ(s string) string {
	s = strings.ToLower(s)
	s = reNorm.ReplaceAllString(s, " ")
	s = reMyeot.ReplaceAllString(s, "몇 $1")
	for _, r := range colloq {
		s = strings.ReplaceAll(s, r[0], r[1])
	}
	return strings.Join(strings.Fields(s), " ")
}

func runesOf(s string) []rune { return []rune(normQ(s)) }

// valueForms 는 값 하나를 질문에서 부를 수 있는 꼴들로 만든다.
// 「양유(더덕)-경작지」 → 양유·더덕(꼬리 경작지) / 「노지온주(조생)」 → 노지온주·조생 / 「남원읍」 → 남원
func valueForms(v string, d *SDim) (forms []string, qual string) {
	lv := strings.ToLower(strings.TrimSpace(v))
	if d.Exact {
		return []string{lv}, ""
	}
	base := lv
	for k := range d.Qual {
		if strings.HasSuffix(base, "-"+k) {
			base, qual = strings.TrimSuffix(base, "-"+k), k
		}
	}
	add := func(f string) {
		f = strings.TrimSpace(f)
		if f == "" || (len(forms) > 0 && (generic[f] || badForm[f])) {
			return
		}
		for _, x := range forms {
			if x == f {
				return
			}
		}
		forms = append(forms, f)
		if r := []rune(f); len(r) >= 3 && (r[len(r)-1] == '류') {
			forms = append(forms, string(r[:len(r)-1]))
		}
		if d.Region {
			if r := []rune(f); len(r) >= 3 && strings.ContainsRune("읍면동리", r[len(r)-1]) {
				forms = append(forms, string(r[:len(r)-1]))
			}
		}
	}
	add(lv)
	add(base)
	if i := strings.Index(base, "("); i > 0 {
		add(base[:i])
		in := base[i+1:]
		in = strings.TrimSuffix(in, ")")
		if j := strings.Index(in, ")"); j >= 0 {
			in = in[:j]
		}
		for _, p := range regexp.MustCompile(`[,·/]`).Split(in, -1) {
			add(p)
		}
		add(strings.NewReplacer("(", "", ")", "").Replace(base))
	}
	for _, p := range regexp.MustCompile(`[·/]`).Split(base, -1) {
		if p != base {
			add(p)
		}
	}
	if d.Region {
		add(canonRegion(v))
	}
	return
}

var badForm = setOf("기타", "전체", "계", "소계", "합계", "그외", "그 외", "총계", "없음", "-", "기본운영", "해당없음")

// phrases 는 데이터셋 하나에서 질문과 대조할 말 목록을 만든다(한 번 만들면 기억).
func (lt *LT) phrases() []*phrase {
	if lt.phr != nil {
		return lt.phr
	}
	s := lt.S
	out := []*phrase{}
	byText := map[string]*phrase{}
	add := func(text string, p phrase) {
		r := runesOf(text)
		if len(r) == 0 {
			return
		}
		p.text = r
		p.single = len(r) == 1
		key := string(r) + "|" + string(rune('0'+p.kind)) + "|" + string(rune('0'+p.dim))
		if p.kind == pVal {
			if x, ok := byText[key]; ok {
				for _, v := range p.vals {
					if !contains(x.vals, v) {
						x.vals = append(x.vals, v)
					}
				}
				return
			}
		}
		pp := p
		out = append(out, &pp)
		byText[key] = &pp
	}
	for _, t := range append(append([]string{}, s.Terms...), s.Strong...) {
		add(t, phrase{kind: pSubj})
	}
	for _, t := range lt.Titles {
		add(t, phrase{kind: pTitle})
	}
	for i := range lt.Dims {
		d := &lt.Dims[i]
		for _, t := range d.Terms {
			add(t, phrase{kind: pDimTerm, dim: i})
		}
		for _, inf := range d.Info {
			for _, t := range inf.Terms {
				add(t, phrase{kind: pInfo, dim: i, qual: inf.Col})
			}
		}
		for k, syns := range d.Qual {
			for _, t := range syns {
				add(t, phrase{kind: pQual, dim: i, qual: k})
			}
		}
		for v, syns := range d.Syn {
			for _, t := range syns {
				add(t, phrase{kind: pVal, dim: i, vals: []string{v}})
			}
			add(v, phrase{kind: pVal, dim: i, vals: []string{v}})
		}
		if d.NoValues || d.Contains || d.Flag != "" {
			continue
		}
		for _, v := range lt.Vals[i] {
			if v == "" {
				continue
			}
			fs, q := valueForms(v, d)
			for _, f := range fs {
				add(f, phrase{kind: pVal, dim: i, vals: []string{v}, qual: q})
			}
		}
	}
	for i, m := range lt.Meas {
		for _, t := range m.Terms {
			if strings.Contains(t, "+") {
				parts := [][]rune{}
				for _, p := range strings.Split(t, "+") {
					parts = append(parts, runesOf(p))
				}
				out = append(out, &phrase{kind: pPlus, idx: i, parts: parts, text: runesOf(strings.ReplaceAll(t, "+", ""))})
				continue
			}
			add(t, phrase{kind: pMeas, idx: i})
		}
	}
	for i, c := range s.Counts {
		for _, t := range c.Terms {
			add(t, phrase{kind: pCount, idx: i})
		}
	}
	for c, fs := range sidoFull {
		for _, f := range fs {
			add(f, phrase{kind: pRegion, region: c})
		}
	}
	lt.phr = out
	return out
}

type tokSpan struct{ a, b int }

func tokens(q []rune) []tokSpan {
	out := []tokSpan{}
	st := -1
	for i, r := range q {
		if r == ' ' {
			if st >= 0 {
				out = append(out, tokSpan{st, i})
				st = -1
			}
		} else if st < 0 {
			st = i
		}
	}
	if st >= 0 {
		out = append(out, tokSpan{st, len(q)})
	}
	return out
}

func isEnding(r []rune) bool {
	if len(r) == 0 {
		return true
	}
	if len(r) > 5 {
		return false
	}
	for _, x := range r {
		if !endingRunes[x] {
			return false
		}
	}
	return true
}

// reAgeTok 「20대」「30대 남자」 — 나이대 조건. 데이터에 없으면 흘려듣지 않는다.
var reAgeTok = regexp.MustCompile(`^[1-9]0대`)

func ignorableWord(w string) bool {
	if reAgeTok.MatchString(w) {
		return false
	}
	if w == "" || ignorable[w] || reNumTok.MatchString(w) {
		return true
	}
	if isPredicate(w) && !stemIsTerm(w) {
		return true
	}
	if s := stripJosa(w); s != w && (ignorable[s] || reNumTok.MatchString(s)) {
		return true
	}
	// 꼬리(조사·말끝)를 하나씩 떼어 보며 남는 말이 무시해도 되는 말인가
	r := []rune(w)
	for k := len(r) - 1; k >= 1; k-- {
		if isEnding(r[k:]) && (ignorable[string(r[:k])] || reNumTok.MatchString(string(r[:k]))) {
			return true
		}
	}
	return isEnding(r)
}

func findAll(q, p []rune) []int {
	out := []int{}
	if len(p) == 0 || len(p) > len(q) {
		return out
	}
	for i := 0; i+len(p) <= len(q); i++ {
		ok := true
		for j := range p {
			if q[i+j] != p[j] {
				ok = false
				break
			}
		}
		if ok {
			out = append(out, i)
		}
	}
	return out
}

// GroundQ 는 질문(시점 말을 지운 것)을 데이터셋 하나에 대어 본다.
func GroundQ(blanked string, lt *LT) *Ground {
	q := runesOf(blanked)
	g := &Ground{LT: lt, Q: q, Contain: map[int][]string{}}
	toks := tokens(q)
	inParen := make([]bool, len(q)+1)
	depth := 0
	for i, r := range q {
		if r == '(' || r == '（' {
			depth++
		}
		inParen[i] = depth > 0
		if (r == ')' || r == '）') && depth > 0 {
			depth--
		}
	}
	tokStart := map[int]bool{}
	tokOf := make([]int, len(q))
	for ti, t := range toks {
		tokStart[t.a] = true
		for i := t.a; i < t.b; i++ {
			tokOf[i] = ti
		}
	}
	type cand struct {
		p     *phrase
		spans [][2]int
		ln    int
	}
	cands := []cand{}
	for _, p := range lt.phrases() {
		if p.kind == pPlus {
			spans := [][2]int{}
			ok := true
			for _, part := range p.parts {
				pos := findAll(q, part)
				if len(pos) == 0 {
					ok = false
					break
				}
				spans = append(spans, [2]int{pos[0], pos[0] + len(part)})
			}
			if ok {
				cands = append(cands, cand{p, spans, len(p.text)})
			}
			continue
		}
		for _, i := range findAll(q, p.text) {
			e := i + len(p.text)
			if (p.kind == pRegion || (p.kind == pVal && lt.Dims[p.dim].Region)) && !tokStart[i] {
				continue
			}
			if p.single || len(p.text) == 2 && p.kind == pVal && lt.Dims[p.dim].Region {
				// 한 글자 말(귤·차·김)은 낱말 하나 전체(+조사)일 때만
				if !tokStart[i] {
					continue
				}
				t := toks[tokOf[i]]
				if !isEnding(q[e:t.b]) {
					continue
				}
			}
			cands = append(cands, cand{p, [][2]int{{i, e}}, len(p.text)})
		}
	}
	sort.SliceStable(cands, func(a, b int) bool {
		if cands[a].ln != cands[b].ln {
			return cands[a].ln > cands[b].ln
		}
		return cands[a].p.kind < cands[b].p.kind
	})
	cov := make([]bool, len(q))
	for _, c := range cands {
		free := true
		for _, sp := range c.spans {
			for i := sp[0]; i < sp[1]; i++ {
				if cov[i] {
					free = false
				}
			}
		}
		if !free {
			continue
		}
		for _, sp := range c.spans {
			for i := sp[0]; i < sp[1]; i++ {
				cov[i] = true
			}
		}
		g.Occs = append(g.Occs, occ{c.p, c.spans})
	}
	// 값 안에 든 낱말(무장애 코스의 「화장실」「주차장」)
	for i := range lt.Dims {
		if !lt.Dims[i].Contains {
			continue
		}
		for _, t := range toks {
			if cov[t.a] {
				continue
			}
			w := stripJosa(string(q[t.a:t.b]))
			// 데이터 제목에 든 낱말(코스 이름 등)은 행을 고르는 말이 아니라 데이터를 가리키는 말이다
			if len([]rune(w)) < 2 || ignorableWord(w) || inTitle(w, lt) {
				continue
			}
			hit := false
			for _, v := range lt.Vals[i] {
				if strings.Contains(strings.ToLower(v), w) {
					hit = true
					break
				}
			}
			if hit {
				g.Contain[i] = append(g.Contain[i], w)
				for k := t.a; k < t.a+len([]rune(w)); k++ {
					cov[k] = true
				}
			}
		}
	}
	g.Covered = cov
	// 남은 낱말 검사
	for _, t := range toks {
		segs := [][2]int{}
		st := -1
		for i := t.a; i < t.b; i++ {
			if !cov[i] && st < 0 {
				st = i
			}
			if cov[i] && st >= 0 {
				segs = append(segs, [2]int{st, i})
				st = -1
			}
		}
		if st >= 0 {
			segs = append(segs, [2]int{st, t.b})
		}
		for _, sg := range segs {
			w := string(q[sg[0]:sg[1]])
			whole := sg[0] == t.a && sg[1] == t.b
			// 괄호 안 풀이말(「절화(꽃꽂이용 꽃)」)은 앞말을 풀어 쓴 것이라 따로 맞추지 않는다
			if inParen[sg[0]] {
				continue
			}
			// 데이터 제목에 그대로 든 낱말(「구글 리뷰」「안심코드」)은 그 데이터를 가리키는 말이다
			if inTitle(w, lt) {
				g.Score += 2
				g.TitleHit = append(g.TitleHit, stripJosa(w))
				continue
			}
			if whole {
				if !ignorableWord(w) {
					g.Miss = append(g.Miss, w)
				}
				continue
			}
			if sg[1] == t.b && isEnding(q[sg[0]:sg[1]]) {
				continue
			}
			if len([]rune(w)) >= 2 && ignorableWord(w) {
				continue
			}
			g.Miss = append(g.Miss, w)
		}
	}
	// 점수·근거
	cue := append([]rune{}, q...)
	for _, o := range g.Occs {
		k := o.p.kind
		n := 0
		for _, sp := range o.spans {
			n += sp[1] - sp[0]
			if k != pDimTerm {
				for i := sp[0]; i < sp[1]; i++ {
					cue[i] = ' '
				}
			}
		}
		switch k {
		case pRegion:
			g.Regions = append(g.Regions, o.p.region)
			g.Score += n
		case pSubj, pTitle:
			g.Score += n + 3
			g.Anchor = append(g.Anchor, string(o.p.text))
		case pVal:
			g.Score += n + 1
			if !lt.Dims[o.p.dim].Region {
				g.Anchor = append(g.Anchor, string(o.p.text))
			}
		case pMeas, pPlus:
			g.Score += n + 1
			if len(o.p.text) >= 2 && (!weakMeas[string(o.p.text)] || (o.p.kind == pMeas && o.p.idx < len(lt.Meas) && lt.Meas[o.p.idx].Strong)) {
				g.Anchor = append(g.Anchor, string(o.p.text))
			}
		default:
			g.Score += n
		}
	}
	g.Cue = string(cue)
	return g
}

func hasLetter(s string) bool {
	for _, r := range s {
		if unicode.IsLetter(r) {
			return true
		}
	}
	return false
}

// stemIsTerm 은 말끝(-한·-된)을 떼면 데이터 낱말이 되는가(「귀농한」→ 귀농). 그런 말은 흘려듣지 않는다.
func stemIsTerm(w string) bool {
	r := []rune(w)
	for k := len(r) - 1; k >= 2; k-- {
		if termVocab[string(r[:k])] {
			return true
		}
	}
	return false
}

// inTitle 은 낱말(조사를 뗀 것)이 데이터 제목에 그대로 들어 있는가. 두 글자 이상만 본다.
func inTitle(w string, lt *LT) bool {
	if lt == nil || lt.D == nil {
		return false
	}
	x := stripJosa(w)
	if x == "" {
		x = w
	}
	if len([]rune(x)) < 2 {
		return false
	}
	t := strings.ToLower(strings.ReplaceAll(lt.D.Title, " ", ""))
	return strings.Contains(t, strings.ToLower(x))
}
