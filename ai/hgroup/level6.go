// Level 6（docs/level-6.mdx）：Tempo Clues——Focus Shifting、Valuable Tempo Clue、Tempo Clue Chop Move（TCCM），
// 以及 TCCM 優先於 Trash Chop Move。

package hgroup

import (
	"slices"

	"hanabi/game"
)

// Level 6 的判斷會決定要不要 Chop Move，必須大家看法一致（和 Level 4 一樣）。「知道能打」「是哪張牌」照大家的筆記判斷
// （原文的 Focus Shifting 就是看筆記：被 play clue 的 r1 大家都記成 r1），但只用每個人算法都一樣的那部分（publicPoss）——
// 持有者私下的推論、只有部分人知道的 finesse、各家對「要 prompt 幾張」的推論都可能不同
// （最初直接用各家筆記與 cands 的 prompt 數時，5 人局四成的 TCCM 各家看法不一）。

// publicPoss：大家都推得出的這張牌的可能——提示資訊（扣掉已打出、打不出去的），再套上它當焦點時那個提示的解讀
// （有不需要連接牌的解讀時只看它們），
// 以及某個提示的每種解讀都要它當連接牌時的那幾種牌面（例如被 prompt 成 r2）。已知沒用的牌回傳 0。
func (s *state) publicPoss(id int) idSet {
	n := s.notes[id]
	if n.bad {
		return 0
	}
	p := n.explicit & s.useful()
	narrow := func(u idSet) {
		if q := p & u; q != 0 && u != 0 {
			p = q
		}
	}
	for i := len(s.recs) - 1; i >= 0; i-- {
		if rec := s.recs[i]; rec.focus == id && len(rec.cands) > 0 {
			// 有不需要連接牌的解讀（直接、delayed 或 save）就只看它們：「也許有人被 finesse」的解讀各家不同——
			// 看不到自己牌的人排除不了自己被 finesse，看得到的人早就排除了
			var u, direct idSet
			for _, c := range rec.cands {
				u |= setOf(c.id)
				if len(c.deps) == 0 {
					direct |= setOf(c.id)
				}
			}
			if direct != 0 {
				u = direct
			}
			narrow(u)
			break
		}
	}
	for _, rec := range s.recs {
		var u idSet
		all := len(rec.cands) > 0
		for _, c := range rec.cands {
			i := slices.IndexFunc(c.deps, func(d dep) bool { return d.card == id })
			if i < 0 {
				all = false
				break
			}
			u |= setOf(c.deps[i].id)
		}
		if all {
			narrow(u)
		}
	}
	return p
}

// publicKnown：大家都知道這張牌現在就能打（publicPoss 全是現在能打的牌面）。
func (s *state) publicKnown(id int) bool {
	p := s.publicPoss(id)
	return p != 0 && !slices.ContainsFunc(p.list(), func(i ident) bool { return !s.playableNow(i) })
}

// publicElsewhere：除了 except，有已提示的牌大家都知道確定是 m。
func (s *state) publicElsewhere(m ident, except int) bool {
	for _, h := range s.hands {
		for _, id := range h {
			if id != except && s.notes[id].clued {
				if t, ok := s.publicPoss(id).single(); ok && t == m {
					return true
				}
			}
		}
	}
	return false
}

// tempoFocus：只點到已被提示的牌（Tempo Clue）時的焦點是最左邊那張。Level 6 Focus Shifting：它已經知道能打、
// 而點到的牌中還有不知道能打的，焦點移到最左邊那張不知道能打的牌（全都知道能打就是 Burn Clue，仍是最左邊）。
// cards 是點到的舊牌。例如紅色點到 r1（被 play clue 的焦點）與另一張紅牌，再次紅色時焦點是後者。
func (s *state) tempoFocus(R int, cards []int) int {
	first := -1
	for _, id := range s.hands[R] {
		if !slices.Contains(cards, id) {
			continue
		}
		if first < 0 {
			first = id
		}
		if !s.knows(lvTempo) || !s.publicKnown(id) {
			return id
		}
	}
	return first
}

