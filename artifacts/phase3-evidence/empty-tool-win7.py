import os,pathlib,subprocess,json,hashlib,time
root=pathlib.Path(__file__).resolve().parent
env=os.environ.copy()
env['H_PRODUCT']=str(root/'empty-tool-amd64.exe')
env['PATH']=str(root/'runtime/git/cmd')+os.pathsep+env.get('PATH','')
test=root/'empty-tool-amd64.test.exe'
started=time.time()
boot=subprocess.check_output(['wmic','os','get','lastbootuptime']).decode('utf-8','replace')
run=subprocess.run([str(test),'-test.v','-test.timeout=180s'],env=env,stdout=subprocess.PIPE,stderr=subprocess.STDOUT)
output=run.stdout.decode('utf-8','replace')
print(json.dumps({'boot':boot,'started':started,'ended':time.time(),'test_sha256':hashlib.sha256(test.read_bytes()).hexdigest(),'product_sha256':hashlib.sha256((root/'empty-tool-amd64.exe').read_bytes()).hexdigest(),'exit':run.returncode,'passed':sum(l.startswith('--- PASS:') for l in output.splitlines()),'failed':[l for l in output.splitlines() if l.startswith('--- FAIL:')],'skipped':[l for l in output.splitlines() if l.startswith('--- SKIP:')],'output':output}))
