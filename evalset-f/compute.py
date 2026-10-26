# Deterministic answer computation for evalset-f. Run: python3 -I compute.py <B>
import sys, os, io, json
import pandas as pd
B = sys.argv[1]
def load(i, header='infer'):
    for p in [f'{B}/muleumeong/full/{i}.bin', f'{B}/muleumeong/internal/agent/full/{i}.bin']:
        if os.path.exists(p):
            b = open(p, 'rb').read()
            if b[:2] == b'PK': return pd.read_excel(io.BytesIO(b), header=0 if header=='infer' else header)
            for e in ['utf-8-sig', 'cp949']:
                try: return pd.read_csv(io.BytesIO(b), encoding=e, low_memory=False, header=header)
                except Exception: pass
    raise SystemExit(f'missing {i}')
pd.set_option('display.width', 250); pd.set_option('display.max_columns', 20)
items = []
def show(x): print(x.to_string() if hasattr(x, 'to_string') else x)
def add(ds, q, value=None, unit=None, top=None, direction=None, tol=0, src=None):
    if src is not None: show(src)
    v = None if value is None else (round(float(value), 4) if tol else int(value))
    items.append(dict(id=f'F{len(items)+1:02d}', q=q, type='answerable', datasets=[str(ds)],
                      answer=dict(value=v, unit=unit, top=top, direction=direction), tolerance_pct=tol))
    print('->', items[-1]['id'], ds, v, unit, top, direction, '\n')

# ---- single lookups ----
d = load(1364); r = d[(d['기준 연도']==2023)&(d['시도별']=='제주')&(d['구분별']=='절화류')]
add(1364, '2023년에 제주에서 절화(꽃꽂이용 꽃) 키우는 농가가 몇 호였어요?', r['농가수 (호)'].item(), '호', src=r)
d = load(1363); r = d[(d['기준 연도']==2023)&(d['시도']=='제주특별자치도')&(d['품명']=='차')]
add(1363, '2023년 제주에서 차(茶) 생산량이 얼마나 됐나요?', r['생산량[M/T]'].item(), '톤', src=r)
d = load(1295); r = d[d.base_year_month==202205]
add(1295, '2022년 5월에 천지연폭포 찾은 관람객 수가 몇 명이었어요?', r.cheonjiyeon_pokpo.item(), '명', src=r)
d = load(900); r = d[(d['연월']=='2021. 08')&(d['시도']=='제주특별자치도')]
add(900, '2021년 8월 한 달 동안 제주에서 태어난 아기는 몇 명인가요?', r['출생아수(명)'].item(), '명', src=r)
d = load(1357); r = d[(d['기준년월']=='Aug-24')&(d['수출입구분명']=='수출')&(d['국가명']=='일본')]
add(1357, '2024년 8월 한 달간 우리나라 수산물을 일본에 수출한 금액은 몇 달러예요?', r['당월수출입미화금액(달러)'].item(), '달러', src=r)
d = load(1356); r = d[(d['기준년월']=='Aug-24')&(d['수출입구분명']=='수출')&(d['수산물수출입품목명']=='넙치(활어)')]
add(1356, '2024년 8월에 활광어(넙치 활어)를 몇 kg 수출했나요?', r['당월수출입중량(킬로그램)'].item(), 'kg', src=r)
d = load(1351); r = d[d['구분연도']==2024]
add(1351, '2024년 제주 실업률이 몇 퍼센트였죠?', r['실업률'].item(), '%', tol=1, src=r)
d = load(905, header=None); r = d[(d[0]=='1100고지 제주')&(d[4]==20210120)]
add(905, '구글 리뷰 분석 자료에서 1100고지의 2021년 1월 평점은 몇 점이었나요?', r[2].item(), '점', tol=1, src=r)
d = load(906, header=None); r = d[(d[0]=='064노트')&(d[4]==20210120)]
add(906, '구글 음식점 리뷰 분석에서 064노트 식당의 2021년 1월 평점이 어떻게 돼요?', r[2].item(), '점', tol=1, src=r)
d = load(1352); r = d[(d['해당연도']==2024)&(d['구분']=='1분기')&(d['측정지점']=='영어교습소')]
add(1352, '2024년 1분기 한마음병원 근처 영어교습소 지점에서 잰 낮 12시 소음이 몇 dB였어요?', r['낮시간대 소음도 (12시)'].item(), 'dB', src=r)
d = load(879); r = d[(d.channel=='트위터')&(d.keyword1=='커플여행')&(d.period=='코로나_전')&(d['rank']==1)]
add(879, '코로나 전에 트위터에서 커플여행이랑 제주를 같이 언급할 때 제일 많이 나온 연관어가 뭐예요?', top=r.ass_word.item(), src=r)
d = load(873, header=None); names = d[0].iloc[2:]
add(873, '제주에 공시 대상 사립 유치원이 몇 곳 있어요?', names.notna().sum(), '곳', src=names)

