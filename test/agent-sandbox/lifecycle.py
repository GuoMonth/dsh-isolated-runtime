#!/usr/bin/env python3
"""Real healthy-node stop/start, with exact Pod terminal watch evidence."""
import json, os, subprocess, threading, time
from pathlib import Path
from lab import K, NS, OUT, get, call, patch, pods, ready, save, wait

LAB = OUT.parent
B = json.loads((OUT/'binding.json').read_text())
NAME, UID = B['name'], B['uid']

def admin(path):
    p=call('exec','-n',NS,'probe','--','node','-e',f"fetch('http://127.0.0.1:30501/{path}').then(r=>r.text()).then(console.log)")
    return json.loads(p.stdout)

def snapshot():
    return {'sandbox':get('sandbox',NAME),'pod':pods(UID)[0], 'service':get('svc',NAME), 'endpoints':get('endpointslices'), 'pvcs':{r:get('pvc',n) for r,n in B['pvcNames'].items()}, 'binding':B,'expectedPodSpec':json.loads((OUT/'template.json').read_text())}

def check_volumes():
    for role,name in B['pvcNames'].items():
        p=get('pvc',name);assert p['metadata']['uid']==B['pvcUids'][role],role+' UID changed'
        assert not p['metadata'].get('ownerReferences'),role+' unexpectedly GC owned'

def require_terminal(events, pod_uid):
    for event in events:
        containers=event.get('containers') or []
        if event.get('uid')==pod_uid and event.get('phase') in ['Succeeded','Failed'] and containers and all('terminated' in c.get('state',{}) for c in containers):
            return
    raise RuntimeError('StopUnverified: no kubelet terminal event; do not resume or delete volumes')

def stop(round_no):
    pod=pods(UID)[0];pod_uid=pod['metadata']['uid'];events=[];finished=threading.Event();errors=[]
    watcher=subprocess.Popen(K+['get','--raw',f"/api/v1/namespaces/{NS}/pods?watch=true&fieldSelector=metadata.name%3D{pod['metadata']['name']}&resourceVersion={pod['metadata']['resourceVersion']}"],stdout=subprocess.PIPE,stderr=subprocess.PIPE,text=True)
    def consume():
        buf='';decoder=json.JSONDecoder()
        try:
            for line in watcher.stdout:
                buf+=line
                try:entry,pos=decoder.raw_decode(buf.lstrip())
                except json.JSONDecodeError:continue
                buf=buf.lstrip()[pos:];p=entry.get('object',{});statuses=p.get('status',{}).get('containerStatuses',[])
                if p.get('metadata',{}).get('uid')!=pod_uid:continue
                event={'type':entry.get('type'),'uid':pod_uid,'phase':p.get('status',{}).get('phase'),'terminating':bool(p.get('metadata',{}).get('deletionTimestamp')),'containers':[{'name':c['name'],'state':c.get('state')} for c in statuses]}
                events.append(event)
                if statuses and all('terminated' in c.get('state',{}) for c in statuses) and p.get('status',{}).get('phase') in ['Succeeded','Failed']:finished.set()
        except Exception as e:errors.append(str(e))
    t=threading.Thread(target=consume,daemon=True);t.start();time.sleep(.5)
    holder=None
    if round_no==1:
        holder=subprocess.Popen([str(LAB/'ws-hold'),'--state',str(LAB/'private/probe-state.json')],stdout=subprocess.PIPE,stderr=subprocess.PIPE,text=True)
        assert holder.stdout.readline().strip()=='stream-open'
    revoked=admin('revoke')
    if holder:
        output,err=holder.communicate(timeout=10);assert holder.returncode==0 and 'stream-closed' in output,(output,err)
    started=time.monotonic();intent=patch(NAME,'Suspended');generation=intent['metadata']['generation']
    def stopped():
        sb=get('sandbox',NAME)
        condition=any(c['type']=='Suspended' and c['status']=='True' and c.get('observedGeneration')==generation for c in sb.get('status',{}).get('conditions',[]))
        return sb if condition and not pods(UID) else None
    sb=wait(stopped,60);finished.wait(2)
    watcher.terminate();watcher.wait(timeout=5);t.join(2)
    stderr=watcher.stderr.read()
    if stderr:errors.append(stderr)
    save(f'stop-watch-{round_no}',{'podUid':pod_uid,'events':events,'watchErrors':errors})
    check_volumes()
    endpoints=get('endpointslices')['items']
    assert not any(e.get('conditions',{}).get('ready') for s in endpoints for e in (s.get('endpoints') or []))
    result={'round':round_no,'seconds':time.monotonic()-started,'podUid':pod_uid,'generation':generation,'positiveTerminalEvidence':finished.is_set(),'websocketRevoked':holder is not None,'stats':revoked}
    save(f'stop-result-{round_no}',result)
    require_terminal(events,pod_uid)
    print(json.dumps({'phase':'stopped',**result}),flush=True)
    return pod_uid

def start(old_uid,round_no):
    check_volumes();started=time.monotonic();patch(NAME,'Running');wait(lambda:ready(NAME));p=pods(UID)[0];assert p['metadata']['uid']!=old_uid
    check_volumes();admin('allow')
    result={'round':round_no,'seconds':time.monotonic()-started,'podUid':p['metadata']['uid'],'sandboxUid':UID}
    save(f'start-result-{round_no}',result);print(json.dumps({'phase':'resumed',**result}),flush=True)
    p=subprocess.run([str(LAB/'dshprobe'),'--connect','127.0.0.1:30500','--authority','sandbox-spike.cells.test','--state-file',str(LAB/'private/probe-state.json'),'--resume'],capture_output=True,text=True)
    if p.returncode:raise RuntimeError(p.stderr)
    save(f'dsh-resume-{round_no}',{'passed':True,'message':p.stdout.strip()})

if __name__=='__main__':
    save('verified-snapshot',snapshot())
    pod=pods(UID)[0]['metadata']['name']
    call('exec','-n',NS,pod,'--','node','-e',"const fs=require('fs');fs.writeFileSync('/var/lib/dsh/data/workspace/sandbox-spike.txt','data-marker');fs.writeFileSync('/var/lib/dsh-private/spike.txt','private-marker')")
    old=stop(1);start(old,1)
    pod=pods(UID)[0]['metadata']['name']
    out=call('exec','-n',NS,pod,'--','node','-e',"const fs=require('fs');if(fs.readFileSync('/var/lib/dsh/data/workspace/sandbox-spike.txt','utf8')!=='data-marker'||fs.readFileSync('/var/lib/dsh-private/spike.txt','utf8')!=='private-marker')process.exit(1);console.log('both PVC markers retained')").stdout
    save('file-persistence',{'passed':True,'message':out.strip(),'method':'kubectl exec (storage test, not DSH model tool)'})
    print(out.strip(),flush=True)
