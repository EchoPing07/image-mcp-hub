// ============================================================
// image-mcp-hub admin SPA (vanilla JS)
// i18n (zh-first) · light/dark theme · dashboard with stats
// Editorial style: warm paper + cinnabar, serif headings.
// ============================================================
"use strict";

const API = "/admin/api";
const LANG_KEY = "ims_lang";
const THEME_KEY = "ims_theme";

// ---------- i18n ----------

const I18N = {
  zh: {
    tab_dashboard: "统计", tab_models: "模型", tab_settings: "设置", tab_images: "图库",
    theme_l: "浅色模式", theme_d: "深色模式",
    theme_light: "切换到浅色模式", theme_dark: "切换到深色模式", logout: "退出登录",
    login_placeholder: "管理密码", login_btn: "登 录",
    err_password: "密码错误",
    refresh: "刷新",
    dash_title: "统计",
    stat_requests: "请求总数", stat_success: "成功", stat_failures: "失败",
    stat_rate: "成功率", stat_images: "生成图片",
    stat_avg: "平均耗时",
    stat_models_sub: "已配置 {n} 个模型",
    dash_chart: "近 30 天请求趋势", dash_models: "模型统计", dash_recent: "最近调用",
    fail_top: "失败原因 Top",
    dash_empty: "暂无统计数据，调用 MCP 工具后这里会展示图表",
    chart_tip: "{date} · {n} 次请求",
    col_model: "模型", col_requests: "请求", col_success: "成功", col_failures: "失败",
    col_rate: "成功率", col_avg: "平均耗时", col_images: "出图", col_last: "最近调用",
    ok: "成功", fail: "失败",
    models_title: "模型",
    models_empty: "暂无模型，添加一个即可将其暴露为 MCP 工具",
    key_count: "{n} 个 API Key",
    edit: "编辑", delete: "删除",
    del_model_title: "删除模型",
    del_model_msg: "确定删除模型「{name}」吗？此操作不可恢复。",
    saved: "已保存", deleted: "已删除",
    modal_add: "添加模型", modal_edit: "编辑模型",
    tool_name: "工具名称", name_hint: "需匹配 ^[a-zA-Z][a-zA-Z0-9_]{0,63}$，作为 MCP 工具名使用。",
    model_id: "模型 ID", base_url: "上游地址", api_keys: "API Keys",
    keys_hint: "每次调用按列表轮换（round-robin），游标持久化，重启后接续。",
    description: "工具描述", tpl_placeholder: "选择描述模板…",
    cancel: "取消", save: "保存", add_model: "添加模型",
    settings_title: "设置",
    server: "服务", listen_host: "监听地址", port: "端口",
    mcp_token: "MCP Token", admin_password: "管理密码",
    storage_title: "存储与清理", image_dir: "图片目录",
    max_age: "最大保留天数", max_count: "最大保留数量", save_settings: "保存设置",
    settings_hint: "端口与存储目录修改后需重启生效；Token、密码与清理规则即时生效。",
    cleanup_hint: "0 表示不启用该规则。两项均为 0 时图片永久保留；满足任一规则即删除（含元数据）。",
    images_title: "图片", images_empty: "暂无图片", no_prompt: "（无提示词）",
    delete_image: "删除图片", del_image_title: "删除图片",
    del_image_msg: "确定删除这张图片吗？",
    meta_model: "模型", meta_model_id: "模型 ID", meta_prompt: "提示词",
    meta_time: "时间", meta_params: "参数", meta_upstream: "上游信息", meta_file: "文件名",
    desc_templates: [
      "根据文本描述生成一张图片，并返回本地图片 URL。",
      "根据描述创建高质量图片，支持尺寸、质量与风格选项。",
      "根据提示词生成图片，可通过 size（如 1024x1024）与 n 控制输出。",
      "文生图工具，返回一个或多个本地图片 URL。",
    ],
  },
  en: {
    tab_dashboard: "Statistics", tab_models: "Models", tab_settings: "Settings", tab_images: "Gallery",
    theme_l: "Light", theme_d: "Dark",
    theme_light: "Switch to light mode", theme_dark: "Switch to dark mode", logout: "Log out",
    login_placeholder: "Password", login_btn: "Log in",
    err_password: "Wrong password",
    refresh: "Refresh",
    dash_title: "Statistics",
    stat_requests: "Total requests", stat_success: "Succeeded", stat_failures: "Failed",
    stat_rate: "Success rate", stat_images: "Images generated",
    stat_avg: "Avg latency",
    stat_models_sub: "{n} models configured",
    dash_chart: "Requests — last 30 days", dash_models: "Per-model stats", dash_recent: "Recent calls",
    fail_top: "Top failures",
    dash_empty: "No statistics yet. Charts appear here once MCP tools are called.",
    chart_tip: "{date} · {n} requests",
    col_model: "Model", col_requests: "Requests", col_success: "OK", col_failures: "Fail",
    col_rate: "Rate", col_avg: "Avg time", col_images: "Images", col_last: "Last call",
    ok: "OK", fail: "Fail",
    models_title: "Models",
    models_empty: "No models yet. Add one to expose it as an MCP tool.",
    key_count: "{n} API keys",
    edit: "Edit", delete: "Delete",
    del_model_title: "Delete model",
    del_model_msg: "Delete model \"{name}\"? This cannot be undone.",
    saved: "Saved", deleted: "Deleted",
    modal_add: "Add model", modal_edit: "Edit model",
    tool_name: "Tool name", name_hint: "Must match ^[a-zA-Z][a-zA-Z0-9_]{0,63}$. Used as the MCP tool name.",
    model_id: "Model ID", base_url: "Base URL", api_keys: "API keys",
    keys_hint: "Rotated round-robin per call; cursor persisted across restarts.",
    description: "Description", tpl_placeholder: "Apply template…",
    cancel: "Cancel", save: "Save", add_model: "Add model",
    settings_title: "Settings",
    server: "Server", listen_host: "Listen host", port: "Port",
    mcp_token: "MCP token", admin_password: "Admin password",
    storage_title: "Storage & cleanup", image_dir: "Image directory",
    max_age: "Max age (days)", max_count: "Max count", save_settings: "Save settings",
    settings_hint: "Port and storage dir changes apply after restart. Token, password and cleanup rules apply immediately.",
    cleanup_hint: "0 disables a rule. Both 0 keeps images forever. Files exceeding either rule are deleted (image + sidecar meta).",
    images_title: "Images", images_empty: "No images yet", no_prompt: "(no prompt)",
    delete_image: "Delete image", del_image_title: "Delete image",
    del_image_msg: "Delete this image?",
    meta_model: "Model", meta_model_id: "Model ID", meta_prompt: "Prompt",
    meta_time: "Time", meta_params: "Params", meta_upstream: "Upstream", meta_file: "File",
    desc_templates: [
      "Generate an image from a text prompt and return a local image URL.",
      "Create a high-quality image based on the description. Supports size, quality and style options.",
      "Generate images from a prompt. Pass size (e.g. 1024x1024) and n to control output.",
      "Text-to-image tool. Returns one or more local image URLs.",
    ],
  },
};

