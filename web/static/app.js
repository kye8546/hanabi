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

// 同一時間只顯示一個畫面：開局設定 / 加入 / 等待室 / 牌桌
function views(name) {
  for (const id of ['setup', 'join', 'lobby', 'game']) $('#' + id).hidden = id !== name;
}

// 多人房間的網址帶著 ?r=房間，重新整理或把網址傳給朋友都能用
function setRoomURL(room) {
  const u = new URL(location.href);
  if (room) u.searchParams.set('r', room); else u.searchParams.delete('r');
  history.replaceState(null, '', u);
}

function inviteURL(room) {
  const u = new URL(location.href);
  u.search = '';
  u.hash = '';
  u.searchParams.set('r', room);
  return u.toString();
}

// ---------- 開局設定 ----------

async function loadConfig() {
  const res = await fetch('api/config');
  S.cfg = await res.json();
}

function showSetup(err) {
  closeStream();
  S.room = S.token = S.snap = null;
  setRoomURL(null);
  views('setup');
  document.title = '花火 Hanabi';
  const f = $('#setup-form');
  const last = JSON.parse(store('hanabi.settings') || '{}');

  const seg = $('#players');
  seg.innerHTML = '';
  for (let n = S.cfg.minPlayers; n <= S.cfg.maxPlayers; n++) {
    const id = 'p' + n;
    seg.insertAdjacentHTML('beforeend',
      `<input type="radio" name="players" id="${id}" value="${n}" ${n === (last.players || 3) ? 'checked' : ''}><label for="${id}">${n} 人</label>`);
  }
  renderSeatChoices(last.seats || []);
  const lv = $('#level');
  lv.innerHTML = S.cfg.levels.map((l) =>
    `<option value="${l.level}" ${l.available ? '' : 'disabled'} ${l.level === (last.level || 1) ? 'selected' : ''}>Lv${l.level} ${esc(l.name)}${l.available ? '' : '（尚未實作）'}</option>`).join('');
  $('#variants').innerHTML = S.cfg.variants.map((v) =>
    `<label class="variant"><input type="radio" name="variant" value="${v.code}" ${(last.variant || 'none') === v.code ? 'checked' : ''}>
     <span><b>${esc(v.name)}</b><small>${esc(v.desc)}</small></span></label>`).join('');
  f.name.value = last.name || '';
  f.seed.value = '';
  const e = $('#setup-error');
  e.hidden = !err;
  e.textContent = err || '';
}

// 座位 1 起每個座位選 AI 或朋友（prev 是上次的選擇，人數改變時保留已選的）
function renderSeatChoices(prev) {
  const n = Number($('#setup-form').players.value);
  const box = $('#seats');
  box.innerHTML = '';
  for (let i = 1; i < n; i++) {
    const v = prev[i - 1] === 'human' ? 'human' : 'ai';
    box.insertAdjacentHTML('beforeend', `<div class="seat-choice"><span>座位 ${i}</span><div class="seg small">
      <input type="radio" name="seat${i}" id="seat${i}-ai" value="ai" ${v === 'ai' ? 'checked' : ''}><label for="seat${i}-ai">AI</label>
      <input type="radio" name="seat${i}" id="seat${i}-human" value="human" ${v === 'human' ? 'checked' : ''}><label for="seat${i}-human">朋友</label>
    </div></div>`);
  }
  updateSubmitLabel();
}

function seatChoices() {
  const out = [];
  for (let i = 1; ; i++) {
    const r = $('#setup-form').querySelector(`input[name=seat${i}]:checked`);
    if (!r) return out;
    out.push(r.value);
  }
}

function updateSubmitLabel() {
  $('#setup-submit').textContent = seatChoices().includes('human') ? '建立房間' : '開始';
}

$('#players').addEventListener('change', () => renderSeatChoices(seatChoices()));
$('#seats').addEventListener('change', updateSubmitLabel);

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
  enterRoom(out.room, out.token);
}

// 進入房間。每個房間的通行碼另外記著：離開後再打開邀請連結就能回到原本的座位
function enterRoom(room, token) {
  S.room = room;
  S.token = token;
  store('hanabi.session', JSON.stringify({ room, token }));
  store('hanabi.seat.' + room, token);
  S.snap = null;
  S.resultShown = false;
  openStream();
}

$('#setup-form').addEventListener('submit', (ev) => {
  ev.preventDefault();
  const f = ev.target;
  startGame({
    name: f.name.value, players: Number(f.players.value), level: Number(f.level.value),
    variant: f.variant.value, seed: f.seed.value.trim(), seats: seatChoices(),
  });
});

