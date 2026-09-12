import pathlib,subprocess,json,re,http.server,http.client,threading,queue,os,time,hashlib
root=pathlib.Path(__file__).resolve().parent
run=root/'c3-live-2';run.mkdir(exist_ok=True)
workspace=run/'workspace';workspace.mkdir(exist_ok=True)
home=run/'home';home.mkdir(exist_ok=True)
class Model(http.server.BaseHTTPRequestHandler):
 def do_POST(self):
  req=json.loads(self.rfile.read(int(self.headers['Content-Length'])))
  if not req.get('stream'):
   self.send_response(200);self.end_headers();self.wfile.write(b'{"choices":[{"message":{"role":"assistant","content":"OK"}}]}');return
  msgs=req['messages'];user=[m['content'] for m in msgs if m['role']=='user'][-1]
  if user=='C3WAIT':time.sleep(3)
  tool=msgs[-1]['role']=='tool'
  if user=='C3WRITE' and not tool:
   delta={'tool_calls':[{'index':0,'id':'write1','type':'function','function':{'name':'write','arguments':json.dumps({'path':'created.txt','content':'proof'})}}]};finish='tool_calls'
  else:delta={'content':'DONE'};finish='stop'
  self.send_response(200);self.send_header('Content-Type','text/event-stream');self.end_headers()
  try:self.wfile.write(('data: '+json.dumps({'choices':[{'index':0,'delta':delta}]})+'\n\ndata: '+json.dumps({'choices':[{'index':0,'delta':{},'finish_reason':finish}]})+'\n\ndata: [DONE]\n\n').encode())
  except OSError:pass
 def log_message(self,*args):pass
mock=http.server.HTTPServer(('127.0.0.1',0),Model);threading.Thread(target=mock.serve_forever,daemon=True).start()
env=os.environ.copy();env['USERPROFILE']=str(home)
exe=root/'c3-amd64.exe'
p=subprocess.Popen([str(exe),'--base-url','http://127.0.0.1:%d/v1'%mock.server_port,'--model','mock-model','--sandbox-preference','jobobject','--workspace',str(workspace),'serve'],stdout=subprocess.PIPE,stderr=subprocess.PIPE,env=env)
evidence={'started':time.time(),'hash':hashlib.sha256(exe.read_bytes()).hexdigest(),'checks':[],'events':[]}
try:
 line=p.stdout.readline().decode('utf-8');evidence['startup']=line
 address=re.search(r'http://(127\.0\.0\.1:\d+)',line).group(1)
 c=http.client.HTTPConnection(address,timeout=10);c.request('GET','/');html=c.getresponse().read().decode();token=re.search(r'name=pulse7-token content="([a-f0-9]+)"',html).group(1)
 def api(method,path,body=None,auth=True):
  conn=http.client.HTTPConnection(address,timeout=15);headers={'Content-Type':'application/json'}
  if auth:headers['Authorization']='Bearer '+token
  conn.request(method,path,None if body is None else json.dumps(body),headers);resp=conn.getresponse();raw=resp.read().decode();conn.close();return resp.status,json.loads(raw)
 status,_=api('GET','/api/config',auth=False);evidence['checks'].append(['unauthorized',status]);assert status==401
 status,conf=api('PUT','/api/config',{'base_url':'http://127.0.0.1:%d/v1'%mock.server_port,'model':'mock-model','api_key':'local-test-key'});assert status==200 and 'local-test-key' not in json.dumps(conf)
 status,conn=api('POST','/api/connection-test',{'base_url':'http://127.0.0.1:%d/v1'%mock.server_port,'model':'mock-model'});assert status==200;evidence['checks'].append(['connection',conn])
 assert api('PUT','/api/workspace',{'path':str(workspace)})[0]==200
 stream=http.client.HTTPConnection(address,timeout=20);stream.request('GET','/api/events',headers={'Authorization':'Bearer '+token});response=stream.getresponse();q=queue.Queue()
 def read_events():
  try:
   while True:
    line=response.fp.readline()
    if not line:return
    if line.startswith(b'data: '):
     event=json.loads(line[6:]);evidence['events'].append(event);q.put(event)
  except Exception as err:q.put({'type':'reader_error','data':str(err)})
 threading.Thread(target=read_events,daemon=True).start()
 assert api('PUT','/api/permissions',{'profile':'strict'})[0]==200
 status,turn=api('POST','/api/turns',{'prompt':'C3WRITE'});assert status==202
 deadline=time.time()+25
 while time.time()<deadline:
  e=q.get(timeout=15)
  if e['type']=='permission_request':
   rid=e['data']['requestId'];assert api('POST','/api/permission',{'requestId':rid,'decision':'allow'})[0]==200
  if e['type']=='turn_result':break
 assert e['data']['status']=='success',e
 assert (workspace/'created.txt').read_text()=='proof'
 status,history=api('GET','/api/sessions/'+turn['sessionId']+'/messages?limit=100');assert status==200
 assert any(m.get('tool_calls') for m in history['messages']);evidence['checks'].append(['history_count',len(history['messages'])])
 status,checkpoints=api('GET','/api/checkpoints');assert status==200;evidence['checkpoints']=checkpoints
 status,result=api('GET','/api/tool-result?ref=session:'+turn['sessionId']+'%23tool:write1');assert status==200;evidence['tool_result']=result
 assert api('POST','/api/sessions/resume',{'sessionId':turn['sessionId']})[0]==200
 status,_=api('POST','/api/turns',{'prompt':'C3WAIT'});assert status==202
 time.sleep(.3);assert api('POST','/api/interrupt',{})[0]==200
 while True:
  e=q.get(timeout=15)
  if e['type']=='turn_result':break
 assert e['data']['status']=='interrupted',e
 evidence['checks'].append(['interrupt',e['data']])
 evidence['status']='PASS'
except Exception as err:
 evidence['status']='FAIL';evidence['error']=repr(err);evidence['traceback']=__import__('traceback').format_exc()
finally:
 p.terminate();p.wait(timeout=10)
 try:
  s=__import__('socket').create_connection(tuple([address.split(':')[0],int(address.split(':')[1])]),timeout=1);s.close();evidence['port_released']=False
 except OSError:evidence['port_released']=True
 evidence['stderr']=p.stderr.read().decode('utf-8','replace');evidence['ended']=time.time()
 (run/'evidence.json').write_text(json.dumps(evidence,ensure_ascii=False,indent=2),encoding='utf-8')
 print(json.dumps(evidence,ensure_ascii=True));mock.shutdown()
