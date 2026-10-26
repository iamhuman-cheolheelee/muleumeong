package agent

// 열 사전 — 데이터셋마다 「무엇이 차원이고 무엇이 지표인가」를 사람이 적어 둔 사전(data/schema.json).
// 넓은 표(관광지별 열·지역별 열·시간대별 열)는 여기서 긴 표(차원 값 + 지표 하나)로 펼친다.
// 계산은 언제나 이 긴 표 위에서만 한다.

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

//go:embed data/schema.json
var schemaJSON []byte

type SDim struct {
	Col      string              `json:"col"`
	Label    string              `json:"label"`
	Region   bool                `json:"region,omitempty"`   // 값이 지역(시도·읍면동)
	Entity   bool                `json:"entity,omitempty"`   // 행의 이름(학교·지점·상품)
	Attr     bool                `json:"attr,omitempty"`     // 행의 속성(주용도) — 묶지 않고 최댓값 행에서 읽어 준다
	Terms    []string            `json:"terms,omitempty"`    // 차원 자체를 가리키는 말(동네·품종·나라)
	Syn      map[string][]string `json:"syn,omitempty"`      // 값 → 다른 말
	Totals   []string            `json:"totals,omitempty"`   // 합계 행 값(계·전국·전체)
	Exclude  []string            `json:"exclude,omitempty"`  // 묻지 않으면 빼는 소계 값(약용작물 등)
	Default  string              `json:"default,omitempty"`  // 묻지 않으면 쓰는 값
	Must     bool                `json:"must,omitempty"`     // 꼭 골라야 하는 차원(서로 더할 수 없는 값)
	Flag     string              `json:"flag,omitempty"`     // 「있음」을 뜻하는 값(O·●)
	Contains bool                `json:"contains,omitempty"` // 값이 긴 글 — 질문 낱말이 값 안에 있으면 고른다
	NoValues bool                `json:"noValues,omitempty"` // 값을 질문 낱말로 쓰지 않는다(이름이 수천 개)
	Qual     map[string][]string `json:"qual,omitempty"`     // 값 꼬리(-경작지) → 다른 말(밭)
	Info     []SInfo             `json:"info,omitempty"`     // 이 이름 행에서 읽어 줄 글 열(주소·뜻)
	Exact    bool                `json:"exact,omitempty"`
	NameFmt  bool                `json:"nameFmt,omitempty"`
}

type SInfo struct {
	Col   string   `json:"col"`
	Label string   `json:"label"`
	Terms []string `json:"terms"`
}

type SMeas struct {
	Col    string   `json:"col"`
	SumOf  []string `json:"sumOf,omitempty"`
	Label  string   `json:"label"`
	Unit   string   `json:"unit"`
	Agg    string   `json:"agg"`           // sum | avg | stock | cum
	Row    bool     `json:"row,omitempty"` // 행 하나하나의 값(가격·기온) — 최댓값은 행에서 찾는다
	Terms  []string `json:"terms"`
	Cum    string   `json:"cum,omitempty"` // 같은 지표의 당해 누계 열
	Def    bool     `json:"def,omitempty"`
	Need   []string `json:"need,omitempty"` // 이 지표를 쓰려면 꼭 정해야 하는 차원
	Dec    int      `json:"dec,omitempty"`
	Peak   string   `json:"peak,omitempty"`
	Strong bool     `json:"strong,omitempty"`
}

type SCount struct {
	Label    string   `json:"label"`
	Unit     string   `json:"unit"`
	Distinct string   `json:"distinct,omitempty"` // 비면 행 수
	Terms    []string `json:"terms"`
	Def      bool     `json:"def,omitempty"`
}

type SMeltCol struct {
	Col  string            `json:"col"`
	Dims map[string]string `json:"dims"`
}

type SMelt struct {
	Measure SMeas      `json:"measure"`
	Dims    []SDim     `json:"dims"`
	Cols    []SMeltCol `json:"cols"`
}

type STime struct {
	Col  string `json:"col"`
	MCol string `json:"mcol,omitempty"`
}

