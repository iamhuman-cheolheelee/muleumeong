package agent

// 데이터 찾기 — 제주데이터허브 1,259종 카탈로그(제목·설명·태그·미리보기 열 이름)를
// 두 가지 방법으로 순위를 매기고 RRF(k=60)로 합친다.
//   ① 글자 2-gram BM25 — 「감귤생산량」처럼 띄어 쓰지 않은 말도 잡는다
//   ② 낱말 포함률 — 질문의 핵심 낱말이 제목·태그·열 이름에 몇 개 들어 있나

import (
	"embed"
	"encoding/json"
	"fmt"
	"math"
	"regexp"
	"sort"
	"strings"
)

//go:embed data/catalog.json data/colnames.json data/csv data/prev data/pub data/pub.json
var dataFS embed.FS

type FileInfo struct {
	ID           int    `json:"id"`
	FileID       int    `json:"fileId"`
	FileName     string `json:"fileName"`
	SaveFileName string `json:"saveFileName"`
	FileSize     int    `json:"fileSize"`
	ContentType  string `json:"contentType"`
}

type Dataset struct {
	ID      int       `json:"id"`
	Title   string    `json:"title"`
	Cat     string    `json:"cat"`
	Desc    string    `json:"desc"`
	Tags    string    `json:"tags"`
	Owner   string    `json:"owner"`
	Auth    bool      `json:"auth"`
	File    *FileInfo `json:"file"`
	Preview *FileInfo `json:"preview"`
	API     bool      `json:"api"`
	Updated string    `json:"updated"`
	Views   int       `json:"views"`
	Header  []string  `json:"header,omitempty"` // 미리보기 파일 첫 줄
}

// Open 은 로그인 없이 원본을 바로 내려받을 수 있는가.
func (d *Dataset) Open() bool { return !d.Auth && d.File != nil && isTable(d.File.FileName) }

func isTable(n string) bool {
	n = strings.ToLower(n)
	return strings.HasSuffix(n, ".csv") || strings.HasSuffix(n, ".xlsx")
}

func (d *Dataset) URL() string {
	if p := pubBy[d.ID]; p != nil && (d.ID >= 90000 || !d.Open()) {
		return p.URL
	}
	return fmt.Sprintf("https://jejudatahub.net/data/view/data/%d", d.ID)
}

var (
	Catalog []*Dataset
	byID    = map[int]*Dataset{}
	ColDict = map[string]string{}
	idx     *index
)

func Load() error {
	b, err := dataFS.ReadFile("data/catalog.json")
	if err != nil {
		return err
	}
	if err := json.Unmarshal(b, &Catalog); err != nil {
		return err
	}
	if b, err := dataFS.ReadFile("data/colnames.json"); err == nil {
		json.Unmarshal(b, &ColDict)
	}
	for _, d := range Catalog {
		byID[d.ID] = d
		if pb, err := dataFS.ReadFile(fmt.Sprintf("data/prev/%d.csv", d.ID)); err == nil {
			if t, err := ParseTable(pb, "prev.csv"); err == nil && len(t.Header) > 0 && !strings.HasPrefix(t.Header[0], "<") {
				d.Header = t.Header
			}
		}
	}
	loadPub()
	idx = build(Catalog)
	initEngine()
	return nil
}

func Get(id int) *Dataset { return byID[id] }

// ── 낱말 다루기 ──────────────────────────────────────────────

