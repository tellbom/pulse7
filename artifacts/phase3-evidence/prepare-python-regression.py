import pathlib,shutil,subprocess,json
root=pathlib.Path(__file__).resolve().parent
source=root/'python37'; target=root/'phase3-python-regression'
assert not target.exists(),'fixture runtime already exists'
shutil.copytree(str(source),str(target))
pth=target/'python37._pth'
assert pth.is_file()
before=pth.read_text()
pth.rename(target/'python37._pth.disabled')
fixture=root/'phase3-python-import-check';fixture.mkdir()
(fixture/'module_local.py').write_text('VALUE=42\n')
(fixture/'main.py').write_text('from module_local import VALUE\nprint(VALUE)\n')
result=subprocess.run([str(target/'python.exe'),'main.py'],cwd=str(fixture),stdout=subprocess.PIPE,stderr=subprocess.STDOUT)
evidence={'source_unchanged':(source/'python37._pth').read_text()==before,'isolated_copy':str(target),'disabled_in_copy_only':'python37._pth','exit':result.returncode,'output':result.stdout.decode('utf-8','replace')}
assert evidence['source_unchanged'] and result.returncode==0,evidence
print(json.dumps(evidence))