// tempoOverTrash（Level 6）：「同時像 Tempo Clue 又像 Trash Chop Move」的提示當成 Tempo Clue（比較自然的一手）：
// 點到的新牌依提示本身全是大家都知道的垃圾，而點到的已提示牌中有還不知道能打的。成立時焦點改成那張舊牌，
// 新牌是已知垃圾（it.trashNew）。挑戰題 Tempo or Trash?：紅色點到新的垃圾 r1 與被 5 Save 的 r5，
// 是 r5 的 TCCM（只 Chop Move chop），不是 Trash Chop Move（Chop Move 新牌右邊全部）。
func (s *state) tempoOverTrash(R int, touched, fresh []int, it *interp) bool {
	if !s.knows(lvTempo) || len(fresh) == 0 {
		return false
	}
	var old []int
	for _, y := range touched {
		n := s.notes[y]
		switch {
		case !n.clued:
			// 依提示本身是垃圾：包括這個提示補完資訊的另一張舊牌就是那張（例如紅色把「5」補成 r5，新的紅牌就不是 r5）
			for _, i := range it.explicit[y].list() {
				if !s.trash(i) && !s.knownElsewhere(i, y) && !slices.ContainsFunc(touched, func(z int) bool {
					t, ok := it.explicit[z].single()
					return z != y && ok && t == i
				}) {
					return false
				}
			}
		case !n.bad:
			old = append(old, y)
		}
	}
	if !slices.ContainsFunc(old, func(y int) bool { return !s.publicKnown(y) }) {
		return false
	}
	it.tempo, it.trashNew, it.focus, it.chopFocus = true, slices.DeleteFunc(slices.Clone(touched), func(y int) bool { return slices.Contains(old, y) }), s.tempoFocus(R, old), false
	return true
}

// tempoTarget：Tempo Clue 的焦點大家都知道輪到時能打、而且只靠這一張：接收者角度的每種 play 解讀，中間缺的牌都已在別處、
// 大家都知道是那張（publicPoss 確定；挑戰題 Tempo or Trash? 的 r5：r3、r4 都已被提示）。中間缺的牌要 prompt 別人的
// 已提示牌（它還不確定是那張）或 finesse 時，這個提示會讓兩張以上的牌打出，是有價值的 Tempo Clue（原文第 1 種），不 Chop Move。
// 回傳焦點可能的牌面。
func (s *state) tempoTarget(it *interp) (idSet, bool) {
	var ts idSet
	truth, seen := s.see(s.me, it.focus)
	for _, c := range it.cands {
		// 要我自己盲打的解讀：我看不到自己的牌才會有它，別人都看得到我的牌；我看得到焦點、它不是那張時就不成立
		if seen && c.id != truth && slices.ContainsFunc(c.deps, func(d dep) bool { return d.player == s.me && d.finesse }) {
			continue
		}
		if c.save {
			return 0, false
		}
		t := c.id
		for r := s.stacks[t.color()] + 1; r < t.rank(); r++ {
			if !s.publicElsewhere(identOf(t.color(), r), it.focus) {
				return 0, false
			}
		}
		ts |= setOf(t)
	}
	return ts, ts != 0
}

// tempoValuable（Level 6 The Valuable Tempo Clue）：大家都知道這個 Tempo Clue 有價值（焦點的可能牌面是 ts）——
//  1. 讓兩張以上已被提示的牌能打：另一張被點到的已提示牌因此確定現在就能打（挑戰題 Filling in, I 的 b3、g3）；
//     中間缺的牌要靠這個提示 prompt / finesse 出來的情況，tempoTarget 已排除；
//  2. 焦點不是 5，而且現在 prompt 它的話會 prompt 到接收者另一張已提示的牌（依 prompt 的順序，大家的筆記上可能是它）。
//
// 第 3 種「打出它會讓鎖手的人解鎖」：接收者鎖手時沒有 chop 可以 Chop Move，tempoChopMove 本來就不成立。
// 提示當下就判斷（原文：只看提示後的那一回合，免得各家不同步）。
func (s *state) tempoValuable(R int, it *interp, touched []int, ts idSet) bool {
	if s.tempoExtraPlays(it, touched) > 0 {
		return true
	}
	for _, t := range ts.list() {
		if t.rank() == game.MaxRank {
			return false
		}
		blocked := false
		for _, id := range s.promptOrder(R) {
			n := s.notes[id]
			if n.clued && !s.publicKnown(id) && s.publicPoss(id).has(t) {
				blocked = id != it.focus
				break
			}
		}
		if !blocked {
			return false
		}
	}
	return true
}

// tempoExtraPlays：Tempo Clue 點到的其他已提示牌中，提示前大家不知道能打、提示後確定現在就能打的張數。
func (s *state) tempoExtraPlays(it *interp, touched []int) int {
	n := 0
	for _, y := range touched {
		if y == it.focus || !s.notes[y].clued || s.publicKnown(y) {
			continue
		}
		if p := s.publicPoss(y) & it.explicit[y]; p != 0 && !slices.ContainsFunc(p.list(), func(i ident) bool { return !s.playableNow(i) }) {
			n++
		}
	}
	return n
}

