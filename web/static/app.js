/* 资源中心前端单页应用（原生 JS，无构建依赖） */
"use strict";

/* ---------- SVG 线性图标集（Lucide 风格，stroke 跟随文字色，深色模式自动适配） ---------- */
const ICONS = {
  cloud: '<path d="M17.5 19H9a7 7 0 1 1 6.71-9h1.79a4.5 4.5 0 1 1 0 9Z"/>',
  book: '<path d="M2 3h6a4 4 0 0 1 4 4v14a3 3 0 0 0-3-3H2z"/><path d="M22 3h-6a4 4 0 0 0-4 4v14a3 3 0 0 1 3-3h7z"/>',
  drive: '<line x1="22" x2="2" y1="12" y2="12"/><path d="M5.45 5.11 2 12v6a2 2 0 0 0 2 2h16a2 2 0 0 0 2-2v-6l-3.45-6.89A2 2 0 0 0 16.76 4H7.24a2 2 0 0 0-1.79 1.11z"/><line x1="6" x2="6.01" y1="16" y2="16"/><line x1="10" x2="10.01" y1="16" y2="16"/>',
  mail: '<rect width="20" height="16" x="2" y="4" rx="2"/><path d="m22 7-8.97 5.7a1.94 1.94 0 0 1-2.06 0L2 7"/>',
  activity: '<path d="M22 12h-4l-3 9L9 3l-3 9H2"/>',
  settings: '<path d="M12.22 2h-.44a2 2 0 0 0-2 2v.18a2 2 0 0 1-1 1.73l-.43.25a2 2 0 0 1-2 0l-.15-.08a2 2 0 0 0-2.73.73l-.22.38a2 2 0 0 0 .73 2.73l.15.1a2 2 0 0 1 1 1.72v.51a2 2 0 0 1-1 1.74l-.15.09a2 2 0 0 0-.73 2.73l.22.38a2 2 0 0 0 2.73.73l.15-.08a2 2 0 0 1 2 0l.43.25a2 2 0 0 1 1 1.73V20a2 2 0 0 0 2 2h.44a2 2 0 0 0 2-2v-.18a2 2 0 0 1 1-1.73l.43-.25a2 2 0 0 1 2 0l.15.08a2 2 0 0 0 2.73-.73l.22-.39a2 2 0 0 0-.73-2.73l-.15-.08a2 2 0 0 1-1-1.74v-.5a2 2 0 0 1 1-1.74l.15-.09a2 2 0 0 0 .73-2.73l-.22-.38a2 2 0 0 0-2.73-.73l-.15.08a2 2 0 0 1-2 0l-.43-.25a2 2 0 0 1-1-1.73V4a2 2 0 0 0-2-2z"/><circle cx="12" cy="12" r="3"/>',
  compass: '<circle cx="12" cy="12" r="10"/><polygon points="16.24 7.76 14.12 14.12 7.76 16.24 9.88 9.88 16.24 7.76"/>',
  list: '<rect width="8" height="4" x="8" y="2" rx="1"/><path d="M16 4h2a2 2 0 0 1 2 2v14a2 2 0 0 1-2 2H6a2 2 0 0 1-2-2V6a2 2 0 0 1 2-2h2"/><path d="M12 11h4"/><path d="M12 16h4"/><path d="M8 11h.01"/><path d="M8 16h.01"/>',
  link: '<path d="M10 13a5 5 0 0 0 7.54.54l3-3a5 5 0 0 0-7.07-7.07l-1.72 1.71"/><path d="M14 11a5 5 0 0 0-7.54-.54l-3 3a5 5 0 0 0 7.07 7.07l1.71-1.71"/>',
  upload: '<path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4"/><polyline points="17 8 12 3 7 8"/><line x1="12" x2="12" y1="3" y2="15"/>',
  download: '<path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4"/><polyline points="7 10 12 15 17 10"/><line x1="12" x2="12" y1="15" y2="3"/>',
  search: '<circle cx="11" cy="11" r="8"/><path d="m21 21-4.3-4.3"/>',
  refresh: '<path d="M3 12a9 9 0 0 1 9-9 9.75 9.75 0 0 1 6.74 2.74L21 8"/><path d="M21 3v5h-5"/><path d="M21 12a9 9 0 0 1-9 9 9.75 9.75 0 0 1-6.74-2.74L3 16"/><path d="M8 16H3v5"/>',
  plus: '<path d="M5 12h14"/><path d="M12 5v14"/>',
  back: '<path d="m12 19-7-7 7-7"/><path d="M19 12H5"/>',
  moon: '<path d="M12 3a6 6 0 0 0 9 9 9 9 0 1 1-9-9Z"/>',
  sun: '<circle cx="12" cy="12" r="4"/><path d="M12 2v2"/><path d="M12 20v2"/><path d="m4.93 4.93 1.41 1.41"/><path d="m17.66 17.66 1.41 1.41"/><path d="M2 12h2"/><path d="M20 12h2"/><path d="m6.34 17.66-1.41 1.41"/><path d="m19.07 4.93-1.41 1.41"/>',
  user: '<path d="M19 21v-2a4 4 0 0 0-4-4H9a4 4 0 0 0-4 4v2"/><circle cx="12" cy="7" r="4"/>',
  clock: '<circle cx="12" cy="12" r="10"/><polyline points="12 6 12 12 16 14"/>',
  megaphone: '<path d="m3 11 18-5v12L3 14v-3z"/><path d="M11.6 16.8a3 3 0 1 1-5.8-1.6"/>',
  folder: '<path d="M20 20a2 2 0 0 0 2-2V8a2 2 0 0 0-2-2h-7.9a2 2 0 0 1-1.69-.9L9.6 3.9A2 2 0 0 0 7.93 3H4a2 2 0 0 0-2 2v13a2 2 0 0 0 2 2Z"/>',
  fileText: '<path d="M15 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V7Z"/><path d="M14 2v4a2 2 0 0 0 2 2h4"/><path d="M10 9H8"/><path d="M16 13H8"/><path d="M16 17H8"/>',
  video: '<path d="m22 8-6 4 6 4V8Z"/><rect width="14" height="12" x="2" y="6" rx="2"/>',
  image: '<rect width="18" height="18" x="3" y="3" rx="2"/><circle cx="9" cy="9" r="2"/><path d="m21 15-3.086-3.086a2 2 0 0 0-2.828 0L6 21"/>',
  music: '<path d="M9 18V5l12-2v13"/><circle cx="6" cy="18" r="3"/><circle cx="18" cy="16" r="3"/>',
  archive: '<rect width="20" height="5" x="2" y="3" rx="1"/><path d="M4 8v11a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8"/><path d="M10 12h4"/>',
  disc: '<circle cx="12" cy="12" r="10"/><circle cx="12" cy="12" r="2"/>',
  box: '<path d="M21 8a2 2 0 0 0-1-1.73l-7-4a2 2 0 0 0-2 0l-7 4A2 2 0 0 0 3 8v8a2 2 0 0 0 1 1.73l7 4a2 2 0 0 0 2 0l7-4A2 2 0 0 0 21 16Z"/><path d="m3.3 7 8.7 5 8.7-5"/><path d="M12 22V12"/>',
  paperclip: '<path d="m21.44 11.05-9.19 9.19a6 6 0 0 1-8.49-8.49l8.57-8.57A4 4 0 1 1 18 8.84l-8.59 8.57a2 2 0 0 1-2.83-2.83l8.49-8.48"/>',
  database: '<ellipse cx="12" cy="5" rx="9" ry="3"/><path d="M3 5V19A9 3 0 0 0 21 19V5"/><path d="M3 12A9 3 0 0 0 21 12"/>',
  globe: '<circle cx="12" cy="12" r="10"/><path d="M12 2a14.5 14.5 0 0 0 0 20 14.5 14.5 0 0 0 0-20"/><path d="M2 12h20"/>',
  alert: '<path d="m21.73 18-8-14a2 2 0 0 0-3.48 0l-8 14A2 2 0 0 0 4 21h16a2 2 0 0 0 1.73-3Z"/><path d="M12 9v4"/><path d="M12 17h.01"/>',
  helpCircle: '<circle cx="12" cy="12" r="10"/><path d="M9.09 9a3 3 0 1 1 5.83 1c0 2-3 2-3 4"/><path d="M12 17h.01"/>',
  inbox: '<polyline points="22 12 16 12 14 15 10 15 8 12 2 12"/><path d="M5.45 5.11 2 12v6a2 2 0 0 0 2 2h16a2 2 0 0 0 2-2v-6l-3.45-6.89A2 2 0 0 0 16.76 4H7.24a2 2 0 0 0-1.79 1.11z"/>',
  star: '<polygon points="12 2 15.09 8.26 22 9.27 17 14.14 18.18 21.02 12 17.77 5.82 21.02 7 14.14 2 9.27 8.91 8.26 12 2"/>',
  key: '<path d="m21 2-2 2m-7.61 7.61a5.5 5.5 0 1 1-7.778 7.778 5.5 5.5 0 0 1 7.777-7.777zm0 0L15.5 7.5m0 0 3 3L22 7l-3-3m-3.5 3.5L19 4"/>',
  users: '<path d="M16 21v-2a4 4 0 0 0-4-4H6a4 4 0 0 0-4 4v2"/><circle cx="9" cy="7" r="4"/><path d="M22 21v-2a4 4 0 0 0-3-3.87"/><path d="M16 3.13a4 4 0 0 1 0 7.75"/>',
  radio: '<path d="M4.9 19.1C1 15.2 1 8.8 4.9 4.9"/><path d="M7.8 16.2c-2.3-2.3-2.3-6.1 0-8.5"/><circle cx="12" cy="12" r="2"/><path d="M16.2 7.8c2.3 2.3 2.3 6.1 0 8.5"/><path d="M19.1 4.9C23 8.8 23 15.2 19.1 19.1"/>',
};

function icon(name, size = 16) {
  const path = ICONS[name] || ICONS.box;
  return `<svg class="ic" width="${size}" height="${size}" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">${path}</svg>`;
}

/* ---------- API 封装 ---------- */
async function api(path, opts = {}) {
  const cfg = { headers: { "Content-Type": "application/json" }, ...opts };
  const res = await fetch("/api/v1" + path, cfg);
  if (res.status === 401) { location.href = "/login"; throw new Error("未登录"); }
  const body = await res.json().catch(() => ({ code: 1, msg: "响应解析失败" }));
  if (!res.ok || body.code !== 0) throw new Error(body.msg || `请求失败(${res.status})`);
  return body.data;
}

/* ---------- 工具 ---------- */
const $ = (sel, el = document) => el.querySelector(sel);
const $$ = (sel, el = document) => [...el.querySelectorAll(sel)];
const esc = (s) => String(s ?? "").replace(/[&<>"']/g, (c) => ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;" }[c]));

function fmtSize(n) {
  if (!n && n !== 0) return "-";
  const units = ["B", "KB", "MB", "GB", "TB"];
  let i = 0;
  while (n >= 1024 && i < units.length - 1) { n /= 1024; i++; }
  return n.toFixed(n >= 100 || i === 0 ? 0 : 1) + " " + units[i];
}
function fmtDate(s) {
  if (!s) return "-";
  const d = new Date(s);
  const p = (x) => String(x).padStart(2, "0");
  return `${d.getFullYear()}-${p(d.getMonth() + 1)}-${p(d.getDate())} ${p(d.getHours())}:${p(d.getMinutes())}`;
}
const STATUS_TEXT = { queued: "排队中", running: "转存中", success: "已完成", partial: "部分成功", failed: "失败" };

function toast(msg, isErr = false) {
  let el = $("#toast");
  if (!el) { el = document.createElement("div"); el.id = "toast"; document.body.appendChild(el); }
  el.textContent = msg;
  el.className = isErr ? "err show" : "show";
  clearTimeout(el._t);
  el._t = setTimeout(() => el.classList.remove("show"), 2600);
}

