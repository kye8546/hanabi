// 救 chop：哪些牌必須保留（needsSave）、下一位要棄掉它們時怎麼處理（urgentSave），以及相關的取捨。

package hgroup

import (
	"slices"

	"hanabi/game"
)

// needsSave：（我看得到的）這張 chop 牌依 Save Principle 不能被棄掉。
func (s *state) needsSave(p, id int) bool {
	t := identOfCard(s.cards[id])
	if s.trash(t) || s.gottenAs(s.me, s.me, t, id) {
		return false
	}
	switch {
	case t.rank() == 5, s.critical(t):
		return true
	case t.rank() == 2:
		return !s.twoVisible(s.me, s.me, p, id, t)
	}
	// 唯一一張看得到的可打牌。但我手上有已提示、不確定的牌可能就是它時，它不一定唯一，
	// 我去救還可能造成重複（Level 12 Duplication Responsibility：「The same general concept
	// also applies to cluing playable cards on chop」），交給別人
	if s.maybeMine(t) {
		return false
	}
	if deps, ok := s.connect(s.me, s.me, p, id, t); ok && len(deps) == 0 {
		for q, h := range s.hands {
			for _, o := range h {
				if q != s.me && o != id && identOfCard(s.cards[o]) == t {
					return false
				}
			}
		}
		return true
	}
	return false
}

// saveIsOthersJob：要救的 t 可能和我手上不確定的已提示牌重複（`maybeMine`），交給別人決定（Level 12 Duplication
// Responsibility 第一段：手上有已提示的 2 的人不該負責救 2，以免違反 Good Touch；他看得到我那張，多半根本不用救）：
//   - 在 R 行動前還有別人手上可能重複的已提示牌比我少：交給他救；
//   - 另有一位（不是 R）手上沒有可能是 t 的已提示牌：他看得到我的牌和那張 chop，對這件事是全知的，讓他判斷——
//     就算他輪到前 R 先棄掉了它（不是 critical，還有另一張），期望值仍比冒重複的險高（使用者的判斷）。
//     2 人局沒有這樣的人，照救。critical 的 t 不會走到這裡（看得到最後一張時，我的牌就不會是它）。
func (s *state) saveIsOthersJob(R int, t ident) bool {
	if !s.maybeMine(t) {
		return false
	}
	matching := func(p int) int {
		c := 0
		for _, id := range s.hands[p] {
			if !s.gotten(id) {
				continue
			}
			if p == s.me && s.myPoss(id).has(t) || p != s.me && s.notes[id].poss.has(t) {
				c++
			}
		}
		return c
	}
	mine := matching(s.me)
	for k := 1; (s.me+k)%s.n != R; k++ {
		if matching((s.me+k)%s.n) < mine {
			return true
		}
	}
	for q := range s.n {
		if q != s.me && q != R && matching(q) == 0 {
			return true
		}
	}
	return false
}

// saveWeight：必須保留的牌有多重要（critical / 5 > 2 > 其他，例如唯一看得到的可打牌）。
func (s *state) saveWeight(id int) int {
	t := identOfCard(s.cards[id])
	switch {
	case s.critical(t) || t.rank() == game.MaxRank:
		return 3
	case t.rank() == 2:
		return 2
	}
	return 1
}

// willPlay：p 自己知道手上有牌可打（下一回合不會棄牌）。
// 只要等我這一手就會被排除的解讀（需要我盲打、但我知道不是 finesse）不算，
// 因為輪到 p 之前我已經用行動證明了。
func (s *state) willPlay(p int) bool {
	for _, id := range s.hands[p] {
		if s.notes[id].bad {
			continue
		}
		poss := s.notes[id].poss
		// 被拿到的牌依 Good Touch 不是垃圾：已經打出的牌面（例如我剛模擬打出的 b4）他會排除
		if q := poss & s.useful(); s.gotten(id) && q != 0 {
			poss = q
		}
		for _, rec := range s.recs {
			if rec.focus != id {
				continue
			}
			var u idSet
			for _, c := range rec.cands {
				if !s.failsOnMyTurn(rec, c) {
					u |= setOf(c.id)
				}
			}
			if poss&u != 0 {
				poss &= u
			}
		}
		all := poss != 0
		for _, i := range poss.list() {
			all = all && s.playableNow(i)
		}
		if all {
			return true
		}
	}
	return false
}

