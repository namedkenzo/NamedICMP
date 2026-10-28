// Optional DOM test against the real panel API. This does not render pixels.
'use strict';
const { JSDOM, CookieJar, VirtualConsole } = require('jsdom');
const { execFileSync, spawn } = require('node:child_process');
const fs = require('node:fs');
const os = require('node:os');
const path = require('node:path');
const net = require('node:net');
const crypto = require('node:crypto');
const project = path.resolve(__dirname, '..');
const scratch = fs.mkdtempSync(path.join(os.tmpdir(), 'namedicmp-ui-test-'));
const credentials = { username: 'qa', password: crypto.randomBytes(24).toString('hex') };
const wait = async (check) => { for (let i=0;i<500;i++) { if (await check()) return; await new Promise(r=>setTimeout(r,10)); } throw new Error('DOM test timeout'); };
async function availablePort() { const s=net.createServer(); await new Promise(r=>s.listen(0,'127.0.0.1',r));const port=s.address().port;await new Promise(r=>s.close(r));return port; }
(async()=>{
 let server,dom;
 try {
  const port=await availablePort(),url=`http://127.0.0.1:${port}/`,binary=path.join(scratch,'namedicmp'),config=path.join(scratch,'config.json');
  execFileSync(process.env.GO_BINARY||'go',['build','-buildvcs=false','-o',binary,'./cmd/namedicmp'],{cwd:project,stdio:'inherit'});
  execFileSync(binary,['init','--config',config,'--state',path.join(scratch,'state.json'),'--listen',`127.0.0.1:${port}`,'--public-host','203.0.113.25'],{input:JSON.stringify(credentials)});
  server=spawn(binary,['serve','--config',config,'--panel-only','--admin-socket='],{stdio:['ignore','ignore','pipe']});
  let stderr='';server.stderr.on('data',b=>stderr+=b.toString());
  await wait(async()=>{if(server.exitCode!==null)throw new Error(stderr);try{return (await fetch(url)).ok;}catch{return false;}});
  const jar=new CookieJar(),errors=[],consoleEvents=new VirtualConsole();consoleEvents.on('jsdomError',e=>errors.push(e.message));
  dom=await JSDOM.fromURL(url,{resources:'usable',runScripts:'dangerously',pretendToBeVisual:true,cookieJar:jar,virtualConsole:consoleEvents,beforeParse(w){
   // WebSocket auth, revocation and flow control are exercised by Go tests.
   // This DOM pass checks the explicit unavailable/fallback presentation.
   w.WebSocket=undefined;
   w.matchMedia=()=>({matches:false,addEventListener(){},removeEventListener(){}});w.confirm=()=>true;
   w.document.execCommand=()=>{w.copiedText=w.document.querySelector('#editor textarea').value;return true;};
   w.HTMLDialogElement.prototype.showModal=function(){this.setAttribute('open','');};
   w.HTMLDialogElement.prototype.close=function(){this.removeAttribute('open');};
   w.fetch=async(resource,options={})=>{const target=new URL(resource,url).href,headers={...options.headers},cookie=jar.getCookieStringSync(target);if(cookie)headers.Cookie=cookie;const response=await fetch(target,{...options,headers});for(const value of response.headers.getSetCookie())jar.setCookieSync(value,target);return response;};
  }});
  const w=dom.window,d=w.document,submit=id=>d.querySelector(id).dispatchEvent(new w.Event('submit',{bubbles:true,cancelable:true}));
  await wait(()=>d.querySelector('#sign-in').textContent.includes('ورود'));
  d.querySelector('#username').value=credentials.username;d.querySelector('#password').value=credentials.password;submit('#login-form');
  await wait(()=>!d.querySelector('#app').hidden);await wait(()=>d.querySelector('#metrics').children.length===4);
  if(!d.querySelector('[data-i18n="home"]').textContent.includes('داشبورد'))throw Error('Home was not renamed to Dashboard');
  if(!d.querySelector('#live-state-label').textContent.includes('۳۰'))throw Error('Missing explicit live fallback');
  if(d.querySelector('#probe-cards').textContent.includes('0 ms'))throw Error('Unavailable probe displayed a fake ping');
  if(w.eval('bytes(0.5)').includes('undefined'))throw Error('Sub-byte rate formatting uses an invalid unit');
  if(d.querySelector('#sparkline path'))throw Error('Disabled core fabricated a traffic series');
  d.querySelector('[data-page="users"]').click();d.querySelector('#add-user').click();if(d.querySelector('select[name="encryption"]').value!=='none')throw new Error('Encryption is not off by default');d.querySelector('input[name="name"]').value='DOM-test-user';submit('#editor-form');
  await wait(()=>!d.querySelector('#editor').hasAttribute('open'));await wait(()=>d.querySelector('#users-body').textContent.includes('DOM-test-user'));
  d.querySelector('#users-body .row-actions button').click();const uri=d.querySelector('#connection-uri').textContent;if(!/^pingtunnel:\/\/203\.0\.113\.25\?lport=1080&mode=vpn&key=\d+$/.test(uri))throw new Error('Incorrect connection URI');d.querySelector('#copy-connection-uri').click();await wait(()=>w.copiedText===uri);d.querySelector('#close-editor').click();
  d.querySelector('#users-body .row-actions button:nth-child(2)').click();d.querySelector('select[name="encryption"]').value='aes256';submit('#editor-form');await wait(()=>!d.querySelector('#editor').hasAttribute('open'));await wait(()=>!d.querySelector('#save-editor').disabled);d.querySelector('#users-body .row-actions button').click();await wait(()=>d.querySelector('#editor-fields').textContent.includes('AES-256-GCM'));if(!d.querySelector('#editor-fields').textContent.includes('-encrypt aes256 -encrypt-key'))throw new Error('Encryption parameters not exported');const encryptedURI=new URL(d.querySelector('#connection-uri').textContent);if(encryptedURI.searchParams.has('key')||encryptedURI.searchParams.get('encrypt')!=='aes256'||!encryptedURI.searchParams.get('encrypt_key'))throw new Error('Encrypted URI must contain encryption only');if(d.querySelector('#editor-fields').textContent.includes(' -key '))throw new Error('Numeric password in encrypted command');d.querySelector('#copy-connection-uri').click();await wait(()=>w.copiedText===encryptedURI.href);d.querySelector('#close-editor').click();
  d.querySelector('[data-page="settings"]').click();d.querySelector('#settings-outbounds-tab').click();d.querySelector('#add-outbound').click();d.querySelector('input[name="name"]').value='DOM-test-proxy';d.querySelector('input[name="host"]').value='127.0.0.1';submit('#editor-form');
  await wait(()=>!d.querySelector('#editor').hasAttribute('open'));await wait(()=>d.querySelector('#outbound-cards').textContent.includes('DOM-test-proxy'));
  d.querySelector('#settings-connection-tab').click();
  d.querySelector('#endpoint-family').value='ipv6';d.querySelector('#endpoint-family').dispatchEvent(new w.Event('input',{bubbles:true}));
  if(d.querySelector('#endpoint-host').value!=='')throw Error('Switching family kept the incompatible old literal instead of automatic detection');
  d.querySelector('#endpoint-host').value='2001:db8::25';d.querySelector('#endpoint-host').dispatchEvent(new w.Event('input',{bubbles:true}));submit('#connection-settings-form');
  await wait(()=>!d.querySelector('#save-connection-settings').disabled);
  if(d.querySelector('#settings-error').textContent)throw Error(d.querySelector('#settings-error').textContent);
  d.querySelector('[data-page="users"]').click();d.querySelector('#users-body .row-actions button').click();
  let ipv6URI=new URL(d.querySelector('#connection-uri').textContent);if(ipv6URI.hostname!=='[2001:db8::25]'||ipv6URI.searchParams.get('family')!=='ipv6')throw Error('IPv6 literal export failed');d.querySelector('#close-editor').click();
  d.querySelector('[data-page="settings"]').click();d.querySelector('#endpoint-host').value='vpn.example.com';d.querySelector('#endpoint-host').dispatchEvent(new w.Event('input',{bubbles:true}));submit('#connection-settings-form');await wait(()=>!d.querySelector('#save-connection-settings').disabled);
  d.querySelector('[data-page="users"]').click();d.querySelector('#users-body .row-actions button').click();ipv6URI=new URL(d.querySelector('#connection-uri').textContent);if(ipv6URI.hostname!=='vpn.example.com'||ipv6URI.searchParams.get('family')!=='ipv6')throw Error('IPv6 domain export failed');d.querySelector('#close-editor').click();
  const rawFetch=w.fetch;w.fetch=async(resource,options={})=>{const response=await rawFetch(resource,options);if(String(resource).endsWith('/api/home')){const body=await response.json();body.connection.addresses={ipv4:['198.51.100.25'],ipv6:['2001:db8::25']};return {ok:response.ok,status:response.status,json:async()=>body};}return response;};
  await wait(async()=>{await w.eval('refresh()');return w.eval('home.connection.addresses.ipv6.includes("2001:db8::25")');});d.querySelector('[data-page="home"]').click();
  if(d.querySelector('#host-addresses').textContent.includes('2001:db8::25')||d.querySelector('#host-addresses').textContent.includes('198.51.100.25'))throw Error('IP revealed before tap');
  d.querySelector('#home-ipv6').click();if(!d.querySelector('#home-ipv6').textContent.includes('2001:db8::25'))throw Error('IPv6 reveal failed');d.querySelector('#home-ipv6').click();if(d.querySelector('#home-ipv6').textContent.includes('2001:db8::25'))throw Error('IPv6 hide failed');
  d.querySelector('#home-ipv4').click();if(!d.querySelector('#home-ipv4').textContent.includes('198.51.100.25'))throw Error('IPv4 reveal failed');d.querySelector('#home-ipv4').click();
  d.querySelector('#app .language-toggle').click();if(d.documentElement.dir!=='ltr')throw new Error('English direction failed');
  if(d.documentElement.dataset.theme!=='light')throw new Error('System light preference failed');
  d.querySelector('#app .theme-toggle').click();if(d.documentElement.dataset.theme!=='dark')throw new Error('Dark mode failed');
  d.querySelector('#app .theme-toggle').click();if(d.documentElement.dataset.theme!=='light')throw new Error('Light mode failed');
  d.querySelector('[data-page="home"]').click();if(d.querySelector('#page-home').hidden)throw new Error('Navigation failed');
  // Deterministic fixture for the new diagnostic labels; not a performance or
  // production-network measurement. Missing fields must remain unavailable.
  w.eval(`home.core_enabled=true;home.core={...home.core,busiest_user_tcp_sessions:32,max_sessions_per_user:32,busiest_user_udp_sessions:128,max_udp_sessions_per_user:128,tcp_queue_retries:7,udp_queue_evicted:9,session_limit_tcp_user:2};namedDashboard.render()`);
  const pressure=d.querySelector('#pacing-info');
  if(pressure.querySelectorAll('.has-warning').length!==2)throw Error('Per-user capacity warning missing');
  const reasons=d.querySelector('#pressure-info');
  if(!reasons.textContent.includes('TCP queue admission retries')||!reasons.textContent.includes('UDP oldest-packet evictions')||!reasons.textContent.includes('Per-user TCP rejects'))throw Error('Queue/admission causes not separately labelled');
  if(d.querySelector('.pressure-details').open)throw Error('Detailed counters should start collapsed');
  d.querySelector('.pressure-details').open=true;
  d.querySelector('#logout').click();await wait(()=>!d.querySelector('#login').hidden);
  if(errors.length)throw new Error(errors.join('\n'));
  console.log('DOM smoke passed: login, metrics, users, default-off encryption, AES selection, exact connection URI and clipboard copy, outbounds, languages, themes, navigation and logout.');
 } finally {
  if(dom)dom.window.close();if(server&&server.exitCode===null){server.kill('SIGTERM');await new Promise(r=>server.once('exit',r));}
  fs.rmSync(scratch,{recursive:true,force:true});
 }
})().catch(e=>{console.error(e);process.exitCode=1;});

