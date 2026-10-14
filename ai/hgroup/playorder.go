// 出牌順序：手上有多張確定能打的牌時先打哪張（Level 3 Playing Multiple 1's 的順序與 Level 25 Priority 的表），
// 也用來推測別人下一手打哪張。

package hgroup

import (
	"slices"

	"hanabi/game"
)

// bestPlay 選一張確定能打的牌（playOrder 的第一張）；沒有則回傳 -1。
func (s *state) bestPlay() int {
	if o := s.playOrder(s.me); len(o) > 0 {
		return slices.Index(s.hands[s.me], o[0])
	}
	return -1
}

// knownPoss：p 自己對他的牌 id 知道的可能（我：我的推論；別人：大家都推得出的共同筆記）。
func (s *state) knownPoss(p, id int) idSet {
	if p == s.me {
		return s.myPoss(id)
	}
	return s.possOf(id)
}

// playOrder 回傳 p 確定能打的牌，依他會打出的順序（從他自己的角度）。順序照 Level 25 Priority 的表
// （在那之前不當訊號，只是好的出牌順序；盲打在 decide 第 1 步已優先）：能接上別人已被拿到的牌 >
// 能接上自己手上已被拿到的牌 > 5 > 數字小的 > 同數字時提示給他的焦點牌（Single Card Focus：play clue 的意思是
// 打出焦點牌，例如「1」同時點到 chop 和別張時先打 chop）> 最左邊。
// Level 3 Playing Multiple 1's：1 的順序是 chop 焦點 > 新抽的 1（由左而右）> 起手牌的 1（由右而左）。
func (s *state) playOrder(p int) []int {
	type kc struct {
		id, key, sub int
		one          bool
	}
	var cs []kc
	h := s.hands[p]
	for slot, id := range h {
		if s.notes[id].bad {
			continue
		}
		poss := s.knownPoss(p, id)
		if poss == 0 || slices.ContainsFunc(poss.list(), func(i ident) bool { return !s.playableNow(i) }) {
			continue
		}
		r := game.MaxRank
		for _, i := range poss.list() {
			r = min(r, i.rank())
		}
		key := 2 * int(r)
		if r != game.MaxRank {
			key += 100
		}
		if !s.leadsInto(p, poss, true) {
			key += 10000
			if !s.leadsInto(p, poss, false) {
				key += 1000
			}
		}
		sub := slot
		one := s.knows(lvMultipleOnes) && poss&^rankSet(1) == 0
		switch {
		case one:
			switch {
			case s.chopFocused(id):
				sub = 0
			case !s.notes[id].start:
				sub = 1 + slot
			default:
				sub = 50 + len(h) - slot
			}
		case !s.isFocus(id):
			sub += 50
		}
		cs = append(cs, kc{id, key, sub, one})
	}
	// Level 3 的 1 的順序是約定（Stop Clue 等都靠它），Level 25 的表不能打亂它：被提示的 1 一律照 1 的順序，
	// 整組用其中最優先的那張和其他牌比（例如確定的 b1 能接上自己的 b2，也不會跳到新抽的 1 前面）
	best1 := -1
	for _, c := range cs {
		if c.one && (best1 < 0 || c.key < best1) {
			best1 = c.key
		}
	}
	for i := range cs {
		if cs[i].one {
			cs[i].key = best1
		}
		cs[i].key = 100*cs[i].key + cs[i].sub
	}
	slices.SortStableFunc(cs, func(a, b kc) int { return a.key - b.key })
	out := make([]int, len(cs))
	for i, c := range cs {
		out[i] = c.id
	}
	return out
}

// nextPlay：p 下一手會打的牌（從他自己的角度）；沒有則回傳 -1。
func (s *state) nextPlay(p int) int {
	if o := s.playOrder(p); len(o) > 0 {
		return o[0]
	}
	return -1
}

// rankSet 回傳數字 r 的所有牌面。
func rankSet(r game.Rank) idSet { return hintSet(game.Hint{Kind: game.HintRank, Rank: r}) }

// chopFocused：這張牌是某次給它持有者、焦點在 chop 上的提示的焦點。
func (s *state) chopFocused(id int) bool {
	for _, rec := range s.recs {
		if rec.focus == id && rec.chop && rec.target == s.holder(id) {
			return true
		}
	}
	return false
}

// leadsInto：（p 的角度）牌打出後能接上已被拿到的牌——那張的數字正好大 1，打出後它就能接著打
// （others 為 true 看別人手上，否則看 p 自己手上；Level 25 的理由是避免損失 Tempo，中間還缺牌就沒有幫助）。
// 不確定是哪張時，每一種可能都要能接上（Level 25：對每種可能套用表）。
func (s *state) leadsInto(p int, poss idSet, others bool) bool {
	for _, t := range poss.list() {
		if !s.leadsInto1(p, t, others) {
			return false
		}
	}
	return poss != 0
}

func (s *state) leadsInto1(p int, t ident, others bool) bool {
	for q, h := range s.hands {
		if (q != p) != others {
			continue
		}
		for _, id := range h {
			if !s.live(id) {
				continue
			}
			var c ident
			if v, ok := s.see(s.me, id); ok && q != p {
				c = v
			} else if v, ok := s.knownPoss(q, id).single(); ok && (q == p || q == s.me) {
				c = v // p 自己的牌只看他知道的；我的牌我看不到，用我的推論
			} else {
				continue
			}
			if c.color() == t.color() && c.rank() == t.rank()+1 {
				return true
			}
		}
	}
	return false
}

// isFocus：這張牌是某次給它持有者的提示的焦點。
func (s *state) isFocus(id int) bool {
	for _, rec := range s.recs {
		if rec.focus == id && rec.target == s.holder(id) {
			return true
		}
	}
	return false
}

// likelyPlay 回傳能打機率至少 threshold 的被提示牌（依還沒看到的張數估計）；沒有則回傳 -1。
func (s *state) likelyPlay(threshold float64) int {
	bestSlot, bestP := -1, threshold
	for slot, id := range s.hands[s.me] {
		if !s.gotten(id) {
			continue
		}
		ok, all := 0, 0
		for _, i := range s.myPoss(id).list() {
			n := max(s.remaining(i), 0)
			all += n
			if s.playableNow(i) {
				ok += n
			}
		}
		if all > 0 {
			if p := float64(ok) / float64(all); p >= bestP {
				bestSlot, bestP = slot, p
			}
		}
	}
	return bestSlot
}
