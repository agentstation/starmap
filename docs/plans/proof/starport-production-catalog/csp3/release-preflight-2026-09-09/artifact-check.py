from pathlib import Path
import json,hashlib,subprocess,tarfile,zipfile,tempfile
root=Path.cwd(); dist=root/'dist'; result={'head':'5c40192a620b43341f06a3ad49f4e2a743c741ba','snapshot_exit':0,'published':False,'archives':[],'checksums':[],'native_commands':[],'probe_corrections':['The first native probe omitted HOME and could not resolve the user directory. The corrected probe supplies an isolated home.','The first artifact probe expected Syft JSON. GoReleaser emits SPDX JSON; the corrected check validates SPDX packages.']}
for line in (dist/'checksums.txt').read_text().splitlines():
 digest,name=line.split(None,1);path=dist/name.strip();actual=hashlib.sha256(path.read_bytes()).hexdigest();assert actual==digest,(name,actual,digest);result['checksums'].append({'name':name,'sha256':actual})
for path in sorted(list(dist.glob('*.tar.gz'))+list(dist.glob('*.zip'))):
 if path.suffix=='.zip':
  with zipfile.ZipFile(path) as archive: names=archive.namelist()
 else:
  with tarfile.open(path) as archive:names=archive.getnames()
 required={'README.md','CHANGELOG.md','completions/starmap.bash','completions/starmap.zsh','completions/starmap.fish','manpages/starmap.1.gz'}
 assert required<=set(names),(path.name,required-set(names))
 assert any(n.startswith('LICENSE') for n in names)
 assert ('starmap.exe' if path.suffix=='.zip' else 'starmap') in names
 sbom=Path(str(path)+'.sbom.json');data=json.loads(sbom.read_text());assert data.get('packages') and data['spdxVersion'].startswith('SPDX-')
 result['archives'].append({'name':path.name,'files':names,'sbom_packages':len(data['packages']),'bytes':path.stat().st_size})
with tempfile.TemporaryDirectory(prefix='starmap-release-smoke-') as temporary:
 env={'PATH':'/usr/bin:/bin','HOME':temporary,'STARMAP_HOME':str(Path(temporary)/'product'),'STARMAP_CATALOG_SOURCE':'embedded'}
 binary=dist/'starmap_darwin_arm64_v8.0/starmap'
 for args in [['version'],['config','paths'],['models','list','--help']]:
  run=subprocess.run([str(binary),*args],cwd=temporary,env=env,text=True,capture_output=True,timeout=30)
  result['native_commands'].append({'args':args,'exit':run.returncode,'stdout':run.stdout,'stderr':run.stderr})
  assert run.returncode==0,(args,run.stderr)
  if args==["version"]: assert run.stdout.strip()=="starmap 0.16.6-next"
  if args==["config","paths"]:
   paths=json.loads(run.stdout);assert paths["product"]=="starmap" and paths["build_version"]=="0.16.6-next"
   for key in ["config","data","state","cache"]: assert paths["roots"][key]["path"]==str(Path(temporary)/"product"/key)
   store=next(f for f in paths["files"] if f["id"]=="catalog-store");assert store["policy"]["access"]=="owner-only"
result['status']='PASS';Path('/tmp/starmap-release-preflight-artifact-check.json').write_text(json.dumps(result,indent=2)+'\n')
print(json.dumps({'status':result['status'],'archives':len(result['archives']),'checksum_matches':len(result['checksums']),'native_commands':len(result['native_commands'])}))
