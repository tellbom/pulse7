import paramiko,pathlib,getpass
local=pathlib.Path(__file__).resolve().parent
c=paramiko.SSHClient();c.load_system_host_keys();c.set_missing_host_key_policy(paramiko.AutoAddPolicy())
c.connect('192.168.140.128',username='user',password=getpass.getpass('VM password: '),look_for_keys=False,allow_agent=False,timeout=15)
key=getpass.getpass('Model key (memory only): ')
s=c.open_sftp();s.put(str(local/'empty-tool-live.py'),'C:/Users/user/h-validation/empty-tool-live.py')
i,o,e=c.exec_command('C:/Users/user/h-validation/python37/python.exe C:/Users/user/h-validation/empty-tool-live.py',timeout=240)
i.write(key+'\n');i.flush();i.channel.shutdown_write()
out=o.read().decode('utf-8','replace')+e.read().decode('utf-8','replace')
assert key not in out
(local/'empty-tool-live-win7.txt').write_text(out,encoding='utf-8')
print(out);print('EXIT',o.channel.recv_exit_status());c.close()
