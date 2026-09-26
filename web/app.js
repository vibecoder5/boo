const themes = ["dark", "sepia", "light"];

const FONTS = [
  { id: "serif", label: "Книжный", stack: `"Iowan Old Style", "Palatino Linotype", Palatino, "Times New Roman", serif` },
  { id: "georgia", label: "Georgia", stack: `Georgia, "Times New Roman", serif` },
  { id: "palatino", label: "Palatino", stack: `"Palatino Linotype", Palatino, "Book Antiqua", serif` },
  { id: "times", label: "Times", stack: `"Times New Roman", Times, serif` },
  { id: "garamond", label: "Garamond", stack: `Garamond, "EB Garamond", "Palatino Linotype", serif` },
  { id: "cambria", label: "Cambria", stack: `Cambria, Georgia, serif` },
  { id: "system", label: "Системный", stack: `"Segoe UI", system-ui, sans-serif` },
  { id: "segoe", label: "Segoe UI", stack: `"Segoe UI", sans-serif` },
  { id: "arial", label: "Arial", stack: `Arial, Helvetica, sans-serif` },
  { id: "verdana", label: "Verdana", stack: `Verdana, Geneva, sans-serif` },
  { id: "tahoma", label: "Tahoma", stack: `Tahoma, Geneva, sans-serif` },
  { id: "calibri", label: "Calibri", stack: `Calibri, Candara, "Segoe UI", sans-serif` },
];

const BOOK_FONT_MIN = 14;
const BOOK_FONT_MAX = 36;
const UI_FONT_MIN = 12;
const UI_FONT_MAX = 24;

const state = {
  book: null,
  workspace: { id: "", name: "Библиотека" },
  workspaces: [],
  library: [],
  lists: [],
  selectedListId: "",
  openedListId: "",
  shelfCtxKey: "",
  bookNoteKey: "",
  ui: { theme: "dark", fontSize: 20, bookFont: "serif", bookFontSize: 20, uiFont: "system", uiFontSize: 16, lineHeight: 1.7, maxWidth: 38, sidebarWidth: 280, sidebarOpen: true, notesWidth: 300, notesOpen: true, historyWidth: 280, historyOpen: false, workspacesWidth: 280, workspacesOpen: true, listsWidth: 280, listsOpen: true, welcomeBackground: "", gamificationDisabled: false },
  game: null,
  history: [],
  undo: [],
  readStats: { todaySec: 0, weekSec: 0, monthSec: 0, totalSec: 0, today: "0 с", week: "0 с", month: "0 с", total: "0 с" },
  bookReadStats: null,
  chapterIndex: 0,
  saving: false,
  query: "",
  hits: [],
  shelfQuery: "",
  shelfHits: [],
  shelfHitIndex: -1,
  bookmarks: [],
  highlights: [],
  notes: [],
  todos: [],
  todoBook: null,
  todoDlgKey: "",
  todoDlgTodos: [],
  activeNoteId: "",
  tocFold: [],
  readChapters: [],
  readTOC: [],
  chapterFragment: "",
  chapterHTML: "",
  dictionaries: [],
  drive: {},
};

const NOTE_COLORS = ["yellow", "green", "blue", "pink", "orange"];
const MAX_READ_DURATION_SEC = 24 * 60 * 60;

const readTimer = {
  status: "idle",
  elapsedMs: 0,
  startedAt: 0,
  bookKey: "",
  tick: 0,
};

let shownLevelUp = 0;

const $ = (id) => document.getElementById(id);

async function api(url, options) {
  const res = await fetch(url, options);
  if (!res.ok) {
    const text = await res.text();
    throw new Error(text || res.statusText);
  }
  const type = res.headers.get("content-type") || "";
  if (type.includes("json")) return res.json();
  return res.text();
}

function clampSidebarWidth(w) {
  const max = Math.min(560, Math.floor(window.innerWidth * 0.55));
  return Math.max(180, Math.min(max, Math.round(w)));
}

function applySidebar() {
  const width = clampSidebarWidth(state.ui.sidebarWidth || 280);
  state.ui.sidebarWidth = width;
  document.documentElement.style.setProperty("--sidebar-width", `${width}px`);
  const open = Boolean(state.ui.sidebarOpen);
  $("sidebar").hidden = !open;
  $("sidebarPeek").hidden = open;
  $("tocBtn").setAttribute("aria-expanded", open ? "true" : "false");
}

function clampNotesWidth(w) {
  const max = Math.min(520, Math.floor(window.innerWidth * 0.5));
  return Math.max(220, Math.min(max, Math.round(w)));
}

const NOTES_RAIL = 36;

function applyNotes() {
  const width = clampNotesWidth(state.ui.notesWidth || 300);
  state.ui.notesWidth = width;
  document.documentElement.style.setProperty("--notes-width", `${width}px`);
  document.documentElement.style.setProperty("--notes-rail", `${NOTES_RAIL}px`);
  const bar = $("notesbar");
  const btn = $("notesBtn");
  const hasBook = Boolean(state.book);
  const open = Boolean(hasBook && state.ui.notesOpen);
  if (bar) {
    bar.hidden = !hasBook;
    bar.classList.toggle("collapsed", !open);
  }
  if (btn) {
    btn.hidden = !hasBook;
    btn.setAttribute("aria-expanded", open ? "true" : "false");
  }
}

function setNotesOpen(open) {
  state.ui.notesOpen = Boolean(open);
  applyNotes();
  saveUI();
}

const HISTORY_RAIL = 36;

function clampHistoryWidth(w) {
  const max = Math.min(480, Math.floor(window.innerWidth * 0.45));
  return Math.max(220, Math.min(max, Math.round(w)));
}

function applyHistory() {
  const width = clampHistoryWidth(state.ui.historyWidth || 280);
  state.ui.historyWidth = width;
  document.documentElement.style.setProperty("--history-width", `${width}px`);
  document.documentElement.style.setProperty("--history-rail", `${HISTORY_RAIL}px`);
  const bar = $("historybar");
  const btn = $("historyBtn");
  const hasBook = Boolean(state.book);
  const open = Boolean(hasBook && state.ui.historyOpen);
  if (bar) {
    bar.hidden = !hasBook;
    bar.classList.toggle("collapsed", !open);
  }
  if (btn) {
    btn.hidden = !hasBook;
    btn.setAttribute("aria-expanded", open ? "true" : "false");
  }
}

function setHistoryOpen(open) {
  state.ui.historyOpen = Boolean(open);
  applyHistory();
  saveUI();
}

const WORKSPACES_RAIL = 36;

function clampWorkspacesWidth(w) {
  const max = Math.min(480, Math.floor(window.innerWidth * 0.45));
  return Math.max(220, Math.min(max, Math.round(w)));
}

function applyWorkspaces() {
  const width = clampWorkspacesWidth(state.ui.workspacesWidth || 280);
  state.ui.workspacesWidth = width;
  document.documentElement.style.setProperty("--workspaces-width", `${width}px`);
  document.documentElement.style.setProperty("--workspaces-rail", `${WORKSPACES_RAIL}px`);
  const bar = $("workspacesbar");
  const btn = $("workspacesBtn");
  const onShelf = !state.book;
  const open = Boolean(onShelf && state.ui.workspacesOpen);
  if (bar) {
    bar.hidden = !onShelf;
    bar.classList.toggle("collapsed", !open);
  }
  if (btn) {
    btn.hidden = !onShelf;
    btn.setAttribute("aria-expanded", open ? "true" : "false");
  }
}

function setWorkspacesOpen(open) {
  state.ui.workspacesOpen = Boolean(open);
  applyWorkspaces();
  saveUI();
}

const LISTS_RAIL = 36;

function clampListsWidth(w) {
  const max = Math.min(480, Math.floor(window.innerWidth * 0.45));
  return Math.max(220, Math.min(max, Math.round(w)));
}

function applyLists() {
  const width = clampListsWidth(state.ui.listsWidth || 280);
  state.ui.listsWidth = width;
  document.documentElement.style.setProperty("--lists-width", `${width}px`);
  document.documentElement.style.setProperty("--lists-rail", `${LISTS_RAIL}px`);
  const bar = $("listsbar");
  const btn = $("listsBtn");
  const onShelf = !state.book;
  const open = Boolean(onShelf && state.ui.listsOpen);
  if (bar) {
    bar.hidden = !onShelf;
    bar.classList.toggle("collapsed", !open);
  }
  if (btn) {
    btn.hidden = !onShelf;
    btn.setAttribute("aria-expanded", open ? "true" : "false");
  }
}

function setListsOpen(open) {
  state.ui.listsOpen = Boolean(open);
  applyLists();
  saveUI();
}

const PAGE_WIDTH_MIN = 22;
const PAGE_WIDTH_MAX = 80;

function clampPageWidth(w) {
  return Math.max(PAGE_WIDTH_MIN, Math.min(PAGE_WIDTH_MAX, Math.round(Number(w) || 38)));
}

function applyPageWidth() {
  const w = clampPageWidth(state.ui.maxWidth);
  state.ui.maxWidth = w;
  document.documentElement.style.setProperty("--measure", w >= PAGE_WIDTH_MAX ? "100%" : `${w}rem`);
  const range = $("widthRange");
  if (range && range.value !== String(w)) range.value = String(w);
}

function bumpPageWidth(delta) {
  state.ui.maxWidth = clampPageWidth((state.ui.maxWidth || 38) + delta);
  applyUI();
  saveUI();
}

function fontById(id, fallback) {
  return FONTS.find((f) => f.id === id) || FONTS.find((f) => f.id === fallback) || FONTS[0];
}

function clampBookFontSize(n) {
  return Math.max(BOOK_FONT_MIN, Math.min(BOOK_FONT_MAX, Math.round(Number(n) || 20)));
}

function clampUIFontSize(n) {
  return Math.max(UI_FONT_MIN, Math.min(UI_FONT_MAX, Math.round(Number(n) || 16)));
}

function ensureFonts() {
  const book = fontById(state.ui.bookFont, "serif");
  const ui = fontById(state.ui.uiFont, "system");
  state.ui.bookFont = book.id;
  state.ui.uiFont = ui.id;
  if (!state.ui.bookFontSize) state.ui.bookFontSize = state.ui.fontSize || 20;
  state.ui.bookFontSize = clampBookFontSize(state.ui.bookFontSize);
  state.ui.fontSize = state.ui.bookFontSize;
  state.ui.uiFontSize = clampUIFontSize(state.ui.uiFontSize || 16);
  return { book, ui };
}

function fillFontSelects() {
  for (const id of ["bookFont", "bookFontMenu", "uiFont", "uiFontMenu"]) {
    const el = $(id);
    if (!el || el.options.length) continue;
    for (const font of FONTS) {
      const opt = document.createElement("option");
      opt.value = font.id;
      opt.textContent = font.label;
      el.appendChild(opt);
    }
  }
}

function setControlValue(id, value) {
  const el = $(id);
  if (el && el.value !== String(value)) el.value = String(value);
}

function setControlText(id, value) {
  const el = $(id);
  if (el) el.textContent = String(value);
}

function syncFontControls() {
  setControlValue("bookFont", state.ui.bookFont);
  setControlValue("bookFontMenu", state.ui.bookFont);
  setControlValue("uiFont", state.ui.uiFont);
  setControlValue("uiFontMenu", state.ui.uiFont);
  setControlValue("bookFontRange", state.ui.bookFontSize);
  setControlValue("bookFontRangeMenu", state.ui.bookFontSize);
  setControlValue("uiFontRange", state.ui.uiFontSize);
  setControlValue("uiFontRangeMenu", state.ui.uiFontSize);
  setControlText("bookFontValue", state.ui.bookFontSize);
  setControlText("bookFontValueMenu", state.ui.bookFontSize);
  setControlText("uiFontValue", state.ui.uiFontSize);
  setControlText("uiFontValueMenu", state.ui.uiFontSize);
}

function applyFonts() {
  const { book, ui } = ensureFonts();
  document.documentElement.style.setProperty("--serif", book.stack);
  document.documentElement.style.setProperty("--sans", ui.stack);
  document.documentElement.style.setProperty("--font-size", `${state.ui.bookFontSize}px`);
  document.documentElement.style.setProperty("--ui-font-size", `${state.ui.uiFontSize}px`);
  syncFontControls();
}

function bumpBookFont(delta) {
  state.ui.bookFontSize = clampBookFontSize((state.ui.bookFontSize || state.ui.fontSize || 20) + delta);
  state.ui.fontSize = state.ui.bookFontSize;
  applyUI();
  saveUI();
}

function bumpUIFont(delta) {
  state.ui.uiFontSize = clampUIFontSize((state.ui.uiFontSize || 16) + delta);
  applyUI();
  saveUI();
}

function applyUI() {
  document.documentElement.dataset.theme = state.ui.theme;
  applyFonts();
  document.documentElement.style.setProperty("--line-height", String(state.ui.lineHeight));
  applyPageWidth();
  applySidebar();
  applyNotes();
  applyHistory();
  applyWorkspaces();
  applyLists();
  applyTocFold();
  paintTocHideRead();
  applyWelcomeBackground();
  const gameToggle = $("gameEnabled");
  if (gameToggle) gameToggle.checked = !state.ui.gamificationDisabled;
  renderGame();
}

function welcomeBackgroundUrl(name) {
  if (!name) return "";
  return `/api/ui/background?v=${encodeURIComponent(name)}`;
}

function applyWelcomeBackground() {
  const name = state.ui.welcomeBackground || "";
  const url = welcomeBackgroundUrl(name);
  const body = document.body;
  if (url) {
    body.classList.add("has-welcome-bg");
    document.documentElement.style.setProperty("--welcome-bg", `url("${url}")`);
  } else {
    body.classList.remove("has-welcome-bg");
    document.documentElement.style.removeProperty("--welcome-bg");
  }
  const preview = $("welcomeBgPreview");
  const thumb = $("welcomeBgThumb");
  const clear = $("welcomeBgClear");
  if (preview) preview.hidden = !url;
  if (thumb) {
    if (url) thumb.src = url;
    else thumb.removeAttribute("src");
  }
  if (clear) clear.hidden = !url;
}

function setSidebarOpen(open) {
  state.ui.sidebarOpen = Boolean(open);
  applySidebar();
  saveUI();
}

async function saveUI() {
  state.ui = await api("/api/ui", {
    method: "PUT",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(state.ui),
  });
  applyUI();
}

function scrollRatio() {
  const el = $("reader");
  const max = el.scrollHeight - el.clientHeight;
  if (max <= 0) return 0;
  return el.scrollTop / max;
}

function bookProgress(ratio, index) {
  const total = state.book ? state.book.chapterN : 1;
  const overall = total <= 1 ? ratio : (index + ratio) / total;
  return Math.max(0, Math.min(1, overall));
}

function progressLabel(ratio, index) {
  const total = state.book ? state.book.chapterN : 1;
  const percent = Math.round(bookProgress(ratio, index) * 100);
  const chapter = Math.min(total, index + 1);
  return `Глава ${chapter} из ${total} · ${percent}%`;
}

function setProgress(ratio, index) {
  const pct = `${Math.round(bookProgress(ratio, index) * 100)}%`;
  const label = progressLabel(ratio, index);
  $("progressFill").style.width = pct;
  $("sidebarProgressFill").style.width = pct;
  $("progressMeta").textContent = label;
  $("sidebarProgressText").textContent = label;
  $("progressTrack").title = label;
  if (state.book) {
    $("pageInfo").textContent = label;
  }
}

async function seekBook(overall) {
  if (!state.book) return;
  const total = state.book.chapterN;
  const pos = Math.max(0, Math.min(0.999, overall)) * total;
  const index = Math.min(total - 1, Math.floor(pos));
  const ratio = pos - index;
  await openChapter(index, "", false);
  const reader = $("reader");
  const max = reader.scrollHeight - reader.clientHeight;
  reader.scrollTop = max > 0 ? max * ratio : 0;
  setProgress(ratio, index);
  scheduleSave();
}

function seekFromEvent(e, el) {
  const rect = el.getBoundingClientRect();
  if (rect.width <= 0) return;
  seekBook((e.clientX - rect.left) / rect.width);
}

let saveTimer = 0;
function scheduleSave() {
  if (!state.book) return;
  clearTimeout(saveTimer);
  saveTimer = setTimeout(saveProgress, 350);
}

function readerScreens() {
  const el = $("reader");
  if (!el || el.hidden) return 0;
  const pageH = el.clientHeight;
  if (pageH < 40) return 0;
  return el.scrollHeight / pageH;
}

async function saveProgress() {
  if (!state.book || state.saving) return;
  state.saving = true;
  try {
    const data = await api("/api/progress", {
      method: "PUT",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({
        chapterIndex: state.chapterIndex,
        scrollRatio: scrollRatio(),
        screens: readerScreens(),
      }),
    });
    applyGame(data);
  } finally {
    state.saving = false;
  }
}

function renderBookmarks() {
  const box = $("bookmarksBox");
  const list = $("bookmarkList");
  if (!box || !list) return;
  if (!state.book) {
    box.hidden = true;
    list.replaceChildren();
    return;
  }
  box.hidden = false;
  list.replaceChildren();
  if (!state.bookmarks.length) {
    const empty = document.createElement("p");
    empty.className = "bookmark-empty";
    empty.textContent = "Пока нет закладок";
    list.appendChild(empty);
    return;
  }
  for (const mark of state.bookmarks) {
    const row = document.createElement("div");
    row.className = "bookmark-item";
    const jump = document.createElement("button");
    jump.type = "button";
    jump.className = "jump";
    const title = document.createElement("strong");
    title.textContent = mark.title || mark.chapterTitle || "Закладка";
    const meta = document.createElement("span");
    const chapter = (mark.chapterIndex || 0) + 1;
    const pct = Math.round((mark.scrollRatio || 0) * 100);
    meta.textContent = `Глава ${chapter} · ${pct}% главы`;
    jump.appendChild(title);
    jump.appendChild(meta);
    jump.addEventListener("click", () => openBookmark(mark));
    const remove = document.createElement("button");
    remove.type = "button";
    remove.className = "icon-btn";
    remove.setAttribute("aria-label", "Удалить закладку");
    remove.textContent = "×";
    remove.addEventListener("click", (e) => {
      e.stopPropagation();
      deleteBookmark(mark.id);
    });
    row.appendChild(jump);
    row.appendChild(remove);
    list.appendChild(row);
  }
}

function dictPairLabel(item) {
  const from = ((item && item.fromLang) || "").toUpperCase();
  const to = ((item && item.toLang) || "").toUpperCase();
  if (from && to) return `${from} → ${to}`;
  return "";
}

function activeDictionary() {
  if (!state.book || !state.book.dictionaryId) return null;
  return (state.dictionaries || []).find((item) => item.id === state.book.dictionaryId) || null;
}

function dictActive() {
  return Boolean(activeDictionary());
}

function wordCountLabel(n) {
  const mod10 = n % 10;
  const mod100 = n % 100;
  if (mod10 === 1 && mod100 !== 11) return `${n} слово`;
  if (mod10 >= 2 && mod10 <= 4 && (mod100 < 12 || mod100 > 14)) return `${n} слова`;
  return `${n} слов`;
}

function renderDictionary() {
  const box = $("dictBox");
  const list = $("dictList");
  const hint = $("dictHint");
  if (!box || !list) return;
  if (!state.book) {
    box.hidden = true;
    list.replaceChildren();
    return;
  }
  box.hidden = false;
  list.replaceChildren();
  const items = state.dictionaries || [];
  if (hint) hint.hidden = items.length > 0;
  const activeId = state.book.dictionaryId || "";
  for (const item of items) {
    const row = document.createElement("div");
    row.className = "dict-item";
    const pick = document.createElement("button");
    pick.type = "button";
    pick.className = "pick";
    if (item.id === activeId) pick.classList.add("in");
    const title = document.createElement("strong");
    title.textContent = item.name || "Словарь";
    const meta = document.createElement("span");
    const bits = [dictPairLabel(item)];
    if (item.wordCount) bits.push(wordCountLabel(item.wordCount));
    meta.textContent = bits.filter(Boolean).join(" · ");
    pick.appendChild(title);
    if (meta.textContent) pick.appendChild(meta);
    pick.addEventListener("click", () => setBookDictionary(item.id === activeId ? "" : item.id));
    const remove = document.createElement("button");
    remove.type = "button";
    remove.className = "icon-btn";
    remove.setAttribute("aria-label", "Отключить словарь");
    remove.textContent = "×";
    remove.addEventListener("click", (e) => {
      e.stopPropagation();
      deleteDictionary(item.id);
    });
    row.appendChild(pick);
    row.appendChild(remove);
    list.appendChild(row);
  }
  if (items.length) {
    const off = document.createElement("button");
    off.type = "button";
    off.className = "text-btn dict-off";
    off.textContent = "Выключить словарь";
    if (!activeId) off.classList.add("in");
    off.disabled = !activeId;
    off.addEventListener("click", () => setBookDictionary(""));
    list.appendChild(off);
  }
}

