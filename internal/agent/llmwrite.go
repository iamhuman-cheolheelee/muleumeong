package agent

// 로컬 모델(Qwen2.5-3B)을 두 군데에만 쓴다. 숫자는 언제나 코드가 계산한다.
//  ① 답 문장 다듬기: 코드가 만든 규칙 문장을 모델이 자연스러운 존댓말로 고쳐 쓰고,
//     숫자·단위·지표·대상·시점·방향 말이 하나라도 달라지면 버리고 규칙 문장을 쓴다.
//  ② 낱말 맞추기: 질문 낱말 1~2개가 열 사전에 없어 거절할 때만, 그 데이터의 사전 낱말 목록(닫힌 목록)
//     중에서 같은 뜻을 고르게 한다. 목록 밖 말·NONE 이면 거절을 유지하고, 고른 말로 바꾼 질문을
//     같은 결정론 경로로 다시 푼다.

import (
	"log"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"
)

const WriterLLM = "Qwen2.5-3B (검산 통과)"

// 전체 답을 10초 안에 낸다. 모델 문장은 이 예산 안에서만 기다린다.
const answerBudget = 8 * time.Second

const sysPhrase = `너는 제주 도민에게 공공데이터 계산 결과를 알려 주는 도우미다.
[기준 문장]은 코드가 계산한 정답이다. 이것을 질문에 바로 답하는 자연스러운 존댓말 1~2문장으로 고쳐 쓴다.
반드시 지킬 것:
1. 기준 문장의 숫자·단위·연도를 하나도 빼거나 바꾸지 않고 그대로 옮긴다. 새 숫자를 만들지 않는다. 숫자 표기(쉼표·소수점)도 그대로.
2. 「」 안의 이름, 지표 이름, 대상(지역·항목), 시점을 그대로 쓴다.
3. 기준 문장에 없는 비교·평가·원인·전망을 덧붙이지 않는다.
4. 답 문장만 쓴다. 따옴표·머리말·설명을 붙이지 않는다.`

var (
	reNumU    = regexp.MustCompile(`\d[\d,]*(?:\.\d+)?[%\p{Hangul}A-Za-z]?`)
	reQuote   = regexp.MustCompile(`「([^」]+)」`)
	dirWords  = []string{"늘", "줄", "높아", "낮아", "올랐", "내렸", "증가", "감소"}
	reForeign = regexp.MustCompile(`[\p{Han}\p{Hiragana}\p{Katakana}]`)
	reLatin   = regexp.MustCompile(`[A-Za-z]+`)
	reAwkward = regexp.MustCompile(`\)요|입니다\([^)]*\)\s*(입니다|에|와|과|이고|이며)|요\.\s*요`)
	reNoSpace = strings.NewReplacer(" ", "", " ", "", "'", "", "\"", "")
)

func squash(s string) string { return reNoSpace.Replace(s) }

