import pathlib,subprocess,json,hashlib
root=pathlib.Path(__file__).resolve().parent
rows=[]
for name,entry,expected in [('phase3-s1-key-corrected','calc.py','30.0'),('phase3-s2-python-corrected','main.py','hello')]:
 ws=root/(name+'-workspace')
 meta=json.loads((root/(name+'-meta.json')).read_text(encoding='utf-8'))
 sources={p.name:p.read_text(encoding='utf-8') for p in ws.glob('*.py')}
 for p in ws.glob('*.py'):
  assert hashlib.sha256(p.read_bytes()).hexdigest()==meta['after'][p.name]
 result=subprocess.run([str(root/'phase3-python-regression/python.exe'),'-B',entry],cwd=str(ws),stdout=subprocess.PIPE,stderr=subprocess.STDOUT)
 output=result.stdout.decode('utf-8','replace')
 assert result.returncode==0 and expected in output,(name,result.returncode,output)
 rows.append({'name':name,'exit':result.returncode,'output':output,'source_matches_model_run_after_hash':True,'sources':sources})
meta=json.loads((root/'phase3-s3-key-corrected-meta.json').read_text(encoding='utf-8'))
assert meta['exit']==2 and meta['zero_change']
rows.append({'name':'phase3-s3-key-corrected','exit':meta['exit'],'zero_change':meta['zero_change']})
print(json.dumps(rows,ensure_ascii=True))
