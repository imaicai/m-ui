const $ = (s, r=document) => r.querySelector(s);
const $$ = (s, r=document) => [...r.querySelectorAll(s)];
const PANEL_BASE = (() => { const p=location.pathname||'/'; return p.endsWith('/')?p:(p.slice(0,p.lastIndexOf('/')+1)||'/'); })();
const panelURL = path => PANEL_BASE + String(path||'').replace(/^\/+/, '');

const state = {
  loggedIn: false,
  route: location.hash.replace('#/','') || 'dashboard',
  boot: null,
  publicInfo: null,
  pendingPort: null,
  stats: null,
  netHistory: [],
  modal: null,
  busy: false,
  timer: null,
  clockTimer: null,
  panelUptimeAnchor: null,
  contentOnly: false
};

const icons={
 home:`<svg viewBox="0 0 24 24"><path d="M3 10.5 12 3l9 7.5v9a1.5 1.5 0 0 1-1.5 1.5h-15A1.5 1.5 0 0 1 3 19.5z"/><path d="M9 21v-7h6v7"/></svg>`,
 nodes:`<svg viewBox="0 0 24 24"><ellipse cx="12" cy="5" rx="8" ry="3"/><path d="M4 5v6c0 1.7 3.6 3 8 3s8-1.3 8-3V5M4 11v6c0 1.7 3.6 3 8 3s8-1.3 8-3v-6"/></svg>`,
 link:`<svg viewBox="0 0 24 24"><path d="M10 13a5 5 0 0 0 7.5.5l2-2a5 5 0 0 0-7-7l-1.2 1.2"/><path d="M14 11a5 5 0 0 0-7.5-.5l-2 2a5 5 0 0 0 7 7l1.2-1.2"/></svg>`,
 gear:`<svg viewBox="0 0 24 24"><circle cx="12" cy="12" r="3"/><path d="M19.4 15a1.7 1.7 0 0 0 .34 1.88l.06.06-2.83 2.83-.06-.06A1.7 1.7 0 0 0 15 19.4a1.7 1.7 0 0 0-1 .6 1.7 1.7 0 0 0-.4 1.1V21H9.6v-.1A1.7 1.7 0 0 0 8 19.4a1.7 1.7 0 0 0-1.88.34l-.06.06-2.83-2.83.06-.06A1.7 1.7 0 0 0 3.6 15a1.7 1.7 0 0 0-.6-1 1.7 1.7 0 0 0-1.1-.4H2V9.6h.1A1.7 1.7 0 0 0 3.6 8a1.7 1.7 0 0 0-.34-1.88l-.06-.06 2.83-2.83.06.06A1.7 1.7 0 0 0 8 3.6a1.7 1.7 0 0 0 1-.6 1.7 1.7 0 0 0 .4-1.1V2h4v.1A1.7 1.7 0 0 0 15 3.6a1.7 1.7 0 0 0 1.88-.34l.06-.06 2.83 2.83-.06.06A1.7 1.7 0 0 0 19.4 8c.12.4.34.74.66 1 .3.25.68.4 1.08.4H21v4h-.1A1.7 1.7 0 0 0 19.4 15z"/></svg>`,
 copy:`<svg viewBox="0 0 24 24"><rect x="8" y="8" width="11" height="11" rx="2"/><path d="M16 8V5a2 2 0 0 0-2-2H5a2 2 0 0 0-2 2v9a2 2 0 0 0 2 2h3"/></svg>`,
 edit:`<svg viewBox="0 0 24 24"><path d="M12 20h9"/><path d="M16.5 3.5a2.1 2.1 0 0 1 3 3L8 18l-4 1 1-4z"/></svg>`,
 trash:`<svg viewBox="0 0 24 24"><path d="M3 6h18M8 6V4h8v2M19 6l-1 15H6L5 6M10 10v7M14 10v7"/></svg>`,
 power:`<svg viewBox="0 0 24 24"><path d="M12 2v10"/><path d="M6.2 5.8a8 8 0 1 0 11.6 0"/></svg>`,
 user:`<svg viewBox="0 0 24 24"><circle cx="12" cy="8" r="4"/><path d="M4.5 21a7.5 7.5 0 0 1 15 0"/></svg>`,
 stop:`<svg viewBox="0 0 24 24"><circle cx="12" cy="12" r="8.5"/><rect x="9" y="9" width="6" height="6" rx="1"/></svg>`,
 play:`<svg viewBox="0 0 24 24"><circle cx="12" cy="12" r="8.5"/><path d="m10 8.8 5 3.2-5 3.2z"/></svg>`,
 plus:`<svg viewBox="0 0 24 24"><path d="M12 5v14M5 12h14"/></svg>`,
 github:`<svg viewBox="0 0 24 24"><path d="M15 22v-4a4.8 4.8 0 0 0-1-3.5c3.3-.4 6.8-1.6 6.8-7A5.4 5.4 0 0 0 19.4 4 5 5 0 0 0 19.3.5S18.2.1 15 2a13.4 13.4 0 0 0-7 0C4.8.1 3.7.5 3.7.5A5 5 0 0 0 3.6 4a5.4 5.4 0 0 0-1.4 3.7c0 5.4 3.5 6.6 6.8 7A4.8 4.8 0 0 0 8 18v4"/><path d="M8 19c-3 .9-3-1.5-4.2-2"/></svg>`,
 restart:`<svg viewBox="0 0 24 24"><path d="M20 11a8 8 0 1 0-2.3 5.7"/><path d="M20 4v7h-7"/></svg>`,
 trafficReset:`<svg viewBox="0 0 24 24"><path d="M4 7h16"/><path d="M9 7V4h6v3"/><path d="M7 7l1 13h8l1-13"/><path d="M10 11v5M14 11v5"/></svg>`,
 clock:`<svg viewBox="0 0 24 24"><circle cx="12" cy="12" r="9"/><path d="M12 7v5l3 2"/></svg>`,
 pulse:`<svg viewBox="0 0 24 24"><path d="M3 12h4l2-6 4 12 2-6h6"/></svg>`,
 serverStack:`<svg viewBox="0 0 24 24"><rect x="3" y="4" width="18" height="6" rx="2"/><rect x="3" y="14" width="18" height="6" rx="2"/><path d="M7 7h.01M7 17h.01M11 7h6M11 17h6"/></svg>`,
 shieldCheck:`<svg viewBox="0 0 24 24"><path d="M12 3l7 3v5c0 4.6-2.8 8-7 10-4.2-2-7-5.4-7-10V6z"/><path d="m9 12 2 2 4-4"/></svg>`,
 menu:`<svg viewBox="0 0 24 24"><path d="M4 7h16M4 12h16M4 17h16"/></svg>`,
 network:`<svg viewBox="0 0 24 24"><circle cx="12" cy="12" r="8"/><path d="M4 12h16M12 4c2.2 2.2 3.5 5 3.5 8S14.2 17.8 12 20c-2.2-2.2-3.5-5-3.5-8S9.8 6.2 12 4"/><path d="m17.5 7.5 2-2 2 2M19.5 5.5v4M6.5 16.5l-2 2-2-2M4.5 18.5v-4"/></svg>`
};
function icon(name){return `<span class="icon">${icons[name]}</span>`}

function cookieValue(name){
  const prefix=name+'=';
  for(const part of document.cookie.split(';')){
    const v=part.trim();
    if(v.startsWith(prefix))return decodeURIComponent(v.slice(prefix.length));
  }
  return '';
}
async function api(path, options={}){
  const method=String(options.method||'GET').toUpperCase();
  const mutating=['POST','PUT','PATCH','DELETE'].includes(method);
  const csrf=mutating?cookieValue('mui_csrf'):'';
  const opts={credentials:'same-origin',...options,headers:{...(options.body?{'Content-Type':'application/json'}:{}),...(csrf?{'X-MUI-CSRF':csrf}:{}),...(options.headers||{})}};
  const res=await fetch(panelURL(path),opts);
  let data=null; try{data=await res.json()}catch{}
  if(res.status===401){state.loggedIn=false; state.boot=null; stopTimers(); render(); throw new Error(data?.error||'请先登录')}
  if(!res.ok) throw new Error(data?.error||`请求失败 (${res.status})`);
  return data;
}

