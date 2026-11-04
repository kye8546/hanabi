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

// room 是一局遊戲。遊戲狀態（g、players、紀錄）只在 run 的 goroutine 裡讀寫；
// HTTP 那邊只拿 mu 保護的畫面快照，指令經由 human.reqs 送進去。
type room struct {
	id      string
	seed    uint64
	level   int
	names   []string
	aiDelay time.Duration
	ctx     context.Context
	cancel  context.CancelFunc

	g       *game.Game
	players []player.Player
	log     *session.Log
	human   *webHuman

	mu      sync.Mutex
	snap    []byte // 真人座位最新的畫面 JSON
	subs    map[chan struct{}]bool
	active  time.Time // 最後一次有人連線或下指令
	waiting bool      // 真人正在等指令（同一時間只收一個指令）
	done    bool
	fatal   string // 程式錯誤（例如 AI 回傳不合法行動），顯示在畫面上
}

// command 是瀏覽器送來的指令。
type command struct {
	Cmd    string `json:"cmd"` // play / discard / hint / auto / undo / peek
	Slot   int    `json:"slot"`
	Target int    `json:"target"`
	Kind   string `json:"kind"` // hint：color / rank
	Color  int    `json:"color"`
	Rank   int    `json:"rank"`
}

type reply struct {
	OK    bool       `json:"ok"`
	Error string     `json:"error,omitempty"`
	Info  string     `json:"info,omitempty"`
	Peek  []cardJSON `json:"peek,omitempty"`
}

type request struct {
	cmd   command
	reply chan reply // 緩衝 1，送的人不會卡住
}

func newRoom(id string, n, level int, va game.Variant, seed uint64, name, logDir string, aiDelay time.Duration) (*room, error) {
	g, err := game.NewVariant(n, va, seed)
	if err != nil {
		return nil, err
	}
	names := make([]string, n)
	names[0] = name
	for i := 1; i < n; i++ {
		names[i] = fmt.Sprintf("AI-%d", i)
	}
	gl, err := session.NewLog(logDir, names, level, va, seed)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithCancel(context.Background())
	r := &room{id: id, seed: seed, level: level, names: names, aiDelay: aiDelay, ctx: ctx, cancel: cancel,
		g: g, log: gl, subs: map[chan struct{}]bool{}, active: time.Now()}
	r.human = &webHuman{r: r, seat: 0, reqs: make(chan request)}
	r.players = r.newPlayers()
	r.publish()
	return r, nil
}

// newPlayers 和 console 一樣組裝：真人座位包著代打用的 AI，所有座位都寫紀錄；AI 另外控制節奏、讓畫面跟上。
func (r *room) newPlayers() []player.Player {
	r.human.helper = hgroup.New(r.names[0], r.level)
	ps := []player.Player{session.Helped{
		Player: session.LoggedHuman{Logged: session.Logged{Player: r.human, L: r.log}, H: r.human},
		Helper: r.human.helper,
	}}
	for i := 1; i < len(r.names); i++ {
		ps = append(ps, paced{session.Logged{Player: hgroup.New(r.names[i], r.level), L: r.log}, r})
	}
	return ps
}

// run 跑完整局（含回溯）。結束後保留最後的畫面，讓玩家看結果、下載紀錄。
func (r *room) run() {
	defer func() {
		r.log.Close(r.g)
		r.mu.Lock()
		r.done = true
		r.mu.Unlock()
		r.publish()
	}()
	for {
		err := player.Run(r.g, r.players)
		switch {
		case errors.Is(err, player.ErrUndo):
			rep := r.human.pendingUndo
			r.human.pendingUndo = nil
			ng, ps, turn, ok := session.Rewind(r.g, r.seed, r.human.seat, r.newPlayers)
			if !ok {
				rep <- reply{Error: "還沒有可以回溯的行動"}
				continue
			}
			r.g, r.players = ng, ps
			r.log.Rewind(r.g, turn)
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

// publish 重算真人座位的畫面並通知所有連線。只在 run 的 goroutine（或開局時）呼叫。
func (r *room) publish() {
	r.mu.Lock()
	defer r.mu.Unlock()
	s := buildSnapshot(r.g.View(r.human.seat), r.names, r.level, r.seed, r.waiting)
	s.Error = r.fatal
	b, err := json.Marshal(s)
	if err != nil {
		log.Printf("房間 %s：畫面編碼失敗：%v", r.id, err)
		return
	}
	r.snap = b
	for ch := range r.subs {
		select {
		case ch <- struct{}{}:
		default: // 還沒讀走上一次通知，反正會讀到最新的
		}
	}
}

func (r *room) touch() {
	r.mu.Lock()
	r.active = time.Now()
	r.mu.Unlock()
}

// send 把指令交給正在等待的真人；不是他的回合就直接拒絕（不排隊，避免 AI 行動後才執行舊指令）。
func (r *room) send(c command) reply {
	r.mu.Lock()
	r.active = time.Now()
	if !r.waiting {
		r.mu.Unlock()
		return reply{Error: "還沒輪到你"}
	}
	r.waiting = false // 先佔住，同時送來的第二個指令會被拒絕
	r.mu.Unlock()
	req := request{cmd: c, reply: make(chan reply, 1)}
	select {
	case r.human.reqs <- req:
	case <-r.ctx.Done():
		return reply{Error: "這局已經結束"}
	}
	select {
	case rep := <-req.reply:
		return rep
	case <-r.ctx.Done():
		return reply{Error: "這局已經結束"}
	}
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
	select {
	case <-time.After(p.r.aiDelay - time.Since(start)):
	case <-p.r.ctx.Done():
		return player.QuitAction
	}
	return a
}

// webHuman 是網頁上的真人：Act 等瀏覽器送來的指令。
type webHuman struct {
	r           *room
	seat        int
	reqs        chan request
	helper      *hgroup.Bot
	pendingUndo chan reply // 回溯的結果要等 run 重建局面後才知道
}

func (h *webHuman) Name() string { return h.r.names[h.seat] }

func (h *webHuman) setWaiting(on bool) {
	h.r.mu.Lock()
	h.r.waiting = on
	h.r.mu.Unlock()
}

func (h *webHuman) Act(v game.PlayerView) game.Action {
	h.setWaiting(true)
	h.r.publish()
	for {
		var req request
		select {
		case req = <-h.reqs:
		case <-h.r.ctx.Done():
			return player.QuitAction
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
		h.setWaiting(true)
	}
}

// handle 處理一個指令；done 為 true 時 a 是這一手的行動。
func (h *webHuman) handle(c command, v game.PlayerView) (a game.Action, rep reply, done bool) {
	g := h.r.g
	switch c.Cmd {
	case "peek":
		h.r.log.Note(g.Turn, fmt.Sprintf("t%d 偷看了自己的手牌", g.Turn))
		for _, hc := range g.Hands[h.seat] {
			rep.Peek = append(rep.Peek, toCard(hc.Card, true))
		}
		rep.OK = true
		return a, rep, false
	case "undo":
		return player.UndoAction, rep, true
	case "auto":
		a = h.helper.Act(v)
		why := h.helper.Why()
		h.r.log.SetWhy(v.Turn, "AI 代打："+why)
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

// cleanName 整理玩家名字：去掉空白、最多 12 個字，空的就是「你」。
func cleanName(s string) string {
	s = strings.TrimSpace(s)
	if rs := []rune(s); len(rs) > 12 {
		s = string(rs[:12])
	}
	if s == "" || strings.HasPrefix(s, "AI-") {
		return "你"
	}
	return s
}
