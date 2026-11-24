// Level 5（docs/level-5.mdx）中自成一塊的部分：Urgency Principle 的例外（可以延後盲打的緊急行動）、
// Queued Finesse 的盲打順序。Layered / Clandestine / Hidden Finesse 是連接牌推論的分支，在 connect.go
// （findFinesse 的 layered、findPrompt 的 hiddenFinesse、stepDep 的 layerSlot）；Ambiguous Finesse 在 level2.go 的 passMeansMe。
// 彩虹等多色變體的 Prompt 規則（Prompts in Multi-Color Variants）在 connect.go 的 promptOrder。

package hgroup

import (
	"slices"

	"hanabi/game"
)

// deferrable（Level 5 Urgency Principle 的例外）：e.Player 手上有該盲打的 finesse，卻給了這個提示。
// 點到接收者 chop 上必須保留的牌（`deferWorthy`）的提示是「really important」的事，可以延後盲打——
// 大家再給他一回合，不當成他沒接 finesse。其他提示（例如 Bob is Lazy 挑戰題的 1-for-1）不算。
// 以前點到 chop 就算：使用者回報 20261124-101437：使用者藍色點 AI-3 的 B2，AI-1、AI-2 的 finesse position 都是 B1，
// AI-1 看到 AI-2 的 B1 而改用「1」點 AI-3 chop 上的 W1（還有兩張），AI-2 以為 AI-1 在延後盲打，沒接 Ambiguous Finesse。
// 原文另一個例外「給只有他能給的 finesse」要靠各人自己對那個提示的解讀（看得到的牌不同，結論可能不同），
// 會讓各家對 finesse 有沒有兌現的筆記不一致，不採用（AI 自己也只會為了救 chop 延後盲打）。
func (s *state) deferrable(e game.Event) bool {
	if !s.knows(lvUrgency) || e.Type != game.EvHint {
		return false
	}
	c := s.chop(e.Target)
	if c < 0 || !slices.Contains(e.Touched, slices.Index(s.hands[e.Target], c)) {
		return false
	}
	if t, ok := s.see(s.me, c); ok {
		return s.deferWorthy(t)
	}
	// 接收者是我、看不到那張：提示後它可能是的有用牌面每一種都必須保留才算（例如「5」、「2」救 chop）。
	// 只要有一種可能就算的話，看得到那張不重要的旁觀者判定他讓出了 finesse、我卻以為他在延後，兩邊對誰被 finesse 看法不一
	// （Ambiguous Finesse 的挑戰題：AI-2 讓給使用者後用黃色點我的 chop，使用者盲打出 B1，我仍等 AI-2 而不打 B2）
	poss := s.notes[c].poss & hintSet(e.Hint) & s.useful()
	if poss == 0 {
		return false
	}
	for _, t := range poss.list() {
		if !s.deferWorthy(t) {
			return false
		}
	}
	return true
}

// deferWorthy：chop 上的 t 是 Save Principle 要救的牌（5、critical、2），為它延後盲打才算原文的「really important」。
// 只是唯一看得到的可打牌（例如還有兩張的 1）不算——棄掉只是晚一點打，不值得讓 finesse 的對象各說各話。
// AI 自己延後盲打（stepDeferFinesse）也照這個條件，大家的判斷才一致。
func (s *state) deferWorthy(t ident) bool {
	return !s.trash(t) && (t.rank() == game.MaxRank || t.rank() == 2 || s.critical(t))
}

// stepDeferFinesse（Level 5 Urgency Principle 的例外）：我有該盲打的 finesse，但下一位會棄掉 chop 上必須保留的牌：
// 先救（點到他的 chop，大家看得出是可以延後盲打的緊急行動），下一手再盲打。
func (s *state) stepDeferFinesse(t *turn) (game.Action, string, bool) {
	if !s.knows(lvUrgency) || !slices.ContainsFunc(s.hands[s.me], func(id int) bool { return s.notes[id].finessed && s.knownPlayable(id) }) {
		return game.Action{}, "", false
	}
	if s.guided() {
		return game.Action{}, "", false // Level 11 Guide Principle：上一位剛引導我盲打，現在就打
	}
	a, why, ok := s.urgentSave(t.clues(s), t.v.Hints, true)
	if !ok || a.Type != game.ActHint {
		return game.Action{}, "", false
	}
	chop := s.chop(a.Target)
	if !slices.ContainsFunc(t.clues(s), func(e clueEval) bool { return e.act == a && slices.Contains(e.touched, chop) }) {
		return game.Action{}, "", false
	}
	if c, ok := s.card(chop); !ok || !s.deferWorthy(identOfCard(c)) {
		return game.Action{}, "", false // 別人不會當成可以延後的緊急行動（deferrable）
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
