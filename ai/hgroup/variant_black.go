// Black 變體的慣例（https://hanabi.github.io/variant-specific/black ，所有等級都用）：
// 黑色（每個數字只有 1 張、全都是 critical）的 save clue——黑 2、黑 5 用數字救，黑 3、黑 4 用顏色救（縮小被救的是哪張）；
// 用另一種提示點到 chop 上的它們，不符合例外時就是 play clue（接收者照 play clue 找連接牌，找不到就排除那張）。
// saveCands（interpret.go）呼叫這裡的判斷。

package hgroup

// blackColorSave：黑色提示能不能是救黑 2 / 黑 5 的 save clue。例外只算大家都看得出的兩種：
// 點到兩張以上之前沒被提示的牌（都是黑色），或補上一張已提示牌的資訊（filled in an ancillary card）。
// 「為了不違反 Good Touch」「給了重要的負向資訊」要看接收者看不到的牌、或靠主觀判斷，接收者無從確認，不算
// （AI 也就不會那樣用黑色救，改用數字）。
func (s *state) blackColorSave(it *interp) bool {
	if len(it.newCards) >= 2 {
		return true
	}
	for _, id := range s.touchedNow {
		if n := s.notes[id]; n.clued && it.explicit[id] != n.explicit {
			return true
		}
	}
	return false
}

// florratSave：數字 3 / 4 能不能是救黑 3 / 黑 4 的 save clue（Florrat Save）：還點到另一張新牌，
// 依提示本身它可能是 critical（只用公開的棄牌堆判斷，大家看法一致）。
func (s *state) florratSave(t ident, it *interp) bool {
	for _, id := range it.newCards {
		if id == it.focus {
			continue
		}
		for _, x := range (it.explicit[id] & s.all).list() {
			if x != t && s.critical(x) {
				return true
			}
		}
	}
	return false
}
