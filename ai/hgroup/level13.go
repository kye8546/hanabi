// Level 13（docs/level-13.mdx）：Intermediate Bluffs。
//
// 合法的 Bluff 目標（原文 Legal Bluff-Targets 的表）在 Level 11 的「差一張就能打」之外，再加上任何還不能打、之後有用的 3
// （3 Bluff，顏色或數字提示都可以；Hard 3 Bluff：「3」讓人盲打出同色的 1 也是）與顏色提示點到之前沒被提示的 critical 非 5
// （Critical Color Bluff）。接不接得上照舊是 Level 11 的 Cathy's Connecting Principle（bluffConnects）。
// Hard Bluff：盲打出的牌和焦點同色、中間的牌都 prompt 得到（finesse + prompt），同時又有別的 Bluff 目標時，兩種都記下，
// 被 prompt 的已提示牌不寫筆記（Cathy 不假設她的 3 是 r3）。Known Bluff 與 Good Touch Bluff 見下面的函式。

package hgroup

import (
	"slices"

	"hanabi/game"
)

// extraBluffTargets（Level 13）：Level 11 的目標（targets，差一張就能打）之外，焦點還可能是哪些合法的 Bluff 目標。
// 呼叫時 s.stacks 已經是盲打之前的煙火（bluffTargets 設好的）。新加的目標都不寫中間的牌（assigns 是空的）。
func (s *state) extraBluffTargets(rec *clueRec, ex idSet, x ident, targets idSet, assigns map[ident][]dep) idSet {
	R, giver := rec.target, rec.giver
	ok := func(t ident) bool {
		return !targets.has(t) && !s.playableNow(t) && !s.gottenAs(R, giver, t, rec.focus) && s.remainingFor(R, t) > 0
	}
	// 3 Bluff：任何之後有用的 3（顏色接得上時是 finesse，到不了這裡）
	// Critical Color Bluff：只能用顏色提示、只能是之前完全沒被提示的牌、不能是 5（原文：「includes the black 4」）
	ccb := s.knows(lvCritColorBluff) && rec.hint.Kind == game.HintColor && slices.Contains(rec.fresh, rec.focus)
	for _, t := range (ex & s.useful()).list() {
		if ok(t) && (s.knows(lv3Bluff) && t.rank() == 3 || ccb && t.rank() != game.MaxRank && s.critical(t)) {
			targets |= setOf(t)
			assigns[t] = nil
		}
	}
	if !s.knows(lvHardBluff) || targets == 0 {
		return targets
	}
	// Hard Bluff：焦點也可能是和 x 同色、經由 x 和中間 prompt 得到（或已被拿到）的牌接上的牌——finesse + prompt。
	// 原文：「Cathy marks down both possibilities for later and does not play anything right now」
	defer s.saveSim()()
	s.skipCards = append(slices.Clone(s.skipCards), s.hands[s.me]...)
	for _, t := range (ex & s.useful()).list() {
		if t.color() != x.color() || t.rank() <= x.rank()+1 || !ok(t) {
			continue
		}
		used := append([]int{rec.focus}, rec.fresh...)
		reach := true
		for r := x.rank() + 1; r < t.rank() && reach; r++ {
			m := identOf(t.color(), r)
			if s.alreadyGotten(R, giver, m, used) >= 0 {
				continue
			}
			_, took, prompt := s.findPrompt(R, giver, m, used)
			reach = prompt
			used = append(used, took...)
		}
		if reach {
			targets |= setOf(t)
			assigns[t] = nil
		}
	}
	return targets
}

// goodTouchBluffCard（Level 13 The Good Touch Bluff，旁觀者的推論）：盲打出的 x 接得上提示，照接收者的讀法焦點可以是 x 那一色
// 更大的牌 u（finesse），我卻看得到焦點是別張。給提示者不會給讓接收者打錯的 Bluff，唯一的解釋是 u 已在我手上被提示
// （接收者看得到、依 Good Touch 排除它）：我手上只有一張已提示、可能是 u 的牌時，回傳它和 u。
// 原文：「Since Cathy did not misplay, Donald can reason that he must have the blue 4」。
func (s *state) goodTouchBluffCard(rec *clueRec, x ident) (int, ident) {
	truth, see := s.see(s.me, rec.focus)
	if !s.knows(lvGoodTouchBluff) || !see || rec.target == s.me || rec.giver == s.me {
		return -1, 0
	}
	var us []ident
	for _, t := range (s.notes[rec.focus].explicit & s.useful()).list() {
		if t.color() == x.color() && t.rank() > x.rank() && !s.gottenAs(rec.target, rec.giver, t, rec.focus) && s.remainingFor(rec.target, t) > 0 {
			us = append(us, t)
		}
	}
	if len(us) != 1 || us[0] == truth {
		return -1, 0
	}
	u := us[0]
	card := -1
	for _, id := range s.hands[s.me] {
		if n := s.notes[id]; s.gotten(id) && s.live(id) && n.poss.has(u) {
			if card >= 0 {
				return -1, 0
			}
			card = id
		}
	}
	return card, u
}

