package agent

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"testing"
)

func TestDbg(t *testing.T) {
	if err := Load(); err != nil {
		t.Fatal(err)
	}
	ids := []int{}
	for _, s := range strings.Split(os.Getenv("IDS"), ",") {
		if v, err := strconv.Atoi(s); err == nil {
			ids = append(ids, v)
		}
	}
	for _, q := range strings.Split(os.Getenv("Q"), "|") {
		qt := ParseQTime(q)
		fmt.Printf("Q %s | kind=%s %d-%d g=%d refuse=%s\n", q, qt.Kind, qt.S, qt.E, qt.G, qt.Refuse)
		for _, id := range ids {
			lt, err := LoadLT(id)
			if err != nil {
				fmt.Println(id, err)
				continue
			}
			g := GroundQ(qt.Blanked, lt)
			occ := []string{}
			for _, o := range g.Occs {
				occ = append(occ, fmt.Sprintf("%s/%d", string(o.p.text), o.p.kind))
			}
			fmt.Printf("  %d score=%d miss=%v anchor=%v occ=%v gran=%d rows=%d\n", id, g.Score, g.Miss, g.Anchor, occ, lt.Gran, len(lt.Rows))
			if os.Getenv("V") != "" {
				for i, d := range lt.Dims {
					vs := lt.Vals[i]
					if len(vs) > 12 {
						vs = vs[:12]
					}
					fmt.Printf("    dim %s %v\n", d.Label, vs)
				}
			}
		}
		a := Ask(q, 0)
		fmt.Println("  =>", a.Text)
	}
}
