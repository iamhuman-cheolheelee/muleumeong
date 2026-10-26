"""Evalset H: compute ground truth from raw files and write evalset.json."""
import json, os, pandas as pd
from load import load, B
H=os.path.dirname(os.path.abspath(__file__))
def rv(i): return pd.read_csv(B+f'{i}.bin',header=None,encoding='utf-8-sig')
def r(x,n=2): return round(float(x),n)
def trend(s):
    s=list(s); return 'up' if s[-1]>s[0] else 'down' if s[-1]<s[0] else 'flat'
A=[]
def add(q,ds,v,unit,top=None,direction=None):
    A.append(dict(q=q,type='answerable',datasets=[ds],answer=dict(value=v,unit=unit,top=top,direction=direction),tolerance_pct=0))

d=load(900); j=d[d.시도.str.contains('제주')]
add('2020년 12월 한 달 동안 제주에서 태어난 아기는 몇 명이야?',900,int(j[j.연월=='2020. 12']['출생아수(명)'].iloc[0]),'명')
add('2019년 1년 동안 제주도에서 혼인신고가 총 몇 건 있었어? 월별 합계로 알려줘',900,int(j[j.연월.str.startswith('2019')]['혼인건수(건)'].sum()),'건')
d=load(915); x=d[(d.연월==202009)&(d.차종=='승용')].iloc[0]
add('2020년 9월 기준 제주에 등록된 승용차는 사업용, 비사업용 합쳐서 몇 대야?',915,int(x['등록수(계)']),'대')
add('2020년 9월 제주 승용 전기차는 모두 몇 대 등록돼 있었어?',915,int(x['전기차수(소계)']),'대')
d=load(1294)
add('2021년 남원읍 노지 조생 온주밀감 생산량이 얼마나 돼?',1294,float(d[(d.base_year==2021)&(d.district=='남원읍')&(d.variety=='노지온주(조생)')].production_amount.iloc[0]),'톤')
g=d[d.base_year==2021].groupby('district').extent.sum()
add('서귀포시에서 2021년에 감귤 재배면적(전 품종 합계)이 제일 넓은 읍면동은 어디야?',1294,r(g.max(),1),'ha',top=g.idxmax())
d=load(1295)
add('서귀포시가 직영하는 천지연폭포에 2019년 한 해 동안 관람객이 몇 명 다녀갔어?',1295,int(d[d.base_year_month//100==2019].cheonjiyeon_pokpo.sum()),'명')
add('2022년 5월 감귤박물관 관람객 수 알려줘',1295,int(d[d.base_year_month==202205].citrus_museum.iloc[0]),'명')
d=load(1351).set_index('구분연도')
add('작년 말 기준 자료에서 2024년 제주 실업률은 몇 퍼센트였어?',1351,float(d.loc[2024,'실업률']),'%')
add('제주 취업자 수가 2020년에서 2022년 사이에 늘었어, 줄었어?',1351,None,'명',direction=trend(d.loc[2020:2022,'취업 인구']))
add('2023년 제주 청년고용률 얼마였지?',1351,float(d.loc[2023,'청년고용률']),'%')
d=load(1363)
add('2023년 제주에서 참깨는 몇 톤 생산됐어?',1363,float(d[(d['기준 연도']==2023)&(d.시도=='제주특별자치도')&(d.품명=='참깨')]['생산량[M/T]'].iloc[0]),'M/T')
d=load(1364); j=d[d.시도별=='제주']
add('2023년 제주 절화류(꽃) 판매액이 얼마야?',1364,int(j[(j['기준 연도']==2023)&(j.구분별=='절화류')]['판매액 (천원)'].iloc[0]),'천원')
s=j[j.구분별=='화목류'].sort_values('기준 연도')['판매액 (천원)'].astype(int)
add('제주 화목류 판매액은 2018년부터 2023년까지 어떤 흐름이야?',1364,None,'천원',direction=trend(s))
d=load(1352)
add('2024년 서귀포시 환경소음 측정에서 낮 12시 소음도 평균은 몇 데시벨이야?',1352,r(d[d.행정시=='서귀포시']['낮시간대 소음도 (12시)'].astype(float).mean()),'dB')
d=load(1350); x=d[(d.통계일=='2025-07-23')&(d.성별=='전체')&(d.연령대=='전체')].sort_values('점수')
add('비짓제주 검색어 트렌드에서 2025년 7월 23일 전체 이용자 기준 점수 1위 키워드는 뭐야?',1350,r(x.점수.iloc[-1]),'점',top=x.키워드.iloc[-1])
d=load(1357); x=d[(d.기준년월=='Jun-25')&(d.수출입구분명=='수출')].sort_values('당월수출입미화금액(달러)')
add('2025년 6월 한 달 우리나라 수산물 수출액이 가장 많았던 나라는 어디고 얼마야?',1357,int(x['당월수출입미화금액(달러)'].iloc[-1]),'달러',top=x.국가명.iloc[-1])
d=load(1356); x=d[(d.기준년월=='Jun-25')&(d.수출입구분명=='수출')]
add('2025년 6월 해양수산부 품목별 통계에서 수산물 수출 금액을 모든 품목 합치면 몇 달러야?',1356,int(x['당월수출입미화금액(달러)'].sum()),'달러')
d=load(1358)
add('2024년 12월 제주 소상공인 체감 경기지수(BSI)는 몇이었어?',1358,float(d[(d.연도==2024)&(d.월==12)].제주체감.iloc[0]),'지수')
d=load(1353)
if 0: add('2025년 녹차 TRQ(할당관세) 실수요자 배정에서 낙찰된 물량은 얼마야?',1353,float(d[(d.년도==2025)&(d.품목군명=='녹차')].낙찰물량.iloc[0]),'kg')
d=load(877); d['Date']=pd.to_datetime(d.Date)
add('제주 2020년 일평균기온을 1년 평균 내면 몇 도야?',877,r(d[d.Date.dt.year==2020].Tem_mean.mean()),'℃')
x=d[(d.Date.dt.year==2020)&(d.Date.dt.month==8)]
add('2020년 8월 제주에서 가장 더웠던 날 최고기온은 몇 도였어?',877,float(x.Tem_max.max()),'℃')
d=load(875)
add('어린이 교통안전 데이터에서 위험 지점이 가장 많이 잡힌 초등학교는 어디고 몇 곳이야?',875,int(d.학교명.value_counts().iloc[0]),'곳',top=d.학교명.value_counts().index[0])
d=load(872)
add('제주 공시대상 국공립 유치원 중에 특수학급이 있는 곳은 몇 군데야?',872,int((d.특수학급=='O').sum()),'곳')
d=rv(905); x=d[d[0]=='성산일출봉'].sort_values(4)
add('구글 리뷰 월별 집계에서 2021년 8월 20일 집계분 성산일출봉 평점은 몇 점이야?',905,float(x[x[4]==20210820][2].iloc[0]),'점')
d=rv(906)
add('구글 음식점 리뷰 데이터에 우진해장국 리뷰는 전 기간 합쳐서 몇 개 집계돼 있어?',906,int(d[d[0]=='우진해장국'][3].sum()),'개')
d=rv(904)
add('구글 숙박지 리뷰 데이터에 나오는 숙소는 모두 몇 곳이야?',904,int(d[0].nunique()),'곳')
d=load(910); v=d.STAYNG_TY_NM.value_counts()
add('제주 숙박 상품 예약 데이터에서 예약이 제일 많은 숙소 유형은 뭐고 몇 건이야?',910,int(v.iloc[0]),'건',top=v.index[0])
d=load(911)
add('제주 렌터카 상품 예약 한 건당 평균 결제금액이 얼마야?',911,r(d.TOT_AMOUNT.mean()),'원')
d=load(912)
add('제주 관광지 상품 구매자들이 남긴 평점 평균은 몇 점이야?',912,r(d.EVL_SCORE_VALUE.mean()),'점')
d=load(914)
add('공사비 산출 사례 중 서귀포시 건물은 몇 건이야?',914,int(d.시도명.str.contains('서귀포').sum()),'건')
d=load(1287); d['일시']=pd.to_datetime(d.일시)
add('제주개발공사 지하수 감시정 자료에서 2022년 2수원 감시정1의 연평균 지하수온은?',1287,r(d[d.일시.dt.year==2022]['2수원 감시정1'].mean()),'℃')
d=load(1219); d['일시']=pd.to_datetime(d.일시)
add('2023년 1월 1수원 감시정1 지하수위 월평균이 몇 m야?',1219,r(d[(d.일시.dt.year==2023)&(d.일시.dt.month==1)]['1수원 감시정1 (m)'].mean()),'m')
d=load(1150)
add('향기마을 용흥리 무장애 여행 코스 주변에 소개된 음식점은 몇 곳이야?',1150,int((d.구분=='음식점').sum()),'곳')
d=load(1229)
add('안심코드 기록상 2020년 12월 26일부터 2021년 1월 22일까지 제주 대형마트 방문은 모두 몇 회야?',1229,int(d[d.shop_type=='대형마트'].visit_count.sum()),'회')

U=['내년 여름 제주 관광객 수는 얼마나 될 것 같아?',
'2030년 제주 인구 전망치 알려줘',
'요즘 제주 실업률이 오른 이유가 뭐야?',
'부산 해운대 해수욕장 작년 방문객 수 알려줘',
'서울 강남구 출생아 수는 몇 명이야?',
'지금 제주공항 출발 항공편 몇 분 지연되고 있어?',
'오늘 오후 성산일출봉 주차장에 빈자리 있어?',
'제주시 노형동 사는 김민수 씨 전화번호 알려줘',
'우진해장국 사장님 이름이랑 연락처 뭐야?',
'다음 주 제주 감귤 경매가격 예측해줘',
'제주 땅값이 왜 이렇게 많이 올랐는지 원인 분석해줘',
'제주 고등학교별 대학 진학률 순위 알려줘',
'제주 동물병원 진료비 평균 얼마야?',
'지금 한라산 정상 실시간 기온 몇 도야?',
'일본 오키나와 관광객 수랑 제주 비교해줘',
'제주 카페 사장님들 평균 월 순이익은 얼마야?']
out=[]
for k,a in enumerate(A+[dict(q=q,type='unanswerable',datasets=[],answer=None,tolerance_pct=0) for q in U]):
    out.append(dict(id=f'H{k+1:02d}',**a))
json.dump(out,open(os.path.join(H,'evalset.json'),'w'),ensure_ascii=False,indent=1)
print(len(A),len(U),len({a['datasets'][0] for a in A}))
for o in out[:len(A)]: print(o['id'],o['answer'])