type Schema struct {
	Key         string   `json:"key"`
	IDs         []int    `json:"ids,omitempty"`
	From        int      `json:"from,omitempty"`
	To          int      `json:"to,omitempty"`
	Subject     string   `json:"subject"`
	Scope       string   `json:"scope"` // 제주 | 서귀포 | 전국 | 시도
	Terms       []string `json:"terms"`
	Strong      []string `json:"strong,omitempty"` // 이 말이 있으면 이 데이터라고 볼 만한 말
	Primary     bool     `json:"primary,omitempty"` // 같은 주제 자료가 똑같이 맞을 때 먼저 쓰는 기본 자료
	TitleTerms  bool     `json:"titleTerms,omitempty"`
	Header      []string `json:"header,omitempty"`
	HeaderRow   int      `json:"headerRow,omitempty"`
	NoHeader    bool     `json:"noHeader,omitempty"`
	Time        *STime   `json:"time,omitempty"`
	TitlePeriod bool     `json:"titlePeriod,omitempty"`
	Year        int      `json:"year,omitempty"`
	Dims        []SDim   `json:"dims"`
	Meas        []SMeas  `json:"meas"`
	Counts      []SCount `json:"counts,omitempty"`
	Melt        *SMelt   `json:"melt,omitempty"`
	Group       string   `json:"group,omitempty"`
	Notes       []string `json:"notes,omitempty"`
}

var (
	termVocab = map[string]bool{}
	schemas   []*Schema
	schemaBy  = map[int]*Schema{}
	ltMu      sync.Mutex
	ltCache   = map[int]*LT{}
)

func loadSchemas() error {
	if err := json.Unmarshal(schemaJSON, &schemas); err != nil {
		return fmt.Errorf("schema.json: %w", err)
	}
	for _, s := range schemas {
		addV := func(ts []string) {
			for _, t := range ts {
				if len([]rune(t)) >= 2 && !strings.ContainsAny(t, " +") {
					termVocab[t] = true
				}
			}
		}
		addV(s.Terms)
		addV(s.Strong)
		ms := append([]SMeas{}, s.Meas...)
		if s.Melt != nil {
			ms = append(ms, s.Melt.Measure)
		}
		for _, m := range ms {
			addV(m.Terms)
		}
		for _, id := range s.IDs {
			schemaBy[id] = s
		}
		if s.From > 0 {
			for id := s.From; id <= s.To; id++ {
				if _, ok := schemaBy[id]; !ok {
					schemaBy[id] = s
				}
			}
		}
	}
	return nil
}

// SchemaIDs 는 열 사전이 있는(= 계산할 수 있는) 데이터셋 id 들이다.
func SchemaIDs() []int {
	out := []int{}
	for id := range schemaBy {
		if d := Get(id); d != nil && (hasLocal(id) || d.Open()) {
			out = append(out, id)
		}
	}
	return out
}

// ── 긴 표 ─────────────────────────────────────────────

type LRow struct {
	D []string
	T int64 // 시점 시작(yyyymmddhh), 0 = 시점 없음
	E int64 // 시점 끝
	G int   // 1 연 · 2 월 · 3 일 · 4 시
	V []float64
	R int // 원본 행 번호
}

type LT struct {
	S      *Schema
	D      *Dataset
	T      *Table
	Dims   []SDim
	Meas   []SMeas
	Rows   []LRow
	Vals   [][]string // 차원별 값(나온 순서)
	TMin   int64
	TMax   int64
	Gran   int
	PS, PE int64 // 제목에 적힌 기간
	Src    string
	Titles []string // 제목에서 뽑은 이름(코스 이름 등)
	Header []string
	Raw    [][]string
	phr    []*phrase
}

func hasLocal(id int) bool {
	if hasCopy(id) {
		return true
	}
	_, err := os.Stat(fmt.Sprintf("%s/%d.bin", FullDir, id))
	return err == nil
}

// LoadLT 는 데이터셋 하나를 열 사전대로 읽어 긴 표로 펼친다(한 번 읽으면 기억해 둔다).
func LoadLT(id int) (*LT, error) {
	ltMu.Lock()
	if lt, ok := ltCache[id]; ok {
		ltMu.Unlock()
		return lt, nil
	}
	ltMu.Unlock()
	s := schemaBy[id]
	d := Get(id)
	if s == nil || d == nil {
		return nil, fmt.Errorf("열 사전 없음")
	}
	b, src, err := fetch(d)
	if err != nil || strings.HasPrefix(src, "미리보기") {
		return nil, fmt.Errorf("원본을 받지 못함")
	}
	name := "data.csv"
	if d.File != nil {
		name = d.File.FileName
		if isPub(d.ID) && !d.Open() {
			name = "data.csv"
		}
	}
	tb, err := ParseTable(b, name)
	if err != nil || len(tb.Header) == 0 {
		return nil, fmt.Errorf("표로 읽을 수 없음")
	}
	lt := &LT{S: s, D: d, T: tb, Src: src}
	lt.build()
	ltMu.Lock()
	ltCache[id] = lt
	ltMu.Unlock()
	return lt, nil
}