function applyDictState(data) {
  state.dictionaries = data.dictionaries || [];
  if (state.book && data.dictionaryId !== undefined) {
    state.book.dictionaryId = data.dictionaryId || "";
  }
  dictCache.clear();
  renderDictionary();
  paintDictCtx("");
  if (!dictActive()) hideDictTip();
}

async function setBookDictionary(id) {
  if (!state.book) return;
  try {
    const data = await api("/api/library/dictionary", {
      method: "PUT",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ key: state.book.key, id: id || "" }),
    });
    applyDictState(data);
  } catch (err) {
    alert(err.message || "Не удалось переключить словарь");
  }
}

async function deleteDictionary(id) {
  try {
    const data = await api(`/api/dictionaries?id=${encodeURIComponent(id)}`, { method: "DELETE" });
    applyDictState(data);
  } catch (err) {
    alert(err.message || "Не удалось удалить словарь");
  }
}

function pickDictionaryFile() {
  const input = $("dictInput");
  if (!input) return;
  input.value = "";
  input.click();
}

async function uploadDictionary(file) {
  if (!file) return;
  try {
    const body = new FormData();
    body.append("file", file);
    const data = await api("/api/dictionaries", { method: "POST", body });
    applyDictState(data);
    setSidebarOpen(true);
  } catch (err) {
    alert(err.message || "Не удалось подключить словарь");
  }
}

async function uploadWelcomeBackground(file) {
  if (!file) return;
  try {
    const body = new FormData();
    body.append("file", file);
    state.ui = await api("/api/ui/background", { method: "POST", body });
    applyUI();
  } catch (err) {
    alert(err.message || "Не удалось сохранить картинку");
  }
}

async function clearWelcomeBackground() {
  try {
    state.ui = await api("/api/ui/background", { method: "DELETE" });
    applyUI();
  } catch (err) {
    alert(err.message || "Не удалось убрать картинку");
  }
}

function toggleDictionary() {
  if (!state.book) return;
  const items = state.dictionaries || [];
  if (!items.length) {
    setSidebarOpen(true);
    pickDictionaryFile();
    return;
  }
  if (dictActive()) {
    setBookDictionary("");
    return;
  }
  setBookDictionary(items[0].id);
}

function visibleSnippet() {
  const text = ($("content").innerText || "").replace(/\s+/g, " ").trim();
  if (!text) return "";
  const max = $("reader").scrollHeight - $("reader").clientHeight;
  const ratio = max > 0 ? $("reader").scrollTop / max : 0;
  const start = Math.min(text.length, Math.floor(text.length * ratio));
  return text.slice(start, start + 80).trim();
}

async function addBookmark() {
  if (!state.book) return;
  try {
    const snippet = visibleSnippet();
    const chapter = state.book.chapters[state.chapterIndex];
    const title = snippet || (chapter && chapter.title) || `Глава ${state.chapterIndex + 1}`;
    const data = await api("/api/bookmarks", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({
        chapterIndex: state.chapterIndex,
        scrollRatio: scrollRatio(),
        title,
      }),
    });
    state.bookmarks = data.bookmarks || [];
    renderBookmarks();
    setSidebarOpen(true);
  } catch (err) {
    alert(err.message || "Не удалось сохранить закладку");
  }
}

async function deleteBookmark(id) {
  try {
    const data = await api(`/api/bookmarks?id=${encodeURIComponent(id)}`, { method: "DELETE" });
    state.bookmarks = data.bookmarks || [];
    if (data.undo) state.undo = data.undo;
    renderBookmarks();
    renderUndo();
  } catch (err) {
    alert(err.message || "Не удалось удалить закладку");
  }
}

async function openBookmark(mark) {
  await openChapter(mark.chapterIndex, "", false);
  const reader = $("reader");
  const max = reader.scrollHeight - reader.clientHeight;
  reader.scrollTop = max > 0 ? max * (mark.scrollRatio || 0) : 0;
  setProgress(mark.scrollRatio || 0, mark.chapterIndex);
  scheduleSave();
}

function tocKey(path) {
  return path.join(".");
}

function isTocItemCollapsed(key) {
  return state.tocFold.includes(key);
}

function applyTocFold() {
  const collapsed = Boolean(state.ui.tocCollapsed);
  const box = $("tocBox");
  const nav = $("toc");
  const btn = $("tocFold");
  const hideRead = $("tocHideRead");
  if (!box || !nav || !btn) return;
  box.classList.toggle("collapsed", collapsed);
  nav.hidden = collapsed;
  btn.setAttribute("aria-expanded", collapsed ? "false" : "true");
  if (hideRead) hideRead.hidden = collapsed;
}

function hideReadChaptersOn() {
  return Boolean(state.ui.hideReadChapters);
}

function paintTocHideRead() {
  const btn = $("tocHideRead");
  if (!btn) return;
  const on = hideReadChaptersOn();
  btn.classList.toggle("in", on);
  btn.setAttribute("aria-pressed", on ? "true" : "false");
}

function toggleHideReadChapters() {
  state.ui.hideReadChapters = !hideReadChaptersOn();
  paintTocHideRead();
  refreshTOC();
  saveUI();
}

function tocNodeVisible(item, path) {
  if (!hideReadChaptersOn()) return true;
  const trail = path || [];
  const kids = item.children || [];
  if (kids.some((kid, i) => tocNodeVisible(kid, trail.concat(i)))) return true;
  if (item.chapterIndex >= 0) return !tocItemRead(item, trail);
  return false;
}

function toggleTocSection() {
  state.ui.tocCollapsed = !state.ui.tocCollapsed;
  applyTocFold();
  saveUI();
}

function toggleTocItem(key) {
  if (isTocItemCollapsed(key)) {
    state.tocFold = state.tocFold.filter((id) => id !== key);
  } else {
    state.tocFold = state.tocFold.concat(key);
  }
  refreshTOC();
  saveTocFold();
}

let tocFoldTimer = 0;
function saveTocFold() {
  if (!state.book) return;
  clearTimeout(tocFoldTimer);
  tocFoldTimer = setTimeout(async () => {
    try {
      const data = await api("/api/toc-fold", {
        method: "PUT",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ collapsed: state.tocFold }),
      });
      state.tocFold = data.tocFold || [];
    } catch (err) {
      console.error(err);
    }
  }, 200);
}

function refreshTOC() {
  if (!state.book) {
    if ($("toc")) $("toc").replaceChildren();
    paintTocHideRead();
    return;
  }
  renderTOC(state.book.toc || [], $("toc"));
  applyTocFold();
  paintTocHideRead();
}

function chapterRead(index) {
  return (state.readChapters || []).some((i) => i === index);
}

function tocPathRead(key) {
  return (state.readTOC || []).includes(key);
}

function tocItemRead(item, path) {
  if (!item || item.chapterIndex < 0) return false;
  if (tocPathRead(tocKey(path || []))) return true;
  return chapterRead(item.chapterIndex);
}

function tocItemActive(item) {
  if (!item || item.chapterIndex < 0 || item.chapterIndex !== state.chapterIndex) return false;
  const frag = item.fragment || "";
  const cur = state.chapterFragment || "";
  if (cur) return frag === cur;
  return !frag;
}

function chapterReadLabel(read) {
  return read ? "Снять отметку о прочтении главы" : "Отметить главу прочитанной";
}

function applyReadMarks(data) {
  if (!data) return;
  if (data.readChapters) state.readChapters = data.readChapters;
  if (data.readTOC) state.readTOC = data.readTOC;
}

async function setChapterRead(index, read, tocKey) {
  if (!state.book) return;
  const body = { read };
  if (tocKey) body.tocKey = tocKey;
  else {
    if (index < 0) return;
    body.chapterIndex = index;
  }
  const data = await api("/api/chapters/read", {
    method: "PUT",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(body),
  });
  applyReadMarks(data);
  applyGame(data);
  refreshTOC();
  paintChapterRead();
}

function paintChapterRead() {
  const hasBook = Boolean(state.book);
  const read = hasBook && chapterRead(state.chapterIndex);
  const mark = $("pageChapterRead");
  if (mark) {
    mark.hidden = !hasBook;
    mark.classList.toggle("in", read);
    mark.textContent = read ? "Глава прочитана" : "Отметить главу";
    mark.setAttribute("aria-pressed", read ? "true" : "false");
  }
  const ctxBtn = $("ctxChapterRead");
  if (ctxBtn) {
    ctxBtn.textContent = chapterReadLabel(read);
    ctxBtn.classList.toggle("in", read);
  }
}

function renderTOC(items, into, path) {
  into.replaceChildren();
  const trail = path || [];
  items.forEach((item, i) => {
    const itemPath = trail.concat(i);
    if (!tocNodeVisible(item, itemPath)) return;
    const key = tocKey(itemPath);
    const kids = item.children || [];
    const hasKids = kids.some((kid, j) => tocNodeVisible(kid, itemPath.concat(j)));
    const node = document.createElement("div");
    node.className = "toc-node";
    const row = document.createElement("div");
    row.className = "toc-row";
    if (hasKids) {
      const collapsed = isTocItemCollapsed(key);
      const twist = document.createElement("button");
      twist.type = "button";
      twist.className = "toc-twist";
      twist.setAttribute("aria-label", collapsed ? "Развернуть главу" : "Свернуть главу");
      twist.setAttribute("aria-expanded", collapsed ? "false" : "true");
      twist.addEventListener("click", (e) => {
        e.preventDefault();
        e.stopPropagation();
        toggleTocItem(key);
      });
      row.appendChild(twist);
    } else {
      const spacer = document.createElement("span");
      spacer.className = "toc-twist-spacer";
      row.appendChild(spacer);
    }
    const a = document.createElement("a");
    a.href = "#";
    a.textContent = item.title || "…";
    if (tocItemActive(item)) a.classList.add("active");
    if (item.chapterIndex >= 0 && tocItemRead(item, itemPath)) a.classList.add("read");
    a.addEventListener("click", (e) => {
      e.preventDefault();
      if (item.chapterIndex >= 0) openChapter(item.chapterIndex, item.fragment, true);
    });
    row.appendChild(a);
    if (item.chapterIndex >= 0) {
      const read = tocItemRead(item, itemPath);
      const mark = document.createElement("button");
      mark.type = "button";
      mark.className = "toc-read";
      if (read) mark.classList.add("in");
      mark.title = chapterReadLabel(read);
      mark.setAttribute("aria-label", chapterReadLabel(read));
      mark.setAttribute("aria-pressed", read ? "true" : "false");
      mark.addEventListener("click", (e) => {
        e.preventDefault();
        e.stopPropagation();
        setChapterRead(item.chapterIndex, !read, key).catch((err) => {
          alert(err.message || "Не удалось отметить главу");
        });
      });
      row.appendChild(mark);
    }
    node.appendChild(row);
    if (hasKids) {
      const nest = document.createElement("div");
      nest.className = "nested";
      nest.hidden = isTocItemCollapsed(key);
      renderTOC(kids, nest, itemPath);
      node.appendChild(nest);
    }
    into.appendChild(node);
  });
  if (!trail.length && !into.childElementCount && hideReadChaptersOn()) {
    const empty = document.createElement("p");
    empty.className = "toc-empty";
    empty.textContent = "Нет непрочитанных глав";
    into.appendChild(empty);
  }
}

function bookWord(n) {
  const mod10 = n % 10;
  const mod100 = n % 100;
  if (mod10 === 1 && mod100 !== 11) return "книга";
  if (mod10 >= 2 && mod10 <= 4 && (mod100 < 12 || mod100 > 14)) return "книги";
  return "книг";
}

function workspaceDisplayList() {
  const pinned = [];
  const rest = [];
  for (const item of state.workspaces || []) {
    if (item.pinned) pinned.push(item);
    else rest.push(item);
  }
  return pinned.concat(rest);
}

let workspaceSortDrag = null;
let workspaceSortBound = false;

function bindWorkspaceSortListeners() {
  if (workspaceSortBound) return;
  workspaceSortBound = true;
  document.addEventListener("pointermove", onWorkspaceSortMove);
  document.addEventListener("pointerup", onWorkspaceSortPointerUp);
  document.addEventListener("pointercancel", onWorkspaceSortPointerUp);
}

function onWorkspaceSortMove(e) {
  const drag = workspaceSortDrag;
  if (!drag || e.pointerId !== drag.pointerId) return;
  if (!drag.moved && Math.abs(e.clientY - drag.startY) < 6) return;
  e.preventDefault();
  drag.moved = true;
  const card = drag.card;
  card.classList.add("dragging");
  document.body.classList.add("workspace-sorting");
  const grid = $("workspaceGrid");
  if (!grid) return;
  const pinned = card.classList.contains("pinned");
  const others = [...grid.querySelectorAll(".workspace-card")].filter(
    (el) => el !== card && el.classList.contains("pinned") === pinned
  );
  let before = null;
  for (const other of others) {
    const rect = other.getBoundingClientRect();
    if (e.clientY < rect.top + rect.height / 2) {
      before = other;
      break;
    }
  }
  if (before) {
    grid.insertBefore(card, before);
    return;
  }
  if (pinned) {
    const firstRest = [...grid.querySelectorAll(".workspace-card")].find(
      (el) => el !== card && !el.classList.contains("pinned")
    );
    if (firstRest) grid.insertBefore(card, firstRest);
    else grid.appendChild(card);
    return;
  }
  grid.appendChild(card);
}

async function onWorkspaceSortPointerUp(e) {
  const drag = workspaceSortDrag;
  if (!drag || e.pointerId !== drag.pointerId) return;
  const moved = drag.moved;
  const item = drag.item;
  const card = drag.card;
  workspaceSortDrag = null;
  card.classList.remove("dragging");
  document.body.classList.remove("workspace-sorting");
  try {
    card.releasePointerCapture(drag.pointerId);
  } catch (_) { /* already released */ }
  if (moved) {
    await persistWorkspaceOrder();
    return;
  }
  if (e.type === "pointercancel") return;
  if (!item.current && item.id !== (state.workspace && state.workspace.id)) {
    switchWorkspace(item.id);
  }
}

function bindWorkspaceSort(card, item) {
  bindWorkspaceSortListeners();
  card.addEventListener("pointerdown", (e) => {
    if (e.button !== 0) return;
    if (e.target.closest("button")) return;
    e.preventDefault();
    workspaceSortDrag = {
      card,
      item,
      pointerId: e.pointerId,
      startY: e.clientY,
      moved: false,
    };
    try {
      card.setPointerCapture(e.pointerId);
    } catch (_) { /* capture is optional */ }
  });
  card.addEventListener("dragstart", (e) => e.preventDefault());
}

async function persistWorkspaceOrder() {
  const grid = $("workspaceGrid");
  if (!grid) return;
  const ids = [...grid.querySelectorAll(".workspace-card")].map((el) => el.dataset.id).filter(Boolean);
  const expected = workspaceDisplayList().map((item) => item.id);
  if (ids.length === expected.length && ids.every((id, i) => id === expected[i])) return;
  try {
    const payload = await api("/api/workspaces/order", {
      method: "PUT",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ ids }),
    });
    await applyState(payload, Boolean(payload.book));
  } catch (err) {
    renderWorkspaces();
    alert(err.message || "Не удалось сохранить порядок пространств");
  }
}

async function setWorkspacePinned(id, pinned) {
  const payload = await api("/api/workspaces/pin", {
    method: "PUT",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ id, pinned }),
  });
  await applyState(payload, Boolean(payload.book));
}

function renderWorkspaces() {
  const grid = $("workspaceGrid");
  const title = $("workspaceTitle");
  const shelfLabel = $("shelfLabel");
  if (!grid) return;
  const current = state.workspace || {};
  if (title) title.textContent = current.name || "Библиотека";
  if (shelfLabel) {
    shelfLabel.textContent = current.name ? `Полка · ${current.name}` : "Полка";
  }
  grid.replaceChildren();
  for (const item of workspaceDisplayList()) {
    const card = document.createElement("div");
    card.className = "workspace-card";
    card.dataset.id = item.id || "";
    card.setAttribute("role", "button");
    card.tabIndex = 0;
    card.draggable = false;
    if (item.pinned) card.classList.add("pinned");
    if (item.current || item.id === current.id) card.classList.add("current");
    const handle = document.createElement("span");
    handle.className = "drag-handle";
    handle.setAttribute("aria-hidden", "true");
    handle.textContent = "⋮⋮";
    card.appendChild(handle);
    const name = document.createElement("strong");
    name.textContent = item.name || "Без названия";
    const meta = document.createElement("span");
    const count = item.bookCount || 0;
    const status = [];
    if (item.pinned) status.push("Закреплено");
    if (item.current || item.id === current.id) status.push("Открыто");
    status.push(`${count} ${bookWord(count)}`);
    meta.textContent = status.join(" · ");
    card.appendChild(name);
    card.appendChild(meta);
    const pin = document.createElement("button");
    pin.type = "button";
    pin.className = "pin";
    pin.setAttribute("aria-label", item.pinned ? "Открепить пространство" : "Закрепить пространство");
    pin.setAttribute("title", item.pinned ? "Открепить" : "Закрепить");
    pin.textContent = "📌";
    pin.addEventListener("click", async (e) => {
      e.stopPropagation();
      try {
        await setWorkspacePinned(item.id, !item.pinned);
      } catch (err) {
        alert(err.message || "Не удалось закрепить пространство");
      }
    });
    card.appendChild(pin);
    const rename = document.createElement("button");
    rename.type = "button";
    rename.className = "rename";
    rename.setAttribute("aria-label", "Переименовать пространство");
    rename.textContent = "✎";
    rename.addEventListener("click", (e) => {
      e.stopPropagation();
      setWorkspaceFormOpen(true, item.id);
    });
    card.appendChild(rename);
    bindWorkspaceSort(card, item);
    card.addEventListener("keydown", (e) => {
      if (e.target !== card) return;
      if (e.key !== "Enter" && e.key !== " ") return;
      e.preventDefault();
      if (!item.current && item.id !== current.id) switchWorkspace(item.id);
    });
    grid.appendChild(card);
  }
}

function workspaceById(id) {
  return (state.workspaces || []).find((item) => item.id === id) || null;
}

function setWorkspaceFormOpen(open, renameId) {
  const form = $("workspaceForm");
  if (!form) return;
  if (open && !state.book && !state.ui.workspacesOpen) setWorkspacesOpen(true);
  form.hidden = !open;
  form.dataset.renameId = open && renameId ? renameId : "";
  const submit = $("workspaceFormSubmit");
  const input = $("workspaceName");
  if (open) {
    setListFormOpen(false);
    const item = renameId ? workspaceById(renameId) : null;
    input.value = item ? (item.name || "") : "";
    input.placeholder = item ? "Новое название" : "Название, например «Учёба»";
    if (submit) submit.textContent = item ? "Сохранить" : "Создать";
    input.focus();
    if (item) input.select();
  } else {
    input.placeholder = "Название, например «Учёба»";
    if (submit) submit.textContent = "Создать";
  }
}

async function createWorkspace(name) {
  const payload = await api("/api/workspaces", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ name }),
  });
  setWorkspaceFormOpen(false);
  await applyState(payload, false);
}

async function renameWorkspace(id, name) {
  const payload = await api("/api/workspaces", {
    method: "PUT",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ id, name }),
  });
  setWorkspaceFormOpen(false);
  await applyState(payload, Boolean(payload.book));
}

function selectedList() {
  if (!state.selectedListId) return null;
  return (state.lists || []).find((item) => item.id === state.selectedListId) || null;
}

function openedList() {
  if (!state.openedListId) return null;
  return (state.lists || []).find((item) => item.id === state.openedListId) || null;
}

function syncListSelection() {
  const ids = new Set((state.lists || []).map((item) => item.id));
  if (state.selectedListId && !ids.has(state.selectedListId)) state.selectedListId = "";
  if (state.openedListId && !ids.has(state.openedListId)) state.openedListId = "";
}

function renderLists() {
  const grid = $("listGrid");
  const title = $("listsTitle");
  if (!grid) return;
  const current = selectedList();
  if (state.selectedListId && !current) state.selectedListId = "";
  if (title) title.textContent = current ? current.name : "Все книги";
  grid.replaceChildren();

  const all = document.createElement("button");
  all.type = "button";
  all.className = "workspace-card";
  if (!state.selectedListId) all.classList.add("current");
  const allName = document.createElement("strong");
  allName.textContent = "Все книги";
  const allMeta = document.createElement("span");
  const allCount = (state.library || []).length;
  allMeta.textContent = `${allCount} ${bookWord(allCount)}`;
  all.appendChild(allName);
  all.appendChild(allMeta);
  all.addEventListener("click", () => selectList(""));
  grid.appendChild(all);

  for (const item of state.lists || []) {
    const card = document.createElement("button");
    card.type = "button";
    card.className = "workspace-card";
    if (item.id === state.selectedListId) card.classList.add("current");
    const name = document.createElement("strong");
    name.textContent = item.name || "Без названия";
    const meta = document.createElement("span");
    const count = (item.bookKeys || []).length;
    meta.textContent = `${count} ${bookWord(count)}`;
    card.appendChild(name);
    card.appendChild(meta);
    const remove = document.createElement("button");
    remove.type = "button";
    remove.className = "remove";
    remove.setAttribute("aria-label", "Удалить список");
    remove.textContent = "×";
    remove.addEventListener("click", (e) => {
      e.stopPropagation();
      deleteList(item.id);
    });
    remove.addEventListener("dblclick", (e) => e.stopPropagation());
    card.appendChild(remove);
    card.title = "Двойной щелчок — открыть список";
    card.addEventListener("click", () => selectList(item.id));
    card.addEventListener("dblclick", (e) => {
      e.preventDefault();
      openListScreen(item.id);
    });
    grid.appendChild(card);
  }
}

