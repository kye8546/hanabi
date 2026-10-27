// Level 8（docs/level-8.mdx）：終局（End-Game）——No Chop Moves in the End-Game（endGame）；
// 被迫的終局拖延（使用者要求所有等級都有）：pace、mustBurn、burnClue。

package hgroup

import (
	"hanabi/game"
)

// endGame（Level 8 The End-Game）：pace 小於人數就是終局（牌堆空了也算），大家都算得出（publicPace）。
// 終局不 Chop Move（No Chop Moves in the End-Game：沒有「之後」可言，不必替之後保留牌）：像 Chop Move 的提示
// 改當成下一個最簡單的解讀，通常是拖延提示（findChopMove、tempoChopMove）；Order Chop Move 與
// Scream / Shout Discard Chop Move 也不算（hanabi-bot 同樣在終局關掉 Last Resorts 的棄牌；挑戰題 3 Cards Left：
// 提示 0 個、Alice 有已知能打的 r4 卻棄牌，是為了換提示讓 Bob 的三張 5 來得及打，不是 Scream Discard）。
func (s *state) endGame() bool {
	return s.knows(lvEndGame) && (s.deck == 0 || s.publicPace() < s.n)
}

// pace = 目前分數 + 牌堆張數 + 人數 − 這局還拿得到的最高分（H-Group Level 8）。
// 每次棄牌 pace 減 1；pace < 0 就不可能拿到最高分。
func (s *state) pace(v game.PlayerView) int {
	best := 0
	for _, c := range s.colors {
		for r := game.Rank(1); r <= game.MaxRank; r++ {
			if r > s.stacks[c] && s.discarded[identOf(c, r)] >= identOf(c, r).copies() {
				break
			}
			best++
		}
	}
	return v.Score() + v.DeckSize + s.n - best
}

// mustBurn：牌堆還有牌、有提示可用，而且這次棄牌會讓最高分拿不到時，必須拖延：
//   - pace 0：再棄一張就不可能拿到最高分。
//   - pace 1，有隊友手上沒有能打的牌，而我的牌可能能打（Pace +1 Rule：讓他去做最後一次棄牌）。
//   - 牌堆只剩 1 張，而某位隊友手上有 2 張以上還需要打的牌（他只剩一回合打不完）。
func (s *state) mustBurn(v game.PlayerView) bool {
	if v.DeckSize == 0 || v.Hints == 0 {
		return false
	}
	switch s.pace(v) {
	case 0:
		return true
	case 1:
		if s.teammateIdle() && s.mightPlay() {
			return true
		}
	}
	// 隊友手上還需要打的牌比他剩下的回合多（一回合只能打一張），而我不抽牌能讓他多一回合，就拖延
	// （Level 8：Team Distribution / Burning）。牌堆只剩 1 張時，我一棄牌每人就只剩一回合，
	// 任何握有 2 張以上還需要打的牌的隊友都打不完（之後的人也可能接著拖延），一律拖延。
	for k := 1; k < s.n; k++ {
		p := (s.me + k) % s.n
		var need idSet
		for _, id := range s.hands[p] {
			if t := identOfCard(s.cards[id]); !s.trash(t) {
				need |= setOf(t)
			}
		}
		left := s.turnsLeft(k, v.DeckSize-1)
		if need.count() > left && (v.DeckSize == 1 || s.turnsLeft(k, v.DeckSize) > left) {
			return true
		}
	}
	return false
}

// turnsLeft：我行動後牌堆剩 deck 張，之後每回合都抽牌的話，我後面第 k 位還有幾回合
// （再 deck 回合抽完，接著每人最後一回合，共 deck + n 回合）。
func (s *state) turnsLeft(k, deck int) int {
	if k > deck+s.n {
		return 0
	}
	return (deck+s.n-k)/s.n + 1
}

// teammateIdle：有隊友手上（我看得到的）沒有任何現在能打的牌。
func (s *state) teammateIdle() bool {
	for p, h := range s.hands {
		if p == s.me {
			continue
		}
		idle := true
		for _, id := range h {
			if s.playableNow(identOfCard(s.cards[id])) {
				idle = false
			}
		}
		if idle {
			return true
		}
	}
	return false
}

// mightPlay：我手上有牌可能是現在能打的牌。
func (s *state) mightPlay() bool {
	for _, id := range s.hands[s.me] {
		for _, i := range s.myPoss(id).list() {
			if s.playableNow(i) {
				return true
			}
		}
	}
	return false
}

// burnClue 挑一個不會造成誤解的拖延提示：先選讓對方確定某張已提示牌能打的重複提示，
// 再選對方會正確理解、也不點到垃圾的提示。
func (s *state) burnClue(evs []clueEval) (clueEval, bool) {
	if ev, ok := best(evs, func(e clueEval) bool { return e.kind == kindTempo && e.knowsPlay && !e.misleads }); ok {
		return ev, true
	}
	return best(evs, func(e clueEval) bool { return e.understood && e.bad == 0 && !e.misleads })
}
