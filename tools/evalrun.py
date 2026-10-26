# 평가셋 채점: python3 evalrun.py <base> <evalset.json> <out.json>
import json, sys, re, urllib.request, urllib.parse
base, es, out = sys.argv[1], json.load(open(sys.argv[2])), sys.argv[3]
def nums(s): return [float(x.replace(',', '')) for x in re.findall(r'-?\d[\d,]*(?:\.\d+)?', s or '')]
rows = []
for e in es:
    a = json.load(urllib.request.urlopen(base + "/api/ask?q=" + urllib.parse.quote(e['q']), timeout=180))
    r = {'id': e['id'], 'q': e['q'], 'type': e.get('type', e.get('kind')), 'ms': a.get('totalMs')}
    if a.get('refused'):
        r['out'] = 'refuse'; r['detail'] = a.get('error'); r['cands'] = [(c['id'], c['why'][:90]) for c in a.get('cands') or []]
    else:
        txt = a['text'] + ' ' + ' '.join(f['value'] for f in a['result']['facts'] or [])
        r['chosen'] = a['chosen']['id']; r['text'] = a['text']
        ok = False
        if e.get('type', e.get('kind')) == 'answerable':
            ans = e['answer']; ok = a['chosen']['id'] in e['datasets']
            if ok and ans.get('value') is not None:
                tol = max(abs(ans['value']) * e.get('tolerance_pct', 1) / 100, 0.051)
                ok = any(abs(n - ans['value']) <= tol for n in nums(txt))
            if ok and ans.get('top'):
                ok = any(t.replace(' ', '') in txt.replace(' ', '') for t in [ans['top']] + list(e.get('top_alias') or []))
            if ok and ans.get('direction') in ('증가', '감소'):
                ok = (('늘' in txt or '높아' in txt) if ans['direction'] == '증가' else ('줄' in txt or '낮아' in txt))
        r['out'] = 'correct' if ok else 'wrong'
    rows.append(r)
A = [r for r in rows if r['type'] == 'answerable']; U = [r for r in rows if r['type'] == 'unanswerable']
ref = [r for r in rows if r['out'] == 'refuse']
m = {'n': len(rows), 'answerable': len(A), 'unanswerable': len(U),
     'correct': sum(r['out'] == 'correct' for r in A),
     'wrong': sum(r['out'] == 'wrong' for r in rows),
     'refused': len(ref), 'refused_ok': sum(r['type'] == 'unanswerable' for r in ref),
     'unans_refused': sum(r['out'] == 'refuse' for r in U)}
m['accuracy_answerable'] = round(m['correct'] / max(1, len(A)) * 100, 1)
m['refusal_precision'] = round(m['refused_ok'] / max(1, len(ref)) * 100, 1)
m['refusal_recall_unanswerable'] = round(m['unans_refused'] / max(1, len(U)) * 100, 1)
m['wrong_rate'] = round(m['wrong'] / len(rows) * 100, 1)
json.dump({'metrics': m, 'rows': rows}, open(out, 'w'), ensure_ascii=False, indent=1)
print(json.dumps(m, ensure_ascii=False))
for r in rows:
    if r['out'] != 'correct' and not (r['type'] == 'unanswerable' and r['out'] == 'refuse'):
        print(r['id'], r['type'], r['out'], r['q'], '|', r.get('text', r.get('detail'))[:140], r.get('chosen', ''), r.get('cands', '')[:2] if r.get('cands') else '')
