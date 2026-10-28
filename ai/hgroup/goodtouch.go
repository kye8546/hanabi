// Good Touch 與卡片推論：被拿到的牌之間互相排除（gottenAs、gtpExcluded、revealGTP），
// 給提示者知不知道自己的牌（giverMayKnow、giverKnew、knowsFinessed），以及我對自己手牌的推論（deduce）。

package hgroup

import (
	"slices"

	"hanabi/game"
)

// revealGTP：被拿到的牌 x 打出後公開了牌面 t。依 Good Touch，跟它互相「看得到」的其他被拿到的牌都不會是 t
// （後拿到的那張，給提示者當時看得到先拿到的那張；同一個提示點到的牌也不會重複）。
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
			// 同一個提示點到的也不會重複——save clue 也一樣：Save Principle 允許給提示者違反 Good Touch，
			// 但接收者照樣依 Good Touch 寫筆記（Beginner's Guide Save Principle：「From Good Touch Principle,
			// Bob will write "b2, b3, b4, b5" on his two other blue cards」）。Black 變體裡「1」點到 chop 總是
			// 可能是救 k1，以前把它當成可能重複，打出一張 1 之後就不敢打其他的 1
			if xn.seq > n.seq && xn.giver != p || n.seq > xn.seq && (n.giver != hx || p == s.me && s.giverKnew(id, x, t)) ||
				n.seq == xn.seq {
				n.poss &^= setOf(t)
			}
		}
	}
}

// recsWanting：有解讀要牌 x 當連接牌（prompt / finesse）打出的提示（打出前呼叫，打出後提示可能已經結案）。
func (s *state) recsWanting(x int) []*clueRec {
	var out []*clueRec
	for _, rec := range s.recs {
		if slices.ContainsFunc(append(slices.Clone(rec.cands), rec.fallback...), func(c cand) bool {
			return slices.ContainsFunc(c.deps, func(d dep) bool { return d.card == x })
		}) {
			out = append(out, rec)
		}
	}
	return out
}

// revealConnection：牌 x（牌面 t）照提示 recs 的解讀當連接牌打出。給提示者計畫要 x 打出 t，就不會同時點到另一張 t
// （planned 的連接牌也算已被拿到），所以那個提示點到的其他新牌都不是 t——接收者收到提示時看不到焦點、不知道是 finesse，
// 筆記上可能還留著 t（使用者回報 20261027-124308：黃色點到 AI-1 的 Y3 與另一張黃牌，finesse 使用者的 Y2；
// 那張黃牌記成「Y2 或 Y4」，Y2 盲打出來後還當成不確定而沒打 Y4）。被 finesse 的 x 沒被拿到時 revealGTP 不處理這種情況。
func (s *state) revealConnection(recs []*clueRec, x int, t ident) {
	for _, rec := range recs {
		for _, id := range rec.fresh {
			if n, ok := s.note(id); ok && id != x && s.holder(id) >= 0 && !n.bad && n.poss.has(t) && n.poss.count() > 1 {
				n.poss &^= setOf(t)
			}
		}
	}
}

// receiverHadIt：牌 x（牌面 t）打出前，recs 中哪些提示的接收者不論哪種解讀都把 t 當成已被拿到——每種解讀不是要 x 當 t
// 打出（finesse / prompt），就是焦點本身是 t（他自己那張）。他不確定 x 是不是被 finesse 的，但之後他給的提示
// 不會點到 t（Good Touch：已被拿到，或和自己不確定的已提示牌重複）。打出前呼叫（打出後解讀會被刪減）。
func (s *state) receiverHadIt(recs []*clueRec, x int, t ident) []*clueRec {
	var out []*clueRec
	for _, rec := range recs {
		all := append(slices.Clone(rec.cands), rec.fallback...)
		if len(all) > 0 && !slices.ContainsFunc(all, func(c cand) bool {
			return c.id != t && !slices.ContainsFunc(c.deps, func(d dep) bool { return d.card == x && d.id == t })
		}) {
			out = append(out, rec)
		}
	}
	return out
}

