import pandas as pd, io
from pathlib import Path
_P=Path(__file__).resolve().parent.parent
B=str((_P/'muleumeong' if (_P/'muleumeong').is_dir() else _P)/'full')+'/'
def load(i):
    raw=open(B+f'{i}.bin','rb').read()
    if raw[:2]==b'PK': return pd.read_excel(io.BytesIO(raw))
    for enc in ('utf-8-sig','cp949'):
        try: return pd.read_csv(io.BytesIO(raw),encoding=enc,low_memory=False)
        except Exception: pass
