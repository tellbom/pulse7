from pathlib import Path
import json,re,subprocess,hashlib
root=Path(__file__).resolve().parent
r=json.loads((root/'c2-results.json').read_text(encoding='utf-8'))
notice='[JobObject] Win7 兼容模式：仅主动加入会话 Job 的直接进程保证退出清理；第三方后代可脱离，不保证清理，也不视为异常。'
def norm(s,remove_notice=False):
 lines=s.splitlines()
 if remove_notice:lines=[l for l in lines if l!=notice]
 return '\n'.join(re.sub(r'(=== EXIT DONE code=\d+ )[^ ]+( ===)',r'\1<TIMESTAMP>\2',l) for l in lines)
a=r['cases']; summary={'normalized_fields':['EXIT DONE timestamp only'],'allowed_added_lines':[{'text':notice,'source':'H-Win7-Silent-Breakaway-Decision.md'}], 'matched_30_stdout_equal':norm(a['rc007']['stdout'])==norm(a['c2matched']['stdout'],True),'matched_30_stderr_equal':a['rc007']['stderr']==a['c2matched']['stderr'],'default_stdout_equal':norm(a['rc007']['stdout'])==norm(a['c2default']['stdout'],True),'default_known_difference':'30 vs 100: NOT MET; not normalized','all_case_exit_zero':all(x['exit']==0 for x in a.values()),'protocol_types':[json.loads(l)['type'] for l in a['c2json']['stdout'].splitlines()],'protocol_human_matches_text':norm(a['c2json']['stderr'])==norm(a['c2default']['stdout']),'test_pass_count':r['test']['stdout'].count('--- PASS:'),'test_exit':r['test']['exit']}
old=subprocess.check_output(['git','show','301ec11:agent/events.go']).decode('utf-8')
new=Path('agent/events_cli.go').read_text(encoding='utf-8')
summary['cli_renderer_moved_verbatim']=old[old.index('type cliEventRenderer struct'):old.index('type eventBus struct')].strip()==new[new.index('type cliEventRenderer struct'):new.index('// CLI setup')].strip()
(root/'c2-comparison.json').write_text(json.dumps(summary,ensure_ascii=False,indent=2),encoding='utf-8')
print(json.dumps(summary,ensure_ascii=False,indent=2))
diff=subprocess.check_output(['git','diff','--ignore-cr-at-eol','rc-0.7','301ec11','--','agent/*.go']).decode('utf-8')
rows=[];file=''
for l in diff.splitlines():
 if l.startswith('+++ b/'):file=l[6:]
 if l.startswith('+') and not l.startswith('+++') and re.search(r'\b(out|outln|outPrint)\(',l):rows.append((file,l[1:].strip()))
(root/'c2-prior-output-additions.txt').write_text('\n'.join(f'{f}: {l}' for f,l in rows),encoding='utf-8')
print('prior output additions:',len(rows))
