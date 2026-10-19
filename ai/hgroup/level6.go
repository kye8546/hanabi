// Level 6（docs/level-6.mdx）：Tempo Clues——Focus Shifting、Valuable Tempo Clue、Tempo Clue Chop Move（TCCM），
// 以及 TCCM 優先於 Trash Chop Move。

package hgroup

import (
	"slices"

	"hanabi/game"
)

// Level 6 的判斷會決定要不要 Chop Move，必須大家看法一致（和 Level 4 一樣）：一律只用公開的資訊——
// 哪些牌被提示過（clued，不含只有部分人知道的 finesse）、提示本身的資訊（explicit）、煙火與棄牌。
// 持有者私下的推論、各家的共同筆記都可能不同（實測用筆記判斷時，5 人局四成的 TCCM 各家看法不一）。

// clueKnown：只看提示資訊 ex（再扣掉已打出、打不出去的牌面），這張牌確定現在就能打。
func (s *state) clueKnown(ex idSet) bool {
	p := ex & s.useful()
	return p != 0 && !slices.ContainsFunc(p.list(), func(i ident) bool { return !s.playableNow(i) })
}

// tempoFocus：只點到已被提示的牌（Tempo Clue）時的焦點是最左邊那張。Level 6 Focus Shifting：它已經知道能打、
// 而點到的牌中還有不知道能打的，焦點移到最左邊那張不知道能打的牌（全都知道能打就是 Burn Clue，仍是最左邊）。
// cards 是點到的舊牌。「知道能打」只看提示資訊（clueKnown）。
func (s *state) tempoFocus(R int, cards []int) int {
	first := -1
	for _, id := range s.hands[R] {
		if !slices.Contains(cards, id) {
			continue
		}
		if first < 0 {
			first = id
		}
		if !s.knows(lvTempo) || !s.clueKnown(s.notes[id].explicit) {
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
	if !slices.ContainsFunc(old, func(y int) bool { return !s.clueKnown(s.notes[y].explicit) }) {
		return false
	}
	it.tempo, it.trashNew, it.focus, it.chopFocus = true, slices.DeleteFunc(slices.Clone(touched), func(y int) bool { return slices.Contains(old, y) }), s.tempoFocus(R, old), false
	return true
}

// tempoTarget：Tempo Clue 的焦點依提示資訊確定是哪張牌 t，而且大家都知道它輪到時能打：現在就能打，
// 或中間缺的每一張都在別人手上、提示資訊就確定是那張的已提示牌（挑戰題 Tempo or Trash? 的 r5：r3、r4 都已被提示）。
// 不確定的話（例如要 prompt 別人不知道是哪張的已提示牌），各家對「讓幾張牌能打」的看法可能不同，不算 TCCM。
func (s *state) tempoTarget(it *interp) (ident, bool) {
	t, ok := (it.explicit[it.focus] & s.useful()).single()
	if !ok {
		return 0, false
	}
	for r := s.stacks[t.color()] + 1; r < t.rank(); r++ {
		if !s.knownElsewhere(identOf(t.color(), r), it.focus) {
			return 0, false
		}
	}
	return t, true
}

// tempoValuable（Level 6 The Valuable Tempo Clue）：大家都知道這個 Tempo Clue 有價值（焦點 t 是 tempoTarget）——
//  1. 讓兩張以上已被提示的牌能打：另一張被點到的已提示牌因此確定現在就能打（挑戰題 Filling in, I 的 b3、g3）；
//  2. 焦點不是 5，而且現在 prompt t 的話會 prompt 到接收者另一張已提示的牌（依 prompt 的順序，提示資訊上可能是 t）。
//
// 第 3 種「打出它會讓鎖手的人解鎖」：接收者鎖手時沒有 chop 可以 Chop Move，tempoChopMove 本來就不成立。
// 提示當下就判斷（原文：只看提示後的那一回合，免得各家不同步）。
func (s *state) tempoValuable(R int, it *interp, touched []int, t ident) bool {
	if s.tempoExtraPlays(it, touched) > 0 {
		return true
	}
	if t.rank() == game.MaxRank {
		return false
	}
	for _, id := range s.promptOrder(R) {
		n := s.notes[id]
		if n.clued && !n.bad && !s.clueKnown(n.explicit) && (n.explicit & s.useful()).has(t) {
			return id != it.focus
		}
	}
	return false
}

// tempoExtraPlays：Tempo Clue 點到的其他已提示牌中，提示前不知道能打、提示後依提示資訊確定現在就能打的張數。
func (s *state) tempoExtraPlays(it *interp, touched []int) int {
	n := 0
	for _, y := range touched {
		if y != it.focus && s.notes[y].clued && !s.notes[y].bad && !s.clueKnown(s.notes[y].explicit) && s.clueKnown(it.explicit[y]) {
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
// 不是每個人都知道）、焦點本來不知道能打（否則是 Burn Clue）、而且依提示資訊確定是哪張（tempoTarget）。
// 有價值的 Tempo Clue 記在 it.valuable（和一般 play clue 一樣，可以隨時給）。
func (s *state) tempoChopMove(giver, R int, it *interp, touched []int) {
	if !s.knows(lvTempo) || !it.tempo || s.clueKnown(s.notes[it.focus].explicit) ||
		slices.ContainsFunc(touched, func(y int) bool { return !s.notes[y].clued && !slices.Contains(it.trashNew, y) }) {
		return
	}
	t, ok := s.tempoTarget(it)
	if !ok {
		return
	}
	if s.tempoValuable(R, it, touched, t) {
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
