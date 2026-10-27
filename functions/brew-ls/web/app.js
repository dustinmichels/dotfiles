"use strict";

const state = {
  packages: [],
  outdated: { formula: {}, cask: {} },
  text: "",
  type: "all",
  year: null,
  sort: "date-desc",
  selected: null, // { type, name }
  info: new Map(), // "type:name" -> brew info JSON, or { error }
  running: false,
};

const $ = (id) => document.getElementById(id);
const keyOf = (type, name) => `${type}:${name}`;
const MONTHS = ["Jan", "Feb", "Mar", "Apr", "May", "Jun", "Jul", "Aug", "Sep", "Oct", "Nov", "Dec"];
const SHELF_ROWS = 14; // keep in sync with --shelf-rows in style.css
const dateFmt = new Intl.DateTimeFormat(undefined, { year: "numeric", month: "short", day: "numeric" });
const longDateFmt = new Intl.DateTimeFormat(undefined, { dateStyle: "full", timeStyle: "short" });

// Build DOM nodes without innerHTML so brew-provided text is never parsed as markup.
function h(tag, props = {}, ...children) {
  const el = document.createElement(tag);
  for (const [k, v] of Object.entries(props)) {
    if (v == null || v === false) continue;
    if (k === "class") el.className = v;
    else if (k === "dataset") Object.assign(el.dataset, v);
    else if (k.startsWith("on")) el.addEventListener(k.slice(2), v);
    else el.setAttribute(k, v === true ? "" : v);
  }
  return fill(el, ...children);
}

// Replace el's children, skipping null/false/undefined so `cond && node` works inline.
function fill(el, ...children) {
  el.replaceChildren();
  for (const c of children.flat(Infinity)) {
    if (c == null || c === false) continue;
    el.append(c instanceof Node ? c : String(c));
  }
  return el;
}

async function api(path) {
  const res = await fetch(path);
  const body = await res.json().catch(() => ({}));
  if (!res.ok) throw new Error(body.error || `${res.status} ${res.statusText}`);
  return body;
}

function outdatedFor(pkg) {
  return state.outdated[pkg.type]?.[pkg.name] ?? null;
}

function findInstalled(type, name) {
  return state.packages.find((p) => p.type === type && p.name === name) ?? null;
}

function yearOf(pkg) {
  return pkg.epoch > 0 ? new Date(pkg.epoch * 1000).getFullYear() : null;
}

function brewCommand(action, type, name) {
  const flag = type === "cask" ? " --cask" : action === "install" ? " --formula" : "";
  return `brew ${action}${flag} ${name}`;
}

// ---------- Filtering ----------

function matches(pkg) {
  if (state.type === "outdated" ? !outdatedFor(pkg) : state.type !== "all" && pkg.type !== state.type) return false;
  if (state.year != null && yearOf(pkg) !== state.year) return false;
  if (state.text) {
    const q = state.text.toLowerCase();
    if (!pkg.name.toLowerCase().includes(q) && !(pkg.desc || "").toLowerCase().includes(q)) return false;
  }
  return true;
}

function visible() {
  const list = state.packages.filter(matches);
  const byName = (a, b) => a.name.localeCompare(b.name);
  if (state.sort === "name") list.sort(byName);
  else if (state.sort === "date-asc") list.sort((a, b) => a.epoch - b.epoch || byName(a, b));
  else list.sort((a, b) => b.epoch - a.epoch || byName(a, b));
  return list;
}

// ---------- Rendering ----------

function render() {
  const list = visible();
  renderSummary();
  renderShelf(new Set(list.map((p) => keyOf(p.type, p.name))));
  renderRows(list);
  renderToolbar();
  renderDetails();
}

function renderSummary() {
  const formulae = state.packages.filter((p) => p.type === "formula").length;
  const casks = state.packages.length - formulae;
  const outdated = state.packages.filter(outdatedFor).length;
  fill($("summary"),
    h("strong", {}, formulae), " formulae and ", h("strong", {}, casks), " casks installed on request",
    outdated ? [", ", h("strong", { class: "warn" }, `${outdated} with updates`)] : null,
  );
}