function selectList(id) {
  state.selectedListId = id || "";
  if (!id) state.openedListId = "";
  else if (state.openedListId) state.openedListId = id;
  closeAddToListDlg();
  renderLists();
  renderLibrary();
}

function openListScreen(id) {
  if (!id) return;
  hideShelfCtx();
  hideWelcomeCtx();
  closeAddBookDlg();
  closeAddToListDlg();
  state.selectedListId = id;
  state.openedListId = id;
  renderLists();
  renderLibrary();
}

function closeListScreen() {
  state.openedListId = "";
  state.selectedListId = "";
  closeAddToListDlg();
  renderLists();
  renderLibrary();
}

function setListFormOpen(open) {
  const form = $("listForm");
  if (!form) return;
  if (open && !state.book && !state.ui.listsOpen) setListsOpen(true);
  form.hidden = !open;
  if (open) {
    setWorkspaceFormOpen(false);
    $("listName").value = "";
    $("listName").focus();
  }
}

async function createList(name, bookKey) {
  const payload = await api("/api/lists", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ name, key: bookKey || "" }),
  });
  setListFormOpen(false);
  await applyState(payload, Boolean(payload.book));
}

async function deleteList(id) {
  try {
    const payload = await api(`/api/lists?id=${encodeURIComponent(id)}`, { method: "DELETE" });
    if (state.selectedListId === id) state.selectedListId = "";
    if (state.openedListId === id) state.openedListId = "";
    await applyState(payload, Boolean(payload.book));
  } catch (err) {
    alert(err.message || "Не удалось удалить список");
  }
}

async function addToList(id, key) {
  const payload = await api("/api/lists/books", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ id, key }),
  });
  await applyState(payload, Boolean(payload.book));
}

async function removeFromList(id, key) {
  const payload = await api(`/api/lists/books?id=${encodeURIComponent(id)}&key=${encodeURIComponent(key)}`, {
    method: "DELETE",
  });
  await applyState(payload, Boolean(payload.book));
}

function bookInList(list, key) {
  return Boolean(list && (list.bookKeys || []).includes(key));
}

function formatDueAt(iso) {
  if (!iso) return "";
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return "";
  return d.toLocaleString("ru-RU", {
    day: "numeric",
    month: "short",
    hour: "2-digit",
    minute: "2-digit",
  });
}

function pad2(n) {
  return String(n).padStart(2, "0");
}

function toLocalInput(iso) {
  if (!iso) return "";
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return "";
  return `${d.getFullYear()}-${pad2(d.getMonth() + 1)}-${pad2(d.getDate())}T${pad2(d.getHours())}:${pad2(d.getMinutes())}`;
}

function localNowParts() {
  const d = new Date();
  return {
    date: `${d.getFullYear()}-${pad2(d.getMonth() + 1)}-${pad2(d.getDate())}`,
    time: `${pad2(d.getHours())}:${pad2(d.getMinutes())}`,
  };
}

function setTodoDueNow(dateId, timeId) {
  const now = localNowParts();
  const dateEl = $(dateId);
  const timeEl = $(timeId);
  if (dateEl) dateEl.value = now.date;
  if (timeEl) timeEl.value = now.time;
}

function todoDueFromInputs(dateId, timeId) {
  const dateEl = $(dateId);
  const timeEl = $(timeId);
  const date = dateEl ? dateEl.value : "";
  const time = timeEl ? timeEl.value : "";
  if (!date) return "";
  return `${date}T${time || "00:00"}`;
}

function bindTodoDatePicker(id) {
  const el = $(id);
  if (!el || !el.showPicker) return;
  el.addEventListener("click", () => {
    try {
      el.showPicker();
    } catch (_) {
      /* picker already open or not allowed */
    }
  });
}

function todoDueOverdue(item) {
  if (!item || item.done || !item.dueAt) return false;
  const d = new Date(item.dueAt);
  return !Number.isNaN(d.getTime()) && d.getTime() < Date.now();
}

function fillTodoList(list, items, key) {
  if (!list) return;
  list.replaceChildren();
  if (!items.length) {
    const empty = document.createElement("p");
    empty.className = "todo-empty";
    empty.textContent = "Пока нет дел. Добавьте строку и время исполнения.";
    list.appendChild(empty);
    return;
  }
  for (const item of items) {
    const row = document.createElement("div");
    row.className = "todo-item";
    if (item.done) row.classList.add("done");
    if (todoDueOverdue(item)) row.classList.add("overdue");
    const label = document.createElement("label");
    const check = document.createElement("input");
    check.type = "checkbox";
    check.checked = Boolean(item.done);
    check.addEventListener("change", () => setTodoDone(key, item, check.checked));
    const body = document.createElement("span");
    body.className = "todo-body";
    const text = document.createElement("span");
    text.className = "todo-text";
    text.textContent = item.text || "Без названия";
    const due = document.createElement("span");
    due.className = "todo-due";
    due.textContent = item.dueAt ? formatDueAt(item.dueAt) : "";
    body.appendChild(text);
    if (due.textContent) body.appendChild(due);
    label.appendChild(check);
    label.appendChild(body);
    const remove = document.createElement("button");
    remove.type = "button";
    remove.className = "remove";
    remove.setAttribute("aria-label", "Удалить задание");
    remove.textContent = "×";
    remove.addEventListener("click", (e) => {
      e.preventDefault();
      deleteTodo(key, item.id);
    });
    row.appendChild(label);
    row.appendChild(remove);
    list.appendChild(row);
  }
}

function renderWelcomeTodos() {
  const box = $("todoBox");
  const list = $("welcomeTodoList");
  const title = $("todoTitle");
  const form = $("welcomeTodoForm");
  if (!box || !list) return;
  const book = !state.book && state.todoBook && state.todoBook.key ? state.todoBook : null;
  box.hidden = !book;
  if (!book) {
    list.replaceChildren();
    return;
  }
  if (title) title.textContent = book.title || "Без названия";
  fillTodoList(list, state.todos || [], book.key);
  if (form) form.hidden = false;
}

function historyBookKey() {
  if (state.book && state.book.key) return state.book.key;
  if (state.history && state.history[0] && state.history[0].bookKey) return state.history[0].bookKey;
  if (state.todoBook && state.todoBook.key) return state.todoBook.key;
  if (state.library && state.library[0]) return state.library[0].key;
  return "";
}

function historyCanOpen(item) {
  if (!item || !item.bookKey) return false;
  const book = (state.library || []).find((entry) => entry.key === item.bookKey);
  return Boolean(book && book.canOpen) || Boolean(state.book && state.book.key === item.bookKey);
}

function formatHistoryWhen(iso) {
  if (!iso) return "";
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return "";
  return d.toLocaleString("ru-RU", {
    day: "numeric",
    month: "short",
    hour: "2-digit",
    minute: "2-digit",
  });
}

function historyProgressText(item) {
  const chapter = (item.chapterIndex || 0) + 1;
  const pct = Math.round((item.scrollRatio || 0) * 100);
  const title = (item.chapterTitle || "").trim();
  if (title) return `${title} · ${pct}%`;
  return `Глава ${chapter} · ${pct}%`;
}

function fillHistoryList(list, items) {
  if (!list) return;
  list.replaceChildren();
  if (!items.length) {
    const empty = document.createElement("p");
    empty.className = "history-empty";
    empty.textContent = "Пока нет записей. Они появятся, когда вы закроете книгу, остановите таймер или добавите отметку.";
    list.appendChild(empty);
    return;
  }
  for (const item of items) {
    const row = document.createElement("button");
    row.type = "button";
    row.className = "history-item";
    if (item.kind === "note") row.classList.add("note");
    if (item.kind === "read_time") row.classList.add("read-time");
    if (!historyCanOpen(item)) row.classList.add("dead");
    const when = document.createElement("span");
    when.className = "when";
    when.textContent = formatHistoryWhen(item.createdAt);
    const title = document.createElement("strong");
    const detail = document.createElement("span");
    detail.className = "detail";
    if (item.kind === "note") {
      title.textContent = item.text || "Отметка";
      const bits = [item.title || "Книга"];
      const progress = historyProgressText(item);
      if (progress) bits.push(progress);
      detail.textContent = bits.join(" · ");
    } else if (item.kind === "read_time") {
      title.textContent = item.text || "Чтение";
      const bits = [item.title || "Книга"];
      const progress = historyProgressText(item);
      if (progress) bits.push(progress);
      detail.textContent = bits.join(" · ");
    } else {
      title.textContent = item.title || "Книга";
      const bits = ["Читал"];
      if (item.author) bits.push(item.author);
      bits.push(historyProgressText(item));
      detail.textContent = bits.join(" · ");
    }
    row.appendChild(when);
    row.appendChild(title);
    if (detail.textContent) row.appendChild(detail);
    if (historyCanOpen(item)) {
      row.addEventListener("click", () => openFromHistory(item));
    } else {
      row.disabled = true;
    }
    list.appendChild(row);
  }
}

function renderHistory() {
  const items = (state.history || []).slice(0, 10);
  fillHistoryList($("welcomeHistoryList"), items);
  fillHistoryList($("readerHistoryList"), items);
  const welcomeForm = $("welcomeHistoryForm");
  if (welcomeForm) welcomeForm.hidden = !historyBookKey() || Boolean(state.book);
}

const READ_STAT_PERIODS = [
  ["today", "Сегодня"],
  ["week", "Неделя"],
  ["month", "Месяц"],
  ["total", "Всего"],
];

function emptyReadStats() {
  return { todaySec: 0, weekSec: 0, monthSec: 0, totalSec: 0, today: "0 с", week: "0 с", month: "0 с", total: "0 с" };
}

function fillReadStats(grid, stats) {
  if (!grid) return;
  const data = stats || emptyReadStats();
  grid.replaceChildren();
  for (const [key, label] of READ_STAT_PERIODS) {
    const cell = document.createElement("div");
    cell.className = "read-stat";
    const name = document.createElement("span");
    name.className = "label";
    name.textContent = label;
    const value = document.createElement("strong");
    value.className = "value";
    value.textContent = data[key] || "0 с";
    cell.appendChild(name);
    cell.appendChild(value);
    grid.appendChild(cell);
  }
}

function renderReadStats() {
  fillReadStats($("welcomeReadStatsGrid"), state.readStats);
  fillReadStats($("bookReadStatsGrid"), state.bookReadStats || emptyReadStats());
  const title = $("readStatsDlgTitle");
  if (title) title.textContent = (state.book && (state.book.title || "").trim()) || "Эта книга";
}

function applyReadStats(payload) {
  if (!payload) return;
  if (payload.readStats) state.readStats = payload.readStats;
  if (Object.prototype.hasOwnProperty.call(payload, "bookReadStats")) {
    state.bookReadStats = payload.bookReadStats || null;
  }
  renderReadStats();
}

function pointsWord(n) {
  const n10 = n % 10;
  const n100 = n % 100;
  if (n10 === 1 && n100 !== 11) return "очко";
  if (n10 >= 2 && n10 <= 4 && (n100 < 12 || n100 > 14)) return "очка";
  return "очков";
}

function pointsLabel(n) {
  const v = Math.max(0, Math.round(Number(n) || 0));
  return `${v} ${pointsWord(v)}`;
}

function formatPageCount(hundredths) {
  const n = Math.max(0, Number(hundredths) || 0) / 100;
  const tenths = Math.round(n * 10) / 10;
  if (Math.abs(tenths - Math.round(tenths)) < 0.001) return String(Math.round(tenths));
  return tenths.toFixed(1).replace(".", ",");
}

function taskProgressText(task) {
  if (!task) return "";
  if (task.kind === "pages") {
    const shown = Math.min(task.progress || 0, task.goal || 0);
    return `${formatPageCount(shown)} из ${formatPageCount(task.goal || 0)}`;
  }
  return task.done ? "Готово" : "0 из 1";
}

function gameVisible() {
  if (state.ui.gamificationDisabled) return false;
  return Boolean(state.game && state.game.enabled);
}

function renderGame() {
  const box = $("gameBox");
  const shelf = $("shelfGame");
  const on = gameVisible();
  if (box) box.hidden = !on;
  if (shelf) shelf.hidden = !on;
  if (!on || !state.game) return;
  const game = state.game;
  const level = $("gameLevel");
  const points = $("gamePoints");
  const month = $("gameMonth");
  if (level) level.textContent = `Уровень ${game.level || 1}`;
  if (points) points.textContent = pointsLabel(game.points);
  if (month) month.textContent = `За месяц: ${pointsLabel(game.monthPoints)}. Всего: ${pointsLabel(game.points)}.`;
  if (shelf) shelf.textContent = `Уровень ${game.level || 1} · ${pointsLabel(game.points)}`;
  const span = game.levelSpan || 1;
  const into = game.intoLevel || 0;
  const pct = Math.max(0, Math.min(100, Math.round((into / span) * 100)));
  const fill = $("gameLevelFill");
  const track = $("gameLevelTrack");
  if (fill) fill.style.width = `${pct}%`;
  if (track) {
    track.setAttribute("aria-valuenow", String(pct));
    track.title = `${pointsLabel(into)} из ${pointsLabel(span)} до следующего уровня`;
  }
  const list = $("gameTasks");
  if (!list) return;
  list.replaceChildren();
  for (const task of game.daily || []) {
    const row = document.createElement("li");
    row.className = "game-task";
    if (task.done) row.classList.add("done");
    const mark = document.createElement("span");
    mark.className = "game-task-mark";
    mark.setAttribute("aria-hidden", "true");
    mark.textContent = task.done ? "✓" : "·";
    const title = document.createElement("span");
    title.className = "game-task-title";
    title.textContent = task.title || "";
    const progress = document.createElement("span");
    progress.className = "game-task-progress";
    progress.textContent = taskProgressText(task);
    row.append(mark, title, progress);
    list.appendChild(row);
  }
}

function applyGame(payload) {
  if (!payload || !payload.game) return;
  state.game = payload.game;
  renderGame();
  if (payload.game.levelUp) showLevelUp(payload.game.levelUp);
}

function levelUpOpen() {
  const dlg = $("levelUpDlg");
  return Boolean(dlg && !dlg.hidden);
}

function showLevelUp(levelUp) {
  if (!levelUp || state.ui.gamificationDisabled) return;
  if (levelUp.level <= shownLevelUp) return;
  shownLevelUp = levelUp.level;
  const dlg = $("levelUpDlg");
  const text = $("levelUpText");
  if (!dlg || !text) return;
  text.textContent = levelUp.text || `Новый уровень: ${levelUp.level}`;
  dlg.hidden = false;
  ackLevel(levelUp.level);
}

function closeLevelUp() {
  const dlg = $("levelUpDlg");
  if (dlg) dlg.hidden = true;
}

async function ackLevel(level) {
  try {
    const data = await api("/api/game/ack", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ level }),
    });
    if (data && data.game) {
      state.game = data.game;
      renderGame();
    }
  } catch (err) {
    console.warn(err);
  }
}

async function awardTimerStart() {
  if (state.ui.gamificationDisabled) return;
  try {
    const data = await api("/api/game/timer-start", { method: "POST" });
    applyGame(data);
  } catch (err) {
    console.warn(err);
  }
}

function readStatsDlgOpen() {
  const dlg = $("readStatsDlg");
  return Boolean(dlg && !dlg.hidden);
}

function openReadStatsDlg() {
  if (!state.book) return;
  renderReadStats();
  const dlg = $("readStatsDlg");
  if (dlg) dlg.hidden = false;
}

function closeReadStatsDlg() {
  const dlg = $("readStatsDlg");
  if (dlg) dlg.hidden = true;
}

function undoDetail(item) {
  const title = (item.detail || "").trim();
  const book = (item.bookTitle || "").trim();
  if (title && book && item.kind !== "delete_book") return `${title} · ${book}`;
  return title || book || item.label || "Удаление";
}

function fillUndoList(list, items) {
  if (!list) return;
  list.replaceChildren();
  if (!items.length) {
    const empty = document.createElement("p");
    empty.className = "history-empty";
    empty.textContent = "Здесь появятся удалённые книги, закладки и заметки. Можно отменить последнее или откатиться до выбранной записи.";
    list.appendChild(empty);
    return;
  }
  items.forEach((item, index) => {
    const row = document.createElement("button");
    row.type = "button";
    row.className = "history-item undo";
    const when = document.createElement("span");
    when.className = "when";
    when.textContent = formatHistoryWhen(item.createdAt);
    const title = document.createElement("strong");
    title.textContent = undoDetail(item);
    const detail = document.createElement("span");
    detail.className = "detail";
    detail.textContent = index === 0
      ? item.label || "Отменить"
      : `${item.label || "Удаление"} · откатить досюда`;
    row.appendChild(when);
    row.appendChild(title);
    row.appendChild(detail);
    row.addEventListener("click", () => restoreUndo(item, index));
    list.appendChild(row);
  });
}

function renderUndo() {
  const items = state.undo || [];
  fillUndoList($("welcomeUndoList"), items);
  fillUndoList($("readerUndoList"), items);
  const empty = items.length === 0;
  for (const id of ["undoLastBtn", "readerUndoLastBtn"]) {
    const btn = $(id);
    if (btn) btn.disabled = empty;
  }
}

async function applyUndoPayload(payload) {
  const sameBook = Boolean(state.book && payload.book && state.book.key === payload.book.key);
  const index = sameBook ? state.chapterIndex : 0;
  const ratio = sameBook ? scrollRatio() : 0;
  await applyState(payload, !sameBook && Boolean(payload.book));
  if (sameBook && state.book) {
    await openChapter(index, "", false);
    const reader = $("reader");
    const max = reader.scrollHeight - reader.clientHeight;
    reader.scrollTop = max > 0 ? max * ratio : 0;
    setProgress(ratio, index);
  }
}

async function undoLast() {
  if (!(state.undo || []).length) return;
  try {
    const payload = await api("/api/undo", { method: "POST" });
    await applyUndoPayload(payload);
  } catch (err) {
    alert(err.message || "Не удалось отменить");
  }
}

async function restoreUndo(item, index) {
  if (!item || !item.id) return;
  if (index > 0) {
    const n = index + 1;
    const ok = window.confirm(`Вернуть «${undoDetail(item)}» и ещё ${n - 1} более новых удалений?`);
    if (!ok) return;
  }
  try {
    const payload = await api("/api/undo/restore", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ id: item.id }),
    });
    await applyUndoPayload(payload);
  } catch (err) {
    alert(err.message || "Не удалось откатиться");
  }
}

function readTimerElapsedMs() {
  let ms = readTimer.elapsedMs;
  if (readTimer.status === "running" && readTimer.startedAt) {
    ms += Date.now() - readTimer.startedAt;
  }
  return Math.max(0, ms);
}

function formatTimerClock(ms) {
  const sec = Math.floor(Math.max(0, ms) / 1000);
  const h = Math.floor(sec / 3600);
  const m = Math.floor((sec % 3600) / 60);
  const s = sec % 60;
  if (h > 0) {
    return `${h}:${String(m).padStart(2, "0")}:${String(s).padStart(2, "0")}`;
  }
  return `${m}:${String(s).padStart(2, "0")}`;
}

function stopReadTimerTick() {
  if (readTimer.tick) {
    window.clearInterval(readTimer.tick);
    readTimer.tick = 0;
  }
}

function resetReadTimer() {
  stopReadTimerTick();
  readTimer.status = "idle";
  readTimer.elapsedMs = 0;
  readTimer.startedAt = 0;
  readTimer.bookKey = "";
  renderReadTimer();
}

function renderReadTimer() {
  const box = $("readTimer");
  const time = $("readTimerTime");
  const start = $("readTimerStart");
  const pause = $("readTimerPause");
  const stop = $("readTimerStop");
  if (!box || !time || !start || !pause || !stop) return;
  const hasBook = Boolean(state.book);
  const status = hasBook ? readTimer.status : "idle";
  const ms = hasBook ? readTimerElapsedMs() : 0;
  box.hidden = !hasBook;
  box.classList.toggle("running", status === "running");
  box.classList.toggle("paused", status === "paused");
  time.textContent = formatTimerClock(ms);
  time.setAttribute("aria-label", `Время чтения ${time.textContent}`);
  start.hidden = status === "running";
  start.textContent = status === "paused" ? "Продолжить" : "Старт";
  pause.hidden = status !== "running";
  stop.hidden = status === "idle";
}

function startReadTimerTick() {
  stopReadTimerTick();
  readTimer.tick = window.setInterval(renderReadTimer, 250);
}

