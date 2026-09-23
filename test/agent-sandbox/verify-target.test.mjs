// Negative cases use a real cluster snapshot; no mock Pod identity is invented.
import fs from 'node:fs';
import assert from 'node:assert/strict';
import {verifyTarget} from './verify-target.mjs';
const original=JSON.parse(fs.readFileSync(process.env.SANDBOX_SNAPSHOT,'utf8'));
assert.equal(verifyTarget(original),original.pod.status.podIP);
const cases=[
 ['wrong Sandbox UID',s=>{s.binding.uid='wrong';}],
 ['stale Ready generation',s=>{s.sandbox.metadata.generation++;}],
 ['wrong Pod owner',s=>{s.pod.metadata.ownerReferences[0].uid='wrong';}],
 ['extra container',s=>{s.pod.spec.containers.push(structuredClone(s.pod.spec.containers[0]));}],
 ['extra credential injection',s=>{s.pod.spec.containers[0].envFrom=[{secretRef:{name:'foreign'}}];}],
 ['host network',s=>{s.pod.spec.hostNetwork=true;}],
 ['changed image policy',s=>{s.pod.spec.containers[0].imagePullPolicy='Always';}],
 ['wrong PVC UID',s=>{s.pvcs.data.metadata.uid='wrong';}],
 ['foreign PVC owner',s=>{s.pvcs.data.metadata.ownerReferences=[{uid:'wrong'}];}],
 ['unbound PVC',s=>{s.pvcs.data.status.phase='Pending';delete s.pvcs.data.spec.volumeName;}],
 ['missing PVC',s=>{delete s.pvcs.private;}],
 ['wrong endpoint UID',s=>{s.endpoints.items[0].endpoints[0].targetRef.uid='wrong';}],
 ['extra ready endpoint',s=>{s.endpoints.items[0].endpoints.push(structuredClone(s.endpoints.items[0].endpoints[0]));}],
 ['foreign Service',s=>{s.service.metadata.ownerReferences[0].uid='wrong';}],
 ['suspended environment',s=>{s.sandbox.spec.operatingMode='Suspended';}],
];
const results=[];
for(const [name,change] of cases){const s=structuredClone(original);change(s);let rejected=false,code;try{verifyTarget(s);}catch(e){rejected=true;code=e.code;}assert.ok(rejected,name);results.push({name,rejected,code});}
console.log(JSON.stringify({positive:true,negativePassed:results.length,results},null,2));
