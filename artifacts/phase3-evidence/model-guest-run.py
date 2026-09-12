import sys,os,json,pathlib,hashlib,subprocess,datetime
spec=json.load(open(sys.argv[1],encoding='utf-8'))
key=sys.stdin.readline().strip()
root=pathlib.Path(spec['root']); ws=pathlib.Path(spec['workspace']); name=spec['name']
assert root.is_dir() and ws.is_dir() and key
session=root/(name+'.jsonl'); log=root/(name+'.log')
assert not session.exists(), 'Independent run requires new session'
if log.exists(): log.unlink()
def snapshot():
 files={}
 for parent,dirs,names in os.walk(str(ws)):
  dirs[:]=[d for d in dirs if d!='.git']
  for n in names:
   f=pathlib.Path(parent)/n
   files[str(f.relative_to(ws))]=hashlib.sha256(f.read_bytes()).hexdigest()
 return files
before=snapshot()
home=root/(name+'-home');home.mkdir()
env=os.environ.copy();env['PULSE7_API_KEY']=key;env['USERPROFILE']=str(home)
env['PATH']=str(root/spec.get('python_dir','python37'))+os.pathsep+str(root/'runtime/git/cmd')+os.pathsep+env.get('PATH','')
args=[spec['exe'],'--base-url','https://api.deepseek.com','--model','deepseek-flash','--sandbox-preference','jobobject','--workspace',str(ws),'--session',str(session),'--prompt-file',spec['prompt_file']]+spec.get('flags',[])+['exec']
audit=pathlib.Path(spec['exe']).parent/'data/sessions/audit.jsonl'
offset=audit.stat().st_size if audit.exists() else 0
start=datetime.datetime.now().astimezone().isoformat()
with log.open('wb') as f:
 f.write(('RUN_START '+start+'\n').encode());f.flush()
 result=subprocess.run(args,env=env,stdin=subprocess.DEVNULL,stdout=f,stderr=subprocess.STDOUT)
 end=datetime.datetime.now().astimezone().isoformat();f.write(('\nRUN_END '+end+'\nEXIT_CODE='+str(result.returncode)+'\n').encode())
after=snapshot()
changes={k:{'before':before.get(k),'after':after.get(k)} for k in sorted(set(before)|set(after)) if before.get(k)!=after.get(k)}
meta={'name':name,'start':start,'end':end,'exit':result.returncode,'command':args,'exe_sha256':hashlib.sha256(pathlib.Path(spec['exe']).read_bytes()).hexdigest(),'file_count_before':len(before),'file_count_after':len(after),'zero_change':not changes,'changes':changes,'before':before,'after':after}
(root/(name+'-meta.json')).write_text(json.dumps(meta,ensure_ascii=False,indent=2),encoding='utf-8')
with audit.open('rb') as f:
 f.seek(offset);(root/(name+'-audit.jsonl')).write_bytes(f.read())
print(json.dumps({'name':name,'exit':result.returncode,'zero_change':not changes,'start':start,'end':end}),flush=True)