function toast(msg, ok=true){const el=document.createElement('div');el.className='toast '+(ok?'success':'');el.textContent=msg;document.body.appendChild(el);setTimeout(()=>el.remove(),2600)}
function esc(v=''){return String(v).replace(/[&<>'"]/g,c=>({'&':'&amp;','<':'&lt;','>':'&gt;',"'":'&#39;','"':'&quot;'}[c]))}
function fmtBytes(v=0){if(!Number.isFinite(v))return '0 B';const u=['B','KB','MB','GB','TB'];let i=0;while(v>=1024&&i<u.length-1){v/=1024;i++}return `${v.toFixed(i?1:0)} ${u[i]}`}
function fmtRate(v=0){return `${fmtBytes(v)}/s`}
function fmtTrafficBytes(v=0){v=Number(v)||0;const u=['B','KB','MB','GB','TB'];let i=0;while(v>=1024&&i<u.length-1){v/=1024;i++}const digits=v>=100||i===0?0:v>=10?1:2;return `${v.toFixed(digits).replace(/\.0+$|(?<=\.[0-9])0+$/,'')}${u[i]}`}
function fmtUptime(s=0){const d=Math.floor(s/86400),h=Math.floor((s%86400)/3600),m=Math.floor((s%3600)/60);return d?`${d} 天 ${h} 小时`:h?`${h} 小时 ${m} 分钟`:`${m} 分钟`}
function fmtPanelUptime(s=0){s=Math.max(0,Math.floor(Number(s)||0));const d=Math.floor(s/86400),h=Math.floor((s%86400)/3600),m=Math.floor((s%3600)/60),sec=s%60;if(d)return `${d}天 ${h}小时 ${m}分`;if(h)return `${h}小时 ${m}分 ${sec}秒`;if(m)return `${m}分 ${sec}秒`;return `${sec}秒`}
function fmtStableClock(s=0){s=Math.max(0,Math.floor(Number(s)||0));const h=Math.floor(s/3600),m=Math.floor((s%3600)/60),sec=s%60;return [h,m,sec].map(v=>String(v).padStart(2,'0')).join(':')}
function currentPanelUptimeSeconds(){const a=state.panelUptimeAnchor;if(!a)return Number(state.stats?.panel_uptime_seconds||0);return a.seconds+Math.max(0,Math.floor((Date.now()-a.at)/1000))}
function updateStableClock(){const el=$('#stableUptime');if(el)el.textContent=fmtStableClock(currentPanelUptimeSeconds())}
function stopTimers(){clearInterval(state.timer);clearInterval(state.clockTimer);state.timer=null;state.clockTimer=null}
function routeTitle(r){return {dashboard:['仪表盘','服务器状态与资源'],nodes:['节点管理','创建、编辑与维护节点'],subscriptions:['订阅管理','管理订阅入口'],settings:['系统设置','面板、安全与证书']}[r]||['仪表盘','']}
function protocolLabel(n){return n?.security==='reality'?'VLESS · REALITY · Vision':'VLESS · TLS · Vision'}
function countryFlag(code){
  code=String(code||'').trim().toUpperCase();
  if(!/^[A-Z]{2}$/.test(code))return '🌐';
  return [...code].map(c=>String.fromCodePoint(127397+c.charCodeAt(0))).join('')
}
function nodeNameHTML(n){
  const code=String(n?.country||'').trim().toUpperCase(), flag=countryFlag(code);
  const title=code?('IP 国家：'+code):'IP 国家：未识别';
  return `<span class="node-name"><span class="node-flag" title="${esc(title)}" aria-label="${esc(title)}">${flag}</span><b>${esc(n?.name||'')}</b></span>`
}

function brandName(){return state.boot?.settings?.panel_name||state.publicInfo?.name||'m-ui'}
function brandIcon(kind='sidebar'){return `<span class="brand-logo-slot brand-logo-${kind}"><img src="${panelURL('assets/app-icon.png?v=1.5.11')}" width="256" height="256" alt="${esc(brandName())}" class="brand-image" decoding="async"></span>`}

function nav(){const ver=esc(state.boot?.version||state.publicInfo?.version||'1.0.0');return `<aside class="sidebar"><div class="brand">${brandIcon('sidebar')}<div><strong class="brand-title">${esc(brandName())}</strong><small>最简单的代理面板</small></div></div><nav class="nav">
${[['dashboard','home','仪表盘'],['nodes','nodes','节点管理'],['subscriptions','link','订阅管理'],['settings','gear','系统设置']].map(([r,i,t])=>`<button data-route="${r}" class="${state.route===r?'active':''}">${icon(i)}<span>${t}</span></button>`).join('')}
</nav><div class="sidebar-bottom"><div class="system-pill"><div class="statusline stable-runtime">${icon('clock')}<span>面板累计运行</span><b id="stableUptime">${fmtStableClock(currentPanelUptimeSeconds())}</b></div><div class="service-copy"><div class="service-project"><a class="repo-icon-link" href="https://github.com/imaicai/m-ui" target="_blank" rel="noopener noreferrer" title="打开 M-Ui GitHub 源码" aria-label="打开 M-Ui GitHub 源码">${icon('github')}</a><span>M-Ui：最简单的代理面板</span></div><div class="service-project"><a class="repo-icon-link" href="https://github.com/imaicai/MiniJet" target="_blank" rel="noopener noreferrer" title="打开 MiniJet GitHub 源码" aria-label="打开 MiniJet GitHub 源码">${icon('github')}</a><span>小飞机：最简单的代理客户端</span></div><div class="service-copyright">© 2026 <a href="https://github.com/imaicai/m-ui" target="_blank" rel="noopener noreferrer">M-Ui</a> v${ver}</div></div></div><button class="sidebar-restart" id="restartPanelSidebar" type="button"><span class="restart-mark">${icon('restart')}</span><span>重启面板</span></button></div></aside>`}
function mobileHeader(){
  const items=[['dashboard','home','仪表盘'],['nodes','nodes','节点管理'],['subscriptions','link','订阅管理'],['settings','gear','系统设置']];
  return `<div class="mobile-header-brand">${brandIcon('mobile')}<strong class="mobile-brand-title">${esc(brandName())}</strong></div><div class="mobile-menu-wrap"><button class="mobile-restart-btn" id="mobileRestart" type="button" title="重启面板" aria-label="重启面板">${icon('restart')}</button><button class="mobile-menu-toggle" id="mobileMenuToggle" type="button" aria-label="打开导航菜单" aria-expanded="false">${icon('menu')}</button><div class="mobile-nav-dropdown" id="mobileNavDropdown" hidden>${items.map(([r,i,t])=>`<button data-route="${r}" class="${state.route===r?'active':''}">${icon(i)}<span>${t}</span></button>`).join('')}<div class="mobile-nav-separator"></div><button class="mobile-nav-logout" id="mobileLogout" type="button">${icon('power')}<span>退出登录</span></button></div></div>`;
}
function layout(content){
  if(state.contentOnly)return content;
  const [title,sub]=routeTitle(state.route), user=state.boot?.settings?.admin_user||'admin';
  return `<div class="shell">${nav()}<main class="main"><header class="topbar">${mobileHeader()}<div class="page-heading-inline"><h1>${title}</h1><span>${sub}</span></div><div class="userbox-inline"><button class="admin-account" data-route="settings" type="button" title="管理员账号 · 系统设置" aria-label="管理员账号，打开系统设置">${icon('user')}<b>${esc(user)}</b></button><span class="sep">·</span><button class="icon-btn subtle" id="logout" title="退出" aria-label="退出">${icon('power')}</button></div></header><div class="content">${content}</div></main></div>`;
}

function login(){return `<div class="login-shell"><form class="login-card refined" id="loginForm"><div class="login-card-head">${brandIcon('login')}<h1>${esc(brandName())}</h1><p>最简单的代理面板</p></div><div class="field"><label>用户名</label><input class="input" name="user" autocomplete="username" autofocus></div><div class="field"><label>密码</label><input class="input" name="pass" type="password" autocomplete="current-password"></div><button class="btn btn-primary btn-lg" type="submit">登录面板</button><div class="security-note">仅限管理员访问</div></form></div>`}

function stat(label,value,sub,cls,symbol){return `<div class="card stat"><div><div class="label">${label}</div><div class="value">${value}</div><div class="sub">${sub}</div></div><div class="badgeicon ${cls}">${symbol}</div></div>`}
function fmtPercent(pct){pct=Math.max(0,Math.min(100,Number(pct)||0));return (pct<10?pct.toFixed(1):pct.toFixed(0))+'%'}
function ring(label,pct,sub,color='var(--blue)',display=''){pct=Math.max(0,Math.min(100,Number(pct)||0));return `<div class="resource"><b>${label}</b><div class="ring" style="--p:${pct.toFixed(1)};--c:${color}"><b>${display||fmtPercent(pct)}</b></div><small>${sub}</small></div>`}
function chart(){const hist=state.netHistory.slice(-30);if(hist.length<2)return `<div class="chart-empty">等待网络采样…</div>`;const max=Math.max(1,...hist.flatMap(x=>[x.rx,x.tx]));const pts=(key)=>hist.map((x,i)=>`${(i/(hist.length-1))*700},${220-(x[key]/max)*180}`).join(' ');return `<div class="chart"><svg viewBox="0 0 700 240" preserveAspectRatio="none"><polyline points="${pts('rx')}" fill="none" stroke="#00B176" stroke-width="4"/><polyline points="${pts('tx')}" fill="none" stroke="#67CFA9" stroke-width="3"/></svg><div class="chart-legend"><span><i class="legend blue"></i>下载 ${fmtRate(hist.at(-1).rx)}</span><span><i class="legend green"></i>上传 ${fmtRate(hist.at(-1).tx)}</span></div></div>`}


function dashboardProxyTraffic(nodes,st={}){
  let uplink=0,downlink=0;
  if(st.proxy_uplink_bytes!=null||st.proxy_downlink_bytes!=null){
    uplink=Number(st.proxy_uplink_bytes||0);
    downlink=Number(st.proxy_downlink_bytes||0);
  }else{
    for(const n of nodes){
      uplink+=Number(n.uplink_bytes||0);
      downlink+=Number(n.downlink_bytes||0);
    }
  }
  return {uplink,downlink,total:uplink+downlink};
}
function dashboardHealth(st){
  const cpu=Number(st.cpu_percent||0), memory=Number(st.memory_percent||0), disk=Number(st.disk_percent||0);
  const issues=[];
  if(cpu>=90)issues.push(`CPU 使用率较高（${Math.round(cpu)}%）`);
  if(memory>=90)issues.push(`内存使用率较高（${Math.round(memory)}%）`);
  if(disk>=85)issues.push(`磁盘使用率较高（${Math.round(disk)}%）`);
  const swap=Number(st.swap_percent||0), swapTotal=Number(st.swap_total||0);
  if(swapTotal>0&&swap>=80)issues.push(`SWAP 使用率较高（${Math.round(swap)}%）`);
  return {
    label:issues.length?'需要关注':'运行正常',
    sub:`CPU ${fmtPercent(cpu)} · 内存 ${fmtPercent(memory)} · 磁盘 ${fmtPercent(disk)}`,
    cls:issues.length?'i-orange':'i-green',
    issues
  };
}
function dashboardAlerts(st,nodes,health){
  const alerts=[];
  const enabled=nodes.filter(n=>n.enabled).length;
  if(!st.xray_running)alerts.push('Xray 服务未运行');
  if(nodes.length===0)alerts.push('尚未创建代理节点');
  else if(enabled===0)alerts.push('当前没有启用节点');
  alerts.push(...health.issues);
  if(!alerts.length)return '';
  return `<div class="dashboard-alert" role="status"><div class="dashboard-alert-icon">!</div><div><b>需要注意</b><span>${alerts.map(esc).join(' · ')}</span></div></div>`;
}
function dashboardNodeRows(nodes){
  const ordered=[...nodes].sort((a,b)=>{
    if(Boolean(a.enabled)!==Boolean(b.enabled))return a.enabled?-1:1;
    const at=Number(a.uplink_bytes||0)+Number(a.downlink_bytes||0);
    const bt=Number(b.uplink_bytes||0)+Number(b.downlink_bytes||0);
    if(bt!==at)return bt-at;
    return String(a.name||'').localeCompare(String(b.name||''),'zh-CN');
  });
  return ordered.slice(0,5).map(n=>{
    const up=Number(n.uplink_bytes||0),down=Number(n.downlink_bytes||0),total=up+down;
    return `<tr><td><div class="dashboard-node-main">${nodeNameHTML(n)}<small>${n.security==='reality'?'REALITY · Vision':'TLS · Vision'}</small></div></td><td>${n.enabled?'<span class="dot"></span> <span class="online">启用</span>':'<span class="muted">停用</span>'}</td><td><div class="dashboard-traffic-cell"><b>${fmtBytes(total)}</b><small>↓${fmtBytes(down)} · ↑${fmtBytes(up)}</small></div></td><td>${n.port}</td><td><button class="icon-btn copy-uri" data-uri="${esc(n.uri)}" title="复制节点链接" aria-label="复制节点链接">${icon('copy')}</button></td></tr>`;
  }).join('')||`<tr><td colspan="5" class="empty">还没有节点，请到“节点管理”创建节点。</td></tr>`;
}
function dashboard(){
  const st=state.stats||{},nodes=state.boot?.nodes||[];
  const enabled=nodes.filter(n=>n.enabled).length,disabled=nodes.length-enabled;
  const traffic=dashboardProxyTraffic(nodes,st),health=dashboardHealth(st);
  const serviceValue=st.xray_running?'运行正常':'服务异常';
  const serviceSub=st.xray_running?`Xray 正常 · 服务器运行 ${fmtUptime(st.uptime_seconds)}`:'请检查 Xray 服务';
  const nodeValue=nodes.length?`${nodes.length} 个节点`:'暂无节点';
  const nodeSub=`${enabled} 启用 · ${disabled} 停用`;
  const trafficSub=`节点累计 · ↓${fmtBytes(traffic.downlink)} · ↑${fmtBytes(traffic.uplink)}`;
  return layout(`<div class="grid stats">${stat('代理服务',serviceValue,serviceSub,st.xray_running?'i-green':'i-orange',icon('pulse'))}${stat('节点状态',nodeValue,nodeSub,'i-blue',icon('serverStack'))}${stat('代理流量',fmtBytes(traffic.total),trafficSub,'i-green',icon('network'))}${stat('服务器状态',health.label,health.sub,health.cls,icon('shieldCheck'))}</div>
${dashboardAlerts(st,nodes,health)}
<section class="card section-card dashboard-resources"><div class="section-title"><h2>服务器资源</h2><small>实时数据</small></div><div class="resources">${ring('CPU',st.cpu_percent,'系统处理器','hsl(216 82% 62%)')}${ring('磁盘',st.disk_percent,`${fmtBytes(st.disk_used)} / ${fmtBytes(st.disk_total)}`,'hsl(32 82% 62%)')}${ring('内存',st.memory_percent,`${fmtBytes(st.memory_used)} / ${fmtBytes(st.memory_total)}`,'hsl(258 72% 62%)')}${ring('SWAP',st.swap_percent,Number(st.swap_total||0)>0?`${fmtBytes(st.swap_used)} / ${fmtBytes(st.swap_total)}`:'未启用','hsl(335 62% 62%)',Number(st.swap_total||0)>0?'':'未启用')}</div></section>
<div class="grid dashboard-bottom"><section class="card section-card"><div class="section-title"><h2>服务器实时网络</h2><small>服务器网卡 · 最近 30 次采样</small></div>${chart()}</section><section class="card section-card dashboard-node-overview"><div class="section-title"><h2>节点概览</h2><button class="link-btn" data-route="nodes">查看全部 →</button></div><div class="table-wrap"><table class="table dashboard-node-table"><thead><tr><th>节点</th><th>状态</th><th>流量</th><th>端口</th><th>操作</th></tr></thead><tbody>${dashboardNodeRows(nodes)}</tbody></table></div></section></div>`);
}
function nodeModal(){return `<div class="modal-backdrop"><div class="modal"><div class="modal-head"><div class="modal-title"><div class="badgeicon i-blue">◆</div><div><h2>一键添加节点</h2><p>系统自动完成国家识别和推荐配置</p></div></div><button class="close" id="closeModal">×</button></div><div class="form-grid"><label>节点名称</label><input class="input readonly" value="自动生成：国家代码 + 序号（如 US01）" readonly><div class="helper">同一服务器继续添加时自动递增为 US02、US03…</div><label>推荐配置</label><input class="input readonly" value="VLESS + RAW + REALITY + XTLS Vision" readonly><label>服务器地址</label><input class="input readonly" value="自动识别当前服务器公网 IP" readonly><label>监听端口</label><input class="input readonly" value="自动选择可用端口" readonly><label>UUID</label><input class="input readonly" value="自动生成" readonly><label>REALITY</label><input class="input readonly" value="密钥、Short ID、SNI 自动生成/配置" readonly></div><div class="tip"><b>新建节点不再要求先设置域名或 TLS 证书。</b><br><small>现有 TLS 节点仍会继续保留并使用系统设置中的域名和证书，不会被强制转换。</small></div><div class="modal-actions"><button class="btn btn-outline" id="cancelModal">取消</button><button class="btn btn-primary" id="createNode">立即创建</button></div></div></div>`}
function editNodeModal(n){const reality=n.security==='reality';return `<div class="modal-backdrop" id="editNodeBackdrop"><div class="modal"><div class="modal-head"><div class="modal-title"><div class="badgeicon i-blue">✎</div><div><h2>编辑节点</h2><p>只开放日常确实需要调整的项目</p></div></div><button class="close" id="closeEditNode">×</button></div><div class="form-grid"><label>节点名称</label><input class="input" id="editNodeName" value="${esc(n.name)}"><label>连接地址</label><input class="input" id="editNodeAddress" value="${esc(n.address||'')}" placeholder="vpn.example.com 或服务器公网 IP"><div class="helper">填写域名后，分享链接和订阅都会使用该域名。</div><label>监听端口</label><input class="input" id="editNodePort" type="number" min="1024" max="65535" value="${n.port}">${reality?`<label>REALITY 伪装域名</label><input class="input" id="editRealityServerName" value="${esc(n.reality_server_name||'www.apple.com')}" placeholder="www.apple.com"><div class="helper">修改后服务端目标会自动同步为该域名的 443 端口。</div>`:''}<label>协议配置</label><input class="input readonly" value="${reality?'VLESS + RAW + REALITY + XTLS Vision':'VLESS + RAW + TLS + XTLS Vision'}" readonly></div><div class="tip"><b>UUID、REALITY 密钥、Short ID、协议和 Vision 流控不会在这里开放修改。</b><br><small>这些参数没有日常修改必要，误改反而会让已有客户端立即失效。</small></div><div class="modal-actions"><button class="btn btn-outline" id="cancelEditNode">取消</button><button class="btn btn-primary" id="saveEditNode">保存并重载</button></div></div></div>`}

function openEditNodeModal(id){const n=(state.boot?.nodes||[]).find(x=>x.id===id);if(!n){toast('节点不存在',false);return}document.body.insertAdjacentHTML('beforeend',editNodeModal(n));const close=()=>$('#editNodeBackdrop')?.remove();$('#closeEditNode').onclick=close;$('#cancelEditNode').onclick=close;$('#editNodeBackdrop').addEventListener('click',e=>{if(e.target.id==='editNodeBackdrop')close()});$('#saveEditNode').onclick=async()=>{const btn=$('#saveEditNode');const payload={name:$('#editNodeName').value,address:$('#editNodeAddress').value,port:Number($('#editNodePort').value)};if(n.security==='reality')payload.reality_server_name=$('#editRealityServerName').value;btn.disabled=true;btn.textContent='正在校验并保存…';try{state.boot=await api('/api/nodes/'+n.id,{method:'PATCH',body:JSON.stringify(payload)});close();render();toast('节点已更新并重新加载 Xray')}catch(e){btn.disabled=false;btn.textContent='保存并重载';toast(e.message,false)}}}

function nodes(){const rows=(state.boot?.nodes||[]).map(n=>`<tr><td>${nodeNameHTML(n)}</td><td>${protocolLabel(n)}</td><td class="traffic-cell"><span>↑ ${fmtTrafficBytes(n.uplink_bytes)}</span><span>↓ ${fmtTrafficBytes(n.downlink_bytes)}</span></td><td>${esc(n.address||state.boot.settings.domain||'—')}</td><td>${n.port}</td><td>${n.enabled?'<span class="dot"></span> <span class="online">启用</span>':'<span class="muted">停用</span>'}</td><td class="operation-cell"><div class="actions"><button class="icon-btn copy-uri" data-uri="${esc(n.uri)}" title="复制" aria-label="复制">${icon('copy')}</button><button class="icon-btn edit-node" data-id="${n.id}" title="编辑" aria-label="编辑">${icon('edit')}</button><button class="icon-btn reset-node-traffic" data-id="${n.id}" title="清空流量" aria-label="清空流量">${icon('trafficReset')}</button><button class="icon-btn toggle-node ${n.enabled?'stop':''}" data-id="${n.id}" data-enabled="${n.enabled}" title="${n.enabled?'停止':'启用'}" aria-label="${n.enabled?'停止':'启用'}">${icon('power')}</button><button class="icon-btn danger delete-node" data-id="${n.id}" data-name="${esc(n.name)}" title="删除" aria-label="删除">${icon('trash')}</button></div></td></tr>`).join('')||`<tr><td colspan="7" class="empty">暂无节点</td></tr>`;return layout(`<div class="page-stack"><section class="card info-card node-guide"><h3>新手说明</h3><div class="guide-inline"><span>默认 REALITY + Vision</span><span>填写节点域名后优先使用域名</span><span>创建与编辑前自动执行 Xray 校验</span><span>流量为节点累计上传/下载，可单独或全部清空</span></div></section><div class="toolbar node-toolbar"><input class="input search" id="nodeSearch" placeholder="搜索节点"><button class="btn btn-outline clear-all-traffic-btn" id="resetAllTraffic">${icon('trafficReset')}<span>清空全部节点流量</span></button><button class="btn btn-primary new-node-btn" id="addNode">+新建节点</button></div><section class="card table-card node-table-card"><div class="table-wrap"><table class="table"><thead><tr><th>名称</th><th>协议</th><th>流量</th><th>地址</th><th>端口</th><th>状态</th><th>操作</th></tr></thead><tbody id="nodeRows">${rows}</tbody></table></div><div class="table-footer"><span>共 ${state.boot?.nodes?.length||0} 个节点</span><span>REALITY + Vision</span></div></section></div>`)}

function subscriptionURL(s){return location.origin+'/sub/'+encodeURIComponent(s.slug||s.token)}
function randomSubscriptionSlug(){
  const chars='abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789', bytes=new Uint8Array(28);
  if(globalThis.crypto&&crypto.getRandomValues){crypto.getRandomValues(bytes)}else{for(let i=0;i<bytes.length;i++)bytes[i]=Math.floor(Math.random()*256)}
  let out='';for(let i=0;i<bytes.length;i++)out+=chars[bytes[i]%chars.length];return out
}
function parseSubscriptionSlug(value){
  const parsed=new URL(value,location.origin);
  if(parsed.origin!==location.origin)throw new Error('订阅地址需使用当前面板域名');
  const m=parsed.pathname.match(/^\/sub\/([^/]+)\/?$/);
  if(!m)throw new Error('订阅地址必须使用 /sub/自定义后缀');
  return decodeURIComponent(m[1])
}
function subscriptionNodeIDs(s){
  const nodes=state.boot?.nodes||[], known=new Set(nodes.map(n=>n.id));
  if(s?.is_default)return nodes.map(n=>n.id);
  return (Array.isArray(s?.node_ids)?s.node_ids:[]).filter(id=>known.has(id))
}
function subscriptionNodePicker(selected,locked){
  const nodes=state.boot?.nodes||[], picked=new Set(selected||[]);
  if(!nodes.length)return '<div class="sub-node-empty">暂无节点；可以先保存订阅，创建节点后再编辑加入。</div>';
  const tools=locked?'<small>默认订阅自动包含全部节点，后续新建节点也会自动加入</small>':'<div class="sub-node-tools"><button class="mini-link select-all-sub-nodes" type="button">全选</button><button class="mini-link clear-sub-nodes" type="button">清空</button></div>';
  const rows=nodes.map(function(n){
    const checked=picked.has(n.id)?' checked':'', disabled=locked?' disabled':'', status=n.enabled?'启用':'停用';
    const addr=esc(n.address||state.boot?.settings?.domain||'—');
    const code=String(n.country||'').trim().toUpperCase(), flag=countryFlag(code);
    return '<label class="sub-node-option'+(locked?' locked':'')+'"><input class="sub-node-check" type="checkbox" value="'+esc(n.id)+'"'+checked+disabled+'><span class="sub-select-circle" aria-hidden="true"></span><span class="sub-node-copy"><span class="sub-node-title"><span class="sub-node-flag">'+flag+'</span><b>'+esc(n.name)+'</b></span><small>'+esc(protocolLabel(n))+' · '+addr+':'+n.port+'</small></span><em class="'+(n.enabled?'online':'muted')+'">'+status+'</em></label>'
  }).join('');
  return '<div class="sub-node-picker"><div class="sub-node-picker-head"><span>选择节点</span>'+tools+'</div><div class="sub-node-list">'+rows+'</div></div>'
}
function bindSubscriptionNodePicker(root){
  root.querySelector('.select-all-sub-nodes')?.addEventListener('click',()=>root.querySelectorAll('.sub-node-check').forEach(x=>x.checked=true));
  root.querySelector('.clear-sub-nodes')?.addEventListener('click',()=>root.querySelectorAll('.sub-node-check').forEach(x=>x.checked=false))
}
function selectedSubscriptionNodeIDs(root){return [...root.querySelectorAll('.sub-node-check:checked')].map(x=>x.value)}

function newSubscriptionModal(){
  const slug=randomSubscriptionSlug(), u=location.origin+'/sub/'+slug;
  const picked=(state.boot?.nodes||[]).filter(n=>n.enabled).map(n=>n.id);
  const name='订阅 '+((state.boot?.subscriptions?.length||0)+1);
  return '<div class="modal-backdrop" id="newSubBackdrop"><div class="modal subscription-modal"><div class="modal-head"><div class="modal-title"><div class="badgeicon i-blue">'+icon('plus')+'</div><div><h2>新建订阅</h2><p>先编辑订阅地址并选择节点，保存后才会真正创建</p></div></div><button class="close" id="closeNewSub">×</button></div><div class="form-grid"><label>订阅名称</label><input class="input" id="newSubName" value="'+esc(name)+'"><label>订阅地址</label><input class="input" id="newSubURL" value="'+esc(u)+'"><div class="helper">系统已随机生成地址后缀，可直接修改；格式保持为 /sub/自定义后缀</div></div>'+subscriptionNodePicker(picked,false)+'<div class="modal-actions"><button class="btn btn-outline" id="cancelNewSub">取消</button><button class="btn btn-primary" id="saveNewSub">保存并创建</button></div></div></div>'
}
function openNewSubscription(){
  document.body.insertAdjacentHTML('beforeend',newSubscriptionModal());
  const root=$('#newSubBackdrop'), close=()=>root?.remove();
  $('#closeNewSub').onclick=close;$('#cancelNewSub').onclick=close;root.addEventListener('click',e=>{if(e.target===root)close()});bindSubscriptionNodePicker(root);
  $('#saveNewSub').onclick=async()=>{
    const btn=$('#saveNewSub');let slug='';
    try{slug=parseSubscriptionSlug($('#newSubURL').value)}catch(e){toast(e.message||'订阅地址格式不正确',false);return}
    btn.disabled=true;btn.textContent='正在创建…';
    try{
      await api('/api/subscriptions',{method:'POST',body:JSON.stringify({name:$('#newSubName').value,slug,node_ids:selectedSubscriptionNodeIDs(root)})});
      close();await loadBootstrap();render();toast('订阅已创建')
    }catch(e){btn.disabled=false;btn.textContent='保存并创建';toast(e.message,false)}
  }
}

function editSubscriptionModal(s){
  const u=subscriptionURL(s), locked=!!s.is_default;
  const hint=locked?'默认订阅固定排在第一位、不可删除，并自动包含全部节点':'可修改订阅地址，并自由调整该订阅包含的节点';
  return '<div class="modal-backdrop" id="editSubBackdrop"><div class="modal subscription-modal"><div class="modal-head"><div class="modal-title"><div class="badgeicon i-blue">'+icon('edit')+'</div><div><h2>编辑订阅</h2><p>'+hint+'</p></div></div><button class="close" id="closeEditSub">×</button></div><div class="form-grid"><label>订阅名称</label><input class="input" id="editSubName" value="'+esc(s.name)+'"><label>订阅地址</label><input class="input" id="editSubURL" value="'+esc(u)+'"><div class="helper">地址格式保持为 /sub/自定义后缀</div></div>'+subscriptionNodePicker(subscriptionNodeIDs(s),locked)+'<div class="modal-actions"><button class="btn btn-outline" id="cancelEditSub">取消</button><button class="btn btn-primary" id="saveEditSub">保存</button></div></div></div>'
}
function openEditSubscription(id){
  const s=(state.boot?.subscriptions||[]).find(x=>x.id===id);if(!s){toast('订阅不存在',false);return}
  document.body.insertAdjacentHTML('beforeend',editSubscriptionModal(s));
  const root=$('#editSubBackdrop'), close=()=>root?.remove();
  $('#closeEditSub').onclick=close;$('#cancelEditSub').onclick=close;root.addEventListener('click',e=>{if(e.target===root)close()});bindSubscriptionNodePicker(root);
  $('#saveEditSub').onclick=async()=>{
    const btn=$('#saveEditSub');let slug='';
    try{slug=parseSubscriptionSlug($('#editSubURL').value)}catch(e){toast(e.message||'订阅地址格式不正确',false);return}
    const payload={name:$('#editSubName').value,slug};
    if(!s.is_default)payload.node_ids=selectedSubscriptionNodeIDs(root);
    btn.disabled=true;
    try{state.boot=await api('/api/subscriptions/'+s.id,{method:'PATCH',body:JSON.stringify(payload)});close();render();toast('订阅已更新')}catch(e){btn.disabled=false;toast(e.message,false)}
  }
}

function subscriptions(){
  const subs=state.boot?.subscriptions||[];
  const rows=subs.map(s=>{
    const u=subscriptionURL(s), count=subscriptionNodeIDs(s).length, defaultTag=s.is_default?'<span class="default-sub-tag">默认</span>':'';
    const del=s.is_default?'':'<button class="icon-btn danger delete-sub" data-id="'+esc(s.id)+'" data-name="'+esc(s.name)+'" title="删除" aria-label="删除">'+icon('trash')+'</button>';
    const toggle='<button class="icon-btn toggle-sub '+(s.enabled?'stop':'start')+'" data-id="'+esc(s.id)+'" data-enabled="'+s.enabled+'" title="'+(s.enabled?'停止订阅':'启用订阅')+'" aria-label="'+(s.enabled?'停止订阅':'启用订阅')+'">'+icon(s.enabled?'stop':'play')+'</button>';
    return '<tr><td><b>'+esc(s.name)+'</b>'+defaultTag+'</td><td><span class="urlpill">'+esc(u)+'</span></td><td>'+count+'</td><td>'+(s.enabled?'<span class="dot"></span> <span class="online">启用</span>':'<span class="muted">停用</span>')+'</td><td class="operation-cell sub-actions"><div class="actions"><button class="icon-btn copy-sub" data-url="'+esc(u)+'" title="复制" aria-label="复制">'+icon('copy')+'</button>'+toggle+'<button class="icon-btn edit-sub" data-id="'+esc(s.id)+'" title="编辑" aria-label="编辑">'+icon('edit')+'</button>'+del+'</div></td></tr>'
  }).join('');
  return layout('<section class="card info-card subscription-guide"><h3>如何使用订阅地址</h3><div class="guide-inline subscription-steps"><span>默认订阅固定第一并自动包含全部节点</span><span>自定义订阅可单独选择需要分发的节点</span><span>节点变化后订阅内容会自动同步</span></div></section><section class="card subs-table"><div class="section-title"><h2>订阅列表</h2><button class="btn btn-primary" id="newSub">＋ 新建订阅</button></div><div class="table-wrap"><table class="table"><thead><tr><th>名称</th><th>订阅地址</th><th>节点数</th><th>状态</th><th class="operation-head">操作</th></tr></thead><tbody>'+rows+'</tbody></table></div></section>')
}
function settings(){const s=state.boot?.settings||{};return layout(`<section class="card info-card settings-advice"><h3>使用建议</h3><p>新建节点默认使用 <b>VLESS + RAW + REALITY + XTLS Vision</b>。如果填写“节点域名”，新建节点会优先使用该域名作为连接地址；未填写时才使用服务器公网 IP。TLS 证书路径主要用于兼容已有 TLS 节点。建议使用独立根路径保护管理入口。证书有效且匹配节点域名时，HTTP 会自动切换到 HTTPS。</p></section><div class="settings-stack"><section class="card settings-card"><h2>基础设置</h2><p>管理入口与节点域名设置。</p><div class="setting-row"><label>面板名称</label><input class="input" id="setName" value="${esc(s.panel_name||'m-ui')}"><small>显示在页面标题中</small></div><div class="setting-row"><label>面板端口</label><input class="input" id="setPort" type="number" min="1024" max="65535" value="${Number(s.panel_port||location.port||16800)}"><small>修改端口后会自动重启面板，并需要重新登录</small></div><div class="setting-row"><label>面板 URL 根路径</label><input class="input" id="setPath" value="${esc(s.panel_path||'/')}" placeholder="/a8f3k2m9/"><small>修改根路径后直接切换到新入口，不需要重启面板</small></div><div class="setting-row"><label>默认协议</label><input class="input readonly" value="VLESS + RAW + REALITY + XTLS Vision" readonly><small>新建节点固定使用当前推荐配置</small></div><div class="setting-row"><label>节点域名</label><input class="input" id="setDomain" value="${esc(s.domain||'')}" placeholder="vpn.example.com（可留空）"><small>可留空；填写后新建 REALITY 节点的分享地址也会优先使用该域名</small></div></section><section class="card settings-card"><h2>证书路径</h2><p>面板 HTTPS 与 TLS 节点共用证书文件。</p><div class="setting-row"><label>证书文件</label><input class="input" id="setCert" value="${esc(s.cert_file||'~/minijet/ssh/m-ui/fullchain.pem')}"><span class="cert-status">${s.cert_exists?'<span class="dot"></span> 已检测':'<span class="warn-dot"></span> 未找到'}</span></div><div class="setting-row"><label>私钥文件</label><input class="input" id="setKey" value="${esc(s.key_file||'~/minijet/ssh/m-ui/privkey.pem')}"><span class="cert-status">${s.key_exists?'<span class="dot"></span> 已检测':'<span class="warn-dot"></span> 未找到'}</span></div></section><section class="card settings-card"><h2>登录账号</h2><p>修改登录名或密码时必须输入当前密码，保存后需要重新登录。</p><div class="setting-row"><label>登录名</label><input class="input" id="setUser" value="${esc(s.admin_user||'admin')}"><small>仅修改时需要重新登录</small></div><div class="setting-row"><label>当前密码</label><input class="input" id="setCurrentPass" type="password" autocomplete="current-password"><small>仅修改账号信息时需要填写</small></div><div class="setting-row"><label>新密码</label><input class="input" id="setPass" type="password" autocomplete="new-password" placeholder="至少 8 位"><small>不修改密码可留空</small></div><div class="setting-row"><label>确认新密码</label><input class="input" id="setPass2" type="password" autocomplete="new-password"><small>与新密码保持一致</small></div></section><section class="card settings-card maintenance-card"><h2>系统维护</h2><p>用于面板升级与核心维护。</p><div class="maintenance-action"><div><b>完整更新 m-ui</b><small>更新前端、后端、SSH 管理脚本和 Xray Core，并保留现有账号、节点、订阅与设置。</small></div><button class="btn btn-outline" id="fullUpdatePanel" type="button">${icon('restart')}<span>完整更新</span></button></div></section><div class="settings-restart-note">只有修改设置后才可保存。修改面板端口会自动重启并重新登录；修改登录名或密码只需重新登录；其他设置保存后直接生效。</div><div class="savebar"><button class="btn btn-primary" id="saveSettings" disabled>保存设置</button></div></div>`)}
function confirmBox(message, yes){state.modal='confirm';document.body.insertAdjacentHTML('beforeend',`<div class="modal-backdrop" id="confirmBackdrop"><div class="modal" style="max-width:460px"><div class="modal-head"><div><h2 style="margin:0 0 6px">请确认</h2><p style="margin:0;color:var(--muted)">${esc(message)}</p></div><button class="close" id="confirmNo">×</button></div><div class="modal-actions"><button class="btn btn-outline" id="confirmCancel">取消</button><button class="btn btn-danger" id="confirmYes">确认</button></div></div></div>`);const close=()=>$('#confirmBackdrop')?.remove();$('#confirmNo').onclick=close;$('#confirmCancel').onclick=close;$('#confirmBackdrop').addEventListener('click',e=>{if(e.target.id==='confirmBackdrop')close()});$('#confirmYes').onclick=async()=>{close();await yes()}}

async function loadPanelInfo(){try{state.publicInfo=await api('/api/panel-info')}catch{}}
async function loadBootstrap(){state.boot=await api('/api/bootstrap');state.publicInfo={name:state.boot?.settings?.panel_name||'m-ui',version:state.boot?.version||''};document.title=brandName()+' · MiniJet'}
async function loadStats(){try{const st=await api('/api/system/stats');state.stats=st;const traffic=st.node_traffic||{};for(const n of state.boot?.nodes||[]){const t=traffic[n.id];if(!t)continue;n.uplink_bytes=Number(t.uplink_bytes||0);n.downlink_bytes=Number(t.downlink_bytes||0)}state.panelUptimeAnchor={seconds:Number(st.panel_uptime_seconds||0),at:Date.now()};state.netHistory.push({rx:st.rx_rate||0,tx:st.tx_rate||0});if(state.netHistory.length>30)state.netHistory.shift();if(state.route==='dashboard')render();updateStableClock()}catch(e){console.warn(e)}}
function startPolling(){stopTimers();loadStats();state.timer=setInterval(loadStats,5000);state.clockTimer=setInterval(updateStableClock,1000)}

async function restartPanel(){
  if(!confirm('确认重启面板？重启后需要重新登录。'))return;
  const target=new URL(location.href);
  const nextPort=Number(state.pendingPort||state.boot?.settings?.panel_port||location.port||0);
  if(nextPort>0)target.port=String(nextPort);
  const nextPath=state.boot?.settings?.panel_path||PANEL_BASE;
  target.pathname=nextPath;
  target.hash='';
  target.search='';
  toast('正在重启面板…');
  try{await api('/api/system/restart',{method:'POST'})}catch{}
  stopTimers();
  setTimeout(()=>{location.href=target.toString()},4200);
}

function updateProgressScreen(st={progress:2,stage:'启动更新',message:'正在创建独立后台更新任务'}){
  const progress=Math.max(0,Math.min(100,Number(st.progress)||0));
  return `<div class="loading restart-loading update-progress-screen"><div class="update-progress-card"><strong>正在完整更新 m-ui</strong><div class="update-progress-head"><span id="updateProgressStage">${esc(st.stage||'更新中')}</span><b id="updateProgressPercent">${progress}%</b></div><div class="update-progress-track"><i id="updateProgressBar" style="width:${progress}%"></i></div><span class="update-progress-message" id="updateProgressMessage">${esc(st.message||'正在更新，请稍候…')}</span><small>更新过程中面板可能短暂离线；进度会在面板恢复后继续同步。</small><button class="btn btn-outline update-progress-return" id="updateProgressReturn" type="button" hidden>返回登录页</button></div></div>`;
}

function paintUpdateProgress(st){
  const progress=Math.max(0,Math.min(100,Number(st?.progress)||0));
  const stage=$('#updateProgressStage'), percent=$('#updateProgressPercent'), bar=$('#updateProgressBar'), message=$('#updateProgressMessage'), back=$('#updateProgressReturn');
  if(stage)stage.textContent=st?.stage||'更新中';
  if(percent)percent.textContent=progress+'%';
  if(bar)bar.style.width=progress+'%';
  if(message)message.textContent=st?.message||'正在更新，请稍候…';
  if(st?.status==='failed'&&back){
    back.hidden=false;
    back.onclick=()=>{
      sessionStorage.removeItem('mui_full_update_id');
      const target=new URL(location.href);
      target.hash='';
      target.search='';
      location.replace(target.toString());
    };
  }
}

function beginFullUpdateProgress(updateID, initial){
  sessionStorage.setItem('mui_full_update_id',updateID);
  stopTimers();
  state.loggedIn=false;
  state.boot=null;
  document.title='m-ui · 正在完整更新';
  $('#app').innerHTML=updateProgressScreen(initial);
  let stopped=false;
  const target=new URL(location.href);
  target.hash='';
  target.search='';

  const poll=async()=>{
    if(stopped)return;
    try{
      const url=panelURL('/api/system/full-update-status?update_id='+encodeURIComponent(updateID));
      const res=await fetch(url,{cache:'no-store',credentials:'same-origin'});
      if(res.status===404){
        sessionStorage.removeItem('mui_full_update_id');
        location.replace(target.toString());
        return;
      }
      if(res.ok){
        const st=await res.json();
        paintUpdateProgress(st);
        if(st.status==='completed'){
          stopped=true;
          sessionStorage.removeItem('mui_full_update_id');
          document.title='m-ui · 更新完成';
          setTimeout(()=>location.replace(target.toString()),900);
          return;
        }
        if(st.status==='failed'){
          stopped=true;
          document.title='m-ui · 更新失败';
          return;
        }
      }
    }catch{
      const message=$('#updateProgressMessage');
      if(message)message.textContent='面板正在重启或暂时离线，正在等待后台更新状态恢复…';
    }
    setTimeout(poll,1500);
  };
  setTimeout(poll,500);
}

function resumeFullUpdateIfNeeded(){
  const updateID=sessionStorage.getItem('mui_full_update_id');
  if(!updateID)return false;
  beginFullUpdateProgress(updateID,{progress:2,stage:'恢复更新状态',message:'正在读取后台更新进度…'});
  return true;
}

async function fullUpdatePanel(){
  confirmBox('完整更新会更新前端、后端、SSH 管理脚本和 Xray Core，过程中面板会重启。确认现在开始？',async()=>{
    const btn=$('#fullUpdatePanel');
    if(btn){btn.disabled=true;btn.innerHTML=`${icon('restart')}<span>正在启动…</span>`}
    try{
      const out=await api('/api/system/full-update',{method:'POST'});
      const updateID=String(out?.update_id||'');
      if(!updateID)throw new Error('后台未返回更新任务编号');
      beginFullUpdateProgress(updateID,{progress:2,stage:'启动更新',message:out?.message||'完整更新已在后台启动'});
    }catch(e){
      if(btn){btn.disabled=false;btn.innerHTML=`${icon('restart')}<span>完整更新</span>`}
      toast(e.message,false);
    }
  });
}

function syncShellChrome(){
  document.title=brandName()+' · MiniJet';
  const [title,sub]=routeTitle(state.route), user=state.boot?.settings?.admin_user||'admin';
  const h=$('.page-heading-inline h1'), p=$('.page-heading-inline span');
  if(h)h.textContent=title;
  if(p)p.textContent=sub;
  $$('.nav [data-route],.mobile-nav-dropdown [data-route]').forEach(b=>b.classList.toggle('active',b.dataset.route===state.route));
  const brand=$('.brand-title'); if(brand)brand.textContent=brandName();
  const mobileBrand=$('.mobile-brand-title'); if(mobileBrand)mobileBrand.textContent=brandName();
  const account=$('.admin-account b'); if(account)account.textContent=user;
  $$('.brand-logo-sidebar .brand-image,.brand-logo-mobile .brand-image').forEach(image=>image.alt=brandName());
}
function render(){
  document.title=brandName()+' · MiniJet';
  const app=$('#app');
  if(!state.loggedIn){state.contentOnly=false;app.innerHTML=login();bindLogin();return}
  if(!state.boot){state.contentOnly=false;app.innerHTML=`<div class="loading">正在载入 m-ui…</div>`;return}
  const partial=!!app.querySelector('.shell .content');
  state.contentOnly=partial;
  let html=state.route==='nodes'?nodes():state.route==='subscriptions'?subscriptions():state.route==='settings'?settings():dashboard();
  state.contentOnly=false;
  if(partial){
    const content=$('.shell .content');
    if(content)content.innerHTML=html;
    syncShellChrome();
  }else{
    app.innerHTML=html;
  }
  bindCommon();
  if(state.route==='nodes')bindNodes();
  if(state.route==='subscriptions')bindSubs();
  if(state.route==='settings')bindSettings();
}

function bindLogin(){$('#loginForm').addEventListener('submit',async e=>{e.preventDefault();const fd=new FormData(e.target);try{await api('/api/login',{method:'POST',body:JSON.stringify({username:fd.get('user'),password:fd.get('pass')})});state.loggedIn=true;await loadBootstrap();startPolling();render()}catch(err){toast(err.message,false)}})}
function closeMobileMenu(){const menu=$('#mobileNavDropdown'),toggle=$('#mobileMenuToggle');if(menu)menu.hidden=true;if(toggle)toggle.setAttribute('aria-expanded','false')}
function bindCommon(){
  $$('[data-route]').forEach(b=>b.onclick=()=>{closeMobileMenu();state.route=b.dataset.route;location.hash='#/'+state.route;render()});
  const restart=$('#restartPanelSidebar');if(restart)restart.onclick=restartPanel;const mobileRestart=$('#mobileRestart');if(mobileRestart)mobileRestart.onclick=restartPanel;
  const toggle=$('#mobileMenuToggle'),menu=$('#mobileNavDropdown');
  if(toggle&&menu)toggle.onclick=()=>{const opening=menu.hidden;menu.hidden=!opening;toggle.setAttribute('aria-expanded',opening?'true':'false')};
  const logoutAction=async()=>{closeMobileMenu();try{await api('/api/logout',{method:'POST'})}catch{}state.loggedIn=false;state.boot=null;stopTimers();render()};
  const logout=$('#logout');if(logout)logout.onclick=logoutAction;
  const mobileLogout=$('#mobileLogout');if(mobileLogout)mobileLogout.onclick=logoutAction;
  $$('.copy-uri').forEach(b=>b.onclick=()=>copyText(b.dataset.uri,'节点链接已复制'))
}
function openNodeModal(){document.body.insertAdjacentHTML('beforeend',nodeModal());const close=()=>$('.modal-backdrop')?.remove();$('#closeModal').onclick=close;$('#cancelModal').onclick=close;$('.modal-backdrop').addEventListener('click',e=>{if(e.target.classList.contains('modal-backdrop'))close()});$('#createNode').onclick=async()=>{const btn=$('#createNode');btn.disabled=true;btn.textContent='正在识别并创建…';try{const out=await api('/api/nodes/default',{method:'POST'});close();await loadBootstrap();render();toast(out.warning?`${out.node.name} 已创建；${out.warning}`:`${out.node.name} 已创建并生效`)}catch(err){btn.disabled=false;btn.textContent='立即创建';toast(err.message,false)}}}
function bindNodes(){
  $('#addNode')?.addEventListener('click',openNodeModal);
  $('#resetAllTraffic')?.addEventListener('click',()=>confirmBox('确认清空全部节点累计流量？该操作无法恢复。',async()=>{
    try{state.boot=await api('/api/nodes/reset-traffic',{method:'POST'});render();toast('全部节点流量已清空')}catch(e){toast(e.message,false)}
  }));
  $$('.copy-uri').forEach(b=>b.onclick=()=>copyText(b.dataset.uri,'节点链接已复制'));
  $$('.edit-node').forEach(b=>b.onclick=()=>openEditNodeModal(b.dataset.id));
  $$('.reset-node-traffic').forEach(b=>b.onclick=async()=>{
    b.disabled=true;
    try{state.boot=await api('/api/nodes/'+b.dataset.id+'/reset-traffic',{method:'POST'});render();toast('节点流量已清空')}catch(e){b.disabled=false;toast(e.message,false)}
  });
  $$('.toggle-node').forEach(b=>b.onclick=async()=>{b.disabled=true;try{state.boot=await api('/api/nodes/'+b.dataset.id,{method:'PATCH',body:JSON.stringify({enabled:b.dataset.enabled!=='true'})});render();toast(b.dataset.enabled==='true'?'节点已停止':'节点已启用')}catch(e){b.disabled=false;toast(e.message,false)}});
  $$('.delete-node').forEach(b=>b.onclick=()=>confirmBox(`删除节点“${b.dataset.name}”？删除后 Xray 会立即重载。`,async()=>{try{await api('/api/nodes/'+b.dataset.id,{method:'DELETE'});await loadBootstrap();render();toast('节点已删除')}catch(e){toast(e.message,false)}}));
  $('#nodeSearch')?.addEventListener('input',e=>{const q=e.target.value.toLowerCase();$$('#nodeRows tr').forEach(tr=>tr.style.display=tr.textContent.toLowerCase().includes(q)?'':'none')});
}
function bindSubs(){
  $$('.copy-sub').forEach(b=>b.onclick=()=>copyText(b.dataset.url,'订阅链接已复制'));
  $$('.toggle-sub').forEach(b=>b.onclick=async()=>{b.disabled=true;try{state.boot=await api('/api/subscriptions/'+b.dataset.id,{method:'PATCH',body:JSON.stringify({enabled:b.dataset.enabled!=='true'})});render();toast(b.dataset.enabled==='true'?'订阅已停止':'订阅已启用')}catch(e){b.disabled=false;toast(e.message,false)}});
  $$('.edit-sub').forEach(b=>b.onclick=()=>openEditSubscription(b.dataset.id));
  $('#newSub')?.addEventListener('click',openNewSubscription);
  $$('.delete-sub').forEach(b=>b.onclick=()=>confirmBox('删除订阅“'+b.dataset.name+'”？',async()=>{try{await api('/api/subscriptions/'+b.dataset.id,{method:'DELETE'});await loadBootstrap();render();toast('订阅已删除')}catch(e){toast(e.message,false)}}))
}
function bindSettings(){
  const s=state.boot?.settings||{};
  const original={
    panel_name:String(s.panel_name||'m-ui').trim(),
    panel_port:Number(s.panel_port||location.port||16800),
    panel_path:String(s.panel_path||'/').trim(),
    domain:String(s.domain||'').trim(),
    cert_file:String(s.cert_file||'').trim(),
    key_file:String(s.key_file||'').trim(),
    admin_user:String(s.admin_user||'admin').trim()
  };
  const read=()=>({
    panel_name:$('#setName').value.trim(),
    panel_port:Number($('#setPort').value),
    panel_path:$('#setPath').value.trim(),
    domain:$('#setDomain').value.trim(),
    cert_file:$('#setCert').value.trim(),
    key_file:$('#setKey').value.trim(),
    admin_user:$('#setUser').value.trim(),
    current_password:$('#setCurrentPass').value,
    new_password:$('#setPass').value,
    confirm_password:$('#setPass2').value
  });
  const changed=()=>{
    const p=read();
    return p.panel_name!==original.panel_name ||
      p.panel_port!==original.panel_port ||
      p.panel_path!==original.panel_path ||
      p.domain!==original.domain ||
      p.cert_file!==original.cert_file ||
      p.key_file!==original.key_file ||
      p.admin_user!==original.admin_user ||
      p.new_password!=='';
  };
  const save=$('#saveSettings');
  const refreshSaveState=()=>{save.disabled=!changed()||state.busy};
  ['setName','setPort','setPath','setDomain','setCert','setKey','setUser','setCurrentPass','setPass','setPass2'].forEach(id=>$('#'+id)?.addEventListener('input',refreshSaveState));
  refreshSaveState();
  $('#fullUpdatePanel')?.addEventListener('click',fullUpdatePanel);

  save?.addEventListener('click',async()=>{
    if(!changed()||state.busy)return;
    const payload=read();
    state.busy=true;
    save.disabled=true;
    save.textContent='正在保存…';
    try{
      const out=await api('/api/settings',{method:'PUT',body:JSON.stringify(payload)});
      const nextPort=Number(out.settings?.panel_port||location.port||16800);
      const nextPath=out.settings?.panel_path||'/';
      const target=new URL(location.href);
      target.port=String(nextPort);
      target.pathname=nextPath;
      target.hash='#/settings';
      target.search='';
      state.publicInfo={name:out.settings?.panel_name||'m-ui',version:out.version||''};

      if(out.restart_required){
        state.loggedIn=false;
        state.boot=null;
        stopTimers();
        document.title=(state.publicInfo.name||'m-ui')+' · MiniJet';
        $('#app').innerHTML=`<div class="loading restart-loading"><div><strong>设置已保存</strong><span>面板端口已修改，正在重启；稍后请重新登录…</span></div></div>`;
        setTimeout(()=>{location.href=target.toString()},4200);
        return;
      }

      if(out.relogin_required){
        state.loggedIn=false;
        state.boot=null;
        stopTimers();
        document.title=(state.publicInfo.name||'m-ui')+' · MiniJet';
        if(out.panel_path_changed){
          setTimeout(()=>{location.href=target.toString()},300);
        }else{
          render();
          toast('设置已保存，请使用新账号信息重新登录');
        }
        return;
      }

      state.boot=out;
      document.title=brandName()+' · MiniJet';
      if(out.panel_path_changed){
        location.href=target.toString();
        return;
      }
      state.busy=false;
      render();
      toast('设置已保存并生效');
    }catch(e){
      state.busy=false;
      save.disabled=!changed();
      save.textContent='保存设置';
      toast(e.message,false);
    }
  });
}
async function copyText(text,msg){try{await navigator.clipboard.writeText(text);toast(msg)}catch{const ta=document.createElement('textarea');ta.value=text;document.body.appendChild(ta);ta.select();document.execCommand('copy');ta.remove();toast(msg)}}

window.addEventListener('hashchange',()=>{state.route=location.hash.replace('#/','')||'dashboard';if(state.loggedIn)render()});
(async()=>{await loadPanelInfo();if(resumeFullUpdateIfNeeded())return;try{await api('/api/me');state.loggedIn=true;await loadBootstrap();startPolling()}catch{}render()})();