let lang = localStorage.getItem(LANG_KEY);
if (lang !== "zh" && lang !== "en") {
  lang = (navigator.language || "en").toLowerCase().startsWith("zh") ? "zh" : "en";
  localStorage.setItem(LANG_KEY, lang);
}

const t = (key, vars) => {
  let s = (I18N[lang] && I18N[lang][key]) || I18N.en[key] || key;
  if (vars) for (const [k, v] of Object.entries(vars)) s = s.replaceAll("{" + k + "}", String(v));
  return s;
};

const fmtTime = (iso) => new Date(iso).toLocaleString(lang === "zh" ? "zh-CN" : "en-US", { hour12: false });
const fmtShort = (iso) => new Date(iso).toLocaleString(lang === "zh" ? "zh-CN" : "en-US", { month: "2-digit", day: "2-digit", hour: "2-digit", minute: "2-digit", hour12: false });

// ---------- theme (light / dark only) ----------

let theme = localStorage.getItem(THEME_KEY);
if (theme !== "light" && theme !== "dark") {
  theme = window.matchMedia && window.matchMedia("(prefers-color-scheme: dark)").matches ? "dark" : "light";
}
function applyTheme() {
  document.documentElement.setAttribute("data-theme", theme);
  document.documentElement.lang = lang === "zh" ? "zh-CN" : "en";
}
function toggleTheme() {
  theme = theme === "dark" ? "light" : "dark";
  localStorage.setItem(THEME_KEY, theme);
  render();
}

// ---------- icons (inline SVG, no emoji) ----------

