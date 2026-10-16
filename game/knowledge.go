package game

// Knowledge 是持有者根據收到的提示，對自己某張手牌的認知。
// 用 bitmask 記錄「還可能是」哪些顏色與數字，因此負向資訊（例如「不是紅色」）也會保留。
type Knowledge struct {
	Colors uint8 // 第 i 位元 = 可能是 AllColors[i]
	Ranks  uint8 // 第 r-1 位元 = 可能是數字 r
	Clued  bool  // 曾被任何提示「點到」（正向觸及），進階策略常用

	// 只記正向提示（被點到）的值，不含「不是 X」的推論；介面隱藏反向資訊時使用
	HintedColors uint8
	HintedRanks  uint8
}

// NewKnowledge 回傳變體 va 中一張剛抽到、毫無資訊的牌的認知。
func NewKnowledge(va Variant) Knowledge {
	k := Knowledge{Ranks: 1<<MaxRank - 1}
	for _, c := range va.Colors() {
		k.Colors |= 1 << c
	}
	return k
}

func (k Knowledge) CanBeColor(c Color) bool { return k.Colors&(1<<c) != 0 }
func (k Knowledge) CanBeRank(r Rank) bool   { return k.Ranks&(1<<(r-1)) != 0 }

// HintedColor 回傳這張牌曾被正向提示點到的顏色；被兩種以上的顏色點到就一定是彩虹色。
func (k Knowledge) HintedColor() (Color, bool) {
	if k.HintedColors&(k.HintedColors-1) != 0 {
		return Rainbow, true
	}
	for _, c := range AllColors {
		if k.HintedColors&(1<<c) != 0 {
			return c, true
		}
	}
	return 0, false
}

// HintedRank 回傳這張牌曾被正向提示點到的數字。
func (k Knowledge) HintedRank() (Rank, bool) {
	for r := Rank(1); r <= MaxRank; r++ {
		if k.HintedRanks&(1<<(r-1)) != 0 {
			return r, true
		}
	}
	return 0, false
}

// KnownColor 在只剩一種可能顏色時回傳該顏色。
func (k Knowledge) KnownColor() (Color, bool) {
	cs := k.PossibleColors()
	if len(cs) == 1 {
		return cs[0], true
	}
	return 0, false
}

// KnownRank 在只剩一種可能數字時回傳該數字。
func (k Knowledge) KnownRank() (Rank, bool) {
	rs := k.PossibleRanks()
	if len(rs) == 1 {
		return rs[0], true
	}
	return 0, false
}

func (k Knowledge) PossibleColors() []Color {
	var out []Color
	for _, c := range AllColors {
		if k.CanBeColor(c) {
			out = append(out, c)
		}
	}
	return out
}

func (k Knowledge) PossibleRanks() []Rank {
	var out []Rank
	for r := Rank(1); r <= MaxRank; r++ {
		if k.CanBeRank(r) {
			out = append(out, r)
		}
	}
	return out
}

// apply 依一次提示更新認知：被點到的牌收斂成該值，沒被點到的牌排除該值。
// 顏色提示也會點到彩虹色的牌：被點到是「該顏色或彩虹色」，沒被點到則兩者都不是。
func (k *Knowledge) apply(h Hint, touched bool) {
	switch h.Kind {
	case HintColor:
		m := uint8(1<<h.Color | 1<<Rainbow)
		if touched {
			k.Colors &= m
			k.HintedColors |= 1 << h.Color
		} else {
			k.Colors &^= m
		}
	case HintRank:
		if touched {
			k.Ranks = 1 << (h.Rank - 1)
			k.HintedRanks |= 1 << (h.Rank - 1)
		} else {
			k.Ranks &^= 1 << (h.Rank - 1)
		}
	}
	if touched {
		k.Clued = true
	}
}
