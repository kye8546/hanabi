// Level 3（docs/level-3.mdx）：Playing Multiple 1's 多出來的 1、Fix Clue（解讀與給法）、
// Information Lock on In-Between Cards、Sarcastic Discard。1 的出牌順序在 playorder.go。

package hgroup

import (
	"slices"

	"hanabi/game"
)

// markExcessOnes（Level 3 Playing Multiple 1's）：手上被提示的 1 比它們可能的牌面種類還多時，
// 依 Good Touch 多出來的那幾張（照出牌順序排在最後的）永遠不會打出，是已知沒用的牌。
func (s *state) markExcessOnes() {
	if !s.knows(lvMultipleOnes) {
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

// findFix（Level 3 The Fix Clue）：從接收者 R 的角度，這個提示是否在修正違反 Good Touch 的牌。
//   - 補資訊（fill in）：點到 R 手上已被拿到、原本被當成有用的牌，提示後它只可能是沒用的牌（垃圾，或別處已被拿到的重複），
//     或和原本的筆記矛盾（Breaking Information Lock）。
//   - 不給新資訊（Stop Clue）：沒點到新牌、點到的牌也沒多任何資訊，卻點到 R 下一手要打的牌：那張是壞的。
//     （提示滿 8 / 鎖手時重新點 5 是 5 Stall，不算。）
//
// 「A clue cannot be a Play Clue and a Fix Clue at the same time」：是 fix clue 就沒有 play / save 的解讀。
func (s *state) findFix(giver, R int, h game.Hint, touched []int, it *interp) {
	if !s.knows(lvFixClue) {
		return
	}
	good := func(set idSet, y int) idSet {
		var g idSet
		for _, i := range set.list() {
			if !s.trash(i) && !s.gottenAs(R, giver, i, y) {
				g |= setOf(i)
			}
		}
		return g
	}
	mark := func(y int, bad bool, poss idSet) {
		it.fix = true
		if it.fixPoss == nil {
			it.fixPoss = map[int]idSet{}
		}
		it.fixPoss[y] = poss
		if bad {
			it.fixed = append(it.fixed, y)
		}
	}
	for _, y := range touched {
		if !s.live(y) {
			continue
		}
		before := s.notes[y].poss
		if good(before, y) == 0 {
			continue // 本來就知道沒用
		}
		ex := it.explicit[y]
		switch after := before & ex; {
		case after == 0:
			// 筆記被證實是錯的：只剩提示本身的資訊（依 Good Touch 仍先假設是有用的牌）
			if g := good(ex, y); g != 0 {
				mark(y, false, g)
			} else {
				mark(y, true, ex)
			}
		case good(after, y) == 0:
			mark(y, true, after)
		}
	}
	// 補資訊後 R 手上兩張以上已被拿到的牌確定是同一張：揭露了重複（例如三張被提示的 1 被紅色補成全是 r1），
	// 只留一張，其餘是壞的
	same := map[ident][]int{}
	for _, y := range touched {
		if _, done := it.fixPoss[y]; done || !s.live(y) {
			continue
		}
		if t, ok := (s.notes[y].poss & it.explicit[y]).single(); ok {
			same[t] = append(same[t], y)
		}
	}
	for _, ys := range same {
		for _, y := range ys[1:] {
			mark(y, true, s.notes[y].poss&it.explicit[y])
		}
	}
	if it.fix || len(it.newCards) > 0 || s.fiveStall(giver, h) && s.mustStall(giver) {
		return
	}
	for _, y := range touched {
		if it.explicit[y] != s.notes[y].explicit {
			return
		}
	}
	if y := s.nextPlay(R); y >= 0 && slices.Contains(touched, y) {
		// 壞掉的牌：已經沒用，或可能和別處被拿到的牌重複。它根本不可能是壞牌時（例如確定是還沒打出的 5），
		// Fix Clue 沒東西可修，就不是 Stop Clue（多半只是拖延）
		var dup idSet
		for _, i := range s.notes[y].poss.list() {
			if s.trash(i) || s.gottenAs(R, giver, i, y) || s.mayDuplicate(R, y, i) {
				dup |= setOf(i)
			}
		}
		if dup != 0 {
			mark(y, true, dup)
		}
	}
}

// mayDuplicate：（R 的角度）R 的牌 y 若是 i，可能和另一張被拿到的牌重複：R 看得到那張就是 i，
// 或那張是 R 自己看不到的牌、可能是 i，而 i 還剩至少兩張（例如兩張被提示的 1）。
func (s *state) mayDuplicate(R, y int, i ident) bool {
	for _, h := range s.hands {
		for _, z := range h {
			if z == y || !s.live(z) {
				continue
			}
			if t, ok := s.see(R, z); ok {
				if t == i {
					return true
				}
			} else if s.holder(z) == R && s.notes[z].poss.has(i) && s.remainingFor(R, i) >= 2 {
				return true
			}
		}
	}
	return false
}

// applyFix：Fix Clue 只修正違反 Good Touch 的牌。被修正的牌寫下新筆記（確定沒用的標成 bad，會先棄掉）；
// 一起點到的新牌依 Good Touch 是有用的牌，但不代表能打（不是 play clue）。
func (s *state) applyFix(R, giver int, it interp) {
	for y, p := range it.fixPoss {
		n := s.notes[y]
		if p != 0 {
			n.poss = p
		}
		n.willPlay = false
		n.bad = slices.Contains(it.fixed, y)
	}
	for _, id := range it.newCards {
		s.notes[id].poss = nonFocusPoss(s.notes[id].poss, s.freshUseful(R, giver, id), 0)
	}
}

// evalFix 評估一個會被當成 Fix Clue 的提示：修正了幾張壞掉的牌；把好牌當成壞牌、或讓人寫下錯誤筆記就是誤導。
func (s *state) evalFix(it interp, ev clueEval, h game.Hint, dist int) clueEval {
	ev.kind = kindFix
	broken := s.brokenCards()
	for y, p := range it.fixPoss {
		t := identOfCard(s.cards[y])
		now, isBroken := broken[y]
		switch {
		case slices.Contains(it.fixed, y) && !s.badCard(y):
			ev.misleads, ev.costly = true, true // 好牌被當成沒用的牌，會被棄掉
		case p != 0 && !p.has(t):
			ev.misleads = true
		case isBroken:
			ev.fixes++
			if now {
				ev.fixesNow++
			}
		}
		if p != 0 && !slices.Contains(it.fixed, y) && !s.playableNow(t) &&
			!slices.ContainsFunc(p.list(), func(i ident) bool { return !s.playableNow(i) }) {
			ev.misleads, ev.danger = true, true
		}
	}
	for _, id := range it.newCards {
		p := nonFocusPoss(s.notes[id].poss&it.explicit[id], s.freshUseful(it.receiver, s.me, id), 0)
		c := identOfCard(s.cards[id])
		switch {
		case !p.has(c) && p != 0 && !slices.ContainsFunc(p.list(), func(i ident) bool { return !s.playableNow(i) }):
			ev.misleads, ev.danger = true, true // 他會以為能打而立刻打錯
		case !p.has(c) || s.trash(c) || s.gottenAs(s.me, s.me, c, id) || s.maybeMine(c):
			// 一起點到垃圾或重複的牌：違反 Good Touch（筆記會是錯的，之後再修），但比眼前的閃電好（Misplay Cost Principle）
			ev.bad++
		default:
			ev.newGood++
		}
	}
	ev.understood = !ev.misleads
	ev.score = scoreFixNow*ev.fixesNow + scoreFix*ev.fixes + scoreNewGood*ev.newGood - scoreBadTouch*ev.bad - dist
	if h.Kind == game.HintColor {
		ev.score += scoreColor
	}
	return ev
}

// certainMine：我手上一定有的牌面（張數）。我的 k 張已被拿到的牌合起來只可能是 k 種牌面、而每種都只剩 1 張時，
// 這 k 種一定都在我手上（例如兩張被「5」點到、都是「g5 或 w5」的牌，就是一張 g5、一張 w5）。
func (s *state) certainMine() [numIdents]int {
	var out [numIdents]int
	var cards []int
	for _, id := range s.hands[s.me] {
		if s.gotten(id) {
			cards = append(cards, id)
		}
	}
	for mask := 1; mask < 1<<len(cards); mask++ {
		var u idSet
		k := 0
		for i, id := range cards {
			if mask&(1<<i) != 0 {
				u |= s.myPoss(id)
				k++
			}
		}
		ids := u.list()
		if len(ids) != k || slices.ContainsFunc(ids, func(i ident) bool { return s.trash(i) || i.copies()-s.discarded[i] != 1 }) {
			continue
		}
		for _, i := range ids {
			out[i] = 1
		}
	}
	return out
}

// holderView：y 的持有者自己推得出的可能——共同筆記再扣掉他看得到所有剩下張數的牌面
// （別人手上我看得到的牌，加上我手上一定有的牌面，他都看得到）。
func (s *state) holderView(y int, mine [numIdents]int) idSet {
	q := s.holder(y)
	p := s.notes[y].poss
	var seen [numIdents]int
	for r, h := range s.hands {
		if r == q {
			continue
		}
		if r == s.me {
			for i, c := range mine {
				seen[i] += c
			}
			continue
		}
		for _, id := range h {
			if c, ok := s.card(id); ok {
				seen[identOfCard(c)]++
			}
		}
	}
	for _, i := range p.list() {
		if !s.trash(i) && seen[i] >= i.copies()-s.discarded[i] {
			p &^= setOf(i)
		}
	}
	return p
}

// brokenCards 找出隊友手上違反 Good Touch、需要 Fix Clue 的牌（值為 true 表示他會把它當成能打的牌而打錯）：
// 他以為能打但其實不能打、其實是垃圾、筆記和實際牌面矛盾，或和別處被拿到的牌重複
// （兩張的持有者都不知道時；有人知道的話他會 Sarcastic Discard）。
func (s *state) brokenCards() map[int]bool {
	out := map[int]bool{}
	knows := func(z int, t ident) bool {
		if s.holder(z) == s.me {
			return s.myPoss(z) == setOf(t)
		}
		return s.notes[z].poss == setOf(t)
	}
	mine := s.certainMine()
	for q, h := range s.hands {
		if q == s.me {
			continue
		}
		for _, y := range h {
			if !s.live(y) {
				continue
			}
			t := identOfCard(s.cards[y])
			p := s.notes[y].poss
			// 他看得到的牌能排除掉的可能，他自己就知道（例如共同筆記只剩 w5，但他看得到我手上的 w5）：
			// 推得出是垃圾就會自己棄掉，不必修；剩下的可能才決定他會不會打錯
			hv := s.holderView(y, mine)
			if hv == 0 || hv&s.useful() == 0 && s.trash(t) {
				continue
			}
			wrongPlay := !s.notes[y].bad && !s.playableNow(t) && !slices.ContainsFunc(hv.list(), func(i ident) bool { return !s.playableNow(i) })
			dup := false
			if z := s.dupOf(y); z >= 0 && !knows(y, t) && !knows(z, t) {
				dup = true
				if s.holderKnowsPlayable(y) && (s.holder(z) == s.me && s.knownPlayable(z) || s.holderKnowsPlayable(z)) {
					wrongPlay = true // 兩張都會被打，後打的那張會打錯
				}
			}
			if wrongPlay || dup || s.trash(t) || !p.has(t) {
				out[y] = wrongPlay
			}
		}
	}
	return out
}

// badCard：（我看得到的）這張被拿到的牌是沒用的：垃圾，或和別處被拿到的牌重複。
func (s *state) badCard(y int) bool {
	t := identOfCard(s.cards[y])
	if s.trash(t) {
		return true
	}
	return s.dupOf(y) >= 0
}

// dupOf 回傳和 y 重複的另一張被拿到的牌（我看得到它，或是我手上確定是那張的牌）；沒有則回傳 -1。
func (s *state) dupOf(y int) int {
	t := identOfCard(s.cards[y])
	for q, h := range s.hands {
		for _, z := range h {
			if z == y || !s.live(z) {
				continue
			}
			if c, ok := s.see(s.me, z); ok && c == t || q == s.me && s.myPoss(z) == setOf(t) {
				return z
			}
		}
	}
	return -1
}

// nextMisplaysAfter：我打出 slot 這張（每種可能都模擬）後，下一位下一手要打的牌是壞掉的牌（brokenCards）、
// 實際卻不能打——例如 save clue 違反 Good Touch 點到的重複 g2，在 g3 打出後被他當成 g4。回傳這些牌。
// 我打之前他本來就會打錯的不算（那是 decide 裡 fixesNow 的 Fix Clue 要處理的，和我打不打無關）。
func (s *state) nextMisplaysAfter(slot int) []int {
	next := (s.me + 1) % s.n
	broken := s.brokenCards()
	if s.nextMisplay(next, broken) >= 0 {
		return nil
	}
	var risky []int
	for _, t := range s.myPoss(s.hands[s.me][slot]).list() {
		old := s.stacks[t.color()]
		s.stacks[t.color()] = t.rank()
		if y := s.nextMisplay(next, broken); y >= 0 && !slices.Contains(risky, y) {
			risky = append(risky, y)
		}
		s.stacks[t.color()] = old
	}
	return risky
}

// nextMisplay：p 依他自己推得出的筆記（看得到的張數、Good Touch，已打出的牌面也排除）下一手要打的牌，
// 若是壞掉、實際不能打的牌就回傳它，否則回傳 -1。
func (s *state) nextMisplay(p int, broken map[int]bool) int {
	restore := s.saveSim()
	s.overlay = map[int]idSet{}
	for _, y := range s.hands[p] {
		q := s.deduce(p, y)
		if u := q & s.useful(); s.gotten(y) && u != 0 {
			q = u
		}
		s.overlay[y] = q
	}
	y := s.nextPlay(p)
	restore()
	if _, ok := broken[y]; ok && !s.playableNow(identOfCard(s.cards[y])) {
		return y
	}
	return -1
}

// lockMine（Level 3 Information Lock on In-Between Cards）：我看得到焦點是 t、它現在還不能打，中間需要的某張 m
// 不在計畫的 prompt / finesse 裡、別人手上也沒有已被拿到的 m——那就是我手上那張可能是 m 的已提示牌
// （例如 Cathy 的 r4 被提示，Bob 手上的 2 就鎖定成 r2）。只有一張候選時才鎖定。
func (s *state) lockMine(rec *clueRec, it interp) {
	if !it.hasTruth || !it.planOK || it.saveOK || !slices.ContainsFunc(it.cands, func(c cand) bool { return c.id == it.truth }) {
		return
	}
	t := it.truth
	var chain idSet
	for r := s.stacks[t.color()] + 1; r <= t.rank(); r++ {
		chain |= setOf(identOf(t.color(), r))
	}
	for r := s.stacks[t.color()] + 1; r < t.rank(); r++ {
		m := identOf(t.color(), r)
		if slices.ContainsFunc(it.plan, func(d dep) bool { return d.id == m }) {
			continue
		}
		elsewhere := false
		for p, h := range s.hands {
			for _, id := range h {
				if p == s.me || !s.live(id) {
					continue
				}
				if c, ok := s.see(s.me, id); ok && c == m || s.notes[id].poss == setOf(m) {
					elsewhere = true
				}
			}
		}
		// 給提示者看不到自己的牌：他手上可能是 m 的已提示牌，他也許以為就是它，連接牌就不一定是我的
		for _, id := range s.hands[rec.giver] {
			if rec.giver != s.me && s.live(id) && s.notes[id].poss.has(m) {
				elsewhere = true
			}
		}
		if elsewhere {
			continue
		}
		var mine []int
		for _, id := range s.hands[s.me] {
			if s.live(id) && s.myPoss(id).has(m) {
				mine = append(mine, id)
			}
		}
		// 我那張也可能是鏈上的其他牌（例如 g2 或 g4，焦點是 g5）時，不知道它補的是哪一張
		if len(mine) == 1 && s.myPoss(mine[0])&chain == setOf(m) {
			s.notes[mine[0]].poss = setOf(m)
			rec.changed = append(rec.changed, mine[0])
		}
	}
}

// sarcastic（Level 3 The Sarcastic Discard）：有人棄掉一張被拿到、而且不是垃圾的牌 t，表示別人手上已被拿到的牌裡有 t
// （他發現自己那張是重複的）。看得到的人知道是誰；都看不到就是我。那人手上只有一張可能是 t 的牌時，它就是 t
// （有好幾張時不保證是哪一張）。鎖手被迫棄牌不算。
func (s *state) sarcastic(e game.Event) {
	t := identOfCard(e.Card)
	if !s.knows(lvSarcastic) || !s.live(e.Card.ID) || s.trash(t) || s.chop(e.Player) < 0 {
		return
	}
	holder := -1
	for p, h := range s.hands {
		if p == e.Player || p == s.me {
			continue
		}
		for _, id := range h {
			if c, ok := s.card(id); ok && s.live(id) && identOfCard(c) == t {
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

// sarcasticSlot（Level 3 The Sarcastic Discard）：我手上被拿到、確定是 t 的牌，而別人手上已被拿到的牌實際就是 t：
// 我是先發現重複的人，棄掉它（他會知道自己有 t）。提示滿 8 不能棄牌；鎖手時棄牌會被當成被迫的，不用。
func (s *state) sarcasticSlot(v game.PlayerView) int {
	if v.Hints >= game.MaxHints || s.chop(s.me) < 0 {
		return -1
	}
	for slot, x := range s.hands[s.me] {
		t, ok := s.myPoss(x).single()
		if !s.live(x) || !ok || s.trash(t) {
			continue
		}
		for q, h := range s.hands {
			if q == s.me {
				continue
			}
			for _, y := range h {
				// 他已經知道就不必（他會自己打出，我這張之後就是已知沒用的牌）；他的筆記沒有 t 時，棄牌也告訴不了他
				if p := s.notes[y].poss; s.live(y) && identOfCard(s.cards[y]) == t && p.has(t) && p != setOf(t) &&
					(!s.playableNow(t) || s.sdCands(q, t) == 1) {
					return slot
				}
			}
		}
	}
	return -1
}

// sdCands：Sarcastic Discard t 之後，q 手上有幾張可能是 t 的被拿到的牌（共同筆記）。
func (s *state) sdCands(q int, t ident) int {
	n := 0
	for _, id := range s.hands[q] {
		if s.live(id) && s.notes[id].poss.has(t) {
			n++
		}
	}
	return n
}
