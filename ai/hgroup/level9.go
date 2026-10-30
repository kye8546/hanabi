// Level 9（docs/level-9.mdx）Stalling：Stall Table 的拖延情況與嚴重度、Double Discard Avoidance。

package hgroup

import (
	"slices"

	"hanabi/game"
)

// ddaRec：Level 9 Double Discard Avoidance。上一位剛棄掉（或打錯）牌面 id，它只剩最後一張，
// 而 target（下一位）的 chop 依提示本身可能就是它——他這一手棄 chop 可能就棄掉最後一張。
type ddaRec struct {
	target int
	id     ident
}

// detectDDA：p 剛棄掉（或打錯）牌面 id 之後，下一位是否處在 Double Discard Situation（原文的定義：上一位棄牌或打錯，
// 而這一位棄牌可能棄掉那張牌的最後一張）。「可能」照大家都知道的資訊：他 chop 上的牌依提示本身可能是它
// （hanabi-bot 同樣用共同的可能牌面）。只剩一張的牌本來就有的（例如 5）不算：這次棄牌沒有讓它變成最後一張。
// Sarcastic Discard 也不算：棄牌的人表示最後那張在別人已被拿到的牌裡，不會在 chop 上
// （使用者回報 20261030-151814 t27：使用者 Sarcastic Discard 掉 R2，AI-1 以為自己 chop 可能是最後的 R2 而拖延）。
func (s *state) detectDDA(p int, id ident, sarcastic bool) *ddaRec {
	if !s.knows(lvStall) || sarcastic || id.copies() < 2 || !s.critical(id) {
		return nil
	}
	q := (p + 1) % s.n
	if c := s.publicChop(q, nil); c < 0 || !s.notes[c].explicit.has(id) {
		return nil
	}
	return &ddaRec{target: q, id: id}
}

// myDDA：我這一手處在 Double Discard Situation——依我自己的推論，chop 上的牌可能就是那張最後一張
// （我看得到它在別人手上就不是）。這時不棄 chop，改給提示（拖延提示也可以）。
func (s *state) myDDA() bool {
	if s.dda == nil || s.dda.target != s.me {
		return false
	}
	c := s.chop(s.me)
	return c >= 0 && s.myPoss(c).has(s.dda.id)
}

// stallSeverity（Level 9 Stall Table）：giver 這一手的拖延情況嚴重度。4 = 提示滿 8（第一回合除外）、3 = 鎖手、
// 2 = Double Discard Situation 或被 Scream / Shout Discard 叫到、1 = Early Game、0 = 一般情況。
// Double Discard Situation 中，他手上有大家都知道能打 / 沒用的牌時（loaded）不必拖延，不算。
// 只用大家都知道的條件：試過再加上「看得到他還有更好的提示」，各家看到的牌不同（見 stallValid），
// 5 Stall / 5's Chop Move 等讀法不一致，Black 4 人局 1000 局 -0.06。
// 嚴重度越高，能當成拖延的提示越多（5 Stall 從 1 起，Tempo Clue Stall、Fill-In Clue、Hard Burn 從 2 起）。
func (s *state) stallSeverity(giver int) int {
	switch {
	case s.hints >= game.MaxHints && s.moved:
		return 4
	case s.chop(giver) < 0:
		return 3
	case s.screamed(giver) || s.dda != nil && s.dda.target == giver && !s.loaded(giver):
		return 2
	case !s.midGame:
		return 1
	}
	return 0
}

// stallValid（Level 9 Stall Table 的優先順序）：giver 在拖延情況下給的提示，大家才把它當成拖延提示。
// 他手上有大家都知道能打的牌、或（還能棄牌時）大家都知道沒用的牌，就不必拖延（5 Stalls are a Last Resort，
// hanabi-bot 的 giver_loaded）。被 Scream / Shout 叫到的人（Level 7）一定要給提示，不看這個。
// 原文另外說「看得到他還有更好的提示可給，大家就知道他不是在拖延」，但每個人看得到的牌不同（看不到自己的手牌，
// 被 finesse 的人常常就是那個「更好的提示」的對象），試過照做：各家讀法不一致，3000 局 3–5 人局略差（Rainbow -0.02–0.05），
// 不採用——拖延情況下大家一律照拖延解讀。
func (s *state) stallValid(giver int) bool {
	return s.screamed(giver) || !s.loaded(giver)
}

