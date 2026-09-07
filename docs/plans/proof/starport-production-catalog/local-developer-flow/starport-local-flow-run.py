import pathlib,json,os,subprocess,time,urllib.request,http.server,threading,signal
root=pathlib.Path('/tmp/starport-local-flow-root').read_text(); root=pathlib.Path(root)
processes=[]
def launch(name,args,env):
 log=open(root/(name+'.log'),'w');p=subprocess.Popen(args,env=env,stdout=log,stderr=log); processes.append(p);return p
def read(url,method='GET',token=None):
 headers={'Authorization':'Bearer '+token} if token else {}
 req=urllib.request.Request(url,headers=headers,method=method)
 with urllib.request.urlopen(req,timeout=5) as response:return json.load(response)
def wait_ready(url,p):
 for _ in range(100):
  if p.poll() is not None: raise RuntimeError('server exited '+str(p.returncode))
  try: return read(url)
  except Exception:time.sleep(.2)
 raise RuntimeError('readiness timeout')
def run(name,args,env):
 result=subprocess.run(args,env=env,capture_output=True,text=True,timeout=60)
 (root/(name+'.log')).write_text(result.stdout+result.stderr)
 if result.returncode:raise RuntimeError(name+' failed: '+str(result.returncode))
 return result.stdout
def model_limit(d):
 if isinstance(d,dict):
  if d.get('id')=='gpt-4o-mini':return d.get('limits',{}).get('context_window')
  children=d.values()
 elif isinstance(d,list):children=d
 else:return None
 for child in children:
  value=model_limit(child)
  if value is not None:return value
 return None
base={'PATH':os.environ['PATH'],'HOME':str(root/'home')}
spenv=base|{'STARPORT_CONFIG_DIR':str(root/'starport-config'),'STARPORT_CATALOG_STATE_DIR':str(root/'starport-state'),'STARPORT_CATALOG_SOURCE':'embedded','STARPORT_CATALOG_SOURCE_POLL_INTERVAL':'0s','STARPORT_CATALOG_ACQUISITION_ENABLED':'false','STARPORT_SERVER_PORT':'19323'}
smenv=base|{'STARMAP_HOME':str(root/'starmap'),'STARMAP_CATALOG_SOURCE':'embedded','STARMAP_CATALOG_SOURCE_POLL_INTERVAL':'0s','STARMAP_CATALOG_ACQUISITION_ENABLED':'false','STARMAP_CATALOG_WORKSPACE_PATH':str(root/'workspace'),'OPENAI_API_KEY':'local-source-fixture'}
fixture_models=[{'id':'gpt-4o-mini','object':'model','created':1,'owned_by':'openai','context_window':9999}]
class Fixture(http.server.BaseHTTPRequestHandler):
 def do_GET(self):
  self.send_response(200);self.send_header('Content-Type','application/json');self.end_headers();self.wfile.write(json.dumps({'object':'list','data':fixture_models}).encode())
 def log_message(self,*args):pass
server=http.server.ThreadingHTTPServer(('127.0.0.1',19321),Fixture);threading.Thread(target=server.serve_forever,daemon=True).start()
try:
 initial=json.loads(run('init',['/tmp/starport-local-flow-gateway','init','--name','flow-admin','--json'],spenv)); token=initial['api_key']
 p=launch('starport-offline',['/tmp/starport-local-flow-gateway','serve'],spenv);wait_ready('http://127.0.0.1:19323/health/ready',p)
 status=read('http://127.0.0.1:19323/api/v1/admin/catalog/status',token=token);(root/'starport-offline-status.json').write_text(json.dumps(status,indent=2));print('Starport offline ready',flush=True)
 p.send_signal(signal.SIGINT);p.wait(timeout=15)
 run('starmap-update',['/tmp/starport-local-flow-starmap','update','openai','--source','provider-api','--catalog-path',str(root/'workspace'),'-y'],smenv);print('CLI update complete',flush=True)
 p=launch('starmap-server',['/tmp/starport-local-flow-starmap','serve','--host','127.0.0.1','--port','19322'],smenv);wait_ready('http://127.0.0.1:19322/api/v1/stats',p);print('Starmap server ready',flush=True)
 assert model_limit(read('http://127.0.0.1:19322/api/v1/providers/openai/models'))==9999,'acquisition did not change model limit'
 (root/'starmap-acquired-models.json').write_text(json.dumps(read('http://127.0.0.1:19322/api/v1/providers/openai/models'),indent=2))
 (root/'starmap-stats.json').write_text(json.dumps(read('http://127.0.0.1:19322/api/v1/stats'),indent=2))
 spenv|={'STARPORT_CATALOG_SOURCE':'starmap','STARPORT_CATALOG_SOURCE_URL':'http://127.0.0.1:19322/api/v1'}
 p=launch('starport-connected',['/tmp/starport-local-flow-gateway','serve'],spenv);wait_ready('http://127.0.0.1:19323/health/ready',p)
 operation=read('http://127.0.0.1:19323/api/v1/admin/catalog/refresh',method='POST',token=token)
 print('Starport refresh response',json.dumps(operation),flush=True)
 time.sleep(4)
 (root/'starport-connected-status.json').write_text(json.dumps(read('http://127.0.0.1:19323/api/v1/admin/catalog/status',token=token),indent=2));print('Starport connected ready',flush=True)
 fixture_models[0].pop('context_window')
 accepted=read('http://127.0.0.1:19322/api/v1/update?provider=openai&source=providers&fresh=true',method='POST')
 operation_id=accepted['data']['id']
 for _ in range(120):
  update=read('http://127.0.0.1:19322/api/v1/updates/'+operation_id)
  if update['data']['state'] not in ('accepted','running'):break
  time.sleep(1)
 assert update['data']['state']=='succeeded','reset failed'
 assert model_limit(read('http://127.0.0.1:19322/api/v1/providers/openai/models'))==128000,'reset did not restore baseline limit'
 (root/'starmap-reset-models.json').write_text(json.dumps(read('http://127.0.0.1:19322/api/v1/providers/openai/models'),indent=2))
 (root/'starmap-reset-operation.json').write_text(json.dumps(update,indent=2));print('Reset operation',json.dumps(update),flush=True)
 operation=read('http://127.0.0.1:19323/api/v1/admin/catalog/refresh',method='POST',token=token)
 time.sleep(4)
 (root/'starport-reset-status.json').write_text(json.dumps(read('http://127.0.0.1:19323/api/v1/admin/catalog/status',token=token),indent=2))
 reset_status=read('http://127.0.0.1:19323/api/v1/admin/catalog/status',token=token)
 p.send_signal(signal.SIGINT);p.wait(timeout=15)
 p=launch('starport-restarted',['/tmp/starport-local-flow-gateway','serve'],spenv);wait_ready('http://127.0.0.1:19323/health/ready',p)
 restart_status=read('http://127.0.0.1:19323/api/v1/admin/catalog/status',token=token)
 (root/'starport-restart-status.json').write_text(json.dumps(restart_status,indent=2))
 assert restart_status['snapshot']['generation_id']==reset_status['snapshot']['generation_id'],'restart changed generation'
 assert restart_status['snapshot']['payload_checksum']==reset_status['snapshot']['payload_checksum'],'restart changed payload'
 print('Starport restart preserved accepted generation',flush=True)

finally:
 for p in reversed(processes):
  if p.poll() is None:p.send_signal(signal.SIGINT)
 for p in processes:
  try:p.wait(timeout=15)
  except subprocess.TimeoutExpired:p.kill();p.wait()
 server.shutdown()
