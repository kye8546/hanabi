package hgroup

import (
	"slices"

	"hanabi/game"
)

func (s *state) decide(v game.PlayerView) game.Action {
	hand := s.hands[s.me]

	// 1. 被 finesse 的牌要立刻盲打（Finesse Urgency）
	for slot, id := range hand {
		if s.notes[id].finessed && s.knownPlayable(id) {
			s.why = "finesse 盲打"
			return game.Play(slot)
		}
	}

	var evs []clueEval
	if v.Hints > 0 {
		evs = s.allClues()
	}

	// Level 4 The Order Chop Move：照順序該打的 1 跳過幾張，就保護後面第幾位的 chop，不花提示。
	// 能保護下一位必須保留的 chop 時，比給他提示好
	ocm, ocmTarget := -1, -1
	if s.level >= 4 {
		ocm, ocmTarget = s.orderChopMove()
	}
	ocmWhy := "Order Chop Move：不照順序打 1，保護後面玩家 chop 上必須保留的牌"
	// Level 3 Misplay Cost Principle：我打出這張後，下一位手上壞掉的牌會看起來能打（他下一手就打錯，
	// 輪到他之前沒人能修）：先給 Fix Clue，我的牌晚點打。沒有提示可修時，改打別張不會害他打錯的牌；
	// 都不行就先不打（之後棄牌換回提示，讓別人修）。ok 為 false 表示這手不打牌
	playOrFix := func(slot int, why string) (game.Action, bool) {
		risky := s.nextMisplaysAfter(slot)
		if s.level < 3 || len(risky) == 0 {
			s.why = why
			return game.Play(slot), true
		}
		if ev, ok := best(evs, func(e clueEval) bool {
			return e.kind == kindFix && !e.misleads &&
				slices.ContainsFunc(risky, func(y int) bool { return e.covers(e.target, y) })
		}); ok {
			s.why = "Fix Clue：我打出能打的牌後，下一位會把壞掉的牌當成能打而打錯，先修正"
			return ev.act, true
		}
		for _, id := range s.playOrder(s.me) {
			if sl := slices.Index(s.hands[s.me], id); sl != slot && s.ocmSkip(s.me, id) == 0 && len(s.nextMisplaysAfter(sl)) == 0 {
				s.why = why + "（另一張會讓下一位把壞掉的牌當成能打，晚點再打）"
				return game.Play(sl), true
			}
		}
		return game.Action{}, false
	}
	if ocm >= 0 && ocmTarget == (s.me+1)%s.n {
		if a, ok := playOrFix(ocm, ocmWhy); ok {
			return a
		}
	}

	// 2. 檢查隊友的 chop：下一位要棄牌而 chop 上是必須保留的牌，就算自己有牌可打也先救
	if a, why, ok := s.urgentSave(evs, v.Hints, true); ok {
		s.why = why
		return a
	}

	if s.level >= 3 {
		// Level 3 The Sarcastic Discard：我確定手上被拿到的牌和別人已被拿到的牌重複，棄掉它讓那人知道他有這張
		if slot := s.sarcasticSlot(v); slot >= 0 {
			s.why = "Sarcastic Discard：這張和別人已被提示的牌重複"
			return game.Discard(slot)
		}
		// Misplay Cost Principle：用一個提示阻止一次打錯是值得的
		if ev, ok := best(evs, func(e clueEval) bool { return e.kind == kindFix && e.fixesNow > 0 && !e.misleads }); ok {
			s.why = "Fix Clue：阻止隊友打錯"
			return ev.act
		}
	}

	// 3. 打出確定能打的牌。但有一次讓多張牌能打、又不違反 Good Touch 的 play clue，而晚一步就會因為
	// 別人先提示了其中某張的另一份而變得違反 Good Touch，或是只有我能給的 finesse 時，先給提示：
	// 我的牌留著不會跑掉（除非別人已提示的牌正等著接我這張）
	if ocm >= 0 {
		if a, ok := playOrFix(ocm, ocmWhy); ok {
			return a
		}
	}
	if slot := s.bestPlay(); slot >= 0 {
		if s.level >= 3 && len(s.nextMisplaysAfter(slot)) > 0 {
			if a, ok := playOrFix(slot, "打出確定能打的牌"); ok {
				return a
			}
		} else {
			if ev, ok := s.clueBeforePlay(evs, v, slot); ok {
				if s.onlyMyFinesse(ev) {
					s.why = "先給只有我能給的 finesse（自己確定能打的牌晚點打）"
				} else {
					s.why = "先給讓多張牌能打的 play clue（晚一步就會因重複而違反 Good Touch），自己確定能打的牌晚點打"
				}
				return ev.act
			}
			s.why = "打出確定能打的牌"
			return game.Play(slot)
		}
	}

	// 4. Play clue 優先於 save clue；5. save clue
	for _, k := range []clueKind{kindPlay, kindSave} {
		if ev, ok := best(evs, func(e clueEval) bool {
			return (e.kind == k || k == kindSave && s.cmSave(evs, e)) && !s.starvesLockedNext(e, v.Hints)
		}); ok {
			// 剛才相信下一位會先給提示而沒救他的 chop；若我這個提示正好用掉他能給的提示，就改成先救
			if s.takesNextsClue(ev, v.Hints) {
				if a, why, ok := s.urgentSave(evs, v.Hints, false); ok {
					s.why = why + "（我要給的提示會用掉他唯一能給的提示）"
					return a
				}
			}
			// 我用掉最後一個提示後，後面某位的 chop 就沒人能救了：那只能由我現在救（Check Team Chops）
			if p, ok := s.doomsLaterChop(ev, v.Hints); ok {
				chop := s.chop(p)
				if sv, ok := best(evs, func(e clueEval) bool {
					return e.target == p && (e.focus == chop && (e.kind == kindPlay || e.kind == kindSave) || s.cmSave(evs, e))
				}); ok {
					s.why = "救後面玩家的 chop：我用掉最後一個提示的話，輪到他之前沒人能救"
					return sv.act
				}
			}
			s.why = map[clueKind]string{kindPlay: "play clue", kindSave: "save clue"}[k]
			if ev.kind == kindChopMove {
				s.why = map[int]string{cmTrash: "Trash Chop Move", cmFive: "5's Chop Move"}[ev.cmKind] + "：保護 chop 上必須保留的牌"
			}
			return ev.act
		}
	}

	// Level 3：違反 Good Touch 的牌要儘早修正，不論會不會馬上被打錯
	if s.level >= 3 {
		if ev, ok := best(evs, func(e clueEval) bool { return e.kind == kindFix && e.fixes > 0 && !e.misleads }); ok {
			s.why = "Fix Clue：修正違反 Good Touch 的牌"
			return ev.act
		}
	}

	// Early Game 沒有 play / save clue 可給時，可以用 5 Stall 代替棄牌，延長 Early Game（Level 2）。
	// 但手上有已被提示的已知垃圾時先棄它：不會結束 Early Game，還換回一個提示
	if !s.midGame && v.CanDiscard() {
		if slot := s.trashSlot(true); slot >= 0 {
			s.why = "棄已知沒用的牌（不會結束 Early Game，先於 5 Stall）"
			return game.Discard(slot)
		}
	}
	if !s.midGame && v.Hints >= fiveStallHints {
		if ev, ok := best(evs, func(e clueEval) bool { return e.kind == kindStall }); ok {
			s.why = "5 Stall：Early Game 沒有其他提示可給，救下不在 chop 上的 5 代替棄牌"
			return ev.act
		}
	}

	// 牌堆已空的最後一輪：棄牌換不到新牌，閃電還夠時就賭一張最可能能打的被提示牌
	if v.DeckSize == 0 && v.Strikes < game.MaxStrikes-1 {
		if slot := s.likelyPlay(0.5); slot >= 0 {
			s.why = "最後一輪，打出最可能能打的牌"
			return game.Play(slot)
		}
	}

	// 6. 提示 8 個不能棄牌：只好給影響最小的提示
	if v.Hints >= game.MaxHints {
		s.why = "提示滿 8，拖延提示"
		return s.stall(evs)
	}

	// 7. 棄牌；但終局時棄牌會讓滿分變得不可能，就改給不會造成誤解的提示拖延（Level 8 Burning，所有等級都用）
	if s.mustBurn(v) {
		if ev, ok := s.burnClue(evs); ok {
			s.why = "終局拖延：棄牌會抽掉牌堆、來不及滿分"
			return ev.act
		}
	}
	s.why = "棄牌"
	return s.discard(evs)
}