function startReadTimer() {
  if (!state.book || !state.book.key) return;
  if (readTimer.status === "running") return;
  const fresh = readTimer.status !== "paused";
  if (readTimer.status === "paused") {
    readTimer.status = "running";
    readTimer.startedAt = Date.now();
  } else {
    readTimer.status = "running";
    readTimer.elapsedMs = 0;
    readTimer.startedAt = Date.now();
    readTimer.bookKey = state.book.key;
  }
  startReadTimerTick();
  renderReadTimer();
  if (fresh) awardTimerStart();
}

function pauseReadTimer() {
  if (readTimer.status !== "running") return;
  readTimer.elapsedMs = readTimerElapsedMs();
  readTimer.startedAt = 0;
  readTimer.status = "paused";
  stopReadTimerTick();
  renderReadTimer();
}

function readTimerPayload(sec) {
  const key = readTimer.bookKey || (state.book && state.book.key) || "";
  const body = { key, durationSec: sec };
  if (state.book && state.book.key === key) {
    body.chapterIndex = state.chapterIndex;
    body.scrollRatio = scrollRatio();
  }
  return body;
}

async function commitReadTimer(options) {
  const keepOnError = Boolean(options && options.keepOnError);
  const beacon = Boolean(options && options.beacon);
  if (readTimer.status === "idle") return null;
  const snapshot = {
    status: readTimer.status,
    elapsedMs: readTimerElapsedMs(),
    bookKey: readTimer.bookKey,
  };
  let sec = Math.floor(snapshot.elapsedMs / 1000);
  if (sec > MAX_READ_DURATION_SEC) sec = MAX_READ_DURATION_SEC;
  const body = readTimerPayload(sec);
  resetReadTimer();
  if (sec < 1 || !body.key) return null;
  if (beacon) {
    navigator.sendBeacon("/api/history/read-time", new Blob([JSON.stringify(body)], { type: "application/json" }));
    return null;
  }
  try {
    const data = await api("/api/history/read-time", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(body),
    });
    applyReadStats(data);
    applyGame(data);
    return data;
  } catch (err) {
    if (keepOnError) {
      readTimer.status = snapshot.status === "running" ? "paused" : snapshot.status;
      readTimer.elapsedMs = snapshot.elapsedMs;
      readTimer.bookKey = snapshot.bookKey;
      stopReadTimerTick();
      renderReadTimer();
      throw err;
    }
    console.warn(err);
    return null;
  }
}

async function stopReadTimer() {
  try {
    const data = await commitReadTimer({ keepOnError: true });
    if (data && data.history) {
      state.history = data.history;
      renderHistory();
    }
  } catch (err) {
    alert(err.message || "Не удалось записать время чтения");
  }
}

async function addHistoryNote(key, text) {
  const body = { key, text };
  if (state.book && state.book.key === key) {
    body.chapterIndex = state.chapterIndex;
    body.scrollRatio = scrollRatio();
  }
  const data = await api("/api/history", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(body),
  });
  state.history = data.history || [];
  renderHistory();
}

async function openFromHistory(item) {
  if (!item || !item.bookKey) return;
  if (!state.book || state.book.key !== item.bookKey) {
    await openFromShelf(item.bookKey);
  }
  if (!state.book || state.book.key !== item.bookKey) return;
  const index = item.chapterIndex || 0;
  const ratio = item.scrollRatio || 0;
  await openChapter(index, "", false);
  const reader = $("reader");
  const max = reader.scrollHeight - reader.clientHeight;
  reader.scrollTop = max > 0 ? max * ratio : 0;
  setProgress(ratio, index);
  scheduleSave();
}

function bindHistoryForm(formId, textId, keyOf) {
  const form = $(formId);
  if (!form) return;
  form.addEventListener("submit", async (e) => {
    e.preventDefault();
    const key = keyOf();
    const textEl = $(textId);
    const text = textEl ? textEl.value.trim() : "";
    if (!key) return;
    if (!text) {
      if (textEl) textEl.focus();
      return;
    }
    try {
      await addHistoryNote(key, text);
      if (textEl) {
        textEl.value = "";
        textEl.focus();
      }
    } catch (err) {
      alert(err.message || "Не удалось добавить отметку");
    }
  });
}

function todoDlgOpen() {
  const dlg = $("todoDlg");
  return Boolean(dlg && !dlg.hidden);
}

function renderTodoDlg() {
  const dlg = $("todoDlg");
  if (!dlg || dlg.hidden || !state.todoDlgKey) return;
  const title = $("todoDlgTitle");
  if (title) title.textContent = bookMetaOf(state.todoDlgKey).title;
  fillTodoList($("dlgTodoList"), state.todoDlgTodos || [], state.todoDlgKey);
}

function applyTodoPayload(data) {
  const book = data.todoBook || null;
  const todos = data.todos || [];
  const key = book && book.key;
  if (key && state.todoBook && state.todoBook.key === key) {
    state.todoBook = book;
    state.todos = todos;
  }
  if (key && state.todoDlgKey === key) {
    state.todoDlgTodos = todos;
  }
  renderWelcomeTodos();
  renderTodoDlg();
}

async function openTodoDlg(key) {
  if (!key) return;
  hideCtx();
  hideShelfCtx();
  hideWelcomeCtx();
  state.todoDlgKey = key;
  if (state.todoBook && state.todoBook.key === key) {
    state.todoDlgTodos = (state.todos || []).slice();
  } else {
    try {
      const data = await api(`/api/todos?key=${encodeURIComponent(key)}`);
      state.todoDlgTodos = data.todos || [];
    } catch (err) {
      alert(err.message || "Не удалось открыть список дел");
      state.todoDlgKey = "";
      return;
    }
  }
  const dlg = $("todoDlg");
  if (!dlg) return;
  dlg.hidden = false;
  renderTodoDlg();
  setTodoDueNow("dlgTodoDate", "dlgTodoTime");
  const text = $("dlgTodoText");
  if (text) text.focus();
}

function closeTodoDlg() {
  const dlg = $("todoDlg");
  if (dlg) dlg.hidden = true;
  state.todoDlgKey = "";
  state.todoDlgTodos = [];
}

async function addTodo(key, text, dueAt) {
  const data = await api("/api/todos", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ key, text, dueAt }),
  });
  applyTodoPayload(data);
}

async function setTodoDone(key, item, done) {
  try {
    const data = await api("/api/todos", {
      method: "PUT",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({
        id: item.id,
        key,
        text: item.text,
        dueAt: toLocalInput(item.dueAt),
        done,
      }),
    });
    applyTodoPayload(data);
  } catch (err) {
    alert(err.message || "Не удалось обновить задание");
    renderWelcomeTodos();
    renderTodoDlg();
  }
}

async function deleteTodo(key, id) {
  try {
    const data = await api(`/api/todos?id=${encodeURIComponent(id)}&key=${encodeURIComponent(key)}`, {
      method: "DELETE",
    });
    applyTodoPayload(data);
  } catch (err) {
    alert(err.message || "Не удалось удалить задание");
  }
}

function bindTodoForm(formId, textId, dateId, timeId, keyOf) {
  const form = $(formId);
  if (!form) return;
  setTodoDueNow(dateId, timeId);
  bindTodoDatePicker(dateId);
  form.addEventListener("submit", async (e) => {
    e.preventDefault();
    const key = keyOf();
    const textEl = $(textId);
    const dateEl = $(dateId);
    const text = textEl ? textEl.value.trim() : "";
    const dueAt = todoDueFromInputs(dateId, timeId);
    if (!key) return;
    if (!text) {
      if (textEl) textEl.focus();
      return;
    }
    if (!dueAt) {
      if (dateEl) dateEl.focus();
      return;
    }
    try {
      await addTodo(key, text, dueAt);
      if (textEl) textEl.value = "";
      setTodoDueNow(dateId, timeId);
      if (textEl) textEl.focus();
    } catch (err) {
      alert(err.message || "Не удалось добавить задание");
    }
  });
}

function hideWelcomeCtx() {
  const menu = $("welcomeCtx");
  if (menu) menu.hidden = true;
}

function showWelcomeCtx(x, y) {
  const menu = $("welcomeCtx");
  if (!menu) return;
  hideCtx();
  menu.hidden = false;
  const pad = 8;
  const left = Math.min(x, window.innerWidth - menu.offsetWidth - pad);
  const top = Math.min(y, window.innerHeight - menu.offsetHeight - pad);
  menu.style.left = `${Math.max(pad, left)}px`;
  menu.style.top = `${Math.max(pad, top)}px`;
}

function bookFinished(key) {
  if (state.book && state.book.key === key) return Boolean(state.book.finished);
  const item = (state.library || []).find((book) => book.key === key);
  return Boolean(item && item.finished);
}

function finishedLabel(finished) {
  return finished ? "Снять отметку о прочтении" : "Отметить прочитанной";
}

function paintReadMarks() {
  const finished = Boolean(state.book && state.book.finished);
  const pageMark = $("pageReadMark");
  if (pageMark) pageMark.hidden = !finished;
  const ctxBtn = $("ctxFinished");
  if (ctxBtn) {
    ctxBtn.textContent = finishedLabel(finished);
    ctxBtn.classList.toggle("in", finished);
  }
}

async function setBookFinished(key, finished) {
  const payload = await api("/api/library/finished", {
    method: "PUT",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ key, finished }),
  });
  state.library = payload.library || [];
  if (payload.book && state.book && payload.book.key === state.book.key) {
    state.book.finished = Boolean(payload.book.finished);
  } else if (state.book && state.book.key === key) {
    state.book.finished = finished;
  }
  renderLibrary();
  if (state.book) {
    showBookChrome();
  } else {
    paintReadMarks();
  }
  syncBookNoteFields(false);
}

function bookMetaOf(key) {
  if (state.book && state.book.key === key) {
    return {
      title: state.book.title || "Без названия",
      description: state.book.description || "",
      journal: state.book.journal || "",
    };
  }
  const item = (state.library || []).find((book) => book.key === key);
  return {
    title: (item && item.title) || "Без названия",
    description: (item && item.description) || "",
    journal: (item && item.journal) || "",
  };
}

function applyBookMetaLocal(key, description, journal) {
  if (state.book && state.book.key === key) {
    state.book.description = description;
    state.book.journal = journal;
  }
  const item = (state.library || []).find((book) => book.key === key);
  if (item) {
    item.description = description;
    item.journal = journal;
  }
}

function fieldBusy(el) {
  return Boolean(el && document.activeElement === el);
}

function syncBookNoteFields(force) {
  const box = $("bookNoteBox");
  const journal = $("bookJournal");
  if (box) box.hidden = !state.book;
  if (state.book && journal) {
    const meta = bookMetaOf(state.book.key);
    if (force || !fieldBusy(journal)) journal.value = meta.journal;
  }
  const dlg = $("bookNoteDlg");
  if (!dlg || dlg.hidden || !state.bookNoteKey) return;
  const item = (state.library || []).find((book) => book.key === state.bookNoteKey);
  if (!item && !(state.book && state.book.key === state.bookNoteKey)) {
    closeBookNoteDlg(true);
    return;
  }
  const dlgMeta = bookMetaOf(state.bookNoteKey);
  const title = $("bookNoteTitle");
  if (title) title.textContent = dlgMeta.title;
  const dlgDesc = $("dlgDescription");
  const dlgJournal = $("dlgJournal");
  if (dlgDesc && (force || !fieldBusy(dlgDesc))) dlgDesc.value = dlgMeta.description;
  if (dlgJournal && (force || !fieldBusy(dlgJournal))) dlgJournal.value = dlgMeta.journal;
}

function bookNoteOpen() {
  const dlg = $("bookNoteDlg");
  return Boolean(dlg && !dlg.hidden);
}

function readBookNoteDraft(key) {
  const meta = bookMetaOf(key);
  let description = meta.description;
  let journal = meta.journal;
  if (state.book && state.book.key === key) {
    const area = $("bookJournal");
    if (area) journal = area.value;
  }
  if (bookNoteOpen() && state.bookNoteKey === key) {
    const dlgDesc = $("dlgDescription");
    const dlgJournal = $("dlgJournal");
    if (dlgDesc) description = dlgDesc.value;
    if (dlgJournal) journal = dlgJournal.value;
  }
  return { description, journal };
}

const bookNoteTimers = {};
function scheduleBookMetaSave(key) {
  if (!key) return;
  const draft = readBookNoteDraft(key);
  applyBookMetaLocal(key, draft.description, draft.journal);
  clearTimeout(bookNoteTimers[key]);
  bookNoteTimers[key] = setTimeout(() => saveBookMeta(key), 400);
}

async function saveBookMeta(key, silent) {
  if (!key) return;
  const draft = readBookNoteDraft(key);
  try {
    const payload = await api("/api/library/meta", {
      method: "PUT",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({
        key,
        description: draft.description,
        journal: draft.journal,
      }),
    });
    state.library = payload.library || state.library;
    applyGame(payload);
    if (payload.book && state.book && payload.book.key === state.book.key) {
      state.book.description = payload.book.description || "";
      state.book.journal = payload.book.journal || "";
    } else {
      applyBookMetaLocal(key, draft.description, draft.journal);
    }
    renderLibrary();
    syncBookNoteFields(false);
  } catch (err) {
    if (!silent) alert(err.message || "Не удалось сохранить заметку о книге");
  }
}

async function flushBookMeta() {
  const keys = new Set(Object.keys(bookNoteTimers));
  if (state.bookNoteKey) keys.add(state.bookNoteKey);
  if (state.book && state.book.key) keys.add(state.book.key);
  for (const key of keys) {
    clearTimeout(bookNoteTimers[key]);
    delete bookNoteTimers[key];
    if (key) await saveBookMeta(key, true);
  }
}

function openBookNoteDlg(key) {
  if (!key) return;
  const draft = readBookNoteDraft(key);
  applyBookMetaLocal(key, draft.description, draft.journal);
  state.bookNoteKey = key;
  const meta = bookMetaOf(key);
  const dlg = $("bookNoteDlg");
  if (!dlg) return;
  $("bookNoteTitle").textContent = meta.title;
  $("dlgDescription").value = draft.description;
  $("dlgJournal").value = draft.journal;
  hideCtx();
  hideShelfCtx();
  dlg.hidden = false;
  $("dlgJournal").focus();
}

function closeBookNoteDlg(skipSave) {
  const dlg = $("bookNoteDlg");
  if (!dlg || dlg.hidden) {
    if (!state.book) state.bookNoteKey = "";
    return;
  }
  if (!skipSave) {
    const key = state.bookNoteKey;
    if (key) {
      const draft = readBookNoteDraft(key);
      applyBookMetaLocal(key, draft.description, draft.journal);
      saveBookMeta(key, true);
    }
  }
  dlg.hidden = true;
  if (!state.book) state.bookNoteKey = "";
  else state.bookNoteKey = state.book.key;
  syncBookNoteFields(false);
}

let filePropsToken = 0;
let filePropsKey = "";

function filePropsDlgOpen() {
  const dlg = $("filePropsDlg");
  return Boolean(dlg && !dlg.hidden);
}

function closeFileProps() {
  filePropsToken += 1;
  filePropsKey = "";
  const dlg = $("filePropsDlg");
  if (dlg) dlg.hidden = true;
}

function formatFileSize(n) {
  const units = ["Б", "КБ", "МБ", "ГБ"];
  let v = Number(n);
  if (!Number.isFinite(v) || v < 0) v = 0;
  let i = 0;
  while (v >= 1024 && i < units.length - 1) {
    v /= 1024;
    i += 1;
  }
  const text = i === 0
    ? String(Math.round(v))
    : new Intl.NumberFormat("ru", { maximumFractionDigits: 1 }).format(v);
  return `${text} ${units[i]}`;
}

function formatFileTime(iso) {
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return "";
  return new Intl.DateTimeFormat("ru", {
    day: "numeric",
    month: "long",
    year: "numeric",
    hour: "2-digit",
    minute: "2-digit",
  }).format(d);
}

function filePropRow(label, value) {
  const row = document.createElement("div");
  row.className = "file-prop";
  const lab = document.createElement("span");
  lab.className = "label";
  lab.textContent = label;
  const val = document.createElement("span");
  val.className = "value";
  val.textContent = value;
  row.append(lab, val);
  return row;
}

function paintFileProps(data) {
  const list = $("filePropsList");
  const hint = $("filePropsHint");
  if (!list || !hint) return;
  list.replaceChildren();
  $("filePropsTitle").textContent = data.title || "Свойства";
  const rows = [];
  if (data.author) rows.push(["Автор", data.author]);
  rows.push(["Формат", formatLabel(data.format)]);
  if (data.name) rows.push(["Имя файла", data.name]);
  if (!data.bundled && !data.missing && typeof data.size === "number") {
    rows.push(["Размер", formatFileSize(data.size)]);
  }
  const modified = data.modified ? formatFileTime(data.modified) : "";
  if (modified) rows.push(["Изменён", modified]);
  if (data.path) rows.push(["Расположение", data.path]);
  for (const [label, value] of rows) list.appendChild(filePropRow(label, value));
  let note = "";
  if (data.bundled) note = "Эта книга встроена в программу, отдельного файла нет.";
  else if (data.missing) note = "Копия в библиотеке не найдена.";
  hint.hidden = !note;
  hint.textContent = note;
}

async function openFileProps(key) {
  if (!key) return;
  hideCtx();
  const dlg = $("filePropsDlg");
  const list = $("filePropsList");
  const hint = $("filePropsHint");
  if (!dlg || !list || !hint) return;
  const token = filePropsToken + 1;
  filePropsToken = token;
  filePropsKey = key;
  const known = (state.library || []).find((item) => item.key === key) || (state.book && state.book.key === key ? state.book : null);
  $("filePropsTitle").textContent = (known && known.title) || "Свойства";
  list.replaceChildren();
  hint.hidden = false;
  hint.textContent = "Читаю файл…";
  dlg.hidden = false;
  try {
    const data = await api(`/api/library/file?key=${encodeURIComponent(key)}`);
    if (token !== filePropsToken || dlg.hidden) return;
    paintFileProps(data);
  } catch (err) {
    if (token !== filePropsToken || dlg.hidden) return;
    list.replaceChildren();
    hint.hidden = false;
    hint.textContent = (err && err.message && err.message.trim()) || "Не удалось прочитать свойства файла";
  }
}

function hideShelfCtx() {
  const menu = $("shelfCtx");
  if (menu) menu.hidden = true;
  state.shelfCtxKey = "";
}

function otherWorkspaces() {
  const current = (state.workspace && state.workspace.id) || "";
  return (state.workspaces || []).filter((item) => item.id && item.id !== current);
}

function showShelfCtx(x, y, key) {
  const menu = $("shelfCtx");
  const box = $("shelfCtxLists");
  if (!menu || !box) return;
  hideCtx();
  hideWelcomeCtx();
  state.shelfCtxKey = key;
  const finishedBtn = $("shelfCtxFinished");
  if (finishedBtn) {
    const finished = bookFinished(key);
    finishedBtn.textContent = finishedLabel(finished);
    finishedBtn.classList.toggle("in", finished);
  }
  box.replaceChildren();
  const lists = state.lists || [];
  if (!lists.length) {
    const empty = document.createElement("p");
    empty.className = "ctx-empty";
    empty.textContent = "Пока нет списков. Введите название ниже.";
    box.appendChild(empty);
  }
  for (const item of lists) {
    const btn = document.createElement("button");
    btn.type = "button";
    btn.className = "ctx-item";
    const inside = bookInList(item, key);
    if (inside) btn.classList.add("in");
    btn.textContent = inside ? `✓ ${item.name}` : item.name;
    btn.addEventListener("click", async () => {
      hideShelfCtx();
      try {
        if (inside) await removeFromList(item.id, key);
        else await addToList(item.id, key);
      } catch (err) {
        alert(err.message || "Не удалось обновить список");
      }
    });
    box.appendChild(btn);
  }
  const wsBox = $("shelfCtxWorkspaces");
  if (wsBox) {
    wsBox.replaceChildren();
    const workspaces = otherWorkspaces();
    if (!workspaces.length) {
      const empty = document.createElement("p");
      empty.className = "ctx-empty";
      empty.textContent = "Пока нет других пространств. Введите название ниже.";
      wsBox.appendChild(empty);
    }
    for (const item of workspaces) {
      const btn = document.createElement("button");
      btn.type = "button";
      btn.className = "ctx-item";
      btn.textContent = item.name;
      btn.addEventListener("click", async () => {
        hideShelfCtx();
        try {
          await moveBook(key, { id: item.id });
        } catch (err) {
          alert(err.message || "Не удалось перенести книгу");
        }
      });
      wsBox.appendChild(btn);
    }
  }
  menu.hidden = false;
  const nameInput = $("shelfCtxName");
  if (nameInput) nameInput.value = "";
  const wsInput = $("shelfCtxWorkspaceName");
  if (wsInput) wsInput.value = "";
  const pad = 8;
  const left = Math.min(x, window.innerWidth - menu.offsetWidth - pad);
  const top = Math.min(y, window.innerHeight - menu.offsetHeight - pad);
  menu.style.left = `${Math.max(pad, left)}px`;
  menu.style.top = `${Math.max(pad, top)}px`;
}

