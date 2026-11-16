// Level 3（docs/level-3.mdx）：Playing Multiple 1's 多出來的 1、Fix Clue（解讀與給法）、
// Information Lock on In-Between Cards、Sarcastic Discard。1 的出牌順序在 playorder.go。

package hgroup

import (
	"maps"
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
//     （提示滿 8 / 鎖手時重新點 5 是 5 Stall，不算；給提示者鎖手時一律不算——他不能棄牌，只能給提示拖延，
//     重新點已提示的牌照 Tempo Clue / 拖延解讀。使用者回報 20261029-101614 t29：AI-1 鎖手、只剩 1 個提示，
//     重新點 AI-2 的 R2（Tempo Clue）或使用者的 Y5（5 Stall）都不會被誤會，卻因「不給新資訊」被當成誤導而改棄
//     已提示的牌，棄掉了 W5。）
//
// 「A clue cannot be a Play Clue and a Fix Clue at the same time」：是 fix clue 就沒有 play / save 的解讀。
func (s *state) findFix(giver, R int, h game.Hint, touched []int, it *interp) {
	if !s.knows(lvFixClue) || s.notFix {
		return
	}
	// R 沒被點到的牌套上這次的負向資訊：筆記被這個提示推翻的牌不再算「已被拿到的那張」（使用者回報 20261104-122437 t6：
	// 黃色 prompt 時 AI-1 照最左邊把 slot 1 的黃牌記成 Y2，「2」點到 slot 5、沒點 slot 1，以前仍以為 Y2 在 slot 1，
	// 把真正的 Y2 當重複標成壞的，t26 棄掉）
	post := maps.Clone(s.overlay)
	if post == nil {
		post = map[int]idSet{}
	}
	for id, ex := range it.explicit {
		if !slices.Contains(touched, id) {
			post[id] = s.possOf(id) & ex
		}
	}
	// 我是旁觀者時，我手上被可能是 Save Clue 的提示一起點到的牌（`saveLoose`）不一定是好牌：
	// 依 Good Touch 排除過也一樣——提示本身容許它是 i（張數也還有剩），提示揭露 R 的牌是 i 就可能是在修這個重複，不預設自己的牌是好的而讀成 play clue
	mineLoose := func(i ident, y int) bool {
		if s.me == R || s.me == giver || !s.aboutToPlay(y) {
			return false
		}
		return slices.ContainsFunc(s.hands[s.me], func(z int) bool {
			return z != y && s.live(z) && s.saveLoose(z) && s.notes[z].explicit.has(i) && s.remaining(i) > 0
		})
	}
	good := func(set idSet, y int) idSet {
		var g idSet
		for _, i := range set.list() {
			if !s.trash(i) && !s.gottenAs(R, giver, i, y) && !mineLoose(i, y) {
				g |= setOf(i)
			}
		}
		return g
	}
	goodAfter := func(set idSet, y int) idSet {
		defer s.saveSim()()
		s.overlay = post
		return good(set, y)
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
			if g := goodAfter(ex, y); g != 0 {
				mark(y, false, g)
			} else {
				mark(y, true, ex)
			}
		case goodAfter(after, y) == 0:
			mark(y, true, after)
		}
	}
	// 用負向資訊修：提示沒點到新牌，卻讓 R 沒被點到、原本被當成有用的已被拿到的牌被推翻（筆記和提示矛盾），或只剩沒用的牌。
	// 例如 Rainbow 裡被記成 M3、實際是 G3 的牌，任何顏色提示點到它都分不出 G / M，要用不點到它的顏色（點到別張）揭露它不是彩虹色
	// （使用者回報 20261116-120334：t53「3」點到 AI-3 的 G3（綠色已完成）大家記成 M3，t55 AI-2 有提示卻沒修，t60 打錯）。
	// 有新牌時照一般的 play / save clue 讀（負向資訊照樣寫進筆記，只是不當成 Fix Clue）。
	// 只剩同一個牌面的幾張（例如兩張「B3 或 M3」都被排除 M3）交給下面的重複判斷，不在這裡互相當成重複。
	cands := touched
	if len(it.newCards) == 0 {
		for _, y := range s.hands[R] {
			if slices.Contains(touched, y) || !s.live(y) || !s.gotten(y) {
				continue
			}
			before := s.notes[y].poss
			ex := it.explicit[y]
			after := before & ex
			if good(before, y) == 0 || after == before {
				continue
			}
			cands = append(cands, y)
			switch {
			case after == 0:
				if g := goodAfter(ex, y); g != 0 {
					mark(y, false, g)
				} else {
					mark(y, true, ex)
				}
			case good(after, y) == 0:
				mark(y, true, after)
			}
		}
	}
	// 補資訊後 R 手上兩張以上已被拿到的牌確定是同一張：揭露了重複（例如三張被提示的 1 被紅色補成全是 r1），
	// 只留一張，其餘是壞的
	// 筆記和提示矛盾的牌（例如接收者原本推得是「垃圾或焦點的重複」）照提示本身確定的牌面算：給提示者看不到接收者的私下推論，
	// 揭露「兩張都是 b4」就是修正重複的 Fix Clue（使用者回報 20261020-164751 t16：藍色點到 AI-1 的兩張 b4，使用者用「4」揭露重複、
	// 也點到新的 w4；AI-1 早就知道其中一張沒用，就把「4」讀成 w4 的 play clue、Self-Finesse 自己的 w2 而盲打錯）。
	// 保留原本被當成有用的那張，其餘標成壞的
	same := map[ident][]int{}
	for _, y := range cands {
		if _, done := it.fixPoss[y]; done || !s.live(y) {
			continue
		}
		p := s.notes[y].poss & it.explicit[y]
		if p == 0 {
			p = it.explicit[y] & s.useful()
		}
		if t, ok := p.single(); ok {
			same[t] = append(same[t], y)
		}
	}
	for t, ys := range same {
		// 提示前就已經都是 t（例如兩張都被記成 G5）不是這個提示揭露的：再點一次也只標出其中一張，另一張照樣會被當成 t 打錯
		if len(ys) < 2 || !slices.ContainsFunc(ys, func(y int) bool {
			return s.notes[y].poss != setOf(t) || it.explicit[y] != s.notes[y].explicit
		}) {
			continue
		}
		keep := ys[0]
		for _, y := range ys {
			if s.notes[y].poss.has(t) {
				keep = y
				break
			}
		}
		for _, y := range ys {
			if y != keep {
				mark(y, true, setOf(t))
			}
		}
	}
	if it.fix || len(it.newCards) > 0 || s.fiveStall(giver, h) && s.mustStall(giver) || s.chop(giver) < 0 {
		return
	}
	for _, y := range touched {
		if it.explicit[y] != s.notes[y].explicit {
			return
		}
	}
	// Level 6：不給新資訊的 Fix Clue 只在點到的牌都是大家已知道能打的牌時才成立；Focus Shifting 找得到大家還不知道能打的焦點，
	// 就是 Tempo Clue 這個更自然的解釋（原文 Level 3：「only typically work if the card is about to play on the stacks.
	// Otherwise, it would look like a Tempo Clue」）。例如紅色點到被 play clue 的 r1 與另一張紅牌，再次紅色是後者的 tempo clue
	if s.knows(lvTempo) && !s.publicKnown(s.tempoFocus(R, touched)) {
		return
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
		case p != 0 && !p.has(t) && !slices.Contains(it.fixed, y):
			ev.misleads = true // 被標成壞牌的牌記成什麼都會被棄掉，不算誤導
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
	if s.fixMisreadByBystander(it, h, ev.target, ev.touched) {
		ev.misleads, ev.danger = true, true
	}
	// 修完後接收者手上還有會打錯的壞牌：這個提示擋不住眼前的閃電（例如兩張都被記成 G5，Stop Clue 只標出先打的那張，
	// 另一張就確定是 G5 而打錯；使用者回報 20261116-120334 的修正後，重播 20261112-155241 t24 找到的情況）
	if ev.fixesNow > 0 {
		for y, now := range broken {
			if _, ok := it.fixPoss[y]; !now || ok || s.holder(y) != it.receiver || s.playableNow(identOfCard(s.cards[y])) {
				continue // 重複的兩張留下的那張實際能打，不會打錯
			}
			if p := s.notes[y].poss & it.explicit[y]; p != 0 && !slices.ContainsFunc(p.list(), func(i ident) bool { return !s.playableNow(i) }) {
				ev.fixesNow = 0
				break
			}
		}
	}
	ev.understood = !ev.misleads
	ev.score = scoreFixNow*ev.fixesNow + scoreFix*ev.fixes + scoreNewGood*ev.newGood - scoreBadTouch*ev.bad - dist
	if h.Kind == game.HintColor {
		ev.score += scoreColor
	}
	return ev
}

// fixMisreadByBystander：（我是給提示者）有旁觀者看不出這是 Fix Clue——重複的那張在他自己手上，他看不到——
// 而照一般的解讀，他會以為要打自己的牌。旁觀者預設自己的牌都是好的，照 play clue 讀（使用者回報 20261102-155921 t30：
// AI-2 用綠色揭露 AI-1 被「3」點到的是 M3，和使用者手上已提示的 M3 重複；使用者看不到自己那張，讀成 M3 的 play clue、
// finesse 自己的 M2 而盲打錯）。他那張是被可能是 Save Clue 的提示一起點到的牌時（`saveLoose`）他看得懂，不算。
func (s *state) fixMisreadByBystander(it interp, h game.Hint, R int, touched []int) bool {
	blind := false
	for y := range it.fixPoss {
		t := identOfCard(s.cards[y])
		for q, hand := range s.hands {
			if q == s.me || q == R {
				continue
			}
			blind = blind || slices.ContainsFunc(hand, func(z int) bool {
				c, ok := s.card(z)
				u, known := s.possOf(z).single()
				return ok && s.gotten(z) && identOfCard(c) == t && (!known || u != t) && !(s.saveLoose(z) && s.notes[z].explicit.has(t) && s.aboutToPlay(y))
			})
		}
	}
	if !blind {
		return false
	}
	defer s.saveSim()()
	s.notFix = true
	return s.interpret(s.me, R, h, touched).stray
}

// saveLoose：牌 z 是某個焦點在 chop 上的提示一起點到的非焦點新牌。那個提示可能是 Save Clue，原文 Save Principle 允許
// 給提示者為了救牌違反 Good Touch，所以持有者看到別人已提示的牌被補成 z 可能的牌面時，不預設 z 是好牌，
// 會想到那是修正重複的 Fix Clue（使用者回報 20261105-093701：t14 AI-1 用「2」救使用者 chop 上的 G2，一起點到和 AI-2 的 B2
// 重複的 B2；用藍色修 AI-2 時，使用者以前照「自己的牌是好的」讀成 B2 的 play clue、要自己盲打 B1，所以沒人修，AI-2 打錯）。
// 只在被補資訊的牌是接收者本來就要打的牌時這樣讀（原文 Fix Clue 第一個例子：「Since Bob was about to play this card,
// Bob knows that this was a Fix Clue」）；一律這樣讀時，Rainbow 的顏色提示常一起點到彩虹牌，真正的 play clue 也被讀成 Fix Clue，
// 4–5 人局約 -0.05–0.09。
func (s *state) saveLoose(z int) bool {
	return slices.ContainsFunc(s.recs, func(rec *clueRec) bool {
		return rec.chop && rec.focus != z && slices.Contains(rec.fresh, z)
	})
}

// aboutToPlay：（我看得到的）牌 y 的持有者推得出它現在能打（holderView 的每種可能都能打），下一手就會打它。
func (s *state) aboutToPlay(y int) bool {
	hv := s.holderView(y, s.certainMine())
	return hv != 0 && !slices.ContainsFunc(hv.list(), func(i ident) bool { return !s.playableNow(i) })
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

// myCluedCertain：我手上已提示的牌一定有的牌面（別人看得到）。k 張已提示的牌合起來只可能是 k 種牌面時，這 k 種都在我手上
// （已提示的牌依 Good Touch 不會重複；使用者回報 20261110-121508 t7：AI-2 被「1」點到的兩張都是「G1 或 B1」，合起來就是
// G1、B1，AI-3 看得到，依 Good Touch 知道自己被「1」點到的牌不是它們；以前只看單張確定的，AI-2 以為 AI-3 會把 Y1 當 G1 / B1
// 打出而白給 Fix Clue）。
func (s *state) myCluedCertain() idSet {
	var cards []int
	for _, id := range s.hands[s.me] {
		if s.notes[id].clued && s.live(id) {
			cards = append(cards, id)
		}
	}
	if len(cards) > 8 {
		cards = cards[:8]
	}
	var out idSet
	for mask := 1; mask < 1<<len(cards); mask++ {
		var u idSet
		k := 0
		for i, id := range cards {
			if mask&(1<<i) != 0 {
				u |= s.myPoss(id)
				k++
			}
		}
		if len(u.list()) == k {
			out |= u
		}
	}
	return out
}

// holderView：y 的持有者自己推得出的可能——共同筆記再扣掉他看得到所有剩下張數的牌面
// （別人手上我看得到的牌，加上我手上一定有的牌面，他都看得到），被拿到的牌再依 Good Touch 扣掉別人手上已提示牌的牌面。
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
	// Good Touch：他看得到別人手上被提示過的牌是哪張，自己被拿到的牌就不是那張
	// （使用者回報 20261105-093701：AI-2「2」點到的 B2 和使用者被「2」救下的 B2 重複，AI-2 看到那張 B2 就以為自己的是能打的 Y2）
	if s.gotten(y) {
		gt := p
		for r, h := range s.hands {
			if r == q {
				continue
			}
			if r == s.me {
				gt &^= s.myCluedCertain()
				continue
			}
			for _, z := range h {
				if !s.notes[z].clued || !s.live(z) {
					continue
				}
				if c, ok := s.card(z); ok {
					gt &^= setOf(identOfCard(c))
				}
			}
		}
		if gt != 0 {
			p = gt
		}
	}
	return p
}

// brokenCards 找出隊友手上違反 Good Touch、需要 Fix Clue 的牌（值為 true 表示他會把它當成能打的牌而打錯）：
// 他以為能打但其實不能打、其實是垃圾、筆記和實際牌面矛盾，或和別處被拿到的牌重複
// （兩張的持有者都不知道時；有人知道的話他會 Sarcastic Discard）。
// 我計畫的 Hidden Finesse 要先被當成 prompt 打出的已提示牌（筆記和實際牌面不同是故意的，它現在能打）不算。
func (s *state) brokenCards() map[int]bool {
	out := map[int]bool{}
	hidden := func(y int) bool {
		for _, rec := range s.recs {
			if slices.ContainsFunc(rec.plan, func(d dep) bool { return d.card == y && !d.finesse && d.layers > 0 }) {
				return true
			}
		}
		return false
	}
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
			wrongPlay := !s.notes[y].bad && !s.playableNow(t) && !slices.ContainsFunc(hv.list(), func(i ident) bool { return !s.playableNow(i) }) &&
				!s.twinPlaysFirst(y, hv)
			dup := false
			if z := s.dupOf(y); z >= 0 && !knows(y, t) && !knows(z, t) {
				dup = true
				if s.holderKnowsPlayable(y) && (s.holder(z) == s.me && s.knownPlayable(z) || s.holderKnowsPlayable(z)) {
					wrongPlay = true // 兩張都會被打，後打的那張會打錯
				}
			}
			if wrongPlay || dup || s.trash(t) || !p.has(t) && !(s.playableNow(t) && hidden(y)) {
				out[y] = wrongPlay
			}
		}
	}
	return out
}

