import os
import subprocess,json,time,pathlib
k=['kubectl','--kubeconfig',os.environ['KUBECONFIG']]
def run(a):return subprocess.check_output(k+a,text=True)
run(['exec','-n','dsh-mvp-b-platform','probe','--','node','/tmp/action.mjs','start'])
v=json.loads(run(['exec','-n','dsh-mvp-b-platform','probe','--','cat','/tmp/binding.json']));ns=v['ref']['namespace'];p=json.loads(run(['get','pods','-n',ns,'-o','json']))['items'][0]
child="setInterval(()=>require('fs').appendFileSync('/var/lib/dsh/data/workspace/b-background','tick\\n'),100)"
parent="require('child_process').spawn(process.execPath,[\"-e\","+json.dumps(child)+"],{detached:true,stdio:'ignore'}).unref()"
run(['exec','-n',ns,p['metadata']['name'],'--','node','-e',parent]);time.sleep(1)
before=int(run(['exec','-n',ns,p['metadata']['name'],'--','node','-e',"console.log(require('fs').statSync('/var/lib/dsh/data/workspace/b-background').size)"]).strip());assert before>0
stopped=json.loads(run(['exec','-n','dsh-mvp-b-platform','probe','--','node','/tmp/action.mjs','stop']));assert stopped['view']['state']=='Stopped'
run(['exec','-n','dsh-mvp-b-platform','probe','--','node','/tmp/action.mjs','start'])
new=json.loads(run(['get','pods','-n',ns,'-o','json']))['items'][0]
def size():return int(run(['exec','-n',ns,new['metadata']['name'],'--','node','-e',"console.log(require('fs').statSync('/var/lib/dsh/data/workspace/b-background').size)"]).strip())
a=size();time.sleep(1);b=size();assert a==b and a>=before
result={'namespace':ns,'oldPodUID':p['metadata']['uid'],'newPodUID':new['metadata']['uid'],'beforeStopBytes':before,'afterRestartBytes':a,'afterWaitBytes':b,'backgroundResumed':False};(pathlib.Path(os.environ['EVIDENCE']) / 'background.json').write_text(json.dumps(result,indent=2)+'\n');print(json.dumps(result))
