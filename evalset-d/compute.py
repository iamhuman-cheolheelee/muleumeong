import sys, json
sys.path.insert(0,'/tmp/holdD/w')
from load import load
L=load; items=[]
def A(q,ds,value=None,unit=None,top=None,direction=None,tol=0):
    items.append(dict(q=q,type="answerable",datasets=[ds],answer=dict(value=value,unit=unit,top=top,direction=direction),tolerance_pct=tol))
def U(q): items.append(dict(q=q,type="unanswerable",datasets=[],answer=dict(value=None,unit=None,top=None,direction=None),tolerance_pct=0))
def dirn(a,b): return "증가" if b>a else "감소"

d=L(90001).set_index('기준연월')
A("2024년 12월 기준으로 제주에 사는 외국인 몇 명이었어?",90001,int(d.loc['2024-12','외국인']),"명")
A("제주 세대수 2019년 1월이랑 2025년 6월 비교하면 늘었어 줄었어?",90001,None,None,direction=dirn(d.loc['2019-01','세대수'],d.loc['2025-06','세대수']))
d=L(74).set_index('기준연월')
r=d.loc['2024-12']; A("2024년 말 제주시랑 서귀포시 합쳐서 65세 넘은 어르신 몇 명이야?",74,int(r['65세이상 제주시 인구']+r['65세이상 서귀포시 인구']),"명")
d=L(73); r=d.loc[d['65세 이상 인구'].idxmax()]
A("2025년 말 기준 65세 이상 인구가 제일 많은 읍면동은 어디야?",73,int(r['65세 이상 인구']),"명",top=r['읍면동'])
A("2025년 말 제주에 100세 넘은 분들 도 전체로 몇 명 계셔?",73,int(d['100세 이상 인구'].sum()),"명")
d=L(75).set_index('해당연도')
A("2025년 제주 0~4세 아기들 남녀 합치면 몇 명이야?",75,int(d.loc[2025,'0세부터 4세 남자']+d.loc[2025,'0세부터 4세 여자']),"명")
d=L(69); s=d.drop(columns=['행정시','성별','데이터기준일자']).sum()
A("제주에 등록된 외국인 중 어느 나라 사람이 제일 많아? (중국 한국계는 따로 치고)",69,int(s.max()),"명",top=s.idxmax())
A("서귀포시에 등록된 베트남 사람 남녀 합쳐서 몇 명이야?",69,int(d[d['행정시']=='서귀포시']['베트남'].sum()),"명")
d=L(70); d['t']=d['남자']+d['여자']; r=d.loc[d['t'].idxmax()]
A("제주 등록외국인(국적취득자 빼고) 중에 어느 연령대가 제일 많아?",70,int(r['t']),"명",top=r['연령구분'])
d=L(884).set_index('연도')
A("제주 합계출산율 2020년에 몇이었어?",884,float(d.loc[2020,'합계 출산율']),None,tol=1)
r=d['합계 출산율']; A("자료에 있는 해 중에 제주 합계출산율이 제일 높았던 해는 언제야?",884,float(r.max()),None,top=str(int(r.idxmax())),tol=1)
d=L(582).set_index('연도')
A("2025년에 제주 온 대만 관광객 몇 명이야?",582,int(d.loc[2025,'대만']),"명")
A("2024년에서 2025년 사이에 제주 찾은 내국인 관광객 늘었어 줄었어?",582,None,None,direction=dirn(d.loc[2024,'한국'],d.loc[2025,'한국']))
c=d.loc[2025,['일본','미국','대만','중국','홍콩','싱가포르','말레이시아']].astype(int)
A("2025년 제주 온 외국인 관광객 중에 어느 나라가 제일 많았어? (기타 제외)",582,int(c.max()),"명",top=c.idxmax())
d=L(1343); r=d[(d['년도']==2019)&(d['구분']=='입장객')].iloc[0]; m=[f'{i}월' for i in range(1,13)]
A("우도해양도립공원 2019년 한 해 입장객 총 몇 명이었어?",1343,int(r[m].astype(float).sum()),"명")
v=r[m].astype(float); A("2019년에 우도 입장객이 제일 많았던 달은?",1343,int(v.max()),"명",top=v.idxmax())
d=L(328); y=d[d['연도별']==2025]
A("2025년 제주공항 국내선 여객, 도착·출발 합쳐서 몇 명이야?",328,int(y['국내선 여객 (명)'].sum()),"명")
A("2025년 제주공항 국제선 운항 편수는 도착·출발 합쳐서 몇 편이었어?",328,int(y['국제선 운항 (편)'].sum()),"편")
g=d.groupby('연도별')['국제선 여객 (명)'].sum()
A("제주공항 국제선 승객(도착·출발 합계)이 가장 적었던 해는 언제야?",328,int(g.min()),"명",top=str(int(g.idxmin())))
d=L(307).set_index('연도')
A("2024년 제주종합터미널 이용객 몇 명이었어?",307,int(d.loc[2024,'제주종합터미널']),"명")
A("서귀포 시외버스터미널 이용객, 2013년이랑 2024년 비교하면 늘었어?",307,None,None,direction=dirn(d.loc[2013,'서귀포시외버스터미널'],d.loc[2024,'서귀포시외버스터미널']))
d=L(90002).set_index('연도별')
A("2025년 제주에서 교통사고 1년 동안 몇 건 났어?",90002,int(d.loc[2025].sum()),"건")
r=d.loc[2025]; A("2025년에 교통사고가 제일 적었던 달은 몇 월이야?",90002,int(r.min()),"건",top=r.idxmin().split()[0])
d=L(90003); g=d[d['관광지'].str.strip()=='국립제주박물관']
A("국립제주박물관 2019년 방문객, 내국인·외국인 합쳐서 얼마나 돼?",90003,int(g['2019년'].sum()),"명")
d=L(90004).set_index('연도')
A("2018년 제주 렌터카 등록대수 평균이 몇 대였어?",90004,int(d.loc[2018,'렌터카등록대수']),"대")
A("제주연구원 관광시장동향 기준으로 관광객이 제일 많았던 해는?",90004,int(d['관광객수'].max()),"명",top=str(int(d['관광객수'].idxmax())))
d=L(1332).set_index('해당연도')
A("2024년 제주 신선식품 물가지수(2020=100) 얼마였어?",1332,round(float(d.loc[2024,'신선식품']),1),None,tol=1)
d=L(1334).set_index('연도구분')
A("2024년에 제주로 귀농한 가구 몇 가구야?",1334,int(d.loc[2024,'귀농가구']),"가구")
A("제주 귀어가구 2023년보다 2024년에 늘었어?",1334,None,None,direction=dirn(d.loc[2023,'귀어가구'],d.loc[2024,'귀어가구']))
d=L(1324).set_index('연도')
A("2024년 제주 다문화가구 수 알려줘",1324,int(d.loc[2024,'다문화가구']),"가구")
d=L(1325).set_index('해당연도')
A("2023년 제주 비만율 몇 퍼센트였어?",1325,float(d.loc[2023,'비만율']),"%",tol=1)
A("제주 흡연율 2011년이랑 2024년 비교하면 어떻게 바뀌었어?",1325,None,None,direction=dirn(d.loc[2011,'흡연율'],d.loc[2024,'흡연율']))
d=L(978); r=d[(d['연도']==2015)&(d['구분']=='순이동')]
A("2015년에 제주로 순유입된 인구 몇 명이었어?",978,int(r['이동자 수(명)'].iloc[0]),"명")
d=L(193).set_index('년도')
A("2024년 말 서귀포시 노령화지수 얼마야?",193,float(d.loc[2024,'노령화지수']),None,tol=1)
d=L(319); g=d.groupby('해당연도')['관광객(명)'].sum()
A("2024년 제주에 국제크루즈 타고 온 관광객 총 몇 명이야?",319,int(g.loc[2024]),"명")
d=L(1295); c=['cheonjiyeon_pokpo','ceongbang_pokpo','jungmun_deapo','cheonjeyeon_pokpo','yongmeori_coast','citrus_museum']
nm=dict(zip(c,['천지연폭포','정방폭포','중문대포해안주상절리대','천제연폭포','용머리해안','감귤박물관']))
r=d[d['base_year_month']==202205][c].iloc[0]
A("2022년 5월 서귀포시 직영관광지 중 관람객 제일 많았던 곳은?",1295,int(r.max()),"명",top=nm[r.idxmax()])
d=L(1351).set_index('구분연도')
A("2024년 제주 실업률 몇 %였어?",1351,float(d.loc[2024,'실업률']),"%",tol=1)
d=L(900); r=d[(d['연월']=='2020. 12')&(d['시도']=='제주특별자치도')]
A("2020년 12월 한 달 동안 제주에서 아기 몇 명 태어났어?",900,int(r['출생아수(명)'].iloc[0]),"명")