// phraseCheck 는 모델 문장이 규칙 문장의 사실을 그대로 지켰는지 본다. 어긋난 점 목록을 돌려준다.
func phraseCheck(rule, out string, r *Result) []string {
	bad := []string{}
	so, sr := squash(out), squash(rule)
	// 숫자: 두 문장의 숫자 집합이 같아야 한다(새 숫자도, 빠진 숫자도 안 된다)
	rn, on := map[string]bool{}, map[string]bool{}
	for _, n := range reNum.FindAllString(rule, -1) {
		rn[strings.ReplaceAll(n, ",", "")] = true
	}
	for _, n := range reNum.FindAllString(out, -1) {
		k := strings.ReplaceAll(n, ",", "")
		on[k] = true
		if !rn[k] {
			bad = append(bad, "새 숫자 "+n)
		}
	}
	for k := range rn {
		if !on[k] {
			bad = append(bad, "빠진 숫자 "+k)
		}
	}
	// 숫자+단위 꼴(3,358건·2019년)이 그대로 있어야 한다
	for _, t := range reNumU.FindAllString(rule, -1) {
		if !strings.Contains(so, t) {
			bad = append(bad, "단위 「"+t+"」")
		}
	}
	// 이름·지표·대상·1위 항목
	need := []string{}
	for _, m := range reQuote.FindAllStringSubmatch(rule, -1) {
		need = append(need, m[1])
	}
	if r.Measure != "" && strings.Contains(sr, squash(r.Measure)) {
		need = append(need, r.Measure)
	}
	for _, f := range r.Filters {
		v := f
		if i := strings.LastIndex(f, "="); i >= 0 {
			v = f[i+1:]
		}
		if strings.Contains(sr, squash(v)) {
			need = append(need, v)
		}
	}
	for _, f := range r.Facts {
		if f.Key == "top" && strings.Contains(sr, squash(f.Value)) {
			need = append(need, f.Value)
		}
	}
	for _, w := range need {
		if w = squash(w); w != "" && !strings.Contains(so, w) {
			bad = append(bad, "빠진 말 「"+w+"」")
		}
	}
	// 방향 말(늘었다·줄었다)은 규칙 문장과 같아야 한다
	for _, w := range dirWords {
		if strings.Contains(out, w) != strings.Contains(rule, w) {
			bad = append(bad, "방향 말 「"+w+"」")
		}
	}
	// 규칙 문장에 없는 비교·평가 말
	for _, w := range claimWords {
		if strings.Contains(out, w) && !strings.Contains(rule, w) {
			bad = append(bad, "덧붙인 말 「"+w+"」")
		}
	}
	// 한국어 밖 글자(중국어·일본어)와 기준 문장에 없는 영문 낱말은 안 된다
	if reForeign.MatchString(out) && !reForeign.MatchString(rule) {
		bad = append(bad, "한국어 밖 글자")
	}
	for _, w := range reLatin.FindAllString(out, -1) {
		if !strings.Contains(strings.ToLower(rule), strings.ToLower(w)) {
			bad = append(bad, "새 영문 「"+w+"」")
			break
		}
	}
	// 「…입니다(814,491명)에 이어」「(798건)요」처럼 어색한 이음
	if reAwkward.MatchString(out) && !reAwkward.MatchString(rule) {
		bad = append(bad, "어색한 문장 이음")
	}
	if strings.ContainsAny(out, "[]#*\n") || strings.Contains(out, "기준 문장") {
		bad = append(bad, "형식 위반")
	}
	if n := len([]rune(out)); n < 8 || n > 2*len([]rune(rule))+60 {
		bad = append(bad, "길이")
	}
	return bad
}

// llmPhrase 는 규칙 문장을 모델이 다듬게 하고, 검산을 통과한 문장만 돌려준다.
func llmPhrase(q, title string, r *Result, rule string, left time.Duration) (string, string) {
	if left < 1500*time.Millisecond {
		return "", "시간 예산이 남지 않아 규칙 문장"
	}
	if left > 6*time.Second {
		left = 6 * time.Second
	}
	var b strings.Builder
	fmt.Fprintf(&b, "[질문] %s\n[데이터] 「%s」\n[지표] %s\n", q, title, r.Measure)
	if len(r.Filters) > 0 {
		fmt.Fprintf(&b, "[대상] %s\n", strings.Join(r.Filters, ", "))
	}
	if r.Period != "" {
		fmt.Fprintf(&b, "[시점] %s\n", r.Period)
	}
	fmt.Fprintf(&b, "[기준 문장] %s\n", rule)
	o := ChatT("phrase", sysPhrase, b.String(), 200, left)
	if o.Text == "" {
		if o.Err == "" {
			o.Err = "모델 응답 없음"
		}
		return "", o.Err + " → 규칙 문장"
	}
	out := strings.Trim(clean(o.Text), `"' `)
	if bad := phraseCheck(rule, out, r); len(bad) > 0 {
		if len(bad) > 3 {
			bad = bad[:3]
		}
		return "", "AI 문장이 계산 결과와 어긋나(" + strings.Join(bad, ", ") + ") 버리고 규칙 문장을 썼습니다"
	}
	src := "로컬 모델 " + fmt.Sprint(o.Ms) + "ms"
	if o.Level == "캐시" {
		src = "모델 응답 캐시"
	}
	return out, fmt.Sprintf("PC 안의 소형 AI가 다듬은 문장%.0s — 숫자 %d개·단위·지표·대상·시점·방향 말이 코드가 쓴 기준 문장과 모두 일치", src, len(reNum.FindAllString(out, -1)))
}

// ── ② 낱말 맞추기 ──

