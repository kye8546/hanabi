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
	bySave   bool     // 第一次被點到的那個提示可能是 save clue（救牌時允許點到重複的牌）
	start    bool     // 起手牌（Level 3 Playing Multiple 1's：起手牌的 1 由舊到新打）
	bad      bool     // 已知是沒用的牌（Level 3 Fix Clue、多出來的 1），不論筆記上還剩哪些牌面
	cm       bool     // Level 4 Chop Moved：沒被提示，但視同有隱形提示、永遠不棄（chop 跳過它）；被提示點到後就是一般的已提示牌
	touches  int      // 被正向提示點到的次數（Level 4 Order Chop Move：點到次數不同的 1 不算同一組）
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
	fpAt     []int // 提示當下每位玩家的 finesse position（Wrong Prompt 用）
	chop     bool  // 焦點在 chop 上（Level 3 Chop-Focus 例外：這張 1 最先打）
	fresh    []int // 這次提示點到的新牌（之後重新找連接牌時，它們不能當成焦點的連接牌）
	passed   []int // 被認為是 finesse 對象、輪到時卻沒有盲打的人（他看到別人也有那張牌）
}

// state 是 AI 依公開資訊重播整局後的認知。
type state struct {
	n, me     int
	level     int          // 使用的 H-Group 慣例等級
	colors    []game.Color // 這局用到的顏色（Variant.Colors）
	all       idSet        // 這局所有可能的牌面（新牌的筆記從這裡開始）
	hints     int          // 目前的提示數（重播時追蹤，5 Stall 判斷給提示者能不能棄牌用）
	hands     [][]int      // 每位玩家的手牌 ID，0 = 最左（最新）
	notes     map[int]*note
	cards     map[int]game.Card // 我看得到、或已經公開的牌面
	stacks    [game.NumColors]game.Rank
	discarded [numIdents]int // 已進棄牌堆（含打錯）的張數
	recs      []*clueRec
	processed int  // 已處理的事件數
	seq       int  // 目前處理到的事件序號
	midGame   bool // 已經有人棄過牌（Early Game 結束）

	overlay map[int]idSet                              // 模擬提示時暫時覆蓋的筆記
	why     string                                     // 最近一次決策的理由（除錯用）
	trace   func(e game.Event, id int, old, new idSet) // 除錯用：筆記變化時呼叫

	// optimistic：模擬接收者時，把我（給提示者）看不到的自己手牌當成「可能就是需要的連接牌」。
	// 只用來判斷接收者會不會有更簡單的解讀，不能用來判斷他看不看得懂真正的牌
	optimistic bool

	holderOf map[int]int // 牌 ID → 持有者座位
	// vis[obs][i]：obs 看得到（我也知道牌面）的他人手牌中，牌面 i 的張數；visOK 為 false 時要重算
	vis   [game.MaxPlayers][numIdents]int8
	visOK bool

	// pending：解讀提示的當下，把這次被點到的牌視為已被 giver 提示（讓 Good Touch 推論立刻生效）
	pending      []int
	pendingGiver int

	touchedNow  []int // 解讀提示期間：這次被點到的牌
	skipFinesse []int // 重新找連接牌時：這次提示已經輪過、沒有盲打的人（不會是他）
	skipCards   []int // 重新找連接牌時：他沒盲打的那些牌（不算已經有人會打）
	sameClue    []int // 重新找連接牌時：和焦點同一個提示點到的新牌（不能當連接牌）

	excused int // Wrong Prompt：這張 finesse 的牌在這次行動不套用 Finesse Urgency（-1 = 無）

	// cmLast：每位玩家最近一次被 Chop Move 的牌。他下一次棄牌若棄掉 critical，表示那次 Chop Move 是誤會，
	// 撤銷它（Level 4 Critical Discards after a Chop Move）
	cmLast map[int][]int
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
		notes:   map[int]*note{},
		cards:   map[int]game.Card{},
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
	s.holderOf = map[int]int{}
	for p, h := range s.hands {
		for _, id := range h {
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
				s.cards[vc.Card.ID] = vc.Card
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
			s.cards[e.Card.ID] = e.Card
		}
	}
	s.visOK = false
	for _, e := range v.Log[s.processed:] {
		if e.Card.Rank != 0 && mine(e) {
			s.cards[e.Card.ID] = e.Card
		}
		s.seq++
		var before map[int]idSet
		if s.trace != nil {
			before = map[int]idSet{}
			for id, n := range s.notes {
				before[id] = n.poss
			}
		}
		s.apply(e)
		s.visOK = false
		s.commitNotes()
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
		s.checkDeps(e, -1)
		s.onClue(e)
		s.hints--
	case game.EvPlay, game.EvMisplay, game.EvDiscard:
		// Level 4 The Order Chop Move：要在打出前（照他打牌前的手牌）算他跳過了幾張 1
		ocmTarget := -1
		if e.Type == game.EvPlay && s.level >= 4 {
			if k := s.ocmSkip(e.Player, e.Card.ID); k > 0 && k < s.n {
				ocmTarget = (e.Player + k) % s.n
			}
		}
		var wpRec *clueRec
		var wpCard int
		var wpIDs idSet
		if e.Type == game.EvMisplay && s.level >= 2 {
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
		s.hands[e.Player] = slices.Insert(s.hands[e.Player], 0, e.Card.ID)
		s.holderOf[e.Card.ID] = e.Player
		s.notes[e.Card.ID] = s.newNote()
	}
}

func (s *state) removeCard(p, id int) {
	if i := slices.Index(s.hands[p], id); i >= 0 {
		s.hands[p] = slices.Delete(s.hands[p], i, i+1)
	}
	delete(s.holderOf, id)
}

func (s *state) holder(id int) int {
	if p, ok := s.holderOf[id]; ok {
		return p
	}
	return -1
}

// see 回傳 obs 這個座位能否看到牌 id 的牌面（以我知道的資訊為限）。
func (s *state) see(obs, id int) (ident, bool) {
	if s.holder(id) == obs {
		return 0, false
	}
	c, ok := s.cards[id]
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

// oneAway 是 chop 左邊下一張會被棄的牌（Level 4 5's Chop Move 的「one-away from chop」）；沒有則回傳 -1。
func (s *state) oneAway(p int) int {
	h := s.hands[p]
	seen := 0
	for i := len(h) - 1; i >= 0; i-- {
		if !s.gotten(h[i]) && !s.notes[h[i]].cm {
			if seen++; seen == 2 {
				return h[i]
			}
		}
	}
	return -1
}

// chopMove（Level 4 Chop Moves）：把 p 的這些牌標成 Chop Moved（隱形提示、永遠不棄），他的 chop 因此往左移。
func (s *state) chopMove(p int, ids []int) {
	if len(ids) == 0 {
		return
	}
	for _, id := range ids {
		s.notes[id].cm = true
	}
	s.cmLast[p] = slices.Clone(ids)
}

// undoChopMove（Level 4 Critical Discards after a Chop Move）：被 Chop Move 後的下一次棄牌棄掉 critical，
// 最合理的解釋是那次 Chop Move 根本不是給提示者的本意，撤銷它。其他情況 Chop Move 永遠不撤銷
// （Accidental Chop Moves：大家都以為那張已經安全，沒去救後來才變成 critical 的牌）。
func (s *state) undoChopMove(e game.Event) {
	ids, ok := s.cmLast[e.Player]
	if !ok {
		return
	}
	delete(s.cmLast, e.Player)
	if !s.critical(identOfCard(e.Card)) {
		return
	}
	for _, id := range ids {
		if n, ok := s.notes[id]; ok && !n.clued {
			n.cm = false
		}
	}
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

// revealGTP：被拿到的牌 x 打出後公開了牌面 t。依 Good Touch，跟它互相「看得到」的其他被拿到的牌都不會是 t
// （後拿到的那張，給提示者當時看得到先拿到的那張；同一個 play clue 點到的牌也不會重複）。
func (s *state) revealGTP(x int, t ident) {
	xn := s.notes[x]
	hx := s.holder(x)
	if xn.giver < 0 {
		return
	}
	for p, h := range s.hands {
		for _, id := range h {
			n := s.notes[id]
			if id == x || n.giver < 0 || n.bad || !n.poss.has(t) || n.poss.count() == 1 {
				continue
			}
			// 同一個 play clue 點到的也不會重複；save clue 則允許（Save Principle 可以違反 Good Touch）
			if xn.seq > n.seq && xn.giver != p || n.seq > xn.seq && (n.giver != hx || p == s.me && s.giverKnew(id, x, t)) ||
				n.seq == xn.seq && !n.bySave {
				n.poss &^= setOf(t)
			}
		}
	}
}

// sarcastic（Level 3 The Sarcastic Discard）：有人棄掉一張被拿到、而且不是垃圾的牌 t，表示別人手上已被拿到的牌裡有 t
// （他發現自己那張是重複的）。看得到的人知道是誰；都看不到就是我。那人手上只有一張可能是 t 的牌時，它就是 t
// （有好幾張時不保證是哪一張）。鎖手被迫棄牌不算。
func (s *state) sarcastic(e game.Event) {
	t := identOfCard(e.Card)
	if s.level < 3 || !s.live(e.Card.ID) || s.trash(t) || s.chop(e.Player) < 0 {
		return
	}
	holder := -1
	for p, h := range s.hands {
		if p == e.Player || p == s.me {
			continue
		}
		for _, id := range h {
			if c, ok := s.cards[id]; ok && s.live(id) && identOfCard(c) == t {
				holder = p
			}
		}
	}
	if holder < 0 && e.Player != s.me {
		holder = s.me
	}
	if holder < 0 {
		return
	}
	var cands []int
	for _, id := range s.hands[holder] {
		if s.live(id) && s.knownPoss(holder, id).has(t) {
			cands = append(cands, id)
		}
	}
	if len(cands) == 1 {
		s.notes[cands[0]].poss = setOf(t)
	}
}

// markExcessOnes（Level 3 Playing Multiple 1's）：手上被提示的 1 比它們可能的牌面種類還多時，
// 依 Good Touch 多出來的那幾張（照出牌順序排在最後的）永遠不會打出，是已知沒用的牌。
func (s *state) markExcessOnes() {
	if s.level < 3 {
		return
	}
	for p := range s.n {
		var ones []int
		var u idSet
		for _, id := range s.playOrder(p) {
			if k := s.knownPoss(p, id); s.notes[id].clued && k&^rankSet(1) == 0 {
				ones = append(ones, id)
				u |= k
			}
		}
		if k := u.count(); len(ones) > k {
			for _, id := range ones[k:] {
				s.notes[id].bad, s.notes[id].willPlay = true, false
			}
		}
	}
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

// gottenAs 回傳（obs 角度）除了 exclude 之外，是否已有被拿到的牌是 i：
// 看得到的被提示牌實際是 i，或筆記已確定是 i。
// giver 看不到自己的手牌，所以他手上的牌只看筆記（他不會因為看不到的牌違反 Good Touch）。
func (s *state) gottenAs(obs, giver int, i ident, exclude int) bool {
	for p, h := range s.hands {
		for _, id := range h {
			if id == exclude || !s.live(id) || !s.knowsFinessed(giver, id) {
				continue
			}
			if t, ok := s.see(obs, id); ok && t == i && (p != giver || s.giverMayKnow(obs, id, i)) {
				return true
			}
			if t, ok := s.possOf(id).single(); ok && t == i {
				return true
			}
		}
	}
	return false
}

// knowsFinessed：q 知道牌 id 已被拿到。被提示的牌大家都看得到；被 finesse 的牌，那次提示的接收者看不到焦點，
// 只有在他的每種解讀都要這張牌被 finesse 時才知道（其他人看得到焦點，照實際牌面推得出）。
// 例如 Alice 給 Bob 白色（w3，finesse Cathy 的 w2），Bob 自己可能以為是直接的 w2：之後 Bob 提示 Donald 的 2，
// 不知道 Cathy 那張 w2 已被拿到，Donald 不能依 Good Touch 排除自己那張是 w2。
func (s *state) knowsFinessed(q, id int) bool {
	n := s.notes[id]
	if n.clued || n.finRec == nil || q != n.finRec.target || q == s.me {
		return true
	}
	if len(n.finRec.cands) == 0 {
		return false
	}
	for _, c := range n.finRec.cands {
		if !slices.ContainsFunc(c.deps, func(d dep) bool { return d.card == id && d.finesse }) {
			return false
		}
	}
	return true
}

// alreadyGotten 回傳（obs 角度）連接牌 m 是否已經有人會打、不需要 prompt / finesse：
// 某張被拿到的牌筆記確定是 m，或持有者本來就會打它（已知可打、或是 play clue 的焦點）、而且它可能是 m
// （obs 看得到時必須真的是 m）。本來就會打的牌不算 prompt。
// 給提示者看不到自己的牌，所以他手上的牌只有筆記確定是 m 才算。exclude 是焦點與這條連接已用到的牌。
func (s *state) alreadyGotten(obs, giver int, m ident, exclude []int) bool {
	for pl, h := range s.hands {
		for _, id := range h {
			if slices.Contains(exclude, id) || slices.Contains(s.skipCards, id) || !s.live(id) {
				continue
			}
			p := s.possOf(id)
			if pl == giver {
				// 給提示者看不到自己的牌：他自己的已提示牌，只有在別人也看得出他知道是 m 時才會被當成連接牌。
				// 就算看得到它實際就是 m，他不知道就不可能靠它，那就是 finesse / prompt 別人
				// （Level 10 Sarcastic Finesse：「the duplicated card in your own hand must be unknown. Otherwise,
				// the clue receiver will simply think that the clued card is the next card in the suit」）。
				// 我是給提示者時，我對自己牌的私下推論（筆記）別人不一定推得出，不能拿來當大家的共識
				if obs == giver {
					if t, ok := p.single(); ok && t == m {
						return true
					}
					continue
				}
				t, ok := s.see(obs, id)
				if !ok && pl == s.me {
					t, ok = p.single() // 我是給提示者：他看得到我這張，我的推論確定的話就是它
				}
				if ok {
					if t == m && p.has(m) && s.giverMayKnow(obs, id, m) {
						return true
					}
				} else if s.optimistic && giver == s.me && obs != s.me && p.has(m) && s.mineCanBe(id, m) {
					// 我是給提示者、在模擬接收者：他看得到我這張，我看不到，它可能就是 m
					return true
				}
				continue
			}
			if t, ok := p.single(); ok && t == m {
				return true
			}
			if p.has(m) && (s.holderKnowsPlayable(id) || s.notes[id].willPlay) {
				if t, ok := s.see(obs, id); ok && t == m || !ok && s.mineCanBe(id, m) {
					return true
				}
			}
		}
	}
	return false
}

// giverMayKnow：（obs 角度）牌 id 的持有者可能知道它是 m：它是某次給他的提示的焦點，而除了 m 以外的解讀
// 全都要靠 obs 自己的牌（持有者看得到 obs 的牌，obs 自己看不到，所以 obs 無法排除他已經知道）。
// 持有者正在給提示時，他點到的牌（他看得到）是 c 的解讀也不算：他不會點到可能和自己已提示的牌重複的牌
// （Good Touch，`maybeMine`），點了就表示他已排除那個解讀——例如他「g1 或 g3」的牌還在等 Reverse Finesse，
// 他又點到別人的 g1，在他心中那張就是 g3。
func (s *state) giverMayKnow(obs, id int, m ident) bool {
	h := s.holder(id)
	// 大家都推得出的筆記（不是我自己手上的牌——那是我的私下推論）已確定是 m，或提示本身就說明是 m
	if t, ok := s.notes[id].poss.single(); ok && t == m && h != s.me {
		return true
	}
	if t, ok := s.notes[id].explicit.single(); ok && t == m {
		return true
	}
	for i := len(s.recs) - 1; i >= 0; i-- {
		rec := s.recs[i]
		if rec.focus != id || rec.target != h {
			continue
		}
		for _, c := range rec.cands {
			if c.id == m || h == s.pendingGiver && slices.ContainsFunc(s.pending, func(p int) bool {
				t, ok := s.see(obs, p)
				return ok && t == c.id
			}) {
				continue
			}
			// 他看得到 obs 的牌：這個解讀要靠 obs 的牌，或 obs 手上有已被拿到、可能就是 c.id 的牌
			// （他看到就會依 Good Touch 排除），obs 就無法確定他不知道
			if c.save || !slices.ContainsFunc(c.deps, func(d dep) bool { return d.player == obs }) && !s.mayHoldGotten(obs, c.id, id) {
				return false
			}
		}
		return slices.ContainsFunc(rec.cands, func(c cand) bool { return c.id == m })
	}
	return false
}

// mayHoldGotten：obs 手上（除了 exclude）有已被拿到、可能是 i 的牌（依筆記；我的筆記已即時寫入推論）。
func (s *state) mayHoldGotten(obs int, i ident, exclude int) bool {
	for _, id := range s.hands[obs] {
		if id != exclude && s.live(id) && s.notes[id].poss.has(i) {
			return true
		}
	}
	return false
}

// mineCanBe：牌 id 在我手上時，依我自己的推論它還可能是 m。別人看得到實際牌面，
// 我推得出不是的話，他們也不會那樣解讀。不在我手上的牌一律回傳 true。
func (s *state) mineCanBe(id int, m ident) bool {
	return s.holder(id) != s.me || s.myPoss(id).has(m)
}

// ---- 自己手牌的推論 ----

// commitNotes 把我對自己手牌推得出的結論即時寫回卡片筆記（H-Group：每次提示後都要更新筆記），
// 之後不論是決定自己的行動，還是推測別人怎麼解讀，都用同一份筆記。
// 別人的牌只保留大家都推得出的共同筆記：若混入某些人看不到的資訊（例如第三人手上的牌），
// 各家對同一張牌的筆記會不一致，模擬別人的解讀時反而出錯。
func (s *state) commitNotes() {
	for _, id := range s.hands[s.me] {
		s.notes[id].poss = s.deduce(s.me, id)
	}
}

// deduce 回傳持有者 h 對自己的牌 id 推得出的可能：筆記之外，再排除
//   - 張數已用完的牌面（棄牌、煙火、他看得到的他人手牌）；
//   - 依 Good Touch 不可能的牌面（別處已被拿到、且雙方給提示者互相看得到的牌）。
func (s *state) deduce(h, id int) idSet {
	n := s.notes[id]
	p := n.poss
	for _, i := range p.list() {
		if s.remainingFor(h, i) <= 0 || s.gotten(id) && s.gtpExcluded(h, id, i) {
			p &^= 1 << i
		}
	}
	if p == 0 {
		return n.poss
	}
	return p
}

// myPoss 是我對自己手牌推得出的可能（筆記已即時更新，這裡再算一次以涵蓋解讀提示當下的推論）。
func (s *state) myPoss(id int) idSet { return s.deduce(s.me, id) }

// remaining 回傳 i 扣掉棄牌、已打出、以及我看得到的他人手牌後，還有幾張下落不明。
func (s *state) remaining(i ident) int { return s.remainingFor(s.me, i) }

// remainingFor 回傳從 obs 的角度，i 扣掉棄牌、已打出、以及 obs 看得到的他人手牌後還剩幾張
// （obs 不是我時，我自己的牌我看不到，就不扣）。
func (s *state) remainingFor(obs int, i ident) int {
	if !s.visOK {
		s.vis = [game.MaxPlayers][numIdents]int8{}
		for p, h := range s.hands {
			for _, id := range h {
				if c, ok := s.cards[id]; ok {
					for o := range s.n {
						if o != p {
							s.vis[o][identOfCard(c)]++
						}
					}
				}
			}
		}
		s.visOK = true
	}
	left := i.copies() - s.discarded[i] - int(s.vis[obs][i])
	if s.played(i) {
		left--
	}
	return left
}

// visibleGotten 回傳我的牌 mine 依 Good Touch 是否不可能是 i。
func (s *state) visibleGotten(i ident, mine int) bool { return s.gtpExcluded(s.me, mine, i) }

// gtpExcluded 回傳（持有者 h 的角度）他的被拿到的牌 card 依 Good Touch 是否不可能是 i：
// 別處已有被拿到的牌是 i（他看得到它就是 i，或筆記已確定是 i），而且後來拿到的那張，
// 給提示者當時看得到先被拿到的那張（看不到的重複不算違反 Good Touch）。
// 同一個 save clue 點到的牌允許重複（Save Principle 可以違反 Good Touch）。
func (s *state) gtpExcluded(h, card int, i ident) bool {
	m := s.notes[card]
	for q, hand := range s.hands {
		for _, id := range hand {
			pend := !s.gotten(id) && slices.Contains(s.pending, id)
			if id == card || !s.live(id) && !pend {
				continue
			}
			if m.giver >= 0 && !s.knowsFinessed(m.giver, id) {
				continue // card 的給提示者不知道 id 已被 finesse，點到同一張不算違反 Good Touch
			}
			known := false
			if q == h || q == s.me {
				t, ok := s.notes[id].poss.single()
				known = ok && t == i
			} else if c, ok := s.cards[id]; ok {
				known = identOfCard(c) == i
			}
			if !known {
				continue
			}
			o := *s.notes[id]
			if pend {
				o.giver, o.seq = s.pendingGiver, s.seq+1
			}
			if m.giver < 0 || o.giver < 0 {
				return true
			}
			if o.seq < m.seq && (m.giver != q || h == s.me && s.giverKnew(card, id, i)) || m.seq < o.seq && o.giver != h ||
				o.seq == m.seq && !m.bySave {
				return true
			}
		}
	}
	return false
}

// giverKnew：我的牌 nc 被提示時，給提示者自己手上已被提示的牌 x 是 t，他當時是否已經知道（那樣依 Good Touch
// nc 就不會是 t）。他看不到自己的牌，只能靠 x 當時的解讀；我看不到自己的牌，算出的解讀可能多出「要靠我的牌是某張」
// 的可能（例如 Reverse Finesse 我），而他看得到我的牌。假設 nc 是 t 時，那些可能都不成立（我的牌依張數不可能是那張），
// 他當時就知道 x 是 t。
func (s *state) giverKnew(nc, x int, t ident) bool {
	cs, ok := s.notes[nc].giverOwn[x]
	if !ok || len(cs) == 0 || s.holder(nc) != s.me {
		return false
	}
	for _, c := range cs {
		if c.id == t {
			continue
		}
		if c.save {
			return false
		}
		possible := true
		for _, d := range c.deps {
			if d.player != s.me {
				continue // 別人的牌他和我都看得到，這個解讀對他確實成立
			}
			if c, ok := s.cards[d.card]; ok {
				possible = possible && identOfCard(c) == d.id // 已經公開
			} else if !s.notes[d.card].poss.has(d.id) || d.id == t && s.remaining(t) <= 1 {
				possible = false // 我的牌不是它；或 nc 是 t 的話 t 已沒有多的張數
			}
		}
		if possible {
			return false
		}
	}
	return true
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