async function moveBook(key, dest) {
  if (!key) return;
  if (state.bookNoteKey === key) closeBookNoteDlg(true);
  if (state.todoDlgKey === key) closeTodoDlg();
  const body = { key };
  if (dest && dest.id) body.id = dest.id;
  if (dest && dest.name) body.name = dest.name;
  const payload = await api("/api/library/move", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(body),
  });
  await applyState(payload, Boolean(payload.book));
}

async function switchWorkspace(id) {
  try {
    await flushBookMeta();
    if (state.book) await saveProgress();
    const payload = await api("/api/workspaces/current", {
      method: "PUT",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ id }),
    });
    await applyState(payload, false);
  } catch (err) {
    alert(err.message || "Не удалось переключить пространство");
  }
}

function formatLabel(fmt) {
  if (fmt === "fb2") return "FB2";
  if (fmt === "demo") return "демо";
  if (fmt === "guide") return "справка";
  if (fmt === "txt" || fmt === "md") return "TXT";
  return "EPUB";
}

const SHELF_COLS = 5;
const SHELF_ROWS = 1;

function shelfSlotCount(n) {
  const min = SHELF_COLS * SHELF_ROWS;
  if (n <= min) return min;
  return Math.ceil(n / SHELF_COLS) * SHELF_COLS;
}

function addBookDlgOpen() {
  const dlg = $("addBookDlg");
  return Boolean(dlg && !dlg.hidden);
}

function showAddBookDlg() {
  const dlg = $("addBookDlg");
  if (!dlg) return;
  hideWelcomeCtx();
  hideShelfCtx();
  closeAddToListDlg();
  dlg.hidden = false;
}

function closeAddBookDlg() {
  const dlg = $("addBookDlg");
  if (dlg) dlg.hidden = true;
}

function addToListDlgOpen() {
  const dlg = $("addToListDlg");
  return Boolean(dlg && !dlg.hidden);
}

function booksNotInList(list) {
  return (state.library || []).filter((item) => !bookInList(list, item.key));
}

function showAddToListDlg() {
  const dlg = $("addToListDlg");
  const list = openedList();
  if (!dlg || !list) return;
  hideWelcomeCtx();
  hideShelfCtx();
  closeAddBookDlg();
  const eyebrow = $("addToListEyebrow");
  if (eyebrow) eyebrow.textContent = list.name || "Список";
  renderAddToListGrid();
  dlg.hidden = false;
}

function closeAddToListDlg() {
  const dlg = $("addToListDlg");
  if (dlg) dlg.hidden = true;
}

function onShelfAdd() {
  if (state.openedListId) showAddToListDlg();
  else showAddBookDlg();
}

function fillBookCard(card, item) {
  if (item.finished) card.classList.add("read");
  if (item.coverUrl) {
    const img = document.createElement("img");
    img.src = item.coverUrl;
    img.alt = "";
    card.appendChild(img);
  } else {
    const ph = document.createElement("div");
    ph.className = "shelf-cover";
    ph.textContent = ((item.title || "?").trim().charAt(0) || "?").toUpperCase();
    card.appendChild(ph);
  }
  const body = document.createElement("div");
  const h = document.createElement("h3");
  h.textContent = item.title || "Без названия";
  body.appendChild(h);
  const author = document.createElement("p");
  author.textContent = item.author || "";
  body.appendChild(author);
  const meta = document.createElement("p");
  meta.className = "meta";
  const bits = [formatLabel(item.format)];
  if (item.finished) bits.push("прочитано");
  else if (item.chapterN) bits.push(`${item.percent}%`);
  if (item.journal) bits.push("заметка");
  if (!item.canOpen) {
    bits.push("нет файла");
    card.classList.add("dead");
  }
  meta.textContent = bits.join(" · ");
  body.appendChild(meta);
  card.appendChild(body);
  if (item.finished) {
    const mark = document.createElement("span");
    mark.className = "shelf-read";
    mark.textContent = "Прочитано";
    card.appendChild(mark);
  }
}

function renderAddToListGrid() {
  const grid = $("addToListGrid");
  const list = openedList();
  if (!grid) return;
  grid.replaceChildren();
  const books = booksNotInList(list);
  if (!books.length) {
    const empty = document.createElement("p");
    empty.className = "add-to-list-empty";
    empty.textContent = (state.library || []).length
      ? "Все книги полки уже в этом списке."
      : "На полке пока нет книг. Выберите файл ниже.";
    grid.appendChild(empty);
    return;
  }
  for (const item of books) {
    const card = document.createElement("button");
    card.type = "button";
    card.className = "shelf-card";
    fillBookCard(card, item);
    card.addEventListener("click", () => addPickedBook(item.key));
    grid.appendChild(card);
  }
}

async function addPickedBook(key) {
  const list = openedList();
  if (!list || !key) return;
  try {
    await addToList(list.id, key);
    if (!addToListDlgOpen()) return;
    if (!booksNotInList(openedList()).length) closeAddToListDlg();
    else renderAddToListGrid();
  } catch (err) {
    alert(err.message || "Не удалось добавить в список");
  }
}

async function addOpenedListBook(key) {
  const listId = state.openedListId;
  if (!listId || !key) return;
  try {
    await addToList(listId, key);
  } catch (err) {
    alert(err.message || "Не удалось добавить в список");
  }
}

function renderShelfPlaceholder() {
  const card = document.createElement("button");
  card.type = "button";
  card.className = "shelf-card shelf-placeholder";
  const labelText = state.openedListId ? "Добавить в список" : "Добавить книгу";
  card.setAttribute("aria-label", labelText);
  const cover = document.createElement("div");
  cover.className = "shelf-cover";
  cover.textContent = "+";
  card.appendChild(cover);
  const label = document.createElement("p");
  label.className = "shelf-placeholder-label";
  label.textContent = "Добавить";
  card.appendChild(label);
  card.addEventListener("click", onShelfAdd);
  return card;
}

function renderShelfCard(item, listId) {
  const card = document.createElement("button");
  card.type = "button";
  card.className = "shelf-card";
  fillBookCard(card, item);
  const remove = document.createElement("button");
  remove.type = "button";
  remove.className = "remove";
  remove.setAttribute("aria-label", listId ? "Убрать из списка" : "Убрать с полки");
  remove.textContent = "×";
  remove.addEventListener("click", (e) => {
    e.stopPropagation();
    if (listId) {
      removeFromList(listId, item.key).catch((err) => {
        alert(err.message || "Не удалось убрать из списка");
      });
      return;
    }
    removeFromShelf(item.key);
  });
  card.appendChild(remove);
  if (item.canOpen) {
    card.addEventListener("click", () => openFromShelf(item.key));
  }
  card.addEventListener("contextmenu", (e) => {
    e.preventDefault();
    e.stopPropagation();
    showShelfCtx(e.clientX, e.clientY, item.key);
  });
  return card;
}

function shelfSearchPlace(hit) {
  const lines = [];
  const wsName = hit.workspace && hit.workspace.name;
  if (wsName) lines.push(`Пространство · ${wsName}`);
  const lists = (hit.lists || []).map((item) => item.name).filter(Boolean);
  if (lists.length) lines.push(`Список · ${lists.join(", ")}`);
  return lines;
}

function clearShelfSearch() {
  state.shelfQuery = "";
  state.shelfHits = [];
  state.shelfHitIndex = -1;
  const input = $("shelfSearchInput");
  if (input) input.value = "";
  renderShelfSearchResults([]);
}

function renderShelfSearchResults(hits) {
  const box = $("shelfSearchResults");
  if (!box) return;
  box.replaceChildren();
  if (!state.shelfQuery) {
    box.hidden = true;
    return;
  }
  box.hidden = false;
  if (!hits.length) {
    const empty = document.createElement("p");
    empty.className = "shelf-search-empty";
    empty.textContent = "Нет книг с таким названием";
    box.appendChild(empty);
    return;
  }
  hits.forEach((hit, i) => {
    const btn = document.createElement("button");
    btn.type = "button";
    btn.className = "shelf-search-hit";
    if (i === state.shelfHitIndex) btn.classList.add("active");
    if (hit.coverUrl) {
      const img = document.createElement("img");
      img.src = hit.coverUrl;
      img.alt = "";
      btn.appendChild(img);
    } else {
      const ph = document.createElement("div");
      ph.className = "shelf-search-cover";
      ph.textContent = ((hit.title || "?").trim().charAt(0) || "?").toUpperCase();
      btn.appendChild(ph);
    }
    const body = document.createElement("div");
    const title = document.createElement("strong");
    title.textContent = hit.title || "Без названия";
    body.appendChild(title);
    if (hit.author) {
      const author = document.createElement("span");
      author.className = "meta";
      author.textContent = hit.author;
      body.appendChild(author);
    }
    for (const line of shelfSearchPlace(hit)) {
      const meta = document.createElement("span");
      meta.className = "meta";
      meta.textContent = line;
      body.appendChild(meta);
    }
    btn.appendChild(body);
    btn.addEventListener("click", () => openLibraryHit(hit));
    box.appendChild(btn);
  });
}

let shelfSearchTimer = 0;

async function runShelfSearch(q) {
  state.shelfQuery = q.trim();
  state.shelfHitIndex = -1;
  if (!state.shelfQuery) {
    state.shelfHits = [];
    renderShelfSearchResults([]);
    return;
  }
  try {
    const data = await api(`/api/library/search?q=${encodeURIComponent(state.shelfQuery)}`);
    if (data.query !== state.shelfQuery) return;
    state.shelfHits = data.hits || [];
    state.shelfHitIndex = state.shelfHits.length ? 0 : -1;
    renderShelfSearchResults(state.shelfHits);
  } catch (err) {
    state.shelfHits = [];
    renderShelfSearchResults([]);
    console.warn(err);
  }
}

async function openLibraryHit(hit) {
  if (!hit || !hit.key) return;
  try {
    const wsId = hit.workspace && hit.workspace.id;
    if (wsId && state.workspace && wsId !== state.workspace.id) {
      await flushBookMeta();
      if (state.book) await saveProgress();
      const payload = await api("/api/workspaces/current", {
        method: "PUT",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ id: wsId }),
      });
      await applyState(payload, false);
    }
    if (!hit.canOpen) {
      alert("Файл книги недоступен");
      return;
    }
    await openFromShelf(hit.key);
    clearShelfSearch();
  } catch (err) {
    alert(err.message || "Не удалось открыть книгу");
  }
}

function renderLibrary() {
  const shelf = $("shelf");
  const grid = $("shelfGrid");
  syncListSelection();
  const listView = openedList();
  const current = listView || selectedList();
  const shelfLabel = $("shelfLabel");
  if (shelfLabel) {
    const wsName = (state.workspace && state.workspace.name) || "";
    if (listView) {
      shelfLabel.textContent = listView.name || "Список";
    } else if (current) {
      shelfLabel.textContent = `Список · ${current.name}`;
    } else {
      shelfLabel.textContent = wsName ? `Полка · ${wsName}` : "Полка";
    }
  }
  const back = $("listBackBtn");
  if (back) back.hidden = !listView || Boolean(state.book);
  const addBtn = $("shelfAddBtn");
  if (addBtn) {
    const label = listView ? "Добавить в список" : "Добавить книгу";
    addBtn.setAttribute("aria-label", label);
    addBtn.title = label;
  }
  document.body.classList.toggle("list-view", Boolean(listView) && !state.book);
  const items = current
    ? (state.library || []).filter((item) => bookInList(current, item.key))
    : (state.library || []);
  shelf.hidden = Boolean(state.book);
  if (!state.book) $("welcome").hidden = Boolean(listView);
  grid.replaceChildren();
  const slots = shelfSlotCount(items.length);
  for (let i = 0; i < slots; i += 1) {
    const item = items[i];
    grid.appendChild(item ? renderShelfCard(item, listView ? listView.id : "") : renderShelfPlaceholder());
  }
  if (addToListDlgOpen()) renderAddToListGrid();
  if (!state.book) {
    const title = listView ? (listView.name || "Список") : ((state.workspace && state.workspace.name) || "boo");
    if ($("topTitle")) $("topTitle").textContent = title;
    document.title = `${title} — boo`;
  }
}

function showBookChrome() {
  const hasBook = Boolean(state.book);
  document.body.classList.toggle("reading", hasBook);
  $("welcome").hidden = hasBook || Boolean(openedList());
  $("shelf").hidden = hasBook;
  if (hasBook) {
    closeAddBookDlg();
    closeAddToListDlg();
  }
  $("reader").hidden = !hasBook;
  $("progressBar").hidden = !hasBook;
  $("sidebarProgress").hidden = !hasBook;
  $("notesBtn").hidden = !hasBook;
  $("historyBtn").hidden = !hasBook;
  $("readStatsBtn").hidden = !hasBook;
  $("shelfBtn").hidden = !hasBook;
  if (!hasBook) closeReadStatsDlg();
  applySidebar();
  applyNotes();
  applyHistory();
  applyWorkspaces();
  applyLists();
  renderWorkspaces();
  renderLists();
  renderLibrary();
  renderWelcomeTodos();
  renderHistory();
  renderReadStats();
  renderUndo();
  renderBookmarks();
  renderDictionary();
  renderNotes();
  renderReadTimer();
  if (!hasBook) {
    const listView = openedList();
    const wsName = (state.workspace && state.workspace.name) || "boo";
    const title = listView ? (listView.name || "Список") : wsName;
    $("topTitle").textContent = title;
    document.title = `${title} — boo`;
    if ($("toc")) $("toc").replaceChildren();
    if (!bookNoteOpen()) state.bookNoteKey = "";
    syncBookNoteFields(false);
    paintReadMarks();
    paintChapterRead();
    return;
  }
  $("topTitle").textContent = state.book.title;
  document.title = `${state.book.title} — boo`;
  const meta = $("bookMeta");
  meta.replaceChildren();
  if (state.book.coverUrl) {
    const img = document.createElement("img");
    img.src = state.book.coverUrl;
    img.alt = "";
    meta.appendChild(img);
  }
  const h = document.createElement("h2");
  h.textContent = state.book.title;
  meta.appendChild(h);
  if (state.book.author) {
    const p = document.createElement("p");
    p.textContent = state.book.author;
    meta.appendChild(p);
  }
  if (state.book.finished) {
    const mark = document.createElement("p");
    mark.className = "read-mark";
    mark.textContent = "Прочитано";
    meta.appendChild(mark);
  }
  state.bookNoteKey = state.bookNoteKey || state.book.key;
  syncBookNoteFields(false);
  paintReadMarks();
  paintChapterRead();
  refreshTOC();
}

function escapeRegExp(s) {
  return s.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
}

function highlightQuery(root, query, activeIndex) {
  const q = (query || "").trim();
  if (!q) return 0;
  const re = new RegExp(escapeRegExp(q), "gi");
  const walker = document.createTreeWalker(root, NodeFilter.SHOW_TEXT);
  const nodes = [];
  while (walker.nextNode()) nodes.push(walker.currentNode);
  let count = 0;
  for (const node of nodes) {
    const text = node.nodeValue;
    if (!text || !re.test(text)) continue;
    re.lastIndex = 0;
    const frag = document.createDocumentFragment();
    let last = 0;
    let m;
    while ((m = re.exec(text))) {
      if (m.index > last) {
        frag.appendChild(document.createTextNode(text.slice(last, m.index)));
      }
      const mark = document.createElement("mark");
      mark.className = "search-mark";
      if (count === activeIndex) mark.classList.add("current");
      mark.textContent = m[0];
      frag.appendChild(mark);
      count += 1;
      last = m.index + m[0].length;
    }
    if (last < text.length) {
      frag.appendChild(document.createTextNode(text.slice(last)));
    }
    node.parentNode.replaceChild(frag, node);
  }
  const cur = root.querySelector("mark.search-mark.current") || root.querySelector("mark.search-mark");
  if (cur) cur.scrollIntoView({ block: "center", behavior: "smooth" });
  return count;
}

function renderSearchResults(hits) {
  const box = $("searchResults");
  state.hits = hits || [];
  if (!state.query.trim()) {
    box.hidden = true;
    box.replaceChildren();
    return;
  }
  box.hidden = false;
  box.replaceChildren();
  if (!state.hits.length) {
    const empty = document.createElement("p");
    empty.className = "search-empty";
    empty.textContent = "Ничего не найдено";
    box.appendChild(empty);
    return;
  }
  state.hits.forEach((hit, i) => {
    const btn = document.createElement("button");
    btn.type = "button";
    btn.className = "search-hit";
    const title = document.createElement("strong");
    title.textContent = hit.title || `Глава ${hit.chapterIndex + 1}`;
    const snip = document.createElement("span");
    snip.textContent = hit.snippet || "";
    btn.appendChild(title);
    btn.appendChild(snip);
    btn.addEventListener("click", () => {
      openChapter(hit.chapterIndex, "", true, state.query, hit.offset);
    });
    if (i === 0) btn.classList.add("active");
    box.appendChild(btn);
  });
}

let searchTimer = 0;
async function runSearch(query) {
  state.query = query.trim();
  if (!state.book || state.query.length < 2) {
    renderSearchResults([]);
    return;
  }
  try {
    const data = await api(`/api/search?q=${encodeURIComponent(state.query)}`);
    renderSearchResults(data.hits || []);
  } catch (err) {
    console.error(err);
  }
}

function textOffset(root, container, offset) {
  if (container.nodeType !== Node.TEXT_NODE) {
    if (offset < container.childNodes.length) {
      const startAt = container.childNodes[offset];
      const walker = document.createTreeWalker(root, NodeFilter.SHOW_TEXT);
      while (walker.nextNode()) {
        const node = walker.currentNode;
        if (node === startAt || (startAt.contains && startAt.contains(node))) {
          return textOffset(root, node, 0);
        }
      }
    }
    const inner = document.createTreeWalker(container, NodeFilter.SHOW_TEXT);
    let last = null;
    while (inner.nextNode()) last = inner.currentNode;
    if (last) return textOffset(root, last, last.nodeValue.length);
    const after = document.createTreeWalker(root, NodeFilter.SHOW_TEXT);
    while (after.nextNode()) {
      if (container.compareDocumentPosition(after.currentNode) & Node.DOCUMENT_POSITION_FOLLOWING) {
        return textOffset(root, after.currentNode, 0);
      }
    }
    let pos = 0;
    const all = document.createTreeWalker(root, NodeFilter.SHOW_TEXT);
    while (all.nextNode()) pos += all.currentNode.nodeValue.length;
    return pos;
  }
  const walker = document.createTreeWalker(root, NodeFilter.SHOW_TEXT);
  let pos = 0;
  while (walker.nextNode()) {
    if (walker.currentNode === container) {
      return pos + Math.max(0, Math.min(offset, container.nodeValue.length));
    }
    pos += walker.currentNode.nodeValue.length;
  }
  return -1;
}

function rangeFromOffsets(root, start, end) {
  const walker = document.createTreeWalker(root, NodeFilter.SHOW_TEXT);
  let pos = 0;
  let startNode = null;
  let startOff = 0;
  let endNode = null;
  let endOff = 0;
  while (walker.nextNode()) {
    const node = walker.currentNode;
    const len = node.nodeValue.length;
    if (!startNode && start <= pos + len) {
      startNode = node;
      startOff = start - pos;
    }
    if (end <= pos + len) {
      endNode = node;
      endOff = end - pos;
      break;
    }
    pos += len;
  }
  if (!startNode || !endNode) return null;
  const range = document.createRange();
  range.setStart(startNode, Math.max(0, Math.min(startOff, startNode.nodeValue.length)));
  range.setEnd(endNode, Math.max(0, Math.min(endOff, endNode.nodeValue.length)));
  return range.collapsed ? null : range;
}

function wrapRange(range, hl) {
  const mark = document.createElement("mark");
  mark.className = `hl ${hl.color || "yellow"}`;
  mark.dataset.hlId = hl.id;
  try {
    mark.appendChild(range.extractContents());
    range.insertNode(mark);
  } catch {
    return;
  }
}

function applyHighlights(root, list) {
  const items = (list || [])
    .filter((h) => h.chapterIndex === state.chapterIndex && h.end > h.start)
    .slice()
    .sort((a, b) => b.start - a.start);
  for (const hl of items) {
    const range = rangeFromOffsets(root, hl.start, hl.end);
    if (range) wrapRange(range, hl);
  }
}

function wrapNoteRange(range, note) {
  const mark = document.createElement("mark");
  mark.className = `note ${note.color || "yellow"}`;
  mark.dataset.noteId = note.id;
  if (state.activeNoteId === note.id) mark.classList.add("active");
  try {
    mark.appendChild(range.extractContents());
    range.insertNode(mark);
  } catch {
    return;
  }
}

function applyNoteMarks(root, list) {
  const items = (list || [])
    .filter((n) => n.chapterIndex === state.chapterIndex && n.end > n.start)
    .slice()
    .sort((a, b) => b.start - a.start);
  for (const note of items) {
    const range = rangeFromOffsets(root, note.start, note.end);
    if (range) wrapNoteRange(range, note);
  }
}

