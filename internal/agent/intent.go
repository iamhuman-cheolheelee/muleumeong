package agent

// 질문 의도 — 「언제(시점)」·「무슨 단위(몇 명·몇 대·몇 곳·얼마)」·「어디(지역)」·「답할 수 없는 질문인가」.
// 계산과 의미 검산이 모두 이 의도를 기준으로 삼는다.

import (
	"regexp"
	"strconv"
	"strings"
)

// 오늘 기준 연도. 「작년」 = ThisYear-1.
var ThisYear = 2026

type Intent struct {
	Year     int    `json:"year"`     // 0 = 지정 없음
	YearText string `json:"yearText"` // 「작년」·「2022년」
	Relative bool   `json:"relative"` // 작년·올해처럼 상대 시점인가
	FromYear int    `json:"fromYear"` // 견줄 해(있으면)
	Month    int    `json:"month"`    // 「2021년 8월」의 8
	Sign     bool   `json:"sign"`     // 플러스·마이너스를 물었나
	Now      bool   `json:"now"`      // 「지금·현재」 — 가장 최근 시점
	Mean     bool   `json:"mean"`     // 「평균」을 콕 집어 물었나
	Unit     string `json:"unit"`     // 명 | 대 | 곳 | 원 | % | 톤 | ""
	Count    bool   `json:"count"`    // 몇 곳·몇 개 — 행을 세는 질문
	Value    bool   `json:"value"`    // 몇·얼마 — 값 하나를 묻는 질문
	Regions  []string `json:"regions"`
	Refuse   string `json:"refuse,omitempty"` // 계산으로 답할 수 없는 질문이면 그 이유
}

var (
	reYearAbs  = regexp.MustCompile(`((?:19|20)\d{2})\s*년`)
	reYearBare = regexp.MustCompile(`(?:^|\D)((?:19|20)\d{2})(?:\D|$)`)
	reYear2    = regexp.MustCompile(`(?:^|[^\d])(\d{2})년`)
	reSince    = regexp.MustCompile(`((?:19|20)\d{2})\s*년\s*(이후|부터|이래|以後)`)
	reMonth    = regexp.MustCompile(`(?:19|20)\d{2}\s*년\s*(\d{1,2})\s*월`)
	reYearsAgo = regexp.MustCompile(`(\d)\s*년\s*전`)
	reWhy      = regexp.MustCompile(`왜|이유|원인|때문|전망|예측|내년|앞으로|될까|오를까|내릴까|좋을까|추천|해야 ?할까|어떻게 해야`)
	reOffice   = regexp.MustCompile(`도지사|지사|시장|교육감|의장|국회의원|장관|대통령|군수|구청장`)
	rePersonal = regexp.MustCompile(`(씨|님|이웃|친구).{0,6}(주소|집|사는 ?곳)|전화번호|휴대폰|주민번호|이름이 뭐|누구 집|개인 정보|개인정보|연락처`)
	reUnitQ    = []struct {
		re   *regexp.Regexp
		unit string
	}{
		{regexp.MustCompile(`몇\s*(명|분|사람)|인원|인구|사람 수`), "명"},
		{regexp.MustCompile(`몇\s*대|대수`), "대"},
		{regexp.MustCompile(`몇\s*(곳|군데|개소|개|업체|점포|가게)|개수|몇 ?개나`), "곳"},
		{regexp.MustCompile(`얼마.*(수출|수입|팔았|벌었)|(수출|수입|팔았|벌었).*얼마|가격|금액|요금|비용|매출|얼마짜리|몇 ?원|비싼|비싸|판매액|얼마치`), "원"},
		{regexp.MustCompile(`몇\s*(퍼센트|%)|비율|률은|율은|비중`), "%"},
		{regexp.MustCompile(`몇\s*톤`), "톤"},
	}
	reValueQ  = regexp.MustCompile(`몇|얼마|규모|수는|수가|총|모두|전체`)
	regionAll = []string{"제주시", "서귀포시", "제주", "서귀포", "서울", "부산", "대구", "인천", "광주", "대전", "울산", "세종", "경기", "강원", "충북", "충남", "전북", "전남", "경북", "경남"}
	// 제주 읍면동 — 질문에 나오면 그 지역으로 걸러야 한다(「비교하면」 같은 말끝과 헷갈리지 않게 이름표로만 찾는다)
	towns = []string{"한림읍", "애월읍", "구좌읍", "조천읍", "한경면", "추자면", "우도면", "일도1동", "일도2동", "이도1동", "이도2동", "삼도1동", "삼도2동",
		"용담1동", "용담2동", "건입동", "화북동", "삼양동", "봉개동", "아라동", "오라동", "노형동", "외도동", "이호동", "도두동", "대정읍", "남원읍", "성산읍",
		"안덕면", "표선면", "송산동", "정방동", "중앙동", "천지동", "효돈동", "영천동", "동홍동", "서홍동", "대륜동", "대천동", "중문동", "예래동",
		"애월", "한림", "구좌", "조천", "대정", "남원", "표선", "안덕", "한경", "노형", "연동"}
)

