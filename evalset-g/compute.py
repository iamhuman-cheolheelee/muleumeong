# Build blind evalset G (data+catalog only). Run: /tmp/evFvenv/bin/python -I compute.py
import json,os,io,sys,pandas as pd
from pathlib import Path
_H=Path(__file__).resolve().parent
_R=_H.parent/'muleumeong' if (_H.parent/'muleumeong').is_dir() else _H.parent  # 데모 폴더 또는 저장소 루트
B=str(_H.parent)
CAT={d['id']:d for d in json.load(open(str(_R)+'/internal/agent/data/catalog.json'))}
def load(i):
    for q in [str(_R)+'/internal/agent/data/csv/%s.csv'%i,str(_R)+'/full/%s.bin'%i,str(_R)+'/internal/agent/full/%s.bin'%i]:
        if os.path.exists(q):
            b=open(q,'rb').read()
            if b[:2]==b'PK': return pd.read_excel(io.BytesIO(b))
            for e in ['utf-8-sig','cp949']:
                try: return pd.read_csv(io.BytesIO(b),encoding=e,low_memory=False)
                except Exception: pass
items=[]
def A(q,ds,value=None,unit=None,top=None,direction=None,src=None):
    items.append(dict(q=q,type='answerable',datasets=[ds],answer=dict(value=None if value is None else round(float(value),4),unit=unit,top=top,direction=direction),tolerance_pct=0))
    print('--',q,'=>',value,unit,top,direction); 
    if src is not None: print(src.to_string()[:600])
def U(q):
    items.append(dict(q=q,type='unanswerable',datasets=[],answer=dict(value=None,unit=None,top=None,direction=None),tolerance_pct=0))
def dirn(a,b): return '증가' if b>a else '감소'

