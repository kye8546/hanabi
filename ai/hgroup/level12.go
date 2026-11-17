package hgroup

import (
	"slices"

	"hanabi/game"
)

// Level 12 Context（原文 docs/level-12.mdx）。
// Assuming Asymmetric Information、Duplication Responsibility（所有等級都用，見 save.go 的 saveIsOthersJob）、
// Selfish Clue / Selfish Finesse（一般的 delayed play clue / Reverse Finesse 已經讀得懂）、Ego Clue（AI 本來就先打確定的牌）
// 不需要另外的程式；這裡是改變提示意思的兩種 Context Clue。

// earlyEndRec：結束 Early Game 的那次棄牌。
type earlyEndRec struct {
	player int // 棄牌的人
	hints  int // 棄牌前的提示數
}

// focusInverted：Level 12 Focus Inversion。點到 chop 和其他新牌的提示，照提示本身的資訊（含之前的負向資訊）chop 不可能是
// 不用盲打就能打的牌（直接或 prompt）、也不是 save clue 時，焦點改成最左邊的新牌（原文：「the clue is meant to be a Play Clue
// on the leftmost card instead of the chop card」；Bob 的 chop 有「不是 1」的負向資訊，藍色點到 slot 1 與 chop → slot 1 是 b1）。
// chop 要靠 finesse 才接得上的解讀不擋：原文說數字提示的 Self-Finesse 優先於 Focus Inversion「is a specific case where Occam's
// Razor does not apply」，其他情況照 Occam's Razor，要盲打的解讀輸給不用盲打的。這也是必要的——給提示者與接收者之間的人
// 看不到自己的牌，排除不了 chop 是「finesse 我」的解讀，以前只擋掉接收者的 Self-Finesse 時，他們常照它盲打而打錯
// （3000 局 3–5 人局閃電多一倍）。所以最左邊那張也要有不用盲打的解讀才反轉。
// 數字提示要接收者 Self-Finesse 的解讀照樣優先（原文：「Self-Finesses with color are nonsensical」，數字的很常見）。
// 只看提示本身的資訊（explicit），不看接收者私下推出的筆記：原文 Assuming Asymmetric Information——不預設別人有不對稱的資訊。
func (s *state) focusInverted(giver, R int, h game.Hint, it *interp, fresh []int) bool {
	if !s.knows(lvFocusInversion) || !it.chopFocus || it.anySave || len(fresh) < 2 {
		return false
	}
	blind := func(c cand) bool { return slices.ContainsFunc(c.deps, func(d dep) bool { return d.finesse }) }
	selfFin := func(c cand) bool {
		return slices.ContainsFunc(c.deps, func(d dep) bool { return d.player == R && d.finesse })
	}
	base := it.explicit[it.focus]
	for _, c := range s.playCands(R, giver, R, it.focus, base) {
		if !blind(c) || h.Kind == game.HintRank && selfFin(c) {
			return false
		}
	}
	if len(s.saveCands(R, giver, R, it, h, base)) > 0 {
		return false
	}
	left := fresh[0]
	return slices.ContainsFunc(s.playCands(R, giver, R, left, it.explicit[left]), func(c cand) bool { return !blind(c) })
}

// stale1s：Level 12 The Stale 1's Clue。有人棄牌結束 Early Game（還有提示可給，表示沒有該給的 play / save clue——
// Early Game 必須先給完），下一位緊接著用「1」點到另一位的兩張以上的新 1：都是好的話棄牌的人必須先給，所以其中有壞的。
// 照慣例是照順序會先打的那張（Level 3 的順序；原文：「the oldest (rightmost) 1 is bad, because that would be the one that she
// would ordinarily play first」），回傳它與下一張（新的焦點）。棄牌的人就是接收者時他看不到自己的牌，不這樣推論。
func (s *state) stale1s(giver, R int, h game.Hint, it *interp, fresh []int) (bad, next int, ok bool) {
	e := s.earlyEnd
	if !s.knows(lvStale1s) || e == nil || e.player == R || e.hints <= 0 || h.Kind != game.HintRank || h.Rank != 1 || len(fresh) < 2 {
		return -1, -1, false
	}
	order := s.onesOrder(R, fresh)
	return order[0], order[1], true
}

// onesOrder：Level 3 Playing Multiple 1's 的順序——chop > 新抽的 1 由左而右 > 起手牌的 1 由右而左。
func (s *state) onesOrder(R int, ids []int) []int {
	chop := s.chop(R)
	var out, start []int
	if slices.Contains(ids, chop) {
		out = append(out, chop)
	}
	for _, id := range ids {
		switch {
		case id == chop:
		case s.fresh(id):
			out = append(out, id)
		default:
			start = append(start, id)
		}
	}
	slices.Reverse(start)
	return append(out, start...)
}
