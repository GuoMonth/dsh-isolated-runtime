import os
import json,subprocess,pathlib
k=['kubectl','--kubeconfig',os.environ['KUBECONFIG']]
def run(args):return subprocess.check_output(k+args,text=True)
v=json.loads(run(['exec','-n','dsh-mvp-b-platform','probe','--','cat','/tmp/binding.json']));assert v['state']=='Stopped';assert v['ref']['allocationKey']=='b-volume-fault-1';ns=v['ref']['namespace']
p=json.loads(run(['get','pvc','data','-n',ns,'-o','json']));assert p['metadata']['uid']==v['ref']['data']['uid'];assert not json.loads(run(['get','pods','-n',ns,'-o','json']))['items']
run(['delete','pvc','data','-n',ns,'--wait=true','--timeout=30s'])
missing=run(['exec','-n','dsh-mvp-b-platform','probe','--','node','/tmp/action.mjs','missing']);print(missing,end='')
replacement={'apiVersion':'v1','kind':'PersistentVolumeClaim','metadata':{'name':'data','namespace':ns,'annotations':p['metadata']['annotations']},'spec':{'accessModes':['ReadWriteOnce'],'storageClassName':'standard','resources':{'requests':{'storage':'1Gi'}}}}
subprocess.run(k+['create','-f','-'],input=json.dumps(replacement),text=True,check=True,capture_output=True)
new=json.loads(run(['get','pvc','data','-n',ns,'-o','json']));assert new['metadata']['uid']!=p['metadata']['uid']
changed=run(['exec','-n','dsh-mvp-b-platform','probe','--','node','/tmp/action.mjs','replaced']);print(changed,end='')
sb=json.loads(run(['get','sandbox','environment','-n',ns,'-o','json']));assert sb['spec']['operatingMode']=='Suspended';assert not json.loads(run(['get','pods','-n',ns,'-o','json']))['items']
(pathlib.Path(os.environ['EVIDENCE']) / 'volume-identity.json').write_text(json.dumps({'namespace':ns,'originalPVC':p['metadata']['uid'],'replacementPVC':new['metadata']['uid'],'sandboxMode':sb['spec']['operatingMode'],'missingAndReplacementRejected':True},indent=2)+'\n')
