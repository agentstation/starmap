from pathlib import Path
import os,json,subprocess,time,signal,urllib.request,urllib.error
root=Path(Path('/tmp/starport-local-flow-root').read_text())
token=json.loads((root/'init.log').read_text().splitlines()[0])['api_key']
import sys
key=sys.stdin.readline().strip()
if not key:raise RuntimeError('No supplied credential')
env={'PATH':os.environ['PATH'],'HOME':str(root/'home'),'STARPORT_CONFIG_DIR':str(root/'starport-config'),'STARPORT_CATALOG_STATE_DIR':str(root/'starport-state'),'STARPORT_CATALOG_SOURCE':'starmap','STARPORT_CATALOG_SOURCE_URL':'http://127.0.0.1:19322/api/v1','STARPORT_CATALOG_SOURCE_POLL_INTERVAL':'0s','STARPORT_CATALOG_ACQUISITION_ENABLED':'false','STARPORT_SERVER_PORT':'19323','OPENAI_API_KEY':key}
log=open(root/'starport-inference.log','w');p=subprocess.Popen(['/tmp/starport-local-flow-gateway','serve'],env=env,stdout=log,stderr=log)
try:
 for _ in range(100):
  if p.poll() is not None:raise RuntimeError('Gateway startup failed')
  try:
   with urllib.request.urlopen('http://127.0.0.1:19323/health/ready',timeout=2) as response:break
  except Exception:time.sleep(.2)
 else:raise RuntimeError('Gateway readiness timeout')
 headers={'Authorization':'Bearer '+token,'Content-Type':'application/json'}
 req=urllib.request.Request('http://127.0.0.1:19323/api/v1/admin/catalog/status',headers=headers)
 with urllib.request.urlopen(req,timeout=5) as response:status=json.load(response)
 previous=json.loads((root/'starport-reset-status.json').read_text())
 assert status['snapshot']['generation_id']==previous['snapshot']['generation_id'],'restart lost accepted generation'
 payload={'model':'openai/gpt-4o-mini','messages':[{'role':'user','content':'Reply with exactly: Starport catalog flow verified.'}],'stream':True,'max_tokens':32}
 req=urllib.request.Request('http://127.0.0.1:19323/v1/chat/completions',data=json.dumps(payload).encode(),headers=headers)
 started=time.monotonic()
 try:
  with urllib.request.urlopen(req,timeout=60) as response:
   data=response.read().decode();httpstatus=response.status
 except urllib.error.HTTPError as e:
  body=e.read().decode(); body=body.replace(key,'[redacted]').replace(token,'[redacted]')
  (root/'inference-failure.json').write_text(json.dumps({'http_status':e.code,'body':body}));print('Inference HTTP status',e.code);raise SystemExit(1)
 chunks=[json.loads(x[6:]) for x in data.splitlines() if x.startswith('data: {')]
 content=''.join(c.get('choices',[{}])[0].get('delta',{}).get('content','') or '' for c in chunks if c.get('choices'))
 assert '[DONE]' in data and content,'incomplete stream'
 proof={'http_status':httpstatus,'stream_done':True,'chunk_count':len(chunks),'response_text':content,'max_output_tokens':32,'model':payload['model'],'generation_id':status['snapshot']['generation_id'],'payload_checksum':status['snapshot']['payload_checksum'],'catalog_source_available':False,'elapsed_seconds':round(time.monotonic()-started,3),'credential_source':'owner-supplied credential through stdin; value omitted'}
 (root/'inference-proof.json').write_text(json.dumps(proof,indent=2));print(json.dumps(proof))
finally:
 if p.poll() is None:p.send_signal(signal.SIGINT)
 try:p.wait(timeout=15)
 except subprocess.TimeoutExpired:p.kill();p.wait()
