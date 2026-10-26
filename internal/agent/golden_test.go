package agent

import (
	"fmt"
	"testing"
)

// go test ./internal/agent -run Golden -v
func TestGolden(t *testing.T) {
	if err := Load(); err != nil {
		t.Fatal(err)
	}
	g := Golden()
	for _, r := range g.Rows {
		fmt.Printf("%-24s 정답 위치 %d  1위 %s\n", r.Q, r.Pos, r.Top)
	}
	fmt.Printf("recall@1 %d/%d · recall@5 %d/%d\n", g.R1, g.N, g.R5, g.N)
	if g.R5 < g.N*7/10 {
		t.Errorf("recall@5 가 70%% 미만")
	}
}
