package web

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"hanabi/ai/hgroup"
	"hanabi/game"
	"hanabi/player"
	"hanabi/session"
)

// seatKind 是座位的狀態。
type seatKind int

const (
	seatOpen  seatKind = iota // 等朋友加入（開始時還空著就改由 AI 坐）
	seatHuman                 // 已經有人坐
	seatAI
)

var kindNames = map[seatKind]string{seatOpen: "open", seatHuman: "human", seatAI: "ai"}

// offlineGrace：最後一條連線斷掉多久之後才算離線（長輪詢每次請求之間會短暫沒有連線）。
const offlineGrace = 10 * time.Second

// room 是一局遊戲，開始前是等待室（朋友用邀請連結加入空位）。
// 遊戲狀態（g、players、紀錄）只在 run 的 goroutine 裡讀寫；HTTP 那邊只碰 mu 保護的欄位，
// 指令經由各真人座位的 reqs 送進去。
type room struct {
	id      string
	n       int
	va      game.Variant
	seed    uint64
	level   int
	aiDelay time.Duration
	logDir  string
	ctx     context.Context
	cancel  context.CancelFunc

	// 開始時由 start 設定，之後只在 run 的 goroutine 讀寫
	g       *game.Game
	players []player.Player
	log     *session.Log
	notes   map[int]string // 回合 → 畫面紀錄上的註記（AI 代打 / AI 接手）

	mu      sync.Mutex
	started bool
	kinds   []seatKind
	names   []string       // 空位是空字串；開始後不再改變
	tokens  map[string]int // 通行碼 → 座位
	humans  []*webHuman    // 依座位，AI 座位是 nil（開始時建立，之後不再改變）
	views   []*snapshot    // 每個真人座位最新的遊戲畫面（render 再補上座位等資訊）
	snaps   [][]byte       // 每個座位要送出的畫面 JSON
	ver     int            // 畫面的版本，每次 render 加 1（給輪詢用）
	subs    map[chan struct{}]bool
	conns   []int       // 每個座位目前的連線數（SSE 與等待中的輪詢）
	seen    []time.Time // 每個座位最後一次有連線的時間
	active  time.Time   // 最後一次有人連線或下指令
	done    bool
	fatal   string // 程式錯誤（例如 AI 回傳不合法行動），顯示在畫面上
	next    string // 房主開的下一局（同一群人再來一局），大家的畫面跟著換過去
}

// roomConfig 是開一個房間需要的設定。kinds[0] 一定是 seatHuman（房主）；names / tokens 是已經入座的真人。
type roomConfig struct {
	n      int
	level  int
	va     game.Variant
	seed   uint64
	kinds  []seatKind
	names  []string
	tokens map[string]int
}

// command 是瀏覽器送來的指令。
type command struct {
	Cmd    string `json:"cmd"` // play / discard / hint / auto / undo / peek；takeover；等待室的 start / seat / leave；結束後的 rematch
	Slot   int    `json:"slot"`
	Target int    `json:"target"`
	Kind   string `json:"kind"` // hint：color / rank；seat：ai / open
	Color  int    `json:"color"`
	Rank   int    `json:"rank"`
	On     bool   `json:"on"` // takeover：開始 / 停止讓 AI 接手
}

type reply struct {
	OK    bool       `json:"ok"`
	Error string     `json:"error,omitempty"`
	Info  string     `json:"info,omitempty"`
	Peek  []cardJSON `json:"peek,omitempty"`
	Room  string     `json:"room,omitempty"` // rematch：新的房間
}

type request struct {
	cmd   command
	reply chan reply // 緩衝 1，送的人不會卡住
}

func newRoom(id string, cfg roomConfig, logDir string, aiDelay time.Duration) (*room, error) {
	g, err := game.NewVariant(cfg.n, cfg.va, cfg.seed)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithCancel(context.Background())
	r := &room{id: id, n: cfg.n, va: cfg.va, seed: cfg.seed, level: cfg.level, aiDelay: aiDelay, logDir: logDir,
		ctx: ctx, cancel: cancel, g: g,
		kinds: cfg.kinds, names: cfg.names, tokens: cfg.tokens,
		snaps: make([][]byte, cfg.n), subs: map[chan struct{}]bool{},
		conns: make([]int, cfg.n), seen: make([]time.Time, cfg.n), active: time.Now()}
	for s, k := range r.kinds {
		if k == seatAI {
			r.names[s] = aiName(s)
		}
	}
	r.mu.Lock()
	r.render()
	r.mu.Unlock()
	return r, nil
}