const ICONS = {
  dashboard: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linecap="round" stroke-linejoin="round"><rect x="3" y="3" width="7" height="7" rx="1"/><rect x="14" y="3" width="7" height="7" rx="1"/><rect x="14" y="14" width="7" height="7" rx="1"/><rect x="3" y="14" width="7" height="7" rx="1"/></svg>',
  models: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linecap="round" stroke-linejoin="round"><path d="M21 16V8a2 2 0 0 0-1-1.73l-7-4a2 2 0 0 0-2 0l-7 4A2 2 0 0 0 3 8v8a2 2 0 0 0 1 1.73l7 4a2 2 0 0 0 2 0l7-4A2 2 0 0 0 21 16z"/><polyline points="3.27 6.96 12 12.01 20.73 6.96"/><line x1="12" y1="22.08" x2="12" y2="12"/></svg>',
  settings: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linecap="round" stroke-linejoin="round"><line x1="4" y1="21" x2="4" y2="14"/><line x1="4" y1="10" x2="4" y2="3"/><line x1="12" y1="21" x2="12" y2="12"/><line x1="12" y1="8" x2="12" y2="3"/><line x1="20" y1="21" x2="20" y2="16"/><line x1="20" y1="12" x2="20" y2="3"/><line x1="1" y1="14" x2="7" y2="14"/><line x1="9" y1="8" x2="15" y2="8"/><line x1="17" y1="16" x2="23" y2="16"/></svg>',
  images: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linecap="round" stroke-linejoin="round"><rect x="3" y="3" width="18" height="18" rx="2"/><circle cx="8.5" cy="8.5" r="1.5"/><polyline points="21 15 16 10 5 21"/></svg>',
  sun: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="12" r="5"/><line x1="12" y1="1" x2="12" y2="3"/><line x1="12" y1="21" x2="12" y2="23"/><line x1="4.22" y1="4.22" x2="5.64" y2="5.64"/><line x1="18.36" y1="18.36" x2="19.78" y2="19.78"/><line x1="1" y1="12" x2="3" y2="12"/><line x1="21" y1="12" x2="23" y2="12"/><line x1="4.22" y1="19.78" x2="5.64" y2="18.36"/><line x1="18.36" y1="5.64" x2="19.78" y2="4.22"/></svg>',
  moon: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linecap="round" stroke-linejoin="round"><path d="M21 12.79A9 9 0 1 1 11.21 3 7 7 0 0 0 21 12.79z"/></svg>',
  globe: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="12" r="10"/><line x1="2" y1="12" x2="22" y2="12"/><path d="M12 2a15.3 15.3 0 0 1 4 10 15.3 15.3 0 0 1-4 10 15.3 15.3 0 0 1-4-10 15.3 15.3 0 0 1 4-10z"/></svg>',
  logout: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linecap="round" stroke-linejoin="round"><path d="M9 21H5a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h4"/><polyline points="16 17 21 12 16 7"/><line x1="21" y1="12" x2="9" y2="12"/></svg>',
  plus: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round"><line x1="12" y1="5" x2="12" y2="19"/><line x1="5" y1="12" x2="19" y2="12"/></svg>',
  edit: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linecap="round" stroke-linejoin="round"><path d="M12 20h9"/><path d="M16.5 3.5a2.121 2.121 0 0 1 3 3L7 19l-4 1 1-4L16.5 3.5z"/></svg>',
  trash: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linecap="round" stroke-linejoin="round"><polyline points="3 6 5 6 21 6"/><path d="M19 6v14a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V6m3 0V4a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v2"/></svg>',
  refresh: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linecap="round" stroke-linejoin="round"><polyline points="23 4 23 10 17 10"/><polyline points="1 20 1 14 7 14"/><path d="M3.51 9a9 9 0 0 1 14.85-3.36L23 10M1 14l4.64 4.36A9 9 0 0 0 20.49 15"/></svg>',
  close: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round"><line x1="18" y1="6" x2="6" y2="18"/><line x1="6" y1="6" x2="18" y2="18"/></svg>',
};

const icon = (name) => el("span", { class: "ic", html: ICONS[name] || "" });

// ---------- helpers ----------

const state = {
  authed: false,
  tab: "dashboard",
  models: [],
  config: null,
  images: [],
  stats: null,
};

const el = (tag, props = {}, children = []) => {
  const n = document.createElement(tag);
  for (const [k, v] of Object.entries(props)) {
    if (k === "class") n.className = v;
    else if (k === "html") n.innerHTML = v;
    else if (k.startsWith("on")) n.addEventListener(k.slice(2).toLowerCase(), v);
    else if (k === "style" && typeof v === "object") Object.assign(n.style, v);
    else n.setAttribute(k, v);
  }
  for (const c of [].concat(children)) {
    if (c == null || c === false) continue;
    if (c && c.nodeType) n.appendChild(c);
    else n.appendChild(document.createTextNode(String(c)));
  }
  return n;
};

const api = async (path, opts = {}) => {
  const ctrl = new AbortController();
  const timer = setTimeout(() => ctrl.abort(), 15000);
  let r;
  try {
    r = await fetch(API + path, {
      headers: { "Content-Type": "application/json" },
      signal: ctrl.signal,
      ...opts,
    });
  } catch (e) {
    clearTimeout(timer);
    throw new Error("network error");
  }
  clearTimeout(timer);
  if (r.status === 401) { state.authed = false; render(); throw new Error("unauthorized"); }
  const text = await r.text();
  const data = text ? JSON.parse(text) : null;
  if (!r.ok) throw new Error((data && data.error) || r.statusText);
  return data;
};

let toastTimer = null;
const toast = (msg) => {
  document.querySelectorAll(".toast").forEach((n) => n.remove());
  const tEl = el("div", { class: "toast" }, [msg]);
  document.body.appendChild(tEl);
  clearTimeout(toastTimer);
  toastTimer = setTimeout(() => tEl.remove(), 2400);
};

const escapeHtml = (s) =>
  String(s ?? "").replace(/[&<>"']/g, (c) => ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;" }[c]));

function openModal(node) {
  const overlay = el("div", { class: "overlay" });
  overlay.appendChild(node);
  overlay.addEventListener("mousedown", (e) => { if (e.target === overlay) overlay.remove(); });
  document.getElementById("app").appendChild(overlay);
  return overlay;
}

function askConfirm(title, message, okLabel) {
  return new Promise((resolve) => {
    const overlay = openModal(el("div", { class: "modal modal-sm" }));
    const modal = overlay.querySelector(".modal");
    const close = () => { overlay.remove(); resolve(false); };
    modal.appendChild(el("div", { class: "modal-head" }, [
      el("h3", {}, [title]),
      el("button", { class: "close-x" }, [icon("close")]),
    ]));
    modal.querySelector(".close-x").addEventListener("click", close);
    const body = el("div", { class: "modal-body" });
    body.appendChild(el("p", { class: "confirm-msg" }, [message]));
    const actions = el("div", { class: "form-actions" });
    const cancel = el("button", { class: "btn" }, [t("cancel")]);
    cancel.addEventListener("click", close);
    const ok = el("button", { class: "btn danger" }, [okLabel || t("delete")]);
    ok.addEventListener("click", () => { overlay.remove(); resolve(true); });
    actions.appendChild(cancel); actions.appendChild(ok);
    body.appendChild(actions);
    modal.appendChild(body);
  });
}

