# Computes answer key for evalset-c from muleumeong/full/*.bin
import io, json, os, pandas as pd
B = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
def load(i):
    b = open(f'{B}/muleumeong/full/{i}.bin', 'rb').read()
    if b[:2] == b'PK': return pd.read_excel(io.BytesIO(b))
    for e in ('utf-8-sig', 'cp949'):
        try: return pd.read_csv(io.BytesIO(b), encoding=e, low_memory=False)
        except Exception: pass
num = lambda s: pd.to_numeric(s.astype(str).str.replace(',', ''), errors='coerce')
items = []
def A(q, ds, value=None, unit=None, top=None, direction=None, tol=2):
    items.append(dict(q=q, type='answerable', datasets=ds,
        answer=dict(value=None if value is None else round(float(value), 2), unit=unit, top=top, direction=direction), tolerance_pct=tol))
def U(q, ds=[]):
    items.append(dict(q=q, type='unanswerable', datasets=ds, answer=dict(value=None, unit=None, top=None, direction=None), tolerance_pct=0))
dirn = lambda a, b: 'up' if b > a else 'down'

# 1351 고용동향
d = load(1351).set_index('구분연도')
A('제주 2020년 고용률(취업 고용률)은 몇 퍼센트였어?', [1351], d.loc[2020, '취업 고용률'], '%')
A('제주 청년고용률은 2019년에 비해 2023년에 올랐어 내렸어?', [1351], direction=dirn(d.loc[2019, '청년고용률'], d.loc[2023, '청년고용률']))
A('2016~2024년 중 제주 경제활동 참가율이 가장 높았던 해는?', [1351], top=str(d['경제활동 참가율'].idxmax()))
# 1294 감귤
d = load(1294)
A('2021년 서귀포시 감귤 총 생산량은 얼마야?', [1294], d[d.base_year == 2021].production_amount.sum(), '톤')
A('2021년 서귀포시에서 감귤 생산량이 가장 많은 읍면동은 어디야?', [1294], top=d[d.base_year == 2021].groupby('district').production_amount.sum().idxmax())
h = d[d.variety.str.contains('하우스')].groupby('base_year').extent.sum()
A('서귀포시 하우스감귤 재배면적은 2012년보다 2021년에 늘었어 줄었어?', [1294], direction=dirn(h[2012], h[2021]))
# 1295 직영관광지
d = load(1295); y = d.base_year_month // 100
A('2019년 한 해 천지연폭포 관람객 수는 몇 명이야?', [1295], d[y == 2019].cheonjiyeon_pokpo.sum(), '명')
cols = {'cheonjiyeon_pokpo': '천지연폭포', 'ceongbang_pokpo': '정방폭포', 'jungmun_deapo': '중문대포주상절리', 'cheonjeyeon_pokpo': '천제연폭포', 'yongmeori_coast': '용머리해안', 'citrus_museum': '감귤박물관'}
A('2019년 서귀포시 직영관광지 중 관람객이 가장 많았던 곳은?', [1295], top=cols[d[y == 2019][list(cols)].sum().idxmax()])
A('정방폭포 연간 관람객은 2019년 대비 2021년에 늘었어 줄었어?', [1295], direction=dirn(d[y == 2019].ceongbang_pokpo.sum(), d[y == 2021].ceongbang_pokpo.sum()))
# 877 기후
d = load(877); dt = pd.to_datetime(d.Date)
A('2020년 제주 연평균 기온(일평균기온의 평균)은 몇 도였어?', [877], d[dt.dt.year == 2020].Tem_mean.mean(), '℃')
A('2010년 8월 제주 일 최고기온의 평균은 몇 도야?', [877], d[(dt.dt.year == 2010) & (dt.dt.month == 8)].Tem_max.mean(), '℃')
# 900 인구동태
d = load(900); j = d[d['시도'] == '제주특별자치도']; yy = j['연월'].str[:4].astype(int)
A('2020년 제주 출생아 수는 모두 몇 명이야?', [900], j[yy == 2020]['출생아수(명)'].sum(), '명')
A('2019년 제주 혼인 건수는 몇 건이었어?', [900], j[yy == 2019]['혼인건수(건)'].sum(), '건')
A('제주 연간 사망자 수는 2018년에 비해 2020년에 늘었어 줄었어?', [900], direction=dirn(j[yy == 2018]['사망자수(명)'].sum(), j[yy == 2020]['사망자수(명)'].sum()))
# 915 자동차
d = load(915)
A('2020년 9월 기준 제주 승용차 등록 대수(계)는?', [915], d[(d['연월'] == 202009) & (d['차종'] == '승용')]['등록수(계)'].iloc[0], '대')
e = d[d['차종'] != '계'].groupby('연월')['전기차수(소계)'].apply(lambda s: num(s).sum())
A('2019년 12월 제주 전기차 등록 대수는 전체 몇 대야?', [915], e[201912], '대')
A('제주 전기차 등록 대수는 2015년 12월에서 2020년 9월 사이 증가했어 감소했어?', [915], direction=dirn(e[201512], e[202009]))
# 1363 특용작물
d = load(1363); j = d[(d['시도'] == '제주특별자치도') & (d['기준 연도'] == 2021)]
A('2021년 제주에서 생산량이 가장 많은 특용작물은 뭐야?', [1363], top=j.groupby('품명')['생산량[M/T]'].sum().idxmax())
# 1364 화훼
d = load(1364); j = d[(d['시도별'] == '제주') & (d['기준 연도'] == 2023)].copy(); j['v'] = num(j['판매액 (천원)'])
A('2023년 제주 화훼 판매액 합계는 얼마야?', [1364], j.v.sum(), '천원')
A('2023년 제주 화훼 중 판매액이 가장 큰 구분은?', [1364], top=j.set_index('구분별').v.idxmax())
# 1358 BSI
d = load(1358)
A('2023년 제주 소상공인 경기 체감지수(BSI) 연평균은?', [1358], d[d['연도'] == 2023]['제주체감'].mean(), None)
A('제주 소상공인 체감 BSI 연평균은 2020년보다 2022년에 높아졌어 낮아졌어?', [1358], direction=dirn(d[d['연도'] == 2020]['제주체감'].mean(), d[d['연도'] == 2022]['제주체감'].mean()))
# 1352 소음
d = load(1352)
A('2024년 서귀포시 환경소음 측정 건수(측정지점 기준 행 수)는 몇 건이야?', [1352], (d['행정시'] == '서귀포시').sum(), '건')
A('2024년 제주시 환경소음 측정지점의 밤 23시 소음도 평균은 몇 dB이야?', [1352], num(d[d['행정시'] == '제주시']['밤시간대 소음도 (23시)']).mean(), 'dB')
# 1219 / 1287 지하수
d = load(1219); m = d['일시'].str[:7] == '2023-01'
A('2023년 1월 제주개발공사 1수원 감시정1 지하수위 평균은 몇 m야?', [1219], d[m]['1수원 감시정1 (m)'].mean(), 'm', tol=0.5)
d = load(1287)
A('2022년 제주개발공사 1수원 감시정1 지하수온 연평균은 몇 도야?', [1287], num(d['1수원 감시정1']).mean(), '℃')
# 1356 수산물
d = load(1356); m = d[(d['기준년월'] == 'Dec-24') & (d['수출입구분명'] == '수출')]
A('2024년 12월 우리나라 수산물 수출액은 모두 몇 달러야?', [1356], num(m['당월수출입미화금액(달러)']).sum(), '달러')
# 1353 TRQ
d = load(1353); r = d[d['년도'] == 2025]
A('2025년 농산물 TRQ 중 도입물량이 가장 많은 품목군은?', [1353], top=r.groupby('품목군명')['도입물량'].apply(lambda s: num(s).sum()).idxmax())

for q in ['내일 제주 날씨 어때?', '2030년 제주 인구는 몇 명으로 예상돼?', '지금 성산일출봉에 사람 몇 명 있어?',
          '2010년 제주 실업률은 몇 퍼센트였어?', '2005년 서귀포시 감귤 생산량은?', '2015년 천지연폭포 관람객 수는?',
          '울릉도 작년 관광객 수 알려줘', '제주에서 제일 맛있는 흑돼지집 추천해줘', '2025년 제주 출생아 수는 몇 명이야?',
          '2023년 제주 연평균 기온은 몇 도였어?', '제주 아파트 평균 매매가격 2024년 얼마야?', '오늘 제주공항 항공편 지연 몇 건이야?',
          '다음 달 제주 렌터카 요금 오를까?', '2024년 제주 크루즈선 입항 횟수는?']:
    U(q)
for k, it in enumerate(items, 1): it['id'] = f'C{k:02d}'
items = [{'id': x['id'], **{k: v for k, v in x.items() if k != 'id'}} for x in items]
json.dump(items, open(os.path.join(os.path.dirname(os.path.abspath(__file__)), 'evalset.json'), 'w'), ensure_ascii=False, indent=1)
for x in items: print(x['id'], x['type'][0], x['q'], x['answer'])
