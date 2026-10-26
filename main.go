// 물으멍 — 도민이 묻는 말로 제주데이터허브 원본을 찾아 계산하고, 숫자마다 출처를 붙여 답하는 AI 에이전트.
// Go(chi + huma) 단일 바이너리가 API 와 Vite 빌드(web/dist)를 함께 서빙한다.
package main

import (
	"context"
	"embed"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"
	"sort"
	"strings"
	"sync"

	"muleumeong/internal/agent"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humachi"
	"github.com/go-chi/chi/v5"
)

//go:embed all:web/dist
var dist embed.FS

// 화면을 열면 바로 보이는 예시 질문(결과는 시작할 때 미리 계산해 둔다)
var Seeds = []string{
	"서귀포시 감귤 생산량은 몇 년 사이 어떻게 변했나요?",
	"서귀포 직영관광지 중 관람객이 가장 많은 곳은 어디예요?",
	"제주 화훼 농가 수는 다른 시도와 비교하면 어느 정도예요?",
	"요즘 제주 소상공인 체감경기는 어때요?",
	"제주 청년 고용률은 어떻게 변했나요?",
	"제주 출생아 수는 해마다 어떻게 바뀌었나요?",
	"제주도 자동차 등록 대수는 차종별로 어디가 가장 많아요?",
}

func env(k, d string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return d
}

type AskIn struct {
	Q   string `query:"q" maxLength:"200"`
	Pin int    `query:"pin"`
}

type Quality struct {
	Total, Open, API, Login int
	ByFile                  map[string]int
	Scanned                 int
	CP949, UTF8, XlsxInCsv  int
	WithTotal, HTMLPrev     int
	Examples                map[string][]agent.Hit
	ByCat                   [][2]any
}

var (
	qOnce sync.Once
	qual  Quality
	gold  agent.GoldReport
	seedA []*agent.Answer
)

func scan() {
	q := Quality{ByFile: map[string]int{}, Examples: map[string][]agent.Hit{}}
	cat := map[string]int{}
	add := func(k string, d *agent.Dataset) {
		if len(q.Examples[k]) < 5 {
			q.Examples[k] = append(q.Examples[k], agent.Hit{ID: d.ID, Title: d.Title, Cat: d.Cat, Owner: d.Owner, URL: d.URL()})
		}
	}
	for _, d := range agent.Catalog {
		if d.ID >= 90000 {
			continue
		}
		q.Total++
		cat[d.Cat]++
		if d.API {
			q.API++
		}
		if d.Auth {
			q.Login++
		}
		if d.File != nil {
			n := strings.ToLower(d.File.FileName)
			q.ByFile[n[strings.LastIndex(n, ".")+1:]]++
		}
		if d.Open() {
			q.Open++
		}
		b, ok := agent.Preview(d.ID)
		if !ok {
			continue
		}
		q.Scanned++
		if strings.HasPrefix(strings.TrimSpace(string(b[:min(len(b), 64)])), "<") {
			q.HTMLPrev++
			add("html", d)
			continue
		}
		t, err := agent.ParseTable(b, "x.csv")
		if err != nil {
			continue
		}
		switch {
		case strings.Contains(t.Format, "엑셀"):
			q.XlsxInCsv++
			add("xlsx", d)
		case strings.Contains(t.Format, "CP949"):
			q.CP949++
			add("cp949", d)
		default:
			q.UTF8++
		}
		cols := agent.Profile(t, agent.ColDict)
		if len(agent.TotalRows(t, cols)) > 0 {
			q.WithTotal++
			add("total", d)
		}
	}
	for _, k := range func() []string {
		ks := []string{}
		for k := range cat {
			ks = append(ks, k)
		}
		sort.Slice(ks, func(a, b int) bool { return cat[ks[a]] > cat[ks[b]] })
		return ks
	}() {
		q.ByCat = append(q.ByCat, [2]any{k, cat[k]})
	}
	qual = q
}

func main() {
	if err := agent.Load(); err != nil {
		log.Fatal(err)
	}
	scan()
	gold = agent.Golden()
	for _, s := range Seeds {
		seedA = append(seedA, agent.Ask(s, 0))
	}

	r := chi.NewRouter()
	hcfg := huma.DefaultConfig("물으멍 API", "1.0.0")
	hcfg.CreateHooks = nil // $schema 링크(호스트 이름)를 응답에 싣지 않는다
	api := humachi.New(r, hcfg)
	huma.Get(api, "/api/health", func(ctx context.Context, _ *struct{}) (*struct{ Body map[string]any }, error) {
		return &struct{ Body map[string]any }{map[string]any{"ok": true, "datasets": len(agent.Catalog), "model": agent.LLMModel, "llm": agent.Ping()}}, nil
	})
	huma.Get(api, "/api/seeds", func(ctx context.Context, _ *struct{}) (*struct{ Body []*agent.Answer }, error) {
		return &struct{ Body []*agent.Answer }{seedA}, nil
	})
	huma.Get(api, "/api/ask", func(ctx context.Context, in *AskIn) (*struct{ Body *agent.Answer }, error) {
		for _, a := range seedA {
			if a.Q == strings.TrimSpace(in.Q) && in.Pin == 0 {
				return &struct{ Body *agent.Answer }{a}, nil
			}
		}
		return &struct{ Body *agent.Answer }{agent.Ask(in.Q, in.Pin)}, nil
	})
	huma.Get(api, "/api/search", func(ctx context.Context, in *AskIn) (*struct{ Body map[string]any }, error) {
		h, t := agent.Search(in.Q, 20, nil)
		return &struct{ Body map[string]any }{map[string]any{"hits": h, "terms": t}}, nil
	})
	huma.Get(api, "/api/golden", func(ctx context.Context, _ *struct{}) (*struct{ Body agent.GoldReport }, error) {
		return &struct{ Body agent.GoldReport }{gold}, nil
	})
	huma.Get(api, "/api/quality", func(ctx context.Context, _ *struct{}) (*struct{ Body Quality }, error) {
		return &struct{ Body Quality }{qual}, nil
	})
	huma.Get(api, "/api/dataset/{id}", func(ctx context.Context, in *struct {
		ID int `path:"id"`
	}) (*struct{ Body map[string]any }, error) {
		d := agent.Get(in.ID)
		if d == nil {
			return nil, huma.Error404NotFound("없는 데이터")
		}
		out := map[string]any{"dataset": d, "url": d.URL(), "open": d.Open()}
		if b, ok := agent.Preview(d.ID); ok {
			if t, err := agent.ParseTable(b, "x.csv"); err == nil {
				cols := agent.Profile(t, agent.ColDict)
				out["format"], out["cols"], out["totalRows"], out["rows"] = t.Format, cols, len(agent.TotalRows(t, cols)), len(t.Rows)
			}
		}
		return &struct{ Body map[string]any }{out}, nil
	})

	sub, _ := fs.Sub(dist, "web/dist")
	files := http.FileServer(http.FS(sub))
	r.Get("/*", func(w http.ResponseWriter, q *http.Request) {
		p := strings.TrimPrefix(q.URL.Path, "/")
		if p != "" {
			if _, err := fs.Stat(sub, p); err == nil {
				if strings.HasPrefix(p, "assets/") {
					w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
				}
				files.ServeHTTP(w, q)
				return
			}
		}
		b, _ := fs.ReadFile(sub, "index.html")
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write(b)
	})
	addr := env("ADDR", "127.0.0.1:18164")
	log.Print(fmt.Sprintf("물으멍 데이터 %d종 · http://%s", len(agent.Catalog), addr))
	log.Fatal(http.ListenAndServe(addr, r))
}
