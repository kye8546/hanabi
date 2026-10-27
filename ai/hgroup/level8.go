// Level 8（docs/level-8.mdx）：終局（End-Game）——No Chop Moves in the End-Game（endGame）；
// 被迫的終局拖延（使用者要求所有等級都有）：pace、mustBurn、burnClue。

package hgroup

import (
	"slices"
	"strings"

	"hanabi/game"
)

// endGame（Level 8 The End-Game）：pace 小於人數就是終局（牌堆空了也算），大家都算得出（publicPace）。
// 終局不 Chop Move（No Chop Moves in the End-Game：沒有「之後」可言，不必替之後保留牌）：像 Chop Move 的提示
// 改當成下一個最簡單的解讀，通常是拖延提示（findChopMove、tempoChopMove）；Order Chop Move 與
// Scream / Shout Discard Chop Move 也不算（hanabi-bot 同樣在終局關掉 Last Resorts 的棄牌；挑戰題 3 Cards Left：
// 提示 0 個、Alice 有已知能打的 r4 卻棄牌，是為了換提示讓 Bob 的三張 5 來得及打，不是 Scream Discard）。
func (s *state) endGame() bool {
	return s.knows(lvEndGame) && (s.deck == 0 || s.publicPace() < s.n)
}

// pace = 目前分數 + 牌堆張數 + 人數 − 這局還拿得到的最高分（H-Group Level 8）。
// 每次棄牌 pace 減 1；pace < 0 就不可能拿到最高分。
func (s *state) pace(v game.PlayerView) int {
	best := 0
	for _, c := range s.colors {
		for r := game.Rank(1); r <= game.MaxRank; r++ {
			if r > s.stacks[c] && s.discarded[identOf(c, r)] >= identOf(c, r).copies() {
				break
			}
			best++
		}
	}
	return v.Score() + v.DeckSize + s.n - best
}

// mustBurn：牌堆還有牌、有提示可用，而且這次棄牌會讓最高分拿不到時，必須拖延：
//   - pace 0：再棄一張就不可能拿到最高分。
//   - pace 1，有隊友手上沒有能打的牌，而我的牌可能能打（Pace +1 Rule：讓他去做最後一次棄牌）。
//   - 牌堆只剩 1 張，而某位隊友手上有 2 張以上還需要打的牌（他只剩一回合打不完）。
func (s *state) mustBurn(v game.PlayerView) bool {
	if v.DeckSize == 0 || v.Hints == 0 {
		return false
	}
	switch s.pace(v) {
	case 0:
		return true
	case 1:
		if s.teammateIdle() && s.mightPlay() {
			return true
		}
	}
	// 隊友手上還需要打的牌比他剩下的回合多（一回合只能打一張），而我不抽牌能讓他多一回合，就拖延
	// （Level 8：Team Distribution / Burning）。牌堆只剩 1 張時，我一棄牌每人就只剩一回合，
	// 任何握有 2 張以上還需要打的牌的隊友都打不完（之後的人也可能接著拖延），一律拖延。
	for k := 1; k < s.n; k++ {
		left := s.turnsLeft(k, v.DeckSize-1)
		if s.teammateNeed(k) > left && (v.DeckSize == 1 || s.turnsLeft(k, v.DeckSize) > left) {
			return true
		}
	}
	return false
}

// teammateNeed：我後面第 k 位手上還需要打的牌（我看得到的牌面，不重複計算）有幾種。
func (s *state) teammateNeed(k int) int {
	var need idSet
	for _, id := range s.hands[(s.me+k)%s.n] {
		if t := identOfCard(s.cards[id]); !s.trash(t) {
			need |= setOf(t)
		}
	}
	return need.count()
}