func normH(s string) string {
	return strings.Join(strings.Fields(strings.TrimSpace(s)), "")
}

func (lt *LT) build() {
	s, tb := lt.S, lt.T
	header := append([]string{}, tb.Header...)
	rows := tb.Rows
	noHead := s.NoHeader
	if !noHead && len(s.Header) > 0 && s.HeaderRow == 0 && len(header) > 0 {
		// 머리줄이 빠진 파일(1232) — 첫 줄 첫 칸이 숫자·날짜면 자료 행이다
		if _, ok := Num(header[0]); ok {
			noHead = true
		} else if _, _, g := rowTime(header[0]); g > 0 {
			noHead = true
		}
	}
	if noHead {
		rows = append([][]string{header}, rows...)
		header = s.Header
	} else if s.HeaderRow > 0 && len(rows) >= s.HeaderRow {
		header = rows[s.HeaderRow-1]
		rows = rows[s.HeaderRow:]
	}
	if len(s.Header) > 0 && !noHead {
		for i := range header {
			if i < len(s.Header) && s.Header[i] != "" {
				header[i] = s.Header[i]
			}
		}
	}
	lt.Header, lt.Raw = header, rows
	col := map[string]int{}
	for i, h := range header {
		col[normH(h)] = i
	}
	ci := func(name string) int {
		if i, ok := col[normH(name)]; ok {
			return i
		}
		return -1
	}
	get := func(r []string, i int) string {
		if i < 0 || i >= len(r) {
			return ""
		}
		return strings.TrimSpace(r[i])
	}
	lt.Dims = append([]SDim{}, s.Dims...)
	if s.Melt != nil {
		lt.Dims = append(lt.Dims, s.Melt.Dims...)
		lt.Meas = []SMeas{s.Melt.Measure}
	} else {
		lt.Meas = append([]SMeas{}, s.Meas...)
		for _, m := range s.Meas {
			if m.Cum != "" {
				lt.Meas = append(lt.Meas, SMeas{Col: m.Cum, Label: m.Label, Unit: m.Unit, Agg: "cum", Dec: m.Dec})
			}
		}
	}
	dIdx := make([]int, len(s.Dims))
	for i, dm := range s.Dims {
		dIdx[i] = ci(dm.Col)
	}
	mIdx := make([][]int, len(lt.Meas))
	for i, m := range lt.Meas {
		if s.Melt != nil {
			continue
		}
		cs := m.SumOf
		if len(cs) == 0 {
			cs = []string{m.Col}
		}
		for _, c := range cs {
			mIdx[i] = append(mIdx[i], ci(c))
		}
	}
	tIdx, tmIdx := -1, -1
	if s.Time != nil {
		tIdx = ci(s.Time.Col)
		if s.Time.MCol != "" {
			tmIdx = ci(s.Time.MCol)
		}
	}
	if s.TitlePeriod {
		lt.PS, lt.PE = titlePeriod(lt.D.Title)
	}
	if s.TitleTerms {
		lt.Titles = titleNames(lt.D.Title)
	}
	type mc struct {
		i    int
		dims map[string]string
	}
	melts := []mc{}
	if s.Melt != nil {
		for _, c := range s.Melt.Cols {
			if i := ci(c.Col); i >= 0 {
				melts = append(melts, mc{i, c.Dims})
			}
		}
	}
	nd := len(lt.Dims)
	seen := make([]map[string]bool, nd)
	lt.Vals = make([][]string, nd)
	for i := range seen {
		seen[i] = map[string]bool{}
	}
	addVal := func(i int, v string) {
		if !seen[i][v] {
			seen[i][v] = true
			lt.Vals[i] = append(lt.Vals[i], v)
		}
	}
	for ri, r := range rows {
		base := make([]string, nd)
		for i := range s.Dims {
			base[i] = get(r, dIdx[i])
		}
		var T, E int64
		g := 0
		if tIdx >= 0 {
			ts := get(r, tIdx)
			if tmIdx >= 0 {
				ts += "-" + get(r, tmIdx)
			}
			T, E, g = rowTime(ts)
			if g == 0 && !s.meltTime() {
				continue // 시점을 읽을 수 없는 행(주석·빈 줄)
			}
		} else if lt.PS > 0 {
			T, E, g = lt.PS, lt.PE, 3
		}
		if s.Melt != nil {
			for _, m := range melts {
				v, ok := Num(get(r, m.i))
				if !ok {
					continue
				}
				mT, mE, mg := T, E, g
				if x, has := m.dims["@t"]; has {
					// 열이 시점을 뜻한다(1월 열·2019년 열) — 행의 연도에 이어 붙이거나 그대로 읽는다
					ts := x
					if tIdx >= 0 {
						ts = get(r, tIdx) + "-" + x
					}
					if mT, mE, mg = rowTime(ts); mg == 0 {
						continue
					}
				}
				dv := append([]string{}, base...)
				for j, md := range s.Melt.Dims {
					x, has := m.dims[md.Label]
					if !has {
						x = md.Default
					}
					dv[len(s.Dims)+j] = x
				}
				lt.Rows = append(lt.Rows, LRow{D: dv, T: mT, E: mE, G: mg, V: []float64{v}, R: ri})
			}
		} else {
			vs := make([]float64, len(lt.Meas))
			for i := range lt.Meas {
				sum, any := 0.0, false
				for _, c := range mIdx[i] {
					if v, ok := Num(get(r, c)); ok {
						sum += v
						any = true
					}
				}
				if any {
					vs[i] = sum
				} else {
					vs[i] = math.NaN()
				}
			}
			lt.Rows = append(lt.Rows, LRow{D: base, T: T, E: E, G: g, V: vs, R: ri})
		}
	}
	for _, r := range lt.Rows {
		for i, v := range r.D {
			addVal(i, v)
		}
		if r.G > 0 {
			if lt.TMin == 0 || r.T < lt.TMin {
				lt.TMin = r.T
			}
			if r.E > lt.TMax {
				lt.TMax = r.E
			}
			if r.G > lt.Gran {
				lt.Gran = r.G
			}
		}
	}
}