// unlocksMisplay：這個 play clue 讓 t 與連接牌（plan）打出後，有隊友記錯的已提示牌（brokenCards）在他看來變成能打、實際卻不能打
// ——提示本身沒誤導，卻等於叫他打錯（Misplay Cost Principle）。先修（Fix Clue）或給別的提示。使用者回報 20261106-151527 t29：
// AI-2 被 Bluff 讀成 R3 的牌實際是 R4，AI-1 用最後一個提示讓使用者打出 R2，t33 AI-2 就打錯 R4。
func (s *state) unlocksMisplay(t ident, plan []dep) bool {
	broken := s.brokenCards()
	if len(broken) == 0 {
		return false
	}
	saved := s.stacks
	defer func() { s.stacks = saved }()
	for _, u := range append([]ident{t}, depIDs(plan)...) {
		s.stacks[u.color()] = max(s.stacks[u.color()], u.rank())
	}
	for y, now := range s.brokenCards() {
		if w, was := broken[y]; !now || was && w {
			continue
		}
		return true
	}
	return false
}

// twinPlaysFirst：持有者以為 y 只可能是 i（hv），而他照出牌順序會先打的是另一張實際就是 i 的牌——打出後 y 就成了他知道的重複，
// 不會打錯，不必急著修（使用者回報 20261105-161025 t12：「1」點到使用者的 G1 與已打出的 Y1，使用者看得到 AI-3 的 W1，
// 兩張都以為是 G1，照 Level 3 的順序先打新抽的 G1，另一張就是已知垃圾；AI-3 以前當成他會打錯，花提示用黃色修）。
func (s *state) twinPlaysFirst(y int, hv idSet) bool {
	i, ok := hv.single()
	if !ok {
		return false
	}
	p := s.holder(y)
	restore := s.saveSim()
	s.overlay = map[int]idSet{}
	for _, z := range s.hands[p] {
		q := s.deduce(p, z)
		if u := q & s.useful(); s.gotten(z) && u != 0 {
			q = u
		}
		s.overlay[z] = q
	}
	z := s.nextPlay(p)
	restore()
	return z >= 0 && z != y && identOfCard(s.cards[z]) == i
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
			// 別人的 finesse position 看得到就是 m：給提示者可能是要 finesse 他，等他盲打就知道（自我對局 seed 302：
			// 綠色的 g3 要 finesse Donald 的 g2，Bob 把自己「g2 或 w2」的牌鎖成 g2，接著白色要它當 w2 時他就盲打錯）
			if p != s.me && p != rec.giver {
				if fp := s.finessePos(p); fp >= 0 {
					if c, ok := s.see(s.me, fp); ok && c == m {
						elsewhere = true
					}
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
			// 本來就確定是 m（例如之前的 Bluff 已讓它確定）時不算這個提示改的，撤銷這個提示的筆記時才不會連之前的推論
			// 一起抹掉（使用者回報 20261106-144321：t21「4」的 W4 讓 AI-1 把已確定是 W2 的 slot 5 再鎖一次，
			// t22 認定是 Bluff、撤掉筆記時 W2 也被洗掉，中間的牌再由左而右配成 slot 4 = W2、slot 5 = W3，t32 打錯）
			if s.notes[mine[0]].poss != setOf(m) {
				s.notes[mine[0]].poss = setOf(m)
				rec.changed = append(rec.changed, mine[0])
			}
			if s.knows(lvHiddenFinesse) {
				// 等於這個提示 prompt 了我那張牌：它打出來若是別張能打的牌，給提示者看得到那張不是 m，
				// 真正的 m 在我提示當下的 finesse position（Level 5 Hidden Finesse，stepDep 會接手）。使用者回報 20261020-162330：
				// t37「5」的 y5 要 y4，AI-1 把自己被「4」點到的牌鎖成 y4，打出來是 g4 後沒有去盲打 finesse position 的 y4
				rec.plan = append(rec.plan, dep{player: s.me, card: mine[0], id: m})
			}
		}
	}
}