// ---------- 用邀請連結加入 ----------

function settingsText(s) {
  const variant = s.variant === 'none' ? '' : ` · ${esc(s.variantName)}`;
  return `${s.seats.length} 人局${variant} · Lv${s.level} ${esc(s.levelName)}`;
}

async function showJoin(room) {
  closeStream();
  S.room = room;
  S.token = S.snap = null;
  views('join');
  setRoomURL(room);
  document.title = '加入牌局 · 花火';
  const body = $('#join-body');
  body.innerHTML = '<p class="muted">讀取中…</p>';
  let info;
  try {
    const res = await fetch('api/room?room=' + encodeURIComponent(room), { cache: 'no-store' });
    info = await res.json();
    if (!res.ok) throw new Error(info.error);
  } catch (e) {
    body.innerHTML = `<p class="error">${esc(e.message || '連不上伺服器')}</p>`;
    body.append(newGameButton());
    return;
  }
  if (info.phase !== 'lobby') {
    body.innerHTML = `<p>${settingsText(info)}</p><p class="error">這局已經開始了，不能再加入。</p>`;
    body.append(newGameButton());
    return;
  }
  const last = JSON.parse(store('hanabi.settings') || '{}');
  body.innerHTML = `<p>${settingsText(info)}</p>
    <label class="field"><span>你的名字</span><input id="join-name" maxlength="12" placeholder="玩家" autocomplete="nickname" value="${esc(last.name || '')}"></label>
    <div class="field"><span>座位 <small>（照行動順序）</small></span><ol class="seatlist">${info.seats.map((st, i) => `
      <li><span class="seat">${i}</span><span class="who">${seatLabel(st, i)}</span>
      ${st.kind === 'open' ? `<button class="primary" data-sit="${i}">坐這裡</button>` : ''}</li>`).join('')}</ol></div>`;
  if (!info.seats.some((st) => st.kind === 'open')) {
    body.insertAdjacentHTML('beforeend', '<p class="error">座位已經坐滿了。</p>');
    body.append(newGameButton());
  }
  for (const b of body.querySelectorAll('[data-sit]')) {
    b.addEventListener('click', () => joinSeat(room, Number(b.dataset.sit)));
  }
}

function newGameButton() {
  const b = el('button', '', '自己開一局');
  b.addEventListener('click', () => showSetup());
  return b;
}

async function joinSeat(room, seat) {
  const name = $('#join-name').value;
  const last = JSON.parse(store('hanabi.settings') || '{}');
  store('hanabi.settings', JSON.stringify({ ...last, name }));
  let out;
  try {
    const res = await fetch('api/join', {
      method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ room, name, seat }),
    });
    out = await res.json();
  } catch {
    toast('連不上伺服器', 'error');
    return;
  }
  if (out.error) {
    toast(out.error, 'error');
    showJoin(room);
    return;
  }
  enterRoom(out.room, out.token);
}

// 座位的說明文字（等待室與加入畫面共用）
function seatLabel(st, i) {
  if (st.kind === 'open') return '<span class="muted">等待朋友加入</span>';
  let t = esc(st.name);
  if (st.kind === 'ai') t += ' <span class="badge">AI</span>';
  if (i === 0) t += ' <span class="badge">房主</span>';
  return t;
}

// ---------- 等待室 ----------

function renderLobby(s) {
  views('lobby');
  setRoomURL(S.room);
  document.title = '等待室 · 花火';
  $('#lobby-info').innerHTML = settingsText(s);
  $('#invite').value = inviteURL(S.room);
  $('#lobby-seats').innerHTML = s.seats.map((st, i) => {
    let t = `<li class="${i === s.seat ? 'me' : ''}"><span class="seat">${i}</span><span class="who">`;
    if (st.kind === 'human') t += `<i class="dot ${st.online ? 'on' : ''}" title="${st.online ? '在線上' : '離線'}"></i> `;
    t += seatLabel(st, i);
    if (i === s.seat) t += ' <small class="muted">（你）</small>';
    t += '</span>';
    if (s.host && i !== 0) {
      if (st.kind === 'open') t += `<button data-seat="${i}" data-kind="ai">改成 AI</button>`;
      else if (st.kind === 'ai') t += `<button data-seat="${i}" data-kind="open">改成等朋友</button>`;
      else t += `<button data-seat="${i}" data-kind="open">請他離開</button>`;
    }
    return t + '</li>';
  }).join('');
  const open = s.seats.filter((st) => st.kind === 'open').length;
  const box = $('#lobby-actions');
  if (s.host) {
    box.innerHTML = `${open ? `<p class="muted">還有 ${open} 個空位，現在開始就改由 AI 坐。</p>` : ''}
      <button class="primary" data-lobby="start">開始遊戲</button>
      <button data-lobby="cancel">取消，回到開局設定</button>`;
  } else {
    box.innerHTML = `<p class="waiting thinking">等房主開始遊戲</p>
      <button data-lobby="leave">離開座位</button>`;
  }
}