// stepStallBeforePlay（Level 8 End-Game Strategy / Burning）：終局中打牌也會抽牌、讓遊戲提早結束。照原文的問法
// 「Does stalling produce a line that is better than playing?」，把「現在打出確定能打的牌」和「現在拖延（有提示就給
// 拖延提示，提示 0 個就棄牌換一個提示）」各推演到遊戲結束（endgameRollout），拖延能打出的牌比較多才拖延。
// 挑戰題 3 Cards Left：提示 0 個，Alice 打 r4 的話 Bob 的三張 5 來不及打完；棄牌換提示、之後大家拖延才拿得到滿分。
// 試過只看「有隊友的牌比他剩下的回合多」就拖延：大家都替別人拖延、沒人打牌，5 人局 -0.1–0.6。
func (s *state) stepStallBeforePlay(t *turn) (game.Action, string, bool) {
	slot := s.bestPlay()
	if !s.knows(lvEndGame) || t.v.DeckSize == 0 || slot < 0 || !s.endGame() {
		return game.Action{}, "", false
	}
	mine, ok := s.myPoss(s.hands[s.me][slot]).single()
	if !ok {
		return game.Action{}, "", false
	}
	var a game.Action
	var why string
	switch {
	case t.v.Hints > 0:
		ev, ok := s.burnClue(t.clues(s))
		if !ok {
			return game.Action{}, "", false
		}
		a, why = ev.act, "終局拖延：推演起來先拖延、我的牌晚點打，大家打得完的牌比較多"
	case !t.v.CanDiscard():
		return game.Action{}, "", false
	default:
		// 提示 0 個：棄牌換一個提示讓之後的人拖延（要棄得安全：已知垃圾，或我看得到每一張還需要的牌）
		why = "終局：推演起來棄牌換提示讓大家拖延、我的牌晚點打，大家打得完的牌比較多"
		if pa, pwhy, ok := s.positionalMove(t.v); ok && pa.Type == game.ActDiscard {
			a, why = pa, pwhy
		} else if ts := s.trashSlot(false); ts >= 0 {
			a = game.Discard(ts)
		} else if c := s.chop(s.me); c >= 0 && s.seesEvery() {
			a = game.Discard(slices.Index(s.hands[s.me], c))
		} else {
			return game.Action{}, "", false
		}
	}
	if s.endgameRollout(mine, true, t.v) <= s.endgameRollout(mine, false, t.v) {
		return game.Action{}, "", false
	}
	return a, why, true
}

// endgameRollout：從現在推演到遊戲結束，回傳之後最多打得出幾張牌。模型（兩條路線同樣保守）：每個人只打已被拿到、
// 牌面確定（我看得到，或我自己的筆記確定）的牌；沒被提示的牌、之後才給的提示都不算。每一回合，有能打的牌的人
// 可以打（先打能接上別人的那張）或拖延——有提示、牌堆還有牌就給提示（不抽牌），否則棄牌（換回提示）——取打出最多的
// 一條（大家照同一個推演合作）；沒有能打的牌就拖延。牌堆空了之後拖延沒有用，能打就打。
// 我這一手：stall 時拖延，否則打出 mine。局面只由煙火、提示數、牌堆、最後一輪剩幾回合、輪到誰決定，記下算過的。
func (s *state) endgameRollout(mine ident, stall bool, v game.PlayerView) int {
	known := make([][]ident, s.n)
	for p, h := range s.hands {
		for _, id := range h {
			var t ident
			ok := false
			if p == s.me {
				t, ok = s.myPoss(id).single()
			} else {
				t, ok = s.see(s.me, id)
			}
			if ok && s.live(id) && !s.trash(t) {
				known[p] = append(known[p], t)
			}
		}
	}
	type pos struct {
		stacks                   [game.NumColors]game.Rank
		hints, deck, final, turn int
	}
	memo := map[pos]int{}
	score := func(st [game.NumColors]game.Rank) int {
		n := 0
		for _, c := range s.colors {
			n += int(st[c])
		}
		return n
	}
	var best func(q pos) int
	// act：q.turn 那位玩家打出 t（ok）或拖延之後，推進到下一回合
	act := func(q pos, t ident, ok bool) int {
		before := q.deck
		if ok {
			q.stacks[t.color()] = t.rank()
			if t.rank() == game.MaxRank && q.hints < game.MaxHints {
				q.hints++
			}
			q.deck = max(q.deck-1, 0)
		} else if q.hints > 0 && q.deck > 0 {
			q.hints--
		} else {
			q.hints = min(q.hints+1, game.MaxHints)
			q.deck = max(q.deck-1, 0)
		}
		switch {
		case q.final > 0:
			q.final--
		case before > 0 && q.deck == 0:
			q.final = s.n
		}
		q.turn = (q.turn + 1) % s.n
		return best(q)
	}
	best = func(q pos) int {
		if q.final == 0 {
			return score(q.stacks)
		}
		if r, ok := memo[q]; ok {
			return r
		}
		p := (s.me + q.turn) % s.n
		var next ident
		found, waiting := false, false
		for _, t := range known[p] {
			if t.rank() <= q.stacks[t.color()] {
				continue
			}
			waiting = true
			if t.rank() != q.stacks[t.color()]+1 {
				continue
			}
			if !found || t.rank() < game.MaxRank && slices.ContainsFunc(known, func(k []ident) bool {
				return slices.Contains(k, identOf(t.color(), t.rank()+1))
			}) {
				next, found = t, true
			}
		}
		for _, k := range known {
			for _, t := range k {
				waiting = waiting || t.rank() > q.stacks[t.color()]
			}
		}
		var r int
		switch {
		case !waiting:
			r = score(q.stacks) // 沒有人有牌要打了
		case !found:
			r = act(q, 0, false)
		case q.deck == 0:
			r = act(q, next, true)
		default:
			r = max(act(q, next, true), act(q, 0, false))
		}
		memo[q] = r
		return r
	}
	start := pos{stacks: s.stacks, hints: v.Hints, deck: v.DeckSize, final: -1}
	return act(start, mine, !stall) - score(s.stacks)
}

