#!/usr/bin/env python3
"""Delete only the disposable test environment; retain its data PVC as evidence."""
import copy, json, subprocess, time
from lab import NS, OUT, GROUP, call, create, get, pods, save, wait
from lifecycle import B, NAME, UID, stop, check_volumes, admin
from faults import remove

# Real same-name PVC replacement, using a separate never-mounted disposable claim.
claim={'apiVersion':'v1','kind':'PersistentVolumeClaim','metadata':{'name':'uid-replacement','namespace':NS,'labels':{'agents.dsh.io/environment-uid':UID}},'spec':{'storageClassName':'standard','accessModes':['ReadWriteOnce'],'resources':{'requests':{'storage':'1Gi'}}}}
first=create(claim);remove('persistentvolumeclaims',claim['metadata']['name'],first['metadata']['uid'])
wait(lambda:call('get','pvc',claim['metadata']['name'],'-n',NS,check=False).returncode!=0)
second=create(claim);assert second['metadata']['uid']!=first['metadata']['uid']
snapshot=json.loads((OUT/'verified-snapshot.json').read_text());snapshot['binding']['pvcNames']['data']=second['metadata']['name'];snapshot['binding']['pvcUids']['data']=first['metadata']['uid'];snapshot['pvcs']['data']=second
save('replaced-pvc-snapshot',snapshot)
# Exact UID deletion must reject the old identity and preserve the new PVC.
p=call('delete','--raw',f'/api/v1/namespaces/{NS}/persistentvolumeclaims/{second["metadata"]["name"]}','-f','-',value={'apiVersion':'v1','kind':'DeleteOptions','preconditions':{'uid':first['metadata']['uid']}},check=False)
assert p.returncode and get('pvc',second['metadata']['name'])['metadata']['uid']==second['metadata']['uid']
save('pvc-replacement',{'oldUid':first['metadata']['uid'],'newUid':second['metadata']['uid'],'staleDeleteRejected':True})
remove('persistentvolumeclaims',second['metadata']['name'],second['metadata']['uid'])

# Stop is a required operation before deletion; no forced deletion/finalizer removal.
stop(3);check_volumes()
remove('sandboxes',NAME,UID,GROUP)
wait(lambda:call('get','sandbox',NAME,'-n',NS,check=False).returncode!=0)
wait(lambda:call('get','service',NAME,'-n',NS,check=False).returncode!=0)
assert not pods(UID)
# Simulate interruption between CR deletion and explicit private cleanup.
check_volumes();save('delete-interrupted',{'sandboxAbsent':True,'bothExternalPvcUidsIntact':True,'accessRevoked':admin('stats')['revoked']})
# Retry uses saved immutable binding, not a guessed name.
private=get('pvc',B['pvcNames']['private']);assert private['metadata']['uid']==B['pvcUids']['private']
assert private['metadata']['labels']['agents.dsh.io/environment-uid']==UID
remove('persistentvolumeclaims',B['pvcNames']['private'],B['pvcUids']['private'])
wait(lambda:call('get','pvc',B['pvcNames']['private'],'-n',NS,check=False).returncode!=0)
data=get('pvc',B['pvcNames']['data']);assert data['metadata']['uid']==B['pvcUids']['data'] and not data['metadata'].get('ownerReferences')
# A read-only diagnostic Pod verifies actual retained bytes after environment deletion.
image=json.loads((OUT/'versions.json').read_text())['dshImage']
reader=create({'apiVersion':'v1','kind':'Pod','metadata':{'name':'retained-reader','namespace':NS},'spec':{'restartPolicy':'Never','automountServiceAccountToken':False,'securityContext':{'runAsUser':1000,'runAsGroup':1000,'runAsNonRoot':True,'seccompProfile':{'type':'RuntimeDefault'}},'containers':[{'name':'read','image':image,'command':['node','-e',"const fs=require('fs');if(fs.readFileSync('/retained/workspace/sandbox-spike.txt','utf8')!=='data-marker')process.exit(1);console.log('retained-data-readable')"],'volumeMounts':[{'name':'retained','mountPath':'/retained','readOnly':True}],'securityContext':{'allowPrivilegeEscalation':False,'readOnlyRootFilesystem':True,'capabilities':{'drop':['ALL']}}}],'volumes':[{'name':'retained','persistentVolumeClaim':{'claimName':B['pvcNames']['data'],'readOnly':True}}]}})
wait(lambda:get('pod','retained-reader').get('status',{}).get('phase')=='Succeeded',60)
message=call('logs','-n',NS,'retained-reader').stdout.strip()
result={'sandboxDeleted':True,'serviceDeleted':True,'privatePvcDeleted':True,'dataPvcUidRetained':data['metadata']['uid'],'retainedDataRead':message,'interruptedCleanupRetried':True}
save('deletion-result',result);print(json.dumps(result,indent=2),flush=True)
remove('pods','retained-reader',reader['metadata']['uid'])