function renderShelf(visibleKeys) {
  const dated = state.packages.filter((p) => p.epoch > 0);
  const shelf = $("shelf");
  if (!dated.length) {
    shelf.replaceChildren();
    return;
  }
  const years = dated.map(yearOf);
  const first = Math.min(...years);
  const last = Math.max(...years);
  const buckets = new Map(); // "year-month" -> packages, oldest at the bottom
  for (const p of [...dated].sort((a, b) => a.epoch - b.epoch)) {
    const d = new Date(p.epoch * 1000);
    const k = `${d.getFullYear()}-${d.getMonth()}`;
    if (!buckets.has(k)) buckets.set(k, []);
    buckets.get(k).push(p);
  }
  const sel = state.selected && keyOf(state.selected.type, state.selected.name);

  const yearEls = [];
  for (let y = first; y <= last; y++) {
    const months = [];
    for (let m = 0; m < 12; m++) {
      const pkgs = buckets.get(`${y}-${m}`) ?? [];
      months.push(h("div", {
        class: "month",
        style: `--cols: ${Math.max(1, Math.ceil(pkgs.length / SHELF_ROWS))}`,
        title: `${MONTHS[m]} ${y}: ${pkgs.length} installed`,
      },
        pkgs.map((p) => {
          const k = keyOf(p.type, p.name);
          const cls = ["cell", p.type, outdatedFor(p) && "outdated", k === sel && "selected", !visibleKeys.has(k) && "dim"];
          return h("button", {
            type: "button",
            class: cls.filter(Boolean).join(" "),
            title: `${p.name}, installed ${dateFmt.format(new Date(p.epoch * 1000))}`,
            tabindex: "-1",
            "aria-label": p.name,
            onclick: () => select(p.type, p.name),
          });
        })));
    }
    yearEls.push(h("div", { class: `year${state.year != null && state.year !== y ? " dim" : ""}` },
      h("div", { class: "months" }, months),
      h("button", {
        type: "button",
        class: "year-label",
        "aria-pressed": String(state.year === y),
        onclick: () => { state.year = state.year === y ? null : y; render(); },
      }, y),
    ));
  }
  const atEnd = shelf.scrollLeft + shelf.clientWidth >= shelf.scrollWidth - 4;
  shelf.replaceChildren(...yearEls);
  if (atEnd) shelf.scrollLeft = shelf.scrollWidth; // newest installs stay in view
}

function renderRows(list) {
  const sel = state.selected && keyOf(state.selected.type, state.selected.name);
  $("rows").replaceChildren(...list.map((p) => {
    const next = outdatedFor(p);
    const k = keyOf(p.type, p.name);
    return h("tr", {
      class: p.type,
      tabindex: "0",
      dataset: { key: k },
      "aria-selected": String(k === sel),
      onclick: () => select(p.type, p.name),
      onkeydown: onRowKey,
    },
      h("td", { class: "name" }, p.name),
      h("td", { class: "ver" }, p.version, next && h("span", { class: "next", title: "Update available" }, `→ ${next.latest}`)),
      h("td", { class: "col-desc" }, p.desc),
      h("td", { class: "date" }, p.epoch > 0 ? dateFmt.format(new Date(p.epoch * 1000)) : "Unknown"),
    );
  }));

  const empty = $("empty");
  empty.hidden = list.length > 0;
  if (!list.length) {
    empty.textContent = state.packages.length
      ? "No installed packages match these filters. Clear the filter text or pick another view."
      : "Homebrew reports no packages installed on request. Use Add package to install one.";
  }
}

function onRowKey(e) {
  const row = e.currentTarget;
  if (e.key === "Enter" || e.key === " ") {
    e.preventDefault();
    row.click();
  } else if (e.key === "ArrowDown" || e.key === "ArrowUp") {
    e.preventDefault();
    const target = e.key === "ArrowDown" ? row.nextElementSibling : row.previousElementSibling;
    if (target) {
      target.focus();
      target.click();
    }
  }
}

