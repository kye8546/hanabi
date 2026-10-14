// Level 4（docs/level-4.mdx）：Chop Moves——Trash Chop Move、5's Chop Move、Order Chop Move 的解讀與給法。

package hgroup

import (
	"slices"

	"hanabi/game"
)

const (
	cmTrash = 1 + iota // The Trash Chop Move
	cmFive             // The 5's Chop Move
)

// findChopMove（Level 4）：從接收者 R 的角度，這個提示是否是 Chop Move。fresh 是沒被 Chop Move 的新牌。
//   - The Trash Chop Move：點到的牌全是 R 推得出的垃圾（已打出、打不出去，或別處已被拿到 / 張數已用完）。
//     最右邊那張新牌右邊、沒被提示的牌全部 Chop Move（垃圾就在 chop 上時沒有牌可 Chop Move，不算；那是 Level 14 的 Trash Push）。
//   - The 5's Chop Move：不是拖延的情況下（5 Stall 優先），數字 5 點到的最右邊那張新的 5 正好在 chop 左邊一張：
//     Chop Move chop 上那一張（只有一張）。點到 chop 的是 5 Save；離 chop 兩張以上的 5 仍是 play clue。
func (s *state) findChopMove(giver, R int, h game.Hint, touched, fresh []int, it *interp) {
	if !s.knows(lvChopMove) || len(fresh) == 0 {
		return
	}
	// 只用大家都知道的資訊（提示本身、被提示與 Chop Move 的牌）：私下的推論或只有部分人知道的 finesse
	// 會讓各家對「是不是 Chop Move、Chop Move 哪幾張」看法不一
	allTrash := true
	for _, y := range touched {
		// 點到還被當成有用的舊牌是 Fix Clue 的範圍（接收者私下可能早就知道它沒用，各家看法會不一樣）
		if s.live(y) || !s.trashFor(y, it.explicit[y]) {
			allTrash = false
			break
		}
	}
	last := fresh[len(fresh)-1]
	if allTrash {
		var cm []int
		for _, id := range s.hands[R][slices.Index(s.hands[R], last)+1:] {
			if n := s.notes[id]; !n.clued && !n.cm {
				cm = append(cm, id)
			}
		}
		if len(cm) > 0 {
			it.cmKind, it.cm, it.focus = cmTrash, cm, last
		}
		return
	}
	if h.Kind != game.HintRank || h.Rank != game.MaxRank || s.fiveStall(giver, h) {
		return
	}
	var unclued []int // 由右而左
	for i := len(s.hands[R]) - 1; i >= 0; i-- {
		if n := s.notes[s.hands[R][i]]; !n.clued && !n.cm {
			unclued = append(unclued, s.hands[R][i])
		}
	}
	if len(unclued) >= 2 && last == unclued[1] && !slices.Contains(touched, unclued[0]) {
		it.cmKind, it.cm, it.focus = cmFive, []int{unclued[0]}, last
	}
}

// trashFor：牌 y 的可能 p（提示本身的資訊）全是大家都知道的垃圾：已打出、打不出去，或別處有提示本身就確定是那張的牌
// （Trash Chop Move 原文的「already played or accounted for」）。只用共同資訊：要是只有接收者推得出是垃圾
// （例如靠旁觀者自己看不到的牌），旁觀者會把它當成 play clue 而盲打。
func (s *state) trashFor(y int, p idSet) bool {
	if p == 0 {
		return false
	}
	for _, i := range p.list() {
		if !s.trash(i) && !s.knownElsewhere(i, y) {
			return false
		}
	}
	return true
}

// knownElsewhere：除了 y 以外，有提示本身就確定是 i 的已提示牌（大家都知道）。
func (s *state) knownElsewhere(i ident, y int) bool {
	for _, h := range s.hands {
		for _, id := range h {
			if id == y || !s.live(id) {
				continue
			}
			if t, ok := s.notes[id].explicit.single(); ok && t == i {
				return true
			}
		}
	}
	return false
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
		if n, ok := s.note(id); ok && !n.clued {
			n.cm = false
		}
	}
}

// evalChopMove 評估一個會被當成 Chop Move 的提示：被保護的牌有幾張必須保留、幾張白白被保護。
// 5's Chop Move 點到的 5 本身也被救下（記成任何 5）。
func (s *state) evalChopMove(it interp, ev clueEval, h game.Hint, dist int) clueEval {
	R := it.receiver
	ev.kind, ev.cmKind, ev.cm = kindChopMove, it.cmKind, it.cm
	var seen idSet
	useless := func(id int) bool {
		c := identOfCard(s.cards[id])
		u := s.trash(c) || seen.has(c) || s.gottenAs(s.me, s.me, c, id) || s.maybeMine(c)
		seen |= setOf(c)
		return u
	}
	for _, id := range it.cm {
		switch {
		case useless(id):
			ev.cmWaste++
		case s.needsSave(R, id):
			ev.cmSaves++
		default:
			ev.cmUseful++
		}
	}
	if it.cmKind == cmFive {
		ev.misleads, ev.danger, ev.costly = s.misleads(R, it, ev.touched)
		for _, id := range it.newCards {
			if useless(id) {
				ev.bad++
			} else {
				ev.newGood++
			}
		}
	}
	ev.understood = !ev.misleads && ev.cmWaste == 0
	ev.score = scoreCMFirstSave + scoreCMMoreSave*(ev.cmSaves-1) + scoreCMUseful*ev.cmUseful + scoreNewGood*ev.newGood -
		scoreBadTouch*ev.bad - scoreCMWaste*ev.cmWaste - dist
	if h.Kind == game.HintColor {
		ev.score += scoreColor
	}
	return ev
}

