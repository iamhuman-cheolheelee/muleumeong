package agent

// 모델 호출 — 숫자를 묻지 않는다. 두 군데만 쓴다.
//   ① 질문 넓히기: 도민 말 → 데이터 낱말(최대 4개)
//   ② 답 쓰기: 사실표의 숫자만 옮겨 쉬운 문장 3개
// 3단 강등: 캐시(cache/*.json) → 로컬 모델(OpenAI 호환 llama-server, 6초) → 규칙(코드 안 문장 틀)

import (
	"bytes"
	"context"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

var (
	LLMURL   = env("LLM_URL", "http://127.0.0.1:18165/v1/chat/completions")
	LLMModel = env("LLM_MODEL", "qwen2.5-3b-instruct")
	CacheDir = env("CACHE_DIR", "cache")
	// 서버가 부를 수 있는 모델은 이것뿐이다(임의 중계 방지)
	allowModels = setOf("qwen2.5-3b-instruct", "qwen2.5-7b-instruct")
)

func env(k, d string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return d
}

type llmOut struct {
	Text  string `json:"text"`
	Level string `json:"level"` // 캐시 | 로컬 모델 | 규칙
	Model string `json:"model"`
	Ms    int64  `json:"ms"`
	Err   string `json:"err,omitempty"`
}

func cacheKey(kind, prompt string) string {
	h := sha1.Sum([]byte(kind + "\x00" + LLMModel + "\x00" + prompt))
	return filepath.Join(CacheDir, kind+"-"+hex.EncodeToString(h[:8])+".json")
}

// Chat 은 캐시 → 로컬 모델 순으로 시도한다. 둘 다 안 되면 Level 이 빈 값.
func Chat(kind, system, prompt string, maxTok int) llmOut {
	return ChatT(kind, system, prompt, maxTok, 12*time.Second)
}

// ChatT 는 Chat 과 같되 기다리는 시간을 정한다(답 전체 10초 예산 안에서 부른다).
func ChatT(kind, system, prompt string, maxTok int, timeout time.Duration) llmOut {
	ck := cacheKey(kind, system+"\n"+prompt)
	if b, err := os.ReadFile(ck); err == nil {
		var o llmOut
		if json.Unmarshal(b, &o) == nil && o.Text != "" {
			o.Level = "캐시"
			return o
		}
	}
	if os.Getenv("LLM_OFF") == "1" || !allowModels[LLMModel] {
		return llmOut{Err: "모델 꺼짐"}
	}
	t0 := time.Now()
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	body, _ := json.Marshal(map[string]any{
		"model": LLMModel, "temperature": 0, "max_tokens": maxTok, "seed": 7,
		"messages": []map[string]string{{"role": "system", "content": system}, {"role": "user", "content": prompt}},
	})
	req, _ := http.NewRequestWithContext(ctx, "POST", LLMURL, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return llmOut{Err: "로컬 모델 응답 없음"}
	}
	defer resp.Body.Close()
	var r struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if json.NewDecoder(resp.Body).Decode(&r) != nil || len(r.Choices) == 0 {
		return llmOut{Err: fmt.Sprintf("로컬 모델 오류(%d)", resp.StatusCode)}
	}
	o := llmOut{Text: strings.TrimSpace(r.Choices[0].Message.Content), Level: "로컬 모델", Model: LLMModel, Ms: time.Since(t0).Milliseconds()}
	os.MkdirAll(CacheDir, 0o755)
	if b, err := json.MarshalIndent(o, "", " "); err == nil {
		os.WriteFile(ck, b, 0o644)
	}
	return o
}

const sysExpand = `너는 제주데이터허브 검색 도우미다. 도민의 질문을 데이터 제목에 나올 법한 명사 키워드로 바꾼다.
규칙: 키워드 2~4개, 쉼표로만 구분, 설명 금지. 지역명(제주·서귀포)은 넣지 않는다.`

// Expand 는 모델이 낸 키워드를 돌려준다. 실패하면 빈 목록.
func Expand(q string) ([]string, llmOut) {
	o := Chat("expand", sysExpand, "질문: "+q, 40)
	if o.Text == "" {
		return nil, o
	}
	out := []string{}
	for _, w := range regexp.MustCompile(`[,\n·/]`).Split(o.Text, -1) {
		w = strings.Trim(strings.TrimSpace(w), `."'-*0123456789 `)
		if r := []rune(w); len(r) >= 2 && len(r) <= 10 && reWordFull.MatchString(w) && !stop[w] {
			out = append(out, w)
		}
		if len(out) == 4 {
			break
		}
	}
	return out, o
}

const sysWrite = `너는 제주 도민에게 데이터 분석 결과를 알려 주는 도우미다.
반드시 지킬 것:
1. 아래 [사실표]에 있는 숫자만 그대로 옮긴다. 새 숫자를 계산하거나 만들지 않는다. 숫자 표기(쉼표·단위)도 사실표 그대로 쓴다.
2. [기준 문장]이 이미 맞는 답이다. 뜻과 숫자를 바꾸지 말고 도민이 읽기 쉬운 존댓말 3문장으로 다듬는다.
3. 숫자 뒤 단위는 사실표에 있는 것만 쓴다. 사실표에 단위가 없으면 단위를 붙이지 않는다.
4. 대괄호 표시([주의] 등)나 「사실표」라는 말을 문장에 쓰지 않는다. 어려운 통계 용어를 쓰지 않는다.`

func writePrompt(q, title string, r *Result) string {
	var b strings.Builder
	fmt.Fprintf(&b, "[질문] %s\n[데이터] 「%s」\n[계산] %s · 측정: %s", q, title, r.OpLabel, r.Measure)
	if len(r.Filters) > 0 {
		fmt.Fprintf(&b, " · 조건: %s", strings.Join(r.Filters, ", "))
	}
	fmt.Fprintf(&b, " · 기간: %s\n[사실표]\n", r.Period)
	for _, f := range r.Facts {
		fmt.Fprintf(&b, "- %s: %s\n", f.Label, f.Value)
	}
	fmt.Fprintf(&b, "[기준 문장]\n%s\n", Template(title, r))
	b.WriteString("[주의]\n")
	if len(r.Notes) == 0 {
		b.WriteString("- 공개 통계를 그대로 더한 값이며, 원인까지 말해 주지는 않습니다.\n")
	}
	for _, n := range r.Notes {
		fmt.Fprintf(&b, "- %s\n", n)
	}
	return b.String()
}

var reNumUnit = regexp.MustCompile(`(\d[\d,]*(?:\.\d+)?)\s?(톤|명|호|대|ha|kg|원|천원|백만원|가구|세대|건|개|곳|배)`)

var reWordFull = regexp.MustCompile(`^[\p{Hangul}A-Za-z0-9]+$`)

var claimWords = []string{"전국", "보다", "가장", "제일", "최고", "최저", "급증", "급감", "평균", "높았", "낮았", "많았", "적었", "늘었", "줄었", "증가", "감소", "때문", "덕분", "전망"}

var reNum = regexp.MustCompile(`\d[\d,]*(?:\.\d+)?`)

// CheckNumbers 는 문장 속 숫자가 모두 사실표·질문에 있는지 본다. 없는 숫자 목록을 돌려준다.
func CheckNumbers(text, q string, r *Result) []string {
	allowed := map[string]bool{}
	facts := ""
	addAll := func(s string) {
		facts += " " + s
		for _, n := range reNum.FindAllString(s, -1) {
			allowed[strings.ReplaceAll(n, ",", "")] = true
			// 2023.06 같은 시점은 「2023년 6월」로 풀어 쓸 수 있다
			if m := reYM.FindStringSubmatch(n); m != nil {
				allowed[m[1]] = true
				allowed[strings.TrimLeft(m[2], "0")] = true
			}
		}
	}
	addAll(q)
	addAll(r.Period)
	for _, f := range r.Facts {
		addAll(f.Label + " " + f.Value)
	}
	for _, p := range r.Series {
		addAll(p.K)
	}
	for _, n := range r.Notes {
		addAll(n)
	}
	for _, f := range r.Filters {
		addAll(f)
	}
	addAll(r.Measure)
	bad := []string{}
	for _, n := range reNum.FindAllString(text, -1) {
		k := strings.ReplaceAll(n, ",", "")
		if !allowed[k] && k != "1" && k != "2" && k != "3" {
			bad = append(bad, n)
		}
	}
	// 숫자 뒤 단위가 사실표와 다르면(예: 사실표엔 단위가 없는데 「톤」을 붙임) 버린다
	for _, m := range reNumUnit.FindAllStringSubmatch(text, -1) {
		if !strings.Contains(strings.ReplaceAll(facts, " ", ""), m[1]+m[2]) {
			if !contains(bad, "단위 「"+m[2]+"」") {
				bad = append(bad, "단위 「"+m[2]+"」")
			}
		}
	}
	// 사실표에 없는 비교·평가 말(「전국보다」·「가장」·「급증」)을 덧붙이면 버린다
	// 질문 속 말은 근거가 아니다(「전국보다 높았어?」를 그대로 따라 쓰면 안 된다)
	ground := r.Measure + " " + r.Period + " " + strings.Join(r.Filters, " ")
	for _, f := range r.Facts {
		ground += " " + f.Label + " " + f.Value
	}
	for _, p := range r.Series {
		ground += " " + p.K
	}
	for _, w := range claimWords {
		if strings.Contains(text, w) && !strings.Contains(ground, w) {
			bad = append(bad, "덧붙인 말 「"+w+"」")
		}
	}
	if strings.Contains(text, "[") || strings.Contains(text, "사실표") {
		bad = append(bad, "형식 위반")
	}
	return bad
}

// Ping 은 로컬 모델이 살아 있는지 본다(1초).
func Ping() bool {
	c := &http.Client{Timeout: time.Second}
	u := strings.Replace(LLMURL, "/v1/chat/completions", "/health", 1)
	r, err := c.Get(u)
	if err != nil {
		return false
	}
	r.Body.Close()
	return r.StatusCode == 200
}
