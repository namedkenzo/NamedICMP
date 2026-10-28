'use strict';
// No charting CDN, third-party telemetry or per-browser ping process.
Object.assign(dictionary.en, {
 home:'Dashboard',homeTitle:'Dashboard',overview:'NETWORK OBSERVATORY',homeHint:'A live view of your network, from packets to performance.',
 liveNetwork:'Live Network',liveConnected:'Live · 1 second',liveConnecting:'Connecting…',liveReconnecting:'Reconnecting…',livePaused:'Live view paused',liveStale:'Stale · retrying',liveUnsupported:'WebSocket unavailable · 30s refresh',
 tunnelPayload:'Tunnel payload',interfaceTraffic:'VPS interfaces',packetRate:'ICMP packets / s',fiveMinutes:'Last 5 minutes · memory only',now:'Now',
 pathQuality:'Path quality',probeHint:'VPS → target, not client/game ping. One echo per family every 10 seconds; last 30 sent probes. Filtering can prevent replies.',
 ping:'Ping',packetLoss:'Packet loss',jitter:'Jitter',jitterHint:'Mean absolute RTT difference between consecutive successful probes; timeouts break the pair.',
 probeWaiting:'Waiting for first probe',probeDisabled:'ICMP core disabled',probeUnavailable:'Probe unavailable',probeTimeout:'No echo reply',probeOK:'Echo reply received',probeWindow:'Replies / sent',probeRange:'Min / average / max',lastProbe:'Last probe',unknown:'Unavailable',
 payloadHint:'Client upload / download payload. Excludes ICMP headers and TCP retransmissions; not a bandwidth test.',
 networkHint:'All non-loopback interfaces, including other services. Virtual interfaces can count the same packet more than once.',packetHint:'Core ICMP traffic, including control packets, rejected packets and retransmissions. Not a packet-loss measurement.',
 coreErrors:'Drops & outbound errors',queueHealth:'Queues & pacing',interfaceHealth:'Interface health',hostWide:'HOST-WIDE',
 dropped:'Core discard / retry events',dropHint:'Counters since service start, not measured network packet loss. TCP queue rejection is retried locally. A historical event alone does not mean the current connection is unhealthy.',
 activeFlows:'Active flows',activeUsers:'Active users',flowHint:'TCP / UDP flows, not VPN devices',coreOff:'ICMP off',storageOK:'Usage storage OK',storageError:'Usage storage error',listener:'Listener',notListening:'Not listening',
 rejected:'Rejected packets',outboundErrors:'Outbound failures',dnsErrors:'DNS failures',policyErrors:'Policy blocks',timeouts:'Outbound timeouts',networkErrors:'Network failures',otherErrors:'Other outbound failures',socketErrors:'Socket write failures',oversizedUDP:'Oversized UDP',queueDrops:'Writer queue pressure events',sessionDrops:'Session queue drops',sessionLimits:'Session limit rejects',
 controlQueue:'Control queue',tcpQueue:'TCP queue',udpQueue:'UDP queue',pacing:'ICMP send limit',burst:'Burst allowance',sessionCapacity:'Flow capacity',udpLimit:'Maximum UDP payload',
 tcpQueueRetries:'TCP queue admission retries',controlQueueDrops:'Control queue drops',udpQueueEvicted:'UDP oldest-packet evictions',udpQueueRejected:'UDP new-packet rejects',
 globalFlowRejects:'Global flow-cap rejects',tcpUserRejects:'Per-user TCP rejects',udpUserRejects:'Per-user UDP rejects',stoppingRejects:'Shutdown rejects',
 busiestTCP:'Busiest user · TCP / limit',busiestUDP:'Busiest user · UDP / limit',udpIdleTimeouts:'UDP idle expirations',udpReadErrors:'UDP outbound read errors',udpWriteErrors:'UDP outbound write errors',
 pressureDetails:'Detailed queue & capacity reasons',
 rxTotal:'Interface received',txTotal:'Interface sent',rxErrors:'RX errors',txErrors:'TX errors',rxDrops:'RX drops',txDrops:'TX drops',
 kernelDiagnostics:'Kernel ICMP counters & read-only network settings',kernelHint:'Host-wide totals since boot. These are not tunnel-only statistics. No firewall or sysctl setting is changed by this dashboard.',networkSettings:'Network settings',
 heapMemory:'Go heap allocated',processMemory:'Process resident memory',goroutines:'Go routines',fileDescriptors:'Open file descriptors',swap:'Swap used',packetsSecond:'packets/s',trafficMeasurement:'Traffic measurement',trafficChart:'Live traffic chart',refreshAt:'Sampled',
});
Object.assign(dictionary.fa, {
 home:'داشبورد',homeTitle:'داشبورد',overview:'دیدبان شبکه',homeHint:'نمای زندهٔ شبکهٔ شما؛ از بسته‌ها تا عملکرد.',
 liveNetwork:'شبکهٔ زنده',liveConnected:'زنده · هر ۱ ثانیه',liveConnecting:'در حال اتصال…',liveReconnecting:'اتصال مجدد…',livePaused:'نمای زنده متوقف است',liveStale:'داده قدیمی · تلاش مجدد',liveUnsupported:'WebSocket در دسترس نیست · به‌روزرسانی ۳۰ ثانیه‌ای',
 tunnelPayload:'محتوای تونل',interfaceTraffic:'رابط‌های VPS',packetRate:'بستهٔ ICMP در ثانیه',fiveMinutes:'۵ دقیقهٔ اخیر · فقط در حافظه',now:'اکنون',
 pathQuality:'کیفیت مسیر',probeHint:'از VPS تا مقصد، نه پینگ کلاینت یا بازی. یک Echo برای هر خانواده هر ۱۰ ثانیه؛ پنجرهٔ ۳۰ ارسال اخیر. فیلتر ICMP ممکن است مانع پاسخ شود.',
 ping:'پینگ',packetLoss:'افت بسته',jitter:'جیتر',jitterHint:'میانگین اختلاف مطلق RTT پاسخ‌های موفقِ متوالی؛ پس از بی‌پاسخ‌ماندن، جفت جدید آغاز می‌شود.',
 probeWaiting:'در انتظار اولین تست',probeDisabled:'هستهٔ ICMP خاموش است',probeUnavailable:'تست در دسترس نیست',probeTimeout:'بدون پاسخ Echo',probeOK:'پاسخ Echo دریافت شد',probeWindow:'پاسخ / ارسال',probeRange:'کمینه / میانگین / بیشینه',lastProbe:'آخرین تست',unknown:'نامشخص',
 payloadHint:'محتوای آپلود و دانلود از دید کلاینت؛ بدون سربار ICMP و ارسال مجدد TCP. این نمودار تست ظرفیت پهنای باند نیست.',
 networkHint:'همهٔ رابط‌ها جز loopback، شامل سرویس‌های دیگر. رابط‌های مجازی ممکن است یک بسته را چند بار بشمارند.',packetHint:'ترافیک ICMP هسته، شامل بسته‌های کنترل، بسته‌های ردشده و ارسال مجدد؛ این عدد اندازه‌گیری افت بسته نیست.',
 coreErrors:'حذف بسته و خطاهای خروجی',queueHealth:'صف‌ها و کنترل نرخ',interfaceHealth:'سلامت رابط شبکه',hostWide:'کل میزبان',
 dropped:'رویدادهای حذف یا تلاش دوبارهٔ هسته',dropHint:'شمارنده‌ها از شروع سرویس‌اند، نه افت اندازه‌گیری‌شدهٔ مسیر. رد ورود TCP به صف، محلی دوباره تلاش می‌شود. رویداد قدیمی لزوماً به معنی خرابی فعلی نیست.',
 activeFlows:'جریان‌های فعال',activeUsers:'کاربران فعال',flowHint:'جریان‌های TCP / UDP، نه تعداد دستگاه VPN',coreOff:'ICMP خاموش',storageOK:'ذخیرهٔ مصرف سالم',storageError:'خطا در ذخیرهٔ مصرف',listener:'دریافت ICMP',notListening:'غیرفعال',
 rejected:'بسته‌های ردشده',outboundErrors:'خطاهای خروجی',dnsErrors:'خطای DNS',policyErrors:'مسدودشده طبق سیاست',timeouts:'مهلت خروجی تمام‌شده',networkErrors:'خطاهای شبکه',otherErrors:'سایر خطاهای خروجی',socketErrors:'خطای نوشتن سوکت',oversizedUDP:'بستهٔ UDP بزرگ‌تر از حد',queueDrops:'رویدادهای فشار صف ارسال',sessionDrops:'حذف از صف جریان',sessionLimits:'رد به‌دلیل سقف جریان',
 controlQueue:'صف کنترل',tcpQueue:'صف TCP',udpQueue:'صف UDP',pacing:'سقف ارسال ICMP',burst:'ظرفیت ارسال لحظه‌ای',sessionCapacity:'ظرفیت جریان',udpLimit:'بیشینهٔ محتوای UDP',
 tcpQueueRetries:'تلاش دوباره برای ورود TCP به صف',controlQueueDrops:'حذف از صف کنترل',udpQueueEvicted:'حذف قدیمی‌ترین بستهٔ UDP',udpQueueRejected:'رد بستهٔ جدید UDP',
 globalFlowRejects:'رد به‌دلیل سقف کل جریان‌ها',tcpUserRejects:'رد به‌دلیل سقف TCP کاربر',udpUserRejects:'رد به‌دلیل سقف UDP کاربر',stoppingRejects:'رد هنگام توقف هسته',
 busiestTCP:'بیشترین TCP یک کاربر / سقف',busiestUDP:'بیشترین UDP یک کاربر / سقف',udpIdleTimeouts:'انقضای UDP غیرفعال',udpReadErrors:'خطای خواندن خروجی UDP',udpWriteErrors:'خطای نوشتن خروجی UDP',
 pressureDetails:'جزئیات علت‌های فشار صف و ظرفیت',
 rxTotal:'کل دریافت رابط',txTotal:'کل ارسال رابط',rxErrors:'خطای دریافت',txErrors:'خطای ارسال',rxDrops:'حذف در دریافت',txDrops:'حذف در ارسال',
 kernelDiagnostics:'شمارنده‌های ICMP کرنل و تنظیمات فقط‌خواندنی شبکه',kernelHint:'آمار کل میزبان از زمان روشن‌شدن؛ منحصر به تونل نیست. داشبورد هیچ تنظیم فایروال یا sysctl را تغییر نمی‌دهد.',networkSettings:'تنظیمات شبکه',
 heapMemory:'حافظهٔ تخصیص‌یافتهٔ Go',processMemory:'حافظهٔ مقیم فرایند',goroutines:'روتین‌های Go',fileDescriptors:'فایل‌های باز',swap:'مصرف Swap',packetsSecond:'بسته/ثانیه',trafficMeasurement:'نوع اندازه‌گیری ترافیک',trafficChart:'نمودار زندهٔ ترافیک',refreshAt:'زمان نمونه',
});
iconPaths.home=['M3 3h7v7H3zM14 3h7v7h-7zM3 14h7v7H3zM14 14h7v7h-7z'];
document.querySelector('[data-icon="home"]').replaceChildren(icon('home'));

