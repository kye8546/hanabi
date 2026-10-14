// 連接牌：一個 play clue 要能打出所需的 prompt / finesse（connect），以及之後追蹤它們有沒有兌現（checkDeps）。
// Level 2 的多重連接（Double Prompt / Finesse、Reverse Finesse、Self-Finesse）都在這裡的分支裡。

package hgroup

import (
	"slices"

	"hanabi/game"
)

// connect 找出讓 t 能打出所需的連接牌。已被拿到且確定的牌不需要動作；
// 其餘依序找 prompt（優先）或 finesse。Level 1 只允許一個 prompt 或 finesse；
// Level 2 允許多個（Double Prompt / Double Finesse / Prompt + Finesse），但連接牌要依座位順序打出
// （不依順序的 Out-of-Order Finesse 是 Level 20）。
func (s *state) connect(obs, giver, R, focus int, t ident) ([]dep, bool) {
	if s.trash(t) {
		return nil, false
	}
	var deps []dep
	used := append([]int{focus}, s.sameClue...)
	last := 0
	c := t.color()
	for r := s.stacks[c] + 1; r < t.rank(); r++ {
		m := identOf(c, r)
		if s.alreadyGotten(obs, giver, m, used) {
			continue
		}
		// Prompts > Finesses：已提示的牌（包括接收者自己的，self-prompt）優先於 finesse
		d, found := s.findPrompt(obs, giver, m, used)
		if !found {
			if d, found = s.findFinesse(obs, giver, R, m, used); !found {
				return nil, false
			}
		}
		if len(deps) > 0 && !s.knows(lvMultiConnect) {
			return nil, false // 雙重 prompt / finesse 是 Level 2
		}
		pos := (d.player - giver + s.n) % s.n
		if pos < last {
			return nil, false
		}
		last = pos
		deps = append(deps, d)
		used = append(used, d.card)
	}
	return deps, true
}

// connectRec 為已發生的提示 rec 重新找連接牌：同一個提示點到的新牌不能當焦點的連接牌
// （delayed play clue 只能接之前就被拿到的牌；當時它們也還沒被提示）。
func (s *state) connectRec(rec *clueRec, obs int, t ident) ([]dep, bool) {
	defer s.saveSim()()
	s.sameClue = rec.fresh
	return s.connect(obs, rec.giver, rec.target, rec.focus, t)
}

// alreadyGotten 回傳（obs 角度）連接牌 m 是否已經有人會打、不需要 prompt / finesse：
// 某張被拿到的牌筆記確定是 m，或持有者本來就會打它（已知可打、或是 play clue 的焦點）、而且它可能是 m
// （obs 看得到時必須真的是 m）。本來就會打的牌不算 prompt。
// 給提示者看不到自己的牌，所以他手上的牌只有筆記確定是 m 才算。exclude 是焦點與這條連接已用到的牌。
func (s *state) alreadyGotten(obs, giver int, m ident, exclude []int) bool {
	for pl, h := range s.hands {
		for _, id := range h {
			if slices.Contains(exclude, id) || slices.Contains(s.skipCards, id) || !s.live(id) {
				continue
			}
			p := s.possOf(id)
			if pl == giver {
				// 給提示者看不到自己的牌：他自己的已提示牌，只有在別人也看得出他知道是 m 時才會被當成連接牌。
				// 就算看得到它實際就是 m，他不知道就不可能靠它，那就是 finesse / prompt 別人
				// （Level 10 Sarcastic Finesse：「the duplicated card in your own hand must be unknown. Otherwise,
				// the clue receiver will simply think that the clued card is the next card in the suit」）。
				// 我是給提示者時，我對自己牌的私下推論（筆記）別人不一定推得出，不能拿來當大家的共識
				if obs == giver {
					if t, ok := p.single(); ok && t == m {
						return true
					}
					continue
				}
				t, ok := s.see(obs, id)
				if !ok && pl == s.me {
					t, ok = p.single() // 我是給提示者：他看得到我這張，我的推論確定的話就是它
				}
				if ok {
					if t == m && p.has(m) && s.giverMayKnow(obs, id, m) {
						return true
					}
				} else if s.optimistic && giver == s.me && obs != s.me && p.has(m) && s.mineCanBe(id, m) {
					// 我是給提示者、在模擬接收者：他看得到我這張，我看不到，它可能就是 m
					return true
				}
				continue
			}
			if t, ok := p.single(); ok && t == m {
				return true
			}
			if p.has(m) && (s.holderKnowsPlayable(id) || s.notes[id].willPlay) {
				if t, ok := s.see(obs, id); ok && t == m || !ok && s.mineCanBe(id, m) {
					return true
				}
			}
		}
	}
	return false
}

