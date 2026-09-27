from pathlib import Path
import subprocess,uuid,time,os,json,datetime
root=Path('/Users/jack/src/github.com/agentstation/starport-native-catalog');out=Path('/tmp/d41-final-task-gate');out.mkdir(exist_ok=True)
source_diff=subprocess.check_output(['git','diff','HEAD','--'],cwd=root)
source_files=sorted(root.glob('internal/catalog/*.go'))
import hashlib
source_hashes={str(p.relative_to(root)):hashlib.sha256(p.read_bytes()).hexdigest() for p in source_files}
cids=[];report={'started':datetime.datetime.now(datetime.timezone.utc).isoformat()};env=dict(os.environ,GOTOOLCHAIN='go1.27.1',GOWORK='off')
try:
 services=[('TEST_VALKEY_REPLACEMENT_URL','valkey/valkey@sha256:9acdf6f0ae1771ea63c401e127054b2d1779227b9230dcfae37fa684610eaa4f',6379,[],['valkey-cli','ping']),('TEST_FRESH_VALKEY_URL','valkey/valkey@sha256:9acdf6f0ae1771ea63c401e127054b2d1779227b9230dcfae37fa684610eaa4f',6379,[],['valkey-cli','ping']),('TEST_VALKEY_URL','valkey/valkey@sha256:9acdf6f0ae1771ea63c401e127054b2d1779227b9230dcfae37fa684610eaa4f',6379,[],['valkey-cli','ping']),('TEST_POSTGRES_URL','postgres@sha256:f1c3376c26f2609ab9f29f71f824103fe2fcd8ee0346485cb6122a4f93df6f94',5432,['-e','POSTGRES_HOST_AUTH_METHOD=trust','-e','POSTGRES_DB=starport_test'],['pg_isready','-U','postgres','-d','starport_test'])]
 report['images']=[s[1] for s in services]
 report['source_sha256']=source_hashes
 report['producer_sha256']={str(p.relative_to(Path('/Users/jack/src/github.com/agentstation/starmap-object-ci-repair'))):hashlib.sha256(p.read_bytes()).hexdigest() for p in Path('/Users/jack/src/github.com/agentstation/starmap-object-ci-repair/runtime').glob('*.go')}
 report['environment']={'GOTOOLCHAIN':'go1.27.1','GOWORK':'off'}
 for name,image,port,args,ready in services:
  cid=subprocess.check_output(['docker','run','-d','--name','csp11-fleet-'+uuid.uuid4().hex[:8],'-p',f'127.0.0.1::{port}',*args,image],text=True).strip();cids.append(cid)
  for _ in range(100):
   if subprocess.run(['docker','exec',cid,*ready],capture_output=True).returncode==0:break
   time.sleep(.2)
  else:raise RuntimeError('backend did not start')
  address=subprocess.check_output(['docker','port',cid,f'{port}/tcp'],text=True).strip()
  env[name]='redis://'+address if port==6379 else 'postgres://postgres@'+address+'/starport_test?sslmode=disable'
 cmd=['bash','/Users/jack/src/github.com/agentstation/starmap-object-ci-repair/scripts/verify-catalog-product.sh','--starport-root',str(root),'--task','CSP11','--json'];report['command']=cmd
 with (out/'task-report.json').open('w') as log,(out/'stderr.log').open('w') as stderr:
  result=subprocess.run(cmd,cwd=root,env=env,stdout=log,stderr=stderr,timeout=700)
 report['exit_code']=result.returncode
finally:
 report['cleanup']=[subprocess.run(['docker','rm','-f',c],capture_output=True).returncode for c in cids]
 report['finished']=datetime.datetime.now(datetime.timezone.utc).isoformat();(out/'runner.json').write_text(json.dumps(report,indent=2)+'\n');print(json.dumps(report))

raise SystemExit(report.get("exit_code",1))