// unblocksIdle：我打出 slot 的牌後，某位隊友手上（我看得到）接在它後面的牌就能打，而他現在沒有別張能打——他在等我這張。
func (s *state) unblocksIdle(slot int) bool {
	t, ok := s.myPoss(s.hands[s.me][slot]).single()
	if !ok || t.rank() >= game.MaxRank {
		return false
	}
	nxt := identOf(t.color(), t.rank()+1)
	for q, h := range s.hands {
		if q == s.me {
			continue
		}
		waits, other := false, false
		for _, id := range h {
			c, _ := s.see(s.me, id)
			waits = waits || c == nxt && s.gotten(id)
			other = other || s.playableNow(c) && s.gotten(id)
		}
		if waits && !other {
			return true
		}
	}
	return false
}

// turnsLeft：我行動後牌堆剩 deck 張，之後每回合都抽牌的話，我後面第 k 位還有幾回合
// （再 deck 回合抽完，接著每人最後一回合，共 deck + n 回合）。
func (s *state) turnsLeft(k, deck int) int {
	if k > deck+s.n {
		return 0
	}
	return (deck+s.n-k)/s.n + 1
}

// teammateIdle：有隊友手上（我看得到的）沒有任何現在能打的牌。
func (s *state) teammateIdle() bool {
	for p, h := range s.hands {
		if p == s.me {
			continue
		}
		idle := true
		for _, id := range h {
			if s.playableNow(identOfCard(s.cards[id])) {
				idle = false
			}
		}
		if idle {
			return true
		}
	}
	return false
}

// mightPlay：我手上有牌可能是現在能打的牌。
func (s *state) mightPlay() bool {
	for _, id := range s.hands[s.me] {
		for _, i := range s.myPoss(id).list() {
			if s.playableNow(i) {
				return true
			}
		}
	}
	return false
}

// burnClue 挑一個不會造成誤解的拖延提示：先選讓對方確定某張已提示牌能打的重複提示，
// 再選對方會正確理解、也不點到垃圾的提示。
func (s *state) burnClue(evs []clueEval) (clueEval, bool) {
	if ev, ok := best(evs, func(e clueEval) bool { return e.kind == kindTempo && e.knowsPlay && !e.misleads }); ok {
		return ev, true
	}
	return best(evs, func(e clueEval) bool { return e.understood && e.bad == 0 && !e.misleads })
}

