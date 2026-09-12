import subprocess,pathlib,json,datetime,hashlib,os,http.server,threading,re
root=pathlib.Path(__file__).resolve().parent
out=root/'c2-evidence'
out.mkdir(exist_ok=True)
def run(label,args):
    log=out/(label+'.json')
    if log.exists():log.unlink()
    started=datetime.datetime.now().isoformat()
    p=subprocess.run(args,stdout=subprocess.PIPE,stderr=subprocess.PIPE,timeout=120)
    result=dict(started=started,ended=datetime.datetime.now().isoformat(),argv=args,exit=p.returncode,stdout=p.stdout.decode('utf-8','replace'),stderr=p.stderr.decode('utf-8','replace'))
    log.write_text(json.dumps(result,ensure_ascii=False,indent=2),encoding='utf-8')
    return result
exe=root/'c2-amd64.test.exe'
test=run('events',[str(exe),'-test.v','-test.run','Test(EventConsumer|StreamJSON|TurnResult|SessionInit)'])
class Mock(http.server.BaseHTTPRequestHandler):
    def do_POST(self):
        self.rfile.read(int(self.headers.get('Content-Length','0')))
        self.send_response(200);self.send_header('Content-Type','text/event-stream');self.end_headers()
        self.wfile.write(b'data: {"choices":[{"index":0,"delta":{"content":"CLI_COMPARE_OK"}}]}\n\ndata: {"choices":[{"index":0,"delta":{},"finish_reason":"stop"}]}\n\ndata: [DONE]\n\n')
    def log_message(self,*args):pass
server=http.server.HTTPServer(('127.0.0.1',0),Mock)
threading.Thread(target=server.serve_forever,daemon=True).start()
workspace=out/'workspace';workspace.mkdir(exist_ok=True)
results={}
for label,product,extra in [('rc007','c2-rc007.exe',[]),('c2matched','c2-amd64.exe',['--max-rounds','30']),('c2default','c2-amd64.exe',[]),('c2json','c2-amd64.exe',['--output-format','stream-json'])]:
    args=[str(root/product),'--base-url','http://127.0.0.1:%d/v1'%server.server_port,'--api-key','dummy','--model','mock-model','--workspace',str(workspace),'--sandbox-preference','jobobject','--session',str(out/(label+'.jsonl'))]+extra+['exec','CLI_COMPARE']
    session=out/(label+'.jsonl')
    if session.exists():session.unlink()
    results[label]=run(label,args)
server.shutdown()
summary={'test':test,'cases':results,'hashes':{n:hashlib.sha256((root/n).read_bytes()).hexdigest() for n in ['c2-amd64.test.exe','c2-amd64.exe','c2-rc007.exe']}}
(out/'results.json').write_text(json.dumps(summary,ensure_ascii=False,indent=2),encoding='utf-8')
print(json.dumps(summary,ensure_ascii=True))
