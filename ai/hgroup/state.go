package hgroup

import (
	"slices"

	"hanabi/game"
)

// note 是一張手牌的「卡片筆記」（H-Group 要求每個人替每張牌做筆記）。
type note struct {
	clued    bool     // 曾被提示點到
	finessed bool     // 沒被點到但已被承諾要盲打（finesse），視同隱形提示
	finRec   *clueRec // finesse 這張牌的那次提示（knowsFinessed 用）
	willPlay bool     // 是 play clue 的焦點（或被 prompt / finesse），持有者遲早會打
	giver    int      // 第一次「拿到」這張牌（提示或 finesse）的給提示者；-1 = 還沒被拿到
	seq      int      // 第一次被拿到時的事件序號
	start    bool     // 起手牌（Level 3 Playing Multiple 1's：起手牌的 1 由舊到新打）
	bad      bool     // 已知是沒用的牌（Level 3 Fix Clue、多出來的 1），不論筆記上還剩哪些牌面
	cm       bool     // Level 4 Chop Moved：沒被提示，但視同有隱形提示、永遠不棄（chop 跳過它）；被提示點到後就是一般的已提示牌
	touches  int      // 被正向提示點到的次數（Level 4 Order Chop Move：點到次數不同的 1 不算同一組）
	hinted   uint16   // 點到它的提示值（位元 0–7 顏色、8–12 數字）；正向資訊的多寡（Level 5 Prompts in Multi-Color Variants）
	drawn    int      // 抽到這張牌時的事件序號（起手牌是 0；Level 5 Layered Finesse：提示之後才抽的牌不在 layer 裡）
	explicit idSet    // 只由提示本身（正向與負向）得到的可能
	poss     idSet    // 依慣例推論後的可能（隱含資訊）
	// giverOwn：這張牌第一次被提示時，給提示者自己手上被提示的牌當時的解讀（牌 ID → 解讀），
	// 用來判斷他當時知不知道自己那張是什麼（giverKnew）
	giverOwn map[int][]cand
}

// dep 是一張提示要能打出所需、但還沒被「拿到」的連接牌（prompt 或 finesse）。
type dep struct {
	player  int
	card    int // 預期打出的牌 ID
	id      ident
	finesse bool // true = finesse（應立即盲打）；false = prompt
	// layers：我（推論者）看得到、他會先打出的其他能打的牌有幾張（Level 5 Layered Finesse 的 layer，或 Hidden Finesse
	// 先被當成 prompt 打出的已提示牌）。card 是他第一張會打的牌——他看不到自己的牌，以為它就是 id
	layers int
}

// cand 是被提示焦點牌的一種可能解讀。
type cand struct {
	id   ident
	deps []dep // Level 1 最多一個；Level 2 可多個（依打出順序）
	save bool  // save clue 的解讀（不代表可以打）
}

// clueRec 記錄一次提示的推論，供之後驗證 finesse / prompt 是否兌現、或失誤時重置。
type clueRec struct {
	giver, target int
	focus         int
	cands         []cand // 接收者角度的所有可能解讀
	plan          []dep  // 我看得到焦點牌時，實際需要的連接（由我來追蹤他人是否兌現）
	changed       []int  // 這次提示寫下隱含筆記的牌，失誤時要重置
	// fallback：需要接收者自己 self-prompt / self-finesse 的解讀。只在其他解讀都不成立後才考慮
	// （Level 2 The Self-Finesse：「only consider ... if there are no other possibilities」）
	fallback []cand
	fpAt     []int      // 提示當下每位玩家的 finesse position（Wrong Prompt 用）
	chop     bool       // 焦點在 chop 上（Level 3 Chop-Focus 例外：這張 1 最先打）
	fresh    []int      // 這次提示點到的新牌（之後重新找連接牌時，它們不能當成焦點的連接牌）
	passed   []int      // 被認為是 finesse 對象、輪到時卻沒有盲打的人（他看到別人也有那張牌）
	seq      int        // 提示時的事件序號
	stale    bool       // 別人打出了 plan 要的牌面，煙火更新後要重算 plan（replan）
	saveView []bool     // 我看得到焦點時：每位旁觀者是否可能把它讀成 save clue（interp.saveView）
	free     freeChoice // Rainbow 變體的 Free Choice（提示當下算好，重新找連接牌時沿用）
	struck   bool       // 提示之後有人打錯（筆記被重置過，有人沒盲打可能是因為這樣，passMeansMe 用）
}

