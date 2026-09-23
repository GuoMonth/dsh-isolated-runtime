#!/usr/bin/env python3
"""Finite cluster faults using only fresh names and synthetic task-owned data."""
import copy, json, subprocess, time
from concurrent.futures import ThreadPoolExecutor
from lab import GROUP, NS, OUT, call, create, get, patch, pods, ready, save, wait
from lifecycle import B, NAME, UID, admin, snapshot, start, stop, check_volumes


def remove(kind_plural,name,uid,group='v1'):
    base='/api' if group=='v1' else '/apis'
    path=f'{base}/{group}/namespaces/{NS}/{kind_plural}/{name}'
    return call('delete','--raw',path,'-f','-',value={'apiVersion':'v1','kind':'DeleteOptions','preconditions':{'uid':uid},'propagationPolicy':'Foreground'})

def fault_spec(name):
    spec=copy.deepcopy(get('sandbox',NAME)['spec'])
    for v in spec['podTemplate']['spec']['volumes']:
        if 'persistentVolumeClaim' in v:v['persistentVolumeClaim']['claimName']=name+'-'+v['name']
    return spec

def run():
    results={}
    # Unknown create response: discard response, rediscover exact original key;
    # a duplicate create is rejected, with no fresh key or replacement object.
    name='unknown-create';spec=fault_spec(name);spec['operatingMode']='Suspended'
    obj={'apiVersion':GROUP,'kind':'Sandbox','metadata':{'name':name,'namespace':NS},'spec':spec}
    create(obj);first=get('sandbox',name)
    repeated=call('create','-f','-',value=obj,check=False)
    assert repeated.returncode and 'AlreadyExists' in repeated.stderr
    assert get('sandbox',name)['metadata']['uid']==first['metadata']['uid'] and not pods(first['metadata']['uid'])
    results['unknownCreate']={'sameUid':True,'duplicateRejected':True,'method':'discard successful create response then rediscover original key; not a network fault injection'}
    remove('sandboxes',name,first['metadata']['uid'],GROUP)

    name='missing-volume';sb=create({'apiVersion':GROUP,'kind':'Sandbox','metadata':{'name':name,'namespace':NS},'spec':fault_spec(name)})
    uid=sb['metadata']['uid'];wait(lambda:pods(uid));time.sleep(3)
    p=pods(uid)[0];assert p['status']['phase']=='Pending'
    assert not any(x['metadata']['name'].startswith(name+'-') for x in get('pvc')['items'])
    results['missingVolume']={'podPhase':p['status']['phase'],'noPvcAutoCreated':True,'sandboxReady':bool(ready(name))}
    save('missing-volume-sandbox',get('sandbox',name));remove('sandboxes',name,uid,GROUP);wait(lambda:not pods(uid))

    # A preexisting unowned Pod without adoption/tracking labels is left intact.
    name='foreign-pod';foreign_spec=copy.deepcopy(json.loads((OUT/'template.json').read_text()))
    foreign_spec['containers'][0].update({'command':['node','-e','setInterval(()=>{},1000)'],'workingDir':'/','volumeMounts':[],'env':[]})
    for key in ['startupProbe','readinessProbe','livenessProbe']:foreign_spec['containers'][0].pop(key,None)
    foreign_spec['volumes']=[]
    foreign=create({'apiVersion':'v1','kind':'Pod','metadata':{'name':name,'namespace':NS},'spec':foreign_spec})
    sb=create({'apiVersion':GROUP,'kind':'Sandbox','metadata':{'name':name,'namespace':NS},'spec':fault_spec(name)})
    uid=sb['metadata']['uid'];time.sleep(3)
    current=get('pod',name);assert current['metadata']['uid']==foreign['metadata']['uid'] and not current['metadata'].get('ownerReferences')
    assert not ready(name) and not pods(uid)
    results['foreignPod']={'leftUnowned':True,'unchangedUid':True,'sandboxReady':False}
    remove('sandboxes',name,uid,GROUP);remove('pods',name,foreign['metadata']['uid'])

    # Two updates using the same revision must not both mutate the binding's intent.
    sb=get('sandbox',NAME);rv=sb['metadata']['resourceVersion']
    def update(value):
        ops=[{'op':'test','path':'/metadata/uid','value':UID},{'op':'test','path':'/metadata/resourceVersion','value':rv},{'op':'add','path':'/metadata/annotations/spike-cas','value':value},{'op':'replace','path':'/spec/operatingMode','value':'Running'}]
        return call('patch','sandbox',NAME,'-n',NS,'--type=json','-p',json.dumps(ops),check=False)
    # Ensure annotation parent exists before reading the shared revision.
    call('annotate','sandbox',NAME,'-n',NS,'spike-cas=initial','--overwrite');rv=get('sandbox',NAME)['metadata']['resourceVersion']
    with ThreadPoolExecutor(2) as pool:attempts=list(pool.map(update,['a','b']))
    assert sum(x.returncode==0 for x in attempts)==1
    results['concurrentCas']={'accepted':1,'staleRejected':1,'scope':'K8s resourceVersion compare-and-swap; full platform concurrency belongs to W2'}

    # No caller/adapter writes exist while upstream recreates a manually deleted Pod.
    admin('revoke');old=pods(UID)[0];old_uid=old['metadata']['uid'];started=time.monotonic()
    remove('pods',old['metadata']['name'],old_uid)
    wait(lambda:pods(UID) and pods(UID)[0]['metadata']['uid']!=old_uid and ready(NAME))
    check_volumes();admin('allow')
    results['controllerRebuildWithoutCaller']={'newPodUid':pods(UID)[0]['metadata']['uid'],'sameSandboxUid':get('sandbox',NAME)['metadata']['uid']==UID,'originalPvcUids':True,'seconds':time.monotonic()-started,'scope':'normal Pod deletion on a healthy node; not node partition'}
    p=subprocess.run([str(OUT.parent/'dshprobe'),'--connect','127.0.0.1:30500','--authority','sandbox-spike.cells.test','--state-file',str(OUT.parent/'private/probe-state.json'),'--resume'],text=True,capture_output=True)
    assert p.returncode==0,p.stderr
    results['nativeDshAfterRebuild']=True
    save('fault-results',results)
    print(json.dumps(results,indent=2),flush=True)

if __name__=='__main__':run()