// ---------- render ----------

function render() {
  applyTheme();
  const app = document.getElementById("app");
  app.innerHTML = "";
  if (!state.authed) { app.appendChild(renderLogin()); return; }
  const layout = el("div", { class: "layout" });
  layout.appendChild(renderSidebar());
  const main = el("div", { class: "main" });
  const c = el("div", { class: "container" });
  if (state.tab === "dashboard") c.appendChild(renderDashboard());
  else if (state.tab === "models") c.appendChild(renderModels());
  else if (state.tab === "settings") c.appendChild(renderSettings());
  else if (state.tab === "images") c.appendChild(renderImages());
  main.appendChild(c);
  layout.appendChild(main);
  app.appendChild(layout);
}

function renderLogin() {
  const wrap = el("div", { class: "login-wrap" });
  const card = el("div", { class: "login-card" });
  const input = el("input", { type: "password", placeholder: t("login_placeholder"), autofocus: "true" });
  input.addEventListener("keydown", (e) => { if (e.key === "Enter") doLogin(input.value); });
  card.appendChild(input);
  input.focus();
  const btn = el("button", { class: "btn primary login-btn" }, [t("login_btn")]);
  btn.addEventListener("click", () => doLogin(input.value));
  card.appendChild(btn);
  wrap.appendChild(card);
  return wrap;
}

async function doLogin(pw) {
  try {
    await api("/login", { method: "POST", body: JSON.stringify({ password: pw }) });
    state.authed = true;
    await loadAll();
    await loadStats();
    render();
  } catch (e) {
    toast(t("err_password"));
  }
}

function renderSidebar() {
  const sb = el("div", { class: "sidebar" });
  sb.appendChild(el("div", { class: "side-brand" }, [
    el("span", { class: "brand-dot" }),
    el("span", { class: "wordmark" }, ["image-mcp-hub"]),
  ]));
  const nav = el("div", { class: "side-nav" });
  for (const [id, label, svg] of [
    ["dashboard", t("tab_dashboard"), ICONS.dashboard],
    ["models", t("tab_models"), ICONS.models],
    ["images", t("tab_images"), ICONS.images],
    ["settings", t("tab_settings"), ICONS.settings],
  ]) {
    const item = el("button", { class: "nav-item" + (state.tab === id ? " active" : "") }, [
      el("span", { class: "ic", html: svg }),
      el("span", { class: "nav-label" }, [label]),
    ]);
    item.addEventListener("click", () => {
      state.tab = id;
      if (id === "dashboard") loadStats();
      else if (id === "images") loadImages();
      render();
    });
    nav.appendChild(item);
  }
  sb.appendChild(nav);

  const foot = el("div", { class: "side-foot" });
  const langBtn = el("button", { class: "nav-item" }, [icon("globe"), el("span", { class: "nav-label" }, [lang === "zh" ? "English" : "中文"])]);
  langBtn.addEventListener("click", () => {
    lang = lang === "zh" ? "en" : "zh";
    localStorage.setItem(LANG_KEY, lang);
    render();
  });
  const themeBtn = el("button", { class: "nav-item", title: theme === "dark" ? t("theme_light") : t("theme_dark") }, [
    icon(theme === "dark" ? "sun" : "moon"),
    el("span", { class: "nav-label" }, [theme === "dark" ? t("theme_l") : t("theme_d")]),
  ]);
  themeBtn.addEventListener("click", toggleTheme);
  const out = el("button", { class: "nav-item", title: t("logout") }, [icon("logout"), el("span", { class: "nav-label" }, [t("logout")])]);
  out.addEventListener("click", async () => {
    try { await api("/logout", { method: "POST" }); } catch (e) {}
    state.authed = false;
    render();
  });
  foot.appendChild(langBtn); foot.appendChild(themeBtn); foot.appendChild(out);
  sb.appendChild(foot);
  return sb;
}

// ---------- dashboard ----------

function pageHead(title, sub, actions) {
  const head = el("div", { class: "page-head" });
  const text = el("div", { class: "page-head-text" });
  text.appendChild(el("h1", { class: "page-title" }, [title]));
  if (sub) text.appendChild(el("p", { class: "page-sub" }, [sub]));
  head.appendChild(text);
  if (actions) head.appendChild(actions);
  return head;
}

function statCard(label, value, sub, tone) {
  const c = el("div", { class: "stat" + (tone ? " " + tone : "") });
  c.appendChild(el("div", { class: "label" }, [label]));
  c.appendChild(el("div", { class: "value" }, [value]));
  if (sub) c.appendChild(el("div", { class: "sub" }, [sub]));
  return c;
}

