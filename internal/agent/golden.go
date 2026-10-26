package agent

// 찾기 정답표 — 도민 말투 질문 20개와 정답 데이터(사람이 제목·열을 보고 고름, 검색 규칙을 고친 뒤에 정답을 바꾸지 않는다).

type GoldRow struct {
	Q   string `json:"q"`
	Pos int    `json:"pos"` // 정답이 나온 순위(0 = 5위 안에 없음)
	Top string `json:"top"`
}

type GoldReport struct {
	N    int       `json:"n"`
	R1   int       `json:"r1"`
	R5   int       `json:"r5"`
	Rows []GoldRow `json:"rows"`
}

var golden = []struct {
	Q  string
	OK []int
}{
	{"서귀포 귤 얼마나 나와요", []int{1294, 1313}},
	{"천지연폭포 손님 몇 명 와요", []int{1295}},
	{"꽃 키우는 농가 다른 지역이랑 비교", []int{1364}},
	{"요즘 장사 경기 어때요", []int{1358}},
	{"청년 일자리 사정", []int{1351, 1322}},
	{"아기 몇 명 태어났어요", []int{900}},
	{"제주 차 몇 대 등록돼 있어요", []int{915, 779, 324}},
	{"동네 소음 시끄러운 정도", []int{1352, 144}},
	{"시내버스 노선 알려줘", []int{1315}},
	{"생활 쓰레기 얼마나 나와요", []int{1345, 1034, 840}},
	{"제주 미세먼지 월별", []int{931, 489}},
	{"교차로 교통사고", []int{1028}},
	{"숙박업소 몇 곳 있어요", []int{249, 837}},
	{"제주 물가 오르는 중이에요", []int{1332}},
	{"감염병 얼마나 걸려요", []int{896}},
	{"중학교 몇 개", []int{1010}},
	{"수산물 수출 순위", []int{1357, 1356}},
	{"공항 이용객 통계", []int{787, 785}},
	{"렌터카 어디 오래 머무나", []int{1216, 1215, 1214, 1213}},
	{"주유소 기름값 평균", []int{893, 894}},
}


func Golden() GoldReport {
	g := GoldReport{N: len(golden), Rows: []GoldRow{}}
	for _, x := range golden {
		hits, _ := Search(x.Q, 5, nil)
		row := GoldRow{Q: x.Q}
		for i, h := range hits {
			for _, ok := range x.OK {
				if h.ID == ok && row.Pos == 0 {
					row.Pos = i + 1
				}
			}
		}
		if len(hits) > 0 {
			row.Top = hits[0].Title
		}
		if row.Pos == 1 {
			g.R1++
		}
		if row.Pos > 0 {
			g.R5++
		}
		g.Rows = append(g.Rows, row)
	}
	return g
}