// knownBluff（Level 13 The Known Bluff）：上一位剛給了 rec，我坐在他的 Bluff Seat、看得到焦點 t，但照我看得到的牌沒有任何
// 連接計畫（例如我知道自己沒有那張：負向資訊、看得到所有剩下的那張），接收者照他的解讀又會把它當成現在就能打的別張打錯，
// 而 t 是合法的 Bluff 目標——給提示者的意思只能是 Bluff 我：把我的 finesse position 記成要盲打（這一手就打，Lie Principle）。
// 原文：「if the next player cannot see a better explanation for the clue, then they should blind-play their Finesse Position card」。
func (s *state) knownBluff(rec *clueRec, it interp) {
	R := rec.target
	if !s.knows(lvKnownBluff) || !s.bluffSeat(rec.giver, s.me) || R == s.me || !it.hasTruth || len(rec.plan) > 0 ||
		it.planOK || it.saveOK || it.anySave || it.tempo || it.fix || it.cmKind != 0 || !slices.Contains(rec.fresh, rec.focus) {
		return
	}
	t := it.truth
	if s.trash(t) || s.playableNow(t) || slices.ContainsFunc(rec.cands, func(c cand) bool { return c.save }) {
		return
	}
	// 接收者沒有人盲打就會打錯：他有直接（不需連接牌）、現在就能打、卻不是 t 的解讀
	if !slices.ContainsFunc(rec.cands, func(c cand) bool { return len(c.deps) == 0 && c.id != t && s.playableNow(c.id) }) {
		return
	}
	fp := s.bluffSlot(rec, s.me, nil)
	if fp < 0 {
		return
	}
	n := s.notes[fp]
	// 我要確定自己沒有缺的下一張 m（原文：「the next player will know for sure that they do not have the "connecting" card」）：
	// finesse position 的提示資訊排除了 m，或剩下的每一張 m 都在別人手上。只是照我看得到的牌排不出計畫不夠——
	// 我看不到自己的牌，m 可能就在我手上別的位置，我的已提示牌也可能是連接牌（自我對局 Black 5 人局 seed 2475 t11：
	// 藍色點 P2 的 B3，P0 自己手上就有 B1、被「2」點過的牌是 B2，他排不出計畫就當成 Known Bluff 盲打而打錯）
	m := identOf(t.color(), s.stacks[t.color()]+1)
	if n.explicit.has(m) && s.visibleElsewhere(m)+s.discarded[m] < m.copies() {
		return
	}
	// 給提示者以外還有人手上有 m 時，多半是我看不懂的 finesse（例如各家認定他的 finesse position 不同），不是 Bluff
	// （自我對局 Black 5 人局 seed 2551 t12：紅色點 P0 的 R3，P2 以為 P3 的 R1 不在 finesse position 上、自己的 finesse position
	// 又被「1」排除，當成 Known Bluff 盲打而打錯；給提示者和 P3 都認為 P3 的 finesse position 就是 R1）
	for p, h := range s.hands {
		if p == s.me || p == rec.giver {
			continue
		}
		for _, id := range h {
			if c, ok := s.card(id); ok && identOfCard(c) == m {
				return
			}
		}
	}
	var play idSet
	for _, i := range (n.poss & s.useful()).list() {
		if s.playableNow(i) {
			play |= setOf(i)
		}
	}
	if play == 0 {
		return
	}
	// t 要是合法的 Bluff 目標：照接不上提示的任一張能打的牌 x 判斷（我不知道自己會打出哪張）
	for _, x := range play.list() {
		if s.bluffConnects(rec, x) {
			continue
		}
		if targets, _ := s.bluffTargets(rec, s.notes[rec.focus].explicit, fp, x); !targets.has(t) {
			return
		}
		s.got(fp, rec.giver)
		n.poss, n.finessed, n.finRec, n.willPlay = play, true, rec, true
		rec.changed = append(rec.changed, fp)
		return
	}
}

// visibleElsewhere：我看得到的別人手上有幾張 m。
func (s *state) visibleElsewhere(m ident) int {
	k := 0
	for p, h := range s.hands {
		if p == s.me {
			continue
		}
		for _, id := range h {
			if c, ok := s.card(id); ok && identOfCard(c) == m {
				k++
			}
		}
	}
	return k
}