function renderDashboard() {
  const s = state.stats;
  const wrap = el("div", {});

  const refresh = el("button", { class: "btn sm" }, [icon("refresh"), t("refresh")]);
  refresh.addEventListener("click", loadStats);
  wrap.appendChild(pageHead(t("dash_title"), "", refresh));

  const total = s ? s.total_requests : 0;
  const rate = total ? Math.round((s.total_success / total) * 100) : 0;
  const avgMs = total ? Math.round((s.total_ms || 0) / total) : 0;
  const avgText = avgMs >= 1000 ? (avgMs / 1000).toFixed(1) + " s" : avgMs + " ms";

  const stats = el("div", { class: "stats" });
  stats.appendChild(statCard(t("stat_requests"), total, t("stat_models_sub", { n: state.models.length })));
  stats.appendChild(statCard(t("stat_success"), s ? s.total_success : 0, null, "ok"));
  stats.appendChild(statCard(t("stat_failures"), s ? s.total_failures : 0, null, "danger"));
  stats.appendChild(statCard(t("stat_rate"), rate + "%", null));
  stats.appendChild(statCard(t("stat_images"), s ? s.total_images : 0, null));
  stats.appendChild(statCard(t("stat_avg"), total ? avgText : "—", null));
  wrap.appendChild(stats);

  if (!s || total === 0) {
    wrap.appendChild(el("div", { class: "section" }, [el("div", { class: "empty" }, [t("dash_empty")])]));
    return wrap;
  }

  const chartCard = el("div", { class: "section" });
  chartCard.appendChild(el("div", { class: "section-head" }, [el("div", { class: "section-title" }, [t("dash_chart")])]));
  chartCard.appendChild(buildChart(s.daily));
  wrap.appendChild(chartCard);

  const modelCard = el("div", { class: "section" });
  modelCard.appendChild(el("div", { class: "section-head" }, [el("div", { class: "section-title" }, [t("dash_models")])]));
  modelCard.appendChild(buildModelTable(s.models, s.recent));
  wrap.appendChild(modelCard);

  const failTop = buildFailTop(s.recent);
  if (failTop) {
    const failCard = el("div", { class: "section" });
    failCard.appendChild(el("div", { class: "section-head" }, [el("div", { class: "section-title" }, [t("fail_top")])]));
    failCard.appendChild(failTop);
    wrap.appendChild(failCard);
  }

  const recentCard = el("div", { class: "section" });
  recentCard.appendChild(el("div", { class: "section-head" }, [el("div", { class: "section-title" }, [t("dash_recent")])]));
  recentCard.appendChild(buildRecent(s.recent));
  wrap.appendChild(recentCard);

  return wrap;
}

// "2026-07-05" -> "7/5" — short, no leading zeros
const fmtDay = (dateStr) => {
  const p = String(dateStr).split("-");
  return p.length === 3 ? String(+p[1]) + "/" + String(+p[2]) : String(dateStr);
};

function buildChart(daily) {
  const wrap = el("div", { class: "chart-wrap" });
  const max = Math.max(1, ...daily.map((d) => d.requests));
  // labels must fit inside a cell: narrow screens get fewer, wider spacing
  const step = (typeof window !== "undefined" && window.innerWidth < 700) ? 7 : 5;
  wrap.appendChild(el("div", { class: "chart-legend" }, [
    el("span", { class: "lg" }, [el("i", { class: "sw ok" }), t("ok")]),
    el("span", { class: "lg" }, [el("i", { class: "sw fail" }), t("fail")]),
  ]));
  const bars = el("div", { class: "chart" });
  daily.forEach((d, i) => {
    const pct = d.requests === 0 ? 3 : Math.max(8, Math.round((d.requests / max) * 100));
    const cell = el("div", { class: "bcell" });
    const bwrap = el("div", { class: "bwrap" });
    const col = el("div", {
      class: "bar" + (d.requests === 0 ? " zero" : ""),
      style: { height: pct + "%" },
      title: t("chart_tip", { date: d.date, n: d.requests }),
    });
    if (d.requests > 0) {
      const okShare = Math.min(100, Math.round((d.success / d.requests) * 100));
      if (d.failures) col.appendChild(el("div", { class: "seg fail", style: { height: (100 - okShare) + "%" } }));
      if (d.success) col.appendChild(el("div", { class: "seg ok", style: { height: okShare + "%" } }));
    }
    bwrap.appendChild(col);
    cell.appendChild(bwrap);
    cell.appendChild(el("span", { class: "bl" + (i % step === 0 ? "" : " hide") }, [fmtDay(d.date)]));
    bars.appendChild(cell);
  });
  wrap.appendChild(bars);
  return wrap;
}