// cmSave：e 是能當 save clue 用的 Chop Move——保護了接收者 chop 上必須保留的牌，也沒有白白保護沒用的牌。
// Trash Chop Move 不能用在可以直接用 save clue 救的牌上（直接提示總是多給一些資訊），
// 除非 chop 上連著好幾張必須保留的牌（一次救不完）。
func (s *state) cmSave(evs []clueEval, e clueEval) bool {
	if e.kind != kindChopMove || e.misleads || e.cmWaste > 0 || e.bad > 0 {
		return false
	}
	chop := s.chop(e.target)
	if chop < 0 || !slices.Contains(e.cm, chop) || !s.needsSave(e.target, chop) {
		return false
	}
	// Chop Move 後他的手牌全被提示或 Chop Move（鎖手）、又沒牌可打：他不能棄牌，只能拖延或棄可能有用的牌。
	// 但這次保護的牌（新點到的與被 Chop Move 的）本來就全都必須保留時，鎖手是遲早的事：直接救 chop 後，
	// 下一張 chop 也得再花一個提示救（例如 5's Chop Move 點到的 w5 與 chop 上 critical 的 b4，一個提示救兩張）
	if s.chopAfter(e.target, append(slices.Clone(e.touched), e.cm...)) < 0 && !s.willPlay(e.target) {
		for _, id := range append(slices.Clone(e.touched), e.cm...) {
			if !s.gotten(id) && !s.needsSave(e.target, id) {
				return false
			}
		}
	}
	if e.cmKind == cmTrash && e.cmSaves < 2 {
		_, direct := best(evs, func(d clueEval) bool {
			return d.target == e.target && d.focus == chop && (d.kind == kindPlay || d.kind == kindSave) && !d.misleads
		})
		return !direct
	}
	return true
}

// ocmSkip（Level 4 The Order Chop Move）：p 打出被提示的 1（x）時，照 Level 3 的順序跳過了幾張。
// 只算「不知道是哪張」的 1：x 的牌面已知、x 是之前被 finesse 的、被點到的次數和其他 1 不同、
// 被跳過的 1 已知沒用（playOrder 不含它）時都不算。不是 Order Chop Move 時回傳 0。
func (s *state) ocmSkip(p, x int) int {
	unknownOne := func(id int) bool {
		k := s.knownPoss(p, id)
		return k&^rankSet(1) == 0 && k.count() > 1
	}
	n := s.notes[x]
	if !s.knows(lvChopMove) || !n.clued || n.finessed || n.bad || !unknownOne(x) || s.mayBeFinessed(x) {
		return 0
	}
	skip := 0
	for _, id := range s.playOrder(p) {
		if id == x {
			return skip
		}
		if m := s.notes[id]; m.clued && !m.finessed && m.touches == n.touches && unknownOne(id) {
			skip++
		}
	}
	return 0
}

// mayBeFinessed：某個提示的可能解讀（或我推得的計畫）要 x 被 finesse 盲打。我看不到焦點時不一定確定它被 finesse
// （例如還有直接解讀），但它可能是之前的提示 finesse 的牌，打出它就不是 Order Chop Move。
func (s *state) mayBeFinessed(x int) bool {
	isX := func(d dep) bool { return d.card == x && d.finesse }
	for _, rec := range s.recs {
		if slices.ContainsFunc(rec.plan, isX) {
			return true
		}
		for _, c := range append(slices.Clone(rec.cands), rec.fallback...) {
			if slices.ContainsFunc(c.deps, isX) {
				return true
			}
		}
	}
	return false
}

// orderChopMove：選一張要打的 1，讓 Order Chop Move 保護後面某位 chop 上必須保留的牌（不花提示）。
// 優先保護沒牌可打（快要棄牌）、又比較近的人。回傳要打的位置與被保護的玩家；沒有則回傳 -1, -1。
func (s *state) orderChopMove() (slot, target int) {
	slot, target = -1, -1
	bestScore := 0
	for _, x := range s.playOrder(s.me) {
		k := s.ocmSkip(s.me, x)
		if k <= 0 || k >= s.n {
			continue
		}
		p := (s.me + k) % s.n
		chop := s.chop(p)
		if chop < 0 || !s.needsSave(p, chop) {
			continue
		}
		score := 10*s.saveWeight(chop) - k
		if !s.willPlay(p) {
			score += 100
		}
		if score > bestScore {
			slot, target, bestScore = slices.Index(s.hands[s.me], x), p, score
		}
	}
	return slot, target
}