$('#lobby-seats').addEventListener('click', (ev) => {
  const b = ev.target.closest('[data-seat]');
  if (!b) return;
  const name = S.snap.seats[Number(b.dataset.seat)].name;
  if (S.snap.seats[Number(b.dataset.seat)].kind === 'human' && !confirm(`請 ${name} 離開這個座位？`)) return;
  act({ cmd: 'seat', target: Number(b.dataset.seat), kind: b.dataset.kind });
});

$('#lobby-actions').addEventListener('click', async (ev) => {
  const b = ev.target.closest('[data-lobby]');
  if (!b) return;
  switch (b.dataset.lobby) {
    case 'start': {
      const open = S.snap.seats.filter((st) => st.kind === 'open').length;
      if (open && !confirm(`還有 ${open} 個空位，改由 AI 坐並開始？`)) return;
      act({ cmd: 'start' });
      break;
    }
    case 'cancel':
      store('hanabi.session', null);
      showSetup();
      break;
    case 'leave':
      await act({ cmd: 'leave' });
      store('hanabi.session', null);
      store('hanabi.seat.' + S.room, null);
      showSetup();
      break;
  }
});

$('#copy-invite').addEventListener('click', async () => {
  const input = $('#invite');
  try {
    await navigator.clipboard.writeText(input.value);
  } catch {
    input.select();
    document.execCommand('copy');
  }
  toast('已複製邀請連結');
});

// ---------- 連線 ----------

function qs() { return `room=${encodeURIComponent(S.room)}&token=${encodeURIComponent(S.token)}`; }

function closeStream() {
  if (S.es) S.es.close();
  S.es = null;
  clearTimeout(S.sseTimer);
  S.pollGen = (S.pollGen || 0) + 1; // 讓進行中的輪詢自己結束
}

function showState(state) {
  if (state.next) { // 房主替同一群人開了下一局：通行碼照用，換過去
    const d = $('#dialog');
    if (d.open) d.close();
    toast('房主開了下一局');
    enterRoom(state.next, S.token);
    return;
  }
  S.snap = state;
  if (state.multi) setRoomURL(S.room);
  render();
}

const GONE_TEXT = {
  left: '你已經不在這局的座位上了',
  closed: '這局已經關閉（閒置太久）',
};

function roomGone(msg) {
  closeStream();
  store('hanabi.session', null);
  if (S.room) store('hanabi.seat.' + S.room, null);
  if (S.snap && S.snap.phase === 'game') toast(msg, 'error'); else showSetup(msg);
}

// 先用 Server-Sent Events；幾秒內沒收到畫面（Cloudflare 的免帳號通道等代理伺服器會把串流整個緩衝住）
// 或連不上，就改用長輪詢；串流被緩衝時記住這個網址要用輪詢（下一局不必再等）。
function openStream() {
  closeStream();
  S.cm = new Set(JSON.parse(store('hanabi.cm.' + S.room) || '[]'));
  if (store('hanabi.poll') === '1') { poll(); return; }
  const es = new EventSource('api/stream?' + qs());
  S.es = es;
  let got = false;
  const fallback = (buffered) => {
    if (got || S.es !== es) return;
    closeStream();
    if (buffered) store('hanabi.poll', '1'); // 連得上卻收不到畫面才記住；連不上可能只是伺服器重開
    poll();
  };
  S.sseTimer = setTimeout(() => fallback(true), 4000);
  es.addEventListener('state', (ev) => {
    got = true;
    showState(JSON.parse(ev.data));
  });
  es.addEventListener('gone', (ev) => {
    const reason = JSON.parse(ev.data || '{}').reason;
    roomGone(GONE_TEXT[reason] || GONE_TEXT.closed);
  });
  // 收到過畫面就讓 EventSource 自己重連；從沒收到就改輪詢（找不到這局時由輪詢回到開局畫面）
  es.onerror = () => fallback(false);
}