const sysMap = `너는 도민 질문의 낱말 하나를 데이터 사전 낱말로 옮기는 도우미다.
[낱말]과 뜻이 똑같은 말(다른 표기·줄임말·입말)을 [목록]에서 하나 골라 목록에 적힌 그대로 쓴다.
뜻이 똑같은 말이 목록에 없거나, 더 넓은 말·더 좁은 말·비슷하기만 한 말뿐이면 NONE 이라고 쓴다. 헷갈리면 NONE.
고른 말 하나만 쓴다. 설명 금지.
예시)
[질문] 작년 신생아 몇 명? [낱말] 신생아 [목록] 출생아 수, 사망자 수, 혼인 건수 → 출생아 수
[질문] 우리 동네 해녀 몇 명? [낱말] 해녀 [목록] 어업 가구, 어선 수, 수산물 → NONE
[질문] 전기 버스 몇 대? [낱말] 전기 [목록] 버스, 승합, 화물 → NONE
[질문] 경유차 몇 대? [낱말] 경유차 [목록] 승용, 화물, 휘발유, 경유 → 경유
[질문] 오름 탐방객 몇 명? [낱말] 탐방객 [목록] 오름 이름, 면적, 높이 → NONE`

const sysSame = `두 낱말이 이 질문 안에서 정확히 같은 대상을 가리키면 「예」, 하나라도 더 넓거나 좁거나 다른 대상이면 「아니오」만 쓴다.
예시) 「신생아」와 「출생아 수」 → 예 / 「전기」와 「화물」 → 아니오 / 「차령」와 「승용」 → 아니오 / 「해녀」와 「어업 가구」 → 아니오`

// dropable 은 빼도 되는 꼴(동사·말끝)인가. 명사(해녀·쌍)는 빼지 못한다.
func dropable(w string) bool {
	if isPredicate(w) {
		return true
	}
	r := []rune(w)
	c := r[len(r)-1]
	if c >= 0xAC00 && c <= 0xD7A3 {
		jong := (c - 0xAC00) % 28
		if jong == 4 && len(r) >= 2 { // 빌린·올린·남긴·찾아본(관형형 ㄴ)
			return true
		}
	}
	return strings.HasSuffix(w, "어") || strings.HasSuffix(w, "고") && len(r) >= 3
}

// unitAfterMyeot 은 「몇 쌍」「얼마어치」처럼 세는 단위 자리인가.
func unitAfterMyeot(q, w string) bool {
	return strings.Contains(q, "몇 "+w) || strings.Contains(q, "몇"+w) || strings.Contains(q, "얼마"+w)
}

type nearPick struct {
	lt    *LT
	miss  []string
	score int
}

// nearMisses 는 낱말 1~2개만 닿지 않은 데이터 중 가장 가까운 것들을 돌려준다.
func nearMisses(qt QTime) []nearPick {
	out := []nearPick{}
	for _, id := range SchemaIDs() {
		lt, err := LoadLT(id)
		if err != nil || len(lt.Rows) == 0 || lt.D == nil {
			continue
		}
		g := GroundQ(qt.Blanked, lt)
		if len(g.Anchor) == 0 || len(g.Miss) == 0 || len(g.Miss) > 2 {
			continue
		}
		out = append(out, nearPick{lt, g.Miss, g.Score*10 - len(g.Miss)})
	}
	sort.SliceStable(out, func(a, b int) bool { return out[a].score > out[b].score })
	if len(out) == 0 {
		return out
	}
	// 가장 가까운 점수와 같은 것만(최대 2개)
	k := 1
	for k < len(out) && k < 2 && out[k].score == out[0].score {
		k++
	}
	return out[:k]
}

func vocabFor(lt *LT, miss []string) []string {
	seen := map[string]bool{}
	pri, rest := []string{}, []string{}
	mr := map[rune]bool{}
	for _, w := range miss {
		for _, c := range w {
			mr[c] = true
		}
	}
	for _, p := range lt.phrases() {
		if p.kind == pPlus || p.kind == pRegion {
			continue
		}
		t := string(p.text)
		if len(p.text) < 2 || seen[t] || reNum.MatchString(t) && len(reNum.FindString(t)) == len(t) {
			continue
		}
		seen[t] = true
		if p.kind != pVal {
			pri = append(pri, t)
			continue
		}
		share := false
		for _, c := range p.text {
			if mr[c] {
				share = true
			}
		}
		if share {
			pri = append(pri, t)
		} else {
			rest = append(rest, t)
		}
	}
	all := append(pri, rest...)
	if len(all) > 120 {
		all = all[:120]
	}
	return all
}