/* ---------- 全局状态 ---------- */
const state = {
  user: null,            // 当前登录用户 {id, username}
  providers: [],
  parseResult: null,   // 当前解析结果
  liveTaskId: null,    // 进行中的任务
  rpSource: "pan1me",   // 资源广场当前论坛源
  rpCat: "全部",         // 分类筛选（游客态默认走搜索）
  rpDefaultQ: "电视剧",  // 游客态默认搜索词（短剧版块无电视剧帖）
  rpPage: 1,
  rpQuery: "",
  rpCache: [],
  libTab: "search",    // 书库当前 tab：search / shelf / history
  settingsTab: "drive", // 设置页当前分类
  libMode: "jump",      // 书库模式：jump / direct
  libBase: "",          // jump 模式的跳转基址
  libQuery: "",
  libPage: 1,
  drivePath: [{ id: 0, name: "我的网盘" }], // 我的网盘面包屑导航栈
  drivePage: 1,          // 我的网盘当前目录页码（每页最多 50 条）
  mailToken: "",        // 邮箱列表分页 token
  mailProv: "gmail",     // 当前邮箱供应商
};

/* ---------- 路由 ---------- */
const routes = {
  dashboard: renderDashboard,
  resources: renderResources,
  library: renderLibrary,
  drive: renderDrive,
  mailbox: renderMailbox,
  monitor: renderMonitor,
  records: renderRecords,
  settings: renderSettings,
  coming: renderComing,
};

