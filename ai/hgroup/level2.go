// Level 2（docs/level-2.mdx）中自成一塊的技巧：5 Stall、Wrong Prompt、finesse 對象讓給後一位（retryFinesse、passMeansMe）、
// Stomping on a Finesse。多重 prompt / finesse 本身在 connect.go。

package hgroup

import (
	"slices"

	"hanabi/game"
)

// fiveStallHints：Early Game 至少還有這麼多提示才用 5 Stall 代替棄牌。
var fiveStallHints = 1

// mustStall：給提示者這一手不能棄牌（提示滿 8，或手牌全被拿到），只能給拖延提示。
func (s *state) mustStall(giver int) bool { return s.hints >= game.MaxHints || s.chop(giver) < 0 }

// fiveStall：數字 5 提示點到不在 chop 上的新的 5 時，可以當成 5 Stall（Level 2）。只重新點到舊的 5 是 tempo clue
// （這張能打），除非給提示者這一手不能棄牌（那只是拖延）：
// Early Game 中，或給提示者不能棄牌（提示滿 8、或手牌全被拿到）時，只是救下這些 5、拖延時間。
func (s *state) fiveStall(giver int, h game.Hint) bool {
	if !s.knows(lvFiveStall) || h.Kind != game.HintRank || h.Rank != game.MaxRank {
		return false
	}
	return !s.midGame || s.hints >= game.MaxHints || s.chop(giver) < 0
}

// retryFinesse：被認為是 finesse 對象的人輪到了卻沒有盲打——他沒打，表示他看到別人的 finesse position 上
// 也有那張牌，所以是那個人（例如 AI-1 看到逆飛對象 AI-3 也有 g2 而不打，就是 AI-3）。把他排除後重新找連接牌，
// 補回對應的解讀，也重算我自己看得到焦點時的計畫（輪到我就寫下 finesse 筆記）。before 是推進前的解讀。
func (s *state) retryFinesse(rec *clueRec, e game.Event, played int, before, kept []cand) []cand {
	var lost []ident
	for _, c := range before {
		for _, d := range c.deps {
			if d.finesse && d.player == e.Player && d.card != played && s.playableNow(d.id) &&
				slices.Contains(s.hands[d.player], d.card) && !slices.ContainsFunc(kept, func(k cand) bool { return k.id == c.id }) {
				lost = append(lost, c.id)
			}
		}
	}
	if len(lost) == 0 {
		return kept
	}
	rec.passed = append(rec.passed, e.Player)
	defer s.saveSim()()
	s.skipFinesse = rec.passed
	// 他沒盲打的那張不算「已經有人會打」（這個提示可能已把它記成被 finesse；別的提示的筆記不動）
	var skip []int
	for _, c := range before {
		for _, d := range c.deps {
			if d.finesse && d.player == e.Player {
				skip = append(skip, d.card)
			}
		}
	}
	s.skipCards = skip
	for _, id := range lost {
		deps, ok := s.connectRec(rec, rec.target, id)
		if !ok || len(deps) == 0 {
			continue
		}
		// 只補回「看得到另一位玩家的 finesse position 確實是那張」的解讀（他沒打最可能是看到了那張）；
		// 要靠接收者自己或我（看不到）的，不補——原文：那人沒盲打時，通常是另一種直接的解讀
		visible := true
		for _, d := range deps {
			if d.finesse {
				if t, ok := s.see(rec.target, d.card); !ok || t != d.id && d.layers == 0 || d.player == s.me {
					visible = false
				}
			}
		}
		if visible {
			kept = append(kept, cand{id: id, deps: deps})
		}
	}
	// 我的計畫（我看得到焦點）：只有在他的角度確實需要 finesse、而且他能看到的人裡只可能是我（或他自己）時，
	// 他沒打才代表「是我」——若他看得到連接牌已在別處（被提示 / 會被打），他不打另有理由
	if t, ok := s.see(s.me, rec.focus); ok && slices.Contains(lost, t) && s.passMeansMe(rec, e.Player, t, s.onFinessePos(rec, before, e.Player, t)) {
		if deps, ok := s.connectRec(rec, s.me, t); ok {
			for _, d := range deps {
				if !slices.ContainsFunc(rec.plan, func(p dep) bool { return p.card == d.card }) && d.player != e.Player {
					s.markDep(rec, d)
				}
			}
			rec.plan = slices.DeleteFunc(deps, func(d dep) bool { return d.player == e.Player })
		}
	}
	return kept
}

// onFinessePos：解讀 t 要 p 盲打的牌，我看得到確實就是 t 需要的那張、是他在提示當下的 finesse position（沒有 layer）。
// Level 5 The Ambiguous Finesse 的前提是「兩人的 finesse position 都是那張」：他明明有卻不打，才表示他看到我也有；
// 他的 finesse position 不是那張（例如我以為的 layer、或我記的 finesse position 和他不同）時，他不打另有原因。
func (s *state) onFinessePos(rec *clueRec, cs []cand, p int, t ident) bool {
	for _, c := range cs {
		if c.id != t {
			continue
		}
		for _, d := range c.deps {
			if d.player == p && d.finesse {
				if x, ok := s.see(s.me, d.card); ok && x == d.id && d.layers == 0 && rec.fpAt != nil && d.card == rec.fpAt[p] {
					return true
				}
			}
		}
	}
	return false
}