function paintContent(query, hitOffset) {
  hideNoteTip();
  hideDictTip();
  const root = $("content");
  root.innerHTML = state.chapterHTML;
  applyHighlights(root, state.highlights);
  applyNoteMarks(root, state.notes);
  bindContentLinks();
  if (query) highlightQuery(root, query, hitOffset || 0);
}

function refreshChapter() {
  const reader = $("reader");
  const top = reader.scrollTop;
  paintContent(state.query, 0);
  reader.scrollTop = top;
}

function captureSelection() {
  const sel = window.getSelection();
  if (!sel || sel.rangeCount === 0 || sel.isCollapsed) return null;
  const root = $("content");
  const range = sel.getRangeAt(0);
  if (!root.contains(range.commonAncestorContainer)) return null;
  const start = textOffset(root, range.startContainer, range.startOffset);
  const end = textOffset(root, range.endContainer, range.endOffset);
  if (start < 0 || end <= start) return null;
  const text = range.toString().replace(/\s+/g, " ").trim();
  if (!text) return null;
  return { start, end, text };
}

let savedSel = null;
let ctxMarkId = "";
let ctxNoteId = "";

function hideCtx() {
  const menu = $("ctxMenu");
  if (menu) menu.hidden = true;
  ctxMarkId = "";
  ctxNoteId = "";
  hideShelfCtx();
  hideWelcomeCtx();
  hideNoteTip();
}

function showCtx(x, y, opts) {
  hideShelfCtx();
  hideWelcomeCtx();
  const menu = $("ctxMenu");
  const hasSel = Boolean(opts && opts.sel);
  const hlId = (opts && opts.hlId) || "";
  const noteId = (opts && opts.noteId) || "";
  const dictWord = (opts && opts.dictWord) || "";
  $("ctxHighlightBox").hidden = !hasSel && !hlId;
  $("ctxNoteBox").hidden = !hasSel && !noteId;
  $("ctxRemove").hidden = !hlId || hasSel;
  $("ctxNoteRemove").hidden = !noteId || hasSel;
  paintDictCtx(dictWord);
  paintReadMarks();
  paintChapterRead();
  menu.hidden = false;
  const pad = 8;
  const left = Math.min(x, window.innerWidth - menu.offsetWidth - pad);
  const top = Math.min(y, window.innerHeight - menu.offsetHeight - pad);
  menu.style.left = `${Math.max(pad, left)}px`;
  menu.style.top = `${Math.max(pad, top)}px`;
}

async function openChapter(index, fragment, resetScroll, query, hitOffset) {
  if (!state.book || index < 0 || index >= state.book.chapterN) return;
  hideCtx();
  const data = await api(`/api/chapter?i=${index}`);
  state.chapterIndex = data.index;
  state.chapterFragment = fragment || "";
  state.chapterHTML = data.html;
  paintContent(query, hitOffset);
  const heading = $("content").querySelector("h1, h2");
  const sameTitle = heading && heading.textContent.trim() === (data.title || "").trim();
  $("chapterTitle").textContent = data.title || "";
  $("chapterTitle").hidden = !data.title || sameTitle;
  paintChapterRead();
  $("prevBtn").disabled = index <= 0;
  $("nextBtn").disabled = index >= state.book.chapterN - 1;
  refreshTOC();
  const reader = $("reader");
  if (resetScroll) reader.scrollTop = 0;
  if (!query && fragment) {
    const target = document.getElementById(fragment);
    if (target) target.scrollIntoView();
  }
  setProgress(resetScroll ? 0 : scrollRatio(), index);
  scheduleSave();
}

async function addHighlight(color, sel) {
  if (!state.book || !sel) return;
  try {
    const data = await api("/api/highlights", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({
        chapterIndex: state.chapterIndex,
        start: sel.start,
        end: sel.end,
        color,
        text: sel.text,
      }),
    });
    state.highlights = data.highlights || [];
    refreshChapter();
    window.getSelection()?.removeAllRanges();
    savedSel = null;
  } catch (err) {
    alert(err.message || "Не удалось сохранить выделение");
  }
}

async function recolorHighlight(id, color) {
  try {
    const data = await api("/api/highlights", {
      method: "PUT",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ id, color }),
    });
    state.highlights = data.highlights || [];
    refreshChapter();
  } catch (err) {
    alert(err.message || "Не удалось изменить цвет");
  }
}

async function deleteHighlight(id) {
  try {
    const data = await api(`/api/highlights?id=${encodeURIComponent(id)}`, { method: "DELETE" });
    state.highlights = data.highlights || [];
    refreshChapter();
  } catch (err) {
    alert(err.message || "Не удалось убрать выделение");
  }
}

function renderNotes() {
  const list = $("noteList");
  if (!list) return;
  list.replaceChildren();
  if (!state.book) return;
  if (!state.notes.length) {
    const empty = document.createElement("p");
    empty.className = "note-empty";
    empty.textContent = "Выделите фрагмент в тексте и добавьте заметку через контекстное меню или кнопку «Добавить».";
    list.appendChild(empty);
    return;
  }
  for (const note of state.notes) {
    const card = document.createElement("article");
    card.className = `note-card ${note.color || "yellow"}`;
    card.dataset.noteCard = note.id;
    if (state.activeNoteId === note.id) card.classList.add("active");
    const quote = document.createElement("button");
    quote.type = "button";
    quote.className = "quote";
    quote.textContent = note.text || "Фрагмент";
    quote.addEventListener("click", () => openNote(note));
    const meta = document.createElement("span");
    meta.className = "meta";
    meta.textContent = `Глава ${(note.chapterIndex || 0) + 1}`;
    const area = document.createElement("textarea");
    area.placeholder = "Текст заметки…";
    area.value = note.body || "";
    area.addEventListener("focus", () => {
      state.activeNoteId = note.id;
      markActiveNote(note.id);
    });
    area.addEventListener("input", () => {
      note.body = area.value;
      scheduleNoteSave(note);
    });
    const tools = document.createElement("div");
    tools.className = "note-tools";
    const colors = document.createElement("div");
    colors.className = "ctx-colors";
    for (const color of NOTE_COLORS) {
      const sw = document.createElement("button");
      sw.type = "button";
      sw.className = `ctx-swatch ${color}`;
      sw.setAttribute("aria-label", color);
      sw.addEventListener("click", () => updateNote(note.id, { color, body: note.body || "" }));
      colors.appendChild(sw);
    }
    const remove = document.createElement("button");
    remove.type = "button";
    remove.className = "text-btn";
    remove.textContent = "Убрать";
    remove.addEventListener("click", () => deleteNote(note.id));
    tools.appendChild(colors);
    tools.appendChild(remove);
    card.appendChild(quote);
    card.appendChild(meta);
    card.appendChild(area);
    card.appendChild(tools);
    list.appendChild(card);
  }
}

function markActiveNote(id) {
  $("content").querySelectorAll("mark.note").forEach((el) => {
    el.classList.toggle("active", el.dataset.noteId === id);
  });
  $("noteList").querySelectorAll(".note-card").forEach((el) => {
    el.classList.toggle("active", el.dataset.noteCard === id);
  });
}

const NOTE_TIP_DELAY = 600;
let noteTipTimer = 0;

function hideNoteTip() {
  clearTimeout(noteTipTimer);
  noteTipTimer = 0;
  const tip = $("noteTip");
  if (!tip) return;
  tip.hidden = true;
  tip.textContent = "";
  tip.className = "note-tip";
}

function positionNoteTip(tip, anchor) {
  const pad = 10;
  const gap = 8;
  const rect = anchor.getBoundingClientRect();
  const tw = tip.offsetWidth;
  const th = tip.offsetHeight;
  let left = rect.left + rect.width / 2 - tw / 2;
  left = Math.max(pad, Math.min(left, window.innerWidth - tw - pad));
  let top = rect.top - th - gap;
  if (top < pad) top = rect.bottom + gap;
  if (top + th > window.innerHeight - pad) {
    top = Math.max(pad, window.innerHeight - th - pad);
  }
  tip.style.left = `${Math.round(left)}px`;
  tip.style.top = `${Math.round(top)}px`;
}

function showNoteTip(mark) {
  if (!mark.isConnected) return;
  const note = state.notes.find((n) => n.id === mark.dataset.noteId);
  const body = note && (note.body || "").trim();
  if (!body) return;
  const tip = $("noteTip");
  if (!tip) return;
  tip.className = `note-tip ${note.color || "yellow"}`;
  tip.textContent = body;
  tip.hidden = false;
  positionNoteTip(tip, mark);
}

function scheduleNoteTip(mark) {
  hideNoteTip();
  const note = state.notes.find((n) => n.id === mark.dataset.noteId);
  if (!note || !(note.body || "").trim()) return;
  noteTipTimer = setTimeout(() => showNoteTip(mark), NOTE_TIP_DELAY);
}

function noteMarkFrom(el) {
  return el && el.closest ? el.closest("mark.note") : null;
}

function noteHasBody(mark) {
  const note = mark && state.notes.find((n) => n.id === mark.dataset.noteId);
  return Boolean(note && (note.body || "").trim());
}

let ctxDictWord = "";
let dictTipWord = "";
let dictMoveTimer = 0;
let dictLookupSeq = 0;
const dictCache = new Map();
const DICT_HOVER_DELAY = 180;

function hideDictTip() {
  clearTimeout(dictMoveTimer);
  dictMoveTimer = 0;
  dictLookupSeq += 1;
  dictTipWord = "";
  const tip = $("dictTip");
  if (!tip) return;
  tip.hidden = true;
  if ($("dictTipWord")) $("dictTipWord").textContent = "";
  if ($("dictTipText")) $("dictTipText").textContent = "";
  if ($("dictTipMeta")) $("dictTipMeta").textContent = "";
}

function pointAnchor(x, y) {
  return {
    getBoundingClientRect() {
      return { left: x, right: x, top: y - 6, bottom: y + 8, width: 0, height: 14 };
    },
  };
}

function selectionAnchor() {
  const sel = window.getSelection();
  if (!sel || sel.rangeCount === 0 || sel.isCollapsed) return null;
  const rect = sel.getRangeAt(0).getBoundingClientRect();
  if (!rect || (rect.width === 0 && rect.height === 0)) return null;
  return { getBoundingClientRect: () => rect };
}

function dictQueryFromText(text) {
  const q = (text || "").replace(/\s+/g, " ").trim();
  if (!q || q.length > 40) return "";
  if (!/[\p{L}]/u.test(q)) return "";
  return q;
}

function expandWord(text, offset) {
  if (!text || offset < 0 || offset > text.length) return "";
  const isWord = (ch) => /[\p{L}\p{M}\p{N}'’ʼ-]/u.test(ch);
  let i = offset;
  if (i === text.length || (!isWord(text[i]) && i > 0 && isWord(text[i - 1]))) i -= 1;
  if (i < 0 || i >= text.length || !isWord(text[i])) return "";
  let start = i;
  let end = i + 1;
  while (start > 0 && isWord(text[start - 1])) start -= 1;
  while (end < text.length && isWord(text[end])) end += 1;
  return text.slice(start, end).replace(/^['’ʼ-]+|['’ʼ-]+$/g, "");
}

function wordAtPoint(x, y) {
  const root = $("content");
  if (!root) return "";
  let node = null;
  let offset = 0;
  if (document.caretPositionFromPoint) {
    const pos = document.caretPositionFromPoint(x, y);
    if (pos) {
      node = pos.offsetNode;
      offset = pos.offset;
    }
  } else if (document.caretRangeFromPoint) {
    const range = document.caretRangeFromPoint(x, y);
    if (range) {
      node = range.startContainer;
      offset = range.startOffset;
    }
  }
  if (!node || node.nodeType !== Node.TEXT_NODE || !root.contains(node)) return "";
  return expandWord(node.nodeValue || "", offset);
}

function showDictTip(data, anchor) {
  const tip = $("dictTip");
  if (!tip || !data || !data.found) return;
  const entries = data.entries || [];
  const word = (entries[0] && entries[0].word) || data.query || "";
  const text = entries.map((item) => item.text).filter(Boolean).join("\n");
  if (!text) return;
  dictTipWord = data.query || word;
  $("dictTipWord").textContent = word;
  $("dictTipText").textContent = text;
  const pair = dictPairLabel({ fromLang: data.fromLang, toLang: data.toLang });
  $("dictTipMeta").textContent = pair || data.name || "";
  tip.hidden = false;
  if (anchor) positionNoteTip(tip, anchor);
}

async function lookupDict(query, anchor) {
  const q = dictQueryFromText(query);
  if (!q || !dictActive()) {
    hideDictTip();
    return;
  }
  if (dictTipWord && dictTipWord.toLowerCase() === q.toLowerCase() && $("dictTip") && !$("dictTip").hidden) {
    if (anchor) positionNoteTip($("dictTip"), anchor);
    return;
  }
  const cached = dictCache.get(`${state.book.dictionaryId}:${q.toLowerCase()}`);
  if (cached) {
    if (cached.found) showDictTip(cached, anchor);
    else hideDictTip();
    return;
  }
  const seq = ++dictLookupSeq;
  try {
    const data = await api(`/api/dictionaries/lookup?q=${encodeURIComponent(q)}`);
    dictCache.set(`${state.book.dictionaryId}:${q.toLowerCase()}`, data);
    if (seq !== dictLookupSeq) return;
    if (data.found) showDictTip(data, anchor);
    else hideDictTip();
  } catch {
    if (seq === dictLookupSeq) hideDictTip();
  }
}

function scheduleDictAt(x, y) {
  if (!dictActive()) return;
  clearTimeout(dictMoveTimer);
  dictMoveTimer = setTimeout(() => {
    const word = wordAtPoint(x, y);
    if (!word) {
      hideDictTip();
      return;
    }
    lookupDict(word, pointAnchor(x, y));
  }, DICT_HOVER_DELAY);
}

function translateSelectionOrWord(word, x, y) {
  const q = dictQueryFromText(word);
  if (!q) return;
  if (!dictActive()) {
    setSidebarOpen(true);
    return;
  }
  lookupDict(q, selectionAnchor() || pointAnchor(x, y));
}

function paintDictCtx(word) {
  ctxDictWord = dictQueryFromText(word);
  const box = $("ctxDictBox");
  const translate = $("ctxTranslate");
  const toggle = $("ctxDictToggle");
  if (!box) return;
  const hasBook = Boolean(state.book);
  box.hidden = !hasBook;
  if (translate) {
    translate.hidden = !hasBook || !ctxDictWord;
    translate.textContent = ctxDictWord ? `Перевести «${ctxDictWord}»` : "Перевести";
  }
  if (toggle) {
    const items = state.dictionaries || [];
    if (!items.length) {
      toggle.textContent = "Подключить словарь…";
    } else if (dictActive()) {
      const item = activeDictionary();
      toggle.textContent = item ? `Выключить · ${item.name}` : "Выключить словарь";
    } else {
      toggle.textContent = "Включить словарь";
    }
  }
}

const noteSaveTimers = {};
function scheduleNoteSave(note) {
  clearTimeout(noteSaveTimers[note.id]);
  noteSaveTimers[note.id] = setTimeout(() => {
    updateNote(note.id, { color: note.color, body: note.body || "" }, true);
  }, 400);
}

async function addNote(color, sel) {
  if (!state.book || !sel) return;
  try {
    const data = await api("/api/notes", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({
        chapterIndex: state.chapterIndex,
        start: sel.start,
        end: sel.end,
        color,
        text: sel.text,
        body: "",
      }),
    });
    state.notes = data.notes || [];
    applyGame(data);
    state.activeNoteId = data.note && data.note.id ? data.note.id : "";
    refreshChapter();
    window.getSelection()?.removeAllRanges();
    savedSel = null;
    setNotesOpen(true);
    renderNotes();
    const card = document.querySelector(`[data-note-card="${state.activeNoteId}"]`);
    const area = card && card.querySelector("textarea");
    if (area) {
      area.focus();
      card.scrollIntoView({ block: "nearest" });
    }
  } catch (err) {
    alert(err.message || "Не удалось сохранить заметку");
  }
}

async function updateNote(id, patch, silent) {
  const current = state.notes.find((n) => n.id === id);
  try {
    const data = await api("/api/notes", {
      method: "PUT",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({
        id,
        color: patch.color || (current && current.color) || "yellow",
        body: patch.body != null ? patch.body : (current && current.body) || "",
      }),
    });
    state.notes = data.notes || [];
    applyGame(data);
    if (data.note && data.note.id) state.activeNoteId = data.note.id;
    if (!silent) {
      refreshChapter();
      renderNotes();
    } else {
      const next = state.notes.find((n) => n.id === id);
      if (next && current) {
        current.color = next.color;
        current.body = next.body;
      }
    }
  } catch (err) {
    if (!silent) alert(err.message || "Не удалось изменить заметку");
  }
}

async function deleteNote(id) {
  try {
    const data = await api(`/api/notes?id=${encodeURIComponent(id)}`, { method: "DELETE" });
    state.notes = data.notes || [];
    if (data.undo) state.undo = data.undo;
    if (state.activeNoteId === id) state.activeNoteId = "";
    refreshChapter();
    renderNotes();
    renderUndo();
  } catch (err) {
    alert(err.message || "Не удалось убрать заметку");
  }
}

async function openNote(note) {
  state.activeNoteId = note.id;
  if (note.chapterIndex !== state.chapterIndex) {
    await openChapter(note.chapterIndex, "", false);
  }
  setNotesOpen(true);
  renderNotes();
  markActiveNote(note.id);
  const mark = $("content").querySelector(`mark.note[data-note-id="${note.id}"]`);
  if (mark) mark.scrollIntoView({ block: "center" });
  const card = document.querySelector(`[data-note-card="${note.id}"]`);
  if (card) card.scrollIntoView({ block: "nearest" });
}

function addNoteFromSelection(color) {
  const sel = savedSel || captureSelection();
  if (!sel) {
    setNotesOpen(true);
    return;
  }
  addNote(color || "yellow", sel);
}

function bindContentLinks() {
  $("content").querySelectorAll("a[data-href], a[data-fragment]").forEach((a) => {
    a.addEventListener("click", (e) => {
      const href = a.getAttribute("data-href");
      const fragment = a.getAttribute("data-fragment") || "";
      if (!href && fragment) {
        e.preventDefault();
        const target = document.getElementById(fragment);
        if (target) target.scrollIntoView();
        return;
      }
      if (!href || !state.book) return;
      const chapter = state.book.chapters.find((ch) => ch.href === href);
      if (!chapter) return;
      e.preventDefault();
      openChapter(chapter.index, fragment, true);
    });
  });
}

async function applyState(payload, restore) {
  const nextKey = (payload.book && payload.book.key) || "";
  const curKey = (state.book && state.book.key) || "";
  if (curKey && curKey !== nextKey) {
    const committed = await commitReadTimer();
    if (committed) {
      if (committed.history) payload.history = committed.history;
      if (committed.readStats) payload.readStats = committed.readStats;
    }
  }
  state.book = payload.book;
  state.workspace = payload.workspace || { id: "", name: "Библиотека" };
  state.workspaces = payload.workspaces || [];
  state.library = payload.library || [];
  state.lists = payload.lists || [];
  syncListSelection();
  state.bookmarks = payload.bookmarks || [];
  state.highlights = payload.highlights || [];
  state.notes = payload.notes || [];
  state.todos = payload.todos || [];
  state.todoBook = payload.todoBook || null;
  state.history = payload.history || [];
  state.undo = payload.undo || [];
  applyReadStats(payload);
  if (todoDlgOpen()) {
    const dlgKey = state.todoDlgKey;
    const still = dlgKey && (payload.library || []).some((item) => item.key === dlgKey);
    if (!still) closeTodoDlg();
    else if (state.todoBook && state.todoBook.key === dlgKey) state.todoDlgTodos = state.todos;
  }
  if (filePropsDlgOpen() && (!filePropsKey || !(payload.library || []).some((item) => item.key === filePropsKey))) {
    closeFileProps();
  }
  state.activeNoteId = "";
  state.tocFold = payload.tocFold || [];
  state.readChapters = payload.readChapters || [];
  state.readTOC = payload.readTOC || [];
  state.dictionaries = payload.dictionaries || [];
  dictCache.clear();
  hideDictTip();
  state.query = "";
  state.hits = [];
  if ($("searchInput")) $("searchInput").value = "";
  renderSearchResults([]);
  if (payload.ui) state.ui = payload.ui;
  if (payload.drive) renderDrive(payload.drive);
  applyGame(payload);
  applyUI();
  if (bookNoteOpen() && state.bookNoteKey) {
    const still = (payload.library || []).some((item) => item.key === state.bookNoteKey);
    if (!still) closeBookNoteDlg(true);
  }
  showBookChrome();
  if (!state.book) return;
  let index = 0;
  let ratio = 0;
  if (restore && payload.progress) {
    index = payload.progress.chapterIndex || 0;
    ratio = payload.progress.scrollRatio || 0;
  }
  await openChapter(index, "", false);
  const reader = $("reader");
  const max = reader.scrollHeight - reader.clientHeight;
  reader.scrollTop = max > 0 ? max * ratio : 0;
  setProgress(ratio, index);
}

