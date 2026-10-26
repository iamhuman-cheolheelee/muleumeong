package agent

// 에이전트 한 바퀴 — 질문 이해 → 데이터 찾기 → 불러오기 → 계산 → 답 쓰기·검산
// 각 단계는 도구(Tool) 하나이고, 화면의 「일한 순서」에 그대로 찍힌다.

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

type Step struct {
	Tool   string `json:"tool"`
	Name   string `json:"name"`
	Detail string `json:"detail"`
	Ms     int64  `json:"ms"`
	State  string `json:"state"` // ok | warn | skip
}

type Answer struct {
	Q        string   `json:"q"`
	Terms    []string `json:"terms"`
	Extra    []string `json:"extra"`
	Hits     []Hit    `json:"hits"`
	Chosen   *Hit     `json:"chosen"`
	Skipped  []string `json:"skipped"`
	Source   string   `json:"source"` // 원본 전체 | 내려받아 둔 원본 | 미리보기
	Format   string   `json:"format"`
	Rows     int      `json:"rows"`
	Result   *Result  `json:"result"`
	Text     string   `json:"text"`
	Writer   string   `json:"writer"` // 캐시 | 로컬 모델 | 규칙 문장
	Check    string   `json:"check"`
	Steps    []Step   `json:"steps"`
	TotalMs  int64    `json:"totalMs"`
	Error    string   `json:"error,omitempty"`
	Intent   Intent   `json:"intent"`
	Meaning  *Meaning `json:"meaning,omitempty"`
	NumOK    bool     `json:"numOk"`
	Refused  bool     `json:"refused"`
	Cands    []Cand   `json:"cands"`
}

var (
	Live    = os.Getenv("LIVE_FETCH") != "0"
	FullDir = env("FULL_DIR", "full")
	fileMu  sync.Mutex
	fileMem = map[int][]byte{}
)

// fetch 는 원본 파일을 가져온다: 메모리 → 내려받아 둔 원본(data/csv) → 데이터허브 실시간 → 미리보기.
func fetch(d *Dataset) ([]byte, string, error) {
	fileMu.Lock()
	if b, ok := fileMem[d.ID]; ok {
		fileMu.Unlock()
		return b, "원본 전체(이번 실행에서 받아 둔 것)", nil
	}
	fileMu.Unlock()
	if b, err := dataFS.ReadFile(fmt.Sprintf("data/csv/%d.csv", d.ID)); err == nil {
		return b, "원본 전체(데이터허브에서 내려받아 둔 사본)", nil
	}
	fp := filepath.Join(FullDir, fmt.Sprintf("%d.bin", d.ID))
	if b, err := os.ReadFile(fp); err == nil && len(b) > 0 {
		return b, "원본 전체(데이터허브에서 내려받아 둔 사본)", nil
	}
	if b, err := dataFS.ReadFile(fmt.Sprintf("data/pub/%d.csv", d.ID)); err == nil {
		return b, "원본 전체(같은 기관이 공공데이터포털에 로그인 없이 공개한 파일)", nil
	}
	if Live && d.Open() && d.File.FileSize < 30<<20 {
		if b, err := download(d.File); err == nil {
			fileMu.Lock()
			fileMem[d.ID] = b
			fileMu.Unlock()
			os.MkdirAll(FullDir, 0o755)
			os.WriteFile(fp, b, 0o644)
			return b, "원본 전체(데이터허브 실시간 내려받기)", nil
		}
	}
	if b, err := dataFS.ReadFile(fmt.Sprintf("data/prev/%d.csv", d.ID)); err == nil {
		return b, "미리보기 파일(일부 행)", nil
	}
	return nil, "", fmt.Errorf("파일을 가져오지 못했습니다")
}

func download(f *FileInfo) ([]byte, error) {
	v := url.Values{}
	v.Set("id", strconv.Itoa(f.ID))
	v.Set("fileId", strconv.Itoa(f.FileID))
	v.Set("fileName", f.FileName)
	v.Set("saveFileName", f.SaveFileName)
	v.Set("fileSize", strconv.Itoa(f.FileSize))
	v.Set("contentType", f.ContentType)
	c := &http.Client{Timeout: 20 * time.Second}
	resp, err := c.Get("https://jejudatahub.net/api/file/test-download?" + v.Encode())
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	return io.ReadAll(io.LimitReader(resp.Body, 32<<20))
}