// sarcastic（Level 3 The Sarcastic Discard）：有人棄掉一張被拿到、而且不是垃圾的牌 t，表示別人手上已被拿到的牌裡有 t
// （他發現自己那張是重複的）。看得到的人知道是誰；都看不到就是我。那人手上只有一張可能是 t 的牌時，它就是 t
// （有好幾張時不保證是哪一張）。鎖手被迫棄牌不算。回傳這次棄牌是不是 Sarcastic Discard（Level 10 起也可能是
// Gentleman's / Baton Discard：沒有人手上有已被拿到、可能是 t 的牌時，t 傳到另一張的 finesse position；
// certain = 這是 Certain Discard，已由 certainDiscard 處理）。
func (s *state) sarcastic(e game.Event, certain bool) bool {
	t := identOfCard(e.Card)
	if !s.knows(lvSarcastic) || !s.live(e.Card.ID) || s.trash(t) || s.chop(e.Player) < 0 && !s.deliberateLocked(e.Card.ID) {
		return false
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
		if !certain {
			s.gentleman(e) // 我自己棄的：別人手上看不到被拿到的 t，是 Gentleman's / Baton Discard
		}
		return true
	}
	cands := s.sdTargets(holder, t, s.deliberateLocked(e.Card.ID))
	if len(cands) == 0 && !certain {
		s.gentleman(e) // Sarcastic Discard 優先；我也沒有可能是 t 的已提示牌，就是 Gentleman's / Baton Discard
	}
	if len(cands) == 1 {
		s.notes[cands[0]].poss = setOf(t)
		s.notes[cands[0]].sdAs = setOf(t)
	}
	return true
}