var (
	reWord = regexp.MustCompile(`[\p{Hangul}A-Za-z0-9]+`)
	josa   = []string{"에서는", "에서", "으로", "이에요", "예요", "인가요", "나요", "은요", "는요", "까지", "부터", "보다", "처럼", "하고", "이랑", "랑", "은", "는", "이", "가", "을", "를", "에", "의", "로", "와", "과", "도", "만", "요"}
	// 질문 말투일 뿐 데이터를 가리지 않는 말
	stop = setOf("제주", "제주도", "제주특별자치도", "요즘", "최근", "어때", "어때요", "알려줘", "알려", "주세요", "얼마나", "어떻게", "어떤", "어디", "변했", "변했나", "변했어", "바뀌었", "가장", "많은", "많이", "곳", "몇", "년", "사이", "비교", "비교하면", "다른", "어느", "정도", "수는", "수", "있나", "있어", "궁금", "궁금해", "되나", "줄었", "줄었나", "늘었", "늘었나", "지난", "동안", "그리고", "데이터", "현황", "정보", "추이", "현재", "올해", "작년", "보여줘", "말해줘", "높은", "낮은", "적은", "인지", "대비", "어떻", "해요", "했나", "했나요")
	// 도민 말 → 데이터 말
	synonym = map[string][]string{
		"체감경기": {"경기동향", "BSI", "소상공인", "체감"}, "경기": {"경기동향", "BSI"}, "장사": {"소상공인", "매출"}, "가게": {"소상공인", "상가"},
		"관광객": {"관광", "방문객", "관람객"}, "관람객": {"관람객수", "관광지"}, "손님": {"방문객", "관람객"},
		"일자리": {"고용", "취업"}, "취업": {"고용", "취업"}, "청년": {"청년고용률"}, "아기": {"출생"}, "출생아": {"출생"}, "아이": {"출생"},
		"차": {"자동차"}, "자동차": {"자동차", "등록"}, "전기차": {"자동차", "전기차"}, "시끄": {"소음"}, "소음": {"소음", "소음도"},
		"귤": {"감귤"}, "감귤": {"감귤", "생산량"}, "꽃": {"화훼"}, "화훼": {"화훼", "재배"}, "농가": {"농가수"},
		"인구": {"인구"}, "날씨": {"기후", "기온"}, "기온": {"기후", "기온"}, "더위": {"기온"}, "버스": {"버스", "노선"},
	}
)

func setOf(xs ...string) map[string]bool {
	m := map[string]bool{}
	for _, x := range xs {
		m[x] = true
	}
	return m
}

func stripJosa(w string) string {
	for _, j := range josa {
		if strings.HasSuffix(w, j) && len([]rune(w))-len([]rune(j)) >= 2 {
			return strings.TrimSuffix(w, j)
		}
	}
	return w
}

// Terms 는 질문에서 데이터를 가리키는 낱말만 남긴다(조사·말투 제거 + 동의어 확장).
func Terms(q string) []string {
	seen, out := map[string]bool{}, []string{}
	add := func(w string) {
		if w != "" && !seen[w] && !stop[w] {
			seen[w] = true
			out = append(out, w)
		}
	}
	for _, w := range reWord.FindAllString(q, -1) {
		w = stripJosa(w)
		if stop[w] || len([]rune(w)) < 2 && !strings.ContainsAny(w, "차귤꽃김쌀굴밤콩") {
			continue
		}
		add(w)
		for k, syn := range synonym {
			if strings.Contains(w, k) {
				for _, s := range syn {
					add(s)
				}
			}
		}
	}
	return out
}

func bigrams(s string) []string {
	out := []string{}
	for _, w := range reWord.FindAllString(strings.ToLower(s), -1) {
		r := []rune(w)
		if len(r) == 1 {
			out = append(out, w)
		}
		for i := 0; i+1 < len(r); i++ {
			out = append(out, string(r[i:i+2]))
		}
	}
	return out
}

// ── 색인 ────────────────────────────────────────────────────

type index struct {
	tf    []map[string]float64
	dl    []float64
	avg   float64
	df    map[string]int
	text  []string
}

func docText(d *Dataset) (weighted string, plain string) {
	h := strings.Join(d.Header, " ")
	plain = strings.ToLower(d.Title + " " + d.Tags + " " + h + " " + d.Cat)
	weighted = strings.Repeat(d.Title+" ", 3) + strings.Repeat(d.Tags+" ", 2) + strings.Repeat(h+" ", 2) + d.Desc + " " + d.Cat
	return
}

func build(ds []*Dataset) *index {
	ix := &index{df: map[string]int{}}
	for _, d := range ds {
		w, p := docText(d)
		tf := map[string]float64{}
		bg := bigrams(w)
		for _, g := range bg {
			tf[g]++
		}
		for g := range tf {
			ix.df[g]++
		}
		ix.tf = append(ix.tf, tf)
		ix.dl = append(ix.dl, float64(len(bg)))
		ix.text = append(ix.text, p)
		ix.avg += float64(len(bg))
	}
	ix.avg /= math.Max(1, float64(len(ds)))
	return ix
}

type Hit struct {
	ID      int      `json:"id"`
	Title   string   `json:"title"`
	Cat     string   `json:"cat"`
	Owner   string   `json:"owner"`
	Updated string   `json:"updated"`
	Open    bool     `json:"open"`
	API     bool     `json:"api"`
	Format  string   `json:"format"`
	Score   float64  `json:"score"`
	BM25    int      `json:"bm25Rank"`
	Cover   int      `json:"coverRank"`
	Matched []string `json:"matched"`
	Header  []string `json:"header"`
	URL     string   `json:"url"`
}