// findPrompt：從給提示者的下一位開始，第一位手上有「可能是 m 的被提示牌」的玩家，
// 取他最左邊那張（used 是這條連接已用到的牌；Double Prompt 由左而右）。
// 「Prompts > Finesses」是第一人稱的規則：obs 看得到那張不是 m 時，給提示者也看得到，
// 就不是要 prompt 它，跳過繼續找。
func (s *state) findPrompt(obs, giver int, m ident, used []int) (dep, bool) {
	for k := 1; k < s.n; k++ {
		p := (giver + k) % s.n
		for _, id := range s.hands[p] {
			n := s.notes[id]
			if slices.Contains(used, id) || !n.clued || !s.possOf(id).has(m) || n.willPlay || s.holderKnowsPlayable(id) {
				continue
			}
			// 看得到它不是 m（或我推得出自己的牌不是 m）：給提示者也看得到，不會想 prompt 它，
			// 跳過它繼續找（同一人的下一張已提示牌，最後才是 finesse）
			if t, ok := s.see(obs, id); ok && t != m || !ok && !s.mineCanBe(id, m) {
				continue
			}
			return dep{player: p, card: id, id: m}, true
		}
	}
	return dep{}, false
}

// finesseSlot 是 p 的 finesse position，跳過這條連接已用到的牌（Double Finesse 時同一人的下一張）。
// Level 2–4：他手上已有之前的提示 finesse 的牌還沒打時，不再 finesse 他後面的牌
// （同一人排隊盲打不同提示的牌是 Level 5 的 Queued Finesse）。
func (s *state) finesseSlot(p int, used []int) int {
	for _, id := range s.hands[p] {
		if slices.Contains(used, id) || slices.Contains(s.touchedNow, id) {
			continue
		}
		if n := s.notes[id]; n.finessed && !n.clued && s.knows(lvMultiConnect) && !s.knows(lvQueuedFinesse) {
			return -1
		}
		if !s.gotten(id) {
			return id
		}
	}
	return -1
}

// findFinesse 找 finesse position 上是 m 的玩家。看得到的優先；都看不到時，才推定是看不到的那位（通常是 obs 自己）。
// Level 1 只找給提示者與接收者之間（不含兩者）的玩家。Level 2 加上接收者之後的玩家（Reverse Finesse），
// 最後才是接收者自己（Self-Finesse 只在沒有其他可能時才成立）。s.skipFinesse 是這次提示已經輪過、沒有盲打的人。
func (s *state) findFinesse(obs, giver, R int, m ident, used []int) (dep, bool) {
	var order []int
	for k := 1; k < s.n; k++ {
		p := (giver + k) % s.n
		if p == R {
			if !s.knows(lvReverseFinesse) {
				break
			}
			continue
		}
		order = append(order, p)
	}
	if s.knows(lvSelfFinesse) {
		order = append(order, R)
	}
	var unknown *dep
	for _, p := range order {
		if slices.Contains(s.skipFinesse, p) {
			continue
		}
		fp := s.finesseSlot(p, used)
		if fp < 0 {
			continue
		}
		if t, ok := s.see(obs, fp); ok {
			if t == m {
				return dep{player: p, card: fp, id: m, finesse: true}, true
			}
			continue
		}
		if p == s.me && !s.myPoss(fp).has(m) {
			continue // 我推得出自己的 finesse position 不是 m（看得到它的人更知道）
		}
		if unknown == nil {
			unknown = &dep{player: p, card: fp, id: m, finesse: true}
		}
	}
	if unknown != nil {
		return *unknown, true
	}
	return dep{}, false
}

// markDep 在被 prompt / finesse 的牌上寫筆記。
func (s *state) markDep(rec *clueRec, d dep) {
	n := s.notes[d.card]
	n.poss = setOf(d.id)
	n.willPlay = true
	if d.finesse {
		s.got(d.card, rec.giver)
		n.finessed, n.finRec = true, rec
	}
	rec.changed = append(rec.changed, d.card)
}

// selfMark：每種解讀都要 who 自己同一張牌（self-prompt，或 Level 2 的 self-finesse）時，那張牌就是連接牌。
func (s *state) selfMark(rec *clueRec, who int) {
	card, fin := -1, false
	var ids idSet
	for _, c := range rec.cands {
		i := slices.IndexFunc(c.deps, func(d dep) bool { return d.player == who })
		if i < 0 || !s.knows(lvMultiConnect) && len(c.deps) != 1 {
			return
		}
		d := c.deps[i]
		if card >= 0 && d.card != card {
			return
		}
		card, fin = d.card, d.finesse
		ids |= setOf(d.id)
	}
	if card < 0 || s.holder(card) != who {
		return
	}
	n := s.notes[card]
	p := n.poss & ids
	if p == 0 {
		return
	}
	n.poss = p
	if fin {
		s.got(card, rec.giver)
		n.finessed, n.willPlay, n.finRec = true, true, rec
	}
	rec.changed = append(rec.changed, card)
}