// loaded：p 手上有大家都知道能打的牌，或（還能棄牌時）大家都知道沒用的牌——他有事可做，不必拖延。
func (s *state) loaded(p int) bool {
	return s.publicPlayable(p, -1) || s.hints < game.MaxHints && s.hasPublicTrash(p, -1)
}

// fiveStallTarget（Level 9 Early Game）：p 手上可以被 5 Stall 的牌（沒被提示、沒被 Chop Move、不在 chop 上的 5，
// 不含 used）中最靠近 chop 的那張；沒有則回傳 -1。
func (s *state) fiveStallTarget(p int, used []int) int {
	best, bestDist := -1, 0
	for _, id := range s.hands[p] {
		if c := s.cards[id]; c.Rank != game.MaxRank || s.gotten(id) || s.notes[id].cm || id == s.chop(p) || slices.Contains(used, id) {
			continue
		}
		if d := s.chopDistance(p, id); best < 0 || d < bestDist {
			best, bestDist = id, d
		}
	}
	return best
}

// chopDistance：p 的牌 id 右邊還有幾張沒被提示、沒被 Chop Move 的牌（chop 左邊一張是 1；原文 5 Stalls Closest to Chop 的「one-away from chop」）。
func (s *state) chopDistance(p, id int) int {
	h := s.hands[p]
	d := 0
	for _, o := range h[slices.Index(h, id)+1:] {
		if !s.gotten(o) && !s.notes[o].cm {
			d++
		}
	}
	return d
}

// stallDistance（Level 9 5 Stalls Closest to Chop）：5 Stall e 點到的新 5 中最靠近 chop 的那張離 chop 多遠。
func (s *state) stallDistance(e clueEval) int {
	d := -1
	for _, id := range e.touched {
		if !s.gotten(id) && !s.notes[id].cm {
			if x := s.chopDistance(e.target, id); d < 0 || x < d {
				d = x
			}
		}
	}
	return d
}

// anySaveStall（Level 9 Stall Table 的 Locked Hand Save / 8 Clue Save）：giver 在拖延情況下給的這個提示只是救下焦點
// （任何有用的牌，不只 critical / 2 / 5），不是 play clue——只有直接能打的牌面可能讓人打出，不會有人照它盲打。
//   - Locked Hand Save：giver 鎖手（嚴重度 3），點到接收者 chop 上的新牌，而且提示後接收者沒有鎖手。
//   - 8 Clue Save：提示滿 8（嚴重度 4，第一回合除外），點到新牌、沒點到 slot 1。
//
// 給提示者有事可做時不是拖延（stallValid）。hanabi-bot 在鎖手時的嚴重度 3 以上
// 都允許 Locked Hand Save（提示滿 8 時 chop 本來就在 8 Clue Save 的範圍內）。
func (s *state) anySaveStall(giver, R int, touched []int, it *interp) bool {
	if !s.knows(lvStall) || it.fix || it.cmKind != 0 || it.endStall || len(it.newCards) == 0 {
		return false
	}
	switch sev := s.stallSeverity(giver); {
	case sev == 4 && !slices.Contains(touched, s.hands[R][0]):
	case sev >= 3 && it.chopFocus && s.publicChop(R, touched) >= 0:
	default:
		return false
	}
	return s.stallValid(giver)
}

// anxietySlot（Level 9 The Anxiety Play）：鎖手的 p 在提示 0 個時要打的牌——最可能能打的那張（筆記扣掉沒用的牌面後，
// 能打的牌面所佔的比例），一樣時打最左邊的（Leftmost Playable Principle）。都不可能能打時回傳 -1（他只能棄牌）。
// 原文：刻意讓鎖手的人停在 0 個提示，就表示他有一張牌能打。
func (s *state) anxietySlot(p int) int {
	best, bestP := -1, 0.0
	for slot, id := range s.hands[p] {
		n := s.notes[id]
		if n.bad {
			continue
		}
		poss := n.poss
		if p == s.me {
			poss = s.myPoss(id)
		}
		if u := poss & s.useful(); u != 0 {
			poss = u
		}
		k := 0
		for _, i := range poss.list() {
			if s.playableNow(i) {
				k++
			}
		}
		if pr := float64(k) / float64(max(poss.count(), 1)); pr > bestP {
			best, bestP = slot, pr
		}
	}
	return best
}

