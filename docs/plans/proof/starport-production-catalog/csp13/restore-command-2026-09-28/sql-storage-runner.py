import json,subprocess,urllib.parse,os
names=['starport-csp13-postgres','starport-csp13-mysql']
containers=json.loads(subprocess.check_output(['docker','inspect',*names]))
env=os.environ.copy()
for c in containers:
 name=c['Name'].lstrip('/')
 values=dict(v.split('=',1) for v in c['Config']['Env'] if '=' in v)
 ports=c['NetworkSettings']['Ports']
 if 'postgres' in name:
  port=ports['5432/tcp'][0]['HostPort']
  user=values.get('POSTGRES_USER','postgres');database=values.get('POSTGRES_DB',user)
  env['TEST_POSTGRES_URL']='postgres://'+urllib.parse.quote(user,safe='')+':'+urllib.parse.quote(values['POSTGRES_PASSWORD'],safe='')+'@127.0.0.1:'+port+'/'+urllib.parse.quote(database,safe='')+'?sslmode=disable'
 elif 'mysql' in name:
  port=ports['3306/tcp'][0]['HostPort']
  env['TEST_MYSQL_DSN']='root:'+values['MYSQL_ROOT_PASSWORD']+'@tcp(127.0.0.1:'+port+')/'+values.get('MYSQL_DATABASE','starport')+'?parseTime=true'
 elif 'valkey' in name:env['TEST_VALKEY_URL']='redis://127.0.0.1:'+ports['6379/tcp'][0]['HostPort']
 else:env['TEST_BLOB_S3_ENDPOINT']='http://127.0.0.1:'+ports['9000/tcp'][0]['HostPort']
env.update(GOWORK='off',GOTOOLCHAIN='go1.27.1')
import sys
with open(sys.argv[1],'wb') as output:
 result=subprocess.run(sys.argv[2:],env=env,stdout=output,stderr=subprocess.STDOUT)
sys.exit(result.returncode)