// markCommon：我是接收者、看不到焦點牌時，每種解讀都要同一張別人的牌被 finesse，
// 那張牌就確定被 finesse 了（看得到焦點牌的人也會照 plan 寫下同樣的筆記），之後大家的 finesse position 才一致。
func (s *state) markCommon(rec *clueRec) {
	if !s.knows(lvMultiConnect) || len(rec.cands) == 0 {
		return
	}
	for _, d := range rec.cands[0].deps {
		if !d.finesse || d.player == s.me || s.notes[d.card].finessed {
			continue
		}
		ids := setOf(d.id)
		all := true
		for _, c := range rec.cands[1:] {
			i := slices.IndexFunc(c.deps, func(o dep) bool { return o.card == d.card && o.finesse })
			if i < 0 {
				all = false
				break
			}
			ids |= setOf(c.deps[i].id)
		}
		if !all || s.holder(d.card) != d.player {
			continue
		}
		n := s.notes[d.card]
		if p := n.poss & ids; p != 0 {
			n.poss = p
		}
		s.got(d.card, rec.giver)
		n.finessed, n.willPlay, n.finRec = true, true, rec
		rec.changed = append(rec.changed, d.card)
	}
}

type depStatus int

const (
	depPending depStatus = iota
	depDone
	depFail
)

// stepDep 依某位玩家的行動判斷一個連接是否兌現。played 是他打出/棄掉的牌（給提示時為 -1）。
// wait 為 true 時不套用 Finesse Urgency（self-finesse 要等其他解讀被排除，接收者暫時不打是正常的）。
func (s *state) stepDep(d dep, e game.Event, played int, wait bool) (dep, depStatus) {
	if d.player != e.Player {
		return d, depPending
	}
	if played == d.card {
		if e.Type == game.EvPlay && identOfCard(e.Card) == d.id {
			return d, depDone
		}
		if e.Type == game.EvPlay && !d.finesse {
			// prompt 打出的是別張（也成功）：承諾移到他下一張可能的被提示牌（由左而右）
			for _, id := range s.hands[d.player] {
				if id != played && s.notes[id].clued && s.notes[id].poss.has(d.id) {
					d.card = id
					return d, depPending
				}
			}
		}
		return d, depFail
	}
	if !d.finesse || (wait || d.card == s.excused) && slices.Contains(s.hands[d.player], d.card) {
		return d, depPending
	}
	if !slices.Contains(s.hands[d.player], d.card) {
		return d, depFail
	}
	// finesse 要立刻盲打：牌已經可以打了，他卻做了別的事（不是去打另一張被 finesse 的牌）。
	// 但他自己的筆記還不確定那張現在能打（例如 self-finesse 可能是 b2 或 w2，b1 還沒打出）時，等是對的
	if s.playableNow(d.id) && !(played >= 0 && s.notes[played].finessed) {
		if n := s.notes[d.card]; n.finessed && !s.holderKnowsPlayable(d.card) {
			return d, depPending
		}
		return d, depFail
	}
	return d, depPending
}