// state 是 AI 依公開資訊重播整局後的認知。
type state struct {
	n, me  int
	level  int          // 使用的 H-Group 慣例等級
	colors []game.Color // 這局用到的顏色（Variant.Colors）
	all    idSet        // 這局所有可能的牌面（新牌的筆記從這裡開始）
	hints  int          // 目前的提示數（重播時追蹤，5 Stall 判斷給提示者能不能棄牌用）
	deck   int          // 目前牌堆的張數（重播時追蹤，Level 6 判斷是不是終局的拖延用）
	hands  [][]int      // 每位玩家的手牌 ID，0 = 最左（最新）
	// 以下三個都用牌 ID 索引（track 讓長度涵蓋看過的每張牌；用 slice 而不是 map，查找是熱點）
	notes     []*note     // 每張牌的筆記（沒看過的牌是 nil）
	cards     []game.Card // 我看得到、或已經公開的牌面（Rank 0 = 不知道；讀 ok 用 card）
	stacks    [game.NumColors]game.Rank
	discarded [numIdents]int // 已進棄牌堆（含打錯）的張數
	recs      []*clueRec
	processed int  // 已處理的事件數
	seq       int  // 目前處理到的事件序號
	midGame   bool // 已經有人棄過牌（Early Game 結束）

	why   string                                     // 最近一次決策的理由（寫進對局紀錄）
	trace func(e game.Event, id int, old, new idSet) // 除錯用：筆記變化時呼叫

	sim // 推論時暫時套用的假設情境（見 sim）

	holderOf []int // 牌 ID → 持有者座位（-1 = 不在任何人手上）
	// vis[obs][i]：obs 看得到（我也知道牌面）的他人手牌中，牌面 i 的張數；visOK 為 false 時要重算
	vis   [game.MaxPlayers][numIdents]int8
	visOK bool

	excused int // Wrong Prompt：這張 finesse 的牌在這次行動不套用 Finesse Urgency（-1 = 無）
	// deferred：Level 5 Urgency Principle 的例外——這次行動（緊急的提示）可以延後盲打，不算沒接 finesse
	deferred bool

	// cmLast：每位玩家最近一次被 Chop Move 的牌。他下一次棄牌若棄掉 critical，表示那次 Chop Move 是誤會，
	// 撤銷它（Level 4 Critical Discards after a Chop Move）
	cmLast map[int][]int
}

// sim 是推論時暫時套用的假設情境：解讀一次提示（實際發生或模擬）、替舊提示重新找連接牌、
// 推測別人下一手時才有值，平常全是零值。改它之前一律先 saveSim、結束時整組還原
// （defer s.saveSim()()），所以可以巢狀使用，也不會漏還原。
type sim struct {
	overlay map[int]idSet // 暫時覆蓋的筆記（例如接收者「提示後」的筆記，possOf 先讀它）

	// optimistic：模擬接收者時，把我（給提示者）看不到的自己手牌當成「可能就是需要的連接牌」。
	// 只用來判斷接收者會不會有更簡單的解讀，不能用來判斷他看不看得懂真正的牌
	optimistic bool

	// pending：解讀提示的當下，把這次被點到的牌視為已被 pendingGiver 提示（讓 Good Touch 推論立刻生效）
	pending      []int
	pendingGiver int

	touchedNow  []int // 解讀提示期間：這次被點到的牌（不在 finesse position 上）
	skipFinesse []int // 重新找連接牌時：這次提示已經輪過、沒有盲打的人（不會是他）
	skipCards   []int // 重新找連接牌時：他沒盲打的那些牌（不算已經有人會打）
	sameClue    []int // 重新找連接牌時：和焦點同一個提示點到的新牌（不能當連接牌）
	// assumeMine：推測別人的角度時，把我自己 finesse position 上看不到的牌當成「就是要的那張」（他看得到；
	// Level 5 Ambiguous Finesse：他沒盲打，是不是因為看到我有那張）
	assumeMine bool

	free freeChoice // 解讀 / 重新找連接牌時：這個提示的 Free Choice（Rainbow 變體，variant_rainbow.go）
}

// saveSim 存下目前的假設情境，回傳還原它的函式。
func (s *state) saveSim() (restore func()) {
	saved := s.sim
	return func() { s.sim = saved }
}