func aiName(seat int) string { return fmt.Sprintf("AI-%d", seat) }

// multi：兩位以上真人（含還空著的位子）。多人局不能回溯、偷看，也不先公開種子。mu 要鎖著。
func (r *room) multi() bool {
	n := 0
	for _, k := range r.kinds {
		if k != seatAI {
			n++
		}
	}
	return n > 1
}

// start 開始遊戲：還空著的位子改由 AI 坐，建立紀錄與玩家，之後由 run 的 goroutine 接手。
func (r *room) start() error {
	r.mu.Lock()
	if r.started {
		r.mu.Unlock()
		return errors.New("遊戲已經開始了")
	}
	var humans []int
	for s, k := range r.kinds {
		if k == seatOpen {
			r.kinds[s] = seatAI
			r.names[s] = aiName(s)
		}
		if r.kinds[s] == seatHuman {
			humans = append(humans, s)
		}
	}
	gl, err := session.NewLog(r.logDir, r.names, humans, r.level, r.va, r.seed)
	if err != nil {
		r.mu.Unlock()
		return err
	}
	r.started = true
	r.log = gl
	r.notes = map[int]string{}
	r.humans = make([]*webHuman, r.n)
	for _, s := range humans {
		r.humans[s] = &webHuman{r: r, seat: s, reqs: make(chan request)}
	}
	r.players = r.newPlayers()
	r.mu.Unlock()
	r.publish()
	go r.run()
	return nil
}

// newPlayers 和 console 一樣組裝：真人座位包著代打用的 AI，所有座位都寫紀錄；AI 另外控制節奏、讓畫面跟上。
func (r *room) newPlayers() []player.Player {
	ps := make([]player.Player, r.n)
	for s := range r.n {
		if h := r.humans[s]; h != nil {
			h.helper = hgroup.New(r.names[s], r.level)
			ps[s] = session.Helped{
				Player: session.LoggedHuman{Logged: session.Logged{Player: h, L: r.log}, H: h},
				Helper: h.helper,
			}
		} else {
			ps[s] = paced{session.Logged{Player: hgroup.New(r.names[s], r.level), L: r.log}, r}
		}
	}
	return ps
}

// run 跑完整局（含回溯）。結束後保留最後的畫面，讓玩家看結果、下載紀錄。
func (r *room) run() {
	defer func() {
		r.log.Close(r.g)
		r.mu.Lock()
		r.done = true
		for _, h := range r.humans {
			if h != nil {
				h.waiting = false
			}
		}
		r.mu.Unlock()
		r.publish()
	}()
	for {
		err := player.Run(r.g, r.players)
		switch {
		case errors.Is(err, player.ErrUndo):
			h := r.humans[0] // 只有單人局能回溯
			rep := h.pendingUndo
			h.pendingUndo = nil
			ng, ps, turn, ok := session.Rewind(r.g, r.seed, h.seat, r.newPlayers)
			if !ok {
				rep <- reply{Error: "還沒有可以回溯的行動"}
				continue
			}
			r.g, r.players = ng, ps
			r.log.Rewind(r.g, turn)
			for t := range r.notes {
				if t >= turn {
					delete(r.notes, t)
				}
			}
			rep <- reply{OK: true, Info: "已回溯到你上一次行動之前"}
			continue
		case errors.Is(err, player.ErrQuit):
			return
		case err != nil:
			log.Printf("房間 %s：%v", r.id, err)
			r.log.Note(r.g.Turn, "錯誤："+err.Error())
			r.mu.Lock()
			r.fatal = "程式錯誤，這局無法繼續：" + err.Error()
			r.mu.Unlock()
		}
		return
	}
}

// publish 重算每個真人座位的遊戲畫面並送出。只在 run 的 goroutine（或開始時）呼叫。
func (r *room) publish() {
	views := make([]*snapshot, r.n)
	for s, h := range r.humans {
		if h != nil {
			v := buildSnapshot(r.g.View(s), r.level, r.seed, r.notes)
			views[s] = &v
		}
	}
	r.mu.Lock()
	r.views = views
	r.render()
	r.mu.Unlock()
}