func ParseIntent(q string) Intent {
	in := Intent{Regions: []string{}}
	if reWhy.MatchString(q) {
		in.Refuse = "원인·전망·추천은 공개 통계를 계산해 답할 수 없어요. 「얼마나」·「몇」처럼 숫자로 물어 주세요."
	}
	if rePersonal.MatchString(q) {
		in.Refuse = "개인정보는 다루지 않아요."
		if reOffice.MatchString(q) {
			in.Refuse = "공직자 이름 같은 사실 정보는 통계 데이터에 들어 있지 않아 답할 수 없어요."
		}
	}
	switch {
	case strings.Contains(q, "재작년"):
		in.Year, in.YearText, in.Relative = ThisYear-2, "재작년", true
	case strings.Contains(q, "작년") || strings.Contains(q, "지난해"):
		in.Year, in.YearText, in.Relative = ThisYear-1, "작년", true
	case strings.Contains(q, "올해") || strings.Contains(q, "금년"):
		in.Year, in.YearText, in.Relative = ThisYear, "올해", true
	}
	if m := reYearsAgo.FindStringSubmatch(q); m != nil && in.Year == 0 {
		n, _ := strconv.Atoi(m[1])
		in.Year, in.YearText, in.Relative = ThisYear-n, m[0], true
	}
	if ms := reYearAbs.FindAllStringSubmatch(q, -1); len(ms) > 0 {
		// 해가 둘 이상이면 마지막 해가 답할 해, 첫 해가 견줄 해(「2016년보다 … 2024년엔」)
		m := ms[len(ms)-1]
		in.Year, _ = strconv.Atoi(m[1])
		in.YearText, in.Relative = m[1]+"년", false
		if len(ms) > 1 {
			in.FromYear, _ = strconv.Atoi(ms[0][1])
		}
	} else if m := reYear2.FindStringSubmatch(q); m != nil && in.Year == 0 {
		y, _ := strconv.Atoi(m[1])
		in.Year, in.YearText = 2000+y, m[1]+"년"
	} else if m := reYearBare.FindStringSubmatch(q); m != nil && in.Year == 0 {
		in.Year, _ = strconv.Atoi(m[1])
		in.YearText = m[1] + "년"
	}
	// 「2000년 이후 제일 더웠던 날」의 2000년은 답할 해가 아니라 범위의 시작이다
	if m := reSince.FindStringSubmatch(q); m != nil && strings.HasPrefix(in.YearText, m[1]) {
		in.FromYear, _ = strconv.Atoi(m[1])
		in.Year, in.YearText = 0, ""
	}
	if m := reMonth.FindStringSubmatch(q); m != nil && in.Year > 0 {
		in.Month, _ = strconv.Atoi(m[1])
	}
	if m := regexp.MustCompile(`(작년|지난해|올해|재작년)\s*(\d{1,2})\s*월`).FindStringSubmatch(q); m != nil && in.Month == 0 {
		in.Month, _ = strconv.Atoi(m[2])
	}
	if strings.Contains(q, "이번 달") || strings.Contains(q, "이번달") {
		in.Year, in.Month, in.YearText, in.Relative = ThisYear, 10, "이번 달", true
	}
	in.Sign = regexp.MustCompile(`플러스|마이너스|양수|음수`).MatchString(q)
	in.Now = regexp.MustCompile(`지금|현재|요즘 몇|최신`).MatchString(q)
	for _, u := range reUnitQ {
		if u.re.MatchString(q) {
			in.Unit = u.unit
			break
		}
	}
	in.Count = in.Unit == "곳"
	in.Mean = strings.Contains(q, "평균")
	in.Value = reValueQ.MatchString(q)
	rest := q
	for _, r := range regionAll {
		if r == "경기" && !regexp.MustCompile(`경기도|경기 ?지역|경기권`).MatchString(rest) {
			continue // 「체감경기」의 경기는 지역이 아니다
		}
		if strings.Contains(rest, r) {
			in.Regions = append(in.Regions, r)
			rest = strings.ReplaceAll(rest, r, " ")
		}
	}
	for _, w := range towns {
		if strings.Contains(q, w) && !contains(in.Regions, w) {
			in.Regions = append(in.Regions, w)
		}
	}
	return in
}

// unitFits 는 질문이 묻는 단위와 측정 열이 맞는가.
func unitFits(unit string, label string) bool {
	l := label
	switch unit {
	case "":
		return true
	case "명":
		return regexp.MustCompile(`명|인원|객|인구|자|수|생|민|가구|세대`).MatchString(l) && !regexp.MustCompile(`금액|원\)|천원|백만원|률|율|%`).MatchString(l)
	case "대":
		return regexp.MustCompile(`대|차|등록|수`).MatchString(l)
	case "곳":
		return regexp.MustCompile(`수|개소|곳|개|업체|점포`).MatchString(l) && !regexp.MustCompile(`인원|명\)|금액|률|율`).MatchString(l)
	case "원":
		return regexp.MustCompile(`원|금액|액|가격|요금|비용|매출|단가|소득|임금|지가|값`).MatchString(l)
	case "%":
		return regexp.MustCompile(`률|율|%|비율|비중|지수`).MatchString(l)
	case "톤":
		return regexp.MustCompile(`톤|량|kg|생산`).MatchString(l)
	}
	return true
}