func since(t time.Time) int64 { return time.Since(t).Milliseconds() }

// Ask 는 질문 하나를 끝까지 처리한다. pin 이 0이 아니면 그 데이터셋을 쓴다(사용자가 후보를 바꿨을 때).
func Ask(q string, pin int) *Answer {
	T := time.Now()
	a := &Answer{Q: q, Steps: []Step{}, Skipped: []string{}}
	q = strings.TrimSpace(q)
	for _, r := range colloq {
		q = strings.ReplaceAll(q, r[0], r[1])
	}
	if q == "" {
		a.Error = "질문이 비어 있습니다"
		return a
	}

	// ① 질문 이해
	t0 := time.Now()
	a.Terms = Terms(q)
	extra, _ := Expand(q)
	a.Extra = extra
	det := "핵심 낱말: " + strings.Join(a.Terms, ", ")
	if len(extra) > 0 {
		det += " · AI가 덧붙인 검색어: " + strings.Join(extra, ", ")
	} else {
		det += " · 비슷한 말 사전만 사용"
	}
	a.Steps = append(a.Steps, Step{"understand", "질문 이해", det, since(t0), "ok"})

	in := ParseIntent(q)
	a.Intent = in
	if in.Refuse != "" {
		hits, _ := Search(q, 6, extra)
		a.Hits = hits
		a.refuse(in.Refuse, nil)
		a.Steps = append(a.Steps, Step{"refuse", "답할 수 있는 질문인지", in.Refuse, 0, "warn"})
		a.TotalMs = since(T)
		return a
	}

	if engineReady {
		return askEngine(a, q, extra, T)
	}

	// ② 데이터 찾기 — 관련도 순서대로 후보를 하나씩 계산해 보고, 의미 검산을 통과한 첫 후보만 쓴다.
	// (원본을 받을 수 있다고 앞세우지 않는다. 받을 수 없는 후보는 건너뛰고 이유를 남긴다.)
	t0 = time.Now()
	hits, _ := Search(q, 12, extra)
	if len(hits) > 8 {
		a.Hits = hits[:8]
	} else {
		a.Hits = hits
	}
	type tried struct {
		d    *Dataset
		h    *Hit
		tb   *Table
		cols []Col
		res  Result
		m    Meaning
		src  string
	}
	try := func(h *Hit) (*tried, string) {
		x := Get(h.ID)
		if x == nil {
			return nil, "없는 데이터"
		}
		if !(x.Open() || hasCopy(x.ID)) {
			why := "원본은 데이터허브 로그인·이용 신청이 필요해 계산하지 않음"
			if x.File != nil && !isTable(x.File.FileName) {
				why = "표가 아닌 파일(" + x.File.FileName[strings.LastIndex(x.File.FileName, ".")+1:] + ")"
			} else if !x.Auth && x.File == nil {
				why = "내려받을 원본 파일 없음(오픈 API만)"
			}
			return nil, why
		}
		b, src, err := fetch(x)
		if err != nil || strings.HasPrefix(src, "미리보기") {
			return nil, "원본을 받지 못함"
		}
		name := "data.csv"
		if x.File != nil {
			name = x.File.FileName
			if isPub(x.ID) && !x.Open() {
				name = "data.csv"
			}
		}
		tb, err := ParseTable(b, name)
		if err != nil || len(tb.Header) == 0 {
			return nil, "표로 읽을 수 없는 파일"
		}
		cols := Profile(tb, ColDict)
		res := Analyze(q, tb, cols, in)
		m := CheckMeaning(q, in, x, tb, cols, &res)
		tr := &tried{x, h, tb, cols, res, m, src}
		if !m.Pass {
			return tr, strings.Join(m.Fails(), " · ")
		}
		return tr, ""
	}
	var pick *tried
	var firstFail *tried
	if pin > 0 {
		var h *Hit
		for i := range hits {
			if hits[i].ID == pin {
				h = &hits[i]
			}
		}
		if d := Get(pin); d != nil && h == nil {
			x := Hit{ID: d.ID, Title: d.Title, Cat: d.Cat, Owner: d.Owner, Updated: d.Updated, Open: d.Open(), API: d.API, Header: d.Header, URL: d.URL()}
			h = &x
		}
		if h != nil {
			tr, why := try(h)
			if why == "" {
				pick = tr
			} else {
				firstFail = tr
				a.Cands = append(a.Cands, Cand{Hit: *h, Why: why})
			}
		}
	} else {
		computed := 0
		for i := range hits {
			if computed >= 6 {
				break
			}
			if len(hits[i].Matched) == 0 {
				a.Cands = append(a.Cands, Cand{Hit: hits[i], Why: "질문 낱말이 제목·열 이름에 없음"})
				continue
			}
			tr, why := try(&hits[i])
			if tr != nil {
				computed++
			}
			if why == "" {
				pick = tr
				break
			}
			if tr != nil && firstFail == nil {
				firstFail = tr
			}
			a.Cands = append(a.Cands, Cand{Hit: hits[i], Why: why})
			a.Skipped = append(a.Skipped, hits[i].Title+": "+why)
		}
	}
	if pick == nil {
		a.Steps = append(a.Steps, Step{"search_catalog", "데이터 찾기", fmt.Sprintf("1,259종 중 후보 %d개를 계산해 봤지만 질문과 뜻이 맞는 원본이 없음", len(a.Cands)), since(t0), "warn"})
		if firstFail != nil {
			m := firstFail.m
			a.Meaning = &m
		}
		a.refuse("맞는 데이터를 찾지 못했어요", a.Cands)
		a.TotalMs = since(T)
		return a
	}
	d := pick.d
	a.Chosen = pick.h
	a.Steps = append(a.Steps, Step{"search_catalog", "데이터 찾기", fmt.Sprintf("1,259종에서 후보 %d개 → 「%s」(겹친 낱말: %s)", len(hits), d.Title, strings.Join(a.Chosen.Matched, ", ")), since(t0), "ok"})
	tb, res := pick.tb, pick.res
	a.Source, a.Format, a.Rows = pick.src, tb.Format, len(tb.Rows)
	a.Steps = append(a.Steps, Step{"fetch_file", "원본 불러오기", fmt.Sprintf("%s · %s · %s행 %d열", pick.src, tb.Format, Fmt(float64(len(tb.Rows))), len(tb.Header)), 0, "ok"})

	// ④ 계산
	res.Code = recipe(d, tb, &res)
	a.Result = &res
	a.Steps = append(a.Steps, Step{"compute", "코드로 계산", fmt.Sprintf("%s · 측정 「%s」 · 원본 %s행 사용 · 확인된 숫자 %d개", res.OpLabel, res.Measure, Fmt(float64(res.RowsUsed)), len(res.Facts)), 0, "ok"})
	m := pick.m
	a.Meaning = &m
	a.Steps = append(a.Steps, Step{"check_meaning", "의미 검산", fmt.Sprintf("주제·지표·연산·대상·시점 %d가지 모두 맞음", len(m.Items)), 0, "ok"})

	// ⑤ 답 쓰기·숫자 검산
	t0 = time.Now()
	tmpl := Template(d.Title, &res)
	a.Text, a.Writer, a.Check = tmpl, "규칙 문장", "정해진 틀에 계산 결과 숫자만 끼워 넣었습니다"
	a.NumOK = true
	if len(res.Facts) > 0 {
		o := Chat("write", sysWrite, writePrompt(q, d.Title, &res), 260)
		if o.Text != "" {
			if bad := CheckNumbers(o.Text, q, &res); len(bad) == 0 && keepsNotes(o.Text, &res) {
				a.Text, a.Writer = clean(o.Text), o.Level
				a.Check = fmt.Sprintf("문장 속 숫자 %d개가 모두 계산 결과와 일치", len(reNum.FindAllString(o.Text, -1)))
			} else if len(bad) > 0 {
				a.Check = fmt.Sprintf("AI 문장에 계산 결과에 없는 숫자나 단위가 붙어(%s) 그 문장은 버리고, 정해진 틀로 다시 썼습니다", strings.Join(bad, ", "))
			} else {
				a.Check = "AI 문장이 시점 안내를 빠뜨려 정해진 틀로 다시 썼습니다"
			}
		} else if o.Err != "" {
			a.Check = o.Err + " → 규칙 문장"
		}
	}
	a.Steps = append(a.Steps, Step{"write_answer", "답 쓰기·숫자 검산", a.Check, since(t0), "ok"})
	a.TotalMs = since(T)
	return a
}