// Search 는 질문을 받아 상위 k개 데이터셋을 돌려준다.
func Search(q string, k int, extra []string) ([]Hit, []string) {
	terms := Terms(q)
	for _, e := range extra {
		if !stop[e] && !contains(terms, e) {
			terms = append(terms, e)
		}
	}
	qg := bigrams(strings.Join(terms, " "))
	n := float64(len(Catalog))
	type sc struct {
		i     int
		bm, c float64
		m     []string
	}
	all := make([]sc, len(Catalog))
	for i := range Catalog {
		s := 0.0
		for _, g := range qg {
			f := idx.tf[i][g]
			if f == 0 {
				continue
			}
			idf := math.Log(1 + (n-float64(idx.df[g])+0.5)/(float64(idx.df[g])+0.5))
			s += idf * f * 2.2 / (f + 1.2*(0.25+0.75*idx.dl[i]/idx.avg))
		}
		m := []string{}
		for _, t := range terms {
			if strings.Contains(idx.text[i], strings.ToLower(t)) {
				m = append(m, t)
			}
		}
		all[i] = sc{i, s, float64(len(m)), m}
	}
	rank := func(key func(sc) float64) map[int]int {
		ord := make([]sc, len(all))
		copy(ord, all)
		sort.SliceStable(ord, func(a, b int) bool { return key(ord[a]) > key(ord[b]) })
		r := map[int]int{}
		for p, x := range ord {
			if key(x) > 0 {
				r[x.i] = p + 1
			}
		}
		return r
	}
	rb := rank(func(x sc) float64 { return x.bm })
	rc := rank(func(x sc) float64 { return x.c + x.bm/1000 })
	hits := []Hit{}
	for i, d := range Catalog {
		s := 0.0
		if r, ok := rb[i]; ok {
			s += 1 / (60 + float64(r))
		}
		if r, ok := rc[i]; ok {
			s += 1 / (60 + float64(r))
		}
		if s == 0 {
			continue
		}
		h := Hit{ID: d.ID, Title: d.Title, Cat: d.Cat, Owner: d.Owner, Updated: d.Updated, Open: d.Open(), API: d.API,
			Score: math.Round(s*1e5) / 1e5, BM25: rb[i], Cover: rc[i], Matched: all[i].m, Header: d.Header, URL: d.URL()}
		if d.File != nil {
			h.Format = d.File.FileName[strings.LastIndex(d.File.FileName, ".")+1:]
		}
		hits = append(hits, h)
	}
	sort.SliceStable(hits, func(a, b int) bool { return hits[a].Score > hits[b].Score })
	if len(hits) > k {
		hits = hits[:k]
	}
	return hits, terms
}

func contains(xs []string, x string) bool {
	for _, y := range xs {
		if y == x {
			return true
		}
	}
	return false
}

// Preview 는 내려받아 둔 미리보기 파일을 준다.
func Preview(id int) ([]byte, bool) {
	b, err := dataFS.ReadFile(fmt.Sprintf("data/prev/%d.csv", id))
	return b, err == nil && len(b) > 0
}

// ── 공공데이터포털 사본 ─────────────────────────────────
// 데이터허브에서 로그인이 필요한 자료 가운데, 같은 기관이 공공데이터포털(data.go.kr)에
// 로그인 없이 올려 둔 같은 이름의 파일을 data/pub 에 넣어 두고 계산에 쓴다.
// 허브에 없는 자료는 90000번대 번호를 붙여 목록에 더한다.

type PubInfo struct {
	ID       int    `json:"id"`
	Title    string `json:"title"`
	Provider string `json:"provider"`
	URL      string `json:"url"`
	Cat      string `json:"cat"`
	Period   string `json:"period"`
}

var pubBy = map[int]*PubInfo{}

func loadPub() {
	b, err := dataFS.ReadFile("data/pub.json")
	if err != nil {
		return
	}
	var ps []*PubInfo
	if json.Unmarshal(b, &ps) != nil {
		return
	}
	for _, p := range ps {
		pubBy[p.ID] = p
		if byID[p.ID] == nil {
			d := &Dataset{ID: p.ID, Title: p.Title, Cat: p.Cat, Owner: p.Provider, Desc: p.Title + " (공공데이터포털 파일데이터)", Tags: p.Cat}
			Catalog = append(Catalog, d)
			byID[p.ID] = d
		}
	}
}

func isPub(id int) bool { return pubBy[id] != nil }