// failsOnMyTurn：這個解讀需要我現在盲打（finesse），但我推論出的計畫裡沒有，我不會打，它就會被排除。
func (s *state) failsOnMyTurn(rec *clueRec, c cand) bool {
	for _, d := range c.deps {
		if d.finesse && d.player == s.me && s.playableNow(d.id) &&
			!slices.ContainsFunc(rec.plan, func(p dep) bool { return p.card == d.card }) {
			return true
		}
	}
	return false
}

// urgentSave：下一位玩家 chop 上有必須保留的牌、而他沒有牌可打（會棄牌）時，現在就處理。
// 能讓他下一手就打牌（直接提示、prompt 或 finesse 他）就優先給（Give Play Clues Over Save Clues：讓他忙著打牌就不會棄 chop），
// 否則救 chop。
// trust 為 true 時，Early Game 中相信下一位會先給提示而不棄牌。
func (s *state) urgentSave(evs []clueEval, hints int, trust bool) (game.Action, string, bool) {
	next := (s.me + 1) % s.n
	chop := s.chop(next)
	if chop < 0 || !s.needsSave(next, chop) || s.willPlay(next) || trust && s.nextWillClue(next, hints, nil) ||
		s.playLetsNextPlay(next) {
		return game.Action{}, "", false
	}
	// 我用掉最後一個提示後，後面某位的 chop 就沒人能救了，而那張比下一位的 chop 更重要（例如 critical 對上
	// 唯一看得到的可打牌）：改救他，讓下一位棄比較不重要的牌
	laterFirst := func(ev clueEval) (game.Action, bool) {
		p, ok := s.doomsLaterChop(ev, hints)
		if !ok || s.saveWeight(s.chop(p)) <= s.saveWeight(chop) {
			return game.Action{}, false
		}
		pc := s.chop(p)
		sv, ok := best(evs, func(e clueEval) bool {
			return e.target == p && (e.focus == pc && (e.kind == kindPlay || e.kind == kindSave) || s.cmSave(evs, e))
		})
		return sv.act, ok
	}
	const laterWhy = "救後面玩家更重要的 chop：只剩一個提示，救下一位的 chop 會讓他的沒人能救"
	if ev, ok := best(evs, func(e clueEval) bool { return e.kind == kindPlay && e.nextPlays }); ok {
		if a, ok := laterFirst(ev); ok {
			return a, laterWhy, true
		}
		return ev.act, "下一位的 chop 需要救，先給讓他下一手就打牌的 play clue（Play Clue 優先於 Save Clue）", true
	}
	ev, ok := best(evs, func(e clueEval) bool {
		return e.target == next && (e.focus == chop && (e.kind == kindPlay || e.kind == kindSave) || s.cmSave(evs, e))
	})
	if !ok {
		return game.Action{}, "", false
	}
	if a, ok := laterFirst(ev); ok {
		return a, laterWhy, true
	}
	// 救了之後他的新 chop 是更重要的牌（例如 5），而他仍然沒牌可打：救這張只會讓他改棄那張。
	// 改成保護那張更重要的牌（不動到 chop，例如 5 Stall），讓他棄比較不重要的 chop；沒有這種提示就不處理
	if nc := s.chopAfter(next, append(slices.Clone(ev.touched), ev.cm...)); nc >= 0 && s.needsSave(next, nc) && s.saveWeight(nc) > s.saveWeight(chop) {
		if pv, ok := best(evs, func(e clueEval) bool {
			return e.covers(next, nc) && !e.covers(next, chop) &&
				(e.kind == kindSave || e.kind == kindStall || e.kind == kindPlay || e.kind == kindChopMove && e.cmWaste == 0) && !e.misleads
		}); ok {
			return pv.act, "下一位的 chop 後面是更重要的牌，救 chop 只會讓他改棄那張：先保護那張", true
		}
		return game.Action{}, "", false
	}
	if ev.kind == kindChopMove {
		return ev.act, "緊急救下一位的 chop（" + map[int]string{cmTrash: "Trash Chop Move", cmFive: "5's Chop Move"}[ev.cmKind] + "）", true
	}
	return ev.act, "緊急救下一位的 chop", true
}

// playLetsNextPlay：我打出確定能打的牌之後，下一位就會知道他有牌可打（例如他的 delayed play clue 正等我這張），
// 他也就不會棄 chop，不必花提示救。
func (s *state) playLetsNextPlay(next int) bool {
	slot := s.bestPlay()
	if slot < 0 {
		return false
	}
	t, ok := s.myPoss(s.hands[s.me][slot]).single()
	if !ok {
		return false
	}
	old := s.stacks[t.color()]
	s.stacks[t.color()] = t.rank()
	defer func() { s.stacks[t.color()] = old }()
	return s.willPlay(next)
}