// posRec：Level 8 Positional Discard / Positional Misplay。from 在終局棄掉（或打錯）slot 位置上「不該棄」的牌，
// 要他之後、那個位置上有能打的牌（持有者自己還不知道）的最後 plays 位玩家盲打那個位置的牌
// （原文：「if two or more players have a playable card on the same slot, then the blind-play is always targeted
// at the final player with the playable card」）。每位玩家只在他之後的第一回合回應；輪回 from 就結束。
type posRec struct {
	from, slot, plays int
	cards             []int  // 每位玩家當時在 slot 位置的牌 ID（from 自己、或沒有那個位置的人是 -1）
	target            []bool // 我看得到的：那張牌當時能打、持有者還不知道（我自己的那張輪到我時再判斷）
	acted             []bool // 之後已經行動過的人
	done              int    // 已經回應（打出那張牌）的人數
}

// posExpected：P 這一手「該棄」的牌——大家都知道沒用的已提示牌（myTrashSlot 的順序），否則 chop（publicChop）。
// 鎖手（沒有 chop、也沒有已知垃圾）回傳 -1，他棄什麼都不是 Positional Discard。
func (s *state) posExpected(P int) int {
	h := s.hands[P]
	for slot := len(h) - 1; slot >= 0; slot-- {
		if s.screamTrash(h[slot]) {
			return h[slot]
		}
	}
	return s.publicChop(P, nil)
}

// detectPositional（Level 8，在棄牌 / 打錯的事件處理前呼叫）：P 棄掉或打錯牌 id（牌面 t）是不是 Positional Discard / Misplay。
//   - 只算沒被提示、沒被 Chop Move、沒有任何提示要他盲打的牌（finesse 的盲打打錯、盲打 Chop Moved 的牌都不算），
//     而且不是 critical（他看得到每一張還需要的牌，才知道自己沒被提示的牌都是垃圾或重複；棄掉 critical 是失誤）。
//   - 棄牌：不是他該棄的牌（已知垃圾或 chop），也不是提示本身就確定沒用的牌 → Positional Discard，一張盲打。
//   - 打錯：就是他該棄的那張（棄它不會被當成 Positional Discard）→ Positional Misplay，一張盲打；
//     不是那張（Positional Discard 也行得通，卻多花一個閃電）→ Double Positional Misplay，兩張盲打。
//
// 回應中的盲打（resp：s.pos 等的那張牌）不是新的 Positional Misplay。
func (s *state) detectPositional(e game.Event, resp bool) *posRec {
	if resp || e.Type == game.EvPlay || s.critical(identOfCard(e.Card)) {
		return nil
	}
	P, id := e.Player, e.Card.ID
	plays := 1
	switch {
	case e.Type == game.EvDiscard && !s.posDiscardLike(P, id):
		return nil
	case e.Type == game.EvMisplay:
		if !s.posMisplayLike(P, id) {
			return nil
		}
		if id != s.posExpected(P) {
			plays = 2
		}
	}
	pr := &posRec{from: P, slot: e.Slot, plays: plays, cards: make([]int, s.n), target: make([]bool, s.n), acted: make([]bool, s.n)}
	for q := range s.n {
		pr.cards[q] = -1
		if q != P && e.Slot < len(s.hands[q]) {
			c := s.hands[q][e.Slot]
			pr.cards[q] = c
			if q != s.me {
				pr.target[q] = s.posTarget(c)
			}
		}
	}
	return pr
}

// posUnknown：P 的牌 id 可以拿來做 Positional Discard / Misplay：終局、沒被提示、沒被 Chop Move、沒被 finesse，
// 而且 P 有該棄的牌（不是鎖手）。
func (s *state) posUnknown(P, id int) bool {
	n := s.notes[id]
	return s.knows(lvEndGame) && s.endGame() && !n.clued && !n.cm && !n.finessed && s.posExpected(P) >= 0
}

// posDiscardLike：P 棄掉牌 id 會被當成 Positional Discard（不看牌面是不是 critical）：posUnknown，
// 而且不是他該棄的牌、也不是提示本身就確定沒用的牌。
func (s *state) posDiscardLike(P, id int) bool {
	return s.posUnknown(P, id) && id != s.posExpected(P) && s.notes[id].explicit&s.useful() != 0
}