// sarcasticSlot（Level 3 The Sarcastic Discard）：我手上被拿到、確定是 t 的牌，而別人手上已被拿到的牌實際就是 t：
// 我是先發現重複的人，棄掉它（他會知道自己有 t）。提示滿 8 不能棄牌；鎖手時棄牌會被當成被迫的，不用。
func (s *state) sarcasticSlot(v game.PlayerView) int {
	if v.Hints >= game.MaxHints {
		return -1
	}
	for slot, x := range s.hands[s.me] {
		t, ok := s.myPoss(x).single()
		if !s.live(x) || !ok || s.trash(t) || s.chop(s.me) < 0 && !s.deliberateLocked(x) {
			continue
		}
		for q, h := range s.hands {
			if q == s.me {
				continue
			}
			for _, y := range h {
				// 他已經知道就不必（他會自己打出，我這張之後就是已知沒用的牌）；他的筆記沒有 t 時，棄牌也告訴不了他
				// 他的筆記被推論錯成別張（沒有 t，提示資訊卻容許 t）也算：他手上只有那一張可能是 t 時，棄牌讓他改過來
				// （使用者回報 20261111-163529 t24：AI-2 被「4」點到的 B4 被大家記成 Y4，AI-3 確定自己的是 B4，
				// 以前直接打出，AI-2 那張就成了他不知道的重複牌，之後會當 Y4 打錯）
				if p := s.notes[y].poss; s.live(y) && identOfCard(s.cards[y]) == t && p != setOf(t) &&
					(p.has(t) && (!s.playableNow(t) || s.sdCands(q, t) == 1) || !p.has(t) && s.deliberateLocked(x) && len(s.sdTargets(q, t, true)) == 1) {
					return slot
				}
			}
		}
	}
	return -1
}

