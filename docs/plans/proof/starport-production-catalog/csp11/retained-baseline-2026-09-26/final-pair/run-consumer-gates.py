from pathlib import Path
import subprocess,os,datetime,json,shlex
root=Path('/Users/jack/src/github.com/agentstation/starport-native-catalog'); out=Path('/tmp/d41-final-consumer-gates');out.mkdir(exist_ok=True)
source=(root/'AGENTS.md').read_text();block=source.split('Before a pull request, run:',1)[1].split('```bash',1)[1].split('```',1)[0]
commands=[s.strip() for s in block.splitlines() if s.strip()]
env=dict(os.environ,GOTOOLCHAIN='go1.27.1',GOWORK='off',CATALOG_DRIVEN_STARMAP_ROOT=str(root.parent/'starmap-object-ci-repair'),STARMAP_OWNERSHIP_STARMAP_ROOT=str(root.parent/'starmap-object-ci-repair'))
report={'started':datetime.datetime.now(datetime.timezone.utc).isoformat(),'source':str(root/'AGENTS.md'),'workspace':env['GOWORK'],'results':[]}
for i,cmd in enumerate(commands):
 item={'command':cmd,'started':datetime.datetime.now(datetime.timezone.utc).isoformat()};print('RUN '+cmd,flush=True)
 with (out/f'{i:02}.log').open('w') as log:
  try:r=subprocess.run(shlex.split(cmd),cwd=root,env=env,stdout=log,stderr=subprocess.STDOUT,timeout=900);item['exit_code']=r.returncode
  except subprocess.TimeoutExpired:item['timeout_seconds']=900
 item['finished']=datetime.datetime.now(datetime.timezone.utc).isoformat();report['results'].append(item);(out/'report.json').write_text(json.dumps(report,indent=2)+'\n')
 print('RESULT '+str(item.get('exit_code','timeout')),flush=True)
 if item.get('exit_code')!=0:break
report['finished']=datetime.datetime.now(datetime.timezone.utc).isoformat();(out/'report.json').write_text(json.dumps(report,indent=2)+'\n')

raise SystemExit(0 if len(report["results"])==len(commands) and all(r.get("exit_code")==0 for r in report["results"]) else 1)
