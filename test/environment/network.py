import os
import subprocess,json,pathlib
k=['kubectl','--kubeconfig',os.environ['KUBECONFIG']]
def run(a):return subprocess.check_output(k+a,text=True)
v=json.loads(run(['exec','-n','dsh-mvp-b-platform','probe','--','cat','/tmp/binding.json']));ns=v['ref']['namespace'];p=json.loads(run(['get','pods','-n',ns,'-o','json']))['items'][0];ip=p['status']['podIP'];name=p['metadata']['name']
image=p['spec']['containers'][0]['image'];probe={'apiVersion':'v1','kind':'Pod','metadata':{'name':'unlabelled-probe','namespace':'dsh-mvp-b-platform'},'spec':{'automountServiceAccountToken':False,'restartPolicy':'Never','containers':[{'name':'probe','image':image,'imagePullPolicy':'IfNotPresent','command':['node','-e','setInterval(()=>{},1000)']}]}}
subprocess.run(k+['create','-f','-'],input=json.dumps(probe),text=True,check=True,capture_output=True)
run(['wait','-n','dsh-mvp-b-platform','pod/unlabelled-probe','--for=condition=Ready','--timeout=60s'])
script="require('http').get({host:"+json.dumps(ip)+",port:8080,path:'/',signal:AbortSignal.timeout(3000)},r=>{console.error('unexpected reachable '+r.statusCode);process.exit(1)}).on('error',e=>{if(e.name!=='AbortError')throw e;console.log('ingress-denied')})"
ingress=run(['exec','-n','dsh-mvp-b-platform','unlabelled-probe','--','node','-e',script]).strip()
egress=run(['exec','-n',ns,name,'--','node','-e',"require('https').get('https://kubernetes.default.svc',{rejectUnauthorized:false,signal:AbortSignal.timeout(3000)},r=>{console.error('unexpected API reachable '+r.statusCode);process.exit(1)}).on('error',e=>{if(e.name!=='AbortError')throw e;console.log('api-egress-denied')})"]).strip()
run(['delete','pod','unlabelled-probe','-n','dsh-mvp-b-platform','--wait=true'])
e={'namespace':ns,'podUID':p['metadata']['uid'],'unlabelledIngress':ingress,'workloadAPIEgress':egress};(pathlib.Path(os.environ['EVIDENCE']) / 'network.json').write_text(json.dumps(e,indent=2)+'\n');print(json.dumps(e))
