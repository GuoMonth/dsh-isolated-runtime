const fs = require('node:fs');
const path = require('node:path');
const crypto = require('node:crypto');
const [mode, root] = process.argv.slice(2);
const credentialsPath = path.join(root, 'credentials.json');
const secretPath = path.join(root, 'identity/client-secret');
if (mode === 'prepare') {
  if (!fs.existsSync(credentialsPath)) fs.writeFileSync(credentialsPath, JSON.stringify({
    email: 'owner@localhost', password: crypto.randomBytes(24).toString('base64url'),
  }) + '\n', {mode: 0o600, flag: 'wx'});
  if (!fs.existsSync(secretPath)) fs.writeFileSync(secretPath, crypto.randomBytes(32).toString('base64url'), {mode: 0o600, flag: 'wx'});
} else if (mode === 'render') {
  const bcrypt = require(path.join(root, 'identity/node_modules/bcryptjs'));
  const credentials = JSON.parse(fs.readFileSync(credentialsPath));
  const clientSecret = fs.readFileSync(secretPath, 'utf8');
  const list = JSON.parse(fs.readFileSync(0, 'utf8'));
  for (const item of list.items) {
    if (item.kind === 'ConfigMap' && item.metadata.name === 'dex') {
      item.kind = 'Secret'; delete item.data;
      item.stringData = {'config.yaml': JSON.stringify({
        issuer: 'https://dex.dsh-system.svc:15556/dex', storage: {type: 'memory'},
        web: {https: '0.0.0.0:5554', tlsCert: '/etc/dex/tls/tls.crt', tlsKey: '/etc/dex/tls/tls.key'},
        oauth2: {skipApprovalScreen: true}, enablePasswordDB: true,
        staticClients: [{id: 'dsh-browser', name: 'DSH Browser', secret: clientSecret,
          redirectURIs: ['https://auth.cells.test:18443/oauth2/callback']}],
        // Keep the opaque subject so local RoleBindings remain compatible.
        staticPasswords: [{email: credentials.email, hash: bcrypt.hashSync(credentials.password, 12),
          username: 'owner', name: 'Owner', userID: 'alice-sub', groups: ['developers']}],
      })};
    }
    if (item.kind === 'Deployment' && item.metadata.name === 'dex') {
      const volume = item.spec.template.spec.volumes.find(v => v.name === 'config');
      delete volume.configMap; volume.secret = {secretName: 'dex'};
    }
  }
  process.stdout.write(JSON.stringify(list));
} else throw new Error('usage: identity.cjs prepare|render STATE_DIRECTORY');