// stallSituation（Level 6 The Tempo Clue Stall；細節見 Level 9 的 Stall Table）：大家都看得出 giver 這一手是在拖延：
// 提示滿 8、他鎖手（不能棄牌），或終局（pace ≤ 1 或牌堆只剩 1 張，Level 8 的被迫拖延）。
// 這時沒有價值的 Tempo Clue 只是拖延；其他時候（包括 Early Game）就是 Tempo Clue Chop Move。
func (s *state) stallSituation(giver int) bool {
	return s.mustStall(giver) || s.deck > 0 && (s.deck <= 1 || s.publicPace() <= 1)
}

// publicPace 是大家都算得出的 pace（Level 8）：分數 + 牌堆張數 + 人數 − 還拿得到的最高分。
func (s *state) publicPace() int {
	best, score := 0, 0
	for _, c := range s.colors {
		score += int(s.stacks[c])
		for r := game.Rank(1); r <= game.MaxRank; r++ {
			if r > s.stacks[c] && s.discarded[identOf(c, r)] >= identOf(c, r).copies() {
				break
			}
			best++
		}
	}
	return score + s.deck + s.n - best
}

// tempoChopMove（Level 6 The Tempo Clue Chop Move）：沒有價值的 Tempo Clue 不在拖延的情況下給，一定另有用意——
// 除了讓焦點打出，還 Chop Move 接收者 chop 上的牌。點到的牌都要是之前就被提示過的（被 finesse 的不算，
// 不是每個人都知道）、焦點本來大家不知道能打（否則是 Burn Clue）、輪到時能打而且只靠這一張（tempoTarget）。
// 有價值的 Tempo Clue 記在 it.valuable（和一般 play clue 一樣，可以隨時給）。
func (s *state) tempoChopMove(giver, R int, it *interp, touched []int) {
	if !s.knows(lvTempo) || !it.tempo || s.publicKnown(it.focus) ||
		slices.ContainsFunc(touched, func(y int) bool { return !s.notes[y].clued && !slices.Contains(it.trashNew, y) }) {
		return
	}
	ts, ok := s.tempoTarget(it)
	if !ok {
		return
	}
	if s.tempoValuable(R, it, touched, ts) {
		it.valuable = true
		return
	}
	chop := s.publicChop(R, touched)
	if chop < 0 || s.stallSituation(giver) {
		return
	}
	it.cmKind, it.cm = cmTempo, []int{chop}
}

// publicChop：R 最右邊沒被提示、沒被 Chop Move、這次也沒被點到的牌（被 finesse 的牌不是每個人都知道，不跳過）。
func (s *state) publicChop(R int, touched []int) int {
	h := s.hands[R]
	for i := len(h) - 1; i >= 0; i-- {
		if n := s.notes[h[i]]; !n.clued && !n.cm && !slices.Contains(touched, h[i]) {
			return h[i]
		}
	}
	return -1
}

// oneAwayValuable（Level 6 The Value of One-Away-From-Playable Cards）：p 的牌 id 是差一張就能打的牌，
// 而且是唯一看得到的那張（不在別人手上、沒有已被拿到的同一張，也不可能是我不確定的已提示牌）。
// 照慣例不能直接用 play / save clue 救它，但值得用 Chop Move 間接保護，或用 Discard Modulation 讓它晚點被棄。
func (s *state) oneAwayValuable(p, id int) bool {
	t := identOfCard(s.cards[id])
	if s.trash(t) || t.rank() != s.stacks[t.color()]+2 || s.gottenAs(s.me, s.me, t, id) || s.maybeMine(t) {
		return false
	}
	for q, h := range s.hands {
		for _, o := range h {
			if q != s.me && o != id && identOfCard(s.cards[o]) == t {
				return false
			}
		}
	}
	return true
}

// discardModulation（Level 6 Discard Modulation）：我要給的 play clue，下一位也能給出同樣的結果（接收者不是他、
// 計畫也不用到他或我的牌），而他的 chop 是高價值、又不能直接救的牌（差一張就能打，oneAwayValuable），
// 他也沒有別的事可做（會棄 chop）：我棄牌、讓他給這個提示，他的 chop 就多保住一陣子。
// Early Game 必須先給完 play / save clue，不這樣做。
func (s *state) discardModulation(ev clueEval, hints int) bool {
	if !s.knows(lvTempo) || !s.midGame || hints >= game.MaxHints || ev.kind != kindPlay {
		return false
	}
	N := (s.me + 1) % s.n
	if ev.target == N || slices.ContainsFunc(ev.plan, func(d dep) bool { return d.player == N || d.player == s.me }) {
		return false
	}
	c := s.chop(N)
	return c >= 0 && !s.keepsChop(N) && !s.needsSave(N, c) && s.oneAwayValuable(N, c) && s.chop(s.me) >= 0
}