// checkDeps 在每個行動「之前」呼叫，驗證所有待兌現的 prompt / finesse。
func (s *state) checkDeps(e game.Event, played int) {
	// 這張牌已能由我看得到的某個提示解釋（它在那次提示的連接計畫中），
	// 就不能拿來當作其他提示的證據
	explained := map[*clueRec]bool{}
	for _, rec := range s.recs {
		for _, d := range rec.plan {
			if d.card == played {
				explained[rec] = true
			}
		}
	}
	otherExplains := func(rec *clueRec) bool {
		for r := range explained {
			if r != rec {
				return true
			}
		}
		return false
	}
	for _, rec := range s.recs {
		// 接收者角度的解讀
		passedBefore := slices.Clone(rec.cands)
		kept, changed, done := s.stepCands(rec.cands, e, played, -1)
		if s.knows(lvSelfFinesse) && e.Player != rec.target && e.Player != s.me && !slices.Contains(rec.passed, e.Player) {
			kept = s.retryFinesse(rec, e, played, passedBefore, kept)
		}
		// 可能是 save clue 的提示不會觸發 prompt，已提示的連接牌被打出證明不了什麼；
		// 但 finesse 的盲打不同：save clue 不會讓人盲打，沒有別的提示能解釋的盲打就證明是 play clue
		mayBeSave := slices.ContainsFunc(kept, func(c cand) bool { return c.save })
		blind := done >= 0 && slices.ContainsFunc(rec.cands, func(c cand) bool {
			return slices.ContainsFunc(c.deps, func(d dep) bool { return d.card == done && d.finesse })
		})
		if done >= 0 && (mayBeSave && !blind || otherExplains(rec) && !explained[rec]) {
			// 別的提示已解釋這次打出：不排除，只把這個連接視為已完成
			kept = dropDep(kept, done)
		} else if done >= 0 {
			kept, changed = onlyWith(kept, done), true
		}
		// 還沒輪到的 self-prompt / self-finesse 解讀也照樣追蹤；接收者自己的 finesse 不必急著打
		fb, _, fdone := s.stepCands(rec.fallback, e, played, rec.target)
		if fdone >= 0 {
			fb = onlyWith(fb, fdone)
		}
		rec.cands, rec.fallback = kept, fb
		promoted := false
		if len(rec.cands) == 0 && len(rec.fallback) > 0 {
			// 其他解讀都不成立了：輪到 self-prompt / self-finesse（Level 2 The Self-Finesse）
			rec.cands, rec.fallback = rec.fallback, nil
			changed, promoted = true, true
		}
		if changed {
			s.refreshFocus(rec)
		}
		if promoted {
			s.selfMark(rec, rec.target)
		}
		if changed && rec.target == s.me {
			s.markCommon(rec)
		}

		// 我自己推論出的連接
		var plan []dep
		for _, d := range rec.plan {
			nd, st := s.stepDep(d, e, played, false)
			switch st {
			case depFail:
				s.resetNote(d.card)
			case depPending:
				if nd.card != d.card {
					s.markDep(rec, nd)
				}
				plan = append(plan, nd)
			}
		}
		rec.plan = plan
	}
}

// stepCands 依這次行動推進每種解讀的連接，丟掉連接失敗的解讀。done 是剛兌現的連接牌（沒有則 -1）。
// waitFor 這位玩家的 finesse 不套用 Finesse Urgency。
func (s *state) stepCands(cs []cand, e game.Event, played, waitFor int) (kept []cand, changed bool, done int) {
	done = -1
	for _, c := range cs {
		fail := false
		for i, d := range c.deps {
			nd, st := s.stepDep(d, e, played, d.player == waitFor)
			switch st {
			case depFail:
				fail = true
			case depDone:
				done = d.card
			}
			c.deps[i] = nd
		}
		if fail {
			changed = true
			continue
		}
		kept = append(kept, c)
	}
	return kept, changed, done
}

// onlyWith：有人照這個提示打出了連接牌 card，只保留需要那張牌的解讀（並把它視為已完成）。
func onlyWith(cs []cand, card int) []cand {
	var only []cand
	for _, c := range cs {
		if i := slices.IndexFunc(c.deps, func(d dep) bool { return d.card == card }); i >= 0 {
			c.deps = slices.Delete(slices.Clone(c.deps), i, i+1)
			only = append(only, c)
		}
	}
	return only
}

// dropDep 把連接牌 card 視為已完成（不排除任何解讀）。
func dropDep(cs []cand, card int) []cand {
	for i := range cs {
		cs[i].deps = slices.DeleteFunc(slices.Clone(cs[i].deps), func(d dep) bool { return d.card == card })
	}
	return cs
}

func depsOf(cs []cand) [][]dep {
	var out [][]dep
	for _, c := range cs {
		out = append(out, c.deps)
	}
	return out
}

// recheckDelayed：delayed play clue 的解讀是靠「已經有人會打的牌」接上的。那些牌打出或棄掉後，
// 若證實不是需要的牌（例如以為可能是 g1 的 1 打出來是 y1），這個解讀就不成立了，從筆記移除。
func (s *state) recheckDelayed() {
	for _, rec := range s.recs {
		if s.holder(rec.focus) < 0 {
			continue
		}
		kept := slices.DeleteFunc(slices.Clone(rec.cands), func(c cand) bool {
			if c.save || len(c.deps) > 0 || s.trash(c.id) || s.playableNow(c.id) {
				return false
			}
			deps, ok := s.connectRec(rec, rec.target, c.id)
			return !ok || len(deps) > 0
		})
		if len(kept) > 0 && len(kept) < len(rec.cands) {
			rec.cands = kept
			s.refreshFocus(rec)
		}
	}
}

// afterPlay 重新檢查還在追蹤的提示，並丟掉已經沒有東西要追蹤的紀錄。
func (s *state) afterPlay() {
	s.recheckDelayed()
	s.recs = slices.DeleteFunc(s.recs, func(r *clueRec) bool {
		if len(r.plan) > 0 || s.holder(r.focus) >= 0 {
			return false
		}
		for _, c := range r.cands {
			if len(c.deps) > 0 {
				return false
			}
		}
		return true
	})
}
