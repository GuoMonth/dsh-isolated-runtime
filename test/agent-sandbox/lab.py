#!/usr/bin/env python3
"""Bounded, test-only upstream Sandbox lab. Never touches existing Cell resources."""
import json, os, subprocess, time, uuid
from pathlib import Path

KUBECONFIG = os.environ['KUBECONFIG']
OUT = Path(os.environ['SANDBOX_EVIDENCE']); OUT.mkdir(parents=True, exist_ok=True)
NS = 'sandbox-spike'
K = ['kubectl', '--kubeconfig', KUBECONFIG, '--request-timeout=20s']
GROUP = 'agents.x-k8s.io/v1beta1'
IMAGE = os.environ['DSH_SPIKE_IMAGE']
ROOT = Path(__file__).resolve().parents[2]

def call(*args, value=None, check=True):
    p = subprocess.run(K + list(args), input=json.dumps(value) if value is not None else None, text=True, capture_output=True, timeout=45)
    if check and p.returncode: raise RuntimeError(p.stderr.strip())
    return p

def get(kind, name=None, ns=NS):
    a=['get',kind,'-n',ns,'-o','json']
    if name:a.append(name)
    return json.loads(call(*a).stdout)

def create(obj): return json.loads(call('create','-f','-','-o','json',value=obj).stdout)
def apply(obj): return json.loads(call('apply','-f','-','-o','json',value=obj).stdout)
def patch(name, mode, rv=None):
    sb=get('sandbox',name)
    ops=[{'op':'test','path':'/metadata/uid','value':sb['metadata']['uid']}, {'op':'test','path':'/metadata/resourceVersion','value':rv or sb['metadata']['resourceVersion']}, {'op':'replace','path':'/spec/operatingMode','value':mode}]
    return json.loads(call('patch','sandbox',name,'-n',NS,'--type=json','-p',json.dumps(ops),'-o','json').stdout)

def wait(fn, timeout=120):
    end=time.monotonic()+timeout; last=None
    while time.monotonic()<end:
        try:
            last=fn()
            if last:return last
        except RuntimeError as e:last=str(e)
        time.sleep(.4)
    raise TimeoutError(str(last))

def save(name, obj): (OUT/(name+'.json')).write_text(json.dumps(obj,indent=2)+'\n')
def pods(uid): return [p for p in get('pods')['items'] if any(o.get('uid')==uid and o.get('controller') for o in p['metadata'].get('ownerReferences',[]))]
def ready(name):
    sb=get('sandbox',name)
    return sb if any(c['type']=='Ready' and c['status']=='True' and c.get('observedGeneration')==sb['metadata']['generation'] for c in sb.get('status',{}).get('conditions',[])) else None

def setup(name='dsh-local'):
    if call('get','ns',NS,check=False).returncode:
        create({'apiVersion':'v1','kind':'Namespace','metadata':{'name':NS,'labels':{'app.kubernetes.io/part-of':'agent-sandbox-local'}}})
    t=json.loads((ROOT/'packages/cell-connector/src/templates/cell-mvp-v1.json').read_text())['podTemplate']
    substitutions={'__CELL_UID__':'pending','__CELL_NAME__':name,'__IMAGE__':IMAGE,'__ORIGIN_HOST__':'sandbox-spike.cells.test','__CPU_LIMIT__':'1','__CPU_REQUEST__':'100m','__MEMORY_LIMIT__':'1Gi','__MEMORY_REQUEST__':'128Mi'}
    raw=json.dumps(t)
    for a,b in substitutions.items():raw=raw.replace(a,b)
    t=json.loads(raw);t['metadata']={'labels':{'agents.dsh.io/test':'agent-sandbox-local','agents.dsh.io/environment':name}}
    c=t['spec']['containers'][0];c.pop('envFrom');t['spec']['serviceAccountName']='workload'
    for role in ['data','private']:
        next(v for v in t['spec']['volumes'] if v['name']==role)['persistentVolumeClaim']['claimName']=name+'-'+role
    sb=create({'apiVersion':GROUP,'kind':'Sandbox','metadata':{'name':name,'namespace':NS,'labels':{'agents.dsh.io/test':'agent-sandbox-local'}},'spec':{'operatingMode':'Suspended','service':True,'podTemplate':t}})
    uid=sb['metadata']['uid']
    apply({'apiVersion':'v1','kind':'ServiceAccount','metadata':{'name':'workload','namespace':NS},'automountServiceAccountToken':False})
    binding={'namespace':NS,'name':name,'uid':uid,'origin':'https://sandbox-spike.cells.test','pvcNames':{},'pvcUids':{}}
    for role in ['data','private']:
        # External claims are never GC-owned; private is explicitly deleted only after stop.
        pvc=create({'apiVersion':'v1','kind':'PersistentVolumeClaim','metadata':{'name':name+'-'+role,'namespace':NS,'labels':{'agents.dsh.io/environment-uid':uid,'agents.dsh.io/test':'agent-sandbox-local','agents.dsh.io/role':role}},'spec':{'storageClassName':'standard','accessModes':['ReadWriteOnce'],'resources':{'requests':{'storage':'1Gi'}}}})
        binding['pvcNames'][role]=pvc['metadata']['name'];binding['pvcUids'][role]=pvc['metadata']['uid']
    apply({'apiVersion':'networking.k8s.io/v1','kind':'NetworkPolicy','metadata':{'name':'workload','namespace':NS},'spec':{'podSelector':{'matchLabels':{'agents.dsh.io/test':'agent-sandbox-local'}},'policyTypes':['Ingress','Egress'],'ingress':[{'from':[{'podSelector':{'matchLabels':{'agents.dsh.io/probe':'true'}}}],'ports':[{'port':8080,'protocol':'TCP'},{'port':8081,'protocol':'TCP'}]}],'egress':[{'to':[{'namespaceSelector':{'matchLabels':{'kubernetes.io/metadata.name':'kube-system'}},'podSelector':{'matchLabels':{'k8s-app':'kube-dns'}}}],'ports':[{'port':53,'protocol':'UDP'},{'port':53,'protocol':'TCP'}]}]}})
    save('binding',binding);save('template',t['spec']);save('sandbox-initial',sb)
    assert not pods(uid), 'Suspended creation must not launch a workload'
    started=time.monotonic();patch(name,'Running');sb=wait(lambda:ready(name));p=pods(uid);assert len(p)==1
    save('sandbox-running',sb);save('pod-running',p[0]);save('create-result',{'readySeconds':time.monotonic()-started,'podUid':p[0]['metadata']['uid'],'sandboxUid':uid})
    print(json.dumps({'phase':'running','pod':p[0]['metadata']['name'],'readySeconds':round(time.monotonic()-started,3)}))

if __name__=='__main__': setup()