window.namedDashboard = (()=>{
 const points=[];
 let socket=null,retry=null,watch=null,failures=0,lastReceived=0,state='liveConnecting';
 const available=n=>typeof n==='number'&&Number.isFinite(n);
 const number=n=>available(n)?formatNumber(n):'—';
 const ms=n=>available(n)?formatNumber(n)+' ms':'—';
 const rate=n=>available(n)?bytes(n)+'/s':'—';
 const amount=n=>available(n)?bytes(n):'—';
 const clock=n=>n?new Date(n).toLocaleTimeString(lang==='fa'?'fa-IR':'en-GB'):'—';
 const eligible=()=>!!session&&page==='home'&&!document.hidden&&!$('app').hidden;
 function liveState(value){state=value;$('live-state-label').textContent=t(value);$('live-state').dataset.state=value;}
 function stop(){clearTimeout(retry);clearInterval(watch);retry=watch=null;if(socket){const old=socket;socket=null;old.onclose=null;old.close();}liveState('livePaused');}
 function reset(){stop();points.length=0;lastReceived=0;failures=0;}
 function push(point){if(!point||!available(point.at))return;const last=points.at(-1);if(last?.at===point.at)points[points.length-1]=point;else if(!last||point.at>last.at)points.push(point);while(points.length>300||points.length&&points[0].at<point.at-300000)points.shift();}
 function sync(){
  if(!eligible()){stop();return;}
  if(socket||retry)return;
  if(typeof WebSocket!=='function'){liveState('liveUnsupported');return;}
  liveState(failures?'liveReconnecting':'liveConnecting');
  let ws;try{ws=new WebSocket((location.protocol==='https:'?'wss://':'ws://')+location.host+'/api/live');}catch{liveState('liveUnsupported');return;}socket=ws;
  const connectionStarted=Date.now();
  watch=setInterval(()=>{if(Date.now()-Math.max(connectionStarted,lastReceived)>5000){liveState('liveStale');ws.close();}},2000);
  ws.onmessage=event=>{
   if(socket!==ws||!eligible())return;
   let frame;try{frame=JSON.parse(event.data);}catch{return;}
   if(!frame.data||!['snapshot','sample'].includes(frame.type))return;
   if(frame.type==='snapshot'){points.length=0;for(const point of frame.history||[])push(point);}
   push(frame.data.traffic);home={...home,...frame.data};lastReceived=Date.now();failures=0;liveState('liveConnected');render();
  };
  ws.onclose=()=>{
   if(socket!==ws)return;socket=null;clearInterval(watch);watch=null;
   if(!eligible())return;
   liveState(lastReceived?'liveStale':'liveReconnecting');
   retry=setTimeout(async()=>{retry=null;if(!eligible())return;try{await api('session');}catch{if(!session)return;}sync();},Math.min(30000,1000*2**Math.min(failures++,5)));
  };
  ws.onerror=()=>{};
 }
 function rows(id,entries){const dl=$(id);dl.replaceChildren(...entries.map(([key,value,warning])=>{const row=el('div',warning?'has-warning':'');row.append(el('dt','',t(key)),el('dd','',value??'—'));return row;}));}
 function chip(label,good){const node=el('span','health-chip '+(good===true?'is-good':good===false?'is-warning':'is-muted'));node.append(el('span','dot'),el('span','',label));return node;}
 function renderProbes(probes){
  $('probe-cards').replaceChildren(...(probes||[]).map(p=>{
   const card=el('div','probe');const heading=el('div','probe-heading');heading.append(el('span','chip',p.family==='ipv6'?'IPv6':'IPv4'),el('code','probe-target',p.target));
   const status=el('div','probe-status '+(p.status==='ok'?'is-good':p.status==='timeout'?'is-warning':''),t({ok:'probeOK',timeout:'probeTimeout',waiting:'probeWaiting',disabled:'probeDisabled',unavailable:'probeUnavailable'}[p.status]||'unknown'));
   if(p.error){status.title=p.error;card.append(heading,status);}else card.append(heading,status);
   const values=el('div','probe-values');
   for(const [label,value]of [['ping',ms(p.rtt_ms)],['packetLoss',available(p.loss_percent)?number(p.loss_percent)+'%':'—'],['jitter',ms(p.jitter_ms)]]){const box=el('div');box.append(el('span','',t(label)),el('strong','',value));if(label==='jitter')box.title=t('jitterHint');values.append(box);}
   const detail=el('div','probe-detail');detail.append(el('span','',t('probeWindow')+': '+number(p.received)+' / '+number(p.sent)),el('span','',t('lastProbe')+': '+clock(p.last_at)));
   const range=el('div','probe-range',t('probeRange')+': '+[p.minimum_ms,p.average_ms,p.maximum_ms].map(ms).join(' / '));
   card.append(values,detail,range);if(p.error)card.append(el('p','probe-error',p.error));return card;
  }));
 }
 function renderChart(h){
  const scope=$('traffic-scope').value,k=h.core||{};
  const isPackets=scope==='packets',isInterface=scope==='interface';
  const keys=isPackets?['packets_in','packets_out']:isInterface?['rx','tx']:['download','upload'];
  const values=isPackets?[k.packets_in_per_second,k.packets_out_per_second]:isInterface?[h.network_available?h.rx_per_second:null,h.network_available?h.tx_per_second:null]:[k.payload_download_per_second,k.payload_upload_per_second];
  const labels=isPackets?['packetsIn','packetsOut']:['download','upload'];
  const format=isPackets?n=>number(n)+' '+t('packetsSecond'):rate;
  $('network-rates').replaceChildren(...values.map((value,i)=>{const box=el('div','traffic-rate series-'+i),label=el('div','rate-label');label.append(icon(i?'up':'down'),el('span','',t(labels[i])));box.append(label,el('div','rate-value',format(value)));return box;}));
  $('traffic-hint').textContent=t(isPackets?'packetHint':isInterface?'networkHint':'payloadHint');
  $('traffic-scope').setAttribute('aria-label',t('trafficMeasurement'));
  if(!points.length&&h.traffic)push(h.traffic);
  const end=points.at(-1)?.at||Date.now(),start=end-300000;
  const measured=isInterface?h.network_available:!!h.core_enabled;
  const chartPoints=measured?points:[];
  const maximum=Math.max(1,...chartPoints.flatMap(p=>keys.map(key=>available(p[key])?p[key]:0)))*1.15;
  const chart=svgNode('svg',{viewBox:'0 0 600 184',preserveAspectRatio:'none','aria-hidden':'true'});
  const defs=svgNode('defs');
  for(let i=0;i<2;i++){const gradient=svgNode('linearGradient',{id:'live-fill-'+i,x1:0,y1:0,x2:0,y2:1});gradient.append(svgNode('stop',{offset:'0%','stop-color':'var(--chart-'+i+')','stop-opacity':'.22'}),svgNode('stop',{offset:'100%','stop-color':'var(--chart-'+i+')','stop-opacity':'0'}));defs.append(gradient);}
  chart.append(defs);
  for(let i=0;i<5;i++)chart.append(svgNode('line',{x1:0,x2:600,y1:8+i*42,y2:8+i*42,class:'chart-grid'}));
  for(let i=0;i<2;i++){
   let segment=[];const segments=[];
   for(const p of chartPoints){if(!available(p[keys[i]])||p.at<start)continue;if(segment.length&&p.at-segment.at(-1).at>2500){segments.push(segment);segment=[];}segment.push(p);}if(segment.length)segments.push(segment);
   for(const group of segments){const coords=group.map(p=>[(p.at-start)/300000*600,176-168*Math.max(0,p[keys[i]])/maximum]);const path=coords.map(([x,y],j)=>(j?'L':'M')+x.toFixed(2)+' '+y.toFixed(2)).join(' ');const [first,last]=[coords[0],coords.at(-1)];if(coords.length>1)chart.append(svgNode('path',{d:path+' L'+last[0]+' 176 L'+first[0]+' 176 Z',fill:'url(#live-fill-'+i+')'}));chart.append(svgNode('path',{d:path,fill:'none',stroke:'var(--chart-'+i+')','stroke-width':2,'vector-effect':'non-scaling-stroke','stroke-linejoin':'round'}),svgNode('circle',{cx:last[0],cy:last[1],r:2.5,fill:'var(--chart-'+i+')'}));}
  }
  $('sparkline').replaceChildren(chart);$('sparkline').setAttribute('aria-label',t('trafficChart')+': '+values.map((v,i)=>t(labels[i])+' '+format(v)).join(', '));
  $('chart-scale').replaceChildren(...[maximum,maximum/2,0].map(v=>el('span','',!measured?'—':isPackets?formatNumber(v):bytes(v)+'/s')));
 }
 function render(){
  if(!home)return;
  const h=home,k=h.core||{},health=h.health||{},enabled=!!h.core_enabled,disk=h.disk_total?100*(1-h.disk_free/h.disk_total):null;
  $('metrics').replaceChildren(metric('cpu',h.cpu_available?number(h.cpu_percent)+'%':'—',number(h.cpu_cores)+' '+t('cores'),h.cpu_percent),metric('ram',h.ram_total?number(h.ram_percent)+'%':'—',amount(h.ram_used)+' / '+amount(h.ram_total),h.ram_percent),metric('storage',available(disk)?number(disk)+'%':'—',amount(h.disk_free)+' '+t('available'),disk),metric('sessions',enabled?number(k.sessions):'—',t('flowHint'),k.max_sessions?100*k.sessions/k.max_sessions:0));
  $('health-strip').replaceChildren(chip(t(enabled&&k.running?'running':'coreOff'),enabled&&k.running?true:null),chip(t(h.storage_healthy?'storageOK':'storageError'),!!h.storage_healthy),...[[4,'IPv4'],[6,'IPv6']].map(([family,label])=>chip(label+' · '+t(k.icmp_families?.includes(family)?'listener':'notListening'),k.icmp_families?.includes(family)?true:null)),chip(t('activeUsers')+' · '+(enabled?number(k.active_users):'—'),null));
  renderChart(h);renderProbes(h.probes);
  rows('tunnel-info', [['activeFlows',enabled?number(k.sessions)+' / '+number(k.max_sessions):'—'],['TCP',number(k.tcp_sessions)],['UDP',number(k.udp_sessions)],['packetsIn',number(k.packets_in)],['packetsOut',number(k.packets_out)],['TCP ↑ / ↓',number(k.tcp_packets_in)+' / '+number(k.tcp_packets_out)],['UDP ↑ / ↓',number(k.udp_packets_in)+' / '+number(k.udp_packets_out)],['activeReader',k.packet_reader||'—']]);
  rows('error-info',[['dropped','dropped'],['rejected','rejected'],['outboundErrors','outbound_errors'],['dnsErrors','outbound_dns_errors'],['policyErrors','outbound_policy_errors'],['timeouts','outbound_timeouts'],['networkErrors','outbound_network_errors'],['otherErrors','outbound_other_errors'],['socketErrors','socket_write_errors'],['oversizedUDP','oversized_udp']].map(([label,key])=>[label,number(k[key])]));
  $('queue-info').replaceChildren(...[['controlQueue','control_queue'],['tcpQueue','tcp_queue'],['udpQueue','udp_queue']].map(([label,key])=>{const node=el('div','queue-meter'),heading=el('div');heading.append(el('span','',t(label)),el('strong','',number(k[key])+' / '+number(k[key+'_capacity'])));node.append(heading,progress(k[key+'_capacity']?100*k[key]/k[key+'_capacity']:0));return node;}));
  rows('pacing-info',[
   ['pacing',available(k.icmp_packets_per_second)?number(k.icmp_packets_per_second)+' '+t('packetsSecond'):'—'],['burst',number(k.icmp_burst_packets)],['udpLimit',enabled?'8 KiB':'—'],
   ['busiestTCP',number(k.busiest_user_tcp_sessions)+' / '+number(k.max_sessions_per_user),available(k.busiest_user_tcp_sessions)&&k.busiest_user_tcp_sessions>=k.max_sessions_per_user],
   ['busiestUDP',number(k.busiest_user_udp_sessions)+' / '+number(k.max_udp_sessions_per_user),available(k.busiest_user_udp_sessions)&&k.busiest_user_udp_sessions>=k.max_udp_sessions_per_user],
   ['queueDrops',number(k.write_queue_dropped)],['sessionDrops',number(k.session_queue_dropped)],['sessionLimits',number(k.session_limit_rejected)]
  ]);
  rows('pressure-info',[
   ['tcpQueueRetries',number(k.tcp_queue_retries)],['controlQueueDrops',number(k.control_queue_dropped)],['udpQueueEvicted',number(k.udp_queue_evicted)],['udpQueueRejected',number(k.udp_queue_rejected)],
   ['globalFlowRejects',number(k.session_limit_global)],['tcpUserRejects',number(k.session_limit_tcp_user)],['udpUserRejects',number(k.session_limit_udp_user)],['stoppingRejects',number(k.session_limit_stopping)],
   ['udpIdleTimeouts',number(k.udp_idle_timeouts)],['udpReadErrors',number(k.udp_read_errors)],['udpWriteErrors',number(k.udp_write_errors)]
  ]);
  rows('system-info',[['hostname',h.hostname],['os',h.os],['kernel',h.kernel],['architecture',h.architecture],['uptime',number(Math.floor(h.uptime_seconds/86400))+' '+t('days')+' · '+Math.floor(h.uptime_seconds/3600)%24+'h'],['processMemory',amount(h.process_rss)],['heapMemory',amount(h.process_memory)],['swap',amount(h.swap_used)+' / '+amount(h.swap_total)],['goroutines',number(h.goroutines)],['fileDescriptors',number(health.fd_used)+' / '+number(health.fd_limit)]]);
  rows('interface-info',[['rxTotal',h.network_available?amount(h.network_rx):'—'],['txTotal',h.network_available?amount(h.network_tx):'—'],...['rxErrors','txErrors','rxDrops','txDrops'].map((label,i)=>[label,h.network_available?number(h[['interface_rx_errors','interface_tx_errors','interface_rx_drops','interface_tx_drops'][i]]):'—'])]);
  if(document.querySelector('.kernel-card').open){for(const [id,value]of [['kernel-ipv4',health.icmp4],['kernel-ipv6',health.icmp6],['kernel-settings',health.settings]])rows(id,value?Object.entries(value).sort(([a],[b])=>a.localeCompare(b)).map(([key,v])=>[key,number(v)]):[['unknown','—']]);}
  $('core-state').textContent=t(enabled?(h.storage_healthy?'running':'unhealthy'):'preview');$('core-dot').style.background=enabled?(h.storage_healthy?'var(--green)':'var(--red)'):'var(--muted)';
  $('live-state-label').textContent=t(state);$('last-updated').textContent=t('refreshAt')+' '+clock(h.sampled_at_ms);
 }
 $('traffic-scope').addEventListener('change',render);
 document.querySelector('.kernel-card').addEventListener('toggle',render);
 document.addEventListener('visibilitychange',sync);
 window.addEventListener('pagehide',stop);window.addEventListener('pageshow',sync);
 return {render,sync,reset};
})();
preferences();window.namedDashboard.sync();
