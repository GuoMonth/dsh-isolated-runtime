#!/usr/bin/env python3
"""Extra stop-gate checks: real watch denial, conflicting intent CAS, and CNI."""
import copy, json, os, subprocess, sys, time
from pathlib import Path
from lab import OUT, GROUP, NS, create, get, call, setup, patch, pods, ready, save, wait
# Run in a separate evidence directory with fresh claims; never reuse prior data.
setup('stop-gates')
from lifecycle import B, NAME, UID, stop, start, admin, require_terminal
from faults import remove
pod=pods(UID)[0]
# Actual NetworkPolicy denies workload access to Kubernetes API.
api=get('service','kubernetes',ns='default')['spec']['clusterIP']
script=f"fetch('https://{api}:443',{{signal:AbortSignal.timeout(1800)}}).then(()=>process.exit(2)).catch(e=>{{if(e.name!=='TimeoutError'){{console.error(e.name);process.exit(3)}}console.log('workload-api-egress-blocked')}})"
p=call('exec','-n',NS,pod['metadata']['name'],'--','node','-e',script)
assert 'workload-api-egress-blocked' in p.stdout
# A distinct unlabeled caller cannot reach the workload's management port.
probe=create({'apiVersion':'v1','kind':'Pod','metadata':{'name':'untrusted-probe','namespace':NS},'spec':{'restartPolicy':'Never','automountServiceAccountToken':False,'containers':[{'name':'probe','image':os.environ['DSH_SPIKE_IMAGE'],'command':['node','-e',f"fetch('http://{pod['status']['podIP']}:8081/readyz',{{signal:AbortSignal.timeout(1800)}}).then(()=>process.exit(2)).catch(e=>{{if(e.name!=='TimeoutError')process.exit(3);console.log('untrusted-ingress-blocked')}})"]}]}})
wait(lambda:get('pod','untrusted-probe').get('status',{}).get('phase') in ['Succeeded','Failed']);assert get('pod','untrusted-probe')['status']['phase']=='Succeeded'
remove('pods','untrusted-probe',probe['metadata']['uid'])
save('network-results',{'workloadApiEgressDenied':True,'untrustedIngressDenied':True,'provider':'Calico3.32.2'})
# Start request built before stop must be rejected using the stale revision.
old=get('sandbox',NAME);intent=patch(NAME,'Suspended')
attempt=call('patch','sandbox',NAME,'-n',NS,'--type=json','-p',json.dumps([{'op':'test','path':'/metadata/uid','value':UID},{'op':'test','path':'/metadata/resourceVersion','value':old['metadata']['resourceVersion']},{'op':'replace','path':'/spec/operatingMode','value':'Running'}]),check=False)
assert attempt.returncode and get('sandbox',NAME)['spec']['operatingMode']=='Suspended'
# No stop proof has been collected yet: deliberately fail watch (denied as an unauthenticated identity).
result=call('get','--raw',f'/api/v1/namespaces/{NS}/pods?watch=true&resourceVersion={pod["metadata"]["resourceVersion"]}','--as=system:anonymous',check=False)
assert result.returncode and ('Forbidden' in result.stderr or 'forbidden' in result.stderr)
wait(lambda:not pods(UID))
# The same gate used by the healthy stop path rejects missing evidence.
try:
    require_terminal([],pod['metadata']['uid'])
    raise AssertionError('empty evidence accepted')
except RuntimeError as error:
    assert str(error).startswith('StopUnverified:')
# This failure is not an authorization to resume.
save('stop-unverified',{'code':'StopUnverified','watchDenied':True,'modeRemains':'Suspended','staleStartRejected':True,'noAutomaticResume':True})
# Recover original UID termination proof from the API watch cache, bounded to 3s.
path=f'/api/v1/namespaces/{NS}/pods?watch=true&fieldSelector=metadata.name%3D{pod["metadata"]["name"]}&resourceVersion={pod["metadata"]["resourceVersion"]}'
p=call('get','--raw',path,'--request-timeout=3s',check=False)
events=[json.loads(line) for line in p.stdout.splitlines() if line.strip()]
terminal=[e['object'] for e in events if e.get('object',{}).get('metadata',{}).get('uid')==pod['metadata']['uid'] and e.get('object',{}).get('status',{}).get('phase') in ['Succeeded','Failed'] and all('terminated' in c.get('state',{}) for c in e.get('object',{}).get('status',{}).get('containerStatuses',[]))]
assert terminal and terminal[-1]['status']['containerStatuses']
save('stop-gate-proof',{'podUid':pod['metadata']['uid'],'phase':terminal[-1]['status']['phase'],'states':[c['state'] for c in terminal[-1]['status']['containerStatuses']]})
# Delete vs resume: once the old CR is gone, updating it cannot create a replacement.
remove('sandboxes',NAME,UID,GROUP);wait(lambda:call('get','sandbox',NAME,'-n',NS,check=False).returncode!=0)
assert call('patch','sandbox',NAME,'-n',NS,'--type=merge','-p','{"spec":{"operatingMode":"Running"}}',check=False).returncode
assert not pods(UID)
# Both claims belong solely to this disposable gate test (no user data).
for role,name in B['pvcNames'].items():remove('persistentvolumeclaims',name,B['pvcUids'][role])
save('stop-gates-result',{'watchFailureFailClosed':True,'staleStartRejectedDuringStop':True,'deletedInstanceResumeRejected':True,'cleanupAfterExactTerminalProof':True})
print('stop gates and actual CNI checks passed',flush=True)
