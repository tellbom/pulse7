import subprocess,pathlib,time,json,hashlib
root=pathlib.Path(__file__).resolve().parent
exe=root/'phase3-final-amd64.exe'
log=open(str(root/'phase3-cli-listener-stderr.txt'),'wb')
p=subprocess.Popen([str(exe),'--sandbox-preference','jobobject','--workspace',str(root),'repl'],stdin=subprocess.PIPE,stdout=subprocess.PIPE,stderr=log)
evidence={'started':time.time(),'sha256':hashlib.sha256(exe.read_bytes()).hexdigest(),'pid':p.pid}
try:
 time.sleep(2)
 assert p.poll() is None,'CLI exited before observation'
 raw=subprocess.check_output(['netstat','-ano','-p','tcp']).decode('utf-8','replace')
 rows=[line.strip() for line in raw.splitlines() if line.split() and line.split()[-1]==str(p.pid)]
 evidence['tcp_rows']=rows
 assert not any('LISTENING' in line for line in rows),rows
 evidence['status']='PASS'
finally:
 p.stdin.write(b'/exit\n'); p.stdin.flush()
 try:p.wait(timeout=10)
 except subprocess.TimeoutExpired:p.terminate(); p.wait(timeout=5)
 log.close();evidence['ended']=time.time()
 evidence['historical_fixture_paths']={str(path):path.exists() for path in [pathlib.Path('C:/Users/user/T7/process-copy'),pathlib.Path('C:/Users/user/f1f4-validation/live')]}
 print(json.dumps(evidence))