async function poll() {
  const gen = S.pollGen;
  let ver = 0;
  while (gen === S.pollGen) {
    let res;
    try {
      res = await fetch(`api/poll?${qs()}&since=${ver}`, { cache: 'no-store' });
    } catch {
      await new Promise((ok) => setTimeout(ok, 2000)); // 網路斷了，稍後再試
      continue;
    }
    if (gen !== S.pollGen) return;
    if (res.status === 404) {
      const out = await res.json().catch(() => ({}));
      roomGone(out.error || '找不到這局（可能已過期或伺服器重新啟動過），請開新的一局');
      return;
    }
    if (res.status !== 200) {
      if (res.status !== 204) await new Promise((ok) => setTimeout(ok, 2000));
      continue;
    }
    const out = await res.json();
    if (gen !== S.pollGen) return;
    if (out.gone) { roomGone(GONE_TEXT[out.gone] || GONE_TEXT.closed); return; }
    ver = out.ver;
    showState(out.state);
  }
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
    return {};
  }
  if (out.error) { toast(out.error, 'error'); return out; }
  if (out.peek) {
    dialog(`<h2>你的手牌（偷看）</h2><div class="peek">${out.peek.map((c, i) =>
      `<div class="slot"><div class="pos">${i + 1}</div>${faceCard(c).outerHTML}</div>`).join('')}</div>
      <p class="muted">紀錄會註明你在這一回合偷看過。</p>`);
  }
  if (out.info) toast(out.info);
  return out;
}

// ---------- 畫面 ----------

