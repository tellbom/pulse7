import paramiko,getpass,pathlib,json,sys
local=pathlib.Path(__file__).resolve().parent
repo=local.parent.parent
c=paramiko.SSHClient();c.load_system_host_keys();c.set_missing_host_key_policy(paramiko.AutoAddPolicy())
c.connect('192.168.140.128',port=22,username='user',password=getpass.getpass('VM password: '),look_for_keys=False,allow_agent=False,timeout=15)
key=getpass.getpass('Official model key (memory only): ');s=c.open_sftp()
root='C:/Users/user/h-validation'
s.put(str(local/'model-guest-run.py'),root+'/phase3-model-guest-run.py')
print('READY',flush=True)
for line in sys.stdin:
 a=json.loads(line); name=a['name']; ws=root+'/'+name+'-workspace'
 s.mkdir(ws)
 for filename,content in a.pop('files').items():
  with s.open(ws+'/'+filename,'wb') as f:f.write(content.encode('utf-8'))
 a.update(root=root,workspace=ws,exe=root+'/phase3-final-amd64.exe',prompt_file=root+'/'+name+'-prompt.txt')
 with s.open(a['prompt_file'],'wb') as f:f.write(a.pop('prompt').encode('utf-8'))
 with s.open(root+'/'+name+'-spec.json','wb') as f:f.write(json.dumps(a).encode('utf-8'))
 i,o,e=c.exec_command(root+'/python37/python.exe '+root+'/phase3-model-guest-run.py '+root+'/'+name+'-spec.json',timeout=3600)
 i.write(key+'\n');i.flush();i.channel.shutdown_write()
 result=o.read().decode('utf-8','replace')+e.read().decode('utf-8','replace')
 assert key not in result,'secret found in output'
 print(result,flush=True)
 for suffix in ['.log','.jsonl','-meta.json','-audit.jsonl','-spec.json']:
  try:
   with s.open(root+'/'+name+suffix,'rb') as f: content=f.read()
   assert key.encode() not in content,'secret found in artifact'
   (local/(name+suffix)).write_bytes(content)
  except OSError as err:print('FETCH FAILED '+suffix+': '+str(err),flush=True)
 print('EXIT_CODE='+str(o.channel.recv_exit_status()),flush=True)