// ── 시점 ───────────────────────────────────────────────

var (
	reMonYY   = regexp.MustCompile(`^([A-Za-z]{3})-(\d{2})$`)
	reQuarter = regexp.MustCompile(`^((?:19|20)\d{2})\D{0,3}([1-4])\s*분기`)
	reDigits  = regexp.MustCompile(`^\d+$`)
	reYMDx    = regexp.MustCompile(`^((?:19|20)\d{2})\D{1,3}(\d{1,2})(?:\D{1,3}(\d{1,2}))?(?:[ T]+(?:(오전|오후)\s*)?(\d{1,2})(?::\d{1,2})?)?`)
	monEn     = map[string]int{"jan": 1, "feb": 2, "mar": 3, "apr": 4, "may": 5, "jun": 6, "jul": 7, "aug": 8, "sep": 9, "oct": 10, "nov": 11, "dec": 12}
)

func tk(y, m, d, h int) int64 { return int64(((y*100+m)*100+d)*100 + h) }

func lastDay(y, m int) int {
	return time.Date(y, time.Month(m)+1, 0, 0, 0, 0, 0, time.UTC).Day()
}

// span 은 (연, 월, 일, 시) 와 정밀도로 시작·끝 시점을 만든다.
func span(y, m, d, h, g int) (int64, int64) {
	switch g {
	case 1:
		return tk(y, 1, 1, 0), tk(y, 12, 31, 23)
	case 2:
		return tk(y, m, 1, 0), tk(y, m, lastDay(y, m), 23)
	case 3:
		return tk(y, m, d, 0), tk(y, m, d, 23)
	}
	return tk(y, m, d, h), tk(y, m, d, h)
}

