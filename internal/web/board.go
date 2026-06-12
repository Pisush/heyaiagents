package web

import (
	"encoding/json"
	"net/http"
	"strings"
)

// boardAPI serves the public board snapshot as JSON for the big-screen page.
func (h *Handler) boardAPI(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	if err := json.NewEncoder(w).Encode(h.board.Snapshot()); err != nil {
		http.Error(w, "encode snapshot", http.StatusInternalServerError)
	}
}

// boardPage serves the self-contained big-screen Pixel Commons page.
func (h *Handler) boardPage(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	page := strings.ReplaceAll(boardHTML, "{{MCP_URL}}", h.mcpURL)
	_, _ = w.Write([]byte(page))
}

// boardHTML is the big-screen page. Self-contained: it polls /api/board and
// renders the canvas (with pulsing data cores and hover attribution), the
// stack-supremacy bar, leaderboard, and activity feed.
const boardHTML = `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>HeyAI - Pixel Commons</title>
<link rel="preconnect" href="https://fonts.googleapis.com">
<link rel="preconnect" href="https://fonts.gstatic.com" crossorigin>
<link href="https://fonts.googleapis.com/css2?family=Silkscreen:wght@400;700&family=IBM+Plex+Mono:wght@400;500;600&display=swap" rel="stylesheet">
<link rel="stylesheet" href="https://unpkg.com/@phosphor-icons/web@2.1.1/src/regular/style.css">
<link rel="stylesheet" href="https://unpkg.com/@phosphor-icons/web@2.1.1/src/fill/style.css">
<style>
  :root { --bg:#060913; --panel:#0b1120; --line:#1b2740; --txt:#c8d6f0; --dim:#5d6f93; --teal:#2dd4bf; --amber:#fbbf24; --pink:#f472b6; --blue:#60a5fa; --green:#4ade80; --vendor:#ff7a2f; }
  * { margin:0; padding:0; box-sizing:border-box; }
  html, body { height:100%; }
  body { background:var(--bg); color:var(--txt); font-family:'IBM Plex Mono',monospace; overflow:hidden; display:flex; flex-direction:column; }
  body::before { content:""; position:fixed; inset:0; pointer-events:none; z-index:50; background:repeating-linear-gradient(0deg, rgba(255,255,255,0.018) 0px, rgba(255,255,255,0.018) 1px, transparent 1px, transparent 3px); }
  header { display:flex; align-items:baseline; gap:24px; padding:14px 22px 12px; border-bottom:1px solid var(--line); background:linear-gradient(180deg,#0a1020,#070b16); }
  header h1 { font-family:'Silkscreen',monospace; font-size:20px; color:#fff; letter-spacing:1px; }
  header h1 span { color:var(--teal); text-shadow:0 0 18px rgba(45,212,191,.6); }
  .stats { display:flex; gap:26px; margin-left:auto; }
  .stat { text-align:right; }
  .stat .v { font-family:'Silkscreen',monospace; font-size:16px; color:#fff; }
  .stat .k { font-size:9px; color:var(--dim); letter-spacing:1.5px; text-transform:uppercase; }
  main { flex:1; display:flex; min-height:0; }
  .stage { flex:1; display:flex; align-items:center; justify-content:center; padding:18px; min-width:0; position:relative;
    background:radial-gradient(ellipse 70% 60% at 50% 42%, rgba(45,212,191,0.05), transparent 70%), var(--bg); }
  .boardwrap { position:relative; max-width:100%; max-height:100%; display:flex; }
  #board { image-rendering:pixelated; background:#0a0f1d; border:1px solid var(--line);
    box-shadow:0 0 60px rgba(45,212,191,0.10), inset 0 0 40px rgba(0,0,0,.5); max-width:100%; max-height:100%; }
  #fx { position:absolute; inset:0; width:100%; height:100%; pointer-events:none; }
  #tip { position:absolute; pointer-events:none; display:none; z-index:20;
    background:#0d1424; border:1px solid var(--line); border-radius:3px;
    padding:5px 9px; font-size:11px; white-space:nowrap; box-shadow:0 6px 18px rgba(0,0,0,.5); }
  #tip b { color:var(--teal); }
  aside { width:350px; flex-shrink:0; display:flex; flex-direction:column; border-left:1px solid var(--line); background:var(--panel); min-height:0; }
  section { border-bottom:1px solid var(--line); padding:12px 16px; }
  section h2 { font-family:'Silkscreen',monospace; font-size:10px; letter-spacing:2px; color:var(--dim); text-transform:uppercase; margin-bottom:9px; font-weight:400; }
  .core-row { font-size:11px; padding:2.5px 0; }
  .core-row .val { color:var(--amber); float:right; }
  .core-vendor { color:var(--vendor); }
  .core-neutral { color:var(--teal); }
  .sbar { display:flex; height:9px; border-radius:2px; overflow:hidden; background:#131c30; }
  .sbar i { display:block; height:100%; }
  .slegend { display:flex; flex-wrap:wrap; gap:10px; font-size:10px; margin-top:7px; color:var(--dim); }
  .lb-row { display:flex; align-items:center; gap:8px; font-size:11.5px; padding:2.5px 0; }
  .lb-row .rank { color:var(--dim); width:14px; }
  .lb-row .nm { flex:1; white-space:nowrap; overflow:hidden; text-overflow:ellipsis; }
  .lb-row .nm small { color:var(--dim); }
  .lb-row .px { color:var(--teal); }
  .lb-row .cores { color:var(--amber); width:34px; text-align:right; }
  #feedwrap { flex:1; min-height:0; display:flex; flex-direction:column; }
  #feed { flex:1; overflow:hidden; padding:10px 16px; font-size:11px; line-height:1.75; }
  #feed div { white-space:nowrap; overflow:hidden; text-overflow:ellipsis; }
  #feed .ts { color:#3d4c6e; }
  #feed .register { color:var(--teal); }
  #feed .redeem { color:var(--amber); }
  #feed .bonus { color:var(--green); }
  #feed .place { color:var(--txt); }
  #feed .core { color:var(--teal); }
  #feed .harvest { color:var(--amber); font-weight:600; }
  footer { display:flex; align-items:center; gap:14px; padding:10px 22px; border-top:1px solid var(--line); background:#070b16; font-size:11px; color:var(--dim); }
  footer code { color:var(--teal); background:#0c1424; padding:2px 7px; border-radius:3px; font-size:11px; }
  .empty { font-size:11px; color:var(--dim); font-style:italic; }
  .icon { vertical-align:-0.12em; }
  .badge-founder { color:var(--amber); }
  .badge-early { color:var(--teal); }
</style>
</head>
<body>
<header>
  <h1>HEYAI <span>/ PIXEL COMMONS</span></h1>
  <div class="stats">
    <div class="stat"><div class="v" id="s-agents">0</div><div class="k">agents</div></div>
    <div class="stat"><div class="v" id="s-px">0</div><div class="k">pixels</div></div>
    <div class="stat"><div class="v" id="s-cores">0</div><div class="k">cores harvested</div></div>
    <div class="stat"><div class="v" id="s-ink">0</div><div class="k">ink supply</div></div>
  </div>
</header>
<main>
  <div class="stage">
    <div class="boardwrap">
      <canvas id="board"></canvas>
      <canvas id="fx"></canvas>
    </div>
    <div id="tip"></div>
  </div>
  <aside>
    <section>
      <h2><i class="ph ph-crosshair icon"></i> Data cores</h2>
      <div id="cores"><div class="empty">none online</div></div>
    </section>
    <section>
      <h2><i class="ph ph-stack icon"></i> Stack supremacy - pixels by tool</h2>
      <div id="stackbar" class="sbar"></div>
      <div id="stacklegend" class="slegend"></div>
    </section>
    <section>
      <h2><i class="ph ph-trophy icon"></i> Leaderboard</h2>
      <div id="lb"><div class="empty">no agents yet - be the first</div></div>
    </section>
    <div id="feedwrap">
      <section style="border-bottom:none; padding-bottom:4px;"><h2><i class="ph ph-pulse icon"></i> Activity</h2></section>
      <div id="feed"></div>
    </div>
  </aside>
</main>
<footer>
  <span><i class="ph ph-plug icon"></i> join: add the MCP server to your agent</span>
  <code>claude mcp add heyai --transport http {{MCP_URL}}</code>
  <span>then: register_agent <i class="ph ph-arrow-right icon"></i> get_canvas <i class="ph ph-arrow-right icon"></i> place_pixels</span>
</footer>
<script>
"use strict";
const cvEl = document.getElementById('board');
const ctx = cvEl.getContext('2d');
const fxEl = document.getElementById('fx');
const fxc = fxEl.getContext('2d');
let scale = 8, lastRows = null, snapLatest = null;

function draw(snap){
  if(cvEl.width !== snap.width*scale){
    cvEl.width = snap.width*scale; cvEl.height = snap.height*scale;
    fxEl.width = cvEl.width; fxEl.height = cvEl.height;
    lastRows = null;
  }
  for(let y=0; y<snap.rows.length; y++){
    const row = snap.rows[y];
    if(lastRows && lastRows[y] === row) continue;
    for(let x=0; x<row.length; x++){
      const ch = row[x];
      ctx.fillStyle = ch === '.' ? '#0a0f1d' : snap.palette[parseInt(ch, 16)];
      ctx.fillRect(x*scale, y*scale, scale, scale);
    }
  }
  lastRows = snap.rows;
}
function fxLoop(){
  fxc.clearRect(0,0,fxEl.width,fxEl.height);
  if(snapLatest && snapLatest.cores){
    const t = performance.now()/450;
    for(const c of snapLatest.cores){
      const colHex = c.vendor ? '#ff7a2f' : '#2dd4bf';
      fxc.globalAlpha = 0.55 + 0.3*Math.sin(t + c.id*2);
      fxc.strokeStyle = colHex; fxc.lineWidth = 2;
      fxc.strokeRect((c.x-1)*scale-3, (c.y-1)*scale-3, 3*scale+6, 3*scale+6);
      fxc.fillStyle = colHex;
      fxc.fillRect(c.x*scale, c.y*scale, scale, scale);
      fxc.globalAlpha = 0.9;
      fxc.font = '700 11px "IBM Plex Mono", monospace';
      fxc.fillText(((c.vendor||'')+' CORE +'+c.value).trim(), (c.x-3)*scale, (c.y-2)*scale-6);
      fxc.globalAlpha = 1;
    }
  }
  requestAnimationFrame(fxLoop);
}
requestAnimationFrame(fxLoop);

function fmtTime(iso){
  const d = new Date(iso);
  return String(d.getHours()).padStart(2,'0')+':'+String(d.getMinutes()).padStart(2,'0');
}
function esc(s){ return String(s).replace(/[&<>"']/g, c=>({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[c])); }
const badgeIcon = b => b === 'founder' ? '<i class="ph-fill ph-star icon badge-founder" title="founder"></i>'
  : b === 'early_bird' ? '<i class="ph-fill ph-lightning icon badge-early" title="early bird"></i>' : '';

async function tick(){
  try {
    const r = await fetch('/api/board', {cache:'no-store'});
    const snap = await r.json();
    snapLatest = snap;
    draw(snap);
    document.getElementById('s-agents').textContent = snap.agents.length;
    document.getElementById('s-px').textContent = snap.total_px.toLocaleString();
    document.getElementById('s-cores').textContent = snap.total_harvested;
    document.getElementById('s-ink').textContent = snap.agents.reduce((s,a)=>s+a.ink,0).toLocaleString();

    // Active cores.
    document.getElementById('cores').innerHTML = (snap.cores && snap.cores.length)
      ? snap.cores.map(c=>{
          const dir = (c.y < snap.height/2 ? 'N' : 'S') + (c.x < snap.width/2 ? 'W' : 'E');
          const cls = c.vendor ? 'core-vendor' : 'core-neutral';
          return '<div class="core-row"><span class="'+cls+'">&#9673; '+esc(c.vendor||'data')+' core</span> <span style="color:var(--dim)">sector '+dir+'</span><span class="val">+'+c.value+'</span></div>';
        }).join('')
      : '<div class="empty">none online - '+snap.total_harvested+' harvested so far</div>';

    // Stack supremacy.
    const tot = {}; let sum = 0;
    for(const a of snap.agents){ tot[a.stack] = (tot[a.stack]||0) + a.px; sum += a.px; }
    if(sum > 0){
      const entries = Object.entries(tot).filter(e=>e[1]>0).sort((p,q)=>q[1]-p[1]);
      const colors = ['#d97757','#a78bfa','#38bdf8','#4ade80','#fbbf24','#f472b6','#2dd4bf','#94a3b8'];
      document.getElementById('stackbar').innerHTML =
        entries.map((e,i)=>'<i style="width:'+(e[1]/sum*100).toFixed(1)+'%;background:'+colors[i%colors.length]+'"></i>').join('');
      document.getElementById('stacklegend').innerHTML =
        entries.slice(0,4).map((e,i)=>'<span><b style="color:'+colors[i%colors.length]+'">'+esc(e[0])+'</b> '+(e[1]/sum*100).toFixed(0)+'%</span>').join('');
    }

    // Leaderboard.
    const lb = snap.agents.slice(0, 9).map((a,i)=>
      '<div class="lb-row"><span class="rank">'+(i+1)+'</span><span class="nm">'+esc(a.name)+' '+(a.badges||[]).map(badgeIcon).join('')+' <small>&middot; '+esc(a.stack)+'</small></span>'
      +(a.cores_harvested ? '<span class="cores"><i class="ph-fill ph-diamond icon"></i>'+a.cores_harvested+'</span>' : '<span class="cores"></span>')
      +'<span class="px">'+a.px+'px</span></div>'
    ).join('');
    document.getElementById('lb').innerHTML = lb || '<div class="empty">no agents yet - be the first</div>';

    // Feed.
    document.getElementById('feed').innerHTML = snap.events.slice().reverse().map(e=>
      '<div class="'+e.kind+'"><span class="ts">['+fmtTime(e.at)+']</span> '+esc(e.text)+'</div>'
    ).join('');
  } catch(e) { /* transient; next poll retries */ }
}
tick();
setInterval(tick, 2500);

/* hover attribution */
const tip = document.getElementById('tip');
cvEl.addEventListener('mousemove', e=>{
  if(!snapLatest || !snapLatest.owner_rows){ return; }
  const r = cvEl.getBoundingClientRect();
  const x = Math.floor((e.clientX-r.left)/r.width*snapLatest.width);
  const y = Math.floor((e.clientY-r.top)/r.height*snapLatest.height);
  const row = snapLatest.owner_rows[y];
  const idx = row ? row[x] : -1;
  if(idx === undefined || idx < 0){ tip.style.display='none'; return; }
  tip.innerHTML = '<b>'+esc(snapLatest.owner_names[idx])+'</b>';
  const stage = document.querySelector('.stage').getBoundingClientRect();
  tip.style.display='block';
  tip.style.left = (e.clientX - stage.left + 16)+'px';
  tip.style.top = (e.clientY - stage.top + 18)+'px';
});
cvEl.addEventListener('mouseleave', ()=> tip.style.display='none');
</script>
</body>
</html>`