func newState(v game.PlayerView, level int) *state {
	s := &state{
		n:       v.NumPlayers,
		me:      v.Seat,
		level:   level,
		colors:  v.Colors(),
		all:     suitIdents(v.Variant),
		hints:   game.MaxHints,
		excused: -1,
		hands:   make([][]int, v.NumPlayers),
		cmLast:  map[int][]int{},
	}
	for p, h := range v.Hands {
		for _, vc := range h {
			s.hands[p] = append(s.hands[p], vc.Card.ID)
		}
	}
	// 從目前狀態往回倒推出開局（手牌、煙火、棄牌），之後再從頭重播所有事件
	s.stacks = v.Fireworks
	for _, c := range v.Discards {
		s.discarded[identOfCard(c)]++
	}
	for i := len(v.Log) - 1; i >= 0; i-- {
		e := v.Log[i]
		switch e.Type {
		case game.EvDraw:
			s.hands[e.Player] = s.hands[e.Player][1:]
		case game.EvPlay, game.EvMisplay, game.EvDiscard:
			s.hands[e.Player] = slices.Insert(s.hands[e.Player], e.Slot, e.Card.ID)
			if e.Type == game.EvPlay {
				s.stacks[e.Card.Color] = e.Card.Rank - 1
			} else {
				s.discarded[identOfCard(e.Card)]--
			}
		}
	}
	s.hints = initialHints(v)
	s.deck = v.DeckSize
	for _, e := range v.Log {
		if e.Type == game.EvDraw {
			s.deck++
		}
	}
	for p, h := range s.hands {
		for _, id := range h {
			s.track(id)
			s.notes[id] = s.newNote()
			s.notes[id].start = true
			s.holderOf[id] = p
		}
	}
	return s
}

// initialHints 回傳開局的提示數：正常對局是 8；從 8 照紀錄推算和目前畫面不符時（測試直接改了提示數），
// 改從目前的提示數往回推（打出 5 是否回收提示無法確定，當成有回收）。
func initialHints(v game.PlayerView) int {
	fwd := game.MaxHints
	for _, e := range v.Log {
		switch {
		case e.Type == game.EvHint:
			fwd--
		case e.Type == game.EvDiscard, e.Type == game.EvPlay && e.Card.Rank == game.MaxRank:
			fwd = min(fwd+1, game.MaxHints)
		}
	}
	if fwd == v.Hints {
		return game.MaxHints
	}
	h := v.Hints
	for i := len(v.Log) - 1; i >= 0; i-- {
		switch e := v.Log[i]; {
		case e.Type == game.EvHint:
			h++
		case e.Type == game.EvDiscard, e.Type == game.EvPlay && e.Card.Rank == game.MaxRank:
			h = max(h-1, 0)
		}
	}
	return h
}

// track 讓用牌 ID 索引的 slice 涵蓋 id。
func (s *state) track(id int) {
	for len(s.notes) <= id {
		s.notes = append(s.notes, nil)
		s.cards = append(s.cards, game.Card{})
		s.holderOf = append(s.holderOf, -1)
	}
}

// setCard 記下看得到或已公開的牌面。
func (s *state) setCard(c game.Card) {
	s.track(c.ID)
	s.cards[c.ID] = c
}

// note 回傳牌 id 的筆記（沒看過的牌回傳 false）。
func (s *state) note(id int) (*note, bool) {
	if id < 0 || id >= len(s.notes) || s.notes[id] == nil {
		return nil, false
	}
	return s.notes[id], true
}

// card 回傳我知不知道牌 id 的牌面。
func (s *state) card(id int) (game.Card, bool) {
	if id < 0 || id >= len(s.cards) || s.cards[id].Rank == 0 {
		return game.Card{}, false
	}
	return s.cards[id], true
}

func (s *state) newNote() *note { return &note{explicit: s.all, poss: s.all, giver: -1} }

// got 記錄這張牌第一次被拿到（提示或 finesse）的人與時間。
func (s *state) got(id, giver int) {
	if n := s.notes[id]; n.giver < 0 {
		n.giver, n.seq = giver, s.seq
	}
}