// render 把遊戲畫面（等待室則是設定）加上座位、輪到誰等資訊，替每個真人座位編成 JSON，通知所有連線。
// mu 要鎖著；不讀遊戲狀態，所以 HTTP 那邊（入座、AI 接手、上下線）也能呼叫。
func (r *room) render() {
	multi := r.multi()
	seats := make([]seatJSON, r.n)
	for s := range r.n {
		seats[s] = seatJSON{Name: r.names[s], Kind: kindNames[r.kinds[s]], Online: r.online(s)}
		if r.humans != nil && r.humans[s] != nil {
			seats[s].Takeover = r.humans[s].takeover
		}
	}
	for s := range r.n {
		r.snaps[s] = nil
		if r.kinds[s] != seatHuman {
			continue
		}
		var v snapshot
		if r.started {
			if r.views == nil || r.views[s] == nil {
				continue
			}
			v = *r.views[s]
			h := r.humans[s]
			v.Phase = "game"
			v.YourTurn = h.waiting && !v.Over && v.Current == s
			v.Takeover = h.takeover
		} else {
			v = lobbySnapshot(r.va, r.level, r.seed)
		}
		if multi && !v.Over {
			v.Seed = "" // 知道種子就能算出整副牌
		}
		v.Seat, v.Host, v.Multi, v.Seats, v.Names = s, s == 0, multi, seats, r.names
		v.Error, v.Next = r.fatal, r.next
		b, err := json.Marshal(v)
		if err != nil {
			log.Printf("房間 %s：畫面編碼失敗：%v", r.id, err)
			continue
		}
		r.snaps[s] = b
	}
	r.ver++
	for ch := range r.subs {
		select {
		case ch <- struct{}{}:
		default: // 還沒讀走上一次通知，反正會讀到最新的
		}
	}
}

// online：座位上的人現在連著（mu 要鎖著）。
func (r *room) online(seat int) bool {
	return r.conns[seat] > 0 || time.Since(r.seen[seat]) < offlineGrace
}

// connect / disconnect 記錄一條連線（SSE 或等待中的輪詢）；上下線時更新大家的畫面。
func (r *room) connect(seat int, ch chan struct{}) {
	r.mu.Lock()
	defer r.mu.Unlock()
	was := r.online(seat)
	r.subs[ch] = true
	r.conns[seat]++
	r.seen[seat] = time.Now()
	r.active = time.Now()
	if !was {
		r.render()
	}
}

func (r *room) disconnect(seat int, ch chan struct{}) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.subs, ch)
	r.conns[seat]--
	r.seen[seat] = time.Now()
	r.active = time.Now()
	if r.conns[seat] == 0 {
		time.AfterFunc(offlineGrace+100*time.Millisecond, func() {
			r.mu.Lock()
			defer r.mu.Unlock()
			if !r.online(seat) {
				r.render()
			}
		})
	}
}

// seatOf 回傳通行碼對應的座位（-1 = 不是這個房間的人，例如已經離開或被請出去）。
func (r *room) seatOf(token string) int {
	r.mu.Lock()
	defer r.mu.Unlock()
	if s, ok := r.tokens[token]; ok {
		return s
	}
	return -1
}

// join 讓一位朋友坐進空位（seat < 0 = 第一個空位），回傳他的通行碼。
func (r *room) join(seat int, name string) (string, int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.started {
		return "", 0, errors.New("遊戲已經開始了")
	}
	if seat < 0 {
		for s, k := range r.kinds {
			if k == seatOpen {
				seat = s
				break
			}
		}
		if seat < 0 {
			return "", 0, errors.New("座位已經坐滿了")
		}
	}
	if seat >= r.n || r.kinds[seat] != seatOpen {
		return "", 0, errors.New("這個位子不能坐（可能剛被別人坐走）")
	}
	token := randomHex(16)
	r.tokens[token] = seat
	r.kinds[seat] = seatHuman
	r.names[seat] = r.uniqueName(cleanName(name, fmt.Sprintf("玩家%d", seat)), seat)
	r.active = time.Now()
	r.render()
	return token, seat, nil
}

// uniqueName 和別的座位同名時加上座位號碼。mu 要鎖著。
func (r *room) uniqueName(name string, seat int) string {
	for s, other := range r.names {
		if s != seat && other == name {
			return fmt.Sprintf("%s（%d）", name, seat)
		}
	}
	return name
}

