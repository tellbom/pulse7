import pathlib, subprocess, os, json, re, time, socket, urllib.request, urllib.parse, base64, struct, hashlib, http.server, threading
root = pathlib.Path(__file__).resolve().parent
run = root / ('c4-browser-' + str(int(time.time())))
run.mkdir()
home = run / 'home'; home.mkdir()
workspace = run / 'workspace'; workspace.mkdir()
env = os.environ.copy(); env['USERPROFILE'] = str(home)
class Model(http.server.BaseHTTPRequestHandler):
 def do_POST(self):
  req=json.loads(self.rfile.read(int(self.headers['Content-Length'])))
  if not req.get('stream'):
   self.send_response(200); self.end_headers(); self.wfile.write(b'{"choices":[{"message":{"role":"assistant","content":"OK"}}]}'); return
  msgs=req['messages']; user=[m['content'] for m in msgs if m['role']=='user'][-1]
  if user=='C4WAIT': time.sleep(3)
  if user=='C4BACKGROUND' and msgs[-1]['role']!='tool':
   delta={'tool_calls':[{'index':0,'id':user,'type':'function','function':{'name':'shell','arguments':json.dumps({'command':'echo BACKGROUND-FIRST & ping -n 3 127.0.0.1 >nul & echo BACKGROUND-LAST','background':True})}}]}; finish='tool_calls'
  elif user in ('C4ALLOW','C4DENY') and msgs[-1]['role']!='tool':
   delta={'tool_calls':[{'index':0,'id':user,'type':'function','function':{'name':'write','arguments':json.dumps({'path':user+'.txt','content':'browser-proof'})}}]}; finish='tool_calls'
  else: delta={'content':'DONE '+user}; finish='stop'
  self.send_response(200); self.send_header('Content-Type','text/event-stream'); self.end_headers()
  try:self.wfile.write(('data: '+json.dumps({'choices':[{'index':0,'delta':delta}]})+'\n\ndata: '+json.dumps({'choices':[{'index':0,'delta':{},'finish_reason':finish}]})+'\n\ndata: [DONE]\n\n').encode())
  except OSError:pass
 def log_message(self,*args):pass