async function uploadFile(file) {
  if (!file) return;
  try {
    if (state.book) await saveProgress();
    const body = new FormData();
    body.append("file", file);
    const payload = await api("/api/open", { method: "POST", body });
    const key = payload.book && payload.book.key;
    await applyState(payload, true);
    await addOpenedListBook(key);
  } catch (err) {
    alert(err.message || "Не удалось открыть файл");
  }
}

async function boot() {
  fillFontSelects();
  applyUI();
  const payload = await api("/api/state");
  await applyState(payload, true);
}

$("listCreateBtn").addEventListener("click", () => setListFormOpen(true));
$("listFormCancel").addEventListener("click", () => setListFormOpen(false));
$("listForm").addEventListener("submit", async (e) => {
  e.preventDefault();
  const name = $("listName").value.trim();
  if (!name) {
    $("listName").focus();
    return;
  }
  try {
    await createList(name);
  } catch (err) {
    alert(err.message || "Не удалось создать список");
  }
});
function bindBookNoteField(id, source) {
  const el = $(id);
  if (!el) return;
  el.addEventListener("input", () => {
    const key = source === "dlg" ? state.bookNoteKey : (state.book && state.book.key);
    if (!key) return;
    const pair = {
      bookJournal: "dlgJournal",
      dlgJournal: "bookJournal",
    }[id];
    const other = pair && $(pair);
    if (other && state.book && state.book.key === key && (source === "dlg" || bookNoteOpen())) {
      other.value = el.value;
    }
    scheduleBookMetaSave(key);
  });
}
bindBookNoteField("bookJournal", "sidebar");
bindBookNoteField("dlgDescription", "dlg");
bindBookNoteField("dlgJournal", "dlg");
$("bookNoteExpand").addEventListener("click", () => {
  if (state.book) openBookNoteDlg(state.book.key);
});
$("bookNoteClose").addEventListener("click", () => closeBookNoteDlg());
$("bookNoteBackdrop").addEventListener("click", () => closeBookNoteDlg());
$("shelfCtxNote").addEventListener("click", () => {
  const key = state.shelfCtxKey;
  hideShelfCtx();
  if (key) openBookNoteDlg(key);
});
$("ctxBookNote").addEventListener("click", () => {
  hideCtx();
  if (state.book) {
    setSidebarOpen(true);
    openBookNoteDlg(state.book.key);
  }
});
$("shelfCtxTodo").addEventListener("click", () => {
  const key = state.shelfCtxKey;
  hideShelfCtx();
  if (key) openTodoDlg(key);
});
$("ctxTodo").addEventListener("click", () => {
  hideCtx();
  if (state.book) openTodoDlg(state.book.key);
});
$("shelfCtxFileProps").addEventListener("click", () => {
  const key = state.shelfCtxKey;
  if (key) openFileProps(key);
});
$("ctxFileProps").addEventListener("click", () => {
  if (state.book) openFileProps(state.book.key);
});
$("filePropsClose").addEventListener("click", closeFileProps);
$("filePropsBackdrop").addEventListener("click", closeFileProps);
$("todoDlgClose").addEventListener("click", () => closeTodoDlg());
$("todoBackdrop").addEventListener("click", () => closeTodoDlg());
bindTodoForm("welcomeTodoForm", "welcomeTodoText", "welcomeTodoDate", "welcomeTodoTime", () => state.todoBook && state.todoBook.key);
bindTodoForm("dlgTodoForm", "dlgTodoText", "dlgTodoDate", "dlgTodoTime", () => state.todoDlgKey);
bindHistoryForm("welcomeHistoryForm", "welcomeHistoryText", historyBookKey);
bindHistoryForm("readerHistoryForm", "readerHistoryText", () => state.book && state.book.key);
$("shelfCtxFinished").addEventListener("click", async () => {
  const key = state.shelfCtxKey;
  if (!key) {
    hideShelfCtx();
    return;
  }
  const finished = !bookFinished(key);
  hideShelfCtx();
  try {
    await setBookFinished(key, finished);
  } catch (err) {
    alert(err.message || "Не удалось отметить книгу");
  }
});
$("ctxFinished").addEventListener("click", async () => {
  if (!state.book) {
    hideCtx();
    return;
  }
  const finished = !state.book.finished;
  hideCtx();
  try {
    await setBookFinished(state.book.key, finished);
  } catch (err) {
    alert(err.message || "Не удалось отметить книгу");
  }
});
$("ctxChapterRead").addEventListener("click", async () => {
  if (!state.book) {
    hideCtx();
    return;
  }
  const read = !chapterRead(state.chapterIndex);
  hideCtx();
  try {
    await setChapterRead(state.chapterIndex, read);
  } catch (err) {
    alert(err.message || "Не удалось отметить главу");
  }
});
$("pageChapterRead").addEventListener("click", async () => {
  if (!state.book) return;
  try {
    await setChapterRead(state.chapterIndex, !chapterRead(state.chapterIndex));
  } catch (err) {
    alert(err.message || "Не удалось отметить главу");
  }
});
$("shelfCtxForm").addEventListener("submit", async (e) => {
  e.preventDefault();
  const name = $("shelfCtxName").value.trim();
  const key = state.shelfCtxKey;
  if (!name) {
    $("shelfCtxName").focus();
    return;
  }
  if (!key) {
    hideShelfCtx();
    return;
  }
  hideShelfCtx();
  try {
    await createList(name, key);
  } catch (err) {
    alert(err.message || "Не удалось создать список");
  }
});
$("shelfCtxWorkspaceForm").addEventListener("submit", async (e) => {
  e.preventDefault();
  const name = $("shelfCtxWorkspaceName").value.trim();
  const key = state.shelfCtxKey;
  if (!name) {
    $("shelfCtxWorkspaceName").focus();
    return;
  }
  if (!key) {
    hideShelfCtx();
    return;
  }
  hideShelfCtx();
  try {
    await moveBook(key, { name });
  } catch (err) {
    alert(err.message || "Не удалось перенести книгу");
  }
});
$("workspaceCreateBtn").addEventListener("click", () => setWorkspaceFormOpen(true));
$("workspaceRenameBtn").addEventListener("click", () => {
  const current = state.workspace || {};
  if (!current.id) return;
  setWorkspaceFormOpen(true, current.id);
});
$("workspaceFormCancel").addEventListener("click", () => setWorkspaceFormOpen(false));
$("workspaceForm").addEventListener("submit", async (e) => {
  e.preventDefault();
  const name = $("workspaceName").value.trim();
  if (!name) {
    $("workspaceName").focus();
    return;
  }
  const renameId = $("workspaceForm").dataset.renameId || "";
  try {
    if (renameId) {
      await renameWorkspace(renameId, name);
    } else {
      if (state.book) await saveProgress();
      await createWorkspace(name);
    }
  } catch (err) {
    alert(err.message || (renameId ? "Не удалось переименовать пространство" : "Не удалось создать пространство"));
  }
});
function pickBookFile() {
  const input = $("fileInput");
  if (!input) return;
  input.value = "";
  input.click();
}

async function openDemo() {
  try {
    const payload = await api("/api/demo", { method: "POST" });
    const key = payload.book && payload.book.key;
    await applyState(payload, true);
    await addOpenedListBook(key);
  } catch (err) {
    alert(err.message || "Не удалось открыть демо");
  }
}

async function openGuide() {
  if (state.book && state.book.format === "guide") return;
  try {
    if (state.book) await saveProgress();
    const payload = await api("/api/guide", { method: "POST" });
    await applyState(payload, true);
  } catch (err) {
    alert(err.message || "Не удалось открыть руководство");
  }
}

function filenameFromDisposition(header) {
  if (!header) return "";
  const utf = /filename\*=UTF-8''([^;]+)/i.exec(header);
  if (utf) {
    try {
      return decodeURIComponent(utf[1]);
    } catch {
      return utf[1];
    }
  }
  const plain = /filename="([^"]+)"/i.exec(header) || /filename=([^;]+)/i.exec(header);
  return plain ? plain[1].trim() : "";
}

async function exportLibrary() {
  const btn = $("exportBtn");
  if (btn) {
    btn.disabled = true;
    btn.textContent = "Экспорт…";
  }
  try {
    if (state.book) await saveProgress();
    const res = await fetch("/api/export");
    if (!res.ok) {
      const text = await res.text();
      throw new Error(text || res.statusText);
    }
    const blob = await res.blob();
    const name = filenameFromDisposition(res.headers.get("content-disposition")) || "boo-library.zip";
    const url = URL.createObjectURL(blob);
    const a = document.createElement("a");
    a.href = url;
    a.download = name;
    a.click();
    URL.revokeObjectURL(url);
  } catch (err) {
    alert(err.message || "Не удалось экспортировать библиотеку");
  } finally {
    if (btn) {
      btn.disabled = false;
      btn.textContent = "Экспорт";
    }
  }
}

async function importLibrary(file) {
  if (!file) return;
  if (!confirm("Импорт заменит текущую библиотеку, прогресс, заметки и словари. Продолжить?")) return;
  const btn = $("importBtn");
  if (btn) {
    btn.disabled = true;
    btn.textContent = "Импорт…";
  }
  try {
    if (state.book) await saveProgress();
    const body = new FormData();
    body.append("file", file);
    const payload = await api("/api/import", { method: "POST", body });
    state.selectedListId = "";
    await applyState(payload, false);
  } catch (err) {
    alert(err.message || "Не удалось импортировать библиотеку");
  } finally {
    if (btn) {
      btn.disabled = false;
      btn.textContent = "Импорт";
    }
  }
}

function pickImportFile() {
  const input = $("importInput");
  if (!input) return;
  input.value = "";
  input.click();
}

let driveBusy = false;

function driveStatusText(d) {
  if (!d) return "";
  if (d.lastError && !d.connected) return d.lastError;
  if (d.connected) {
    let msg = d.email ? `Подключено: ${d.email}.` : "Google Диск подключён.";
    if (d.lastSync) {
      const t = new Date(d.lastSync);
      if (!Number.isNaN(t.getTime()) && t.getFullYear() > 2000) {
        msg += ` Последняя синхронизация: ${t.toLocaleString("ru")}.`;
      }
    }
    if (d.lastError) msg += " " + d.lastError;
    return msg;
  }
  if (d.hasCredentials) {
    return "Ключ сохранён. Нажмите «Подключить» — откроется окно входа в Google.";
  }
  return "Книги и настройки можно держать в папке boo на Google Диске. Сначала вставьте ключ приложения из Google Cloud, затем подключите диск. Пока не подключите, в сеть ничего не уходит.";
}

function renderDrive(d) {
  state.drive = d || {};
  const hint = $("driveHint");
  if (hint) hint.textContent = driveStatusText(state.drive);
  const creds = $("driveCreds");
  const connectBtn = $("driveConnectBtn");
  const syncBtn = $("driveSyncBtn");
  const disconnectBtn = $("driveDisconnectBtn");
  const connected = Boolean(state.drive.connected);
  if (creds) creds.hidden = connected;
  if (connectBtn) {
    connectBtn.hidden = connected;
    connectBtn.disabled = driveBusy;
    connectBtn.textContent = driveBusy ? "Вход…" : "Подключить";
  }
  if (syncBtn) {
    syncBtn.hidden = !connected;
    syncBtn.disabled = driveBusy;
    if (!driveBusy) syncBtn.textContent = "Синхронизировать";
  }
  if (disconnectBtn) {
    disconnectBtn.hidden = !connected;
    disconnectBtn.disabled = driveBusy;
  }
}

async function saveDriveCredentials() {
  try {
    const payload = await api("/api/drive/credentials", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({
        clientId: $("driveClientId").value.trim(),
        clientSecret: $("driveClientSecret").value.trim(),
      }),
    });
    const secret = $("driveClientSecret");
    if (secret) secret.value = "";
    renderDrive(payload);
  } catch (err) {
    alert(err.message || "Не удалось сохранить ключ Google");
  }
}

async function connectDrive() {
  if (driveBusy) return;
  driveBusy = true;
  renderDrive(state.drive);
  try {
    const payload = await api("/api/drive/connect", { method: "POST" });
    if (payload.url) window.open(payload.url, "_blank", "noopener");
    const deadline = Date.now() + 3 * 60 * 1000;
    while (Date.now() < deadline) {
      await new Promise((resolve) => setTimeout(resolve, 1000));
      const st = await api("/api/drive");
      renderDrive(st);
      if (st.connected) return;
    }
    alert("Не дождались входа в Google. Если окно браузера открылось — завершите вход и нажмите «Подключить» снова.");
  } catch (err) {
    alert(err.message || "Не удалось подключить Google Диск");
    try {
      renderDrive(await api("/api/drive"));
    } catch {
      renderDrive(state.drive);
    }
  } finally {
    driveBusy = false;
    renderDrive(state.drive);
  }
}

async function syncDrive() {
  if (driveBusy) return;
  const btn = $("driveSyncBtn");
  driveBusy = true;
  if (btn) {
    btn.disabled = true;
    btn.textContent = "Синхронизация…";
  }
  try {
    if (state.book) await saveProgress();
    const payload = await api("/api/drive/sync", { method: "POST" });
    await applyState(payload, Boolean(payload.book));
  } catch (err) {
    alert(err.message || "Не удалось синхронизировать с Google Диском");
    try {
      renderDrive(await api("/api/drive"));
    } catch {
      /* ignore */
    }
  } finally {
    driveBusy = false;
    if (btn) btn.textContent = "Синхронизировать";
    renderDrive(state.drive);
  }
}

async function disconnectDrive() {
  if (!confirm("Отключить Google Диск на этом компьютере? Файлы на Диске останутся.")) return;
  try {
    const payload = await api("/api/drive/disconnect", { method: "POST" });
    await applyState(payload, Boolean(payload.book));
  } catch (err) {
    alert(err.message || "Не удалось отключить Google Диск");
  }
}

$("exportBtn").addEventListener("click", () => exportLibrary());
$("importBtn").addEventListener("click", pickImportFile);
$("importInput").addEventListener("change", (e) => importLibrary(e.target.files[0]));
$("driveSaveCreds").addEventListener("click", saveDriveCredentials);
$("driveConnectBtn").addEventListener("click", connectDrive);
$("driveSyncBtn").addEventListener("click", syncDrive);
$("driveDisconnectBtn").addEventListener("click", disconnectDrive);
$("fileInput").addEventListener("change", (e) => uploadFile(e.target.files[0]));
$("shelfAddBtn").addEventListener("click", onShelfAdd);
$("listBackBtn").addEventListener("click", closeListScreen);
$("addBookClose").addEventListener("click", closeAddBookDlg);
$("addBookBackdrop").addEventListener("click", closeAddBookDlg);
$("addToListClose").addEventListener("click", closeAddToListDlg);
$("addToListBackdrop").addEventListener("click", closeAddToListDlg);
$("addToListFile").addEventListener("click", () => {
  closeAddToListDlg();
  pickBookFile();
});
$("addBookFile").addEventListener("click", () => {
  closeAddBookDlg();
  pickBookFile();
});
$("addBookDemo").addEventListener("click", () => {
  closeAddBookDlg();
  openDemo();
});
$("addBookGuide").addEventListener("click", () => {
  closeAddBookDlg();
  openGuide();
});
$("shelf").addEventListener("contextmenu", (e) => {
  if (e.target.closest(".shelf-card, input, textarea, a, button")) return;
  e.preventDefault();
  if (state.openedListId) showAddToListDlg();
  else showWelcomeCtx(e.clientX, e.clientY);
});
$("welcomeCtxAdd").addEventListener("click", () => {
  hideWelcomeCtx();
  pickBookFile();
});
$("welcomeCtxDemo").addEventListener("click", () => {
  hideWelcomeCtx();
  openDemo();
});
$("welcomeCtxGuide").addEventListener("click", () => {
  hideWelcomeCtx();
  openGuide();
});
$("guideBtn").addEventListener("click", () => openGuide());
$("welcome").addEventListener("contextmenu", (e) => {
  if (e.target.closest(".shelf-card, .todo-box, .history-box, .transfer-box, input, textarea, a, button")) return;
  e.preventDefault();
  showWelcomeCtx(e.clientX, e.clientY);
});
async function goShelf() {
  try {
    await flushBookMeta();
    await saveProgress();
    const payload = await api("/api/close", { method: "POST" });
    await applyState(payload, false);
  } catch (err) {
    alert(err.message || "Не удалось вернуться на полку");
  }
}

async function openFromShelf(key) {
  try {
    if (state.book) await saveProgress();
    const payload = await api("/api/library/open", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ key }),
    });
    await applyState(payload, true);
  } catch (err) {
    alert(err.message || "Не удалось открыть книгу");
  }
}

async function removeFromShelf(key) {
  try {
    if (state.bookNoteKey === key) closeBookNoteDlg(true);
    if (state.todoDlgKey === key) closeTodoDlg();
    const payload = await api(`/api/library?key=${encodeURIComponent(key)}`, { method: "DELETE" });
    await applyState(payload, Boolean(payload.book));
  } catch (err) {
    alert(err.message || "Не удалось убрать книгу");
  }
}

$("searchInput").addEventListener("input", (e) => {
  clearTimeout(searchTimer);
  searchTimer = setTimeout(() => runSearch(e.target.value), 220);
});
$("searchInput").addEventListener("keydown", (e) => {
  if (e.key === "Enter" && state.hits.length) {
    e.preventDefault();
    const hit = state.hits[0];
    openChapter(hit.chapterIndex, "", true, state.query, hit.offset);
  }
});
$("shelfSearchInput").addEventListener("input", (e) => {
  clearTimeout(shelfSearchTimer);
  shelfSearchTimer = setTimeout(() => runShelfSearch(e.target.value), 220);
});
$("shelfSearchInput").addEventListener("focus", () => {
  if (state.shelfQuery) renderShelfSearchResults(state.shelfHits);
});
$("shelfSearchInput").addEventListener("keydown", (e) => {
  if (e.key === "ArrowDown" && state.shelfHits.length) {
    e.preventDefault();
    state.shelfHitIndex = Math.min(state.shelfHits.length - 1, Math.max(0, state.shelfHitIndex) + 1);
    renderShelfSearchResults(state.shelfHits);
    return;
  }
  if (e.key === "ArrowUp" && state.shelfHits.length) {
    e.preventDefault();
    state.shelfHitIndex = Math.max(0, (state.shelfHitIndex < 0 ? 0 : state.shelfHitIndex) - 1);
    renderShelfSearchResults(state.shelfHits);
    return;
  }
  if (e.key === "Enter" && state.shelfHits.length) {
    e.preventDefault();
    const hit = state.shelfHits[Math.max(0, state.shelfHitIndex)] || state.shelfHits[0];
    openLibraryHit(hit);
  }
});
$("shelfBtn").addEventListener("click", goShelf);
$("tocFold").addEventListener("click", toggleTocSection);
$("tocHideRead").addEventListener("click", toggleHideReadChapters);
$("tocBtn").addEventListener("click", () => setSidebarOpen(!state.ui.sidebarOpen));
$("closeToc").addEventListener("click", () => setSidebarOpen(false));
$("sidebarPeek").addEventListener("click", () => setSidebarOpen(true));
$("notesBtn").addEventListener("click", () => setNotesOpen(!state.ui.notesOpen));
$("closeNotes").addEventListener("click", () => setNotesOpen(false));
$("notesRail").addEventListener("click", () => setNotesOpen(true));
$("noteAdd").addEventListener("click", () => addNoteFromSelection("yellow"));
$("historyBtn").addEventListener("click", () => setHistoryOpen(!state.ui.historyOpen));
$("readStatsBtn").addEventListener("click", openReadStatsDlg);
$("readTimerStart").addEventListener("click", startReadTimer);
$("readTimerPause").addEventListener("click", pauseReadTimer);
$("readTimerStop").addEventListener("click", () => stopReadTimer());
$("readTimerStats").addEventListener("click", openReadStatsDlg);
$("readStatsClose").addEventListener("click", closeReadStatsDlg);
$("levelUpOk").addEventListener("click", closeLevelUp);
$("levelUpBackdrop").addEventListener("click", closeLevelUp);
$("gameEnabled").addEventListener("change", async () => {
  const box = $("gameEnabled");
  state.ui.gamificationDisabled = !box.checked;
  try {
    await saveUI();
  } catch (err) {
    state.ui.gamificationDisabled = !state.ui.gamificationDisabled;
    applyUI();
    alert(err.message || "Не удалось сохранить настройку");
    return;
  }
  renderGame();
  if (!state.ui.gamificationDisabled) {
    try {
      const payload = await api("/api/state");
      if (payload.ui) state.ui = payload.ui;
      applyUI();
      applyGame(payload);
    } catch (err) {
      console.warn(err);
    }
  }
});
$("readStatsBackdrop").addEventListener("click", closeReadStatsDlg);
$("closeHistory").addEventListener("click", () => setHistoryOpen(false));
$("historyRail").addEventListener("click", () => setHistoryOpen(true));
$("workspacesBtn").addEventListener("click", () => setWorkspacesOpen(!state.ui.workspacesOpen));
$("closeWorkspaces").addEventListener("click", () => setWorkspacesOpen(false));
$("workspacesRail").addEventListener("click", () => setWorkspacesOpen(true));
$("listsBtn").addEventListener("click", () => setListsOpen(!state.ui.listsOpen));
$("closeLists").addEventListener("click", () => setListsOpen(false));
$("listsRail").addEventListener("click", () => setListsOpen(true));
if ($("undoLastBtn")) $("undoLastBtn").addEventListener("click", () => undoLast());
if ($("readerUndoLastBtn")) $("readerUndoLastBtn").addEventListener("click", () => undoLast());