// passMeansMe：從沒盲打的 p 的角度（不跳過他自己）找 t 的連接時，需要的 finesse 落在他自己或我身上。
// Level 2 只處理「他在給提示者與接收者之間、我在接收者之後」：他沒打，是因為看到我可能被逆飛。
// Level 5 The Ambiguous Finesse / Urgency Principle：其他位置也一樣——他看得到我的 finesse position 上有那張而不打，
// 就是我（大家都會儘快盲打，他不打就表示不是他）；但只限他的角度確實落在我身上，而且他的 finesse position
// 確實是那張（exact，原文的例子是兩人都有）。
func (s *state) passMeansMe(rec *clueRec, p int, t ident, exact bool) bool {
	pos := func(q int) int { return (q - rec.giver + s.n) % s.n }
	reverse := pos(p) < pos(rec.target) && pos(rec.target) < pos(s.me)
	// 提示之後有人打錯：What to Do After a Strike 會重置相關的筆記，他沒盲打可能只是因為這樣（自我對局 seed 948）
	if !reverse && !(s.knows(lvUrgency) && exact && !rec.struck) {
		return false
	}
	// 從他的角度這可能是 save clue（例如他看不到自己手上的另一張 2，以為 chop 上的 2 是 2 Save）：
	// save clue 不會讓人盲打，他不打不代表是我
	if rec.saveView != nil && rec.saveView[p] {
		return false
	}
	defer s.saveSim()()
	s.skipFinesse = nil // 他自己不跳過；他沒打的那張仍不算「已經有人會打」（skipCards 保留）
	s.assumeMine = !reverse
	deps, ok := s.connectRec(rec, p, t)
	if !ok {
		return false
	}
	for _, d := range deps {
		if d.finesse && (d.player == s.me || reverse && d.player == p) {
			return true
		}
	}
	return false
}

// wrongPrompt：player 打錯的 card，從他自己的角度看，是某次提示要 prompt 的已提示牌時（Level 2 The Wrong Prompt），
// 大家都看得出來：給提示者其實是要 finesse 他在提示當下 finesse position 上的牌。
// 在 checkDeps 之前呼叫：把那次提示的 prompt 改成 finesse 那張牌，並回傳那張牌與它可能的牌面。
func (s *state) wrongPrompt(player, card int) (fp int, ids idSet, rec *clueRec) {
	for i := len(s.recs) - 1; i >= 0; i-- {
		rec := s.recs[i]
		if rec.fpAt == nil {
			continue
		}
		fp := rec.fpAt[player]
		if fp < 0 || !slices.Contains(s.hands[player], fp) || s.notes[fp].clued {
			continue
		}
		var targets []ident
		for _, c := range rec.cands {
			targets = append(targets, c.id)
		}
		if t, ok := s.see(s.me, rec.focus); ok {
			targets = append(targets, t)
		}
		var ids idSet
		// 已經記下的 prompt（我看得到焦點時的計畫，或接收者角度的解讀）
		for _, ds := range append([][]dep{rec.plan}, depsOf(rec.cands)...) {
			for _, d := range ds {
				if d.card == card && !d.finesse && s.playableNow(d.id) {
					ids |= setOf(d.id)
				}
			}
		}
		// 從打錯的人自己的角度推（旁觀者看得到那張不是連接牌，但他自己看不到）
		for _, t := range targets {
			deps, ok := s.connectRec(rec, player, t)
			if !ok {
				continue
			}
			for _, d := range deps {
				if d.card == card && !d.finesse && s.playableNow(d.id) {
					ids |= setOf(d.id)
				}
			}
		}
		if ids == 0 {
			continue
		}
		conv := func(ds []dep) {
			for j, d := range ds {
				if d.card == card && !d.finesse {
					ds[j] = dep{player: player, card: fp, id: d.id, finesse: true}
				}
			}
		}
		for _, c := range rec.cands {
			conv(c.deps)
		}
		conv(rec.plan)
		return fp, ids, rec
	}
	return -1, 0, nil
}

// applyWrongPrompt：打錯的人提示當下 finesse position 上的牌就是原本要 prompt 的那張，他下一手盲打。
// 這是大家都看得懂的失誤，不用像一般打錯那樣重置筆記。
func (s *state) applyWrongPrompt(fp int, ids idSet, rec *clueRec) {
	n := s.notes[fp]
	if p := n.poss & ids; p != 0 {
		n.poss = p
	}
	s.got(fp, rec.giver)
	n.finessed, n.willPlay, n.finRec = true, true, rec
	rec.changed = append(rec.changed, fp)
}

// stomps：提示點到某張「某個提示的可能解讀裡要被 finesse 盲打」的牌（Level 2 Stomping on a Finesse：
// 直接提示本來就會被盲打的牌，浪費一個提示）。例如我收到的提示可能是 Reverse Finesse 下一位，
// 我就該等他盲打，不是直接提示他那張。
// 只看我是接收者、看不到焦點牌的提示（看得到時，計畫裡的 finesse 牌已經記成被 finesse，不會是新牌）。
// Level 5：Layered / Hidden Finesse 真正要盲打的牌在 layer 後面，也不能直接提示它（What's Hidden? 挑戰題）。
func (s *state) stomps(touched []int) bool {
	for _, rec := range s.recs {
		if rec.target != s.me {
			continue
		}
		for _, c := range rec.cands {
			for _, d := range c.deps {
				if (d.finesse || d.layers > 0) && d.player != s.me && slices.ContainsFunc(s.depCards(d), func(id int) bool {
					return !s.notes[id].clued && slices.Contains(touched, id)
				}) {
					return true
				}
			}
		}
	}
	return false
}