function buildModelTable(models, recent) {
  const names = Object.keys(models);
  if (!names.length) return el("div", { class: "empty" }, [t("dash_empty")]);
  const wrap = el("div", { class: "table-wrap" });
  const table = el("table", { class: "table" });
  const thead = el("thead", {});
  const hr = el("tr", {});
  for (const h of [t("col_model"), t("col_requests"), t("col_success"), t("col_failures"), t("col_rate"), t("col_avg"), t("col_images"), t("col_last")]) {
    hr.appendChild(el("th", {}, [h]));
  }
  thead.appendChild(hr);
  const tbody = el("tbody", {});
  names.sort().forEach((name) => {
    const m = models[name];
    const r = m.requests ? Math.round((m.success / m.requests) * 100) : 0;
    const avg = m.requests ? Math.round(m.total_ms / m.requests) : 0;
    const tr = el("tr", {});
    tr.appendChild(el("td", { class: "cell-model" }, [name]));
    tr.appendChild(el("td", { class: "num" }, [m.requests]));
    tr.appendChild(el("td", { class: "num ok-text" }, [m.success]));
    tr.appendChild(el("td", { class: "num danger-text" }, [m.failures]));
    const rateCell = el("td", {});
    rateCell.appendChild(el("div", { class: "rate" }, [
      el("div", { class: "track" }, [el("div", { class: "fill", style: { width: r + "%" } })]),
      el("span", { class: "pct" }, [r + "%"]),
    ]));
    tr.appendChild(rateCell);
    tr.appendChild(el("td", { class: "num" }, [avg + " ms"]));
    tr.appendChild(el("td", { class: "num" }, [m.images]));
    tr.appendChild(el("td", { class: "cell-last" }, [lastCallFor(name, recent)]));
    tbody.appendChild(tr);
  });
  table.appendChild(thead); table.appendChild(tbody);
  wrap.appendChild(table);
  return wrap;
}

function lastCallFor(name, recent) {
  if (!recent) return "";
  for (const r of recent) if (r.model === name) return fmtShort(r.time);
  return "";
}

function buildFailTop(recent) {
  if (!recent || !recent.length) return null;
  const counts = new Map();
  for (const r of recent) {
    if (r.ok || !r.error) continue;
    const key = r.error.length > 64 ? r.error.slice(0, 64) : r.error;
    counts.set(key, (counts.get(key) || 0) + 1);
  }
  if (!counts.size) return null;
  const top = [...counts.entries()].sort((a, b) => b[1] - a[1]).slice(0, 5);
  const list = el("div", { class: "recent ftop" });
  top.forEach(([err, n]) => {
    const row = el("div", { class: "rrow" });
    row.appendChild(el("span", { class: "fcount num" }, [n]));
    row.appendChild(el("span", { class: "ferr", title: err }, [err]));
    list.appendChild(row);
  });
  return list;
}

function buildRecent(recent) {
  if (!recent || !recent.length) return el("div", { class: "empty" }, [t("dash_empty")]);
  const list = el("div", { class: "recent" });
  recent.forEach((r) => {
    const row = el("div", { class: "rrow" });
    row.appendChild(el("span", { class: "dot " + (r.ok ? "ok" : "fail") }));
    row.appendChild(el("span", { class: "rmodel" }, [r.model]));
    row.appendChild(el("span", { class: "rtime" }, [fmtTime(r.time)]));
    row.appendChild(el("span", { class: "rdur num" }, [r.duration_ms + " ms"]));
    row.appendChild(el("span", { class: "rstatus " + (r.ok ? "ok-text" : "danger-text") }, [r.ok ? t("ok") : t("fail")]));
    if (!r.ok && r.error) row.appendChild(el("span", { class: "rerr", title: r.error }, [r.error]));
    list.appendChild(row);
  });
  return list;
}

// ---------- models ----------

function renderModels() {
  const wrap = el("div", {});
  const add = el("button", { class: "btn primary sm" }, [icon("plus"), t("add_model")]);
  add.addEventListener("click", () => openModelModal(null));
  wrap.appendChild(pageHead(t("models_title"), null, add));

  const section = el("div", { class: "section" });
  if (!state.models.length) {
    section.appendChild(el("div", { class: "empty" }, [t("models_empty")]));
    wrap.appendChild(section);
    return wrap;
  }
  for (const m of state.models) {
    const row = el("div", { class: "list-row" });
    row.appendChild(el("div", { class: "name" }, [m.name]));
    row.appendChild(el("div", { class: "meta", title: m.base_url }, [
      `${m.model_id} · ${m.base_url} · ${t("key_count", { n: m.api_keys.length })}`,
    ]));
    const actions = el("div", { class: "actions" });
    const edit = el("button", { class: "btn sm" }, [icon("edit"), t("edit")]);
    edit.addEventListener("click", () => openModelModal(m));
    const del = el("button", { class: "btn sm danger" }, [icon("trash"), t("delete")]);
    del.addEventListener("click", () => deleteModel(m.name));
    actions.appendChild(edit); actions.appendChild(del);
    row.appendChild(actions);
    section.appendChild(row);
  }
  wrap.appendChild(section);
  return wrap;
}

function field(grid, label, control) {
  grid.appendChild(el("label", {}, [label]));
  grid.appendChild(control);
  return control;
}