for q in ["2030년 제주 인구는 몇 명쯤 될 것 같아?",
 "2025년 제주에서 교통사고로 돌아가신 분 몇 명이야?",
 "2024년 애월읍에 사는 외국인 몇 명이야?",
 "1985년 제주 합계출산율 알려줘",
 "우도 입장객 2026년 7월에 몇 명이었어?",
 "제주공항 2012년 국내선 여객 수 알려줘",
 "2024년 제주 렌터카 등록대수 평균 몇 대야?",
 "서귀포 시외버스터미널 2025년 이용객 몇 명이야?",
 "제주종합터미널 노선별로 이용객 제일 많은 노선이 어디야?",
 "2024년 제주 국제크루즈 관광객 중 국적별로 중국인 승객은 몇 명이야?",
 "제주 이주하면 어느 동네가 살기 제일 좋아?",
 "2023년 한림읍 다문화가구 수 알려줘",
 "제주시 연동에 사는 김철수 씨 나이가 몇 살이야?",
 "2024년 제주 공항 택시 대기시간 평균이 얼마나 돼?",
 "내년에 제주 물가 오를까?"]: U(q)
DROP={6,10,16,18,21,29,31,38}
items=[it for i,it in enumerate(items,1) if i not in DROP]
items=[{"id":f"D{i:02d}",**it} for i,it in enumerate(items,1)]
for it in items: print(it['id'],it['type'],it['datasets'],it['q'],it['answer'])
json.dump(items,open('/tmp/holdD/evalset.json','w'),ensure_ascii=False,indent=1,default=lambda o:o.item())