// deliberateLocked：鎖手的人棄掉這張不是被迫的——大家都知道它是現在就能打的牌（共同筆記確定）。
// 鎖手被迫棄牌時會棄最不重要的牌，有牌可打就不會被迫棄牌，所以棄它就是 Sarcastic Discard。
// 提示 0 個時不算：那時棄能打的牌可能是為了換提示（Level 9 The Anxiety Play：下一位鎖手，不能讓他停在 0 個提示；
// 自我對局 Rainbow 4 人局 seed 145：鎖手的 AI 棄掉確定的 R2 換提示，被讀成 Sarcastic Discard，下一位把 G2 當 R2 打錯）。
func (s *state) deliberateLocked(id int) bool {
	t, ok := s.possOf(id).single()
	return ok && s.playableNow(t) && s.hints > 0
}

// sdTargets：有人 Sarcastic Discard 掉 t 時，holder 手上可能是 t 的被拿到的牌：先看筆記；筆記都排除了 t 時，
// 改看提示資訊容許 t 的（他的筆記推論錯了——棄牌的人看得到那張就是 t，這就是在告訴他）。好幾張都容許時不保證是哪一張。
// 後者只在 strong（棄掉的是大家都知道現在就能打的牌，一定是刻意的）時才看：一般的棄牌可能只是失誤或以為是重複，
// 一律這樣讀時 3000 局 Lv11 4–5 人局 -0.05–0.12、閃電變多。
func (s *state) sdTargets(holder int, t ident, strong bool) []int {
	var cands []int
	for _, id := range s.hands[holder] {
		if s.live(id) && s.knownPoss(holder, id).has(t) {
			cands = append(cands, id)
		}
	}
	if len(cands) > 0 || !strong {
		return cands
	}
	for _, id := range s.hands[holder] {
		if s.live(id) && s.notes[id].explicit.has(t) {
			cands = append(cands, id)
		}
	}
	return cands
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

func depIDs(plan []dep) []ident {
	out := make([]ident, len(plan))
	for i, d := range plan {
		out[i] = d.id
	}
	return out
}
