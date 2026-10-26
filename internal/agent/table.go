package agent

// 표 읽기 — 제주데이터허브 파일은 UTF-8·CP949 CSV 와 「.csv 이름의 엑셀」이 섞여 있다.
// 확장자를 믿지 않고 첫 바이트로 형식을 가린다.

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"math"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/xuri/excelize/v2"
	"golang.org/x/text/encoding/korean"
)

type Table struct {
	Header []string   `json:"header"`
	Rows   [][]string `json:"-"`
	Format string     `json:"format"` // "CSV(UTF-8)" | "CSV(CP949)" | "엑셀(xlsx)"
	Sheet  string     `json:"sheet,omitempty"`
	Notes  []string   `json:"notes"`
}

func ParseTable(b []byte, name string) (*Table, error) {
	t := &Table{Notes: []string{}}
	if bytes.HasPrefix(b, []byte("PK\x03\x04")) {
		f, err := excelize.OpenReader(bytes.NewReader(b))
		if err != nil {
			return nil, err
		}
		defer f.Close()
		sh := f.GetSheetList()
		if len(sh) == 0 {
			return nil, fmt.Errorf("빈 엑셀")
		}
		rows, err := f.GetRows(sh[0])
		if err != nil {
			return nil, err
		}
		t.Format, t.Sheet = "엑셀(xlsx)", sh[0]
		if strings.HasSuffix(strings.ToLower(name), ".csv") {
			t.Notes = append(t.Notes, "파일 이름은 .csv 인데 실제 내용은 엑셀(xlsx)입니다. 형식을 바이트로 확인해 엑셀로 읽었습니다.")
		}
		if len(sh) > 1 {
			t.Notes = append(t.Notes, fmt.Sprintf("시트가 %d개입니다. 첫 시트 「%s」만 썼습니다.", len(sh), sh[0]))
		}
		return t.fill(rows), nil
	}
	if bytes.HasPrefix(b, []byte("\xef\xbb\xbf")) {
		b = b[3:]
	}
	t.Format = "CSV(UTF-8)"
	if !utf8.Valid(b) {
		d, err := korean.EUCKR.NewDecoder().Bytes(b)
		if err != nil {
			return nil, err
		}
		b, t.Format = d, "CSV(CP949)"
	}
	r := csv.NewReader(bytes.NewReader(b))
	r.LazyQuotes, r.FieldsPerRecord = true, -1
	rows, err := r.ReadAll()
	if err != nil {
		return nil, err
	}
	return t.fill(rows), nil
}

func (t *Table) fill(rows [][]string) *Table {
	for len(rows) > 0 && strings.TrimSpace(strings.Join(rows[0], "")) == "" {
		rows = rows[1:]
	}
	if len(rows) == 0 {
		return t
	}
	for _, h := range rows[0] {
		t.Header = append(t.Header, strings.TrimSpace(h))
	}
	for _, r := range rows[1:] {
		if strings.TrimSpace(strings.Join(r, "")) == "" {
			continue
		}
		row := make([]string, len(t.Header))
		for i := range row {
			if i < len(r) {
				row[i] = strings.TrimSpace(r[i])
			}
		}
		t.Rows = append(t.Rows, row)
	}
	return t
}

// ── 열 성격 판별 ────────────────────────────────────────────

type Col struct {
	I        int      `json:"i"`
	Name     string   `json:"name"`
	Label    string   `json:"label"` // 열 이름 사전을 거친 한국어 이름
	Kind     string   `json:"kind"`  // time | month | num | cat | id | text
	Role     string   `json:"role"`  // sum(합산 가능 측정값) | avg(평균형 측정값) | stock(시점 잔량) | dim(차원) | time(시간) | id(식별자) | text
	RoleWhy  string   `json:"roleWhy"`
	Distinct int      `json:"distinct"`
	Missing  int      `json:"missing"`
	Samples  []string `json:"samples"`
	Min      float64  `json:"min"`
	Max      float64  `json:"max"`
}

var (
	reTimeName = regexp.MustCompile(`(?i)(연도|년도|연월|기준.?연|year|date|일시|일자|^연$|^년$|시점)`)
	reMonName  = regexp.MustCompile(`^(월|month)$`)
	reIDName   = regexp.MustCompile(`(?i)^(id|no|번호|순번|연번|seq)$`)
	reY        = regexp.MustCompile(`^(19|20)\d{2}$`)
	reYM       = regexp.MustCompile(`^((?:19|20)\d{2})[.\-/ ]*\s*(\d{1,2})\.?$`)
	reYMD      = regexp.MustCompile(`^((?:19|20)\d{2})[.\-/]?(\d{2})[.\-/]?(\d{2})(?:[ T].*)?$`)
	reTotal    = regexp.MustCompile(`^(계|합계|총계|소계|전국|전체|총합|total|Total|TOTAL)$`)
)