// posMisplayLike：P 打錯牌 id 會被當成 Positional Misplay（不看牌面是不是 critical）：posUnknown，而且提示 0 個
// （有提示時直接提示那張就好，原文的例子也是 0 個）、打錯後還不到三個閃電、沒有提示要他盲打、也不是他的
// finesse position（大家都看得到的最左邊那張沒被提示的牌）——打錯沒被提示的牌多半只是以為自己被 finesse
// （自我對局 5 人局：誤認成 Double Positional Misplay 又害兩個人盲打，一錯就是三個閃電）。
func (s *state) posMisplayLike(P, id int) bool {
	return s.posUnknown(P, id) && s.hints == 0 && s.strikes < game.MaxStrikes-1 && !s.mayBlindPlay(P, id) &&
		slices.ContainsFunc(s.hands[P][:max(slices.Index(s.hands[P], id), 0)], func(c int) bool { n := s.notes[c]; return !n.clued && !n.cm })
}

// mayBlindPlay：有提示的解讀（或我推得的計畫）要 P 盲打——他打錯沒被提示的牌可能只是 finesse 沒接好，不是 Positional Misplay。
func (s *state) mayBlindPlay(P, id int) bool {
	if s.mayBeFinessed(id) {
		return true
	}
	fin := func(d dep) bool { return d.player == P && d.finesse }
	for _, rec := range s.recs {
		for _, c := range append(slices.Clone(rec.cands), rec.fallback...) {
			if slices.ContainsFunc(c.deps, fin) {
				return true
			}
		}
	}
	return false
}

// posTarget：牌 c（我看得到）是 Positional Discard 可以叫打的牌：現在就能打，持有者自己還不知道
// （不是大家都知道能打的已提示牌，也不是被 finesse、本來就會盲打的牌）。
func (s *state) posTarget(c int) bool {
	t, ok := s.see(s.me, c)
	n := s.notes[c]
	return ok && s.playableNow(t) && !n.finessed && !(n.clued && (s.screamPlayable(c) || s.publicKnown(c)))
}

// posSafeTarget：給 Positional Discard 時，被叫打的那張（最後一位的牌 c）別處沒有已被拿到的同一張——
// 那張先打出，這張就成了重複，等到他回應時已經打不出去。只用在給的人挑位置：回應的人看不到自己的牌，
// 判斷「之後還有沒有人」時照 posTarget（否則他會以為自己可能有同一張而把後面的人排除，搶著回應）。
func (s *state) posSafeTarget(c int) bool {
	t, _ := s.see(s.me, c)
	return !s.gottenElsewhere(s.me, t, c) && !s.maybeMine(t)
}

// posStep：有人在 Positional Discard / Misplay 之後行動（e，抽牌不算）：記下他有沒有回應；回應夠了、或輪回棄牌的人就結束。
func (s *state) posStep(e game.Event) {
	pr := s.pos
	if pr == nil || e.Type == game.EvDraw {
		return
	}
	if e.Player == pr.from {
		s.pos = nil
		return
	}
	if (e.Type == game.EvPlay || e.Type == game.EvMisplay) && e.Card.ID == pr.cards[e.Player] {
		pr.done++
	}
	pr.acted[e.Player] = true
	if pr.done >= pr.plays {
		s.pos = nil
	}
}

// posResponse：e 是照 Positional Discard / Misplay 的盲打（別的提示不能拿它當證據）。
func (s *state) posResponse(e game.Event) bool {
	pr := s.pos
	return pr != nil && (e.Type == game.EvPlay || e.Type == game.EvMisplay) && e.Player != pr.from && e.Card.ID == pr.cards[e.Player]
}

// posWillRespond：p（不是我）輪到時會回應 Positional Discard / Misplay——那個位置上他有能打的牌，
// 而他之後到棄牌的人之前，有能打的牌的人不夠還需要的張數。
func (s *state) posWillRespond(p int) bool {
	pr := s.pos
	if pr == nil || p == s.me || p == pr.from || pr.acted[p] || !pr.target[p] {
		return false
	}
	later := 0
	for k := 1; (p+k)%s.n != pr.from; k++ {
		if pr.target[(p+k)%s.n] {
			later++
		}
	}
	return later < pr.plays-pr.done
}