function openModelModal(m) {
  const isNew = !m;
  const data = m
    ? { ...m, api_keys: m.api_keys.join("\n") }
    : { name: "", model_id: "", base_url: "", api_keys: "", description: "" };

  const overlay = openModal(el("div", { class: "modal" }));
  const modal = overlay.querySelector(".modal");
  const head = el("div", { class: "modal-head" });
  head.appendChild(el("h3", {}, [isNew ? t("modal_add") : t("modal_edit")]));
  const close = el("button", { class: "close-x" }, [icon("close")]);
  close.addEventListener("click", () => overlay.remove());
  head.appendChild(close);
  modal.appendChild(head);

  const body = el("div", { class: "modal-body" });
  const grid = el("div", { class: "form-grid" });

  const nameIn = field(grid, t("tool_name"), el("input", { value: data.name, placeholder: "my-image-model", autocomplete: "off" }));
  grid.appendChild(el("div", { class: "hint field-full" }, [t("name_hint")]));
  field(grid, t("model_id"), el("input", { value: data.model_id, placeholder: "gpt-image-1", autocomplete: "off" }));
  field(grid, t("base_url"), el("input", { value: data.base_url, placeholder: "https://api.openai.com/v1", autocomplete: "off" }));
  const keysTa = el("textarea", { placeholder: "sk-…" });
  keysTa.value = data.api_keys;
  field(grid, t("api_keys"), keysTa);
  grid.appendChild(el("div", { class: "hint field-full" }, [t("keys_hint")]));

  const descTa = el("textarea", { placeholder: "…" });
  descTa.value = data.description || "";
  const tplSel = el("select", {});
  tplSel.appendChild(el("option", { value: "" }, ["— " + t("tpl_placeholder") + " —"]));
  I18N[lang].desc_templates.forEach((tmpl, i) => {
    tplSel.appendChild(el("option", { value: String(i) }, [tmpl.slice(0, 42) + (tmpl.length > 42 ? "…" : "")]));
  });
  tplSel.addEventListener("change", () => { descTa.value = I18N[lang].desc_templates[+tplSel.value]; });
  const descWrap = el("div", {});
  descWrap.appendChild(tplSel);
  descWrap.appendChild(el("div", { style: { height: "8px" } }));
  descWrap.appendChild(descTa);
  field(grid, t("description"), descWrap);

  body.appendChild(grid);
  const actions = el("div", { class: "form-actions" });
  const cancel = el("button", { class: "btn" }, [t("cancel")]);
  cancel.addEventListener("click", () => overlay.remove());
  const save = el("button", { class: "btn primary" }, [t("save")]);
  save.addEventListener("click", () => {
    const inputs = modal.querySelectorAll(".form-grid input");
    saveModel(isNew, m && m.name, {
      name: inputs[0].value.trim(),
      model_id: inputs[1].value.trim(),
      base_url: inputs[2].value.trim(),
      api_keys: keysTa.value.split("\n").map((s) => s.trim()).filter(Boolean),
      description: descTa.value.trim(),
    }, overlay);
  });
  actions.appendChild(cancel); actions.appendChild(save);
  body.appendChild(actions);
  modal.appendChild(body);
  nameIn.focus();
}

async function saveModel(isNew, oldName, body, overlay) {
  try {
    if (isNew) await api("/models", { method: "POST", body: JSON.stringify(body) });
    else await api("/models/" + encodeURIComponent(oldName), { method: "PUT", body: JSON.stringify(body) });
    overlay.remove();
    await loadModels();
    render();
    toast(t("saved"));
  } catch (e) { toast(e.message); }
}

async function deleteModel(name) {
  if (!(await askConfirm(t("del_model_title"), t("del_model_msg", { name })))) return;
  try {
    await api("/models/" + encodeURIComponent(name), { method: "DELETE" });
    await loadModels();
    render();
    toast(t("deleted"));
  } catch (e) { toast(e.message); }
}

// ---------- settings ----------

function renderSettings() {
  const c = state.config;
  const wrap = el("div", {});
  wrap.appendChild(pageHead(t("settings_title"), null, null));

  const section = el("div", { class: "section" });
  section.appendChild(el("div", { class: "section-head" }, [el("div", { class: "section-title" }, [t("server")])]));
  const body = el("div", { class: "section-body" });
  const grid = el("div", { class: "form-grid" });
  const hostIn = field(grid, t("listen_host"), el("input", { value: c.server.host, placeholder: "0.0.0.0" }));
  const portIn = field(grid, t("port"), el("input", { type: "number", value: c.server.port, min: "0", max: "65535" }));
  const tokenIn = field(grid, t("mcp_token"), el("input", { value: c.server.mcp_token, placeholder: "sk-…" }));
  const pwIn = field(grid, t("admin_password"), el("input", { value: c.server.admin_password }));
  grid.appendChild(el("div", { class: "hint field-full" }, [t("settings_hint")]));
  body.appendChild(grid);
  section.appendChild(body);

  const section2 = el("div", { class: "section" });
  section2.appendChild(el("div", { class: "section-head" }, [el("div", { class: "section-title" }, [t("storage_title")])]));
  const body2 = el("div", { class: "section-body" });
  const grid2 = el("div", { class: "form-grid" });
  const dirIn = field(grid2, t("image_dir"), el("input", { value: c.storage.dir, placeholder: "./data/images" }));
  const ageIn = field(grid2, t("max_age"), el("input", { type: "number", value: c.storage.max_age_days, min: "0" }));
  const cntIn = field(grid2, t("max_count"), el("input", { type: "number", value: c.storage.max_count, min: "0" }));
  grid2.appendChild(el("div", { class: "hint field-full" }, [t("cleanup_hint")]));
  body2.appendChild(grid2);
  section2.appendChild(body2);

  const actions = el("div", { class: "form-actions" });
  const save = el("button", { class: "btn primary" }, [t("save_settings")]);
  save.addEventListener("click", async () => {
    try {
      const body = {
        server: { host: hostIn.value.trim(), port: +portIn.value, mcp_token: tokenIn.value, admin_password: pwIn.value },
        storage: { dir: dirIn.value.trim(), max_age_days: +ageIn.value || 0, max_count: +cntIn.value || 0 },
      };
      const res = await api("/config", { method: "PUT", body: JSON.stringify(body) });
      await loadConfig();
      render();
      toast(t("saved"));
      if (res && res.restart_required) setTimeout(() => toast(t("settings_hint")), 2600);
    } catch (e) { toast(e.message); }
  });
  actions.appendChild(save);
  body2.appendChild(actions);

  wrap.appendChild(section);
  wrap.appendChild(section2);
  return wrap;
}

