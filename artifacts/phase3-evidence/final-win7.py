import pathlib, subprocess, hashlib, json, time, os
root=pathlib.Path(__file__).resolve().parent
exe=root/'phase3-final-amd64.test.exe'
started=time.time()
boot=subprocess.check_output(['wmic','os','get','lastbootuptime'],stderr=subprocess.STDOUT).decode('utf-8','replace')
env=os.environ.copy(); env['H_PRODUCT']=str(root/'phase3-final-amd64.exe')
env['PATH']=str(root/'runtime/git/cmd')+os.pathsep+env.get('PATH','')
git_version=subprocess.check_output([str(root/'runtime/git/cmd/git.exe'),'--version'],env=env).decode('utf-8','replace').strip()
result=subprocess.run([str(exe),'-test.v','-test.timeout=180s'],env=env,stdout=subprocess.PIPE,stderr=subprocess.STDOUT)
output=result.stdout.decode('utf-8','replace')
evidence={'started':started,'ended':time.time(),'boot':boot,'git_version':git_version,'sha256':hashlib.sha256(exe.read_bytes()).hexdigest(),'exit_code':result.returncode,'passed':sum(x.startswith('--- PASS:') for x in output.splitlines()),'failed':[x for x in output.splitlines() if x.startswith('--- FAIL:')],'skipped':[x for x in output.splitlines() if x.startswith('--- SKIP:')],'output':output}
print(json.dumps(evidence,ensure_ascii=True))
