// Level 5（docs/level-5.mdx）中自成一塊的部分：Urgency Principle 的例外（可以延後盲打的緊急行動）、
// Queued Finesse 的盲打順序。Layered / Clandestine / Hidden Finesse 是連接牌推論的分支，在 connect.go
// （findFinesse 的 layered、findPrompt 的 hiddenFinesse、stepDep 的 layerSlot）；Ambiguous Finesse 在 level2.go 的 passMeansMe。
// 彩虹等多色變體的 Prompt 規則要等加入那些變體後再補。

package hgroup

import (
	"slices"

	"hanabi/game"
)

// deferrable（Level 5 Urgency Principle 的例外）：e.Player 手上有該盲打的 finesse，卻給了這個提示。
// 點到接收者 chop 的提示（救 5、給快被棄掉的牌 play clue）是「really important」的事，可以延後盲打——
// 大家再給他一回合，不當成他沒接 finesse。其他提示（例如 Bob is Lazy 挑戰題的 1-for-1）不算。
// 原文另一個例外「給只有他能給的 finesse」要靠各人自己對那個提示的解讀（看得到的牌不同，結論可能不同），
// 會讓各家對 finesse 有沒有兌現的筆記不一致，不採用（AI 自己也只會為了救 chop 延後盲打）。
func (s *state) deferrable(e game.Event) bool {
	if !s.knows(lvUrgency) || e.Type != game.EvHint {
		return false
	}
	c := s.chop(e.Target)
	return c >= 0 && slices.Contains(e.Touched, slices.Index(s.hands[e.Target], c))
}

// stepDeferFinesse（Level 5 Urgency Principle 的例外）：我有該盲打的 finesse，但下一位會棄掉 chop 上必須保留的牌：
// 先救（點到他的 chop，大家看得出是可以延後盲打的緊急行動），下一手再盲打。
func (s *state) stepDeferFinesse(t *turn) (game.Action, string, bool) {
	if !s.knows(lvUrgency) || !slices.ContainsFunc(s.hands[s.me], func(id int) bool { return s.notes[id].finessed && s.knownPlayable(id) }) {
		return game.Action{}, "", false
	}
	a, why, ok := s.urgentSave(t.clues(s), t.v.Hints, true)
	if !ok || a.Type != game.ActHint {
		return game.Action{}, "", false
	}
	chop := s.chop(a.Target)
	if !slices.ContainsFunc(t.clues(s), func(e clueEval) bool { return e.act == a && slices.Contains(e.touched, chop) }) {
		return game.Action{}, "", false
	}
	return a, why + "（Urgency Principle 的例外：finesse 晚一手再盲打）", true
}

// blindPlayOrder（Level 5 The Queued Finesse）：被 finesse 的牌照提示的先後盲打（同一個提示的由左而右）。
// Level 2–4 不會有排隊的 finesse，照位置由左而右。
func (s *state) blindPlayOrder() []int {
	var out []int
	for _, id := range s.hands[s.me] {
		if s.notes[id].finessed && s.knownPlayable(id) {
			out = append(out, id)
		}
	}
	if s.knows(lvQueuedFinesse) {
		seq := func(id int) int {
			if r := s.notes[id].finRec; r != nil {
				return r.seq
			}
			return s.notes[id].seq
		}
		slices.SortStableFunc(out, func(a, b int) int { return seq(a) - seq(b) })
	}
	return out
}

// queuedBehind：d 要盲打的牌左邊還有同一人之前被 finesse、還沒打的牌（Queued Finesse；Level 2–4 不會這樣 finesse）。
func (s *state) queuedBehind(d dep) bool {
	for _, id := range s.hands[d.player] {
		if id == d.card {
			return false
		}
		if n := s.notes[id]; n.finessed && !n.clued {
			return true
		}
	}
	return false
}