// llmMap 은 없는 낱말마다 목록 말·DROP 을 받아 바꾼 질문을 돌려준다. 하나라도 NONE·목록 밖이면 "".
func llmMap(q string, np nearPick, left time.Duration) (string, []string, string) {
	if left < 1500*time.Millisecond {
		return "", nil, "시간 예산 없음"
	}
	if left > 4*time.Second {
		left = 4 * time.Second
	}
	vocab := vocabFor(np.lt, np.miss)
	vs := setOf(vocab...)
	words := []string{}
	for _, w := range np.miss {
		s := stripJosa(w)
		if s == "" {
			s = w
		}
		words = append(words, s)
	}
	nq := q
	maps := []string{}
	t0 := time.Now()
	for i, w := range words {
		prompt := fmt.Sprintf("[질문] %s\n[데이터] 「%s」\n[낱말] %s\n[목록] %s\n", q, np.lt.D.Title, w, strings.Join(vocab, ", "))
		o := ChatT("map1", sysMap, prompt, 24, left-time.Since(t0))
		v := strings.TrimSpace(strings.Split(o.Text, "\n")[0])
		if j := strings.LastIndex(v, "="); j >= 0 {
			v = v[j+1:]
		}
		v = strings.Trim(strings.TrimSpace(v), "「」\"'.* ")
		if v == "" || strings.EqualFold(v, "NONE") {
			return "", nil, "「" + w + "」에 맞는 말이 목록에 없음"
		}
		src := np.miss[i]
		if !strings.Contains(nq, src) {
			src = w
		}
		if !strings.Contains(nq, src) {
			return "", nil, "질문에서 낱말 위치를 못 찾음"
		}
		if strings.EqualFold(v, "DROP") {
			return "", nil, "「" + w + "」 빼기는 허용하지 않음"
		}
		if !vs[v] || reNum.MatchString(v) || v == w {
			return "", nil, "목록 밖 말 「" + v + "」"
		}
		if strings.Contains(squash(q), squash(v)) {
			return "", nil, "「" + v + "」는 이미 질문에 있는 말"
		}
		// 같은 뜻인지 한 번 더 묻는다(아니면 거절 유지)
		vo := ChatT("map2", sysSame, fmt.Sprintf("[질문] %s\n「%s」와 「%s」", q, w, v), 4, left-time.Since(t0))
		if !strings.HasPrefix(strings.TrimSpace(vo.Text), "예") {
			return "", nil, "「" + w + "」≠「" + v + "」(재확인 실패)"
		}
		nq = strings.Replace(nq, src, v, 1)
		maps = append(maps, "「"+w+"」→「"+v+"」")
	}
	return nq, maps, ""
}

// tryRewrite 는 거절 직전에 낱말 맞추기를 해 본다. 고친 질문이 하나의 데이터로만 풀릴 때만 그 답을 쓴다.
func tryRewrite(q string, qt QTime, extra []string, T time.Time) *Answer {
	nps := nearMisses(qt)
	var won *Answer
	var wonID int
	for _, np := range nps {
		nq, maps, why := llmMap(q, np, answerBudget-time.Since(T))
		log.Printf("llm_map %q [%d] miss=%v -> %q %v %s", q, np.lt.D.ID, np.miss, nq, maps, why)
		if nq == "" {
			continue
		}
		b := &Answer{Q: q, Steps: []Step{}, Skipped: []string{}, Intent: ParseIntent(nq)}
		if b.Intent.Refuse != "" {
			continue
		}
		askEngineD(b, nq, extra, T, 1)
		log.Printf("llm_map rerun refused=%v err=%s", b.Refused, b.Error)
		if b.Refused || b.Chosen == nil || b.Chosen.ID != np.lt.D.ID {
			continue
		}
		if won != nil && wonID != b.Chosen.ID {
			return nil // 두 데이터로 서로 다르게 풀린다 — 되묻지 않고 거절을 유지
		}
		b.Steps = append([]Step{{"llm_map", "AI 낱말 맞추기", "열 사전에 없던 낱말을 이 데이터의 낱말 목록에서 골라 바꿈: " + strings.Join(maps, ", ") + " → 바꾼 질문 「" + strings.Join(strings.Fields(nq), " ") + "」을 같은 계산 경로로 다시 풂", 0, "ok"}}, b.Steps...)
		if b.Result != nil {
			note := "질문의 " + strings.Join(maps, ", ") + " 로 읽었어요(AI 낱말 맞추기)"
			b.Result.Notes = append(b.Result.Notes, note)
			b.Text += " (" + note + ")"
		}
		won, wonID = b, b.Chosen.ID
	}
	return won
}