type Cand struct {
	Hit
	Why string `json:"why"`
}

// refuse 는 답을 내지 않는다. 숫자를 하나도 보여 주지 않고, 가까운 후보 3개와 맞지 않은 이유만 준다.
func (a *Answer) refuse(msg string, cands []Cand) {
	a.Refused, a.Error = true, msg
	a.Result, a.Chosen, a.Text = nil, nil, msg
	out := []Cand{}
	if cands == nil {
		a.Cands = out // 원인·전망·개인정보 질문은 후보를 보여 주지 않는다
		return
	}
	seen := map[int]bool{}
	for _, c := range cands {
		if len(out) < 3 && !seen[c.ID] {
			out, seen[c.ID] = append(out, c), true
		}
	}
	for _, h := range a.Hits {
		if len(out) < 3 && !seen[h.ID] {
			out, seen[h.ID] = append(out, Cand{Hit: h, Why: "제목은 비슷하지만 계산할 열 정보가 없어 답하지 않았어요"}), true
		}
	}
	a.Cands = out
}

// keepsNotes 는 「작년 자료가 없어 2024년 값」 같은 시점 안내를 AI 문장이 지켰는지 본다.
func keepsNotes(text string, r *Result) bool {
	if r.YearNote == "" {
		return true
	}
	return strings.Contains(text, "없") || strings.Contains(text, "최근")
}