function render() {
  const s = S.snap;
  if (!s) return;
  if (s.phase === 'lobby') { renderLobby(s); return; }
  views('game');
  hideMenu(); // 畫面重畫後選單指向的牌已經不在了
  document.title = s.yourTurn ? '● 輪到你了 · 花火' : '花火 Hanabi';
  $('#game-info').innerHTML = settingsText(s) + (s.seed ? ` · seed ${s.seed}` : '');
  for (const b of document.querySelectorAll('[data-cmd]')) {
    b.hidden = s.multi && b.dataset.cmd !== 'auto'; // 多人局不能回溯、偷看
    b.disabled = !s.yourTurn;
  }
  const neg = $('#btn-neg');
  neg.setAttribute('aria-pressed', S.showNeg ? 'true' : 'false');
  neg.classList.toggle('on', S.showNeg);
  $('#btn-new').textContent = s.multi ? '離開' : '新遊戲';

  renderStatus(s);
  renderBoard(s);
  renderHands(s);
  renderLog(s);

  const tl = $('#turnline');
  if (s.error) {
    tl.innerHTML = `<span class="bad">${esc(s.error)}</span>`;
  } else if (s.over) {
    tl.textContent = '遊戲結束';
  } else if (s.takeover) {
    tl.innerHTML = '<b>AI 正在替你行動</b> <button class="primary" id="resume">我回來了</button>';
    $('#resume').addEventListener('click', () => act({ cmd: 'takeover', target: s.seat, on: false }));
  } else if (s.yourTurn) {
    tl.innerHTML = '<b>輪到你了</b>：點自己的牌打出或棄掉，點隊友的牌給提示';
  } else {
    const cur = s.seats[s.current];
    const waitHuman = cur.kind === 'human' && !cur.takeover;
    tl.innerHTML = `<span class="thinking">${waitHuman ? '等待 ' : ''}${esc(s.names[s.current])}${waitHuman ? ' 行動' : ' 思考中'}</span>`;
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
  const st = s.seats[seat];
  const sec = el('section', 'player' + (mine ? ' mine' : '') + (seat === s.current && !s.over ? ' current' : ''));
  let label = `<span class="seat">${seat}</span> `;
  if (s.multi && st.kind === 'human') label += `<i class="dot ${st.online ? 'on' : ''}" title="${st.online ? '在線上' : '離線'}"></i> `;
  label += esc(s.names[seat]);
  if (s.multi && mine) label += ' <small class="muted">（你）</small>';
  if (s.multi && st.kind === 'ai') label += ' <span class="badge">AI</span>';
  if (st.takeover) label += ' <span class="badge warn">AI 接手中</span>';
  if (seat === s.current && !s.over) label += `<span class="turn">${mine ? '輪到你' : '行動中'}</span>`;
  const header = el('header', '', label);
  if (s.multi && st.kind === 'human' && !s.over && !(mine && st.takeover)) {
    const on = !st.takeover;
    const b = el('button', 'mini', on ? (mine ? '暫時交給 AI' : '讓 AI 接手') : '取消 AI 接手');
    b.addEventListener('click', () => {
      if (on && !confirm(mine ? '暫時讓 AI 替你行動？回來後按「我回來了」接回。'
        : `讓 AI 暫時替 ${s.names[seat]} 行動？他回來後可以按「我回來了」接回。`)) return;
      act({ cmd: 'takeover', target: seat, on });
    });
    header.append(b);
  }
  sec.append(header);
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
  let t = '';
  switch (e.type) {
    case 'play':
      t = `${who} 打出 ${cardChip(e.card)}（位置 ${e.slot + 1}）<span class="good">✓ 成功</span>`;
      if (e.card.rank === 5) {
        t += `，完成${COLORS[e.card.color].name}色煙火！`;
        if (e.bonus) t += '提示 +1';
      }
      break;
    case 'misplay':
      t = `${who} 打出 ${cardChip(e.card)}（位置 ${e.slot + 1}）<span class="bad">✗ 失敗！閃電 +1</span>`;
      break;
    case 'discard':
      t = `${who} 棄掉 ${cardChip(e.card)}（位置 ${e.slot + 1}），提示 +1`;
      break;
    case 'hint':
      t = `${who} 提示 ${esc(s.names[e.target])}「${hintText(e.hint)}」→ 位置 ${e.touched.map((x) => x + 1).join(', ')}`;
      break;
    case 'over':
      return `遊戲結束：${END_TEXT[e.reason] || ''}`;
  }
  if (e.note) t += ` <span class="note">（${esc(e.note)}）</span>`;
  return t;
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
  const notTurn = s.over ? '遊戲已結束' : (s.takeover ? 'AI 正在替你行動' : (!s.yourTurn ? '還沒輪到你' : ''));
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
  const html = `<h2>${END_TEXT[s.endReason] || '遊戲結束'}</h2>
    <p class="final-score">${score} <small>/ ${s.maxScore}</small></p>
    <p>${rating(score, s.maxScore)}${note}</p>`;
  const download = { label: '下載紀錄', onClick: (ev) => { ev.preventDefault(); downloadLog(); } };
  if (!s.multi) {
    const last = JSON.parse(store('hanabi.settings') || '{}');
    dialog(html, [
      download,
      { label: '同一副牌重玩', onClick: () => startGame({ ...last, seed: s.seed, seats: [] }) },
      { label: '再來一局', primary: true, onClick: () => startGame({ ...last, seed: '', seats: [] }) },
      { label: '看盤面' },
    ]);
    return;
  }
  // 多人局：房主替同一群人開下一局，大家自動進到新的等待室
  if (s.host) {
    dialog(html + '<p class="muted">開下一局後，大家會自動進到新的等待室。</p>', [
      download,
      { label: '同一副牌重玩', onClick: () => act({ cmd: 'rematch', on: true }) },
      { label: '同一群人再來一局', primary: true, onClick: () => act({ cmd: 'rematch', on: false }) },
      { label: '看盤面' },
    ]);
  } else {
    dialog(html + '<p class="muted">等房主開下一局，開了會自動進去。</p>', [
      download,
      { label: '離開', onClick: () => leaveGame() },
      { label: '看盤面' },
    ]);
  }
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

// 離開多人局：還沒結束就先讓 AI 接手自己的座位，大家不必等
async function leaveGame() {
  const s = S.snap;
  if (s && s.multi && !s.over && !s.takeover) await act({ cmd: 'takeover', target: s.seat, on: true });
  store('hanabi.session', null);
  showSetup();
}

$('#btn-new').addEventListener('click', () => {
  const s = S.snap;
  if (s && s.multi) {
    if (!s.over && !confirm('離開這局？AI 會接手你的座位。之後用同一個瀏覽器打開邀請連結就能回來。')) return;
    leaveGame();
    return;
  }
  if (s && !s.over && !confirm('放棄這局、回到開局設定？')) return;
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
  const want = new URLSearchParams(location.search).get('r');
  const saved = JSON.parse(store('hanabi.session') || 'null');
  const seat = want && store('hanabi.seat.' + want);
  if (seat) {
    enterRoom(want, seat);
  } else if (saved && !want) {
    S.room = saved.room;
    S.token = saved.token;
    openStream();
  } else if (want) {
    showJoin(want);
  } else {
    showSetup();
  }
})();
