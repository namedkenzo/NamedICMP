'use strict';
// Real browser + real authenticated API/WebSocket. Screenshots are runtime
// measurements in panel-only mode, not synthetic performance claims.
const {execFileSync,spawn}=require('node:child_process');
const fs=require('node:fs');const os=require('node:os');const path=require('node:path');
const crypto=require('node:crypto');const net=require('node:net');const WebSocket=require('ws');
const sleep=ms=>new Promise(r=>setTimeout(r,ms));
async function wait(check){for(let i=0;i<160;i++){if(await check())return;await sleep(100);}throw Error('Browser test timeout');}
async function port(){const s=net.createServer();await new Promise(r=>s.listen(0,'127.0.0.1',r));const p=s.address().port;await new Promise(r=>s.close(r));return p;}
(async()=>{
 const scratch=fs.mkdtempSync(path.join(os.tmpdir(),'namedicmp-browser-'));
 const out='/tmp/namedicmp-dashboard-previews';fs.mkdirSync(out,{recursive:true});
 let server,chrome,ws;const pending=new Map();let sequence=0;
 try{
  const binary=path.join(scratch,'namedicmp'),config=path.join(scratch,'config.json'),httpPort=await port(),debugPort=await port(),url=`http://127.0.0.1:${httpPort}/`;
  const credentials={username:'visual-qa',password:crypto.randomBytes(24).toString('hex')};
  execFileSync(process.env.GO_BINARY||'go',['build','-buildvcs=false','-o',binary,'./cmd/namedicmp'],{stdio:'inherit'});
  execFileSync(binary,['init','--config',config,'--state',path.join(scratch,'state.json'),'--listen',`127.0.0.1:${httpPort}`,'--public-host','203.0.113.25'],{input:JSON.stringify(credentials)});
  server=spawn(binary,['serve','--config',config,'--panel-only','--admin-socket='],{stdio:'ignore'});
  await wait(async()=>{try{return(await fetch(url)).ok;}catch{return false;}});
  const candidates=[process.env.CHROME_BIN,'/usr/bin/google-chrome','/usr/bin/chromium','/usr/bin/chromium-browser'].filter(Boolean);
  const executable=candidates.find(p=>fs.existsSync(p));if(!executable)throw Error('Chrome/Chromium is required for visual QA');
  chrome=spawn(executable,['--headless=new','--no-sandbox','--disable-dev-shm-usage','--no-first-run',`--user-data-dir=${path.join(scratch,'chrome')}`,`--remote-debugging-port=${debugPort}`,'--remote-debugging-address=127.0.0.1','about:blank'],{stdio:'ignore'});
  let target;
  await wait(async()=>{try{target=(await(await fetch(`http://127.0.0.1:${debugPort}/json/list`)).json()).find(t=>t.type==='page');return !!target;}catch{return false;}});
  ws=new WebSocket(target.webSocketDebuggerUrl);await new Promise((resolve,reject)=>{ws.once('open',resolve);ws.once('error',reject);});
  ws.on('message',raw=>{const message=JSON.parse(raw);const job=pending.get(message.id);if(job){pending.delete(message.id);clearTimeout(job.timer);if(message.error)job.reject(Error(JSON.stringify(message.error)));else job.resolve(message.result);}});
  function call(method,params={}){return new Promise((resolve,reject)=>{const id=++sequence,timer=setTimeout(()=>{pending.delete(id);reject(Error('CDP timeout: '+method));},10000);pending.set(id,{resolve,reject,timer});ws.send(JSON.stringify({id,method,params}));});}
  async function evaluate(expression){const result=await call('Runtime.evaluate',{expression,returnByValue:true,awaitPromise:true});if(result.exceptionDetails)throw Error(JSON.stringify(result.exceptionDetails));return result.result.value;}
  await call('Page.enable');await call('Runtime.enable');await call('Emulation.setDeviceMetricsOverride',{width:1440,height:1060,deviceScaleFactor:1,mobile:false});
  await call('Page.navigate',{url});await wait(()=>evaluate(`!!window.namedDashboard`));
  await evaluate(`document.getElementById('username').value=${JSON.stringify(credentials.username)};document.getElementById('password').value=${JSON.stringify(credentials.password)};document.getElementById('login-form').requestSubmit()`);
  await wait(()=>evaluate(`document.getElementById('live-state').dataset.state==='liveConnected'`));
  const first=await evaluate(`home.sampled_at_ms`);
  await wait(async()=>await evaluate(`home.sampled_at_ms`)>first);
  console.log('Browser verified: authenticated WebSocket receives real changing samples.');
  await evaluate(`lang='en';theme='dark';preferences()`);await evaluate('document.fonts.ready.then(()=>true)');await sleep(350);
  async function capture(name,width,height){
   await call('Emulation.setDeviceMetricsOverride',{width,height,deviceScaleFactor:1,mobile:width<600});await sleep(250);
   const bounds=await evaluate(`(()=>{const p=document.querySelector('.live-traffic').getBoundingClientRect(),s=document.getElementById('sparkline').getBoundingClientRect();return {scroll:document.documentElement.scrollWidth,width:innerWidth,plotHeight:s.height,cardHeight:p.height,metrics:document.getElementById('metrics').children.length}})()`);
   if(bounds.scroll>bounds.width+1||bounds.plotHeight<170||bounds.plotHeight>210||bounds.cardHeight>900||bounds.metrics!==4)throw Error('Invalid dashboard layout '+JSON.stringify(bounds));
   const shot=await call('Page.captureScreenshot',{format:'jpeg',quality:65,captureBeyondViewport:false});fs.writeFileSync(path.join(out,name+'.jpg'),Buffer.from(shot.data,'base64'));
   console.log(name,JSON.stringify(bounds));
   if(process.env.DASHBOARD_PREVIEW_LOG==='1')console.log('DASHBOARD_PREVIEW '+name+' '+shot.data);
  }
  await capture('dashboard-dark-en',1440,1060);
  await evaluate(`theme='light';preferences()`);await capture('dashboard-light-en',1440,1060);
  await evaluate(`lang='fa';theme='dark';preferences()`);await capture('dashboard-dark-fa-mobile',412,915);
  await evaluate(`document.querySelector('.kernel-card').open=true;document.getElementById('traffic-scope').value='interface';document.getElementById('traffic-scope').dispatchEvent(new Event('change'))`);
  await capture('dashboard-kernel-fa',1440,1060);
  await evaluate(`selectPage('users')`);await wait(()=>evaluate(`document.getElementById('live-state').dataset.state==='livePaused'`));
  await evaluate(`selectPage('home')`);await wait(()=>evaluate(`document.getElementById('live-state').dataset.state==='liveConnected'`));
  await evaluate(`document.getElementById('logout').click()`);await wait(()=>evaluate(`!document.getElementById('login').hidden`));
  console.log('Browser QA passed: dark/light, mobile RTL, bounded chart, live pause/resume and logout.');
 }finally{
  if(ws)ws.close();if(chrome)chrome.kill('SIGTERM');if(server)server.kill('SIGTERM');
  for(const job of pending.values())clearTimeout(job.timer);
  await sleep(250);fs.rmSync(scratch,{recursive:true,force:true,maxRetries:5,retryDelay:200});
 }
})().catch(error=>{console.error(error);process.exitCode=1;});