function renderToolbar() {
  for (const b of $("type-filter").querySelectorAll("button")) {
    b.setAttribute("aria-checked", String(b.dataset.type === state.type));
  }
  const chip = $("year-chip");
  chip.hidden = state.year == null;
  chip.textContent = state.year != null ? `Installed in ${state.year} ✕` : "";
}

function renderDetails() {
  const panel = $("details");
  if (!state.selected) {
    panel.replaceChildren(h("p", { class: "details-empty" }, "Select a package to see its details, dependencies, and actions."));
    return;
  }
  const { type, name } = state.selected;
  const pkg = findInstalled(type, name);
  const info = state.info.get(keyOf(type, name));
  const next = pkg && outdatedFor(pkg);
  const isCask = type === "cask";

  const latest = next?.latest ?? (info && !info.error ? (isCask ? info.version : info.versions?.stable) : null);
  const homepage = pkg?.homepage || info?.homepage;
  const desc = pkg?.desc || info?.desc;
  const deps = !isCask && info && !info.error ? info.dependencies ?? [] : [];
  const caskDeps = isCask && info && !info.error ? info.depends_on?.formula ?? [] : [];
  const requiredBy = pkg?.required_by ?? [];

  const fact = (label, value) => value ? [h("dt", {}, label), h("dd", {}, value)] : null;
  const pkgLink = (n) => {
    const t = findInstalled("formula", n) ? "formula" : null;
    return h("li", {}, t
      ? h("button", { type: "button", onclick: () => select(t, n) }, n)
      : h("span", {}, n));
  };

  let notice = null;
  if (pkg && requiredBy.length) {
    notice = h("p", { class: "notice" },
      `Required by ${requiredBy.length} installed ${requiredBy.length === 1 ? "package" : "packages"}. Uninstall those first to remove ${name}.`);
  } else if (pkg && next) {
    notice = h("p", { class: "notice" }, `Version ${next.latest} is available.${next.pinned ? " This formula is pinned; run brew unpin first." : ""}`);
  } else if (!pkg && info && !info.error) {
    notice = h("p", { class: "notice ok" }, "Not installed.");
  }

  const actions = h("div", { class: "actions" },
    pkg
      ? [
          h("button", {
            type: "button",
            class: `btn btn-upgrade${next ? " solid" : ""}`,
            disabled: state.running,
            onclick: () => act("upgrade", type, name),
          }, "Upgrade"),
          h("button", {
            type: "button",
            class: "btn btn-danger",
            disabled: state.running || requiredBy.length > 0,
            title: requiredBy.length ? `Required by ${requiredBy.join(", ")}` : null,
            onclick: () => act("uninstall", type, name),
          }, "Uninstall"),
        ]
      : h("button", {
          type: "button",
          class: "btn btn-install solid",
          disabled: state.running || !info || !!info.error,
          onclick: () => act("install", type, name),
        }, "Install"),
    /^https?:\/\//.test(homepage || "") &&
      h("a", { class: "btn btn-quiet", href: homepage, target: "_blank", rel: "noopener noreferrer" }, "Open homepage"),
  );

  fill(panel,
    h("h2", {}, name),
    h("span", { class: `kind ${type}` }, isCask ? "Cask" : "Formula", info?.tap ? ` from ${info.tap}` : ""),
    desc && h("p", { class: "desc" }, desc),
    h("dl", { class: "facts" },
      fact("Installed", pkg?.version ?? (info ? "No" : null)),
      fact("Latest", latest),
      fact("First installed", pkg && (pkg.epoch > 0 ? longDateFmt.format(new Date(pkg.epoch * 1000)) : "Unknown")),
      fact("License", info?.license),
      fact("Auto-updates", isCask && info?.auto_updates ? "Yes, the app updates itself" : null),
    ),
    notice,
    actions,
    !info && h("p", { class: "details-empty" }, "Loading Homebrew info…"),
    info?.error && h("p", { class: "notice" }, `Couldn't load Homebrew info: ${info.error}`),
    requiredBy.length > 0 && [h("h3", {}, "Required by"), h("ul", { class: "deps" }, requiredBy.map(pkgLink))],
    deps.length > 0 && [h("h3", {}, "Depends on"), h("ul", { class: "deps" }, deps.map(pkgLink))],
    caskDeps.length > 0 && [h("h3", {}, "Depends on"), h("ul", { class: "deps" }, caskDeps.map(pkgLink))],
    info?.caveats && [h("h3", {}, "Caveats"), h("pre", { class: "caveats" }, info.caveats.trim())],
    info && !info.error && h("details", {}, h("summary", {}, "Raw brew info"), h("pre", { class: "raw" }, JSON.stringify(info, null, 2))),
  );
}

