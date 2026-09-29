"""Wait for real watch-cache expiry, then stop the unchanged healthy writer."""
import json, os, pathlib, subprocess, time, urllib.parse
k = ['kubectl', '--kubeconfig', os.environ['KUBECONFIG']]
def run(args):
    return subprocess.check_output(k + args, text=True)
v = json.loads(run(['exec', '-n', 'dsh-mvp-b-platform', 'probe', '--', 'cat', '/tmp/binding.json']))
ns = v['ref']['namespace']
pod = json.loads(run(['get', 'pods', '-n', ns, '-o', 'json']))['items'][0]
uid, revision = pod['metadata']['uid'], pod['metadata']['resourceVersion']
path = '/api/v1/namespaces/' + ns + '/pods?' + urllib.parse.urlencode({'watch':'true', 'fieldSelector':'metadata.name=' + pod['metadata']['name'], 'resourceVersion':revision, 'timeoutSeconds':'1'})
deadline = time.monotonic() + 600
expired = None
started = time.monotonic()
advanced = False
while time.monotonic() < deadline:
    # Advance the real Pod watch cache using only the task's observer Pod.
    # The workload Pod itself and its item RV remain unchanged.
    run(['annotate', 'pod', 'probe', '-n', 'dsh-mvp-b-platform', 'environment.dsh.io/test-cache-tick=' + str(time.time_ns()), '--overwrite'])
    if not advanced and time.monotonic() - started > 90:
        # Bounded, genuine cache churn after the unchanged writer has aged.
        for tick in range(128):
            run(['annotate', 'pod', 'probe', '-n', 'dsh-mvp-b-platform', 'environment.dsh.io/test-cache-tick=batch-' + str(tick), '--overwrite'])
        advanced = True
    raw = run(['get', '--raw', path])
    for line in raw.splitlines():
        event = json.loads(line)
        if event.get('type') == 'ERROR' and event.get('object', {}).get('code') == 410:
            expired = event['object']; break
    if expired: break
    time.sleep(10)
assert expired, 'Real Pod item RV did not expire within 10 minutes'
current = json.loads(run(['get', 'pod', pod['metadata']['name'], '-n', ns, '-o', 'json']))
assert current['metadata']['uid'] == uid and current['metadata']['resourceVersion'] == revision
stopped = json.loads(run(['exec', '-n', 'dsh-mvp-b-platform', 'probe', '--', 'node', '/tmp/action.mjs', 'stop']))
assert stopped['view']['state'] == 'Stopped'
sb = json.loads(run(['get', 'sandbox', 'environment', '-n', ns, '-o', 'json']))
a = sb['metadata']['annotations']
assert a['environment.dsh.io/writer-uid'] == uid == a['environment.dsh.io/terminal-uid']
assert int(a['environment.dsh.io/watch-rv']) > int(revision)
result = {'namespace':ns, 'writerUID':uid, 'oldItemRV':revision, 'actualExpiredResponse':expired, 'stopListRV':a['environment.dsh.io/watch-rv'], 'view':stopped['view']}
(pathlib.Path(os.environ['EVIDENCE']) / 'aged-writer.json').write_text(json.dumps(result, indent=2) + '\n')
print(json.dumps(result))