# ---- sums / aggregates ----
d = load(1150); r = d[d['구분']=='음식점']
add(1150, '향기마을 용흥리 무장애 여행지 주변에 음식점이 몇 군데 소개돼 있나요?', len(r), '곳', src=r)
d = load(1099)
add(1099, '향기마을 용흥리 무장애 여행 코스에 안내 지점이 몇 개 있어요?', len(d), '개', src=d[['순서','구분']])
d = load(872); r = d[d['특수학급']=='O']
add(872, '제주 국공립 유치원 중에 특수학급 운영하는 곳이 몇 곳이에요?', len(r), '곳', src=r)
d = load(902); r = d[d.title=='갓전시관']
add(902, '구글에서 갓전시관 리뷰 별점 평균이 몇 점이에요?', r.stars.mean(), '점', tol=1, src=r.stars)
d = load(1287); r = d[d['일시'].str.startswith('2022')]['1수원 감시정1']
add(1287, '삼다수 1취수원 감시정1 지하수 온도, 2022년 평균이 몇 도였나요?', r.mean(), '℃', tol=1, src=r.describe())
d = load(1219); r = d[d['일시'].str.startswith('2023')]['2수원 감시정1 (m)']
add(1219, '2023년에 삼다수 2취수원 감시정1 지하수위가 가장 높았을 때 몇 m였어요?', r.max(), 'm', tol=1, src=r.describe())
d = load(1294); r = d[d.base_year==2021]
add(1294, '2021년 서귀포시 감귤 생산량을 다 합치면 얼마나 돼요?', r.production_amount.sum(), '톤', tol=1, src=r.groupby('district').production_amount.sum())
d = load(1286); r = d[d.shop_type=='대형마트']
add(1286, '안심코드 기록으로 2022년 1월 22일부터 2월 18일까지 대형마트 방문객이 총 몇 명이었나요?', r.visit_count.sum(), '명', src=r.visit_count.describe())
d = load(1283); r = d[d.shop_type=='대형마트']
add(1283, '안심코드 기준 2022년 1월 1일~1월 28일 기간 대형마트 방문객 합계는요?', r.visit_count.sum(), '명', src=r.visit_count.describe())
d = load(875)
add(875, '초등학교 교통안전 데이터에 들어 있는 제주 초등학교는 몇 곳이에요?', d['학교명'].nunique(), '곳', src=d['학교명'].value_counts().head())
d = load(1354); r = d[d['배송가능지역']=='제주특별자치도']
add(1354, '로컬푸드 배송 정보에서 배송 지역이 제주로 돼 있는 상품은 몇 개예요?', len(r), '개', src=r)

# ---- argmax ----
d = load(910); vc = d.STAYNG_TY_NM.value_counts()
add(910, '숙박지 선호·구매 데이터에서 가장 많이 나오는 숙소 유형이 뭐예요?', top=vc.index[0], src=vc.head(3))
d = load(912); vc = d.TRRSRT_TY_NM.value_counts()
add(912, '관광지 상품 선호·구매 데이터에서 제일 많이 나온 관광지 유형은 뭐예요?', top=vc.index[0], src=vc.head(3))
d = load(1350); r = d[(d['통계일']=='2020-12-24')&(d['성별']=='전체')&(d['연령대']=='전체')].sort_values('점수', ascending=False)
add(1350, '2020년 12월 24일 VISIT JEJU 검색어 트렌드에서 전체 점수가 가장 높은 키워드는?', top=r['키워드'].iloc[0], src=r.head(3))
d = load(869); g = d[d['경락일'].str.startswith('2017')].groupby('공영 도매시장명')['거래량'].sum().sort_values(ascending=False)
add(869, '2017년 한 해 동안 무 경락 거래량이 가장 많았던 공영 도매시장은 어디예요?', top=g.index[0], src=g.head(3))

# ---- trends ----
d = load(877); d['y'] = d.Date.str[:4]; g = d.groupby('y').Tem_mean.mean()
add(877, '제주 연평균 기온이 2000년이랑 2020년 비교하면 올랐어요, 내렸어요?', direction='증가' if g['2020']>g['2000'] else '감소', src=g[['2000','2020']])
d = load(1358); g = d.groupby('연도')['경기전반체감'].mean()
add(1358, '소상공인 경기전반 체감지수(연평균)가 2013년에 비해 2024년엔 올랐나요, 떨어졌나요?', direction='증가' if g[2024]>g[2013] else '감소', src=g[[2013,2024]])
d = load(915); r = d[(d['차종']=='승용')&(d['연월'].isin([201001,202009]))].set_index('연월')['등록수(계)']
add(915, '제주 승용차 등록 대수, 2010년 1월이랑 2020년 9월 사이에 늘었나요 줄었나요?', direction='증가' if r[202009]>r[201001] else '감소', src=r)

U = [
 '2030년에 제주 관광객이 몇 명쯤 될까요?',
 '내년 제주 감귤 가격 얼마나 오를 것 같아요?',
 '제주 출생아 수가 왜 이렇게 줄어드는 거예요?',
 '요즘 제주 소상공인 체감경기가 나쁜 원인이 뭐예요?',
 '부산 시내버스 노선이 몇 개나 돼요?',
 '서울 강남구 미세먼지 어제 수치 알려주세요',
 '애월읍 사는 김철수 씨 전화번호 좀 알려주세요',
 '제주 감귤 농가 중에 제일 소득 많은 농가주 이름이 뭐예요?',
 '제주에 로또 1등 나온 복권 판매점이 어디예요?',
 '제주 사람들 중 주식이나 코인 투자하는 사람 비율은요?',
 '제주에 결혼식장(웨딩홀)이 몇 곳 있어요?',
 '제주시에 네일샵이 몇 곳이나 돼요?',
 '2022년 5월 천지연폭포 입장료 수입은 얼마였어요?',
 '2023년에 제주 화훼 수출액이 얼마였나요?',
 '2020년 9월 기준 제주 등록 승용차의 평균 차령은 몇 년이에요?',
]
for q in U:
    items.append(dict(id=f'F{len(items)+1:02d}', q=q, type='unanswerable', datasets=[],
                      answer=dict(value=None, unit=None, top=None, direction=None), tolerance_pct=0))
out = sys.argv[2] if len(sys.argv) > 2 else '/tmp/evF/evalset.json'
json.dump(items, open(out, 'w'), ensure_ascii=False, indent=1)
print(len(items), sum(i['type']=='answerable' for i in items), len({i['datasets'][0] for i in items if i['datasets']}))
