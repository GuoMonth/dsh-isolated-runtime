import fs from 'node:fs';
import path from 'node:path';
const [image, issuer, directory] = process.argv.slice(2);
const shared = ['--gateway-name=dsh', '--gateway-namespace=dsh-system', '--gateway-section-name=https', '--base-domain=cells.test', '--external-https-port=18443'];
for (const [file, container] of [
  ['operator', {name:'manager', image, args:[...shared, '--cell-concurrency=4', '--snapshot-concurrency=2']}],
  ['authorizer', {name:'authorizer', image, args:[...shared, `--oidc-issuer=${issuer}`, '--oidc-client-id=dsh-browser'], env:[{name:'SSL_CERT_FILE',value:'/etc/dsh-ca/ca.crt'}],volumeMounts:[{name:'dex-ca',mountPath:'/etc/dsh-ca',readOnly:true}]}],
]) {
  const spec = {containers:[container]};
  if (file === 'authorizer') spec.volumes = [{name:'dex-ca',configMap:{name:'dex-ca'}}];
  fs.writeFileSync(path.join(directory, `${file}.json`), JSON.stringify({spec:{template:{spec}}}));
}