// lobby 處理等待室的指令：房主開始遊戲、調整空位（AI / 等朋友，已經有人時請他離開），其他人離開。
func (r *room) lobby(seat int, c command) reply {
	switch c.Cmd {
	case "start":
		if seat != 0 {
			return reply{Error: "只有房主能開始遊戲"}
		}
		if err := r.start(); err != nil {
			return reply{Error: err.Error()}
		}
		return reply{OK: true}
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.active = time.Now()
	if r.started {
		return reply{Error: "遊戲已經開始了"}
	}
	switch c.Cmd {
	case "seat":
		if seat != 0 {
			return reply{Error: "只有房主能調整座位"}
		}
		t := c.Target
		if t <= 0 || t >= r.n {
			return reply{Error: "沒有這個座位"}
		}
		r.vacate(t)
		if c.Kind == "ai" {
			r.kinds[t] = seatAI
			r.names[t] = aiName(t)
		}
	case "leave":
		if seat == 0 {
			return reply{Error: "房主不能離開（不想玩了就直接關掉）"}
		}
		r.vacate(seat)
	default:
		return reply{Error: "還沒開始，不能 " + c.Cmd}
	}
	r.render()
	return reply{OK: true}
}

// vacate 讓座位變回空位（坐在上面的人的通行碼作廢）。mu 要鎖著。
func (r *room) vacate(seat int) {
	for tok, s := range r.tokens {
		if s == seat {
			delete(r.tokens, tok)
		}
	}
	r.kinds[seat] = seatOpen
	r.names[seat] = ""
}

// takeover 讓 AI 接手（或停止接手）某位真人的座位：朋友斷線或離開時，大家不必一直等他。
// 他正在等指令時叫醒他的 Act，馬上由 AI 行動。
func (r *room) takeover(by int, c command) reply {
	r.mu.Lock()
	if !r.started || r.done {
		r.mu.Unlock()
		return reply{Error: "現在不能這樣做"}
	}
	t := c.Target
	if t < 0 || t >= r.n || r.humans[t] == nil {
		r.mu.Unlock()
		return reply{Error: "這個座位不是真人"}
	}
	h := r.humans[t]
	r.active = time.Now()
	h.takeover = c.On
	if !c.On || !h.waiting {
		r.render()
		r.mu.Unlock()
		return reply{OK: true}
	}
	h.waiting = false
	r.mu.Unlock()
	return h.deliver(command{Cmd: "wake"})
}

// send 把遊戲指令交給座位上正在等待的真人；不是他的回合就直接拒絕（不排隊，避免之後才執行舊指令）。
func (r *room) send(seat int, c command) reply {
	r.mu.Lock()
	r.active = time.Now()
	if r.done {
		r.mu.Unlock()
		return reply{Error: "這局已經結束"}
	}
	h := r.humans[seat]
	if !h.waiting {
		r.mu.Unlock()
		if h.takeover {
			return reply{Error: "AI 正在接手你的座位，先按「我回來了」"}
		}
		return reply{Error: "還沒輪到你"}
	}
	h.waiting = false // 先佔住，同時送來的第二個指令會被拒絕
	r.mu.Unlock()
	return h.deliver(c)
}

// paced 是 AI 的座位：輪到他時先把目前的畫面推給真人，每一手至少間隔 aiDelay，讓人看得出發生了什麼。
type paced struct {
	session.Logged
	r *room
}

func (p paced) Unwrap() player.Player { return p.Logged }

func (p paced) Act(v game.PlayerView) game.Action {
	p.r.publish()
	start := time.Now()
	a := p.Logged.Act(v)
	if !p.r.pause(start) {
		return player.QuitAction
	}
	return a
}

// pause 等到 start 之後 aiDelay；房間關閉時回傳 false。
func (r *room) pause(start time.Time) bool {
	select {
	case <-time.After(r.aiDelay - time.Since(start)):
		return true
	case <-r.ctx.Done():
		return false
	}
}

// webHuman 是網頁上的真人：Act 等瀏覽器送來的指令。
type webHuman struct {
	r           *room
	seat        int
	reqs        chan request
	helper      *hgroup.Bot
	pendingUndo chan reply // 回溯的結果要等 run 重建局面後才知道

	waiting  bool // 正在等指令（同一時間只收一個指令）；r.mu 保護
	takeover bool // AI 接手這個座位；r.mu 保護
}

func (h *webHuman) Name() string { return h.r.names[h.seat] }

func (h *webHuman) setWaiting(on bool) {
	h.r.mu.Lock()
	h.waiting = on
	h.r.mu.Unlock()
}

func (h *webHuman) takenOver() bool {
	h.r.mu.Lock()
	defer h.r.mu.Unlock()
	return h.takeover
}

// deliver 把指令送進 Act，等它的回覆。
func (h *webHuman) deliver(c command) reply {
	req := request{cmd: c, reply: make(chan reply, 1)}
	select {
	case h.reqs <- req:
	case <-h.r.ctx.Done():
		return reply{Error: "這局已經結束"}
	}
	select {
	case rep := <-req.reply:
		return rep
	case <-h.r.ctx.Done():
		return reply{Error: "這局已經結束"}
	}
}

func (h *webHuman) Act(v game.PlayerView) game.Action {
	announce := true
	for {
		if h.takenOver() {
			return h.autoAct(v)
		}
		h.setWaiting(true)
		if announce {
			h.r.publish()
			announce = false
		}
		var req request
		select {
		case req = <-h.reqs:
		case <-h.r.ctx.Done():
			return player.QuitAction
		}
		if req.cmd.Cmd == "wake" { // 被設成 AI 接手
			req.reply <- reply{OK: true}
			continue
		}
		a, rep, done := h.handle(req.cmd, v)
		if done {
			if a == player.UndoAction {
				h.pendingUndo = req.reply
			} else {
				req.reply <- rep
			}
			return a
		}
		req.reply <- rep
	}
}

// autoAct 是 AI 接手時的一手：和 AI 座位一樣先推畫面、控制節奏。
func (h *webHuman) autoAct(v game.PlayerView) game.Action {
	h.r.publish()
	start := time.Now()
	a := h.helper.Act(v)
	h.r.log.SetWhy(v.Turn, "AI 接手："+h.helper.Why())
	h.r.notes[v.Turn] = "AI 接手"
	if !h.r.pause(start) {
		return player.QuitAction
	}
	return a
}

// handle 處理一個指令；done 為 true 時 a 是這一手的行動。
func (h *webHuman) handle(c command, v game.PlayerView) (a game.Action, rep reply, done bool) {
	g := h.r.g
	h.r.mu.Lock()
	multi := h.r.multi()
	h.r.mu.Unlock()
	switch c.Cmd {
	case "peek":
		if multi {
			return a, reply{Error: "多人局不能偷看"}, false
		}
		h.r.log.Note(g.Turn, fmt.Sprintf("t%d 偷看了自己的手牌", g.Turn))
		for _, hc := range g.Hands[h.seat] {
			rep.Peek = append(rep.Peek, toCard(hc.Card, true))
		}
		rep.OK = true
		return a, rep, false
	case "undo":
		if multi {
			return a, reply{Error: "多人局不能回溯"}, false
		}
		return player.UndoAction, rep, true
	case "auto":
		a = h.helper.Act(v)
		why := h.helper.Why()
		h.r.log.SetWhy(v.Turn, "AI 代打："+why)
		h.r.notes[v.Turn] = "AI 代打"
		return a, reply{OK: true, Info: fmt.Sprintf("AI 代打：%s（%s）", h.actionText(a), why)}, true
	case "play":
		a = game.Play(c.Slot)
	case "discard":
		a = game.Discard(c.Slot)
	case "hint":
		switch c.Kind {
		case "color":
			if c.Color < 0 || c.Color >= game.NumColors || !v.Variant.CanClue(game.Color(c.Color)) {
				return a, reply{Error: "這局不能提示這個顏色"}, false
			}
			a = game.HintColorTo(c.Target, game.Color(c.Color))
		case "rank":
			a = game.HintRankTo(c.Target, game.Rank(c.Rank))
		default:
			return a, reply{Error: "看不懂提示內容"}, false
		}
	default:
		return a, reply{Error: "看不懂指令 " + c.Cmd}, false
	}
	if err := g.Validate(a); err != nil {
		return a, reply{Error: err.Error()}, false
	}
	return a, reply{OK: true}, true
}

// Reject 不會發生（Act 回傳前已經 Validate 過）；萬一發生就記進紀錄，Run 會再呼叫 Act。
func (h *webHuman) Reject(a game.Action, err error) {
	h.r.log.Note(h.r.g.Turn, fmt.Sprintf("不合法的行動 %+v：%v", a, err))
}

// actionText 用文字描述一個還沒執行的行動（位置 1-based）。
func (h *webHuman) actionText(a game.Action) string {
	switch a.Type {
	case game.ActPlay:
		return fmt.Sprintf("打出位置 %d", a.Slot+1)
	case game.ActDiscard:
		return fmt.Sprintf("棄掉位置 %d", a.Slot+1)
	}
	if a.Hint.Kind == game.HintColor {
		return fmt.Sprintf("提示 %s「%s色」", h.r.names[a.Target], a.Hint.Color.Name())
	}
	return fmt.Sprintf("提示 %s「數字 %d」", h.r.names[a.Target], a.Hint.Rank)
}

// cleanName 整理玩家名字：去掉空白、最多 12 個字；空的（或冒充 AI 的）就用 def。
func cleanName(s, def string) string {
	s = strings.TrimSpace(s)
	if rs := []rune(s); len(rs) > 12 {
		s = string(rs[:12])
	}
	if s == "" || strings.HasPrefix(s, "AI-") {
		return def
	}
	return s
}