// stepPositional（Level 8）：回應 Positional Discard / Misplay。還需要 k 張盲打，而我之後（到棄牌的人之前）那個位置上
// 有能打的牌的人不到 k 位，我就是「最後幾位」之一（在我之前該打的人沒打，表示他看到之後有人有；我看不到自己的牌，
// 他看得到）：盲打那個位置的牌。我那張不可能能打時不打。
func (s *state) stepPositional(t *turn) (game.Action, string, bool) {
	pr := s.pos
	if pr == nil || pr.from == s.me || pr.acted[s.me] {
		return game.Action{}, "", false
	}
	c := pr.cards[s.me]
	if c < 0 || s.holder(c) != s.me {
		return game.Action{}, "", false
	}
	later := 0
	for k := 1; (s.me+k)%s.n != pr.from; k++ {
		if pr.target[(s.me+k)%s.n] {
			later++
		}
	}
	if later >= pr.plays-pr.done || !slices.ContainsFunc(s.myPoss(c).list(), s.playableNow) {
		return game.Action{}, "", false
	}
	why := "回應 Positional Discard：盲打同一個位置的牌"
	if pr.plays == 2 {
		why = "回應 Double Positional Misplay：盲打同一個位置的牌"
	}
	return game.Play(slices.Index(s.hands[s.me], c)), why, true
}

// seesEvery（Level 8 Positional Discard 的前提）：我看得到每一張還需要的牌——每個還打得出去、還沒打出的牌面，
// 別人手上都有一張（或我手上有確定是它的已提示牌）。這時我其他的牌一定是垃圾或重複，棄哪一張都一樣。
func (s *state) seesEvery() bool {
	for _, t := range s.useful().list() {
		if s.discarded[t] >= t.copies() {
			continue // 已經拿不到（之後的牌面 dead，不在 useful 裡）
		}
		found := false
		for p, h := range s.hands {
			for _, id := range h {
				if p == s.me {
					u, ok := s.myPoss(id).single()
					found = found || ok && u == t && s.gotten(id)
				} else if c, ok := s.see(s.me, id); ok && c == t {
					found = true
				}
			}
		}
		if !found {
			return false
		}
	}
	return true
}

// positionalTargets：我棄掉（或打錯）slot 位置的牌時，之後那個位置上有能打的牌（持有者還不知道）的玩家，依行動順序。
func (s *state) positionalTargets(slot int) []int {
	var out []int
	for k := 1; k < s.n; k++ {
		q := (s.me + k) % s.n
		if slot < len(s.hands[q]) && s.posTarget(s.hands[q][slot]) {
			out = append(out, q)
		}
	}
	return out
}

// positionalMove（Level 8 Positional Discard / Misplay 的給法）：終局、我看得到每一張還需要的牌（seesEvery）、
// 這一手本來就要棄牌時，改棄（或打錯）一個能叫人盲打的位置：
//   - Positional Discard：不是我該棄的牌（已知垃圾或 chop）的位置，之後有人那個位置上有能打的牌。
//   - Positional Misplay：只有我該棄的那張的位置有（棄它不會被當成 Positional Discard），而且提示 0 個
//     （有提示時棄牌換回的提示還能直接提示那張）、還能承受閃電、那張不可能能打。
//
// 每一種都要我那張牌不可能是 critical（看得到每一張還需要的牌就不會是）。沒有則 ok = false。
func (s *state) positionalMove(v game.PlayerView) (a game.Action, why string, ok bool) {
	// 前一個 Positional Discard / Misplay 還在等回應時不再給（只記得最新的一個，會打亂它）
	if !s.knows(lvEndGame) || s.pos != nil || !s.endGame() || !s.seesEvery() {
		return game.Action{}, "", false
	}
	exp := s.posExpected(s.me)
	if exp < 0 {
		return game.Action{}, "", false
	}
	// 被叫打的是那個位置上有能打的牌的最後一位
	aimed := func(slot int) bool {
		ts := s.positionalTargets(slot)
		return len(ts) > 0 && s.posSafeTarget(s.hands[ts[len(ts)-1]][slot])
	}
	for slot, id := range s.hands[s.me] {
		if !s.posDiscardLike(s.me, id) || !aimed(slot) {
			continue
		}
		if v.CanDiscard() {
			return game.Discard(slot), "Positional Discard：我看得到每一張還需要的牌，棄這個位置叫人盲打同一個位置的牌", true
		}
	}
	slot := slices.Index(s.hands[s.me], exp)
	if !positionalMisplay || !s.posMisplayLike(s.me, exp) || !aimed(slot) ||
		slices.ContainsFunc(s.myPoss(exp).list(), s.playableNow) {
		return game.Action{}, "", false
	}
	return game.Play(slot), "Positional Misplay：棄 chop 不會被當成 Positional Discard，故意打錯它叫人盲打同一個位置的牌", true
}