function currentRoute() {
  let h = location.hash.replace(/^#\//, "").split("?")[0];
  if (h === "home") { location.replace("#/resources"); h = "resources"; } // 旧工作台入口兼容重定向
  return routes[h] ? h : "dashboard";
}

/* ---------- 模块页子导航（顶栏已收敛为「概览」入口） ---------- */
const SUBNAV_GROUPS = {
  resources: { tabs: [["resources", "compass", "资源广场"], ["records", "list", "转存记录"]] },
  records: { tabs: [["resources", "compass", "资源广场"], ["records", "list", "转存记录"]] },
};

function renderSubnav(route) {
  const box = $("#subnav");
  if (!box) return;
  if (route === "dashboard") { box.hidden = true; return; }
  const group = SUBNAV_GROUPS[route];
  box.hidden = false;
  box.innerHTML = `
    <a class="subnav-back" href="#/dashboard">← 返回概览</a>
    ${group ? `<div class="subnav-tabs">${group.tabs
      .map(([r, ic, label]) => `<a href="#/${r}" class="${r === route ? "active" : ""}">${icon(ic, 14)} ${label}</a>`)
      .join("")}</div>` : ""}`;
}

/* ---------- 功能模块注册表（概览页入口；后期扩展时在此登记新模块） ---------- */
const MODULES = [
  { id: "transfer", icon: "cloud", name: "网盘资源转存", desc: "发现资源并一键转存到云盘，进度在转存记录查看", route: "#/resources", ready: true,
    c1l: "#dbe7ff", c2l: "#eef5ff", c1d: "#1a2647", c2d: "#202f56", ac: "#2e63f6" },
  { id: "library", icon: "book", name: "书库", desc: "搜索电子书，一键转存到 Cloudflare R2 / 本地书仓", route: "#/library", ready: true,
    c1l: "#d9f4e6", c2l: "#eefaf3", c1d: "#0f2e21", c2d: "#153728", ac: "#12a150" },
  { id: "drive", icon: "drive", name: "我的网盘", desc: "浏览网盘文件与目录，支持新建、重命名、移动、删除", route: "#/drive", ready: true,
    c1l: "#fdeecd", c2l: "#fef8ec", c1d: "#33290f", c2d: "#3a3015", ac: "#d97706" },
  { id: "mailbox", icon: "mail", name: "Gmail 邮箱", desc: "收件箱浏览、搜索与附件下载；Outlook 规划中", route: "#/mailbox", ready: true,
    c1l: "#fbdcdc", c2l: "#fef4f4", c1d: "#3a1d1d", c2d: "#42211f", ac: "#e04646" },
  { id: "monitor", icon: "activity", name: "Cloudflare 监控", desc: "账户下 Workers、域名、R2 用量统计与站点访问趋势", route: "#/monitor", ready: true,
    c1l: "#e4dcfd", c2l: "#f3f0ff", c1d: "#251c47", c2d: "#2b2156", ac: "#7c5cff" },
  { id: "settings", icon: "settings", name: "账号设置", desc: "统一配置网盘、论坛、邮箱、Cloudflare 与书库", route: "#/settings", ready: true,
    c1l: "#dfe8f2", c2l: "#f0f5fa", c1d: "#1c2536", c2d: "#232d42", ac: "#5b7a99" },
  { id: "subscribe", icon: "radio", name: "资源订阅", desc: "订阅剧集/资源更新，新内容自动转存", route: "#/coming?m=subscribe", ready: false,
    c1l: "#eceff3", c2l: "#f7f9fb", c1d: "#1a2130", c2d: "#1f2739", ac: "#98a2b3" },
  { id: "share", icon: "link", name: "分享管理", desc: "创建、管理自己发出的分享链接", route: "#/coming?m=share", ready: false,
    c1l: "#eceff3", c2l: "#f7f9fb", c1d: "#1a2130", c2d: "#1f2739", ac: "#98a2b3" },
  { id: "offline", icon: "download", name: "离线下载", desc: "磁力链接 / URL 离线到网盘后转存", route: "#/coming?m=offline", ready: false,
    c1l: "#eceff3", c2l: "#f7f9fb", c1d: "#1a2130", c2d: "#1f2739", ac: "#98a2b3" },
];

function moduleById(id) { return MODULES.find((m) => m.id === id); }

/* ---------- 概览页（登录后默认页，功能入口中心） ---------- */
async function renderDashboard(app) {
  const hour = new Date().getHours();
  const greet = hour < 6 ? "夜深了" : hour < 12 ? "早上好" : hour < 14 ? "中午好" : hour < 18 ? "下午好" : hour < 22 ? "晚上好" : "夜深了";
  const greetSub = hour < 6
    ? "夜深了，忙完记得早点休息"
    : hour < 9
      ? "新的一天开始了，祝你今天一切顺利"
      : hour < 12
        ? "上午好，愿你今天高效又从容"
        : hour < 14
          ? "中午好，忙碌之余记得适当休息"
          : hour < 18
            ? "下午好，愿接下来的安排顺利推进"
            : hour < 22
              ? "晚上好，愿你有一个轻松愉快的夜晚"
              : "夜深了，辛苦一天，记得早点休息";
  const name = state.user?.username || "用户";
  const today = new Date().toLocaleDateString("zh-CN", { month: "long", day: "numeric", weekday: "long" });

  app.innerHTML = `
  <section class="dash-hero">
    <div class="dash-hero-text">
      <h1>${greet}，${esc(name)}</h1>
      <p class="sub">今天是 ${today} · ${greetSub}</p>
    </div>
    <span class="hero-water">${icon("cloud", 110)}</span>
  </section>

  <div class="dash-stats" id="dashStats"></div>

  <div class="card">
    <div class="card-title">${icon("box", 18)} 功能模块</div>
    <div class="dash-modules">
      ${MODULES.map((m) => `
        <a class="module-card ${m.ready ? "" : "pending"}" href="${m.route}"
           style="--c1l:${m.c1l};--c2l:${m.c2l};--c1d:${m.c1d};--c2d:${m.c2d};--ac:${m.ac}">
          <span class="mwater">${icon(m.icon, 88)}</span>
          <span class="micon">${icon(m.icon, 22)}</span>
          <span class="mname">${esc(m.name)}</span>
          <span class="mdesc">${esc(m.desc)}</span>
          <span class="module-health checking" data-module-health="${m.id}" title="正在读取最近一次检测结果">
            <i></i><span>加载中</span>
          </span>
        </a>`).join("")}
    </div>
  </div>

  <div class="dash-columns">
    <div class="card">
      <div class="card-title">${icon("clock", 18)} 最近转存 <a class="sub" href="#/records" style="margin-left:auto">查看全部 →</a></div>
      <div id="dashTasks"></div>
    </div>
    <div class="card">
      <div class="card-title">${icon("megaphone", 18)} 使用指引</div>
      <ol class="guide-list">
        <li>在「资源广场」或外部社区找到资源分享链接</li>
        <li>进入「转存工作台」，粘贴链接并解析文件清单</li>
        <li>勾选需要的文件，选择目标目录后一键转存</li>
        <li>在「转存记录」查看任务状态与历史</li>
        <li>在「账号设置」集中配置各项服务账号与凭据</li>
      </ol>
    </div>
  </div>`;

  loadModuleHealth(app);

  // 统计
  try {
    const st = await api("/stats");
    $("#dashStats", app).innerHTML = [
      [st.total_tasks, "累计任务"],
      [st.success_tasks, "成功转存"],
      [fmtSize(st.total_size), "累计体积"],
      [`${st.success_rate.toFixed(1)}%`, "成功率"],
    ].map(([v, l]) => `
      <div class="dash-stat"><b>${v}</b><span>${l}</span></div>`).join("");
  } catch {
    $("#dashStats", app).innerHTML = `<div class="empty">统计加载失败</div>`;
  }

  // 最近任务
  try {
    const data = await api("/tasks?page=1&page_size=5");
    const box = $("#dashTasks", app);
    if (!data.list?.length) {
      box.innerHTML = `<div class="empty">${icon("inbox", 40)}<br>还没有转存记录<br><a href="#/home" style="font-size:13px">去发起第一次转存 →</a></div>`;
    } else {
      box.innerHTML = data.list.map((t) => `
        <div class="dash-task">
          <div class="dt-main">
            <div class="dt-name">${esc(t.share_name || t.share_url)}</div>
            <div class="dt-meta">${t.file_count || 0} 个文件 · ${fmtSize(t.total_size)} · ${esc(t.target_folder)}</div>
          </div>
          <span style="white-space:nowrap"><span class="status-dot ${t.status}"></span>${STATUS_TEXT[t.status] || t.status}</span>
        </div>`).join("");
    }
  } catch { /* 留空 */ }
}

async function loadModuleHealth(app) {
  try {
    const health = await api("/module-health");
    $$('[data-module-health]', app).forEach((el) => {
      const status = health[el.dataset.moduleHealth] || { available: false, label: "待检测", detail: "等待服务端定时检测" };
      const pending = status.label === "待检测";
      el.className = `module-health ${pending ? "checking" : status.available ? "available" : "unavailable"}`;
      el.querySelector("span").textContent = status.label || (status.available ? "可用" : "异常");
      const checkedAt = status.checked_at ? `\n最近检测：${fmtDate(status.checked_at)}` : "";
      el.title = `${status.detail || status.label || ""}${checkedAt}`;
    });
  } catch (e) {
    $$('[data-module-health]', app).forEach((el) => {
      el.className = "module-health unavailable";
      el.querySelector("span").textContent = "读取失败";
      el.title = e.message;
    });
  }
}

/* ---------- 书库（Z-Library → R2） ---------- */
const LIB_EXT_COLORS = { EPUB: "#16a34a", PDF: "#dc2626", MOBI: "#2563eb", AZW3: "#9333ea" };

async function renderLibrary(app) {
  // 当前书库模式（jump/direct）
  let mode = "jump", baseURL = "";
  try {
    const st = await api("/library/status");
    mode = st.mode; baseURL = st.base_url || "https://z-lib.sk";
  } catch {}
  state.libMode = mode; state.libBase = baseURL;

  if (mode === "jump" && state.libTab === "search") {
    return renderLibraryJump(app, baseURL);
  }

  app.innerHTML = `
  <div class="card">
    <div class="card-title">${icon("book", 18)} 书库 <span class="sub" id="libQuota"></span>
      <div class="lib-tabs" style="margin-left:auto">
        <button data-tab="search" class="${state.libTab === "search" ? "active" : ""}">搜索</button>
        <button data-tab="shelf" class="${state.libTab === "shelf" ? "active" : ""}">书架</button>
        <button data-tab="history" class="${state.libTab === "history" ? "active" : ""}">转存记录</button>
      </div>
    </div>
    <div id="libToolbar"></div>
    <div id="libList"></div>
    <div class="pager" id="libPager"></div>
  </div>`;

  $$(".lib-tabs button", app).forEach((b) =>
    b.addEventListener("click", () => { state.libTab = b.dataset.tab; renderLibrary(app); })
  );

  if (state.libTab === "search") await libSearchView(app);
  else if (state.libTab === "shelf") await libShelfView(app);
  else await libHistoryView(app);
}

/* 浏览器跳转模式：站内入口 + 新窗口打开 Z-Library 搜索 */
async function renderLibraryJump(app, baseURL) {
  app.innerHTML = `
  <div class="card" style="background:linear-gradient(135deg,#dbe7ff,#eef5ff);border:1px solid #cddcff">
    <div class="card-title">${icon("book", 18)} 书库 · 浏览器搜索</div>
    <p style="font-size:13.5px;color:var(--text-2);line-height:1.9;margin-bottom:16px">
      Z-Library 启用了严格的浏览器验证（人机挑战），服务端无法直接拉取搜索结果。
      在此输入关键词，点击搜索将在<b>新窗口打开 Z-Library</b>（已登录的浏览器中直接浏览与下载）。
    </p>
    <div class="res-toolbar">
      <input class="input" id="jumpQ" placeholder="书名 / 作者 / 关键词…" value="${esc(state.libQuery || "")}">
      <select class="select" id="jumpLang" style="flex:0 0 110px">
        <option value="">全部语言</option><option value="chinese">中文</option><option value="english">English</option>
      </select>
      <select class="select" id="jumpExt" style="flex:0 0 100px">
        <option value="">全部格式</option><option value="epub">EPUB</option><option value="pdf">PDF</option><option value="mobi">MOBI</option>
      </select>
      <button class="btn" id="jumpBtn">${icon("search")} 在 Z-Library 搜索</button>
    </div>
    <div class="jump-quick">
      <span style="font-size:12.5px;color:var(--text-3)">快捷：</span>
      ${["三体", "机器学习", "Go 语言", "深入理解计算机系统", "百年孤独", "人类简史"].map((k) =>
        `<a class="tag" href="${baseURL}/s/${encodeURIComponent(k)}" target="_blank" rel="noopener">${esc(k)}</a>`).join("")}
    </div>
  </div>
  <div class="card">
    <div class="card-title">${icon("megaphone", 18)} 说明</div>
    <ol class="guide-list">
      <li>跳转搜索需先在浏览器中登录 Z-Library（未登录会提示 Sign In）</li>
      <li>下载的文件保存在浏览器下载目录；如需入库可后续经对象存储/网盘整理</li>
      <li>站内「书架 / 转存记录」仅服务端直连模式可用，当前为跳转模式</li>
    </ol>
  </div>`;

  const doJump = () => {
    const q = $("#jumpQ", app).value.trim();
    if (!q) return toast("请输入关键词", true);
    state.libQuery = q;
    const lang = $("#jumpLang", app).value, ext = $("#jumpExt", app).value;
    let u = `${baseURL}/s/${encodeURIComponent(q)}`;
    const ps = [];
    if (lang) ps.push("languages=" + lang);
    if (ext) ps.push("extensions=" + ext);
    if (ps.length) u += "?" + ps.join("&");
    window.open(u, "_blank", "noopener");
  };
  $("#jumpBtn", app).addEventListener("click", doJump);
  $("#jumpQ", app).addEventListener("keydown", (e) => e.key === "Enter" && doJump());
}

async function libSearchView(app) {
  $("#libToolbar", app).innerHTML = `
  <div class="res-toolbar">
    <input class="input" id="libQ" placeholder="书名 / 作者 / 关键词…" value="${esc(state.libQuery || "")}">
    <select class="select" id="libLang" style="flex:0 0 120px">
      <option value="">全部语言</option><option>中文</option><option>English</option>
    </select>
    <select class="select" id="libExt" style="flex:0 0 110px">
      <option value="">全部格式</option><option>EPUB</option><option>PDF</option><option>MOBI</option>
    </select>
    <button class="btn" id="libSearchBtn">${icon("search")} 搜索</button>
  </div>`;
  $("#libList", app).innerHTML = `<div class="empty">${icon("search", 40)}<br>输入关键词开始找书，例如「三体」「Go」「分布式」</div>`;

  const doSearch = async () => {
    state.libQuery = $("#libQ", app).value.trim();
    state.libPage = 1;
    await loadLibBooks(app);
  };
  $("#libSearchBtn", app).addEventListener("click", doSearch);
  $("#libQ", app).addEventListener("keydown", (e) => e.key === "Enter" && doSearch());
  if (state.libQuery) await loadLibBooks(app);
}

async function loadLibBooks(app) {
  const list = $("#libList", app);
  const pager = $("#libPager", app);
  pager.innerHTML = "";
  list.innerHTML = `<div class="empty">搜索中…</div>`;
  try {
    const qs = new URLSearchParams({
      q: state.libQuery, page: state.libPage || 1,
      lang: $("#libLang", app)?.value || "", ext: $("#libExt", app)?.value || "",
    });
    const res = await api(`/library/search?${qs}`);
    $("#libQuota", app).textContent = res.quota ? `· ${res.quota}` : "";
    if (!res.books?.length) {
      list.innerHTML = `<div class="empty">${icon("search", 40)}<br>没有找到相关书籍</div>`;
      return;
    }
    list.innerHTML = res.books.map(bookCard).join("");
    bindBookCards(app);
    renderPager(pager, { total: res.total, page_size: 10, page: res.page }, state.libPage,
      (p) => { state.libPage = p; loadLibBooks(app); });
  } catch (e) {
    list.innerHTML = `<div class="empty">${icon("alert", 36)}${esc(e.message)}</div>`;
  }
}

function bookCover(b) {
  const color = LIB_EXT_COLORS[(b.extension || "").toUpperCase()] || "#64748b";
  const ch = (b.title || "?").trim().charAt(0);
  return `<div class="book-cover" style="background:linear-gradient(150deg,${color}bb,${color})"><span>${esc(ch)}</span><i>${esc(b.extension || "")}</i></div>`;
}

function bookCard(b) {
  return `
  <div class="book-card" data-id="${esc(b.id)}">
    ${bookCover(b)}
    <div class="binfo">
      <div class="btitle" title="${esc(b.title)}">${esc(b.title)}</div>
      <div class="bauthor">${esc((b.authors || []).join(" / ") || "佚名")}</div>
      <div class="bmeta">
        <span class="tag" style="background:${(LIB_EXT_COLORS[(b.extension || "").toUpperCase()] || "#64748b")}22;color:${LIB_EXT_COLORS[(b.extension || "").toUpperCase()] || "#64748b"}">${esc(b.extension || "?")}</span>
        <span>${esc(b.size_text || "")}</span>·<span>${esc(b.language || "")}</span>${b.year ? `·<span>${esc(b.year)}</span>` : ""}
      </div>
      <div class="bactions">
        <button class="btn sm" data-act="transfer">${icon("upload", 14)} 转存</button>
        <button class="btn ghost sm" data-act="detail">详情</button>
        <button class="btn ghost sm" data-act="shelf">${icon("star", 14)} 收藏</button>
      </div>
      <div class="bdetail" hidden>${esc(b.description || "暂无简介")}</div>
    </div>
  </div>`;
}

function bindBookCards(app) {
  $$(".book-card", app).forEach((card) => {
    const id = card.dataset.id;
    const title = $(".btitle", card).textContent;
    const authors = $(".bauthor", card).textContent.split(" / ").filter((x) => x && x !== "佚名");
    card.querySelectorAll("button[data-act]").forEach((btn) =>
      btn.addEventListener("click", async () => {
        const act = btn.dataset.act;
        if (act === "detail") {
          const d = $(".bdetail", card);
          if (d.textContent === "暂无简介") {
            try { const b = await api(`/library/books/${id}`); d.textContent = b.description || "暂无简介"; } catch {}
          }
          d.hidden = !d.hidden;
        } else if (act === "shelf") {
          try {
            await api("/library/shelf", { method: "POST", body: JSON.stringify({ book_id: id, title, authors }) });
            toast("已加入书架");
          } catch (e) { toast(e.message, true); }
        } else if (act === "transfer") {
          try {
            await api("/library/transfer", { method: "POST", body: JSON.stringify({ book_id: id, title, authors }) });
            toast("转存任务已提交");
            state.libTab = "history";
            renderLibrary($("#app"));
          } catch (e) { toast(e.message, true); }
        }
      })
    );
  });
}

async function libShelfView(app) {
  $("#libToolbar", app).innerHTML = "";
  const body = $("#libList", app);
  $("#libPager", app).innerHTML = "";
  body.innerHTML = `<div class="empty">加载中…</div>`;
  try {
    const items = await api("/library/shelf");
    if (!items?.length) {
      body.innerHTML = `<div class="empty">${icon("book", 40)}<br>书架空空如也，去搜索页收藏几本吧</div>`;
      return;
    }
    body.innerHTML = `
    <table class="tbl"><thead><tr><th>书名</th><th>作者</th><th>格式</th><th>收藏时间</th><th>操作</th></tr></thead>
    <tbody>${items.map((it) => `
      <tr>
        <td style="font-weight:600">${esc(it.title)}</td>
        <td>${esc((it.authors || []).join(" / "))}</td>
        <td>${esc(it.format || "-")}</td>
        <td style="color:var(--text-3);font-size:12.5px">${fmtDate(it.added_at)}</td>
        <td style="white-space:nowrap">
          <button class="btn sm" data-x="transfer" data-id="${esc(it.book_id)}" data-title="${esc(it.title)}" data-authors='${esc(JSON.stringify(it.authors || []))}'>${icon("upload", 14)} 转存</button>
          <button class="btn danger-ghost sm" data-x="del" data-id="${esc(it.book_id)}">移出</button>
        </td>
      </tr>`).join("")}</tbody></table>`;
    $$("button[data-x]", body).forEach((b) =>
      b.addEventListener("click", async () => {
        try {
          if (b.dataset.x === "transfer") {
            await api("/library/transfer", { method: "POST", body: JSON.stringify({ book_id: b.dataset.id, title: b.dataset.title, authors: JSON.parse(b.dataset.authors || "[]") }) });
            toast("转存任务已提交");
            state.libTab = "history";
            renderLibrary($("#app"));
          } else {
            await api(`/library/shelf/${b.dataset.id}`, { method: "DELETE" });
            toast("已移出书架");
            libShelfView(app);
          }
        } catch (e) { toast(e.message, true); }
      })
    );
  } catch (e) {
    body.innerHTML = `<div class="empty">${icon("alert", 36)}${esc(e.message)}</div>`;
  }
}

const BOOK_STATUS = { queued: "排队中", saving: "转存中", done: "已完成", failed: "失败" };

async function libHistoryView(app) {
  $("#libToolbar", app).innerHTML = "";
  const body = $("#libList", app);
  $("#libPager", app).innerHTML = "";
  body.innerHTML = `<div class="empty">加载中…</div>`;
  try {
    const data = await api("/library/history");
    if (!data.list?.length) {
      body.innerHTML = `<div class="empty">${icon("inbox", 40)}<br>暂无转存记录，去搜索页转存第一本书</div>`;
      return;
    }
    body.innerHTML = `
    <table class="tbl"><thead><tr><th>书名</th><th>格式</th><th>大小</th><th>存储位置</th><th>状态</th><th>时间</th><th>操作</th></tr></thead>
    <tbody>${data.list.map((r) => `
      <tr>
        <td style="font-weight:600;max-width:300px;overflow:hidden;text-overflow:ellipsis;white-space:nowrap" title="${esc(r.title)}">${esc(r.title)}</td>
        <td>${esc((r.format || "").toUpperCase())}</td>
        <td>${r.size_bytes ? fmtSize(r.size_bytes) : "-"}</td>
        <td>${esc(r.storage_kind === "local" ? "本地" : r.storage_bucket || "默认 Bucket")}</td>
        <td><span class="status-dot ${r.status === "done" ? "success" : r.status === "failed" ? "failed" : "running"}"></span>${BOOK_STATUS[r.status] || r.status}${r.error ? `<div style="color:var(--danger);font-size:12px;max-width:280px">${esc(r.error)}</div>` : ""}</td>
        <td style="color:var(--text-3);font-size:12.5px">${fmtDate(r.created_at)}</td>
        <td style="white-space:nowrap">
          ${r.status === "done" ? `<a class="btn ghost sm" href="/api/v1/library/download/${esc(r.id)}">${icon("download", 14)} 下载</a>` : ""}
          <button class="btn danger-ghost sm" data-del="${esc(r.id)}">删除</button>
        </td>
      </tr>`).join("")}</tbody></table>`;
    $$("button[data-del]", body).forEach((b) =>
      b.addEventListener("click", async () => {
        try { await api(`/library/history/${b.dataset.del}`, { method: "DELETE" }); toast("已删除"); libHistoryView(app); }
        catch (e) { toast(e.message, true); }
      })
    );
    // 有进行中的任务时自动刷新
    if (data.list.some((r) => ["queued", "saving"].includes(r.status))) {
      setTimeout(() => { if (currentRoute() === "library" && state.libTab === "history") libHistoryView(app); }, 1200);
    }
  } catch (e) {
    body.innerHTML = `<div class="empty">${icon("alert", 36)}${esc(e.message)}</div>`;
  }
}

/* ---------- 我的网盘（浏览 + 管理） ---------- */
const DRIVE_ICONS = {
  mp4: "video", mkv: "video", avi: "video", mov: "video",
  pdf: "fileText", epub: "book", mobi: "book", txt: "fileText", md: "fileText",
  mp3: "music", flac: "music", wav: "music",
  zip: "archive", rar: "archive", "7z": "archive",
  dmg: "disc", exe: "disc", pkg: "disc", iso: "disc",
  jpg: "image", jpeg: "image", png: "image", gif: "image",
};

function fileIcon(entry) {
  if (entry.is_dir) return icon("folder", 16);
  const ext = (entry.name.split(".").pop() || "").toLowerCase();
  return icon(DRIVE_ICONS[ext] || "box", 16);
}

async function renderDrive(app) {
  app.innerHTML = `
  <div class="card">
    <div class="card-title">${icon("drive", 18)} 我的网盘
      <div style="margin-left:auto;display:flex;gap:10px">
        <button class="btn ghost sm" id="driveMkdir">${icon("plus", 14)} 新建文件夹</button>
        <button class="btn ghost sm" id="driveRefresh">${icon("refresh", 14)} 刷新</button>
      </div>
    </div>
    <div class="drive-cap" id="driveCap"></div>
    <div class="drive-crumb" id="driveCrumb"></div>
    <div class="table-wrap" id="driveList"></div>
    <div class="drive-pager" id="drivePager"></div>
  </div>
  <div id="driveModal"></div>`;

  $("#driveMkdir", app).addEventListener("click", () => drivePromptMkdir(app));
  $("#driveRefresh", app).addEventListener("click", () => loadDrive(app));
  await loadDrive(app);
}

async function loadDrive(app) {
  const dir = state.drivePath.at(-1).id;
  const crumb = $("#driveCrumb", app);
  // 面包屑：点击任意层级回退
  crumb.innerHTML = state.drivePath
    .map((p, i) => `<a href="javascript:void(0)" data-i="${i}">${esc(p.name)}</a>`)
    .join('<span class="crumb-sep">/</span>');
  $$("a", crumb).forEach((a) =>
    a.addEventListener("click", () => {
      state.drivePath = state.drivePath.slice(0, Number(a.dataset.i) + 1);
      state.drivePage = 1;
      loadDrive(app);
    })
  );

  const box = $("#driveList", app);
  const pager = $("#drivePager", app);
  box.innerHTML = `<div class="empty">加载中…</div>`;
  pager.innerHTML = "";
  try {
    const data = await api(`/drive/list?dir=${dir}&page=${state.drivePage || 1}`);
    renderDriveList(app, data);
  } catch (e) {
    box.innerHTML = `<div class="empty">${icon("alert", 36)}${esc(e.message)}</div>`;
  }
  loadDriveCap(app);
}

async function loadDriveCap(app) {
  const cap = $("#driveCap", app);
  if (!cap) return;
  try {
    const c = await api("/drive/capacity");
    const pct = c.total ? Math.min(100, (c.used / c.total) * 100) : 0;
    cap.innerHTML = `
      <div class="drive-cap-bar"><div style="width:${pct}%"></div></div>
      <span class="drive-cap-text">${fmtSize(c.used)} / ${fmtSize(c.total)}（${pct.toFixed(1)}%）</span>`;
  } catch { cap.innerHTML = ""; }
}

function renderDriveList(app, data) {
  const box = $("#driveList", app);
  const entries = data.entries || [];
  state.drivePage = data.page || state.drivePage || 1;
  renderDrivePager(app, data, entries.length);
  if (!entries.length) {
    box.innerHTML = `<div class="empty">${icon("folder", 40)}<br>此目录为空</div>`;
    return;
  }
  box.innerHTML = `
  <table class="tbl">
    <thead><tr><th>名称</th><th>大小</th><th>修改时间</th><th style="width:220px">操作</th></tr></thead>
    <tbody>
    ${entries.map((e) => `
      <tr data-id="${e.file_id}" data-name="${esc(e.name)}" data-dir="${e.is_dir}">
        <td><span class="dentry ${e.is_dir ? "isdir" : ""}"><span class="ficon">${fileIcon(e)}</span>${esc(e.name)}</span></td>
        <td style="color:var(--text-3)">${fmtSize(e.size)}</td>
        <td style="color:var(--text-3);font-size:12.5px">${esc(fmtDate(e.update_at))}</td>
        <td>
          ${e.is_dir ? "" : `<a class="btn ghost sm" href="/api/v1/drive/download/${e.file_id}">${icon("download", 14)} 下载</a>`}
          <button class="btn ghost sm" data-op="rename">重命名</button>
          <button class="btn ghost sm" data-op="move">移动</button>
          <button class="btn danger-ghost sm" data-op="trash">删除</button>
        </td>
      </tr>`).join("")}
    </tbody>
  </table>`;

  // 目录点击进入
  $$("tr[data-dir]", box).forEach((tr) =>
    tr.addEventListener("click", (ev) => {
      if (ev.target.closest("button,a")) return;
      if (tr.dataset.dir === "true") {
        state.drivePath.push({ id: Number(tr.dataset.id), name: tr.dataset.name });
        state.drivePage = 1;
        loadDrive(app);
      }
    })
  );

  // 行操作
  $$("button[data-op]", box).forEach((btn) =>
    btn.addEventListener("click", async () => {
      const tr = btn.closest("tr");
      const id = Number(tr.dataset.id);
      const name = tr.dataset.name;
      const op = btn.dataset.op;
      try {
        if (op === "rename") {
          const nn = prompt("重命名为：", name);
          if (!nn || nn === name) return;
          await api("/drive/rename", { method: "POST", body: JSON.stringify({ file_id: id, name: nn }) });
          toast("已重命名");
          loadDrive(app);
        } else if (op === "move") {
          driveMoveDialog(app, id, name);
        } else if (op === "trash") {
          if (!confirm(`确定把「${name}」移入回收站？`)) return;
          await api("/drive/trash", { method: "POST", body: JSON.stringify({ file_ids: [id] }) });
          toast("已移入回收站");
          loadDrive(app);
        }
      } catch (e) { toast(e.message, true); }
    })
  );
}

function renderDrivePager(app, data, count) {
  const pager = $("#drivePager", app);
  const page = data.page || state.drivePage || 1;
  const pageSize = data.page_size || 50;
  const total = Number(data.total) || 0;
  const pages = Math.max(1, Math.ceil(total / pageSize));
  const hasMore = Boolean(data.has_more);
  pager.innerHTML = `
    <button class="btn ghost sm" data-page="${page - 1}" ${page <= 1 ? "disabled" : ""}>上一页</button>
    <span>第 ${page} / ${pages} 页 · 共 ${total} 条 · 每页最多 ${pageSize} 条</span>
    <button class="btn ghost sm" data-page="${page + 1}" ${!hasMore ? "disabled" : ""}>下一页</button>`;
  $$("button[data-page]", pager).forEach((btn) =>
    btn.addEventListener("click", () => {
      state.drivePage = Number(btn.dataset.page);
      loadDrive(app);
    })
  );
}

async function drivePromptMkdir(app) {
  const name = prompt("新文件夹名称：", "");
  if (!name) return;
  try {
    await api("/drive/mkdir", { method: "POST", body: JSON.stringify({ parent_id: state.drivePath.at(-1).id, name }) });
    toast("已创建");
    loadDrive(app);
  } catch (e) { toast(e.message, true); }
}

// 移动弹层：内嵌目录树选目标
async function driveMoveDialog(app, fileID, fileName) {
  const modal = $("#driveModal", app);
  modal.innerHTML = `<div class="modal-mask" id="moveMask"><div class="modal-card">
    <div class="card-title" style="margin-bottom:10px">移动「${esc(fileName)}」到…</div>
    <div class="drive-tree" id="moveTree">加载目录树…</div>
    <div style="display:flex;gap:10px;justify-content:flex-end;margin-top:14px">
      <button class="btn ghost sm" id="moveCancel">取消</button>
      <button class="btn sm" id="moveOk" disabled>移动到根目录</button>
    </div>
  </div></div>`;

  let target = { id: 0, name: "根目录" };
  const okBtn = $("#moveOk", modal);
  const renderTree = (nodes, depth) => nodes.map((n) => `
    <div class="tree-node" style="padding-left:${depth * 18}px">
      <a href="javascript:void(0)" data-id="${n.file_id}" data-name="${esc(n.name)}">${icon("folder", 14)} ${esc(n.name)}</a>
    </div>
    ${renderTree(n.dirs || [], depth + 1)}`).join("");

  try {
    const tree = await api("/drive/tree?dir=0");
    $("#moveTree", modal).innerHTML = `
      <div class="tree-node"><a href="javascript:void(0)" data-id="0" data-name="根目录" class="sel">${icon("folder", 14)} 根目录（顶层）</a></div>
      ${renderTree(tree.dirs || [], 1)}`;
    $$("#moveTree a", modal).forEach((a) =>
      a.addEventListener("click", () => {
        $$("#moveTree a", modal).forEach((x) => x.classList.remove("sel"));
        a.classList.add("sel");
        target = { id: Number(a.dataset.id), name: a.dataset.name };
        okBtn.disabled = false;
        okBtn.textContent = `移动到「${target.name}」`;
      })
    );
  } catch (e) {
    $("#moveTree", modal).innerHTML = `<div class="empty">${esc(e.message)}</div>`;
  }

  $("#moveCancel", modal).addEventListener("click", () => (modal.innerHTML = ""));
  $("#moveMask", modal).addEventListener("click", (e) => { if (e.target.id === "moveMask") modal.innerHTML = ""; });
  okBtn.addEventListener("click", async () => {
    try {
      await api("/drive/move", { method: "POST", body: JSON.stringify({ file_ids: [fileID], target_id: target.id }) });
      toast(`已移动到「${target.name}」`);
      modal.innerHTML = "";
      loadDrive(app);
    } catch (e) { toast(e.message, true); }
  });
}

/* ---------- Gmail 邮箱（多供应商：Outlook 规划中） ---------- */
async function renderMailbox(app) {
  app.innerHTML = `
  <div class="card" style="padding:0;overflow:hidden">
    <div class="mail-toolbar">
      <div class="mail-brand">${icon("mail", 18)} <span id="mailProvTabs" class="lib-tabs"></span></div>
      <input class="input" id="mailQ" placeholder="搜索邮件（支持 from:/is:unread 等 Gmail 语法）" style="flex:1;max-width:400px">
      <button class="btn sm" id="mailSearch">${icon("search", 14)} 搜索</button>
      <button class="btn ghost sm" id="mailRefresh">${icon("refresh", 14)} 刷新</button>
    </div>
    <div class="mail-layout">
      <div class="mail-list" id="mailList"></div>
      <div class="mail-detail" id="mailDetail">
        <div class="empty" style="padding:60px 0">${icon("mail", 40)}<br>选择左侧邮件查看详情</div>
      </div>
    </div>
    <div class="pager" id="mailPager" style="padding:10px"></div>
  </div>`;

  // 供应商 tabs
  let provs = [];
  try {
    provs = await api("/mail/providers");
  } catch {}
  if (!provs.some((pv) => pv.id === state.mailProv)) {
    state.mailProv = provs.find((pv) => !pv.status?.coming)?.id || provs[0]?.id || "gmail";
  }
  const tabs = $("#mailProvTabs", app);
  tabs.innerHTML = provs.map((pv) => {
    const active = pv.id === state.mailProv;
    const coming = pv.status?.coming;
    const unconfigured = pv.status?.configured === false;
    const suffix = coming ? " · 规划中" : unconfigured ? " · 未配置" : "";
    return `<button data-prov="${esc(pv.id)}" class="${active ? "active" : ""}" ${coming ? "disabled title='规划中'" : ""}>${esc(pv.name)}${suffix}</button>`;
  }).join("");
  $$("button[data-prov]", tabs).forEach((b) =>
    b.addEventListener("click", () => { state.mailProv = b.dataset.prov; state.mailToken = ""; renderMailbox(app); })
  );

  const cur = provs.find((pv) => pv.id === state.mailProv);
  if (cur?.status?.configured === false) {
    $("#mailQ", app).disabled = true;
    $("#mailSearch", app).disabled = true;
    $("#mailRefresh", app).disabled = true;
    $(".mail-layout", app).innerHTML = `
      <div class="mail-setup-state">
        <span class="mail-setup-icon">${icon("mail", 30)}</span>
        <h3>尚未配置 Gmail</h3>
        <p>完成 Google OAuth 只读授权后，即可浏览、搜索邮件并下载附件。</p>
        <button class="btn sm" id="mailGoSettings">${icon("settings", 14)} 前往邮箱配置</button>
      </div>`;
    $("#mailPager", app).innerHTML = "";
    $("#mailGoSettings", app).addEventListener("click", () => {
      state.settingsTab = "mail";
      location.hash = "#/settings";
    });
    return;
  }
  if (cur?.status?.coming) {
    $("#mailList", app).innerHTML = `<div class="empty">${icon("alert", 40)}<br>${esc(cur.name)} 邮箱规划中<br><span style="font-size:12px">架构已预留，按 mail.Provider 接口实现后自动上线</span></div>`;
    return;
  }

  $("#mailSearch", app).addEventListener("click", () => { state.mailToken = ""; loadMailList(app); });
  $("#mailQ", app).addEventListener("keydown", (e) => e.key === "Enter" && (state.mailToken = "", loadMailList(app)));
  $("#mailRefresh", app).addEventListener("click", () => loadMailList(app));
  await loadMailList(app);
}

async function loadMailList(app, pageToken) {
  const box = $("#mailList", app);
  box.innerHTML = `<div class="empty">加载中…</div>`;
  try {
    const qs = new URLSearchParams({ q: $("#mailQ", app).value.trim() });
    if (pageToken) qs.set("page_token", pageToken);
    state.mailToken = pageToken || "";
    const data = await api(`/mail/${state.mailProv}/messages?${qs}`);
    if (!data.messages?.length) {
      box.innerHTML = `<div class="empty">${icon("inbox", 40)}<br>没有邮件</div>`;
    } else {
      box.innerHTML = data.messages.map((m) => `
        <div class="mail-item ${m.unread ? "unread" : ""}" data-id="${esc(m.id)}">
          <div class="mi-top">
            <span class="mi-from">${esc(m.from)}</span>
            ${m.has_attachment ? `<span class="mi-att">${icon("paperclip", 13)}</span>` : ""}
          </div>
          <div class="mi-subject">${esc(m.subject)}</div>
          <div class="mi-snippet">${esc(m.snippet)}</div>
          <div class="mi-date">${esc(m.date)}</div>
        </div>`).join("");
      $$(".mail-item", box).forEach((el) =>
        el.addEventListener("click", () => {
          $$(".mail-item", box).forEach((x) => x.classList.remove("sel"));
          el.classList.add("sel");
          loadMailDetail(app, el.dataset.id);
        })
      );
    }
    const pager = $("#mailPager", app);
    pager.innerHTML = `
      <button id="mailPrev" ${pageToken ? "" : "disabled"}>上一组</button>
      <span>${data.result_total ?? "-"} 封邮件</span>
      <button id="mailNext" ${data.next_page_token ? "" : "disabled"}>下一组</button>`;
    const prev = $("#mailPrev", pager), next = $("#mailNext", pager);
    if (next && !next.disabled) next.addEventListener("click", () => loadMailList(app, data.next_page_token));
    if (prev && !prev.disabled) prev.addEventListener("click", () => loadMailList(app, ""));
  } catch (e) {
    box.innerHTML = `<div class="empty">${icon("alert", 36)}${esc(e.message)}</div>`;
  }
}

async function loadMailDetail(app, id) {
  const box = $("#mailDetail", app);
  box.innerHTML = `<div class="empty">加载中…</div>`;
  try {
    const m = await api(`/mail/${state.mailProv}/messages/${id}`);
    box.innerHTML = `
      <div class="md-head">
        <h3>${esc(m.subject)}</h3>
        <div class="md-meta">
          <div>发件人：<b>${esc(m.from)}</b></div>
          <div>收件人：${esc(m.to)}</div>
          <div>时间：${esc(m.date)}</div>
        </div>
      </div>
      ${m.attachments?.length ? `
      <div class="md-atts">
        ${m.attachments.map((a) => `
          <div class="md-att">
            <span class="ma-icon">${icon(a.mime_type?.includes("pdf") ? "fileText" : a.mime_type?.startsWith("image") ? "image" : "paperclip", 16)}</span>
            <span class="ma-name">${esc(a.filename)}</span>
            <span class="ma-size">${fmtSize(a.size)}</span>
            <a class="btn ghost sm" href="/api/v1/mail/${esc(state.mailProv)}/attachment/${esc(m.id)}/${esc(a.id)}">下载</a>
          </div>`).join("")}
      </div>` : ""}
      <pre class="md-body">${esc(m.body_text || "（无文本正文）")}</pre>`;
  } catch (e) {
    box.innerHTML = `<div class="empty">${icon("alert", 36)}${esc(e.message)}</div>`;
  }
}

/* ---------- Cloudflare 账户监控 ---------- */
async function renderMonitor(app) {
  app.innerHTML = `
  <div class="dash-stats" id="cfSummary"></div>
  <div class="card">
    <div class="card-title">${icon("globe", 18)} 站点访问 <span class="sub">近 30 天 · 数据源 Cloudflare Analytics</span></div>
    <div id="cfZones"></div>
  </div>
  <div class="dash-columns">
    <div class="card">
      <div class="card-title">${icon("activity", 18)} Workers <span class="sub">近 30 天调用量</span></div>
      <div class="table-wrap" id="cfWorkers"></div>
    </div>
    <div class="card">
      <div class="card-title">${icon("database", 18)} R2 对象存储 <span class="sub">当前用量</span></div>
      <div class="table-wrap" id="cfR2"></div>
    </div>
  </div>`;

  let d;
  try {
    d = await api("/cfmon/overview");
  } catch (e) {
    $("#cfSummary", app).innerHTML = `<div class="card"><div class="empty">${icon("alert", 36)}${esc(e.message)}</div></div>`;
    return;
  }
  try {
  } catch {}

  // 概要
  const totalReqs = (d.zones || []).reduce((a, z) => a + z.requests, 0);
  const r2Size = (d.r2_buckets || []).reduce((a, b) => a + b.payload_size, 0);
  const r2Objs = (d.r2_buckets || []).reduce((a, b) => a + b.object_count, 0);
  $("#cfSummary", app).innerHTML = [
    [esc(d.account_name || d.account_id || "-"), "Cloudflare 账户"],
    [d.workers?.length ?? 0, "Workers 脚本"],
    [d.zones?.length ?? 0, "接入域名"],
    [totalReqs.toLocaleString(), "30 天总请求"],
    [`${fmtSize(r2Size)} / ${r2Objs.toLocaleString()} 对象`, "R2 存储"],
  ].map(([v, l]) => `<div class="dash-stat"><b style="font-size:${String(v).length > 12 ? "15px" : "24px"}">${v}</b><span>${l}</span></div>`).join("");

  // 站点访问
  const zbox = $("#cfZones", app);
  if (!d.zones?.length) {
    zbox.innerHTML = `<div class="empty">没有查询到域名（Zone Analytics 权限或未接入域名）</div>`;
  } else {
    zbox.innerHTML = d.zones.map((z) => {
      const maxReq = Math.max(...(z.daily || []).map((p) => p.requests), 1);
      const hit = z.requests ? (z.cached_requests / z.requests * 100).toFixed(1) : "0";
      return `
      <div class="cf-zone">
        <div class="cf-zone-head">
          <b>${esc(z.name)}</b>
          <span class="tag ${z.status === "active" ? "green" : "gray"}">${esc(z.status || "-")}</span>
          <div class="cf-zone-stats">
            <span>请求 <b>${z.requests.toLocaleString()}</b></span>
            <span>带宽 <b>${fmtSize(z.bytes)}</b></span>
            <span>缓存命中 <b>${hit}%</b></span>
            <span>威胁拦截 <b>${z.threats.toLocaleString()}</b></span>
            <span>浏览量 <b>${z.page_views.toLocaleString()}</b></span>
          </div>
        </div>
        <div class="cf-chart">
          ${(z.daily || []).map((pt) => `<div class="cf-bar" style="height:${Math.max(4, pt.requests / maxReq * 100)}%" title="${esc(pt.date)}：${pt.requests.toLocaleString()} 次请求"></div>`).join("")}
        </div>
      </div>`;
    }).join("");
  }

  // Workers
  const wbox = $("#cfWorkers", app);
  if (!d.workers?.length) {
    wbox.innerHTML = `<div class="empty">没有 Worker 脚本</div>`;
  } else {
    wbox.innerHTML = `
    <table class="tbl"><thead><tr><th>脚本</th><th>请求数</th><th>错误</th><th>CPU P99</th></tr></thead>
    <tbody>${d.workers.map((w) => `
      <tr>
        <td class="mono">${esc(w.script)}</td>
        <td><b>${w.requests.toLocaleString()}</b></td>
        <td>${w.errors > 0 ? `<span style="color:var(--danger)">${w.errors.toLocaleString()}</span>` : "0"}</td>
        <td>${w.cpu_time_p99_ms.toFixed(1)} ms</td>
      </tr>`).join("")}</tbody></table>`;
  }

  // R2
  const rbox = $("#cfR2", app);
  if (!d.r2_buckets?.length) {
    rbox.innerHTML = `<div class="empty">没有 R2 桶</div>`;
  } else {
    rbox.innerHTML = `
    <table class="tbl"><thead><tr><th>桶</th><th>对象数</th><th>存储量</th></tr></thead>
    <tbody>${d.r2_buckets.map((b) => `
      <tr>
        <td class="mono">${esc(b.name)}</td>
        <td>${b.object_count.toLocaleString()}</td>
        <td><b>${fmtSize(b.payload_size)}</b></td>
      </tr>`).join("")}</tbody></table>`;
  }
}

/* ---------- 占位模块通用页 ---------- */
async function renderComing(app) {
  const qs = new URLSearchParams(location.hash.split("?")[1] || "");
  const mod = moduleById(qs.get("m"));
  const name = mod ? mod.name : "该功能";
  const desc = mod ? mod.desc : "";
  app.innerHTML = `
  <div class="card coming-card">
    ${icon("alert", 52)}
    <h2>${esc(name)}</h2>
    <p class="desc">${esc(desc)}</p>
    <p class="note">该模块正在规划中，当前版本先占位。架构上已预留接入位置（前端模块注册表 + 后端 Provider/Store 扩展点）。</p>
    <div style="display:flex;gap:12px;justify-content:center;margin-top:18px">
      <a class="btn ghost" href="#/dashboard">← 返回概览</a>
      ${mod && mod.id === "transfer" ? `<a class="btn" href="#/home">进入模块</a>` : ""}
    </div>
  </div>`;
}

async function render() {
  const route = currentRoute();
  $$("#nav a").forEach((a) => a.classList.toggle("active", a.dataset.route === route));
  renderSubnav(route);
  const app = $("#app");
  try {
    await routes[route](app);
  } catch (e) {
    app.innerHTML = `<div class="card"><div class="empty">${icon("alert", 36)}${esc(e.message)}</div></div>`;
  }
}
window.addEventListener("hashchange", render);

/* ---------- 首页：转存工作台 ---------- */

/* 一键转存：解析分享 → 全部文件提交 → 转存记录页看进度 */
async function oneClickTransfer(shareUrl, title) {
  toast(`解析中：${title || ""}`);
  try {
    const r = await api("/parse", { method: "POST", body: JSON.stringify({ provider: "123pan", share_url: shareUrl }) });
    const task = await api("/transfer", {
      method: "POST",
      body: JSON.stringify({
        provider: "123pan", share_url: shareUrl, share_name: r.share_name,
        file_ids: r.files.map(f => f.id), file_count: r.file_count, total_size: r.total_size,
      }),
    });
    toast(`已提交转存（${r.file_count} 个文件），进度见转存记录`);
    location.hash = "#/records";
  } catch (e) { toast(e.message, true); }
}

/* ---------- 分页条（书库/邮箱/资源广场共用） ---------- */
function renderPager(box, data, page, go) {
  const pages = Math.max(1, data.total_pages || Math.ceil(data.total / data.page_size));
  box.innerHTML = `
    <button ${page <= 1 ? "disabled" : ""} data-p="${page - 1}">上一页</button>
    <span>第 ${page} / ${pages} 页 · 共 ${data.total} 条</span>
    <button ${page >= pages ? "disabled" : ""} data-p="${page + 1}">下一页</button>`;
  $$("button", box).forEach((b) => b.addEventListener("click", () => go(Number(b.dataset.p))));
}


/* 论坛登录弹窗：反代 iframe 完成人工验证，后端自动截获并持久化 Cookie。 */
function showForumLoginModal(onSuccess) {
  // 移除旧弹窗
  document.querySelector("#forumLoginMask")?.remove();
  const mask = document.createElement("div");
  mask.id = "forumLoginMask";
  mask.className = "modal-mask";
  mask.innerHTML = `
    <div class="modal-card" style="width:min(480px,94vw);height:640px;padding:0;overflow:hidden">
      <div style="display:flex;justify-content:space-between;align-items:center;padding:12px 18px;border-bottom:1px solid var(--border)">
        <b>${icon("key", 16)} 连接论坛（登录+滑块验证）</b>
        <button class="btn ghost sm" id="flClose">✕</button>
      </div>
      <iframe src="/api/forum/proxy/${encodeURIComponent(state.rpSource)}/user-login.htm"
        style="width:100%;height:calc(100% - 46px);border:none"
        id="forumLoginFrame"></iframe>
    </div>`;
  document.body.appendChild(mask);
  const cleanup = () => {
    window.removeEventListener("message", onLoginMessage);
    mask.remove();
  };
  $("#flClose").addEventListener("click", cleanup);
  mask.addEventListener("click", (e) => { if (e.target === mask) cleanup(); });

  // iframe 登录成功后自动关闭并刷新
  const onLoginMessage = async (e) => {
    if (e.origin !== window.location.origin || e.data?.type !== "forum-login-ok") return;
    cleanup();
    toast("论坛连接成功，Cookie 已自动保存（约30天免登）");
    if (typeof onSuccess === "function") await onSuccess();
  };
  window.addEventListener("message", onLoginMessage);
}

/* ---------- 资源广场（论坛实时资源：pan1.me / 123panfx） ---------- */
async function renderResources(app) {
  app.innerHTML = `
  <div class="card">
    <div class="card-title">${icon("compass", 18)} 资源广场
      <span class="sub" id="rpStatus">…</span>
      <div class="lib-tabs" style="margin-left:auto" id="rpTabs"></div>
    </div>
    <div class="res-toolbar">
      <input class="input" id="rpQ" placeholder="输入片名搜索论坛资源，如「繁花」「怪奇物语」…" value="${esc(state.rpQuery)}">
      <select class="select" id="rpCat" style="flex:0 0 100px">
        ${["全部","短剧","电视剧","电影","动漫","综艺","纪录片","其他"].map(c =>
          `<option value="${c}" ${c === state.rpCat ? "selected" : ""}>${c}</option>`).join("")}
      </select>
      <button class="btn" id="rpSearch">${icon("search", 14)} 搜索</button>
      <button class="btn ghost sm" id="rpRefresh">${icon("refresh", 14)} 最新</button>
    </div>
    <div class="res-grid" id="rpGrid"></div>
    <div class="pager" id="rpPager"></div>
  </div>`;

  // 源 tabs
  let sources = [];
  try { sources = await api("/respool/sources"); } catch {}
  // 无登录态：显示连接按钮，在应用内代理页面完成登录和滑块验证。
  if (sources.length && !sources[0].has_login) {
    const toolbar = $("#rpQ", app)?.closest(".res-toolbar");
    if (toolbar && !toolbar.querySelector("#rpLogin")) {
      toolbar.insertAdjacentHTML("beforeend",
        `<button class="btn sm" id="rpLogin" style="white-space:nowrap">${icon("key", 14)} 连接论坛</button>`);
      $("#rpLogin", app).addEventListener("click", () => showForumLoginModal(() => renderResources(app)));
    }
  }
  const tabs = $("#rpTabs", app);
  tabs.innerHTML = sources.map(sv =>
    `<button data-src="${esc(sv.id)}" class="${sv.id === state.rpSource ? "active" : ""}">${esc(sv.name)}</button>`).join("");
  $$("button[data-src]", tabs).forEach(b =>
    b.addEventListener("click", () => { state.rpSource = b.dataset.src; state.rpPage = 1; renderResources(app); }));

  const doSearch = async () => {
    state.rpQuery = $("#rpQ", app).value.trim();
    state.rpPage = 1;
    if (!state.rpQuery) { loadRpThreads(app); return; }
    const grid = $("#rpGrid", app);
    grid.innerHTML = `<div class="empty" style="grid-column:1/-1">搜索「${esc(state.rpQuery)}」中…</div>`;
    try {
      const data = await api(`/media/search?q=${encodeURIComponent(state.rpQuery)}&source=${state.rpSource}&category=${encodeURIComponent(state.rpCat || "全部")}`);
      state.rpCache = data.list || [];
      $("#rpStatus", app).textContent = `· 搜到 ${state.rpCache.length} 帖 · 最多获取前 50 条`;
      renderRpGrid(app, state.rpCache);
    } catch (e) {
      grid.innerHTML = `<div class="empty" style="grid-column:1/-1">${icon("alert", 36)}<br>${esc(e.message)}</div>`;
    }
  };
  $("#rpCat", app).addEventListener("change", (e) => { state.rpCat = e.target.value; state.rpPage = 1; loadRpThreads(app); });
  $("#rpSearch", app).addEventListener("click", doSearch);
  $("#rpQ", app).addEventListener("keydown", (e) => e.key === "Enter" && doSearch());
  $("#rpRefresh", app).addEventListener("click", () => { state.rpQuery = ""; state.rpPage = 1; $("#rpQ", app).value = ""; loadRpThreads(app); });

  const hasLogin = sources.length > 0 && sources[0].has_login;
  const hasTVForum = hasLogin && sources[0].forums?.some(f => f.name.includes("电视剧"));
  if (state.rpQuery) {
    await doSearch();
  } else if (hasTVForum) {
    // 登录态下后端会按优先级自动选择电视剧版块。
    await loadRpThreads(app);
  } else if (state.rpDefaultQ) {
    // 游客态：默认搜索电视剧帖子
    $("#rpQ", app).value = state.rpDefaultQ;
    await doSearch();
  } else {
    await loadRpThreads(app);
  }
}

let rpLock = false;
async function loadRpThreads(app) {
  const grid = $("#rpGrid", app);
  grid.innerHTML = `<div class="empty" style="grid-column:1/-1">抓取论坛最新资源中…</div>`;
  if (rpLock) return; rpLock = true;
  try {
    const data = await api(`/respool/list?source=${state.rpSource}&page=${state.rpPage || 1}&category=${encodeURIComponent(state.rpCat || "全部")}`);
    if (data.total_pages > 0 && data.page > data.total_pages) {
      state.rpPage = data.total_pages;
      rpLock = false;
      return loadRpThreads(app);
    }
    state.rpCache = data.list || [];
    const status = $("#rpStatus", app);
    if (status) status.textContent = `· 前 50 条 · 本页 ${state.rpCache.length} 帖 · 第 ${data.page} / ${data.total_pages} 页${state.rpCat && state.rpCat !== "全部" ? " · " + state.rpCat : ""}`;
    renderRpGrid(app, state.rpCache);
    renderPager($("#rpPager", app),
      { total: data.total, page_size: data.page_size, total_pages: data.total_pages, page: data.page },
      state.rpPage, (pg) => { state.rpPage = pg; loadRpThreads(app); });
  } catch (e) {
    grid.innerHTML = `<div class="empty" style="grid-column:1/-1">${icon("alert", 36)}<br>${esc(e.message)}<br><span style="font-size:12px">论坛源不可达时可稍后刷新重试</span></div>`;
  } finally { rpLock = false; }
}

function renderRpGrid(app, threads) {
  const grid = $("#rpGrid", app);
  const kw = (state.rpQuery || "").toLowerCase();
  const list = kw ? threads.filter(t => t.title.toLowerCase().includes(kw)) : threads;
  if (!list.length) {
    grid.innerHTML = `<div class="empty" style="grid-column:1/-1">${icon("search", 36)}<br>没有匹配的帖子</div>`;
    return;
  }
  grid.innerHTML = list.map(t => `
    <div class="res-card" data-cover-tid="${esc(t.id)}">
      <div class="rcover">
        <div class="rcover-placeholder">${icon("image", 24)}</div>
        <img alt="${esc(t.title)} 海报" loading="lazy" referrerpolicy="no-referrer">
      </div>
      <div class="rcontent">
        <div class="rtitle">${esc(t.title)}</div>
        <div class="rmeta"><span class="tag ${t.category === "电视剧" ? "green" : t.category === "电影" ? "orange" : ""}">${esc(t.category || "其他")}</span></div>
        <div class="raction">
          <button class="btn sm" data-tid="${esc(t.id)}" data-title="${esc(t.title)}" data-category="${esc(t.category || "其他")}">${icon("upload", 14)} 一键转存</button>
        </div>
      </div>
    </div>`).join("");
  $$("button[data-tid]", grid).forEach(b =>
    b.addEventListener("click", () => rpOneClick(b.dataset.tid, b.dataset.title, b.dataset.category, b)));
  loadRpCovers(grid);
}

/* 海报按 4 路小并发异步加载，不阻塞列表与翻页。 */
async function loadRpCovers(grid) {
  const cards = $$('[data-cover-tid]', grid);
  let cursor = 0;
  async function worker() {
    while (cursor < cards.length) {
      const card = cards[cursor++];
      try {
        const data = await api(`/respool/cover?source=${encodeURIComponent(state.rpSource)}&tid=${encodeURIComponent(card.dataset.coverTid)}`);
        if (!data.cover_url || !card.isConnected) continue;
        const img = $(".rcover img", card);
        img.addEventListener("load", () => card.classList.add("has-cover"), { once: true });
        img.src = data.cover_url;
      } catch { /* 单张海报失败不影响资源列表 */ }
    }
  }
  await Promise.all(Array.from({ length: Math.min(4, cards.length) }, worker));
}

/* 一键转存（蓝图链路）：探测/回帖解锁 → 提取分享链接 → 规划目录 → 网盘秒转 */
async function rpOneClick(tid, title, category, btn) {
  const old = btn.innerHTML;
  btn.disabled = true; btn.innerHTML = "解锁中…";
  try {
    // 标题里解析年份与类型线索
    const ym = title.match(/\((19|20)\d{2}\)/);
    const year = ym ? ym[0].slice(1, -1) : "";
    const mediaTypes = { "电视剧": "tv", "短剧": "tv", "电影": "movie", "综艺": "variety", "动漫": "anime", "纪录片": "documentary" };
    const isTV = /剧|集|第.季|S\d/i.test(title);
    const mediaType = mediaTypes[category] || (isTV ? "tv" : "movie");
    btn.innerHTML = "转存中…";
    const r = await api("/media/transfer", {
      method: "POST",
      body: JSON.stringify({ tid, source: state.rpSource, title: title.slice(0, 60), year, media_type: mediaType }),
    });
    toast(`✅ 已转存 ${r.saved_files} 个文件 → ${r.target_path}${r.already_unlocked ? "（免回帖秒提取）" : "（已自动回帖解锁）"}`);
    btn.innerHTML = "已转存";
  } catch (e) {
    toast(e.message, true);
    btn.disabled = false; btn.innerHTML = old;
  }
}

/* ---------- 转存记录 ---------- *//* ---------- 转存记录 ---------- *//* ---------- 转存记录 ---------- */
async function renderRecords(app) {
  app.innerHTML = `
  <div class="card">
    <div class="card-title">${icon("list", 18)} 转存记录 <span class="sub">成功与失败任务均会保留</span></div>
    <div class="table-wrap" id="taskTable"></div>
  </div>`;
  await loadTasks(app);
}

async function loadTasks(app) {
  const box = $("#taskTable", app);
  box.innerHTML = `<div class="empty">加载中…</div>`;
  try {
    const data = await api("/tasks?page=1&page_size=50");
    if (!data.list?.length) {
      box.innerHTML = `<div class="empty">${icon("inbox", 40)}<br>还没有转存记录，去工作台试试吧</div>`;
      return;
    }
    box.innerHTML = `
    <table class="tbl">
      <thead><tr><th>任务</th><th>网盘</th><th>文件数</th><th>大小</th><th>目标目录</th><th>状态</th><th>时间</th><th>操作</th></tr></thead>
      <tbody>
        ${data.list.map((t) => `
        <tr>
          <td><div style="max-width:260px;overflow:hidden;text-overflow:ellipsis;white-space:nowrap" title="${esc(t.share_name || t.share_url)}">${esc(t.share_name || t.share_url)}</div>
              <div class="mono">${esc(t.id)}</div></td>
          <td>${esc(t.provider)}</td>
          <td>${t.file_count}</td>
          <td>${fmtSize(t.total_size)}</td>
          <td style="font-size:12.5px">${esc(t.target_folder)}</td>
          <td><span class="status-dot ${t.status}"></span>${STATUS_TEXT[t.status] || t.status}${t.status === "running" ? ` ${t.progress}%` : ""}</td>
          <td style="font-size:12.5px;color:var(--text-3)">${fmtDate(t.created_at)}</td>
          <td><button class="btn danger-ghost sm" data-del="${esc(t.id)}">删除</button></td>
        </tr>`).join("")}
      </tbody>
    </table>`;
    $$("button[data-del]", box).forEach((b) =>
      b.addEventListener("click", async () => {
        if (!confirm("确定删除这条记录？")) return;
        try { await api(`/tasks/${b.dataset.del}`, { method: "DELETE" }); toast("已删除"); loadTasks(app); }
        catch (e) { toast(e.message, true); }
      })
    );
    // 有进行中的任务时自动刷新
    if (data.list.some((t) => ["queued", "running"].includes(t.status))) {
      setTimeout(() => { if (currentRoute() === "records") loadTasks(app); }, 1500);
    }
  } catch (e) {
    box.innerHTML = `<div class="empty">${icon("alert", 36)}${esc(e.message)}</div>`;
  }
}

/* ---------- 账号设置 ---------- */
const SETTINGS_TABS = [
  { id: "drive", label: "网盘配置", icon: "drive", sections: ["pan123"] },
  { id: "mail", label: "邮箱配置", icon: "mail", sections: ["gmail"] },
  { id: "cloudflare", label: "Cloudflare配置", icon: "activity", sections: ["cloudflare", "r2"] },
  { id: "library", label: "Z-lib配置", icon: "book", sections: ["library"] },
  { id: "forum", label: "资源站点", icon: "compass", sections: ["forum"] },
];

async function renderSettings(app) {
  document.querySelector("#fieldHelpTooltip")?.remove();
  let cfg;
  try {
    cfg = await api("/settings/integrations");
  } catch (e) {
    app.innerHTML = `<div class="card"><div class="empty">${icon("alert", 36)}<br>${esc(e.message)}</div></div>`;
    return;
  }
  const sections = [cfg.pan123, cfg.forum, cfg.gmail, cfg.cloudflare, cfg.r2, cfg.library];
  const configuredCount = sections.filter((item) => item?.configured).length;
  if (!SETTINGS_TABS.some((tab) => tab.id === state.settingsTab)) state.settingsTab = SETTINGS_TABS[0].id;
  const badge = (item) => `<span class="config-status ${item?.configured ? "configured" : "unconfigured"}"><i></i>${item?.configured ? "已配置" : "未配置"}</span>`;
  const secretField = (key, label, configured, placeholder = label) => `
    <div class="form-row secret-row">
      <label>${esc(label)} <small>${configured ? "已配置" : "未配置"}</small></label>
      <div class="secret-input">
        ${icon("key", 15)}
        <input class="input" type="password" autocomplete="new-password" data-config-key="${esc(key)}"
          data-sensitive="true" placeholder="${configured ? "************" : esc(`请输入${placeholder}`)}">
      </div>
    </div>`;
  const identityField = (key, label, value, placeholder) => `
    <div class="form-row">
      <label>${esc(label)} <small>${value ? "已配置" : "未配置"}</small></label>
      <input class="input" data-config-key="${esc(key)}" value="${esc(value || "")}" placeholder="${esc(placeholder)}">
    </div>`;
  const cardHead = (iconName, title, desc, item) => `
    <div class="config-card-head">
      <span class="config-card-icon">${icon(iconName, 20)}</span>
      <div class="config-card-title"><h2>${esc(title)}</h2><p>${esc(desc)}</p></div>
      ${badge(item)}
    </div>`;
  const cardFoot = (hint, extraAction = "") => `
    <div class="config-card-foot">
      <span class="config-test-message" data-test-message data-default="${esc(hint)}">${esc(hint)}</span>
      <div class="config-actions">
        ${extraAction}
        <button class="btn ghost sm" data-test-config>${icon("activity", 14)} 测试连接</button>
        <button class="btn sm" data-save-config disabled>${icon("refresh", 14)} 保存配置</button>
      </div>
    </div>`;

  app.innerHTML = `
  <section class="settings-hero">
    <span class="settings-hero-icon">${icon("settings", 28)}</span>
    <div class="settings-hero-copy">
      <span class="settings-eyebrow">ACCOUNT &amp; SERVICES</span>
      <h1>账号与服务配置</h1>
      <p>集中管理资源中心使用的外部账号、访问凭据与存储服务。</p>
    </div>
    <div class="settings-progress"><strong>${configuredCount}</strong><span>/ ${sections.length} 项已配置</span></div>
  </section>
  <div class="settings-tabs" role="tablist" aria-label="配置分类">
    ${SETTINGS_TABS.map((tab) => `
      <button type="button" role="tab" id="settings-tab-${tab.id}" data-settings-tab="${tab.id}"
        aria-controls="settings-panel-${tab.id}" aria-selected="${tab.id === state.settingsTab}"
        tabindex="${tab.id === state.settingsTab ? "0" : "-1"}">
        ${icon(tab.icon, 17)}<span>${tab.label}</span>
        <span class="settings-tab-count">${tab.sections.filter((section) => cfg[section]?.configured).length}/${tab.sections.length}</span>
      </button>`).join("")}
  </div>
  <div class="settings-panel" role="tabpanel" id="settings-panel-drive" aria-labelledby="settings-tab-drive" data-settings-panel="drive" ${state.settingsTab === "drive" ? "" : "hidden"}>
    <div class="card config-card" data-config-section="pan123">
      ${cardHead("drive", "123云盘", "浏览网盘目录并执行一键转存", cfg.pan123)}
      <div class="config-fields">
        ${identityField("account", "账号", cfg.pan123.account, "手机号或邮箱")}
        ${secretField("password", "密码", cfg.pan123.password_configured)}
      </div>
      ${cardFoot("保存后自动登录获取 Token，失效时自动续期，无需手动维护")}
    </div>
  </div>
  <div class="settings-panel" role="tabpanel" id="settings-panel-forum" aria-labelledby="settings-tab-forum" data-settings-panel="forum" ${state.settingsTab === "forum" ? "" : "hidden"}>
    <div class="card config-card" data-config-section="forum">
      ${cardHead("compass", "资源论坛", "检索 pan1.me、123panfx 等资源站点", cfg.forum)}
      <div class="config-fields">
        ${identityField("account", "账号", cfg.forum.account, "论坛登录账号")}
        ${secretField("password", "密码", cfg.forum.password_configured)}
        <div class="forum-session ${cfg.forum.has_login ? "connected" : "disconnected"}">
          <i></i>
          <div>
            <strong>${cfg.forum.has_login ? "论坛已连接" : "尚未获取登录会话"}</strong>
            <span>${cfg.forum.has_login ? "Cookie 已由系统自动保存，后续访问无需重复登录" : "保存账号密码后，点击连接论坛并完成一次滑块验证"}</span>
          </div>
        </div>
      </div>
      ${cardFoot(cfg.forum.has_login ? "登录状态正常，Cookie 将自动复用" : "Cookie 无需手动填写，登录成功后自动保存", `<button class="btn ghost sm" data-forum-login>${icon("key", 14)} ${cfg.forum.has_login ? "重新登录" : "连接论坛"}</button>`)}
    </div>
  </div>
  <div class="settings-panel" role="tabpanel" id="settings-panel-mail" aria-labelledby="settings-tab-mail" data-settings-panel="mail" ${state.settingsTab === "mail" ? "" : "hidden"}>
    <div class="card config-card" data-config-section="gmail">
      ${cardHead("mail", "Gmail 邮箱", "浏览收件箱、搜索邮件与下载附件", cfg.gmail)}
      <div class="config-fields two-col">
        ${identityField("email", "邮箱", cfg.gmail.email, "name@gmail.com")}
        <div class="form-row"><label>Client ID</label><input class="input" data-config-key="client_id" value="${esc(cfg.gmail.client_id || "")}" placeholder="OAuth Client ID"></div>
        ${secretField("client_secret", "Client Secret", cfg.gmail.client_secret_configured)}
      </div>
      ${cardFoot("使用 Google OAuth 只读授权", `<button class="btn ghost sm" data-gmail-oauth>${icon("key", 14)} 授权 Gmail</button>`)}
    </div>
  </div>
  <div class="settings-panel" role="tabpanel" id="settings-panel-cloudflare" aria-labelledby="settings-tab-cloudflare" data-settings-panel="cloudflare" ${state.settingsTab === "cloudflare" ? "" : "hidden"}>
    <div class="card config-card" data-config-section="cloudflare">
      ${cardHead("activity", "Cloudflare", "监测域名、Workers 与账户用量", cfg.cloudflare)}
      <div class="config-fields">
        <div class="form-row"><label>Account ID</label><input class="input" data-config-key="account_id" value="${esc(cfg.cloudflare.account_id || "")}" placeholder="Cloudflare Account ID"></div>
        ${secretField("api_token", "API Token", cfg.cloudflare.api_token_configured)}
      </div>
      ${cardFoot("建议使用只读权限 Token")}
    </div>

    <div class="card config-card" data-config-section="r2">
      ${cardHead("database", "Cloudflare R2", "复用左侧 Cloudflare Account ID，作为书库文件的对象存储目标", cfg.r2)}
      <div class="config-fields two-col">
        ${secretField("access_key_id", "Access Key ID", cfg.r2.access_key_configured)}
        ${secretField("secret_access_key", "Secret Access Key", cfg.r2.secret_key_configured)}
        <div class="form-row">
          <div class="bucket-field-head">
            <div class="bucket-label-help">
              <label for="r2Bucket">Bucket</label>
              <button type="button" class="field-help" data-bucket-help aria-label="Bucket 填写说明"
                data-help="可从建议列表选择，也可直接输入或修改 Bucket 名称">${icon("helpCircle", 14)}</button>
            </div>
            <button type="button" data-load-r2-buckets>刷新列表</button>
          </div>
          <input class="input" id="r2Bucket" list="r2BucketOptions" data-config-key="bucket"
            value="${esc(cfg.r2.bucket || "")}" placeholder="选择或输入 Bucket 名称" autocomplete="off">
          <datalist id="r2BucketOptions">
            ${cfg.r2.bucket ? `<option value="${esc(cfg.r2.bucket)}"></option>` : ""}
          </datalist>
          <small class="bucket-field-hint" data-r2-bucket-status></small>
        </div>
        <div class="form-row"><label>公开地址 <small>可选</small></label><input class="input" data-config-key="public_base" value="${esc(cfg.r2.public_base || "")}" placeholder="https://files.example.com"></div>
      </div>
      ${cardFoot("未配置时使用本地存储")}
    </div>
  </div>
  <div class="settings-panel" role="tabpanel" id="settings-panel-library" aria-labelledby="settings-tab-library" data-settings-panel="library" ${state.settingsTab === "library" ? "" : "hidden"}>
    <div class="card config-card" data-config-section="library">
      ${cardHead("book", "书库", "配置搜索入口与文件获取模式", cfg.library)}
      <div class="config-fields two-col">
        <div class="form-row"><label>模式</label><select class="select" data-config-key="mode">
          ${["jump", "direct"].map((mode) => `<option value="${mode}" ${mode === (cfg.library.mode || "jump") ? "selected" : ""}>${mode}</option>`).join("")}
        </select></div>
        <div class="form-row"><label>站点地址</label><input class="input" data-config-key="base_url" value="${esc(cfg.library.base_url || "")}" placeholder="https://..."></div>
        ${identityField("email", "邮箱", cfg.library.email, "direct 模式使用")}
        ${secretField("password", "密码", cfg.library.password_configured)}
      </div>
      ${cardFoot("jump 模式无需填写书库账号")}
    </div>
  </div>
  `;

  const activateSettingsTab = (id, focus = false) => {
    state.settingsTab = id;
    $$('[data-settings-tab]', app).forEach((tab) => {
      const selected = tab.dataset.settingsTab === id;
      tab.setAttribute("aria-selected", String(selected));
      tab.tabIndex = selected ? 0 : -1;
      if (selected && focus) tab.focus();
    });
    $$('[data-settings-panel]', app).forEach((panel) => {
      panel.hidden = panel.dataset.settingsPanel !== id;
    });
    if (id === "cloudflare") loadR2Buckets();
  };
  const settingsTabs = $$('[data-settings-tab]', app);
  settingsTabs.forEach((tab, index) => {
    tab.addEventListener("click", () => activateSettingsTab(tab.dataset.settingsTab));
    tab.addEventListener("keydown", (event) => {
      const next = event.key === "ArrowRight" ? (index + 1) % settingsTabs.length
        : event.key === "ArrowLeft" ? (index - 1 + settingsTabs.length) % settingsTabs.length
        : event.key === "Home" ? 0 : event.key === "End" ? settingsTabs.length - 1 : -1;
      if (next < 0) return;
      event.preventDefault();
      activateSettingsTab(settingsTabs[next].dataset.settingsTab, true);
    });
  });

  const collectConfigValues = (card) => {
    const values = {};
    $$('[data-config-key]', card).forEach((input) => {
      const value = input.value.trim();
      if (value) values[input.dataset.configKey] = value;
    });
    return values;
  };

  let r2BucketsRequested = false;
  const loadR2Buckets = async (force = false) => {
    if (r2BucketsRequested && !force) return;
    r2BucketsRequested = true;
    const card = $('[data-config-section="r2"]', app);
    const button = $('[data-load-r2-buckets]', card);
    const status = $('[data-r2-bucket-status]', card);
    const input = $('#r2Bucket', card);
    const optionsList = $('#r2BucketOptions', card);
    const selected = input.value.trim();
    button.disabled = true;
    status.textContent = "正在读取 Bucket 列表…";
    try {
      const result = await api("/settings/r2/buckets", {
        method: "POST",
        body: JSON.stringify({ values: collectConfigValues(card) }),
      });
      const buckets = result.buckets || [];
      const options = [...new Set([...buckets, ...(selected ? [selected] : [])])];
      optionsList.innerHTML = options.map((name) => `<option value="${esc(name)}"></option>`).join("");
      input.value = selected || buckets[0] || "";
      status.textContent = result.buckets?.length
        ? ""
        : "当前账户没有可用的 Bucket，可直接输入名称";
    } catch (e) {
      status.textContent = `无法自动读取：${e.message}；可手动填写`;
    } finally {
      button.disabled = false;
    }
  };
  $('[data-load-r2-buckets]', app).addEventListener("click", () => loadR2Buckets(true));
  const bucketHelpButton = $('[data-bucket-help]', app);
  const hideBucketHelp = () => document.querySelector("#fieldHelpTooltip")?.remove();
  const showBucketHelp = () => {
    hideBucketHelp();
    const tooltip = document.createElement("div");
    tooltip.id = "fieldHelpTooltip";
    tooltip.className = "field-help-tooltip";
    tooltip.setAttribute("role", "tooltip");
    tooltip.textContent = bucketHelpButton.dataset.help;
    document.body.appendChild(tooltip);

    const buttonRect = bucketHelpButton.getBoundingClientRect();
    const tooltipRect = tooltip.getBoundingClientRect();
    let top = buttonRect.top - tooltipRect.height - 9;
    let placement = "top";
    if (top < 8) {
      top = buttonRect.bottom + 9;
      placement = "bottom";
    }
    const left = Math.min(
      Math.max(8, buttonRect.left + buttonRect.width / 2 - tooltipRect.width / 2),
      window.innerWidth - tooltipRect.width - 8,
    );
    tooltip.dataset.placement = placement;
    tooltip.style.top = `${Math.round(top)}px`;
    tooltip.style.left = `${Math.round(left)}px`;
  };
  bucketHelpButton.addEventListener("mouseenter", showBucketHelp);
  bucketHelpButton.addEventListener("mouseleave", hideBucketHelp);
  bucketHelpButton.addEventListener("focus", showBucketHelp);
  bucketHelpButton.addEventListener("blur", hideBucketHelp);
  if (state.settingsTab === "cloudflare") loadR2Buckets();

  const invalidateTest = (card) => {
    const save = $('[data-save-config]', card);
    const test = $('[data-test-config]', card);
    const message = $('[data-test-message]', card);
    save.disabled = true;
    save.innerHTML = `${icon("refresh", 14)} 保存配置`;
    test.classList.remove("passed", "failed");
    test.innerHTML = `${icon("activity", 14)} 测试连接`;
    message.className = "config-test-message";
    message.textContent = message.dataset.default;
  };

  $$('[data-config-key]', app).forEach((input) =>
    input.addEventListener("input", () => invalidateTest(input.closest("[data-config-section]")))
  );
  $$('select[data-config-key]', app).forEach((input) =>
    input.addEventListener("change", () => invalidateTest(input.closest("[data-config-section]")))
  );

  $$('[data-test-config]', app).forEach((button) =>
    button.addEventListener("click", async () => {
      const card = button.closest("[data-config-section]");
      const save = $('[data-save-config]', card);
      const message = $('[data-test-message]', card);
      const old = button.innerHTML;
      const testedValues = collectConfigValues(card);
      const testedSnapshot = JSON.stringify(testedValues);
      button.disabled = true;
      save.disabled = true;
      button.innerHTML = `${icon("refresh", 14)} 测试中…`;
      message.className = "config-test-message testing";
      message.textContent = "正在验证连接与凭据…";
      try {
        const result = await api("/settings/integrations/test", {
          method: "POST",
          body: JSON.stringify({ section: card.dataset.configSection, values: testedValues }),
        });
        if (JSON.stringify(collectConfigValues(card)) !== testedSnapshot) {
          invalidateTest(card);
          message.className = "config-test-message failed";
          message.textContent = "配置已变更，请重新测试";
          return;
        }
        button.classList.remove("failed");
        button.classList.add("passed");
        button.innerHTML = `${icon("activity", 14)} 测试通过`;
        message.className = "config-test-message passed";
        message.textContent = result.detail || "连接测试通过";
        save.disabled = false;
      } catch (e) {
        button.classList.remove("passed");
        button.classList.add("failed");
        button.innerHTML = `${icon("alert", 14)} 重新测试`;
        message.className = "config-test-message failed";
        message.textContent = e.message;
      } finally {
        button.disabled = false;
        if (!button.classList.contains("passed") && !button.classList.contains("failed")) button.innerHTML = old;
      }
    })
  );

  $$('[data-save-config]', app).forEach((button) =>
    button.addEventListener("click", async () => {
      const card = button.closest("[data-config-section]");
      const values = collectConfigValues(card);
      const old = button.textContent;
      button.disabled = true;
      button.textContent = "保存中…";
      try {
        await api("/settings/integrations", {
          method: "POST",
          body: JSON.stringify({ section: card.dataset.configSection, values }),
        });
        toast("配置已保存并立即生效");
        button.textContent = "已保存并生效";
      } catch (e) {
        toast(e.message, true);
        button.textContent = old;
      } finally {
        // 保存后或保存失败都要求重新测试，避免复用过期的测试结果。
        if (button.textContent !== "已保存并生效") button.textContent = old;
        button.disabled = true;
      }
    })
  );

  const gmailOAuthButton = $('[data-gmail-oauth]', app);
  if (gmailOAuthButton) {
    gmailOAuthButton.addEventListener("click", async () => {
      const card = gmailOAuthButton.closest('[data-config-section="gmail"]');
      const values = collectConfigValues(card);
      const popup = window.open("about:blank", "gmailOAuth", "popup,width=560,height=720");
      if (!popup) {
        toast("浏览器阻止了授权窗口，请允许弹窗后重试", true);
        return;
      }
      popup.document.write("<p style='font:14px system-ui;padding:24px'>正在准备 Gmail 授权…</p>");
      const old = gmailOAuthButton.innerHTML;
      gmailOAuthButton.disabled = true;
      gmailOAuthButton.innerHTML = `${icon("refresh", 14)} 等待授权…`;

      let closedTimer;
      const cleanup = () => {
        window.removeEventListener("message", onOAuthMessage);
        if (closedTimer) clearInterval(closedTimer);
        gmailOAuthButton.disabled = false;
        gmailOAuthButton.innerHTML = old;
      };
      const onOAuthMessage = async (event) => {
        if (event.origin !== window.location.origin || !event.data?.type?.startsWith("gmail-oauth-")) return;
        cleanup();
        if (event.data.type === "gmail-oauth-success") {
          toast(event.data.message || "Gmail 授权成功，配置已立即生效");
          await renderSettings(app);
        } else {
          toast(event.data.message || "Gmail 授权失败", true);
        }
      };
      window.addEventListener("message", onOAuthMessage);
      closedTimer = setInterval(() => {
        if (popup.closed) cleanup();
      }, 500);

      try {
        const result = await api("/settings/gmail/oauth/start", {
          method: "POST",
          body: JSON.stringify({
            client_id: values.client_id || "",
            client_secret: values.client_secret || "",
            email: values.email || "",
          }),
        });
        popup.location.replace(result.auth_url);
      } catch (e) {
        popup.close();
        cleanup();
        toast(e.message, true);
      }
    });
  }

  const forumLoginButton = $('[data-forum-login]', app);
  if (forumLoginButton) {
    forumLoginButton.addEventListener("click", () => {
      state.settingsTab = "forum";
      showForumLoginModal(() => renderSettings(app));
    });
  }
}

/* ---------- 启动 ---------- */
(async function init() {
  try {
    state.user = await api("/auth/me");
    renderUserBox();
  } catch { /* 未登录时后端已 302 到 /login，这里不阻塞 */ }
  try {
    const cfg = await api("/config");
    state.providers = cfg.providers || [];
    const h = await api("/health");
    startUptimeClock(h.uptime_seconds || 0);
  } catch { /* 顶栏统计失败不阻塞渲染 */ }
  render();
})();

/* ---------- 顶栏运行时长 ---------- */
function fmtUptime(sec) {
  if (sec < 60) return `${sec} 秒`;
  const m = Math.floor(sec / 60);
  if (m < 60) return `${m} 分钟`;
  const h = Math.floor(m / 60);
  if (h < 24) return `${h} 小时 ${m % 60} 分`;
  const d = Math.floor(h / 24);
  return `${d} 天 ${h % 24} 小时`;
}

function startUptimeClock(baseSec) {
  const bootAt = Date.now() - baseSec * 1000; // 对齐服务端启动时刻
  const tick = () => {
    const el = $("#topStats");
    if (!el) return;
    el.textContent = `⏱ 已运行 ${fmtUptime(Math.floor((Date.now() - bootAt) / 1000))}`;
  };
  tick();
  clearInterval(window.__uptimeTimer);
  window.__uptimeTimer = setInterval(tick, 1000);
}

/* ---------- 顶栏用户区 ---------- */
function renderUserBox() {
  const box = $("#userBox");
  if (!box || !state.user) return;
  box.innerHTML = `
    <span class="user-name">${icon("user", 14)} ${esc(state.user.username)}</span>
    <button class="btn danger-ghost sm" id="logoutBtn">退出</button>`;
  $("#logoutBtn").addEventListener("click", async () => {
    try { await api("/auth/logout", { method: "POST" }); } catch {}
    location.href = "/login";
  });
}