// leavesLockedAnxious（Level 9 The Anxiety Play）：我做了 a 之後提示是 0 個、下一位鎖手又不知道有牌可打——他就會
// 打出最可能能打的牌（anxietySlot）；我看得到那張其實不能打（或他沒有可打的候選）時回傳 true。a 是提示他、讓他從此鎖手的
// 提示時，他要打哪張不好預測，一律算會打錯。原文：「If the team is out of clues and someone's hand is fully clued, then you
// have to discard to generate a clue for them. Similarly, if there is only 1 clue left, you cannot steal it from them.」
func (s *state) leavesLockedAnxious(a game.Action, hints int) bool {
	if !s.knows(lvStall) || a.Type == game.ActDiscard {
		return false
	}
	if a.Type == game.ActHint {
		hints--
	} else if c, ok := s.myPoss(s.hands[s.me][a.Slot]).single(); ok && s.playableNow(c) {
		if c.rank() == game.MaxRank {
			hints++ // 打出 5 換回一個提示
		}
		// 照我打出之後的煙火判斷他要打的牌（例如我打出的正是他那張的同一張，他那張就不能打了）
		old := s.stacks[c.color()]
		s.stacks[c.color()] = c.rank()
		defer func() { s.stacks[c.color()] = old }()
	}
	next := (s.me + 1) % s.n
	if hints > 0 || next == s.me {
		return false
	}
	if a.Type == game.ActHint && a.Target == next {
		h := a.Hint
		var touched []int
		for _, id := range s.hands[next] {
			if h.Matches(s.cards[id]) {
				touched = append(touched, id)
			}
		}
		if pc := s.publicChop(next, nil); pc >= 0 {
			it := s.interpret(s.me, next, h, touched) // Chop Move 的牌也算保護住了
			saved := append(touched, it.cm...)
			if s.n == 2 && slices.Contains(saved, pc) && s.critical(identOfCard(s.cards[pc])) && !s.keepsChop(next) {
				// 2 人局，這個提示救下他 chop 上的 critical：改棄牌換提示的話，他沒牌可打、只能提示我，多半沒事可做，
				// 下一手就棄掉它（確定少分），寧可讓他鎖手、之後 Anxiety Play 可能打錯（閃電，打掉的牌也許還有另一張）。
				// 3 人以上他常有提示可給（給別人或給我），會去給提示、不棄 chop，照舊棄牌換提示（套用時 3–5 人局
				// 10000 局 -0.05–0.12，加上「他沒有提示可給別人」的條件仍 -0.03–0.05）
				// （使用者回報 20261030-120740 t30：AI-1 棄牌換提示，使用者接著棄掉 chop 上的 Y5）
				return false
			}
			return s.chopAfter(next, saved) < 0
		}
		// 他本來就鎖手：提示點到能打的牌，他多半就知道能打了
		return !slices.ContainsFunc(touched, func(id int) bool { return s.playableNow(identOfCard(s.cards[id])) })
	}
	// 鎖手只看大家都知道的（被提示、被 Chop Move）：被 finesse 的牌他自己不一定知道，各家判斷會不一致
	if s.publicChop(next, nil) >= 0 || s.keepsChop(next) {
		return false
	}
	slot := s.anxietySlot(next)
	return slot < 0 || !s.playableNow(identOfCard(s.cards[s.hands[next][slot]]))
}

// feedSlot：棄牌換提示給鎖手的下一位時要棄的牌：已知垃圾、chop，我也鎖手時是最不可能 critical 的已提示牌。
func (s *state) feedSlot() int {
	if slot := s.trashSlot(false); slot >= 0 {
		return slot
	}
	if c := s.chop(s.me); c >= 0 {
		return slices.Index(s.hands[s.me], c)
	}
	slot, _ := s.lockedDiscard()
	return slot
}
