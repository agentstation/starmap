import hashlib,json,pathlib,subprocess,time,urllib.request,urllib.error,sys
state=json.loads(pathlib.Path('/tmp/starport-csp02-compose-state.json').read_text())
root=pathlib.Path(state['root']); project=state['project']
base=['docker','compose','-p',project,'-f','docker-compose.yml','-f','compose.verify.yaml']
report={'source_commit':state['source_commit'],'platform':'linux/arm64','commands':[],'http':[]}
key=''
def run(args,timeout=90):
 r=subprocess.run(base+args,cwd=root,capture_output=True,text=True,timeout=timeout)
 report['commands'].append({'args':args,'exit_code':r.returncode})
 if r.returncode: raise RuntimeError('compose failed: '+str(args))
 return r.stdout

def request(path,body=None,auth=True):
 headers={'Authorization':'Bearer '+key} if auth else {}
 if body is not None: headers['Content-Type']='application/json'
 req=urllib.request.Request('http://127.0.0.1:19324'+path,data=json.dumps(body).encode() if body is not None else None,headers=headers)
 try:
  with urllib.request.urlopen(req,timeout=3) as r: status=r.status; data=r.read()
 except urllib.error.HTTPError as e: status=e.code; data=e.read()
 report['http'].append({'path':path,'method':'POST' if body else 'GET','status':status,'authenticated':auth})
 return status,json.loads(data)

def ready():
 for _ in range(120):
  try:
   with urllib.request.urlopen('http://127.0.0.1:19324/health/ready',timeout=1) as r:
    if r.status==200: return
  except (OSError,urllib.error.URLError): pass
  time.sleep(.5)
 raise RuntimeError('gateway did not become ready')
try:
 run(['up','--build','-d','valkey'])
 key=json.loads(run(['run','--rm','starport','init','--configured-storage','--name','primary-admin','--json']))['api_key']
 run(['run','--rm','starport','auth','rotate','--json'])
 run(['up','-d','starport']); ready()
 assert request('/api/v1/models')[0]==200
 assert request('/api/v1/admin/account-templates',{'id':'compose-persistence','name':'Compose persistence'})[0]==201
 assert request('/api/v1/admin/account-templates/compose-persistence')[0]==200
 run(['up','-d','--force-recreate','starport']); ready()
 assert request('/api/v1/models')[0]==200
 status,_=request('/api/v1/admin/account-templates/compose-persistence')
 report['relational_record_retained']=status==200
 report['verdict']='PASS' if status==200 else 'FAIL'
except Exception as exc:
 report['verdict']='FAIL'; report['error']=str(exc)
finally:
 try: run(['down','--volumes','--remove-orphans'])
 except Exception as exc: report['cleanup_error']=str(exc)
 destination=pathlib.Path(sys.argv[1])
 destination.write_text(json.dumps(report,indent=2)+'\n')
 print(json.dumps(report,indent=2))