let sidebarDrag = null;
$("sidebarResizer").addEventListener("pointerdown", (e) => {
  e.preventDefault();
  sidebarDrag = { x: e.clientX, w: state.ui.sidebarWidth || 280 };
  document.body.classList.add("sidebar-resizing");
  e.currentTarget.setPointerCapture(e.pointerId);
});
$("sidebarResizer").addEventListener("pointermove", (e) => {
  if (!sidebarDrag) return;
  state.ui.sidebarWidth = clampSidebarWidth(sidebarDrag.w + (e.clientX - sidebarDrag.x));
  applySidebar();
});
$("sidebarResizer").addEventListener("pointerup", () => {
  if (!sidebarDrag) return;
  sidebarDrag = null;
  document.body.classList.remove("sidebar-resizing");
  saveUI();
});
$("sidebarResizer").addEventListener("dblclick", () => {
  state.ui.sidebarWidth = 280;
  applySidebar();
  saveUI();
});

let notesDrag = null;
$("notesResizer").addEventListener("pointerdown", (e) => {
  e.preventDefault();
  notesDrag = { x: e.clientX, w: state.ui.notesWidth || 300 };
  document.body.classList.add("notes-resizing");
  e.currentTarget.setPointerCapture(e.pointerId);
});
$("notesResizer").addEventListener("pointermove", (e) => {
  if (!notesDrag) return;
  state.ui.notesWidth = clampNotesWidth(notesDrag.w - (e.clientX - notesDrag.x));
  applyNotes();
});
$("notesResizer").addEventListener("pointerup", () => {
  if (!notesDrag) return;
  notesDrag = null;
  document.body.classList.remove("notes-resizing");
  saveUI();
});
$("notesResizer").addEventListener("dblclick", () => {
  state.ui.notesWidth = 300;
  applyNotes();
  saveUI();
});

let historyDrag = null;
$("historyResizer").addEventListener("pointerdown", (e) => {
  e.preventDefault();
  historyDrag = { x: e.clientX, w: state.ui.historyWidth || 280 };
  document.body.classList.add("history-resizing");
  e.currentTarget.setPointerCapture(e.pointerId);
});
$("historyResizer").addEventListener("pointermove", (e) => {
  if (!historyDrag) return;
  state.ui.historyWidth = clampHistoryWidth(historyDrag.w - (e.clientX - historyDrag.x));
  applyHistory();
});
$("historyResizer").addEventListener("pointerup", () => {
  if (!historyDrag) return;
  historyDrag = null;
  document.body.classList.remove("history-resizing");
  saveUI();
});
$("historyResizer").addEventListener("dblclick", () => {
  state.ui.historyWidth = 280;
  applyHistory();
  saveUI();
});

let workspacesDrag = null;
$("workspacesResizer").addEventListener("pointerdown", (e) => {
  e.preventDefault();
  workspacesDrag = { x: e.clientX, w: state.ui.workspacesWidth || 280 };
  document.body.classList.add("workspaces-resizing");
  e.currentTarget.setPointerCapture(e.pointerId);
});
$("workspacesResizer").addEventListener("pointermove", (e) => {
  if (!workspacesDrag) return;
  state.ui.workspacesWidth = clampWorkspacesWidth(workspacesDrag.w - (e.clientX - workspacesDrag.x));
  applyWorkspaces();
});
$("workspacesResizer").addEventListener("pointerup", () => {
  if (!workspacesDrag) return;
  workspacesDrag = null;
  document.body.classList.remove("workspaces-resizing");
  saveUI();
});
$("workspacesResizer").addEventListener("dblclick", () => {
  state.ui.workspacesWidth = 280;
  applyWorkspaces();
  saveUI();
});

let listsDrag = null;
$("listsResizer").addEventListener("pointerdown", (e) => {
  e.preventDefault();
  listsDrag = { x: e.clientX, w: state.ui.listsWidth || 280 };
  document.body.classList.add("lists-resizing");
  e.currentTarget.setPointerCapture(e.pointerId);
});
$("listsResizer").addEventListener("pointermove", (e) => {
  if (!listsDrag) return;
  state.ui.listsWidth = clampListsWidth(listsDrag.w - (e.clientX - listsDrag.x));
  applyLists();
});
$("listsResizer").addEventListener("pointerup", () => {
  if (!listsDrag) return;
  listsDrag = null;
  document.body.classList.remove("lists-resizing");
  saveUI();
});
$("listsResizer").addEventListener("dblclick", () => {
  state.ui.listsWidth = 280;
  applyLists();
  saveUI();
});
$("prevBtn").addEventListener("click", () => openChapter(state.chapterIndex - 1, "", true));
$("nextBtn").addEventListener("click", () => openChapter(state.chapterIndex + 1, "", true));
fillFontSelects();
$("fontDown").addEventListener("click", () => bumpBookFont(-1));
$("fontUp").addEventListener("click", () => bumpBookFont(1));
function bindFontSelect(id, field) {
  const el = $(id);
  if (!el) return;
  el.addEventListener("change", () => {
    const font = fontById(el.value, field === "bookFont" ? "serif" : "system");
    state.ui[field] = font.id;
    applyUI();
    saveUI();
  });
}
function bindFontRange(id, kind) {
  const el = $(id);
  if (!el) return;
  el.addEventListener("input", () => {
    if (kind === "book") {
      state.ui.bookFontSize = clampBookFontSize(el.value);
      state.ui.fontSize = state.ui.bookFontSize;
    } else {
      state.ui.uiFontSize = clampUIFontSize(el.value);
    }
    applyUI();
  });
  el.addEventListener("change", () => saveUI());
}
bindFontSelect("bookFont", "bookFont");
bindFontSelect("bookFontMenu", "bookFont");
bindFontSelect("uiFont", "uiFont");
bindFontSelect("uiFontMenu", "uiFont");
bindFontRange("bookFontRange", "book");
bindFontRange("bookFontRangeMenu", "book");
bindFontRange("uiFontRange", "ui");
bindFontRange("uiFontRangeMenu", "ui");
$("bookFontDown").addEventListener("click", () => bumpBookFont(-1));
$("bookFontUp").addEventListener("click", () => bumpBookFont(1));
$("bookFontDownMenu").addEventListener("click", () => bumpBookFont(-1));
$("bookFontUpMenu").addEventListener("click", () => bumpBookFont(1));
$("uiFontDown").addEventListener("click", () => bumpUIFont(-1));
$("uiFontUp").addEventListener("click", () => bumpUIFont(1));
$("uiFontDownMenu").addEventListener("click", () => bumpUIFont(-1));
$("uiFontUpMenu").addEventListener("click", () => bumpUIFont(1));
$("bookmarkBtn").addEventListener("click", addBookmark);
$("bookmarkAdd").addEventListener("click", addBookmark);
$("dictAdd").addEventListener("click", pickDictionaryFile);
$("dictInput").addEventListener("change", (e) => uploadDictionary(e.target.files[0]));
$("welcomeBgInput").addEventListener("change", (e) => {
  const input = e.currentTarget;
  uploadWelcomeBackground(input.files[0]);
  input.value = "";
});
$("welcomeBgClear").addEventListener("click", () => clearWelcomeBackground());
$("ctxTranslate").addEventListener("click", (e) => {
  const word = ctxDictWord;
  hideCtx();
  translateSelectionOrWord(word, e.clientX, e.clientY);
});
$("ctxDictToggle").addEventListener("click", () => {
  hideCtx();
  toggleDictionary();
});
$("themeBtn").addEventListener("click", () => {
  const i = themes.indexOf(state.ui.theme);
  state.ui.theme = themes[(i + 1) % themes.length];
  saveUI();
});
$("widthDown").addEventListener("click", () => bumpPageWidth(-2));
$("widthUp").addEventListener("click", () => bumpPageWidth(2));
$("widthDownMenu").addEventListener("click", () => bumpPageWidth(-2));
$("widthUpMenu").addEventListener("click", () => bumpPageWidth(2));
$("widthRange").addEventListener("input", (e) => {
  state.ui.maxWidth = clampPageWidth(e.target.value);
  applyPageWidth();
});
$("widthRange").addEventListener("change", () => saveUI());
$("content").addEventListener("pointerdown", (e) => {
  hideNoteTip();
  hideDictTip();
  if (e.button !== 2) savedSel = null;
});
$("content").addEventListener("pointerover", (e) => {
  if (e.pointerType === "touch") return;
  const mark = noteMarkFrom(e.target);
  if (!mark || !state.book) return;
  if (noteMarkFrom(e.relatedTarget) === mark) return;
  if (noteHasBody(mark)) {
    hideDictTip();
    scheduleNoteTip(mark);
  }
});
$("content").addEventListener("pointerout", (e) => {
  const mark = noteMarkFrom(e.target);
  if (!mark) return;
  if (noteMarkFrom(e.relatedTarget) === mark) return;
  hideNoteTip();
});
$("content").addEventListener("pointermove", (e) => {
  if (e.pointerType === "touch" || !dictActive()) return;
  const mark = noteMarkFrom(e.target);
  if (mark && noteHasBody(mark)) return;
  scheduleDictAt(e.clientX, e.clientY);
});
$("content").addEventListener("pointerleave", () => {
  hideDictTip();
});
$("content").addEventListener("mouseup", (e) => {
  savedSel = captureSelection();
  if (!savedSel || !dictActive() || e.button !== 0) return;
  const q = dictQueryFromText(savedSel.text);
  if (!q || /\s/.test(q)) return;
  lookupDict(q, selectionAnchor() || pointAnchor(e.clientX, e.clientY));
});
$("content").addEventListener("click", (e) => {
  const mark = e.target.closest("mark.note");
  if (!mark || !state.book) return;
  const note = state.notes.find((n) => n.id === mark.dataset.noteId);
  if (note) openNote(note);
});
$("reader").addEventListener("contextmenu", (e) => {
  if (!state.book) return;
  if (e.target.closest("a, input, textarea, button")) return;
  const live = captureSelection();
  if (live) savedSel = live;
  const hl = e.target.closest("mark.hl");
  const note = e.target.closest("mark.note");
  e.preventDefault();
  ctxMarkId = !savedSel && hl ? hl.dataset.hlId || "" : "";
  ctxNoteId = !savedSel && note ? note.dataset.noteId || "" : "";
  const dictWord = (savedSel && savedSel.text) || wordAtPoint(e.clientX, e.clientY);
  showCtx(e.clientX, e.clientY, { sel: savedSel, hlId: ctxMarkId, noteId: ctxNoteId, dictWord });
});
$("ctxColors").addEventListener("click", (e) => {
  const btn = e.target.closest("[data-color]");
  if (!btn) return;
  const color = btn.getAttribute("data-color");
  const sel = savedSel || captureSelection();
  const existing = ctxMarkId;
  hideCtx();
  if (sel) {
    addHighlight(color, sel);
    return;
  }
  if (existing) recolorHighlight(existing, color);
});
$("ctxNoteColors").addEventListener("click", (e) => {
  const btn = e.target.closest("[data-color]");
  if (!btn) return;
  const color = btn.getAttribute("data-color");
  const sel = savedSel || captureSelection();
  const existing = ctxNoteId;
  hideCtx();
  if (sel) {
    addNote(color, sel);
    return;
  }
  if (existing) updateNote(existing, { color });
});
$("ctxRemove").addEventListener("click", () => {
  const id = ctxMarkId;
  hideCtx();
  if (id) deleteHighlight(id);
});
$("ctxNoteRemove").addEventListener("click", () => {
  const id = ctxNoteId;
  hideCtx();
  if (id) deleteNote(id);
});
document.addEventListener("pointerdown", (e) => {
  const menu = $("ctxMenu");
  const shelfMenu = $("shelfCtx");
  const welcomeMenu = $("welcomeCtx");
  if (!menu.hidden && !menu.contains(e.target)) hideCtx();
  if (shelfMenu && !shelfMenu.hidden && !shelfMenu.contains(e.target)) hideShelfCtx();
  if (welcomeMenu && !welcomeMenu.hidden && !welcomeMenu.contains(e.target)) hideWelcomeCtx();
  const shelfSearch = $("shelfSearch");
  if (shelfSearch && !shelfSearch.contains(e.target) && $("shelfSearchResults") && !$("shelfSearchResults").hidden) {
    $("shelfSearchResults").hidden = true;
  }
  if (e.button !== 2 && !$("content").contains(e.target) && !menu.contains(e.target)) {
    savedSel = null;
  }
});
$("reader").addEventListener("scroll", () => {
  hideCtx();
  hideDictTip();
  setProgress(scrollRatio(), state.chapterIndex);
  scheduleSave();
});
$("progressTrack").addEventListener("click", (e) => seekFromEvent(e, e.currentTarget));
$("sidebarProgressTrack").addEventListener("click", (e) => seekFromEvent(e, e.currentTarget));

window.addEventListener("dragover", (e) => {
  if (workspaceSortDrag) {
    e.preventDefault();
    return;
  }
  const types = e.dataTransfer && e.dataTransfer.types;
  if (!types || ![...types].includes("Files")) return;
  e.preventDefault();
  document.body.classList.add("dragover");
});
window.addEventListener("dragleave", () => document.body.classList.remove("dragover"));
window.addEventListener("drop", (e) => {
  e.preventDefault();
  document.body.classList.remove("dragover");
  if (workspaceSortDrag) return;
  const file = e.dataTransfer.files[0];
  if (file) uploadFile(file);
});

window.addEventListener("keydown", (e) => {
    if (e.key === "Escape") {
    if (levelUpOpen()) {
      closeLevelUp();
      e.preventDefault();
      return;
    }
    if ($("noteTip") && !$("noteTip").hidden) {
      hideNoteTip();
      e.preventDefault();
      return;
    }
    if ($("dictTip") && !$("dictTip").hidden) {
      hideDictTip();
      e.preventDefault();
      return;
    }
    if (bookNoteOpen()) {
      closeBookNoteDlg();
      e.preventDefault();
      return;
    }
    if (todoDlgOpen()) {
      closeTodoDlg();
      e.preventDefault();
      return;
    }
    if (readStatsDlgOpen()) {
      closeReadStatsDlg();
      e.preventDefault();
      return;
    }
    if (filePropsDlgOpen()) {
      closeFileProps();
      e.preventDefault();
      return;
    }
    if (addBookDlgOpen()) {
      closeAddBookDlg();
      e.preventDefault();
      return;
    }
    if (addToListDlgOpen()) {
      closeAddToListDlg();
      e.preventDefault();
      return;
    }
    if (!$("ctxMenu").hidden || ($("shelfCtx") && !$("shelfCtx").hidden) || ($("welcomeCtx") && !$("welcomeCtx").hidden)) {
      hideCtx();
      hideShelfCtx();
      hideWelcomeCtx();
      e.preventDefault();
      return;
    }
    if ($("listForm") && !$("listForm").hidden) {
      setListFormOpen(false);
      e.preventDefault();
      return;
    }
    if ($("workspaceForm") && !$("workspaceForm").hidden) {
      setWorkspaceFormOpen(false);
      e.preventDefault();
      return;
    }
    if (document.activeElement === $("shelfSearchInput") || ($("shelfSearchResults") && !$("shelfSearchResults").hidden)) {
      if (state.shelfQuery) {
        clearShelfSearch();
      } else {
        $("shelfSearchInput").blur();
      }
      e.preventDefault();
      return;
    }
    if (document.activeElement === $("searchInput")) {
      if ($("searchInput").value) {
        $("searchInput").value = "";
        state.query = "";
        renderSearchResults([]);
      } else {
        $("searchInput").blur();
      }
      e.preventDefault();
      return;
    }
    if (state.book && state.ui.notesOpen) {
      setNotesOpen(false);
      e.preventDefault();
      return;
    }
    if (state.book && state.ui.historyOpen) {
      setHistoryOpen(false);
      e.preventDefault();
      return;
    }
    if (state.openedListId && !state.book) {
      closeListScreen();
      e.preventDefault();
      return;
    }
    if (!state.book && state.ui.listsOpen) {
      setListsOpen(false);
      e.preventDefault();
      return;
    }
    if (!state.book && state.ui.workspacesOpen) {
      setWorkspacesOpen(false);
      e.preventDefault();
      return;
    }
    if (state.ui.sidebarOpen) {
      setSidebarOpen(false);
      e.preventDefault();
      return;
    }
    if (state.book) goShelf();
    return;
  }
  if (e.key === "F1") {
    e.preventDefault();
    openGuide();
    return;
  }
  if (e.target.matches("input, textarea, select")) return;
  if ((e.ctrlKey || e.metaKey) && (e.key === "z" || e.key === "Z") && !e.shiftKey) {
    e.preventDefault();
    undoLast();
    return;
  }
  if (e.key === "\\") {
    e.preventDefault();
    setSidebarOpen(!state.ui.sidebarOpen);
    return;
  }
  if ((e.key === "n" || e.key === "N") && state.book) {
    e.preventDefault();
    setNotesOpen(!state.ui.notesOpen);
    return;
  }
  if ((e.key === "h" || e.key === "H") && state.book) {
    e.preventDefault();
    setHistoryOpen(!state.ui.historyOpen);
    return;
  }
  if ((e.key === "w" || e.key === "W") && !state.book) {
    e.preventDefault();
    setWorkspacesOpen(!state.ui.workspacesOpen);
    return;
  }
  if ((e.key === "l" || e.key === "L") && !state.book) {
    e.preventDefault();
    setListsOpen(!state.ui.listsOpen);
    return;
  }
  if ((e.key === "b" || e.key === "B") && state.book) {
    e.preventDefault();
    addBookmark();
    return;
  }
  if ((e.key === "d" || e.key === "D") && state.book) {
    e.preventDefault();
    toggleDictionary();
    return;
  }
  if (e.key === "/") {
    e.preventDefault();
    if (state.book) {
      setSidebarOpen(true);
      $("searchInput").focus();
      $("searchInput").select();
    } else if ($("shelfSearchInput")) {
      $("shelfSearchInput").focus();
      $("shelfSearchInput").select();
    }
    return;
  }
  if (e.key === "ArrowRight") openChapter(state.chapterIndex + 1, "", true);
  if (e.key === "ArrowLeft") openChapter(state.chapterIndex - 1, "", true);
  if (e.key === "]") {
    bumpBookFont(1);
  }
  if (e.key === "[") {
    bumpBookFont(-1);
  }
  if (e.key === "{") bumpPageWidth(-2);
  if (e.key === "}") bumpPageWidth(2);
  if (e.key === "t") {
    const i = themes.indexOf(state.ui.theme);
    state.ui.theme = themes[(i + 1) % themes.length];
    saveUI();
  }
});

window.addEventListener("resize", () => {
  hideNoteTip();
  hideDictTip();
  const next = clampSidebarWidth(state.ui.sidebarWidth || 280);
  if (next !== state.ui.sidebarWidth) {
    state.ui.sidebarWidth = next;
    applySidebar();
  }
  const notes = clampNotesWidth(state.ui.notesWidth || 300);
  if (notes !== state.ui.notesWidth) {
    state.ui.notesWidth = notes;
    applyNotes();
  }
  const history = clampHistoryWidth(state.ui.historyWidth || 280);
  if (history !== state.ui.historyWidth) {
    state.ui.historyWidth = history;
    applyHistory();
  }
  const workspaces = clampWorkspacesWidth(state.ui.workspacesWidth || 280);
  if (workspaces !== state.ui.workspacesWidth) {
    state.ui.workspacesWidth = workspaces;
    applyWorkspaces();
  }
  const lists = clampListsWidth(state.ui.listsWidth || 280);
  if (lists !== state.ui.listsWidth) {
    state.ui.listsWidth = lists;
    applyLists();
  }
});

function flushOnLeave() {
  const key = state.bookNoteKey || (state.book && state.book.key) || "";
  if (key) {
    const draft = readBookNoteDraft(key);
    navigator.sendBeacon("/api/library/meta", new Blob([JSON.stringify({
      key,
      description: draft.description,
      journal: draft.journal,
    })], { type: "application/json" }));
  }
  if (!state.book) return;
  const progress = {
    chapterIndex: state.chapterIndex,
    scrollRatio: scrollRatio(),
  };
  navigator.sendBeacon("/api/progress", new Blob([JSON.stringify(progress)], { type: "application/json" }));
  navigator.sendBeacon("/api/history/session", new Blob([JSON.stringify(progress)], { type: "application/json" }));
  if (readTimer.status !== "idle") {
    commitReadTimer({ beacon: true });
  }
}

window.addEventListener("pagehide", flushOnLeave);
window.addEventListener("beforeunload", flushOnLeave);

boot().catch((err) => {
  $("welcome").hidden = false;
  $("shelf").hidden = false;
  console.error(err);
});
