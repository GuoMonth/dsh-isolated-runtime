#!/usr/bin/env python3
"""Create a disposable read-only control-plane probe, never an agent credential."""
import json
from lab import NS, OUT, ROOT, IMAGE, apply, create, call
files={f.name:f.read_text() for f in [ROOT/'test/agent-sandbox/proxy.mjs',ROOT/'test/agent-sandbox/verify-target.mjs',ROOT/'packages/cell-connector/dist/proxy.js',ROOT/'packages/cell-connector/dist/port.js',OUT/'binding.json',OUT/'template.json']}
objects=[
 {'apiVersion':'v1','kind':'ConfigMap','metadata':{'name':'probe-code','namespace':NS},'data':files},
 {'apiVersion':'v1','kind':'ServiceAccount','metadata':{'name':'probe','namespace':NS}},
 {'apiVersion':'rbac.authorization.k8s.io/v1','kind':'Role','metadata':{'name':'probe-read','namespace':NS},'rules':[{'apiGroups':[''],'resources':['pods','services','persistentvolumeclaims'],'verbs':['get','list']},{'apiGroups':['agents.x-k8s.io'],'resources':['sandboxes'],'verbs':['get']},{'apiGroups':['discovery.k8s.io'],'resources':['endpointslices'],'verbs':['get','list']}]},
 {'apiVersion':'rbac.authorization.k8s.io/v1','kind':'RoleBinding','metadata':{'name':'probe-read','namespace':NS},'subjects':[{'kind':'ServiceAccount','name':'probe','namespace':NS}],'roleRef':{'kind':'Role','name':'probe-read','apiGroup':'rbac.authorization.k8s.io'}},
]
for obj in objects:apply(obj)
create({'apiVersion':'v1','kind':'Pod','metadata':{'name':'probe','namespace':NS,'labels':{'agents.dsh.io/probe':'true'}},'spec':{'serviceAccountName':'probe','containers':[{'name':'probe','image':IMAGE,'command':['node','/spike/proxy.mjs'],'ports':[{'containerPort':30500}],'volumeMounts':[{'name':'code','mountPath':'/spike','readOnly':True}],'securityContext':{'runAsUser':1000,'runAsNonRoot':True,'allowPrivilegeEscalation':False,'readOnlyRootFilesystem':True,'capabilities':{'drop':['ALL']}}}],'volumes':[{'name':'code','configMap':{'name':'probe-code'}}]}})
call('wait','--for=condition=Ready','pod/probe','-n',NS,'--timeout=30s')