// ---------- Selection & info ----------

async function select(type, name) {
  state.selected = { type, name };
  render();
  const k = keyOf(type, name);
  if (state.info.has(k)) return;
  try {
    state.info.set(k, await api(`/api/info?type=${encodeURIComponent(type)}&name=${encodeURIComponent(name)}`));
  } catch (err) {
    state.info.set(k, { error: err.message });
  }
  if (state.selected && keyOf(state.selected.type, state.selected.name) === k) renderDetails();
}

// ---------- Loading ----------

async function loadPackages() {
  try {
    state.packages = await api("/api/packages");
  } catch (err) {
    $("summary").textContent = `Couldn't list packages: ${err.message}`;
    return;
  }
  render();
}

async function loadOutdated() {
  try {
    state.outdated = await api("/api/outdated");
    render();
  } catch (err) {
    console.error("brew outdated failed:", err);
  }
}

// ---------- Actions ----------

const ACTION_COPY = {
  install: { title: "Install", verb: "Installing", done: "Installed", cls: "btn-install solid" },
  upgrade: { title: "Upgrade", verb: "Upgrading", done: "Upgraded", cls: "btn-upgrade solid" },
  uninstall: { title: "Uninstall", verb: "Uninstalling", done: "Uninstalled", cls: "btn-danger solid" },
};

function confirmAction(action, type, name) {
  const copy = ACTION_COPY[action];
  const dlg = $("confirm");
  $("confirm-title").textContent = `${copy.title} ${name}?`;
  $("confirm-body").textContent = action === "uninstall"
    ? "Homebrew removes the package from this Mac. Its leftover dependencies stay installed."
    : "This runs the following command:";
  $("confirm-cmd").textContent = brewCommand(action, type, name);
  const ok = $("confirm-ok");
  ok.textContent = copy.title;
  ok.className = `btn ${copy.cls}`;
  dlg.returnValue = "";
  dlg.showModal();
  ok.focus();
  return new Promise((resolve) => dlg.addEventListener("close", () => resolve(dlg.returnValue === "ok"), { once: true }));
}