func yr(k string) string {
	if len(k) == 4 {
		return k + "년"
	}
	return k
}

// polite 는 「늘었다」 같은 사실표 표현을 존댓말로 바꾼다.
func polite(s string) string {
	if strings.HasSuffix(s, "었다") || strings.HasSuffix(s, "졌다") || strings.HasSuffix(s, "였다") {
		return strings.TrimSuffix(s, "다") + "습니다"
	}
	return s
}

func clean(s string) string {
	s = strings.ReplaceAll(s, "**", "")
	return strings.TrimSpace(s)
}

// hasPreview 는 로그인 없이 받는 미리보기 표가 있는가(원본이 신청제여도 일부 행으로 계산할 수 있다).
func hasPreview(d *Dataset) bool {
	return len(d.Header) >= 2
}

func hasCopy(id int) bool {
	if _, err := dataFS.ReadFile(fmt.Sprintf("data/csv/%d.csv", id)); err == nil {
		return true
	}
	_, err := dataFS.ReadFile(fmt.Sprintf("data/pub/%d.csv", id))
	return err == nil
}

func fact(r *Result, k string) string {
	for _, f := range r.Facts {
		if f.Key == k {
			return f.Value
		}
	}
	return ""
}

// Template 은 모델이 없을 때 쓰는 문장 틀이다. 숫자는 사실표에서만 온다.
func Template(title string, r *Result) string {
	if len(r.Facts) == 0 {
		return "「" + title + "」에서 질문에 맞는 숫자를 계산하지 못했습니다. 오른쪽 열 목록을 보고 낱말을 바꿔 물어 주세요."
	}
	cond := ""
	if len(r.Filters) > 0 {
		cond = "(" + strings.Join(r.Filters, ", ") + ")"
	}
	var s string
	switch r.Op {
	case "value":
		s = fmt.Sprintf("「%s」 기준 %s%s %s은 %s입니다. ", title, r.Period, cond, r.Measure, fact(r, "value"))
		if r.Agg == "mean" {
			s = fmt.Sprintf("「%s」 기준 %s%s %s 평균은 %s입니다. ", title, r.Period, cond, r.Measure, fact(r, "value"))
		}
		if v := fact(r, "change"); v != "" && fact(r, "from") != "" {
			s += fmt.Sprintf("비교한 해의 값은 %s이고, 차이는 %s로 %s ", fact(r, "from"), v, fact(r, "dir"))
		}
		if v := fact(r, "sign"); v != "" {
			s += "0보다 " + v + "입니다. "
		}
	case "peak":
		s = fmt.Sprintf("「%s」%s에서 %s은 %s이고, %s에 나왔습니다. ", title, cond, r.Facts[0].Label, fact(r, "value"), fact(r, "when"))
	case "count":
		s = fmt.Sprintf("「%s」 목록%s에 있는 곳은 %s입니다. ", title, cond, fact(r, "value"))
	case "trend":
		s = fmt.Sprintf("「%s」 기준 %s%s은 %s %s에서 %s %s까지 %s. ", title, r.Measure, cond,
			yr(r.Series[0].K), fact(r, "first"), yr(r.Series[len(r.Series)-1].K), fact(r, "last"), polite(fact(r, "change")))
		if fact(r, "change") == "" {
			s = fmt.Sprintf("「%s」 기준 %s%s은 %s %s입니다. ", title, r.Measure, cond, r.Series[0].K, fact(r, "first"))
		}
		s += fmt.Sprintf("가장 높았던 때는 %s, 가장 낮았던 때는 %s입니다. ", fact(r, "max"), fact(r, "min"))
	case "compare":
		if fact(r, "target") != "" {
			s = fmt.Sprintf("「%s」 %s 기준 %s은 %s이고 값은 %s입니다. ", title, r.Period, r.Plan.Target, fact(r, "target"), fact(r, "targetv"))
			if v := fact(r, "vsmed"); v != "" {
				s += fmt.Sprintf("비교한 곳들의 중간값과 견주면 %s이고, 1위는 %s입니다. ", v, fact(r, "top1"))
			}
		} else {
			s = fmt.Sprintf("「%s」 %s 기준 1위는 %s입니다. ", title, r.Period, fact(r, "top1"))
		}
	default:
		s = fmt.Sprintf("「%s」 %s 기준 %s 1위는 %s, 2위는 %s입니다. ", title, r.Period, r.Measure, fact(r, "top1"), fact(r, "top2"))
		if v := fact(r, "top1share"); v != "" {
			s += fmt.Sprintf("1위가 전체의 %s를 차지합니다. ", v)
		}
	}
	if n := yearNoteText(r); n != "" {
		s += "참고: " + n
	} else if len(r.Notes) > 0 {
		s += "참고: " + r.Notes[len(r.Notes)-1]
	} else {
		s += "공개 통계를 그대로 계산한 값이며, 왜 그렇게 됐는지까지 알려 주지는 않습니다."
	}
	return s
}

