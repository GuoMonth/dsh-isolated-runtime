// G1 test-only sample. Production create/identity validation is B, not this script.
import { execFileSync } from 'node:child_process';
import { writeFileSync } from 'node:fs';
import { environmentTemplate } from '../../packages/environment-connector/dist/index.js';
const namespace = process.env.RC2_NAMESPACE || 'dsh-mvp-a-smoke';
const image = process.env.RC2_IMAGE;
if (!process.env.KUBECONFIG || !image) throw new Error('KUBECONFIG and RC2_IMAGE required');
const run = (args, object) => JSON.parse(execFileSync('kubectl', [...args, '-o', 'json'], {encoding:'utf8', ...(object ? {input:JSON.stringify(object)} : {})}));
const create = object => run(['create','-f','-'],object);
create({apiVersion:'v1',kind:'Namespace',metadata:{name:namespace,labels:{'dsh.isolated.io/test':'mvp-a'}}});
create({apiVersion:'v1',kind:'ServiceAccount',metadata:{name:'workload',namespace},automountServiceAccountToken:false});
const pvc=create({apiVersion:'v1',kind:'PersistentVolumeClaim',metadata:{name:'data',namespace},spec:{accessModes:['ReadWriteOnce'],storageClassName:'standard',resources:{requests:{storage:'1Gi'}}}});
const podTemplate=environmentTemplate({image,authority:'environment.test',dataClaim:'data',serviceAccount:'workload',resources:{requests:{cpu:'100m',memory:'256Mi'},limits:{cpu:'1',memory:'1Gi'}}});
const sandbox=create({apiVersion:'agents.x-k8s.io/v1beta1',kind:'Sandbox',metadata:{name:'rc2',namespace},spec:{operatingMode:'Running',service:true,podTemplate}});
const evidence={namespace,pvcUID:pvc.metadata.uid,sandboxUID:sandbox.metadata.uid,image,podTemplate};
if (process.env.RC2_EVIDENCE) writeFileSync(process.env.RC2_EVIDENCE,JSON.stringify(evidence,null,2)+'\n');
console.log(JSON.stringify({namespace,pvcUID:pvc.metadata.uid,sandboxUID:sandbox.metadata.uid}));
