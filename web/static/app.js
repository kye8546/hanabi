'use strict';

// 顏色依 game.Color 的順序：R Y G B W T K M
const COLORS = [
  { L: 'R', name: '紅' }, { L: 'Y', name: '黃' }, { L: 'G', name: '綠' }, { L: 'B', name: '藍' },
  { L: 'W', name: '白' }, { L: 'T', name: '青' }, { L: 'K', name: '黑' }, { L: 'M', name: '彩' },
];
const RAINBOW = 7;
const MAX_HINTS = 8, MAX_STRIKES = 3;
const END_TEXT = { 1: '三道閃電，煙火秀失敗', 2: '所有煙火全部完成！', 3: '牌堆用盡，最後一輪結束' };

const $ = (sel) => document.querySelector(sel);
const S = {
  cfg: null, room: null, token: null, snap: null, es: null,
  showNeg: false, cm: new Set(), resultShown: false,
};

// ---------- 小工具 ----------

function store(key, val) {
  try {
    if (val === undefined) return localStorage.getItem(key);
    if (val === null) localStorage.removeItem(key); else localStorage.setItem(key, val);
  } catch { /* 私密瀏覽等情況沒有 localStorage，照樣能玩 */ }
  return null;
}

function esc(s) {
  return String(s).replace(/[&<>"']/g, (c) => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' }[c]));
}

function el(tag, cls, html) {
  const e = document.createElement(tag);
  if (cls) e.className = cls;
  if (html !== undefined) e.innerHTML = html;
  return e;
}

let toastTimer;
function toast(msg, kind = '') {
  const t = $('#toast');
  t.textContent = msg;
  t.className = 'toast ' + kind;
  t.hidden = false;
  clearTimeout(toastTimer);
  toastTimer = setTimeout(() => { t.hidden = true; }, kind === 'error' ? 4000 : 6000);
}

function dialog(html, actions = [{ label: '關閉' }]) {
  $('#dialog-content').innerHTML = html;
  const box = $('#dialog-actions');
  box.innerHTML = '';
  for (const a of actions) {
    const b = el('button', a.primary ? 'primary' : '', esc(a.label));
    b.value = a.label;
    if (a.onClick) b.addEventListener('click', (ev) => { a.onClick(ev); });
    box.append(b);
  }
  const d = $('#dialog');
  if (!d.open) d.showModal();
}

function cardChip(c) {
  return `<span class="chip c-${COLORS[c.color].L}">${COLORS[c.color].L}${c.rank}</span>`;
}

function hintText(h) {
  if (h.kind === 'color') return `<span class="hint-c c-${COLORS[h.color].L}">${COLORS[h.color].name}色</span>`;
  return `數字 ${h.rank}`;
}

// ---------- 開局設定 ----------

async function loadConfig() {
  const res = await fetch('api/config');
  S.cfg = await res.json();
}

function showSetup(err) {
  closeStream();
  S.room = S.token = S.snap = null;
  $('#game').hidden = true;
  $('#setup').hidden = false;
  const f = $('#setup-form');
  const last = JSON.parse(store('hanabi.settings') || '{}');

  const seg = $('#players');
  seg.innerHTML = '';
  for (let n = S.cfg.minPlayers; n <= S.cfg.maxPlayers; n++) {
    const id = 'p' + n;
    seg.insertAdjacentHTML('beforeend',
      `<input type="radio" name="players" id="${id}" value="${n}" ${n === (last.players || 3) ? 'checked' : ''}><label for="${id}">${n} 人</label>`);
  }
  const lv = $('#level');
  lv.innerHTML = S.cfg.levels.map((l) =>
    `<option value="${l.level}" ${l.available ? '' : 'disabled'} ${l.level === (last.level || 1) ? 'selected' : ''}>Lv${l.level} ${esc(l.name)}${l.available ? '' : '（尚未實作）'}</option>`).join('');
  $('#variants').innerHTML = S.cfg.variants.map((v, i) =>
    `<label class="variant"><input type="radio" name="variant" value="${v.code}" ${(last.variant || 'none') === v.code ? 'checked' : ''}>
     <span><b>${esc(v.name)}</b><small>${esc(v.desc)}</small></span></label>`).join('');
  f.name.value = last.name || '';
  f.seed.value = '';
  f.showneg.checked = store('hanabi.showNeg') === '1';
  const e = $('#setup-error');
  e.hidden = !err;
  e.textContent = err || '';
}

async function startGame(settings) {
  store('hanabi.settings', JSON.stringify({ ...settings, seed: undefined }));
  const res = await fetch('api/new', {
    method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(settings),
  });
  const out = await res.json();
  if (!res.ok) {
    if ($('#setup').hidden) toast(out.error || '開局失敗', 'error'); else showSetup(out.error || '開局失敗');
    return;
  }
  S.room = out.room;
  S.token = out.token;
  store('hanabi.session', JSON.stringify({ room: S.room, token: S.token }));
  S.cm = new Set();
  S.resultShown = false;
  openStream();
}

$('#setup-form').addEventListener('submit', (ev) => {
  ev.preventDefault();
  const f = ev.target;
  S.showNeg = f.showneg.checked;
  store('hanabi.showNeg', S.showNeg ? '1' : '0');
  startGame({
    name: f.name.value, players: Number(f.players.value), level: Number(f.level.value),
    variant: f.variant.value, seed: f.seed.value.trim(),
  });
});

// ---------- 連線 ----------

function qs() { return `room=${encodeURIComponent(S.room)}&token=${encodeURIComponent(S.token)}`; }

function closeStream() {
  if (S.es) S.es.close();
  S.es = null;
}

function openStream() {
  closeStream();
  S.cm = new Set(JSON.parse(store('hanabi.cm.' + S.room) || '[]'));
  const es = new EventSource('api/stream?' + qs());
  S.es = es;
  let got = false;
  es.addEventListener('state', (ev) => {
    got = true;
    S.snap = JSON.parse(ev.data);
    $('#setup').hidden = true;
    $('#game').hidden = false;
    render();
  });
  es.addEventListener('gone', () => {
    closeStream();
    store('hanabi.session', null);
    toast('這局已經關閉（閒置太久）', 'error');
  });
  es.onerror = () => {
    // 連不上又從沒收到畫面：多半是這局已不存在（伺服器重新啟動過），回到開局畫面
    if (!got) {
      closeStream();
      store('hanabi.session', null);
      showSetup(S.snap ? '' : '找不到上一局（可能已過期或伺服器重新啟動過），請開新的一局');
    }
    // 收到過畫面就讓 EventSource 自己重連
  };
}

async function act(cmd) {
  hideMenu();
  let out;
  try {
    const res = await fetch('api/act?' + qs(), {
      method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(cmd),
    });
    out = await res.json();
  } catch {
    toast('連不上伺服器', 'error');
    return;
  }
  if (out.error) { toast(out.error, 'error'); return; }
  if (out.peek) {
    dialog(`<h2>你的手牌（偷看）</h2><div class="peek">${out.peek.map((c, i) =>
      `<div class="slot"><div class="pos">${i + 1}</div>${faceCard(c).outerHTML}</div>`).join('')}</div>
      <p class="muted">紀錄會註明你在這一回合偷看過。</p>`);
  }
  if (out.info) toast(out.info);
}

// ---------- 畫面 ----------

function render() {
  const s = S.snap;
  if (!s) return;
  hideMenu(); // 畫面重畫後選單指向的牌已經不在了
  const variant = s.variant === 'none' ? '' : ` · ${esc(s.variantName)}`;
  $('#game-info').innerHTML = `${s.names.length} 人局${variant} · Lv${s.level} ${esc(s.levelName)} · seed ${s.seed}`;
  for (const b of document.querySelectorAll('[data-cmd]')) b.disabled = !s.yourTurn;
  const neg = $('#btn-neg');
  neg.setAttribute('aria-pressed', S.showNeg ? 'true' : 'false');
  neg.classList.toggle('on', S.showNeg);

  renderStatus(s);
  renderBoard(s);
  renderHands(s);
  renderLog(s);

  const tl = $('#turnline');
  if (s.error) {
    tl.innerHTML = `<span class="bad">${esc(s.error)}</span>`;
  } else if (s.over) {
    tl.textContent = '遊戲結束';
  } else if (s.yourTurn) {
    tl.innerHTML = '<b>輪到你了</b>：點自己的牌打出或棄掉，點隊友的牌給提示';
  } else {
    tl.innerHTML = `<span class="thinking">${esc(s.names[s.current])} 思考中</span>`;
  }
  if (s.over && !S.resultShown) {
    S.resultShown = true;
    showResult(s);
  }
}

function renderStatus(s) {
  const dots = (n, max, on, off) => Array.from({ length: max }, (_, i) => `<i class="${i < n ? on : off}"></i>`).join('');
  let score = `<b>${s.score}</b> / ${s.maxScore}`;
  if (s.maxReachable < s.maxScore) score += `<small>（最多 ${s.maxReachable}）</small>`;
  let pace = 'Pace -';
  let paceCls = '';
  if (s.pace !== null) {
    pace = `Pace ${s.pace}`;
    if (s.pace <= 0) paceCls = 'bad'; else if (s.pace < s.names.length) paceCls = 'warn';
    if (paceCls) pace += '（終局）';
  }
  let final = '';
  if (s.finalTurns > 0) final = `<div class="stat final">⚠ 最後一輪：剩 ${s.finalTurns} 回合</div>`;
  $('#status').innerHTML = `
    <div class="stat"><span class="k">分數</span><span class="v">${score}</span></div>
    <div class="stat"><span class="k">提示 ${s.hints}</span><span class="v tokens">${dots(s.hints, MAX_HINTS, 'tok', 'tok off')}</span></div>
    <div class="stat"><span class="k">閃電 ${s.strikes}</span><span class="v tokens">${dots(s.strikes, MAX_STRIKES, 'strike', 'strike off')}</span></div>
    <div class="stat"><span class="k">牌堆</span><span class="v"><b>${s.deck}</b> 張</span></div>
    <div class="stat ${paceCls}"><span class="k">回合 ${s.turn}</span><span class="v">${pace}</span></div>
    ${final}`;
}

function renderBoard(s) {
  const fw = $('#fireworks');
  fw.innerHTML = '';
  s.colors.forEach((c, i) => {
    const r = s.fireworks[i];
    const d = el('div', `pile c-${COLORS[c].L} ${r ? '' : 'empty'} ${r === 5 ? 'done' : ''}`,
      `<span class="rank">${r || ''}</span><span class="letter">${COLORS[c].L}</span>`);
    d.title = `${COLORS[c].name}色：${r ? '已打到 ' + r : '還沒打出'}`;
    fw.append(d);
  });
  const ds = $('#discards');
  if (!s.discards.length) { ds.innerHTML = '<span class="muted">（無）</span>'; return; }
  ds.innerHTML = s.colors.map((c) => {
    const rs = s.discards.filter((d) => d.color === c).map((d) => d.rank).sort();
    if (!rs.length) return '';
    return `<div class="drow">${rs.map((r) => `<span class="chip c-${COLORS[c].L}">${COLORS[c].L}${r}</span>`).join('')}</div>`;
  }).join('');
}

// 依顯示設定回傳要顯示的顏色 / 數字（同 console 的 colorsShown / ranksShown）
function shownColors(k) { return S.showNeg ? k.pc : (k.hc >= 0 ? [k.hc] : []); }
function shownRanks(k) { return S.showNeg ? k.pr : (k.hr ? [k.hr] : []); }

function possRows(k) {
  const cs = shownColors(k), rs = shownRanks(k);
  const c = cs.length ? cs.map((x) => `<span class="t-${COLORS[x].L}">${COLORS[x].L}</span>`).join('') : '<span class="muted">?</span>';
  const r = rs.length ? rs.join('') : '<span class="muted">?</span>';
  return `<div class="poss">${c}</div><div class="poss">${r}</div>`;
}

function knownShort(k) {
  const cs = shownColors(k), rs = shownRanks(k);
  const r = rs.length === 1 ? rs[0] : '?';
  if (cs.length === 1) return `<span class="known t-${COLORS[cs[0]].L}">${COLORS[cs[0]].L}${r}</span>`;
  return `<span class="known muted">?${r}</span>`;
}

function faceCard(c) {
  const d = el('div', `card c-${COLORS[c.color].L}`,
    `<span class="rank">${c.rank}</span><span class="letter">${COLORS[c.color].L}</span>`);
  return d;
}

// 自己看不到的牌：顯示已知的顏色（底色）與數字
function backCard(k) {
  const cs = shownColors(k), rs = shownRanks(k);
  const known = cs.length === 1 ? cs[0] : -1;
  const d = el('div', 'card back' + (known >= 0 ? ` tint c-${COLORS[known].L}` : ''),
    `<span class="rank">${rs.length === 1 ? rs[0] : '?'}</span><span class="letter">${known >= 0 ? COLORS[known].L : ''}</span>`);
  return d;
}

function renderHands(s) {
  const others = $('#others');
  others.innerHTML = '';
  const n = s.names.length;
  // 依行動順序：從自己的下一位開始，自己放最後
  for (let i = 1; i < n; i++) {
    const sec = handSection(s, (s.seat + i) % n);
    if (i === 1) sec.append(el('div', 'know-label', '牌下面是他自己知道的資訊'));
    others.append(sec);
  }
  const me = $('#me');
  me.innerHTML = '';
  me.append(handSection(s, s.seat));
}

function handSection(s, seat) {
  const mine = seat === s.seat;
  const sec = el('section', 'player' + (mine ? ' mine' : '') + (seat === s.current && !s.over ? ' current' : ''));
  let label = `<span class="seat">${seat}</span> ${esc(s.names[seat])}`;
  if (seat === s.current && !s.over) label += `<span class="turn">${mine ? '輪到你' : '行動中'}</span>`;
  sec.append(el('header', '', label));
  const hand = el('div', 'hand');
  s.hands[seat].forEach((k, i) => {
    const slot = el('div', 'slot');
    slot.dataset.slot = i;
    const pos = el('div', 'pos', `${i + 1}${S.cm.has(k.id) ? ' <span class="cm">cm</span>' : ''}`);
    const card = mine ? backCard(k) : faceCard(k);
    if (k.clued) card.classList.add('clued');
    const btn = el('button', 'cardbtn');
    btn.append(card);
    btn.title = mine ? `你的位置 ${i + 1}` : `${s.names[seat]} 的位置 ${i + 1}`;
    btn.addEventListener('click', (ev) => { ev.stopPropagation(); openMenu(btn, seat, i); });
    slot.append(pos, btn);
    if (mine) {
      if (S.showNeg) slot.append(el('div', 'know', possRows(k)));
    } else {
      slot.append(el('div', 'know', S.showNeg ? possRows(k) : knownShort(k)));
    }
    hand.append(slot);
  });
  sec.append(hand);
  return sec;
}

function describe(e, s) {
  const who = esc(s.names[e.player]);
  switch (e.type) {
    case 'play': {
      let t = `${who} 打出 ${cardChip(e.card)}（位置 ${e.slot + 1}）<span class="good">✓ 成功</span>`;
      if (e.card.rank === 5) {
        t += `，完成${COLORS[e.card.color].name}色煙火！`;
        if (e.bonus) t += '提示 +1';
      }
      return t;
    }
    case 'misplay':
      return `${who} 打出 ${cardChip(e.card)}（位置 ${e.slot + 1}）<span class="bad">✗ 失敗！閃電 +1</span>`;
    case 'discard':
      return `${who} 棄掉 ${cardChip(e.card)}（位置 ${e.slot + 1}），提示 +1`;
    case 'hint':
      return `${who} 提示 ${esc(s.names[e.target])}「${hintText(e.hint)}」→ 位置 ${e.touched.map((x) => x + 1).join(', ')}`;
    case 'over':
      return `遊戲結束：${END_TEXT[e.reason] || ''}`;
  }
  return '';
}

function renderLog(s) {
  const ol = $('#log');
  // 自己上一次行動之後的事件是「最近發生」
  let since = 0;
  s.log.forEach((e, i) => { if (e.player === s.seat && e.type !== 'over') since = i + 1; });
  ol.innerHTML = s.log.map((e, i) =>
    `<li class="${i >= since ? 'recent' : ''} ${e.player === s.seat && e.type !== 'over' ? 'self' : ''}"><span class="t">${e.turn}</span>${describe(e, s)}</li>`).join('');
  ol.scrollTop = ol.scrollHeight;
  // 窄螢幕的紀錄在最下面，自己的牌上方另外列出最近發生的事
  $('#recent').innerHTML = s.log.slice(since).map((e) => `<li>${describe(e, s)}</li>`).join('');
}

// ---------- 選單 ----------

function hideMenu() {
  $('#menu').hidden = true;
  for (const e of document.querySelectorAll('.would')) e.classList.remove('would');
}

function openMenu(anchor, seat, slot) {
  const s = S.snap;
  const k = s.hands[seat][slot];
  const mine = seat === s.seat;
  const m = $('#menu');
  m.innerHTML = '';
  const add = (label, onClick, disabled, note) => {
    const b = el('button', '', label + (note ? `<small>${esc(note)}</small>` : ''));
    b.disabled = !!disabled;
    b.addEventListener('click', (ev) => { ev.stopPropagation(); onClick(); });
    m.append(b);
    return b;
  };
  const notTurn = s.over ? '遊戲已結束' : (!s.yourTurn ? '還沒輪到你' : '');
  if (mine) {
    m.append(el('div', 'menu-title', `你的位置 ${slot + 1}`));
    add('打出', () => act({ cmd: 'play', slot }), notTurn, notTurn);
    const full = s.hints >= MAX_HINTS ? '提示滿 8 個，不能棄牌' : '';
    add('棄掉', () => act({ cmd: 'discard', slot }), notTurn || full, notTurn || full);
  } else {
    m.append(el('div', 'menu-title', `提示 ${esc(s.names[seat])}`));
    const noTok = s.hints <= 0 ? '沒有提示 token 了' : '';
    const opts = s.hintOptions.filter((h) => h.target === seat && h.touched.includes(slot));
    for (const h of opts) {
      const b = add(`「${hintText(h)}」→ 位置 ${h.touched.map((x) => x + 1).join(', ')}`,
        () => act({ cmd: 'hint', target: seat, kind: h.kind, color: h.color, rank: h.rank }), notTurn || noTok);
      const slots = () => anchor.closest('.hand').querySelectorAll('.slot');
      const mark = (on) => { for (const t of h.touched) slots()[t].classList.toggle('would', on); };
      b.addEventListener('mouseenter', () => mark(true));
      b.addEventListener('mouseleave', () => mark(false));
      b.addEventListener('focus', () => mark(true));
      b.addEventListener('blur', () => mark(false));
    }
    if (notTurn || noTok) m.append(el('div', 'menu-note', esc(notTurn || noTok)));
  }
  add(S.cm.has(k.id) ? '取消 cm 標記' : '標記為 Chop Moved（cm）', () => { toggleCM(k.id); hideMenu(); });

  m.hidden = false;
  const r = anchor.getBoundingClientRect();
  const w = m.offsetWidth, h = m.offsetHeight;
  let x = r.left + r.width / 2 - w / 2;
  x = Math.max(8, Math.min(x, window.innerWidth - w - 8));
  let y = r.bottom + 6;
  if (y + h > window.innerHeight - 8) y = Math.max(8, r.top - h - 6);
  m.style.left = x + 'px';
  m.style.top = y + 'px';
  m.querySelector('button:not(:disabled)')?.focus({ preventScroll: true });
}

function toggleCM(id) {
  if (S.cm.has(id)) S.cm.delete(id); else S.cm.add(id);
  store('hanabi.cm.' + S.room, JSON.stringify([...S.cm]));
  render();
}

document.addEventListener('click', (ev) => { if (!$('#menu').contains(ev.target)) hideMenu(); });
document.addEventListener('keydown', (ev) => { if (ev.key === 'Escape') hideMenu(); });
window.addEventListener('resize', hideMenu);
window.addEventListener('scroll', hideMenu, true);

// ---------- 結果 ----------

function rating(score, max) {
  if (score <= 5) return '慘不忍睹，觀眾噓聲四起';
  if (score <= 10) return '平庸，只有零星掌聲';
  if (score <= 15) return '值得尊敬，但觀眾不會記得';
  if (score <= 20) return '優秀，觀眾很滿意';
  if (score < max) return '非凡，觀眾將永遠記得';
  return '傳奇！大人小孩都看得目瞪口呆';
}

function showResult(s) {
  let score = s.score;
  let note = '';
  if (s.endReason === 1) { note = `（煙火進度 ${score}，依官方規則得 0 分）`; score = 0; }
  const last = JSON.parse(store('hanabi.settings') || '{}');
  dialog(`<h2>${END_TEXT[s.endReason] || '遊戲結束'}</h2>
    <p class="final-score">${score} <small>/ ${s.maxScore}</small></p>
    <p>${rating(score, s.maxScore)}${note}</p>`, [
    { label: '下載紀錄', onClick: (ev) => { ev.preventDefault(); downloadLog(); } },
    { label: '同一副牌重玩', onClick: () => startGame({ ...last, seed: s.seed }) },
    { label: '再來一局', primary: true, onClick: () => startGame({ ...last, seed: '' }) },
    { label: '看盤面' },
  ]);
}

// ---------- 工具列 ----------

for (const b of document.querySelectorAll('[data-cmd]')) {
  b.addEventListener('click', () => {
    if (b.dataset.cmd === 'undo' && !confirm('回到你上一次行動之前？')) return;
    act({ cmd: b.dataset.cmd });
  });
}

$('#btn-neg').addEventListener('click', () => {
  S.showNeg = !S.showNeg;
  store('hanabi.showNeg', S.showNeg ? '1' : '0');
  render();
});

function downloadLog() {
  const a = document.createElement('a');
  a.href = 'api/log?' + qs();
  a.download = '';
  document.body.append(a);
  a.click();
  a.remove();
}
$('#btn-log').addEventListener('click', downloadLog);

$('#btn-help').addEventListener('click', () => dialog($('#help').innerHTML));

$('#btn-new').addEventListener('click', () => {
  if (S.snap && !S.snap.over && !confirm('放棄這局、回到開局設定？')) return;
  store('hanabi.session', null);
  showSetup();
});

// ---------- 啟動 ----------

(async function main() {
  S.showNeg = store('hanabi.showNeg') === '1';
  try {
    await loadConfig();
  } catch {
    document.body.innerHTML = '<p class="fatal">連不上伺服器，請稍後重新整理。</p>';
    return;
  }
  const saved = JSON.parse(store('hanabi.session') || 'null');
  if (saved) {
    S.room = saved.room;
    S.token = saved.token;
    openStream();
  } else {
    showSetup();
  }
})();