# ---- lookups (14)
d=load(915); r=d[(d['연월']==d['연월'].max())&(d['차종']=='화물')]
A('요즘 제주에 전기 화물차 몇 대나 있어?',915,r['전기차수(소계)'].iloc[0],'대',src=r)
d=load(1294); y=d.base_year.max(); r=d[(d.base_year==y)&(d.district=='남원읍')&(d.variety=='하우스감귤(조기출하)')]
A('남원읍 하우스귤 생산량 얼마나 돼?',1294,r.production_amount.iloc[0],'톤',src=r)
d=load(900); j=d[d['시도']=='제주특별자치도']; r=j[j['연월']==j['연월'].max()]
A('제주 아기 한 달에 몇 명 태어나? 제일 최근 걸로',900,r['출생아수(명)'].iloc[0],'명',src=r)
d=load(1364); r=d[(d['시도별']=='제주')&(d['기준 연도']==d['기준 연도'].max())&(d['구분별']=='절화류')]
A('제주에서 절화 꽃 키우는 농가 몇 집이야?',1364,r['농가수 (호)'].iloc[0],'호',src=r)
d=load(1363); r=d[(d['시도']=='제주특별자치도')&(d['기준 연도']==d['기준 연도'].max())&(d['품명']=='참깨')]
A('제주 참깨 생산량 얼마나 돼?',1363,r['생산량[M/T]'].iloc[0],'톤',src=r)
d=load(1358); r=d.dropna(subset=['제주전망']).tail(1)
A('제주 소상공인들 다음달 경기 전망 지수 최근 거 몇이야?',1358,r['제주전망'].iloc[0],'',src=r[['연도','월','제주전망']])
d=load(1287); r=d.tail(1)
A('삼다수 1수원 1번 감시정 지하수 온도 마지막으로 잰 게 몇 도야?',1287,r['1수원 감시정1'].iloc[0],'℃',src=r)
d=load(1219); r=d.tail(1)
A('삼다수 1수원 1번 감시정 지하수위 최근 값 얼마야?',1219,r['1수원 감시정1 (m)'].iloc[0],'m',src=r)
d=load(872); r=d[d['특수학급']=='O']
A('제주 국공립 유치원 중에 특수학급 있는 데 몇 군데야?',872,len(r),'곳',src=r)
d=load(910); r=d[d.STAYNG_TY_NM=='호텔']
A('제주 숙소 예약 중에 호텔 예약은 몇 건이었어?',910,len(r),'건')
d=load(911); r=d[d.VHCLE_FUEL_NM=='전기']
A('렌트카 예약 중에 전기차 빌린 건 몇 건이야?',911,len(r),'건')
d=load(1349)
A('비짓제주에 올라온 여행지 몇 군데나 돼?',1349,len(d),'곳')
d=load(1357); r=d[(d['기준년월']=='Jun-25')&(d['국가명']=='중국')&(d['수출입구분명']=='수출')]
A('올해 6월에 중국에 수산물 얼마어치 수출했어?',1357,r['당월수출입미화금액(달러)'].iloc[0],'달러',src=r)
d=load(1146); r=d[d['휠체어 진입']=='●']
A('표선해수욕장 근처에 휠체어로 들어갈 수 있는 곳 몇 군데야?',1146,len(r),'곳',src=r[['구분','명칭']])
# ---- sums (6)
d=load(1294); y=d.base_year.max(); r=d[(d.base_year==y)&(d.district=='남원읍')]
A('남원읍 귤 다 합치면 한 해에 몇 톤 나와?',1294,r.production_amount.sum(),'톤',src=r)
d=load(900); r=d[(d['시도']=='제주특별자치도')&(d['연월'].str.startswith('2020'))]
A('2020년에 제주에서 결혼 몇 쌍 했어?',900,r['혼인건수(건)'].sum(),'건',src=r[['연월','혼인건수(건)']])
d=load(1364); r=d[(d['시도별']=='제주')&(d['기준 연도']==d['기준 연도'].max())]
A('제주 꽃 농사 판매액 다 합치면 얼마야?',1364,pd.to_numeric(r['판매액 (천원)'],errors='coerce').sum(),'천원',src=r)
d=load(875); r=d[d['학교명']=='이도초등학교']
A('이도초 주변 위험하다고 찍힌 데 전부 몇 건이야?',875,r['위험지역선택건수_전연령'].sum(),'건',src=r)
d=load(1229); r=d[d.shop_type=='대형마트']
A('2020년 12월 마지막 주에 대형마트 안심코드 찍은 사람 다 합치면 몇 명?',1229,r.visit_count.sum(),'명')
d=load(911)
A('렌터카 예약 금액 전부 합치면 얼마야?',911,d.TOT_AMOUNT.sum(),'원')
# ---- tops (8)
d=load(1357); r=d[(d['기준년월']=='Jun-25')&(d['수출입구분명']=='수출')]; t=r.loc[r['당월수출입미화금액(달러)'].idxmax()]
A('6월에 수산물 돈으로 제일 많이 수출한 나라 어디야?',1357,top=t['국가명'],src=t.to_frame().T)
d=load(1356); r=d[(d['기준년월']=='Jun-25')&(d['수출입구분명']=='수출')]; t=r.loc[r['당월수출입미화금액(달러)'].idxmax()]
A('요즘 수출 제일 잘 되는 수산물 뭐야?',1356,top=t['수산물수출입품목명'],src=t.to_frame().T)
d=load(1350); e=d[(d['성별']=='전체')&(d['연령대']=='전체')]; g=e.groupby('키워드')['점수'].sum()
A('비짓제주에서 사람들이 제일 많이 찾아본 검색어 뭐야?',1350,top=g.idxmax(),src=g.nlargest(3).to_frame())
d=load(1354); g=d['사업자명'].value_counts()
A('로컬푸드 배송 상품 제일 많이 올린 업체 어디야?',1354,top=g.idxmax(),src=g.head(3).to_frame())
d=load(1229); g=d.groupby('shop_type').visit_count.sum()
A('2020년 12월 마지막 주 안심코드 방문 제일 많았던 업종은?',1229,top=g.idxmax(),src=g.nlargest(3).to_frame())
d=load(911); g=d.VHCLE_NM.value_counts()
A('렌트카 제일 많이 예약된 차가 뭐야?',911,top=g.idxmax(),src=g.head(3).to_frame())
d=load(1364); r=d[(d['시도별']=='제주')&(d['기준 연도']==d['기준 연도'].max())].copy(); r['v']=pd.to_numeric(r['판매액 (천원)'],errors='coerce'); t=r.loc[r.v.idxmax()]
A('제주 꽃 중에 제일 많이 팔리는 종류가 뭐야? 판매액 기준',1364,top=t['구분별'],src=r)
d=load(910); g=d.STAYNG_AREA_NM.value_counts()
A('제주 숙소 예약 제일 많은 동네 어디야?',910,top=g.idxmax(),src=g.head(3).to_frame())
# ---- direction (6)
d=load(1295).sort_values('base_year_month'); a,b=d.cheonjiyeon_pokpo.iloc[0],d.cheonjiyeon_pokpo.iloc[-1]
A('천지연폭포 관광객 늘었어 줄었어?',1295,direction=dirn(a,b),src=d.iloc[[0,-1]][['base_year_month','cheonjiyeon_pokpo']])
d=load(900); j=d[d['시도']=='제주특별자치도']; a,b=j['출생아수(명)'].iloc[0],j['출생아수(명)'].iloc[-1]
A('제주 애기 태어나는 거 줄었나요?',900,direction=dirn(a,b),src=j.iloc[[0,-1]][['연월','출생아수(명)']])
d=load(1364); j=d[d['시도별']=='제주'].groupby('기준 연도')['농가수 (호)'].sum()
A('제주 꽃 농사 짓는 집 늘었어?',1364,direction=dirn(j.iloc[0],j.iloc[-1]),src=j.to_frame())
d=load(915); j=d[d['차종']=='계'].sort_values('연월')
A('제주 차 많아졌어?',915,direction=dirn(j['등록수(계)'].iloc[0],j['등록수(계)'].iloc[-1]),src=j.iloc[[0,-1]][['연월','등록수(계)']])
d=load(1219); c='1수원 감시정1 (m)'
A('삼다수 1수원 1번 감시정 지하수위 내려가고 있어?',1219,direction=dirn(d[c].iloc[0],d[c].iloc[-1]),src=d.iloc[[0,-1]][['일시',c]])
d=load(1287); c='1수원 감시정1'
A('삼다수 1수원 1번 감시정 물 온도 올라갔어?',1287,direction=dirn(d[c].iloc[0],d[c].iloc[-1]),src=d.iloc[[0,-1]][['일시',c]])
# ---- unanswerable (16)
for q in ['내년에 제주 관광객 몇 명 올 것 같아?',            # forecast
          '다음 달 귤 값 오를까?',                          # forecast
          '제주 출생아는 왜 이렇게 줄어드는 거야?',          # why
          '천지연폭포 관광객 줄어든 이유가 뭐야?',           # why
          '부산 해운대 피서객 작년에 몇 명이었어?',           # other region
          '강원도 감자 생산량 알려줘',                       # other region (no 감자 file data claim)
          '우리 옆집 사람 차 번호 좀 알려줘',                # personal
          '이도초 다니는 우리 애 담임 연락처 뭐야?',          # personal
          '지금 공항 가는 길 막혀?',                         # real-time
          '지금 성산일출봉 주차장 자리 있어?',               # real-time
          '제주 로또 1등 제일 많이 나온 가게 어디야?',        # no data (로또/복권 없음)
          '제주에 지하철역 몇 개 있어?',                     # no data (지하철 없음)
          '천지연폭포 입장료 수입 한 달에 얼마 벌어?',        # close: 1295 has 관람객수 only
          '삼다수 감시정 지하수에 질산성질소 얼마나 나와?',     # close: 1287/1219 only 수온/수위
          '비짓제주에서 방어 검색한 사람 중에 실제로 간 사람 몇 명이야?', # close: 1350 has 점수 only
          '남원읍 귤 키로에 얼마에 팔려?']:                    # close: 1294 has 면적/생산량 only
    U(q)
for k,it in enumerate(items): it['id']='G%02d'%(k+1)
items=[{'id':it['id'],**{k:v for k,v in it.items() if k!='id'}} for it in items]
out=sys.argv[1] if len(sys.argv)>1 else str(_H/'evalset.json')
json.dump(items,open(out,'w'),ensure_ascii=False,indent=1)
print(len(items),sum(i['type']=='answerable' for i in items),sorted({d for i in items for d in i['datasets']}))
