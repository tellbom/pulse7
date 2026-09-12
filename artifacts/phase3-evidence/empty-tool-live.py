import pathlib,subprocess,sys,os,time,json,re,http.client,threading,queue,hashlib
root=pathlib.Path(__file__).resolve().parent
key=sys.stdin.readline().strip(); assert key
run=root/('empty-tool-live-'+str(int(time.time())));run.mkdir()
ws=run/'workspace';ws.mkdir();home=run/'home';home.mkdir()
env=os.environ.copy();env['PULSE7_API_KEY']=key;env['USERPROFILE']=str(home)
exe=root/'empty-tool-amd64.exe'
stderr=open(str(run/'stderr.txt'),'wb')
p=subprocess.Popen([str(exe),'--base-url','https://api.deepseek.com','--model','deepseek-v4-pro','--sandbox-preference','jobobject','--workspace',str(ws),'serve'],env=env,stdout=subprocess.PIPE,stderr=stderr)
evidence={'started':time.time(),'product_sha256':hashlib.sha256(exe.read_bytes()).hexdigest(),'endpoint':'https://api.deepseek.com','model':'deepseek-v4-pro','events':[]}
try:
 line=p.stdout.readline().decode('utf-8')
 address=re.search(r'http://(127\.0\.0\.1:\d+)',line).group(1)
 c=http.client.HTTPConnection(address,timeout=20);c.request('GET','/');html=c.getresponse().read().decode();c.close()
 token=re.search(r'name="?pulse7-token"?\s+content="([a-f0-9]+)"',html).group(1)
 def api(method,path,body=None):
  conn=http.client.HTTPConnection(address,timeout=30)
  conn.request(method,path,None if body is None else json.dumps(body),{'Authorization':'Bearer '+token,'Content-Type':'application/json'})
  response=conn.getresponse();data=json.loads(response.read());conn.close()
  assert response.status<300,(response.status,data)
  return data
 connection=http.client.HTTPConnection(address,timeout=120)
 connection.request('GET','/api/events',headers={'Authorization':'Bearer '+token});response=connection.getresponse();events=queue.Queue()
 def consume():
  try:
   while True:
    line=response.fp.readline()
    if not line:return
    if line.startswith(b'data: '):
     event=json.loads(line[6:]);evidence['events'].append(event);events.put(event)
  except Exception as err:events.put({'type':'reader_error','error':str(err)})
 threading.Thread(target=consume,daemon=True).start()
 api('PUT','/api/permissions',{'profile':'open'})
 turn=api('POST','/api/turns',{'prompt':'先用 ls 查看当前空目录，不要用 shell。然后用 write 新建 probe.txt，内容为 EMPTY-TOOL-OK。最后用 read 验证内容。不要访问其它路径。'})
 deadline=time.time()+180
 while True:
  assert time.time()<deadline,'turn deadline exceeded'
  event=events.get(timeout=60)
  if event['type']=='turn_result':break
 assert event['data']['status']=='success',event
 history=api('GET','/api/sessions/'+turn['sessionId']+'/messages?limit=100')
 empty=[m for m in history['messages'] if m['role']=='tool' and m.get('content')=='']
 assert empty,'model did not produce a real empty tool response'
 assert (ws/'probe.txt').read_text(encoding='utf-8').strip()=='EMPTY-TOOL-OK'
 checkpoints=api('GET','/api/checkpoints')['checkpoints'];assert checkpoints
 evidence.update(status='PASS',empty_tool_messages=empty,checkpoints=checkpoints,session_id=turn['sessionId'],file_content=(ws/'probe.txt').read_text(encoding='utf-8'))
except Exception as err:evidence.update(status='FAIL',error=repr(err),traceback=__import__('traceback').format_exc())
finally:
 p.terminate();p.wait(timeout=10);stderr.close();evidence['ended']=time.time()
 text=json.dumps(evidence,ensure_ascii=True)
 assert key not in text
 (run/'result.json').write_text(text,encoding='utf-8');print(text)
