import subprocess,json,pathlib,tempfile,shutil
proof=pathlib.Path('/Users/jack/src/github.com/agentstation/starmap-catalog-requirements/docs/plans/proof/starport-production-catalog/csp0.1/publication-review');proof.mkdir(exist_ok=True)
tmp=pathlib.Path(tempfile.mkdtemp(prefix='catalog-docs-review-'))
def cli(*args):
 p=subprocess.run(['chrome-devtools',*args,'--output-format=json'],capture_output=True,text=True,check=True)
 for line in p.stdout.splitlines():
  if line.startswith('{'):return json.loads(line)
 raise RuntimeError(p.stdout)
def evaluate(script):
 d=cli('evaluate_script',script);return json.loads(d['message'].split('```json\n')[1].split('\n```')[0])
measure=r"""() => {
 const canvas=document.createElement('canvas');canvas.width=canvas.height=1;const ctx=canvas.getContext('2d');
 const rgba=c=>{ctx.clearRect(0,0,1,1);ctx.fillStyle=c;ctx.fillRect(0,0,1,1);return [...ctx.getImageData(0,0,1,1).data].map((x,i)=>i===3?x/255:x)};
 const over=(a,b)=>[0,1,2].map(i=>a[i]*a[3]+b[i]*(1-a[3])).concat(1);
 const lum=a=>a.slice(0,3).map(x=>{x/=255;return x<=.04045?x/12.92:((x+.055)/1.055)**2.4}).reduce((s,x,i)=>s+x*[.2126,.7152,.0722][i],0);
 const samples=[];
 for(const e of document.querySelectorAll('main *')){
  if(![...e.childNodes].some(n=>n.nodeType===3&&n.textContent.trim()))continue;
  const c=getComputedStyle(e),r=e.getBoundingClientRect();if(!r.width||!r.height||c.visibility==='hidden'||c.display==='none')continue;
  let bg=[255,255,255,1],parents=[];for(let p=e;p;p=p.parentElement)parents.push(p);
  for(const p of parents.reverse())bg=over(rgba(getComputedStyle(p).backgroundColor),bg);
  const fg=over(rgba(c.color),bg),a=lum(fg),b=lum(bg);samples.push({text:e.textContent.trim().slice(0,60),contrast:(Math.max(a,b)+.05)/(Math.min(a,b)+.05)});
 }
 const main=document.querySelector('main'),body=document.querySelector('.docs-prose'),code=document.querySelector('pre');
 return {url:location.href,theme:document.documentElement.dataset.theme,width:innerWidth,documentWidth:document.documentElement.scrollWidth,mainWidth:main.getBoundingClientRect().width,bodyFont:getComputedStyle(body).fontSize,bodyLine:getComputedStyle(body).lineHeight,codeFont:code?getComputedStyle(code).fontSize:null,controls:[...document.querySelectorAll('button')].map(e=>({name:e.getAttribute('aria-label')||e.innerText,height:e.getBoundingClientRect().height})),code:[...document.querySelectorAll('pre')].map(e=>({tabIndex:e.tabIndex,width:e.clientWidth,scrollWidth:e.scrollWidth,overflow:getComputedStyle(e).overflowX})),contrastSamples:samples.length,minimumContrast:Math.min(...samples.map(s=>s.contrast)),lowContrast:samples.filter(s=>s.contrast<4.5),deploymentRequests:performance.getEntriesByType('resource').filter(e=>/\/api\/|\/console\/identity/.test(e.name)).map(e=>e.name)};
}"""
rows=[]
for width in [320,1280]:
 for theme in ['dark','light']:
  cli('emulate','--viewport',f'{width}x900','--colorScheme',theme)
  for audience in ['build','account','operate']:
   cli('navigate_page','--url',f'http://127.0.0.1:4177/docs?audience={audience}')
   evaluate(f"async () => {{if(document.documentElement.dataset.theme!=='{theme}')document.querySelector('button[aria-label^=\"Switch to\"]').click();await new Promise(r=>requestAnimationFrame(()=>requestAnimationFrame(r)));return true}}")
   row=evaluate(measure);row['audience']=audience
   name=f'{audience}-{theme}-{width}.png';cli('take_screenshot','--filePath',str(tmp/name));shutil.copyfile(tmp/name,proof/name)
   rows.append(row);(proof/'measurements.json').write_text(json.dumps(rows,indent=2)+'\n')
   print(json.dumps({k:row[k] for k in ['audience','theme','width','documentWidth','minimumContrast','deploymentRequests']}),flush=True)