// sync 讀進目前看得到的牌面，並處理上次之後的新事件。
func (s *state) sync(v game.PlayerView) {
	for p, h := range v.Hands {
		for _, vc := range h {
			if p != s.me && vc.Visible {
				s.setCard(vc.Card)
			}
		}
	}
	// 事件公開的牌面：別人手上的牌我當時就看得到，先全部記下；我自己打出 / 棄掉的牌，處理到那個事件時才記
	// （一次補處理多個事件時，不能在處理較早的事件時就「知道」自己手上後來才公開的牌）
	mine := func(e game.Event) bool {
		return e.Player == s.me && (e.Type == game.EvPlay || e.Type == game.EvMisplay || e.Type == game.EvDiscard)
	}
	for _, e := range v.Log[s.processed:] {
		if e.Card.Rank != 0 && !mine(e) {
			s.setCard(e.Card)
		}
	}
	s.visOK = false
	for _, e := range v.Log[s.processed:] {
		if e.Card.Rank != 0 && mine(e) {
			s.setCard(e.Card)
		}
		s.seq++
		var before map[int]idSet
		if s.trace != nil {
			before = map[int]idSet{}
			for id, n := range s.notes {
				if n != nil {
					before[id] = n.poss
				}
			}
		}
		s.apply(e)
		s.visOK = false
		s.commitNotes()
		// 我對自己焦點牌的推論（commitNotes）可能排除了某些解讀，剩下的都要同一張別人的 finesse 牌時，記下它
		for _, rec := range s.recs {
			if rec.target == s.me {
				s.markCommon(rec)
			}
		}
		s.markExcessOnes()
		for id, old := range before {
			if n := s.notes[id]; n.poss != old && s.holder(id) >= 0 {
				s.trace(e, id, old, n.poss)
			}
		}
	}
	s.processed = len(v.Log)
}

func (s *state) apply(e game.Event) {
	switch e.Type {
	case game.EvHint:
		s.deferred = s.deferrable(e)
		s.checkDeps(e, -1)
		s.deferred = false
		s.onClue(e)
		s.hints--
	case game.EvPlay, game.EvMisplay, game.EvDiscard:
		// Level 4 The Order Chop Move：要在打出前（照他打牌前的手牌）算他跳過了幾張 1
		ocmTarget := -1
		if e.Type == game.EvPlay && s.knows(lvChopMove) {
			if k := s.ocmSkip(e.Player, e.Card.ID); k > 0 && k < s.n {
				ocmTarget = (e.Player + k) % s.n
			}
		}
		var wpRec *clueRec
		var wpCard int
		var wpIDs idSet
		if e.Type == game.EvMisplay && s.knows(lvWrongPrompt) {
			wpCard, wpIDs, wpRec = s.wrongPrompt(e.Player, e.Card.ID)
		}
		if wpRec != nil {
			s.excused = wpCard // 他這次打錯是因為 Wrong Prompt，不算沒盲打那張 finesse 的牌
		}
		s.checkDeps(e, e.Card.ID)
		s.excused = -1
		if e.Type == game.EvPlay {
			s.revealGTP(e.Card.ID, identOfCard(e.Card))
		}
		if e.Type == game.EvDiscard {
			s.focusDiscarded(e.Card.ID)
			s.sarcastic(e)
			s.undoChopMove(e)
		}
		s.removeCard(e.Player, e.Card.ID)
		id := identOfCard(e.Card)
		switch e.Type {
		case game.EvPlay:
			s.stacks[e.Card.Color] = e.Card.Rank
			if e.Card.Rank == game.MaxRank && s.hints < game.MaxHints {
				s.hints++
			}
		case game.EvMisplay:
			s.discarded[id]++
			if wpRec != nil {
				s.applyWrongPrompt(wpCard, wpIDs, wpRec)
			} else {
				s.onStrike(e.Card.ID)
				for _, rec := range s.recs {
					rec.struck = true
				}
			}
		case game.EvDiscard:
			s.discarded[id]++
			// Early Game 是有人棄掉 chop 之前（Level 1）：棄掉已被提示的牌（已知垃圾、Sarcastic Discard、鎖手被迫）不算
			// （Level 9：「only ended when the first completely unknown card is discarded」）
			if !s.gotten(e.Card.ID) {
				s.midGame = true
			}
			s.hints = min(s.hints+1, game.MaxHints)
		}
		if ocmTarget >= 0 {
			if c := s.chop(ocmTarget); c >= 0 {
				s.chopMove(ocmTarget, []int{c})
			}
		}
		s.afterPlay()
	case game.EvDraw:
		s.deck--
		s.hands[e.Player] = slices.Insert(s.hands[e.Player], 0, e.Card.ID)
		s.track(e.Card.ID)
		s.holderOf[e.Card.ID] = e.Player
		s.notes[e.Card.ID] = s.newNote()
		s.notes[e.Card.ID].drawn = s.seq
	}
}