// revealToReceiver：x 打出 t 證明了 recs 的連接（receiverHadIt），它們的接收者之後給的提示點到的牌都不是 t。
// revealGTP 只處理給提示者確定知道已被拿到的牌；被 finesse 的 x，那次提示的接收者看不到焦點、當時不確定是 finesse
// （使用者回報 20261028-163014：白色點 AI-1 的 W2、finesse AI-2 的 W1，AI-1 接著用「1」點使用者的三張 1；
// W1 打出後 AI-2 依 Good Touch 排除了 W1，AI-1 沒有，以為使用者的 1 還可能是 W1，看不出他 t7 的 Order Chop Move）。
func (s *state) revealToReceiver(recs []*clueRec, x int, t ident) {
	for _, rec := range recs {
		for _, h := range s.hands {
			for _, id := range h {
				if n := s.notes[id]; id != x && n.giver == rec.target && n.seq > rec.seq && !n.bad && n.poss.has(t) && n.poss.count() > 1 {
					n.poss &^= setOf(t)
				}
			}
		}
	}
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

// giverMayKnow：（obs 角度）牌 id 的持有者可能知道它是 m：它是某次給他的提示的焦點，而除了 m 以外的解讀
// 全都要靠 obs 自己的牌（持有者看得到 obs 的牌，obs 自己看不到，所以 obs 無法排除他已經知道）。
// 持有者正在給提示時，他點到的牌（他看得到）是 c 的解讀也不算：他不會點到可能和自己已提示的牌重複的牌
// （Good Touch，`maybeMine`），點了就表示他已排除那個解讀——例如他「g1 或 g3」的牌還在等 Reverse Finesse，
// 他又點到別人的 g1，在他心中那張就是 g3。
func (s *state) giverMayKnow(obs, id int, m ident) bool { return s.giverMayKnowD(obs, id, m, 2) }

// giverMayKnowD 同 giverMayKnow；depth 是 holderExcludes 還能往下問「那張牌的持有者知不知道」的層數。
func (s *state) giverMayKnowD(obs, id int, m ident, depth int) bool {
	h := s.holder(id)
	// 大家都推得出的筆記（不是我自己手上的牌——那是我的私下推論）已確定是 m，或提示本身扣掉大家都知道沒用的牌面就只剩 m
	// （使用者回報 20261028-145155 t50：AI-1 只被「3」點過的牌，Y3、B3 都已沒用，大家都知道是 W3；以前不算，
	// 模擬「4」點使用者的 W4 時以為他會 self-finesse 自己的 W3，找不到終局拖延的提示而棄牌，來不及滿分）
	if t, ok := s.notes[id].poss.single(); ok && t == m && h != s.me {
		return true
	}
	if t, ok := (s.notes[id].explicit & s.useful()).single(); ok && t == m {
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
			}) || s.holderExcludes(obs, h, id, rec.giver, c.id, depth) {
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

// holderExcludes：持有者 h 依 Good Touch 知道他的已提示牌 id 不是 c——c 已經打出 / 打不出去，或他看得到別處有已被拿到的牌
// 就是 c（那張若在給他提示的人手上，要那人也知道它是 c：大家的筆記確定，或同樣用 giverMayKnow 推得出）。obs 也看得到那張才算（obs 自己的牌不算）。
// 使用者回報 20261020-152805 t7：使用者的藍牌是 t5 那個藍色的焦點，解讀裡還留著已經打出的 b1、和 AI-1 手上已被提示的 b2，
// AI-2 就以為使用者不一定知道自己的是 b3，不依 Good Touch 排除，把使用者「b4 的 delayed play clue」讀成 b3，再 Sarcastic Discard 掉 b4。
func (s *state) holderExcludes(obs, h, id, giver int, c ident, depth int) bool {
	if s.trash(c) {
		return true
	}
	for p, hand := range s.hands {
		if p == h || p == obs {
			continue
		}
		for _, y := range hand {
			if y == id || !s.live(y) {
				continue
			}
			if t, ok := s.see(obs, y); ok && t == c {
				if p != giver {
					return true
				}
				if u, ok := s.notes[y].poss.single(); ok && u == c || depth > 0 && s.giverMayKnowD(obs, y, c, depth-1) {
					return true
				}
			}
		}
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
				if c, ok := s.card(id); ok {
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
// 同一個 save clue 點到的牌允許重複（Save Principle 可以違反 Good Touch）；Level 8 Distribution Clue 的焦點也是。
func (s *state) gtpExcluded(h, card int, i ident) bool {
	m := s.notes[card]
	if m.dist {
		return false // Level 8 Distribution Clue：刻意重複
	}
	for q, hand := range s.hands {
		for _, id := range hand {
			pend := !s.gotten(id) && slices.Contains(s.pending, id)
			if id == card || !s.live(id) && !pend || s.notes[id].dist {
				continue
			}
			if m.giver >= 0 && !s.knowsFinessed(m.giver, id) {
				continue // card 的給提示者不知道 id 已被 finesse，點到同一張不算違反 Good Touch
			}
			known := false
			if q == h || q == s.me {
				t, ok := s.notes[id].poss.single()
				known = ok && t == i
			} else if c, ok := s.card(id); ok {
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
				o.seq == m.seq {
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
			if c, ok := s.card(d.card); ok {
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

// pruneByMyCards：算接收者的解讀時我看不到自己的牌；現在若推論出我手上某張「被提示的牌」就是 x，
// 接收者其實看得到它，依 Good Touch 不會把焦點當成 x（給提示者看不到自己的牌，所以不算給提示者手上的）。
func (s *state) pruneByMyCards(rec *clueRec) {
	if s.me == rec.giver {
		return
	}
	var mine idSet
	for _, id := range s.hands[s.me] {
		if n := s.notes[id]; n.clued {
			if t, ok := n.poss.single(); ok {
				mine |= setOf(t)
			}
		}
	}
	kept := slices.DeleteFunc(slices.Clone(rec.cands), func(c cand) bool { return mine.has(c.id) })
	if len(kept) > 0 && len(kept) < len(rec.cands) {
		rec.cands = kept
		s.refreshFocus(rec)
	}
}

// freshUseful：（obs 角度）將來有用、而且別處沒有被拿到的牌面。依 Good Touch，被提示的新牌只會是這些
// （別處已被提示的牌面，再點到就是重複）。exclude 是正在推論的那張牌。
func (s *state) freshUseful(obs, giver, exclude int) idSet {
	var u idSet
	for _, i := range s.useful().list() {
		if !s.gottenAs(obs, giver, i, exclude) {
			u |= setOf(i)
		}
	}
	return u
}

// nonFocusPoss 是被提示點到的非焦點新牌的筆記：依 Good Touch 將來一定有用（useful 應已扣掉別處被拿到的牌面）、
// 也不會和焦點重複。
// 找不到這種可能時，它就是看得出來的違反 Good Touch 的牌：只會是垃圾或焦點的重複
// （焦點打出後就是已知沒用的牌，會先棄它而不是 chop）。
func nonFocusPoss(poss, useful, focus idSet) idSet {
	if p := poss & useful &^ focus; p != 0 {
		return p
	}
	if p := poss&^useful | poss&focus; p != 0 {
		return p
	}
	return poss
}

// touchedUseful：提示沒有任何解讀時，依 Good Touch 被點到的牌仍是將來有用、而且別處沒被拿到過的牌。
// 一種都找不到就是違反 Good Touch 的牌（垃圾或重複），維持原本的筆記（之後會被推論成已知沒用的牌）。
func (s *state) touchedUseful(obs, giver, id int, p idSet) idSet {
	var q idSet
	for _, i := range (p & s.useful()).list() {
		if !s.gottenAs(obs, giver, i, id) {
			q |= setOf(i)
		}
	}
	if q != 0 {
		return q
	}
	return p
}

// maybeMine：我手上有被拿到、但我不確定是什麼的牌可能是 c。這時提示 c 可能造成重複，
// 依 Good Touch 也算違反（Level 10 Sarcastic Finesse：「potentially duplicating a card is bad -
// it violates Good Touch Principle」；Level 12 Duplication Responsibility）。
func (s *state) maybeMine(c ident) bool {
	for _, id := range s.hands[s.me] {
		if s.gotten(id) && s.myPoss(id).has(c) {
			return true
		}
	}
	return false
}

// awaited：我收到的提示還有未定的解讀，其中某種要別人 prompt / finesse 出牌面 c（我在等他打），而 card 不是那張牌。
// 等待時要做無關的動作：點到另一張 c，finesse 一兌現它就是重複的牌，同樣違反 Good Touch
// （和 maybeMine 一樣，只是重複的那張在別人手上、要等他打出才確定）。用到同一張牌（例如也靠它接上）不算重複。
func (s *state) awaited(c ident, card int) bool {
	for _, rec := range s.recs {
		if rec.target != s.me || s.holder(rec.focus) != s.me {
			continue
		}
		for _, cd := range rec.cands {
			if slices.ContainsFunc(cd.deps, func(d dep) bool {
				return d.player != s.me && d.id == c && !slices.Contains(s.depCards(d), card)
			}) {
				return true
			}
		}
	}
	return false
}

// pendingLayer（Level 5 Layered Finesse）：c 是某個進行中的 Layered Finesse 還沒盲打出的 layer（我看得到 plan）。
// 別人先打出另一張 c，那張 layer 就成了重複的牌，盲打的人照計畫打出就打錯——提示另一張 c（點到它或要人打出它）
// 和 Good Touch 一樣不該做（自我對局 seed 6171：w2 要 Bob 依序盲打 layer r1、g1 再打 w1，別人又給了要 Emily 盲打 g1
// 的提示，Bob 照計畫盲打 g1 就打錯）。card 是那張 c 本身（用到同一張牌不算）。
func (s *state) pendingLayer(c ident, card int) bool {
	for _, rec := range s.recs {
		// 我看不到焦點時沒有計畫，接收者角度的解讀裡有 Layered Finesse 也算（自我對局 seed 7682：我收到的白色可能是
		// 「w2，P4 依序盲打 layer r1、g1」，我就不該給 P1 點到 r1、g1 的「1」）
		ds := slices.Clone(rec.plan)
		for _, c := range append(slices.Clone(rec.cands), rec.fallback...) {
			ds = append(ds, c.deps...)
		}
		for _, d := range ds {
			if !d.finesse || d.layers == 0 || s.holder(d.card) != d.player {
				continue
			}
			cards := s.depCards(d)
			for _, l := range cards[:len(cards)-1] {
				if l != card {
					if t, ok := s.see(s.me, l); ok && t == c {
						return true
					}
				}
			}
		}
	}
	return false
}

// awaitedFinesse：我收到的提示還有未定的解讀要 p 盲打——他的 finesse position 有沒有被佔走還不確定，
// 等待時不給要他盲打的提示（我以為的 finesse position 可能和他自己認定的不同）。
func (s *state) awaitedFinesse(p int) bool {
	for _, rec := range s.recs {
		if rec.target != s.me || s.holder(rec.focus) != s.me || len(rec.cands) < 2 {
			continue
		}
		for _, cd := range rec.cands {
			if slices.ContainsFunc(cd.deps, func(d dep) bool { return d.player == p && d.finesse }) {
				return true
			}
		}
	}
	return false
}

// focusSingle 回傳接收者在提示後確定的焦點牌面（不確定則為空集合）。
func (s *state) focusSingle(it interp) idSet {
	var u idSet
	for _, c := range it.cands {
		u |= setOf(c.id)
	}
	p := s.notes[it.focus].poss & it.explicit[it.focus]
	if q := p & u; q != 0 {
		p = q
	}
	if f, ok := p.single(); ok {
		return setOf(f)
	}
	return 0
}

// seenAsTrash：提示後接收者推得出這張非焦點新牌不可能有用（只會是垃圾或焦點的重複）。
func (s *state) seenAsTrash(it interp, id int) bool {
	p := s.notes[id].poss & it.explicit[id]
	return p&s.freshUseful(it.receiver, s.me, id)&^s.focusSingle(it) == 0
}