// ---------- images ----------

function renderImages() {
  const wrap = el("div", {});
  const refresh = el("button", { class: "btn sm" }, [icon("refresh"), t("refresh")]);
  refresh.addEventListener("click", loadImages);
  wrap.appendChild(pageHead(t("images_title"), null, refresh));

  const section = el("div", { class: "section" });
  if (!state.images.length) {
    section.appendChild(el("div", { class: "empty" }, [t("images_empty")]));
    wrap.appendChild(section);
    return wrap;
  }
  const grid = el("div", { class: "grid" });
  for (const it of state.images) {
    const tile = el("div", { class: "tile" });
    const img = el("img", { src: it.url, loading: "lazy" });
    img.addEventListener("click", () => openImageModal(it));
    tile.appendChild(img);
    const info = el("div", { class: "info" });
    const prompt = (it.meta && it.meta.prompt) || "";
    info.appendChild(el("div", { class: "prompt" }, [escapeHtml(prompt) || t("no_prompt")]));
    info.appendChild(el("div", {}, [it.meta ? it.meta.model : ""]));
    tile.appendChild(info);
    grid.appendChild(tile);
  }
  section.appendChild(grid);
  wrap.appendChild(section);
  return wrap;
}

function openImageModal(it) {
  const overlay = openModal(el("div", { class: "modal" }));
  const modal = overlay.querySelector(".modal");
  const head = el("div", { class: "modal-head" });
  head.appendChild(el("h3", {}, [it.name]));
  const close = el("button", { class: "close-x" }, [icon("close")]);
  close.addEventListener("click", () => overlay.remove());
  head.appendChild(close);
  modal.appendChild(head);

  const body = el("div", { class: "modal-body" });
  const img = el("img", { src: it.url, style: { width: "100%", borderRadius: "8px", marginBottom: "14px" } });
  body.appendChild(img);

  const mv = el("div", { class: "meta-view" });
  const m = it.meta || {};
  const rows = [
    [t("meta_model"), m.model],
    [t("meta_model_id"), m.model_id],
    [t("meta_prompt"), m.prompt],
    [t("meta_time"), m.time ? fmtTime(m.time) : ""],
    [t("meta_params"), m.params ? JSON.stringify(m.params, null, 2) : ""],
    [t("meta_upstream"), m.upstream ? JSON.stringify(m.upstream, null, 2) : ""],
    [t("meta_file"), m.file],
  ];
  for (const [k, v] of rows) {
    if (!v) continue;
    mv.appendChild(el("div", { class: "row" }, [
      el("div", { class: "k" }, [k]),
      k === t("meta_params") || k === t("meta_upstream")
        ? el("div", { class: "v" }, [el("pre", {}, [escapeHtml(v)])])
        : el("div", { class: "v" }, [escapeHtml(v)]),
    ]));
  }
  body.appendChild(mv);

  const actions = el("div", { class: "form-actions" });
  const del = el("button", { class: "btn danger" }, [icon("trash"), t("delete_image")]);
  del.addEventListener("click", async () => {
    if (!(await askConfirm(t("del_image_title"), t("del_image_msg")))) return;
    try {
      await api("/images/" + encodeURIComponent(it.name), { method: "DELETE" });
      overlay.remove();
      await loadImages();
      render();
      toast(t("deleted"));
    } catch (e) { toast(e.message); }
  });
  actions.appendChild(del);
  body.appendChild(actions);

  modal.appendChild(body);
}

// ---------- data ----------

async function loadAll() { await Promise.all([loadModels(), loadConfig()]); }
async function loadModels() { state.models = await api("/models"); }
async function loadConfig() { state.config = await api("/config"); }
async function loadImages() {
  try { state.images = await api("/images"); if (state.tab === "images") render(); } catch (e) { toast(e.message); }
}
async function loadStats() {
  try {
    state.stats = await api("/stats");
    if (state.tab === "dashboard") render();
  } catch (e) { toast(e.message); }
}

// ---------- boot ----------

(async function init() {
  applyTheme();
  try {
    await api("/session");
    state.authed = true;
    await loadAll();
    await loadStats();
  } catch (e) {
    state.authed = false;
  }
  render();
  // auto-refresh dashboard while it is visible
  setInterval(() => {
    if (state.authed && state.tab === "dashboard") loadStats();
  }, 30000);
})();
