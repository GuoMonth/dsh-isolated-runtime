import os
import subprocess,json,time,pathlib
k=['kubectl','--kubeconfig',os.environ['KUBECONFIG']]
def run(args):return subprocess.check_output(k+args,text=True)
v=json.loads(run(['exec','-n','dsh-mvp-b-platform','probe','--','cat','/tmp/binding.json']))
ns=v['ref']['namespace'];pod=json.loads(run(['get','pods','-n',ns,'-o','json']))['items'][0];old=pod['metadata']['uid'];name=pod['metadata']['name']
run(['exec','-n',ns,name,'--','node','-e',"const fs=require('fs');for(const d of ['workspace','home','dsh'])fs.writeFileSync('/var/lib/dsh/data/'+d+'/b-marker','durable-'+d);"])
run(['delete','pod',name,'-n',ns,'--wait=true','--timeout=60s'])
result=run(['exec','-n','dsh-mvp-b-platform','probe','--','node','/tmp/action.mjs','ready'])
new=json.loads(run(['get','pods','-n',ns,'-o','json']))['items'][0];assert new['metadata']['uid']!=old
run(['exec','-n',ns,new['metadata']['name'],'--','node','-e',"const fs=require('fs'),a=require('assert');for(const d of ['workspace','home','dsh'])a.equal(fs.readFileSync('/var/lib/dsh/data/'+d+'/b-marker','utf8'),'durable-'+d);"])
pvc=json.loads(run(['get','pvc','data','-n',ns,'-o','json']));assert pvc['metadata']['uid']==v['ref']['data']['uid']
e={'name':'real-pod-rebuild','namespace':ns,'oldPodUID':old,'newPodUID':new['metadata']['uid'],'dataUID':pvc['metadata']['uid'],'markers':['workspace','home','dsh'],'image':new['spec']['containers'][0]['image'],'imageID':new['status']['containerStatuses'][0]['imageID']}
(pathlib.Path(os.environ['EVIDENCE']) / 'rebuild-final.json').write_text(json.dumps(e,indent=2)+'\n');print(json.dumps(e))
