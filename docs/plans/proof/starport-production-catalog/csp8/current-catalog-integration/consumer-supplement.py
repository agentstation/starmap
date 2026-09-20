import hashlib,json,os,sys
from pathlib import Path
sys.path.insert(0,'/Users/jack/src/github.com/agentstation/starmap-advisory-exchange/scripts')
import catalog_product_verify as v
plan=Path('/Users/jack/src/github.com/agentstation/starmap-catalog-requirements')
roots={'starmap':Path('/Users/jack/src/github.com/agentstation/starmap-advisory-exchange'),'starport':Path('/Users/jack/src/github.com/agentstation/starport-catalog-integration')}
roster=v.read_json(plan/'docs/plans/proof/starport-production-catalog/acceptance-map.json')
old=v.read_json(roots['starmap']/'scripts/catalog-product-checks.json')
new=v.read_json(plan/'scripts/catalog-product-checks.json')
def leaves(entry):
 if not entry:return []
 if entry['kind']=='all':return [leaf for child in entry['checks'] for leaf in leaves(child)]
 return [entry]
def key(entry):return json.dumps(entry,sort_keys=True)
selected=roster['task_checks']['CSP8']
prior={key(leaf) for identity in selected for leaf in leaves(old['checks'].get(identity))}
additional={key(leaf):leaf for identity in selected for leaf in leaves(new['checks'].get(identity)) if key(leaf) not in prior}
report={'registry_sha256':hashlib.sha256((plan/'scripts/catalog-product-checks.json').read_bytes()).hexdigest(),'scope':'Additional CSP8 leaf checks only; combine with unchanged producer evidence before task assessment.','total':len(additional),'results':{}}
output=Path('/tmp/csp8-current-consumer-supplement.json');cache={}
if output.exists():
 previous=json.loads(output.read_text())
 if previous['registry_sha256']!=report['registry_sha256']:raise ValueError('Registry changed before resume')
 report['results']={k:v for k,v in previous['results'].items() if v['status']=='PASS'}
 report['resumed_passes']=len(report['results'])
for index,(identity,entry) in enumerate(additional.items()):
 if identity in report['results']:continue
 result=v.run_check('CSP8.supplement',entry,roots,cache)
 report['results'][identity]=result
 temporary=output.with_suffix('.partial')
 temporary.write_text(json.dumps(report,indent=2)+'\n')
 temporary.replace(output)
 print(str(index+1)+'/'+str(len(additional))+' '+result['status']+' '+entry.get('test',entry['kind']),flush=True)
sys.exit(1 if any(x['status']=='FAIL' for x in report['results'].values()) else 0)
