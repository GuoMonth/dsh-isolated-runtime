// Test-only in-cluster host for the unchanged production HTTP/WS transport.
import fs from 'node:fs';
import https from 'node:https';
import http from 'node:http';
import { connector } from './proxy.js';
import { verifyTarget } from './verify-target.mjs';
const binding=JSON.parse(fs.readFileSync('/spike/binding.json','utf8'));
const expectedPodSpec=JSON.parse(fs.readFileSync('/spike/template.json','utf8'));
const token=fs.readFileSync('/var/run/secrets/kubernetes.io/serviceaccount/token','utf8');
const ca=fs.readFileSync('/var/run/secrets/kubernetes.io/serviceaccount/ca.crt');
let reads=0, requests=0, revoked=false;
const active=new Set();
function get(path){reads++;return new Promise((resolve,reject)=>{
 const req=https.get({hostname:process.env.KUBERNETES_SERVICE_HOST,port:443,path,ca,headers:{authorization:`Bearer ${token}`},timeout:5000},res=>{let body='';res.on('data',x=>body+=x);res.on('end',()=>{if(res.statusCode!==200)return reject(new Error(`KubernetesReadFailed:${res.statusCode}`));try{resolve(JSON.parse(body));}catch(e){reject(e);}});});req.on('timeout',()=>req.destroy(new Error('KubernetesReadTimeout')));req.on('error',reject);
});}
async function resolve(){
 if(revoked)throw new Error('AccessRevoked');
 const n=encodeURIComponent(binding.namespace),name=encodeURIComponent(binding.name),base=`/api/v1/namespaces/${n}`;
 const [sandbox,list,service,endpoints,data,privatePvc]=await Promise.all([
 get(`/apis/agents.x-k8s.io/v1beta1/namespaces/${n}/sandboxes/${name}`),get(`${base}/pods`),get(`${base}/services/${name}`),get(`/apis/discovery.k8s.io/v1/namespaces/${n}/endpointslices?labelSelector=kubernetes.io%2Fservice-name%3D${name}`),get(`${base}/persistentvolumeclaims/${binding.pvcNames.data}`),get(`${base}/persistentvolumeclaims/${binding.pvcNames.private}`)]);
 const owned=list.items.filter(p=>p.metadata.ownerReferences?.some(o=>o.controller&&o.uid===binding.uid));
 if(owned.length!==1)throw new Error('PodOwnershipMismatch');
 return verifyTarget({sandbox,pod:owned[0],service,endpoints,pvcs:{data,private:privatePvc},binding,expectedPodSpec});
}
function ctx(){const c=new AbortController();active.add(c);return c;}
const server=http.createServer((req,res)=>{
 requests++;const c=ctx();res.once('close',()=>active.delete(c));
 connector(binding.origin,{signal:c.signal,correlationId:'sandbox-spike'},resolve).forward(req,res).catch(e=>{console.error(e.message);if(!res.headersSent)res.writeHead(503);res.end(e.code||e.message);});
});
server.on('upgrade',(req,socket,head)=>{requests++;const c=ctx();socket.once('close',()=>active.delete(c));connector(binding.origin,{signal:c.signal,correlationId:'sandbox-spike-ws'},resolve).upgrade(req,socket,head).catch(e=>{console.error(e.message);socket.destroy();});});
server.listen(30500,'0.0.0.0');
// Administration is reachable only from kubectl exec inside this probe container.
http.createServer((req,res)=>{if(req.url==='/revoke'){revoked=true;for(const c of active)c.abort();active.clear();}else if(req.url==='/allow'){revoked=false;}res.setHeader('content-type','application/json');res.end(JSON.stringify({reads,requests,revoked,active:active.size}));}).listen(30501,'127.0.0.1');