// nextWillClue：Early Game 中，大家棄牌前一定先給完所有 play / save clue（The Early Game），
// 所以下一位只要還有提示可給我看得到的其他玩家，就不會棄牌，不用急著救他的 chop。
// 他要給得出不違反 Good Touch 的提示才算；used 是已經（被我）提示掉的牌，不算。
func (s *state) nextWillClue(next, hints int, used []int) bool {
	if s.midGame || hints <= 0 {
		return false
	}
	for p := range s.n {
		if p == s.me || p == next {
			continue
		}
		for _, id := range s.hands[p] {
			if slices.Contains(used, id) {
				continue
			}
			if t := identOfCard(s.cards[id]); !s.gotten(id) && s.playableNow(t) &&
				!s.gottenAs(s.me, s.me, t, id) && s.cleanClueFor(p, id) {
				return true // 有不違反 Good Touch 的 play clue 可給
			}
		}
		if ch := s.chop(p); ch >= 0 && !slices.Contains(used, ch) && s.needsSave(p, ch) {
			return true // 有 save clue 可給
		}
	}
	return false
}

// doomsLaterChop：ev 會用掉最後一個提示，而之後某位玩家（不是下一位，下一位由 urgentSave 處理）
// chop 上有必須保留的牌、他又沒牌可打；在他之前的玩家都會打牌（不會棄牌換回提示），
// 最後一位也就拿不到提示去救他。回傳那位玩家。
func (s *state) doomsLaterChop(ev clueEval, hints int) (int, bool) {
	if hints > 1 {
		return 0, false
	}
	for k := 2; k < s.n; k++ {
		p := (s.me + k) % s.n
		chop := s.chop(p)
		if chop < 0 || !s.needsSave(p, chop) || s.willPlay(p) || ev.covers(p, chop) {
			continue
		}
		regen := false
		for j := 1; j < k-1; j++ {
			if !s.willPlay((s.me + j) % s.n) {
				regen = true // 他可能棄牌換回提示，讓後面的人去救
			}
		}
		if !regen {
			return p, true
		}
	}
	return 0, false
}

// cleanClueFor：有沒有一個顏色或數字提示能點到 p 的牌 id，而且點到的新牌都不違反 Good Touch
// （不是垃圾、彼此不重複、別處也沒被拿到）。
// 焦點也必須是那張牌（點到 chop 時焦點在 chop；否則是最左邊的新牌），否則對方不會打它。
func (s *state) cleanClueFor(p, id int) bool {
	c := s.cards[id]
	chop := s.chop(p)
	for _, h := range []game.Hint{{Kind: game.HintColor, Color: c.Color}, {Kind: game.HintRank, Rank: c.Rank}} {
		ok := true
		var seen idSet
		focus := -1
		for _, o := range s.hands[p] {
			if s.gotten(o) || !h.Matches(s.cards[o]) {
				continue
			}
			if focus < 0 || o == chop {
				focus = o
			}
			t := identOfCard(s.cards[o])
			if s.trash(t) || seen.has(t) || s.gottenAs(s.me, s.me, t, o) {
				ok = false
			}
			seen |= setOf(t)
		}
		if ok && focus == id {
			return true
		}
	}
	return false
}

// starvesLockedNext：ev 用掉最後一個提示，而下一位手牌全被拿到（沒有 chop）、又沒牌可打：
// 他沒提示可給、也不能棄 chop，只能棄一張已被提示的牌。除非 ev 正好讓他下一手打牌，否則不給，
// 留給我棄牌把提示換回來（讓他能拖延）。
func (s *state) starvesLockedNext(ev clueEval, hints int) bool {
	next := (s.me + 1) % s.n
	if hints > 1 || s.chop(next) >= 0 || s.willPlay(next) || ev.nextPlays || s.chop(s.me) < 0 {
		return false
	}
	return true
}

// takesNextsClue：下一位的 chop 需要救、原本相信他會先給提示（所以沒救），
// 但我給 ev 之後他就沒提示可給了（提示被我用掉，或提示數歸零），而 ev 本身也沒有讓他打牌或救他的 chop。
func (s *state) takesNextsClue(ev clueEval, hints int) bool {
	next := (s.me + 1) % s.n
	chop := s.chop(next)
	if chop < 0 || !s.needsSave(next, chop) || s.willPlay(next) || !s.nextWillClue(next, hints, nil) {
		return false
	}
	if ev.nextPlays || ev.covers(next, chop) {
		return false
	}
	return !s.nextWillClue(next, hints-1, ev.touched)
}
