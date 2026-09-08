from pathlib import Path
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
import hashlib, json, os, shutil, subprocess, tempfile, threading
source=Path(Path('/tmp/starport-local-flow-root').read_text().strip())/'workspace'
root=Path(tempfile.mkdtemp(prefix='starmap-fresh-preview-'))
workspace=root/'workspace'
shutil.copytree(source,workspace)
requests=[]
class Fixture(BaseHTTPRequestHandler):
 def do_GET(self):
  requests.append(self.path)
  self.send_response(200); self.send_header('Content-Type','application/json'); self.end_headers()
  self.wfile.write(json.dumps({'object':'list','data':[{'id':'gpt-4o-mini','object':'model','created':1,'owned_by':'openai'}]}).encode())
 def log_message(self,*args): pass
server=ThreadingHTTPServer(('127.0.0.1',0),Fixture)
threading.Thread(target=server.serve_forever,daemon=True).start()
p=workspace/'providers.yaml'
p.write_text(p.read_text().replace('http://127.0.0.1:19321',f'http://127.0.0.1:{server.server_port}'))
def hashes(): return {str(p.relative_to(workspace)):hashlib.sha256(p.read_bytes()).hexdigest() for p in workspace.rglob('*') if p.is_file()}
before=hashes()
env={k:os.environ[k] for k in ['HOME','PATH','TMPDIR'] if k in os.environ}
env.update(STARMAP_HOME=str(root/'state'),STARMAP_CATALOG_SOURCE='embedded',STARMAP_CATALOG_SOURCE_POLL_INTERVAL='0s',STARMAP_CATALOG_ACQUISITION_ENABLED='false',STARMAP_CATALOG_WORKSPACE_PATH=str(workspace),OPENAI_API_KEY='local-source-fixture')
args=['/tmp/starmap-fresh-cli','update','openai','--source','provider-api','--catalog-path',str(workspace),'--fresh','--dry-run']
try:
 result=subprocess.run(args,env=env,input='n\n',text=True,stdout=subprocess.PIPE,stderr=subprocess.PIPE,timeout=60)
 Path('/tmp/starmap-fresh-cli-preview.log').write_text(result.stdout+result.stderr)
 summary={'command':args,'exit_code':result.returncode,'fixture_requests':requests,'workspace_files':len(before),'workspace_unchanged':before==hashes(),'confirmation_absent':'(y/N)' not in result.stdout+result.stderr,'root':str(root)}
 Path('/tmp/starmap-fresh-cli-preview.json').write_text(json.dumps(summary,indent=2)+'\n')
 print(json.dumps(summary))
 assert result.returncode==0, 'CLI preview failed; inspect retained log'
 assert requests and all(p=='/models' for p in requests),requests
 assert summary['workspace_unchanged'] and summary['confirmation_absent']
 assert 'Acquisition resets: 1' in result.stderr
 apply_args=args[:-1]+['-y']
 applied=subprocess.run(apply_args,env=env,input='n\n',text=True,stdout=subprocess.PIPE,stderr=subprocess.PIPE,timeout=60)
 Path('/tmp/starmap-fresh-cli-apply.log').write_text(applied.stdout+applied.stderr)
 apply_summary={'command':apply_args,'exit_code':applied.returncode,'fixture_requests':list(requests),'confirmation_absent':'(y/N)' not in applied.stdout+applied.stderr,'reset_count_visible':'Acquisition resets: 1' in applied.stderr,'completion_count_visible':'acquisition resets: 1' in applied.stderr}
 Path('/tmp/starmap-fresh-cli-apply.json').write_text(json.dumps(apply_summary,indent=2)+'\n')
 print(json.dumps(apply_summary))
 assert applied.returncode==0 and apply_summary['confirmation_absent'] and apply_summary['reset_count_visible'] and apply_summary['completion_count_visible']
 assert requests==['/models','/models']

finally:
 server.shutdown();server.server_close()