// recipe 는 같은 계산을 pandas 로 다시 해 보는 코드다(재현용).
func recipe(d *Dataset, t *Table, r *Result) string {
	p := r.Plan
	var b strings.Builder
	enc := "utf-8-sig"
	if strings.Contains(t.Format, "CP949") {
		enc = "cp949"
	}
	fmt.Fprintf(&b, "# 물으멍 재현 코드 — 「%s」(#%d)\n# 원본: %s\nimport pandas as pd\n", d.Title, d.ID, d.URL())
	fn := "data.csv"
	if d.File != nil {
		fn = d.File.FileName
		if isPub(d.ID) && !d.Open() {
			fn = "data.csv"
		}
	}
	if strings.Contains(t.Format, "엑셀") {
		fmt.Fprintf(&b, "df = pd.read_excel(%q, sheet_name=0, dtype=str)\n", fn)
	} else {
		fmt.Fprintf(&b, "df = pd.read_csv(%q, encoding=%q, dtype=str)\n", fn, enc)
	}
	b.WriteString("df.columns = df.columns.str.strip()\n")
	for ci, v := range p.Filters {
		fmt.Fprintf(&b, "df = df[df[%q].str.strip() == %q]\n", t.Header[ci], v)
	}
	b.WriteString("TOTAL = {'계','합계','총계','소계','전국','전체','총합'}\n")
	for _, c := range r.Cols {
		if c.Kind == "cat" {
			fmt.Fprintf(&b, "df = df[~df[%q].str.strip().isin(TOTAL)]\n", c.Name)
		}
	}
	if r.Op == "count" {
		b.WriteString("print('곳 수:', len(df))\n")
		return b.String()
	}
	num := func(col string) string { return fmt.Sprintf("pd.to_numeric(df[%q].str.replace(',', ''), errors='coerce')", col) }
	agg := map[string]string{"sum": "sum", "mean": "mean"}[p.Agg]
	if len(p.Measure) == 0 {
		return b.String()
	}
	m := t.Header[p.Measure[0]]
	switch {
	case (p.Op == "trend" || p.Op == "value") && p.TimeCol >= 0:
		fmt.Fprintf(&b, "df['값'] = %s\n", num(m))
		fmt.Fprintf(&b, "df['연'] = df[%q].str.extract(r'((?:19|20)\\d{2})')[0]\n", t.Header[p.TimeCol])
		if p.Monthly {
			if p.MonthCol >= 0 {
				fmt.Fprintf(&b, "df['월'] = df[%q].astype(int)\n", t.Header[p.MonthCol])
			} else {
				fmt.Fprintf(&b, "df['월'] = df[%q].str.replace(r'\\D', '', regex=True).str[4:6].astype(int)\n", t.Header[p.TimeCol])
			}
			fmt.Fprintf(&b, "s = df.groupby(['연', '월'])['값'].%s().tail(24)\n", agg)
		} else if p.Stock {
			if p.MonthCol >= 0 {
				fmt.Fprintf(&b, "df['월'] = df[%q].astype(int)\n", t.Header[p.MonthCol])
			} else {
				fmt.Fprintf(&b, "df['월'] = df[%q].str.replace(r'\\D', '', regex=True).str[4:6].replace('', '0').astype(int)\n", t.Header[p.TimeCol])
			}
			b.WriteString("df = df[df['월'] == df.groupby('연')['월'].transform('max')]  # 쌓인 값은 해마다 마지막 달만\n")
			fmt.Fprintf(&b, "s = df.groupby('연')['값'].%s()\n", agg)
		} else {
			fmt.Fprintf(&b, "s = df.groupby('연')['값'].%s()\n", agg)
		}
		if p.Op == "value" {
			fmt.Fprintf(&b, "print(s)\nprint(%q, s.get(%q))\n", r.Period, strings.TrimSuffix(r.Period, "년"))
		} else {
			b.WriteString("print(s)\nprint('변화율 %:', round((s.iloc[-1] - s.iloc[0]) / abs(s.iloc[0]) * 100, 1))\n")
		}
	case p.Op == "value":
		fmt.Fprintf(&b, "print(%s.%s())\n", num(m), agg)
	case p.Wide && p.Agg == "mean":
		cs := []string{}
		for _, ci := range p.Measure {
			cs = append(cs, fmt.Sprintf("%q", t.Header[ci]))
		}
		fmt.Fprintf(&b, "cols = [%s]\n", strings.Join(cs, ", "))
		b.WriteString("s = df[cols].apply(lambda c: pd.to_numeric(c.str.replace(',', ''), errors='coerce')).mean().round(1).sort_values(ascending=False)\nprint(s)\n")
	case p.Wide:
		cs := []string{}
		for _, ci := range p.Measure {
			cs = append(cs, fmt.Sprintf("%q", t.Header[ci]))
		}
		fmt.Fprintf(&b, "cols = [%s]\n", strings.Join(cs, ", "))
		if p.TimeCol >= 0 {
			fmt.Fprintf(&b, "df = df.sort_values(%q).tail(12)  # 최근 12개월\n", t.Header[p.TimeCol])
		}
		b.WriteString("s = df[cols].apply(lambda c: pd.to_numeric(c.str.replace(',', ''), errors='coerce')).sum().sort_values(ascending=False)\nprint(s)\n")
	default:
		fmt.Fprintf(&b, "df['값'] = %s\n", num(m))
		if p.TimeCol >= 0 && strings.HasSuffix(r.Period, "시점") {
			fmt.Fprintf(&b, "df = df[df[%q] == df[%q].max()]  # 쌓이는 값은 가장 최근 달만\n", t.Header[p.TimeCol], t.Header[p.TimeCol])
		} else if p.TimeCol >= 0 {
			fmt.Fprintf(&b, "df['연'] = df[%q].str.extract(r'((?:19|20)\\d{2})')[0]\ndf = df[df['연'] == df['연'].max()]\n", t.Header[p.TimeCol])
		}
		if p.Group >= 0 {
			fmt.Fprintf(&b, "s = df.groupby(%q)['값'].%s().sort_values(ascending=False)\nprint(s)\n", t.Header[p.Group], agg)
			if p.Target != "" {
				fmt.Fprintf(&b, "print(%q, '순위:', list(s.index).index(%q) + 1, '/', len(s))\n", p.Target, p.Target)
			}
		}
	}
	return b.String()
}

func yearNoteText(r *Result) string {
	if r.YearNote == "" {
		return ""
	}
	for _, n := range r.Notes {
		if strings.Contains(n, "자료가 아직 없어") {
			return n
		}
	}
	return ""
}