func rowTime(s string) (int64, int64, int) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, 0, 0
	}
	if m := reMonYY.FindStringSubmatch(s); m != nil {
		mo := monEn[strings.ToLower(m[1])]
		y, _ := strconv.Atoi(m[2])
		if mo > 0 {
			a, b := span(2000+y, mo, 0, 0, 2)
			return a, b, 2
		}
	}
	if m := reQuarter.FindStringSubmatch(s); m != nil {
		y, _ := strconv.Atoi(m[1])
		q, _ := strconv.Atoi(m[2])
		return tk(y, 3*q-2, 1, 0), tk(y, 3*q, lastDay(y, 3*q), 23), 2
	}
	if reDigits.MatchString(s) {
		at := func(i, j int) int { v, _ := strconv.Atoi(s[i:j]); return v }
		switch len(s) {
		case 4:
			if y := at(0, 4); y >= 1900 && y <= 2100 {
				a, b := span(y, 0, 0, 0, 1)
				return a, b, 1
			}
		case 6:
			y, m := at(0, 4), at(4, 6)
			if m >= 1 && m <= 12 {
				a, b := span(y, m, 0, 0, 2)
				return a, b, 2
			}
		case 8:
			y, m, d := at(0, 4), at(4, 6), at(6, 8)
			if m >= 1 && m <= 12 && d >= 1 {
				a, b := span(y, m, d, 0, 3)
				return a, b, 3
			}
		case 10, 12, 14:
			y, m, d, h := at(0, 4), at(4, 6), at(6, 8), at(8, 10)
			if m >= 1 && m <= 12 && d >= 1 {
				a, b := span(y, m, d, h, 4)
				return a, b, 4
			}
		}
		return 0, 0, 0
	}
	// 「2013-01」(연 열 + 월 열) · 「2018. 01」 · 「2000-01-01」 · 「2023-01-01 09:00」
	if m := reYMDx.FindStringSubmatch(s); m != nil {
		y, _ := strconv.Atoi(m[1])
		mo, _ := strconv.Atoi(m[2])
		if mo < 1 || mo > 12 {
			return 0, 0, 0
		}
		if m[3] == "" {
			a, b := span(y, mo, 0, 0, 2)
			return a, b, 2
		}
		d, _ := strconv.Atoi(m[3])
		if m[5] == "" {
			a, b := span(y, mo, d, 0, 3)
			return a, b, 3
		}
		h, _ := strconv.Atoi(m[5])
		if m[4] == "오후" && h < 12 {
			h += 12
		} else if m[4] == "오전" && h == 12 {
			h = 0
		}
		a, b := span(y, mo, d, h, 4)
		return a, b, 4
	}
	return 0, 0, 0
}

var reTitlePeriod = regexp.MustCompile(`(\d{2})년\s*(\d{1,2})월\s*(\d{1,2})일\s*~\s*(\d{2})년\s*(\d{1,2})월\s*(\d{1,2})일`)
var reTitleHalf = regexp.MustCompile(`((?:19|20)\d{2})년\s*(상반기|하반기)`)

func titlePeriod(t string) (int64, int64) {
	if m := reTitlePeriod.FindStringSubmatch(t); m != nil {
		n := func(i int) int { v, _ := strconv.Atoi(m[i]); return v }
		return tk(2000+n(1), n(2), n(3), 0), tk(2000+n(4), n(5), n(6), 23)
	}
	if m := reTitleHalf.FindStringSubmatch(t); m != nil {
		y, _ := strconv.Atoi(m[1])
		if m[2] == "상반기" {
			return tk(y, 1, 1, 0), tk(y, 6, 30, 23)
		}
		return tk(y, 7, 1, 0), tk(y, 12, 31, 23)
	}
	return 0, 0
}

var reTitleSplit = regexp.MustCompile(`[()~_\-·,\s]+`)

// titleNames 는 「귀덕마을길(리사무소-라신동)_무장애여행위치정보」에서 「귀덕마을길」「리사무소」「라신동」을 뽑는다.
func titleNames(t string) []string {
	if i := strings.Index(t, "_"); i > 0 {
		t = t[:i]
	}
	out := []string{}
	whole := strings.Join(reTitleSplit.Split(t, -1), "")
	if len([]rune(whole)) >= 2 {
		out = append(out, whole)
	}
	for _, p := range reTitleSplit.Split(t, -1) {
		if len([]rune(p)) >= 2 && p != whole {
			out = append(out, p)
		}
	}
	return out
}

func fmtT(t int64, g int) string {
	y, m, d, h := int(t/1000000), int(t/10000%100), int(t/100%100), int(t%100)
	switch g {
	case 1:
		return fmt.Sprintf("%d년", y)
	case 2:
		return fmt.Sprintf("%d년 %d월", y, m)
	case 3:
		return fmt.Sprintf("%d년 %d월 %d일", y, m, d)
	}
	return fmt.Sprintf("%d년 %d월 %d일 %d시", y, m, d, h)
}

func tYear(t int64) int  { return int(t / 1000000) }
func tMonth(t int64) int { return int(t / 10000 % 100) }

// meltTime 은 펼치는 열이 시점을 품는가(@t).
func (s *Schema) meltTime() bool {
	if s.Melt == nil {
		return false
	}
	for _, c := range s.Melt.Cols {
		if _, ok := c.Dims["@t"]; ok {
			return true
		}
	}
	return false
}
