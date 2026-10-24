'use strict';
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const root = path.resolve(__dirname, '..');
const read = file => fs.readFileSync(path.join(root, file), 'utf8');
const server = read('cmd/namedicmp/main.go').match(/const version = "([^"]+)"/)[1];
assert.match(server, /^\d+\.\d+\.\d+$/, 'server release must have a stable version');
assert.equal(read('install.sh').match(/^MANAGER_VERSION=(.+)$/m)[1], server);
assert.equal(JSON.parse(read('package.json')).version, server);
assert.equal(JSON.parse(read('package-lock.json')).version, server);
assert.ok(read('internal/panel/panel.go').includes('"version": "' + server + '"'));
assert.ok(read('internal/panel/web/index.html').includes(server + ' / STABLE'));
assert.ok(fs.existsSync(path.join(root, `docs/RELEASE-v${server}.md`)));
assert.ok(read('.github/workflows/build.yml').includes("tags: ['v*']"));
for (const file of ['clients/namedP/namedP.csproj','cmd/namedp-engine/main.go','internal/client/transport.go','.github/workflows/namedp.yml']) {
  assert.ok(!fs.existsSync(path.join(root, file)), 'Client app code belongs in NamedClients: '+file);
}
assert.ok(read('README.md').includes('https://github.com/namedkenzo/NamedClients'));
assert.ok(read('README.fa.md').includes('https://github.com/namedkenzo/NamedClients'));
console.log(`Server/panel ${server} verified; client applications are separate.`);