// clueBeforePlay：見 decide 第 3 步。
func (s *state) clueBeforePlay(evs []clueEval, v game.PlayerView, slot int) (clueEval, bool) {
	if v.Hints < 2 {
		return clueEval{}, false
	}
	t, ok := s.myPoss(s.hands[s.me][slot]).single()
	if !ok {
		return clueEval{}, false
	}
	// 別人手上已被拿到的牌可能是接在我這張後面的：先打，免得他們等。只有我能給的 finesse 錯過就沒了，
	// 只在他確定那張就是接我這張的牌時才讓路（他還不確定的話，晚一輪打也不會讓他多等）
	nxt := identOf(t.color(), t.rank()+1)
	waits, surely := false, false
	for p, h := range s.hands {
		for _, id := range h {
			if p != s.me && t.rank() < game.MaxRank && s.gotten(id) && s.notes[id].poss.has(nxt) {
				waits = true
				if u, ok := s.notes[id].poss.single(); ok && u == nxt {
					surely = true
				}
			}
		}
	}
	return best(evs, func(e clueEval) bool {
		return e.kind == kindPlay && e.plays >= clueFirstCards && e.bad == 0 && !e.misleads &&
			(!waits && s.perishable(e) || !surely && s.onlyMyFinesse(e))
	})
}

