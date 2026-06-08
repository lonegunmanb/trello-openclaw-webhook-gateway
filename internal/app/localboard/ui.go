package localboard

const indexHTML = `<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>JJC Local Board</title>
<style>
  :root { color-scheme: light dark; }
  body { margin:0; height:100vh; overflow:hidden; display:flex; flex-direction:column; font:14px/20px -apple-system,BlinkMacSystemFont,"Segoe UI",sans-serif; background:var(--background-color-default,#fff); color:var(--text-color-default,#1f2328); }
  header { height:44px; display:flex; align-items:center; gap:12px; padding:0 12px; border-bottom:1px solid var(--border-color-default,#d0d7de); }
  h1 { margin:0; font-size:16px; font-weight:600; }
  .status { color:var(--text-color-muted,#656d76); font-size:12px; }
  .spacer { flex:1; }
  button { border:1px solid var(--border-color-default,#d0d7de); border-radius:6px; padding:4px 10px; background:var(--background-color-default,#fff); color:inherit; font:inherit; cursor:pointer; }
  button:hover { background:var(--background-color-muted,#f6f8fa); }
  button.primary { background:#1f6feb; border-color:#1f6feb; color:#fff; }
  main { flex:1; min-height:0; display:flex; }
  .board { flex:1; min-width:0; display:flex; gap:8px; padding:8px; overflow-x:auto; }
  .col { flex:0 0 282px; display:flex; flex-direction:column; min-height:0; border:1px solid var(--border-color-default,#d0d7de); border-radius:8px; background:var(--background-color-muted,#f6f8fa); }
  .col.drag { outline:2px solid #1f6feb; outline-offset:-2px; }
  .col-head { display:flex; align-items:center; gap:8px; padding:8px 10px; border-bottom:1px solid var(--border-color-default,#d0d7de); }
  .col-head h2 { flex:1; margin:0; font-size:13px; font-weight:600; }
  .count { color:var(--text-color-muted,#656d76); font-size:12px; }
  .col-body { flex:1; min-height:0; overflow-y:auto; padding:6px; }
  .card { margin-bottom:6px; padding:8px; border:1px solid var(--border-color-default,#d0d7de); border-radius:6px; background:var(--background-color-default,#fff); cursor:grab; }
  .card.dragging { opacity:.45; }
  .card .id { color:var(--text-color-muted,#656d76); font-size:11px; }
  .card .title { font-weight:600; word-break:break-word; }
  .meta { color:var(--text-color-muted,#656d76); font-size:12px; }
  .empty { color:var(--text-color-muted,#656d76); font-size:12px; padding:8px; text-align:center; }
  aside { flex:0 0 460px; min-width:0; display:none; flex-direction:column; border-left:1px solid var(--border-color-default,#d0d7de); }
  aside.open { display:flex; }
  .drawer { flex:1; min-height:0; overflow:auto; padding:12px; }
  .drawer h2 { margin:0; font-size:15px; }
  .drawer h3 { margin:16px 0 6px; font-size:12px; text-transform:uppercase; color:var(--text-color-muted,#656d76); }
  pre, textarea, input { width:100%; box-sizing:border-box; border:1px solid var(--border-color-default,#d0d7de); border-radius:6px; background:var(--background-color-default,#fff); color:inherit; font:inherit; }
  pre { padding:8px; white-space:pre-wrap; word-break:break-word; background:var(--background-color-muted,#f6f8fa); max-height:180px; overflow:auto; }
  textarea, input { padding:6px; }
  .comment { margin-bottom:6px; padding:6px 8px; border-radius:6px; background:var(--background-color-muted,#f6f8fa); white-space:pre-wrap; word-break:break-word; font-size:12px; }
  .comment .by { color:var(--text-color-muted,#656d76); font-size:10px; margin-bottom:2px; }
  .modal-backdrop { position:fixed; inset:0; display:none; align-items:center; justify-content:center; background:rgba(0,0,0,.35); }
  .modal-backdrop.open { display:flex; }
  .modal { width:520px; max-width:92vw; padding:16px; border-radius:8px; background:var(--background-color-default,#fff); box-shadow:0 16px 40px rgba(0,0,0,.25); }
  .modal h2 { margin:0 0 10px; font-size:15px; }
  .error { min-height:18px; color:#cf222e; font-size:12px; }
  .actions { display:flex; justify-content:flex-end; gap:8px; margin-top:10px; }
</style>
</head>
<body>
<header><h1>JJC Local Board</h1><span id="status" class="status"></span><span class="spacer"></span><button class="primary" id="new-card">New card</button></header>
<main><section id="board" class="board"></section><aside id="side"></aside></main>
<div id="modal" class="modal-backdrop"><div class="modal"><h2>New card</h2><input id="new-title" placeholder="Title"><div style="height:8px"></div><textarea id="new-desc" rows="8" placeholder="https://github.com/owner/repo/issues/123&#10;&#10;Optional notes"></textarea><div id="new-error" class="error"></div><div class="actions"><button id="cancel">Cancel</button><button class="primary" id="create">Create</button></div></div></div>
<script>
const $ = s => document.querySelector(s);
let state = { columns: [], cards: [] }, selected = "", dragging = "", draggingFrom = "";
async function api(method, path, body) {
  const res = await fetch(path, { method, headers: {"content-type":"application/json"}, body: body ? JSON.stringify(body) : undefined });
  const text = await res.text(); let data = {}; try { data = text ? JSON.parse(text) : {}; } catch { data = { error: text }; }
  if (!res.ok) throw new Error(data.error || res.statusText);
  return data;
}
async function refresh() { state = await api("GET", "/api/state"); state.columns = state.columns || []; state.cards = state.cards || []; render(); }
function render() { renderBoard(); renderSide(); }
function renderBoard() {
  const board = $("#board"); board.innerHTML = "";
  for (const col of state.columns) {
    const cards = state.cards.filter(c => c.idList === col.id);
    const el = document.createElement("div"); el.className = "col"; el.dataset.id = col.id;
    el.innerHTML = '<div class="col-head"><h2>' + esc(col.name) + '</h2><span class="count">' + cards.length + '</span></div><div class="col-body"></div>';
    const body = el.querySelector(".col-body");
    if (cards.length === 0) body.innerHTML = '<div class="empty">empty</div>'; else cards.forEach(c => body.appendChild(cardEl(c)));
    el.ondragover = e => { e.preventDefault(); el.classList.add("drag"); };
    el.ondragleave = () => el.classList.remove("drag");
    el.ondrop = async e => { e.preventDefault(); el.classList.remove("drag"); if (!dragging) return; if (draggingFrom === col.id) return; try { await api("POST", "/api/cards/" + dragging + "/move", {to: col.id}); await refresh(); } catch(err) { alert(err.message); } };
    board.appendChild(el);
  }
}
function cardEl(c) {
  const el = document.createElement("div"); el.className = "card"; el.draggable = true;
  el.innerHTML = '<div class="id">' + esc(c.id) + '</div><div class="title">' + esc(c.name) + '</div><div class="meta">comments: ' + (c.comments ? c.comments.length : 0) + '</div>';
  el.ondragstart = () => { dragging = c.id; draggingFrom = c.idList; el.classList.add("dragging"); };
  el.ondragend = () => { dragging = ""; draggingFrom = ""; el.classList.remove("dragging"); };
  el.onclick = () => { selected = c.id; renderSide(); };
  return el;
}
function renderSide() {
  const side = $("#side"), c = state.cards.find(x => x.id === selected);
  if (!c) { side.className = ""; side.innerHTML = ""; return; }
  side.className = "open";
  side.innerHTML = '<div class="drawer"><div style="display:flex;gap:8px"><h2 style="flex:1">' + esc(c.name) + '</h2><button id="close">Close</button></div><div class="meta">' + esc(c.id) + ' - ' + esc(label(c.idList)) + '</div><h3>Description</h3><pre>' + esc(c.desc) + '</pre><h3>Comment</h3><textarea id="comment" rows="4" placeholder="Comment as human. Ctrl+Enter sends."></textarea><div class="actions"><button class="primary" id="send">Send</button></div><h3>Comments</h3>' + ((c.comments || []).slice().reverse().map(renderComment).join("") || '<div class="empty">none</div>') + '</div>';
  $("#close").onclick = () => { selected = ""; renderSide(); };
  $("#send").onclick = sendComment;
  $("#comment").onkeydown = e => { if (e.key === "Enter" && (e.ctrlKey || e.metaKey)) { e.preventDefault(); sendComment(); } };
}
function renderComment(c) { return '<div class="comment"><div class="by">' + esc(c.by || "unknown") + ' - ' + esc(c.at || "") + '</div>' + esc(c.text) + '</div>'; }
async function sendComment() { const text = $("#comment").value.trim(); if (!text || !selected) return; await api("POST", "/api/cards/" + selected + "/comments", { text }); $("#comment").value = ""; await refresh(); }
function label(id) { const c = state.columns.find(x => x.id === id); return c ? c.name : id; }
function esc(v) { return String(v ?? "").replace(/[&<>"']/g, c => ({"&":"&amp;","<":"&lt;",">":"&gt;",'"':"&quot;","'":"&#39;"}[c])); }
$("#new-card").onclick = () => { $("#modal").classList.add("open"); $("#new-title").value = ""; $("#new-desc").value = ""; $("#new-error").textContent = ""; setTimeout(() => $("#new-desc").focus(), 0); };
$("#cancel").onclick = () => $("#modal").classList.remove("open");
$("#create").onclick = async () => { try { const r = await api("POST", "/api/cards", { title: $("#new-title").value, description: $("#new-desc").value }); selected = r.card.id; $("#modal").classList.remove("open"); await refresh(); } catch(e) { $("#new-error").textContent = e.message; } };
function connect() { const es = new EventSource("/events"); es.onopen = () => { $("#status").textContent = "connected"; refresh(); }; es.onerror = () => $("#status").textContent = "reconnecting"; es.onmessage = refresh; }
refresh().then(connect);
</script>
</body>
</html>`