// positionalMisplay：給 Positional Misplay（要花一個閃電）。
var positionalMisplay = true

// positionalWhy：這一手的理由是 Positional Discard / Misplay（故意棄 / 打錯那個位置）。
func positionalWhy(why string) bool { return strings.HasPrefix(why, "Positional") }

// distributionCands（Level 8 The Distribution Clue）：終局中，照一般解讀沒有任何可能的提示，焦點每種有用的可能
// 都和別人手上已被拿到的牌重複（Good Touch 排除了它們）——是刻意違反 Good Touch，把那位隊友手上的牌分一張給
// 接收者打（他手上要打的牌太多、來不及在遊戲結束前打完，Team Distribution Principle）。
// 照 hanabi-bot：焦點只要有一種有用、不重複的可能就不是。回傳那些重複的牌面當解讀；不是時回傳 nil。
func (s *state) distributionCands(giver, R int, it *interp) []cand {
	if !s.knows(lvEndGame) || !s.endGame() || len(it.newCards) == 0 || it.tempo {
		return nil
	}
	// 重複的那張要在接收者以外的人手上（接收者自己的已提示牌和焦點重複是違反 Good Touch，不是分擔），
	// 大家都看得出它是 t（接收者看得到，或共同筆記確定），在給提示者手上時要他也知道（giverMayKnow）
	dup := func(t ident) bool {
		for q, h := range s.hands {
			if q == R {
				continue
			}
			for _, id := range h {
				if !s.live(id) || !s.knowsFinessed(giver, id) {
					continue
				}
				c, ok := s.see(R, id)
				if u, one := s.possOf(id).single(); !ok && one {
					c, ok = u, true
				}
				if ok && c == t && (q != giver || s.giverMayKnow(R, id, t)) {
					return true
				}
			}
		}
		return false
	}
	var out []cand
	for _, t := range (it.explicit[it.focus] & s.useful()).list() {
		if s.remainingFor(R, t) <= 0 {
			continue
		}
		if !dup(t) {
			return nil
		}
		out = append(out, cand{id: t})
	}
	return out
}

// distributionWorth（Level 8 Team Distribution Principle）：給接收者 R 打 t 的 Distribution Clue 值得給——
// 他的每種解讀都能立刻打，而手上有已被拿到的 t 的隊友（不是我）要打的牌比他剩下的回合多。
func (s *state) distributionWorth(R int, it interp, t ident) bool {
	if !it.distribution || slices.ContainsFunc(it.cands, func(c cand) bool { return !s.playableNow(c.id) }) {
		return false
	}
	for k := 1; k < s.n; k++ {
		q := (s.me + k) % s.n
		if q == R || !slices.ContainsFunc(s.hands[q], func(id int) bool { c, _ := s.see(s.me, id); return s.live(id) && c == t }) {
			continue
		}
		need := 0
		for _, id := range s.hands[q] {
			if c, _ := s.see(s.me, id); s.live(id) && !s.trash(c) {
				need++
			}
		}
		if need > s.turnsLeft(k, s.deck) {
			return true
		}
	}
	return false
}

// distributedAway：牌面 t 已經用 Distribution Clue 分給 p 以外的人打（他手上有 Distribution Clue 的焦點、筆記確定是 t），
// p 手上的那張就留到最後再打（Team Distribution Principle：讓那人分擔）。
func (s *state) distributedAway(p int, t ident) bool {
	for q, h := range s.hands {
		if q == p {
			continue
		}
		for _, id := range h {
			if u, ok := s.notes[id].poss.single(); ok && u == t && s.notes[id].dist {
				return true
			}
		}
	}
	return false
}