func (s *state) removeCard(p, id int) {
	if i := slices.Index(s.hands[p], id); i >= 0 {
		s.hands[p] = slices.Delete(s.hands[p], i, i+1)
	}
	s.holderOf[id] = -1
}

func (s *state) holder(id int) int {
	if id < 0 || id >= len(s.holderOf) {
		return -1
	}
	return s.holderOf[id]
}

// see 回傳 obs 這個座位能否看到牌 id 的牌面（以我知道的資訊為限）。
func (s *state) see(obs, id int) (ident, bool) {
	if s.holder(id) == obs {
		return 0, false
	}
	c, ok := s.card(id)
	if !ok {
		return 0, false
	}
	return identOfCard(c), true
}

func (s *state) possOf(id int) idSet {
	if p, ok := s.overlay[id]; ok {
		return p
	}
	return s.notes[id].poss
}

// gotten 回傳這張牌是否已被「拿到」（被提示或被 finesse）。
func (s *state) gotten(id int) bool { n := s.notes[id]; return n.clued || n.finessed }

// live：被拿到、而且不是已知沒用的牌（Good Touch 的「別處已被拿到」只算這種）。
func (s *state) live(id int) bool { return s.gotten(id) && !s.notes[id].bad }

// chop 是最右邊沒被提示、沒被 finesse、也沒被 Chop Move 的牌；沒有則回傳 -1（鎖手：牌全被提示或 Chop Move）。
func (s *state) chop(p int) int {
	h := s.hands[p]
	for i := len(h) - 1; i >= 0; i-- {
		if !s.gotten(h[i]) && !s.notes[h[i]].cm {
			return h[i]
		}
	}
	return -1
}

// layerSlot：rec 提示當下就在 p 手上、還沒被拿到的牌中最左邊那張（不含 skip）。Level 5 Layered / Hidden Finesse：
// 盲打（或當成 prompt 打出）的牌是別張能打的牌時，真正的牌在這裡（之後才抽到的牌不算）。沒有則回傳 -1。
func (s *state) layerSlot(rec *clueRec, p, skip int) int {
	for _, id := range s.hands[p] {
		if id != skip && s.notes[id].drawn <= rec.seq && !s.gotten(id) {
			return id
		}
	}
	return -1
}

// finessePos 是最左邊沒被提示、也沒被 finesse 的牌；沒有則回傳 -1。
func (s *state) finessePos(p int) int {
	for _, id := range s.hands[p] {
		if !s.gotten(id) {
			return id
		}
	}
	return -1
}

// ---- 牌面性質 ----

func (s *state) played(i ident) bool { return i.rank() <= s.stacks[i.color()] }

// dead 回傳 i 是否因為前面某張牌全被棄光而永遠打不出去。
func (s *state) dead(i ident) bool {
	for r := s.stacks[i.color()] + 1; r < i.rank(); r++ {
		if s.discarded[identOf(i.color(), r)] >= identOf(i.color(), r).copies() {
			return true
		}
	}
	return false
}

func (s *state) trash(i ident) bool { return s.played(i) || s.dead(i) }

// critical 回傳 i 是否為最後一張（其他複本都已棄掉）。
func (s *state) critical(i ident) bool {
	return !s.trash(i) && s.discarded[i] == i.copies()-1
}

func (s *state) playableNow(i ident) bool { return i.rank() == s.stacks[i.color()]+1 }

func (s *state) useful() idSet {
	var u idSet
	for _, i := range s.all.list() {
		if !s.trash(i) {
			u |= 1 << i
		}
	}
	return u
}

func (s *state) knownPlayable(id int) bool {
	if s.notes[id].bad {
		return false
	}
	p := s.myPoss(id)
	for _, i := range p.list() {
		if !s.playableNow(i) {
			return false
		}
	}
	return p != 0
}

func (s *state) knownTrash(id int) bool {
	if s.notes[id].bad {
		return true
	}
	p := s.myPoss(id)
	for _, i := range p.list() {
		if !s.trash(i) && !s.visibleGotten(i, id) {
			return false
		}
	}
	return p != 0
}

// ---- 他人角度：這張牌的持有者自己知道能打嗎 ----

func (s *state) holderKnowsPlayable(id int) bool {
	if s.notes[id].bad {
		return false
	}
	p := s.possOf(id)
	for _, i := range p.list() {
		if !s.playableNow(i) {
			return false
		}
	}
	return p != 0
}