mock=http.server.HTTPServer(('127.0.0.1',0),Model)
threading.Thread(target=mock.serve_forever,daemon=True).start()
exe = root / 'c4-amd64.exe'
log = open(str(run / 'serve-stderr.txt'), 'wb')
p = subprocess.Popen([str(exe), '--workspace', str(workspace), '--sandbox-preference', 'jobobject', 'serve'], stdout=subprocess.PIPE, stderr=log, env=env)
evidence = {'started': time.time(), 'binary_sha256': hashlib.sha256(exe.read_bytes()).hexdigest(), 'checks': []}
browser = None
try:
 line = p.stdout.readline().decode('utf-8')
 address = re.search(r'http://127\.0\.0\.1:\d+', line).group()
 probe = socket.socket(); probe.bind(('127.0.0.1', 0)); port = probe.getsockname()[1]; probe.close()
 chrome_log = open(str(run / 'chrome.txt'), 'wb')
 browser = subprocess.Popen([r'C:\Program Files\Google\Chrome\Application\chrome.exe', '--headless', '--disable-gpu', '--remote-debugging-port='+str(port), '--user-data-dir='+str(run/'chrome-profile'), address], stdout=chrome_log, stderr=chrome_log)
 deadline = time.time()+30
 while True:
  try:
   targets = json.load(urllib.request.urlopen('http://127.0.0.1:%d/json'%port, timeout=2))
   target = next(t for t in targets if t['type']=='page'); break
  except Exception:
   if time.time()>deadline: raise
   time.sleep(.2)
 url=urllib.parse.urlsplit(target['webSocketDebuggerUrl'])
 s=socket.create_connection((url.hostname,url.port),10); s.settimeout(10)
 key=base64.b64encode(os.urandom(16)).decode()
 s.sendall(('GET '+url.path+' HTTP/1.1\r\nHost: '+url.netloc+'\r\nUpgrade: websocket\r\nConnection: Upgrade\r\nSec-WebSocket-Key: '+key+'\r\nSec-WebSocket-Version: 13\r\n\r\n').encode())
 response=b''
 while b'\r\n\r\n' not in response: response+=s.recv(1)
 assert b' 101 ' in response
 def exact(n):
  b=b''
  while len(b)<n:
   part=s.recv(n-len(b)); assert part; b+=part
  return b
 counter=0
 def call(method,params):
  global counter
  counter+=1
  data=json.dumps({'id':counter,'method':method,'params':params}).encode(); mask=os.urandom(4)
  head=bytes([0x81,0x80|len(data)]) if len(data)<126 else bytes([0x81,0xfe])+struct.pack('!H',len(data))
  s.sendall(head+mask+bytes(c^mask[n%4] for n,c in enumerate(data)))
  while True:
   a,b=exact(2); n=b&127
   if n==126:n=struct.unpack('!H',exact(2))[0]
   elif n==127:n=struct.unpack('!Q',exact(8))[0]
   m=exact(4) if b&128 else None; body=exact(n)
   if m:body=bytes(c^m[k%4] for k,c in enumerate(body))
   msg=json.loads(body)
   if msg.get('id')==counter:
    assert 'error' not in msg,msg
    return msg['result']
 def evaluate(expression):
  result=call('Runtime.evaluate',{'expression':expression,'returnByValue':True,'awaitPromise':True})
  assert 'exceptionDetails' not in result,result
  return result['result'].get('value')
 deadline=time.time()+20
 while 'SSE 已连接' not in evaluate('document.body ? document.body.innerText : ""'):
  assert time.time()<deadline,'page did not connect SSE'
  time.sleep(.2)
 evidence['checks'].append('embedded Vue page mounted and authenticated SSE connected')
 evidence['browser']=evaluate('navigator.userAgent')
 evaluate("Array.from(document.querySelectorAll('button')).find(b=>b.textContent==='选择工作区').click()")
 deadline=time.time()+10
 while '工作区已选择' not in evaluate('document.body.innerText'):
  assert time.time()<deadline,'workspace selection did not complete'
  time.sleep(.2)
 evidence['checks'].append('workspace selected through page button')
 def click(text):
  evaluate('Array.from(document.querySelectorAll("button")).find(b=>b.textContent==='+json.dumps(text)+').click()')
 def fill(selector,value):
  evaluate('(()=>{const e=document.querySelector('+json.dumps(selector)+');e.value='+json.dumps(value)+';e.dispatchEvent(new Event("input",{bubbles:true}));e.dispatchEvent(new Event("change",{bubbles:true}));})()')
 def wait_text(text):
  deadline=time.time()+25
  while text not in evaluate('document.body.innerText'):
   assert time.time()<deadline,'missing page text: '+text
   time.sleep(.15)
 fill('input','http://127.0.0.1:%d/v1'%mock.server_port)
 fill('label:nth-of-type(2) input','mock-model')
 fill('input[type=password]','fixture-key')
 click('保存全局配置'); wait_text('已保存到用户全局配置')
 click('连接测试'); wait_text('连接成功')
 fill('select','strict')
 time.sleep(.2)
 for prompt,decision in [('C4DENY','拒绝 / 取消'),('C4ALLOW','允许')]:
  fill('textarea',prompt); click('发送任务'); wait_text('权限确认'); click(decision); wait_text('DONE '+prompt)
  if prompt=='C4DENY': assert not (workspace/(prompt+'.txt')).exists()
  else: assert (workspace/(prompt+'.txt')).read_text()=='browser-proof'
  evidence['checks'].append(prompt+' permission through browser')
 click('列出 checkpoint'); time.sleep(.3)
 assert evaluate('document.querySelectorAll("aside select option").length')>1
 evaluate('(()=>{const e=Array.from(document.querySelectorAll("details")).find(e=>e.innerText.includes("tool_result"));e.open=true;e.querySelector("button").click()})()')
 time.sleep(.2)
 evidence['checks'].append('checkpoint list and full tool result button')
 fill('textarea','C4WAIT'); click('发送任务'); time.sleep(.3); click('中断当前轮'); wait_text('interrupted')
 evidence['checks'].append('interrupt through browser')
 fill('textarea','C4CONTINUE'); click('回答并继续'); wait_text('DONE C4CONTINUE')
 evidence['checks'].append('continue same session after interruption')
 current=evaluate('document.querySelector("fieldset:nth-of-type(2) legend").textContent').split(' · ')[-1]
 click(current); wait_text('历史已恢复')
 assert evaluate('document.querySelectorAll("section article strong").length')>0
 evidence['checks'].append('resume and render persistent message history')
 fill('select','strict'); time.sleep(.2)
 fill('textarea','C4BACKGROUND'); click('发送任务'); wait_text('权限确认'); click('允许'); wait_text('DONE C4BACKGROUND')
 deadline=time.time()+20
 while not evaluate('Array.from(document.querySelectorAll("aside article pre")).some(e=>e.textContent.includes("BACKGROUND-LAST"))'):
  assert time.time()<deadline,'background output not pushed to sidebar'
  time.sleep(.2)
 evidence['checks'].append('background output pushed to sidebar without polling button')
 click('列出 checkpoint'); time.sleep(.3)
 fill('aside select','1'); wait_text('工作区相对 HEAD 的变更文件数')
 evidence['checks'].append('selected checkpoint target metadata rendered')
 evidence['body']=evaluate('document.body.innerText')
 evidence['assets']=evaluate("Promise.all(performance.getEntriesByType('resource').filter(x=>x.name.includes('/assets/')).map(async x=>({path:new URL(x.name).pathname,type:(await fetch(x.name)).headers.get('content-type')})))")
 assert any('javascript' in x['type'] for x in evidence['assets']),evidence['assets']
 assert any('text/css' in x['type'] for x in evidence['assets']),evidence['assets']
 evidence['status']='PASS'
 call('Browser.close',{})
except Exception as err:
 evidence['status']='FAIL'; evidence['error']=repr(err); evidence['traceback']=__import__('traceback').format_exc()
 try:evidence['failure_body']=evaluate('document.body ? document.body.innerText : ""')
 except Exception:pass
finally:
 p.terminate(); p.wait(timeout=10); log.close()
 if browser is not None and browser.poll() is None: browser.terminate()
 evidence['ended']=time.time()
 (run/'evidence.json').write_text(json.dumps(evidence,ensure_ascii=False,indent=2),encoding='utf-8')
 print(json.dumps(evidence,ensure_ascii=True))
 mock.shutdown()