// onlyMyFinesse：這個 play clue 要 finesse 某人，而且除了我以外沒有人能給（其他玩家都是接收者或要打連接牌的人，
// 看不到自己的牌）。原文 Level 20 The Occupied Play Clue 開頭的一般原則：手上有能打的牌時通常先打、讓別人給提示，
// 但只有自己能給的 finesse / prompt 這種有時效的提示，就先給、牌晚點打（例如 3 人局只有我看得到
// 「提示 Cathy 的 g2、Reverse Finesse Alice 的 g1」，打完牌後 Cathy 多半就直接提示 Alice 的 g1 了）。
func (s *state) onlyMyFinesse(e clueEval) bool {
	if !slices.ContainsFunc(e.plan, func(d dep) bool { return d.finesse }) {
		return false
	}
	for q := range s.n {
		if q != s.me && q != e.target && !slices.ContainsFunc(e.plan, func(d dep) bool { return d.player == q }) {
			return false
		}
	}
	return true
}

// perishable：這個提示點到的某張新牌，在第三人手上有沒被提示、現在就能打的同一張。
// 別人很可能先提示那張，之後這個提示就會點到重複的牌而違反 Good Touch（例如 Alice 有 r1、Cathy 有 r1 y1 b1：
// 不趁現在給 Cathy「1」，等別人給了 Alice 的 r1，就只能分別提示 y1、b1）。
func (s *state) perishable(e clueEval) bool {
	for _, id := range e.touched {
		if s.gotten(id) {
			continue
		}
		c := identOfCard(s.cards[id])
		if !s.playableNow(c) {
			continue
		}
		for q, h := range s.hands {
			if q == s.me || q == e.target {
				continue
			}
			for _, o := range h {
				if !s.gotten(o) && identOfCard(s.cards[o]) == c {
					return true
				}
			}
		}
	}
	return false
}

// 先給提示再打牌的門檻：提示至少讓這麼多張牌依序打出。
var clueFirstCards = 2

// stall：不能棄牌時，依序選 tempo clue（讓對方知道能打）、不違反 Good Touch 的提示、影響最小的提示。
func (s *state) stall(evs []clueEval) game.Action {
	tier := func(e clueEval) int {
		switch {
		case e.danger && e.costly:
			return 0 // 他會立刻打錯一張還需要的牌（例如 Level 2 起被當成 self-finesse 而盲打）
		case e.danger:
			return 1 // 會打錯，但打掉的是沒用的牌
		case e.misleads, e.kind == kindChopMove && e.cmWaste > 0:
			// Chop Move 了沒用的牌：他永遠不會棄那些牌，手牌被塞住，之後只能拖延或棄可能有用的牌
			return 2
		case e.kind == kindTempo && e.knowsPlay:
			return 7
		case e.understood && e.bad == 0:
			return 6
		case e.kind == kindTempo:
			return 5
		case e.understood:
			return 4
		}
		return 3
	}
	var b clueEval
	bt := -1
	for _, e := range evs {
		if t := tier(e); t > bt || (t == bt && e.bad < b.bad) {
			b, bt = e, t
		}
	}
	return b.act
}

// trashSlot 回傳我手上已知沒用的牌（由右而左找第一張）；clued 為 true 時只找已被拿到的牌
// （沒被提示的牌就算我推得出是垃圾，別人看來也是棄掉不明的牌，會結束 Early Game）。沒有則回傳 -1。
func (s *state) trashSlot(clued bool) int {
	hand := s.hands[s.me]
	for slot := len(hand) - 1; slot >= 0; slot-- {
		if s.knownTrash(hand[slot]) && (!clued || s.gotten(hand[slot])) {
			return slot
		}
	}
	return -1
}

// discard：先棄已知沒用的牌，再棄 chop；手牌全被提示（鎖手）時給個提示，
// 真的不行才棄最不可能是 critical 的那張。
func (s *state) discard(evs []clueEval) game.Action {
	hand := s.hands[s.me]
	if slot := s.trashSlot(false); slot >= 0 {
		s.why = "棄已知沒用的牌"
		return game.Discard(slot)
	}
	if chop := s.chop(s.me); chop >= 0 {
		return game.Discard(slices.Index(hand, chop))
	}
	if len(evs) > 0 {
		s.why = "鎖手，給提示"
		return s.stall(evs)
	}
	s.why = "鎖手棄牌"
	bestSlot, bestRisk := len(hand)-1, 2.0
	for slot := len(hand) - 1; slot >= 0; slot-- {
		p := s.myPoss(hand[slot]).list()
		crit := 0
		for _, i := range p {
			if s.critical(i) {
				crit++
			}
		}
		if risk := float64(crit) / float64(max(len(p), 1)); risk < bestRisk {
			bestSlot, bestRisk = slot, risk
		}
	}
	return game.Discard(bestSlot)
}