func Num(s string) (float64, bool) {
	s = strings.ReplaceAll(strings.TrimSpace(s), ",", "")
	if s == "" || s == "-" {
		return 0, false
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil || math.IsNaN(v) || math.IsInf(v, 0) {
		return 0, false
	}
	return v, true
}

// ParseTime 은 연도·연월·연월일 문자열을 (연, 월) 로 바꾼다. 월이 없으면 0.
func ParseTime(s string) (int, int, bool) {
	s = strings.TrimSpace(s)
	if reY.MatchString(s) {
		y, _ := strconv.Atoi(s)
		return y, 0, true
	}
	if m := reYMD.FindStringSubmatch(s); m != nil && len(s) >= 8 {
		y, _ := strconv.Atoi(m[1])
		mo, _ := strconv.Atoi(m[2])
		if mo >= 1 && mo <= 12 {
			return y, mo, true
		}
	}
	if m := reYM.FindStringSubmatch(s); m != nil {
		y, _ := strconv.Atoi(m[1])
		mo, _ := strconv.Atoi(m[2])
		if mo >= 1 && mo <= 12 {
			return y, mo, true
		}
	}
	return 0, 0, false
}

func Profile(t *Table, dict map[string]string) []Col {
	cols := make([]Col, len(t.Header))
	for i, h := range t.Header {
		c := Col{I: i, Name: h, Label: h, Samples: []string{}, Min: math.Inf(1), Max: math.Inf(-1)}
		if l, ok := dict[h]; ok {
			c.Label = l
		} else if ko := enLabel(h); ko != "" {
			c.Label = h + "(" + ko + ")"
		}
		seen := map[string]bool{}
		nNum, nTime, nVal, nMon := 0, 0, 0, 0
		for _, r := range t.Rows {
			v := r[i]
			if v == "" {
				c.Missing++
				continue
			}
			nVal++
			if !seen[v] {
				seen[v] = true
				if len(c.Samples) < 4 {
					c.Samples = append(c.Samples, v)
				}
			}
			if f, ok := Num(v); ok {
				nNum++
				c.Min, c.Max = math.Min(c.Min, f), math.Max(c.Max, f)
				if f >= 1 && f <= 12 && f == math.Trunc(f) {
					nMon++
				}
			}
			if _, _, ok := ParseTime(v); ok {
				nTime++
			}
		}
		c.Distinct = len(seen)
		switch {
		case nVal == 0:
			c.Kind = "text"
		case reIDName.MatchString(h) && nNum == nVal:
			c.Kind = "id"
		case reTimeName.MatchString(h) && nTime >= nVal*9/10:
			c.Kind = "time"
		case reMonName.MatchString(h) && nMon == nVal:
			c.Kind = "month"
		case nNum >= nVal*9/10:
			c.Kind = "num"
		case nTime >= nVal*9/10 && c.Distinct > 1:
			c.Kind = "time"
		case c.Distinct <= 60 || c.Distinct*3 <= nVal:
			c.Kind = "cat"
		default:
			c.Kind = "text"
		}
		if math.IsInf(c.Min, 0) {
			c.Min, c.Max = 0, 0
		}
		c.Role, c.RoleWhy = role(&c, nVal)
		cols[i] = c
	}
	return cols
}

// TotalRows 는 범주 열에 「계·합계·전국」 같은 합계 행이 섞였는지 본다. 더하면 두 번 센다.
func TotalRows(t *Table, cols []Col) map[int]bool {
	out := map[int]bool{}
	for _, c := range cols {
		if c.Kind != "cat" {
			continue
		}
		for ri, r := range t.Rows {
			if reTotal.MatchString(r[c.I]) {
				out[ri] = true
			}
		}
	}
	return out
}

func sortedKeys[V any](m map[string]V) []string {
	k := make([]string, 0, len(m))
	for x := range m {
		k = append(k, x)
	}
	sort.Strings(k)
	return k
}

// ── 열 역할 ────────────────────────────────────────────────
// 숫자 열이라고 다 더할 수 있는 것은 아니다. 열 이름과 값 모양으로 역할을 정하고, 계산은 역할이 허락하는 연산만 쓴다.
//   sum   건수·인원·금액처럼 더해도 뜻이 맞는 값
//   avg   비율·지수·평균·기온·가격처럼 더하면 뜻이 깨지는 값 → 평균만
//   stock 인구·등록 대수·누계처럼 쌓여 있는 값 → 기간을 더하지 않고 마지막 시점만
//   id    번호·코드·좌표 → 측정값으로 쓰지 않는다
//   time  연·월·일·시각 → 측정값으로 쓰지 않는다
var (
	reRoleID    = regexp.MustCompile(`(?i)(^id$|_id$|^no\.?$|번호|코드|code|순번|연번|일련|seq|우편|전화|좌표|위도|경도|^lat|^lon|^lng|^x$|^y$|x좌표|y좌표|관리번호|key$)`)
	reRoleTime  = regexp.MustCompile(`(?i)(연도|년도|연월|^연$|^년$|^월$|^일$|일자|일시|날짜|시각|시간대|^시간$|^시$|^분$|요일|주차|year|month|date|day|hour|time|기준일|기준년)`)
	reRoleAvg   = regexp.MustCompile(`(?i)(률|율|비율|비중|지수|평균|bsi|기온|온도|수온|수위|수심|염분|기압|일조|운량|ph|temp|tem_|mean|avg|humid|level|rate|ratio|score|습도|단가|가격|요금|%|농도|수위|소음|점수|만족도|1인당|당\s|밀도|속도|강수|풍속|지가|금리|배율|증감|대비|순위|등급)`)
	reRoleStock = regexp.MustCompile(`(인구|등록|보유|누계|누적|재고|세대수|대수|현재|잔액|면적|시설수|업소수|사업체수|종사자수|재적|정원|병상)`)
)

func role(c *Col, nVal int) (string, string) {
	n := c.Label + " " + c.Name
	switch c.Kind {
	case "time", "month":
		return "time", "시간 열"
	case "id":
		return "id", "번호 열"
	case "cat":
		return "dim", "범주(차원) 열"
	case "text":
		return "text", "글자 열"
	}
	// 여기부터 숫자 열
	if reRoleID.MatchString(c.Name) || reRoleID.MatchString(c.Label) {
		return "id", "이름이 번호·코드·좌표"
	}
	if reRoleTime.MatchString(c.Name) || reRoleTime.MatchString(c.Label) {
		return "time", "이름이 시간·날짜"
	}
	// 값이 모두 다르고 1씩 늘어나는 정수면 순번
	if c.Distinct == nVal && nVal >= 20 && c.Min >= 0 && c.Max-c.Min+1 == float64(nVal) {
		return "id", "값이 1씩 늘어나는 순번"
	}
	if c.Min >= 19000101 && c.Max <= 20991231 {
		return "time", "값이 날짜 모양(YYYYMMDD)"
	}
	if reRoleAvg.MatchString(n) {
		return "avg", "비율·평균형이라 더하지 않고 평균을 냄"
	}
	if reRoleStock.MatchString(n) {
		return "stock", "쌓여 있는 값이라 기간을 더하지 않고 마지막 시점을 씀"
	}
	return "sum", "합산 가능한 값"
}

// enLabel 은 영문 열 이름(visit_count, Tem_mean)을 낱말 단위로 한국어 뜻을 붙인다. 모르는 낱말이 섞이면 붙이지 않는다.
var enWords = map[string]string{
	"visit": "방문", "visitor": "방문객", "count": "수", "cnt": "수", "num": "수", "number": "수", "total": "합계", "sum": "합계",
	"tem": "기온", "temp": "기온", "temperature": "기온", "mean": "평균", "avg": "평균", "average": "평균", "max": "최고", "min": "최저",
	"humid": "습도", "humidity": "습도", "rain": "강수량", "rainfall": "강수량", "precip": "강수량", "wind": "풍속", "speed": "속도",
	"price": "가격", "amount": "금액", "amt": "금액", "sales": "매출", "pop": "인구", "population": "인구", "rate": "비율", "ratio": "비율",
	"score": "점수", "rank": "순위", "level": "수위", "water": "물", "year": "연도", "month": "월", "date": "날짜", "day": "일", "time": "시각",
	"hour": "시", "type": "종류", "name": "이름", "category": "업종", "cat": "업종", "sector": "업종", "area": "지역", "region": "지역",
	"store": "가게", "shop": "가게", "user": "이용자", "users": "이용자", "people": "인원", "person": "인원", "car": "차량", "cars": "차량",
	"keyword": "키워드", "search": "검색", "age": "연령", "sex": "성별", "gender": "성별", "male": "남자", "female": "여자", "id": "번호",
	"lat": "위도", "lon": "경도", "lng": "경도", "x": "x좌표", "y": "y좌표", "code": "코드", "addr": "주소", "address": "주소", "tel": "전화",
	"dt": "날짜", "ym": "연월", "yyyymm": "연월", "yyyymmdd": "날짜", "cnts": "수", "qty": "수량", "volume": "양", "weight": "무게",
	"daily": "일별", "monthly": "월별", "sido": "시도", "sigungu": "시군구", "emd": "읍면동", "upjong": "업종", "gu": "구",
}

func enLabel(h string) string {
	parts := regexp.MustCompile(`[A-Za-z]+`).FindAllString(h, -1)
	if len(parts) == 0 || regexp.MustCompile(`\p{Hangul}`).MatchString(h) {
		return ""
	}
	out := []string{}
	for _, p := range parts {
		k, ok := enWords[strings.ToLower(p)]
		if !ok {
			return ""
		}
		out = append(out, k)
	}
	// visit_count → 방문 수, Tem_mean → 평균 기온(형용 낱말을 앞으로)
	if len(out) == 2 && (out[1] == "평균" || out[1] == "최고" || out[1] == "최저") {
		out[0], out[1] = out[1], out[0]
	}
	return strings.Join(out, " ")
}