async function act(action, type, name) {
  if (state.running || !(await confirmAction(action, type, name))) return;
  const copy = ACTION_COPY[action];
  const log = $("console-log");
  const status = $("console-status");
  $("console").hidden = false;
  log.textContent = "";
  status.className = "console-status running";
  status.textContent = `${copy.verb} ${name}…`;
  state.running = true;
  renderDetails();

  const append = (line) => {
    const stick = log.scrollTop + log.clientHeight >= log.scrollHeight - 8;
    log.append(line + "\n");
    if (stick) log.scrollTop = log.scrollHeight;
  };

  let result = null;
  try {
    const res = await fetch("/api/run", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ action, type, name }),
    });
    if (!res.ok) {
      const body = await res.json().catch(() => ({}));
      throw new Error(body.error || `${res.status} ${res.statusText}`);
    }
    const reader = res.body.pipeThrough(new TextDecoderStream()).getReader();
    let buf = "";
    for (;;) {
      const { value, done } = await reader.read();
      if (done) break;
      buf += value;
      let i;
      while ((i = buf.indexOf("\n")) >= 0) {
        const line = buf.slice(0, i);
        buf = buf.slice(i + 1);
        if (!line) continue;
        const ev = JSON.parse(line);
        if (ev.done) result = ev;
        else append(ev.line ?? "");
      }
    }
    if (!result) throw new Error("Lost connection to brew-ls before the command finished. Check the terminal running brew-ls.");
  } catch (err) {
    result = { ok: false, error: err.message };
  }

  state.running = false;
  if (result.ok) {
    status.className = "console-status ok";
    status.textContent = `${copy.done} ${name}`;
  } else {
    status.className = "console-status fail";
    status.textContent = result.error || `${copy.title} failed`;
  }

  state.info.delete(keyOf(type, name));
  await Promise.all([loadPackages(), loadOutdated()]);
  if (state.selected) select(state.selected.type, state.selected.name);
}

// ---------- Add package ----------

let searchTimer = 0;
let searchSeq = 0;

function openAdd() {
  const dlg = $("add");
  dlg.showModal();
  $("add-query").focus();
  $("add-query").select();
}

async function runSearch(q) {
  const box = $("add-results");
  const seq = ++searchSeq;
  if (q.length < 2) {
    box.replaceChildren(h("p", { class: "hint" }, "Type at least two characters to search Homebrew."));
    return;
  }
  box.replaceChildren(h("p", { class: "hint" }, `Searching for “${q}”…`));
  let res;
  try {
    res = await api(`/api/search?q=${encodeURIComponent(q)}`);
  } catch (err) {
    if (seq === searchSeq) box.replaceChildren(h("p", { class: "hint" }, `Search failed: ${err.message}`));
    return;
  }
  if (seq !== searchSeq) return;
  const group = (type, label) => {
    const names = res[type] ?? [];
    if (!names.length) return null;
    return [h("h3", {}, label), h("ul", {}, names.map((n) => h("li", {},
      h("button", {
        type: "button",
        onclick: () => { $("add").close(); select(type, n); },
      }, h("span", {}, n), findInstalled(type, n) && h("span", { class: "installed-tag" }, "Installed")),
    )))];
  };
  const groups = [group("formula", "Formulae"), group("cask", "Casks")].filter(Boolean);
  box.replaceChildren(...(groups.length ? groups.flat() : [h("p", { class: "hint" }, `Nothing on Homebrew matches “${q}”.`)]));
}

// ---------- Wiring ----------

$("filter").addEventListener("input", (e) => { state.text = e.target.value.trim(); render(); });
$("filter").addEventListener("keydown", (e) => {
  if (e.key === "Escape") { e.target.value = ""; state.text = ""; render(); }
});
$("type-filter").addEventListener("click", (e) => {
  const b = e.target.closest("button[data-type]");
  if (b) { state.type = b.dataset.type; render(); }
});
$("sort").addEventListener("change", (e) => { state.sort = e.target.value; render(); });
$("year-chip").addEventListener("click", () => { state.year = null; render(); });
$("console-close").addEventListener("click", () => { $("console").hidden = true; });
$("add-btn").addEventListener("click", openAdd);
$("add-query").addEventListener("input", (e) => {
  clearTimeout(searchTimer);
  const q = e.target.value.trim();
  searchTimer = setTimeout(() => runSearch(q), 300);
});
$("add-query").addEventListener("keydown", (e) => {
  if (e.key === "Enter") { e.preventDefault(); clearTimeout(searchTimer); runSearch(e.target.value.trim()); }
});
document.addEventListener("keydown", (e) => {
  if (e.key === "/" && !e.target.closest("input, select, textarea, dialog")) {
    e.preventDefault();
    $("filter").focus();
  }
});

runSearch("");
loadPackages().then(loadOutdated);
