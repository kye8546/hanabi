package hgroup

import (
	"slices"

	"hanabi/game"
)

type clueKind int

const (
	kindNone     clueKind = iota // 接收者會誤解，或沒有價值
	kindPlay                     // 會讓新牌被打出
	kindSave                     // 把 chop 上必須保留的牌救下來
	kindTempo                    // 只點到已被提示的牌（只在不能棄牌時才用）
	kindStall                    // 5 Stall：點到不在 chop 上的 5，只是救下來、拖延（Level 2）
	kindFix                      // Fix Clue：修正違反 Good Touch 的牌（Level 3）
	kindChopMove                 // Chop Move：保護 chop 上的牌但不點到它（Level 4 Trash / 5's Chop Move）
)

// clueEval 是我（作為給提示者）對一個候選提示的評估。
type clueEval struct {
	act         game.Action
	target      int
	focus       int
	kind        clueKind
	gets        int   // 因此會被打出的新牌（焦點 + prompt / finesse）
	plan        []dep // play clue 照計畫要打的連接牌
	understood  bool  // 實際焦點牌在接收者的解讀之中
	misleads    bool  // 接收者會因此對某張牌寫下錯誤的筆記
	danger      bool  // 錯誤的筆記會讓他把不能打的牌當成能打（立刻打錯）
	costly      bool  // 會被打錯的牌是還需要的牌（不只是閃電，還少一張有用的牌）
	knowsPlay   bool  // tempo：接收者會因此知道焦點可以打
	immediate   bool  // play clue 的焦點現在就能打（不用等連接牌）
	nextPlays   bool  // 下一位玩家會因此在他下一手打牌（被直接提示、被 prompt 或被 finesse）
	touched     []int
	newGood     int   // 非焦點、有用的新牌
	plays       int   // 照計畫進行後會依序打出的牌數（焦點、連接牌、接得上的非焦點新牌）
	trashMarked int   // 非焦點、接收者推得出是垃圾的新牌（不算違反 Good Touch）
	bad         int   // 違反 Good Touch 的新牌（垃圾或重複）
	fixes       int   // Fix Clue：修正了幾張壞掉的牌
	fixesNow    int   // 其中有幾張原本會被打錯
	cmKind      int   // Chop Move 的種類（cmTrash / cmFive）
	cm          []int // 被 Chop Move 的牌
	cmSaves     int   // 其中必須保留的牌（needsSave）
	cmUseful    int   // 其中其他有用的牌
	cmWaste     int   // 其中沒用的牌（垃圾或重複，白白被保護、塞住手牌）
	score       int
}

// covers：這個提示讓 p 的牌 id 不會被棄（點到它，或 Chop Move 它）。
func (e clueEval) covers(p, id int) bool {
	return e.target == p && (slices.Contains(e.touched, id) || slices.Contains(e.cm, id))
}

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

// fiveStallHints：Early Game 至少還有這麼多提示才用 5 Stall 代替棄牌。
var fiveStallHints = 1

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

// chainPlays：焦點 t 與連接牌照計畫打出後，再加上接得上的非焦點新牌（例如「1」點到的其他 1、白色點到的 w1 w2），
// 總共會打出幾張。
func (s *state) chainPlays(t ident, plan []dep, newCards []int, focus int) int {
	after := s.stacks
	n := 0
	for _, d := range plan {
		after[d.id.color()] = max(after[d.id.color()], d.id.rank())
		n++
	}
	after[t.color()] = max(after[t.color()], t.rank())
	n++
	for changed := true; changed; {
		changed = false
		for _, id := range newCards {
			if id == focus {
				continue
			}
			if c := identOfCard(s.cards[id]); c.rank() == after[c.color()]+1 {
				after[c.color()] = c.rank()
				n++
				changed = true
			}
		}
	}
	return n
}

// knownExtraPlays：非焦點的新牌中，接收者自己就知道能接著打的張數：焦點與連接牌打出後，
// 依提示資訊與 Good Touch 它每種可能都能打（例如「1」點到的其他 1；紅色點到的 r2 不算——他只知道是有用的紅牌）。
func (s *state) knownExtraPlays(it interp, t ident, plan []dep) int {
	after := s.stacks
	for _, d := range plan {
		after[d.id.color()] = max(after[d.id.color()], d.id.rank())
	}
	after[t.color()] = max(after[t.color()], t.rank())
	n := 0
	done := map[int]bool{}
	for changed := true; changed; {
		changed = false
		for _, id := range it.newCards {
			if id == it.focus || done[id] {
				continue
			}
			c := identOfCard(s.cards[id])
			if c.rank() != after[c.color()]+1 {
				continue
			}
			q := s.notes[id].poss & it.explicit[id] & s.useful()
			// 鏈上會打出的牌面（例如焦點本身）依 Good Touch 不會是它，不影響
			if !slices.ContainsFunc(q.list(), func(x ident) bool { return x.rank() == after[x.color()]+1 }) ||
				slices.ContainsFunc(q.list(), func(x ident) bool { return x.rank() > after[x.color()]+1 }) {
				continue
			}
			after[c.color()] = c.rank()
			done[id] = true
			n++
			changed = true
		}
	}
	return n
}

// 接收者自己就知道能接著打的非焦點新牌，每張的加分（另外已算在 newGood 裡）。
var extraPlayBonus = 50

// 先給提示再打牌的門檻：提示至少讓這麼多張牌依序打出。
var clueFirstCards = 2

func best(evs []clueEval, ok func(clueEval) bool) (clueEval, bool) {
	var b clueEval
	found := false
	for _, e := range evs {
		if ok(e) && (!found || e.score > b.score) {
			b, found = e, true
		}
	}
	return b, found
}

// allClues 評估所有合法提示。
func (s *state) allClues() []clueEval {
	var out []clueEval
	for k := 1; k < s.n; k++ {
		R := (s.me + k) % s.n
		for _, c := range s.colors {
			if ev, ok := s.evalClue(R, game.Hint{Kind: game.HintColor, Color: c}, k); ok {
				out = append(out, ev)
			}
		}
		for r := game.Rank(1); r <= game.MaxRank; r++ {
			if ev, ok := s.evalClue(R, game.Hint{Kind: game.HintRank, Rank: r}, k); ok {
				out = append(out, ev)
			}
		}
	}
	return out
}

func (s *state) evalClue(R int, h game.Hint, dist int) (clueEval, bool) {
	var touched []int
	for _, id := range s.hands[R] {
		if h.Matches(s.cards[id]) {
			touched = append(touched, id)
		}
	}
	if len(touched) == 0 {
		return clueEval{}, false
	}
	act := game.HintColorTo(R, h.Color)
	if h.Kind == game.HintRank {
		act = game.HintRankTo(R, h.Rank)
	}
	it := s.interpret(s.me, R, h, touched)
	ev := clueEval{act: act, target: R, focus: it.focus, touched: touched}
	if it.fix {
		return s.evalFix(it, ev, h, dist), true
	}
	if it.cmKind != 0 {
		return s.evalChopMove(it, ev, h, dist), true
	}
	t := it.truth

	var seen idSet
	// 焦點先算：和焦點一樣的非焦點新牌是重複的那張（例如其他 1 都打出了，「1」點到兩張 w1）
	order := it.newCards
	if i := slices.Index(order, it.focus); i > 0 {
		order = append([]int{it.focus}, slices.Delete(slices.Clone(order), i, i+1)...)
	}
	for _, id := range order {
		c := identOfCard(s.cards[id])
		switch {
		case id != it.focus && (s.trash(c) || seen.has(c)) && s.seenAsTrash(it, id):
			// 接收者推得出它不可能有用（垃圾，或只可能是焦點的重複）：不會造成誤解，還多知道一張可以先棄的牌
			ev.trashMarked++
		case s.trash(c) || seen.has(c) || s.gottenAs(s.me, s.me, c, id) || s.maybeMine(c):
			ev.bad++
		case id != it.focus:
			ev.newGood++
		}
		seen |= 1 << c
	}
	// 對方會懂：實際牌面在接收者的解讀中，而且大家照實際牌面推得出一致的意思
	// （它符合 save clue，或作為 play clue 的連接確實存在）
	for _, c := range it.cands {
		if c.id == t && (it.saveOK || it.planOK) {
			ev.understood = true
		}
	}
	ev.misleads, ev.danger, ev.costly = s.misleads(R, it, touched)
	if it.stray {
		ev.misleads, ev.danger = true, true // 計畫外的玩家會盲打（或打已提示的牌），多半打錯
	}
	if s.level >= 3 && len(it.newCards) == 0 && !slices.ContainsFunc(touched, func(id int) bool { return it.explicit[id] != s.notes[id].explicit }) {
		// 不給任何新資訊的提示：接收者若已經（私下推得）知道其中某張是他下一手要打的牌，就會當成 Stop Clue
		// 而棄掉它（Level 3 Fix Clue）。我看不到他的私下推論，不冒這個險
		ev.misleads = true
		return ev, true
	}
	if it.selfRisk {
		alt := it
		alt.cands = it.fallback
		m, d, c := s.misleads(R, alt, touched)
		ev.misleads, ev.danger, ev.costly = ev.misleads || m, ev.danger || d, ev.costly || c
	}

	// 只點到已被提示的牌（Tempo Clue）。但若它讓別人 prompt / finesse 打出原本不會打的牌，
	// 就符合 Minimum Clue Value Principle，當作一般 play clue 評估。接收者本來就會照之前的提示打出這些牌
	// （例如之前的提示只剩 self-prompt 的解讀）時，這個提示沒有價值
	if it.tempo && !(ev.understood && it.planOK && len(it.plan) > 0 && !s.alreadyResolved(R, it.focus, t)) {
		ev.kind = kindTempo
		ev.knowsPlay = ev.understood && s.playableNow(t)
		for _, c := range it.cands {
			ev.knowsPlay = ev.knowsPlay && s.playableNow(c.id)
		}
		ev.score = -dist
		return ev, true
	}
	if !ev.understood || s.stomps(touched) {
		return ev, true
	}

	// 實際照哪種解讀進行：能當 save 的就不會有人盲打
	var plan []dep
	playable := it.planOK
	if it.saveOK && len(it.plan) > 0 {
		playable = false
	}
	if playable {
		plan = it.plan
		if !s.finesseClear(R, plan) || !it.agreed || !s.selfPromptTimely(R, it.cands) {
			// 接收者看得懂，但要打連接牌的人會搞錯是誰、打哪張（例如 prompt 到另一張已提示牌），拖延時也不能用
			ev.misleads = true
			return ev, true
		}
	}
	// 被 prompt / finesse 叫出來的連接牌，若可能和我手上不確定的已提示牌重複，同樣違反 Good Touch
	// （Level 10 Sarcastic Finesse：「Normally, potentially duplicating a card is bad」；那種刻意的用法是 Level 10）
	for _, d := range plan {
		if s.holder(d.card) != s.me && s.maybeMine(d.id) {
			ev.bad++
		}
	}
	soon := playable && s.resolvesToPlay(it.cands, plan, t)

	switch {
	case ev.danger:
		return ev, true
	case soon && ev.bad == 0:
		ev.kind = kindPlay
		ev.gets = 1 + len(plan)
		ev.plan = plan
		ev.plays = s.chainPlays(t, plan, it.newCards, it.focus)
		ev.immediate = len(plan) == 0 && s.playableNow(t)
		next := (s.me + 1) % s.n
		ev.nextPlays = R == next && ev.immediate
		for _, d := range plan {
			ev.nextPlays = ev.nextPlays || d.player == next && s.playableNow(d.id)
		}
		ev.score = 100*ev.gets + 10*ev.newGood + 3*ev.trashMarked
		// 接收者自己就知道能接著打的非焦點新牌：比只是「有用」的新牌值錢（例如「1」一次讓兩張 1 能打，勝過藍色點到 b1 與 b4）
		ev.score += extraPlayBonus * s.knownExtraPlays(it, t, plan)
		if it.chopFocus && s.needsSave(R, it.focus) {
			ev.score += 30
		}
		if len(plan) == 0 && s.playableNow(t) {
			ev.score += 3
		}
	case it.chopFocus && s.needsSave(R, it.focus) && !s.saveIsOthersJob(R, t):
		// 接收者知道要保留（就算還不確定是 play 還是 save）就算救下來了
		ev.kind = kindSave
		ev.score = 50 + 10*ev.newGood - 15*ev.bad
		if ev.misleads {
			ev.score -= 10
		}
	case it.saveOK && !it.chopFocus && ev.bad == 0 && !ev.misleads:
		// 5 Stall：點到的都是將來要用的 5，不會被誤會成 play clue
		ev.kind = kindStall
		ev.score = 10 * (1 + ev.newGood)
	default:
		return ev, true
	}
	if h.Kind == game.HintColor {
		ev.score += 2 // 能選的話優先用顏色提示
	}
	// 給下一位的提示：提示後他的 chop 往左移到必須保留的牌上，而他又沒有能立刻打的牌，
	// 他下一手就會棄掉那張牌（Check Team Chops 要看提示之後的 chop）
	if R == (s.me+1)%s.n && !s.willPlay(R) && !s.knowsPlayNow(it) {
		if nc := s.chopAfter(R, touched); nc >= 0 && s.needsSave(R, nc) {
			ev.score -= 100
		}
	}
	ev.score -= dist
	return ev, true
}

// evalFix 評估一個會被當成 Fix Clue 的提示：修正了幾張壞掉的牌；把好牌當成壞牌、或讓人寫下錯誤筆記就是誤導。
func (s *state) evalFix(it interp, ev clueEval, h game.Hint, dist int) clueEval {
	ev.kind = kindFix
	broken := s.brokenCards()
	for y, p := range it.fixPoss {
		t := identOfCard(s.cards[y])
		now, isBroken := broken[y]
		switch {
		case slices.Contains(it.fixed, y) && !s.badCard(y):
			ev.misleads, ev.costly = true, true // 好牌被當成沒用的牌，會被棄掉
		case p != 0 && !p.has(t):
			ev.misleads = true
		case isBroken:
			ev.fixes++
			if now {
				ev.fixesNow++
			}
		}
		if p != 0 && !slices.Contains(it.fixed, y) && !s.playableNow(t) &&
			!slices.ContainsFunc(p.list(), func(i ident) bool { return !s.playableNow(i) }) {
			ev.misleads, ev.danger = true, true
		}
	}
	for _, id := range it.newCards {
		p := nonFocusPoss(s.notes[id].poss&it.explicit[id], s.freshUseful(it.receiver, s.me, id), 0)
		c := identOfCard(s.cards[id])
		switch {
		case !p.has(c) && p != 0 && !slices.ContainsFunc(p.list(), func(i ident) bool { return !s.playableNow(i) }):
			ev.misleads, ev.danger = true, true // 他會以為能打而立刻打錯
		case !p.has(c) || s.trash(c) || s.gottenAs(s.me, s.me, c, id) || s.maybeMine(c):
			// 一起點到垃圾或重複的牌：違反 Good Touch（筆記會是錯的，之後再修），但比眼前的閃電好（Misplay Cost Principle）
			ev.bad++
		default:
			ev.newGood++
		}
	}
	ev.understood = !ev.misleads
	ev.score = 100*ev.fixesNow + 30*ev.fixes + 10*ev.newGood - 15*ev.bad - dist
	if h.Kind == game.HintColor {
		ev.score += 2
	}
	return ev
}

// evalChopMove 評估一個會被當成 Chop Move 的提示：被保護的牌有幾張必須保留、幾張白白被保護。
// 5's Chop Move 點到的 5 本身也被救下（記成任何 5）。
func (s *state) evalChopMove(it interp, ev clueEval, h game.Hint, dist int) clueEval {
	R := it.receiver
	ev.kind, ev.cmKind, ev.cm = kindChopMove, it.cmKind, it.cm
	var seen idSet
	useless := func(id int) bool {
		c := identOfCard(s.cards[id])
		u := s.trash(c) || seen.has(c) || s.gottenAs(s.me, s.me, c, id) || s.maybeMine(c)
		seen |= setOf(c)
		return u
	}
	for _, id := range it.cm {
		switch {
		case useless(id):
			ev.cmWaste++
		case s.needsSave(R, id):
			ev.cmSaves++
		default:
			ev.cmUseful++
		}
	}
	if it.cmKind == cmFive {
		ev.misleads, ev.danger, ev.costly = s.misleads(R, it, ev.touched)
		for _, id := range it.newCards {
			if useless(id) {
				ev.bad++
			} else {
				ev.newGood++
			}
		}
	}
	ev.understood = !ev.misleads && ev.cmWaste == 0
	ev.score = 45 + 40*(ev.cmSaves-1) + 10*ev.cmUseful + 10*ev.newGood - 15*ev.bad - 20*ev.cmWaste - dist
	if h.Kind == game.HintColor {
		ev.score += 2
	}
	return ev
}

// cmSave：e 是能當 save clue 用的 Chop Move——保護了接收者 chop 上必須保留的牌，也沒有白白保護沒用的牌。
// Trash Chop Move 不能用在可以直接用 save clue 救的牌上（直接提示總是多給一些資訊），
// 除非 chop 上連著好幾張必須保留的牌（一次救不完）。
func (s *state) cmSave(evs []clueEval, e clueEval) bool {
	if e.kind != kindChopMove || e.misleads || e.cmWaste > 0 || e.bad > 0 {
		return false
	}
	chop := s.chop(e.target)
	if chop < 0 || !slices.Contains(e.cm, chop) || !s.needsSave(e.target, chop) {
		return false
	}
	// Chop Move 後他的手牌全被提示或 Chop Move（鎖手）、又沒牌可打：他不能棄牌，只能拖延或棄可能有用的牌
	if s.chopAfter(e.target, append(slices.Clone(e.touched), e.cm...)) < 0 && !s.willPlay(e.target) {
		return false
	}
	if e.cmKind == cmTrash && e.cmSaves < 2 {
		_, direct := best(evs, func(d clueEval) bool {
			return d.target == e.target && d.focus == chop && (d.kind == kindPlay || d.kind == kindSave) && !d.misleads
		})
		return !direct
	}
	return true
}

// ocmSkip（Level 4 The Order Chop Move）：p 打出被提示的 1（x）時，照 Level 3 的順序跳過了幾張。
// 只算「不知道是哪張」的 1：x 的牌面已知、x 是之前被 finesse 的、被點到的次數和其他 1 不同、
// 被跳過的 1 已知沒用（playOrder 不含它）時都不算。不是 Order Chop Move 時回傳 0。
func (s *state) ocmSkip(p, x int) int {
	unknownOne := func(id int) bool {
		k := s.knownPoss(p, id)
		return k&^rankSet(1) == 0 && k.count() > 1
	}
	n := s.notes[x]
	if s.level < 4 || !n.clued || n.finessed || n.bad || !unknownOne(x) || s.mayBeFinessed(x) {
		return 0
	}
	skip := 0
	for _, id := range s.playOrder(p) {
		if id == x {
			return skip
		}
		if m := s.notes[id]; m.clued && !m.finessed && m.touches == n.touches && unknownOne(id) {
			skip++
		}
	}
	return 0
}

// mayBeFinessed：某個提示的可能解讀（或我推得的計畫）要 x 被 finesse 盲打。我看不到焦點時不一定確定它被 finesse
// （例如還有直接解讀），但它可能是之前的提示 finesse 的牌，打出它就不是 Order Chop Move。
func (s *state) mayBeFinessed(x int) bool {
	isX := func(d dep) bool { return d.card == x && d.finesse }
	for _, rec := range s.recs {
		if slices.ContainsFunc(rec.plan, isX) {
			return true
		}
		for _, c := range append(slices.Clone(rec.cands), rec.fallback...) {
			if slices.ContainsFunc(c.deps, isX) {
				return true
			}
		}
	}
	return false
}

// orderChopMove：選一張要打的 1，讓 Order Chop Move 保護後面某位 chop 上必須保留的牌（不花提示）。
// 優先保護沒牌可打（快要棄牌）、又比較近的人。回傳要打的位置與被保護的玩家；沒有則回傳 -1, -1。
func (s *state) orderChopMove() (slot, target int) {
	slot, target = -1, -1
	bestScore := 0
	for _, x := range s.playOrder(s.me) {
		k := s.ocmSkip(s.me, x)
		if k <= 0 || k >= s.n {
			continue
		}
		p := (s.me + k) % s.n
		chop := s.chop(p)
		if chop < 0 || !s.needsSave(p, chop) {
			continue
		}
		score := 10*s.saveWeight(chop) - k
		if !s.willPlay(p) {
			score += 100
		}
		if score > bestScore {
			slot, target, bestScore = slices.Index(s.hands[s.me], x), p, score
		}
	}
	return slot, target
}

// nextMisplaysAfter：我打出 slot 這張（每種可能都模擬）後，下一位下一手要打的牌是壞掉的牌（brokenCards）、
// 實際卻不能打——例如 save clue 違反 Good Touch 點到的重複 g2，在 g3 打出後被他當成 g4。回傳這些牌。
// 我打之前他本來就會打錯的不算（那是 decide 裡 fixesNow 的 Fix Clue 要處理的，和我打不打無關）。
func (s *state) nextMisplaysAfter(slot int) []int {
	next := (s.me + 1) % s.n
	broken := s.brokenCards()
	if s.nextMisplay(next, broken) >= 0 {
		return nil
	}
	var risky []int
	for _, t := range s.myPoss(s.hands[s.me][slot]).list() {
		old := s.stacks[t.color()]
		s.stacks[t.color()] = t.rank()
		if y := s.nextMisplay(next, broken); y >= 0 && !slices.Contains(risky, y) {
			risky = append(risky, y)
		}
		s.stacks[t.color()] = old
	}
	return risky
}

// nextMisplay：p 依他自己推得出的筆記（看得到的張數、Good Touch，已打出的牌面也排除）下一手要打的牌，
// 若是壞掉、實際不能打的牌就回傳它，否則回傳 -1。
func (s *state) nextMisplay(p int, broken map[int]bool) int {
	old := s.overlay
	s.overlay = map[int]idSet{}
	for _, y := range s.hands[p] {
		q := s.deduce(p, y)
		if u := q & s.useful(); s.gotten(y) && u != 0 {
			q = u
		}
		s.overlay[y] = q
	}
	y := s.nextPlay(p)
	s.overlay = old
	if _, ok := broken[y]; ok && !s.playableNow(identOfCard(s.cards[y])) {
		return y
	}
	return -1
}

// badCard：（我看得到的）這張被拿到的牌是沒用的：垃圾，或和別處被拿到的牌重複。
func (s *state) badCard(y int) bool {
	t := identOfCard(s.cards[y])
	if s.trash(t) {
		return true
	}
	return s.dupOf(y) >= 0
}

// dupOf 回傳和 y 重複的另一張被拿到的牌（我看得到它，或是我手上確定是那張的牌）；沒有則回傳 -1。
func (s *state) dupOf(y int) int {
	t := identOfCard(s.cards[y])
	for q, h := range s.hands {
		for _, z := range h {
			if z == y || !s.live(z) {
				continue
			}
			if c, ok := s.see(s.me, z); ok && c == t || q == s.me && s.myPoss(z) == setOf(t) {
				return z
			}
		}
	}
	return -1
}

// brokenCards 找出隊友手上違反 Good Touch、需要 Fix Clue 的牌（值為 true 表示他會把它當成能打的牌而打錯）：
// 他以為能打但其實不能打、其實是垃圾、筆記和實際牌面矛盾，或和別處被拿到的牌重複
// （兩張的持有者都不知道時；有人知道的話他會 Sarcastic Discard）。
func (s *state) brokenCards() map[int]bool {
	out := map[int]bool{}
	knows := func(z int, t ident) bool {
		if s.holder(z) == s.me {
			return s.myPoss(z) == setOf(t)
		}
		return s.notes[z].poss == setOf(t)
	}
	for q, h := range s.hands {
		if q == s.me {
			continue
		}
		for _, y := range h {
			if !s.live(y) {
				continue
			}
			t := identOfCard(s.cards[y])
			p := s.notes[y].poss
			wrongPlay := s.holderKnowsPlayable(y) && !s.playableNow(t)
			dup := false
			if z := s.dupOf(y); z >= 0 && !knows(y, t) && !knows(z, t) {
				dup = true
				if s.holderKnowsPlayable(y) && (s.holder(z) == s.me && s.knownPlayable(z) || s.holderKnowsPlayable(z)) {
					wrongPlay = true // 兩張都會被打，後打的那張會打錯
				}
			}
			if wrongPlay || dup || s.trash(t) || !p.has(t) {
				out[y] = wrongPlay
			}
		}
	}
	return out
}

// sarcasticSlot（Level 3 The Sarcastic Discard）：我手上被拿到、確定是 t 的牌，而別人手上已被拿到的牌實際就是 t：
// 我是先發現重複的人，棄掉它（他會知道自己有 t）。提示滿 8 不能棄牌；鎖手時棄牌會被當成被迫的，不用。
func (s *state) sarcasticSlot(v game.PlayerView) int {
	if v.Hints >= game.MaxHints || s.chop(s.me) < 0 {
		return -1
	}
	for slot, x := range s.hands[s.me] {
		t, ok := s.myPoss(x).single()
		if !s.live(x) || !ok || s.trash(t) {
			continue
		}
		for q, h := range s.hands {
			if q == s.me {
				continue
			}
			for _, y := range h {
				// 他已經知道就不必（他會自己打出，我這張之後就是已知沒用的牌）；他的筆記沒有 t 時，棄牌也告訴不了他
				if p := s.notes[y].poss; s.live(y) && identOfCard(s.cards[y]) == t && p.has(t) && p != setOf(t) &&
					(!s.playableNow(t) || s.sdCands(q, t) == 1) {
					return slot
				}
			}
		}
	}
	return -1
}

// sdCands：Sarcastic Discard t 之後，q 手上有幾張可能是 t 的被拿到的牌（共同筆記）。
func (s *state) sdCands(q int, t ident) int {
	n := 0
	for _, id := range s.hands[q] {
		if s.live(id) && s.notes[id].poss.has(t) {
			n++
		}
	}
	return n
}

// alreadyResolved：之前給 R 的提示（焦點是 focus）在我這一手之後只剩實際牌面 t 的解讀——需要我盲打、但我不會打的解讀
// 會被排除；都排除後就輪到 self-prompt / self-finesse 的 fallback。R 不用再提示就會照它打牌。
func (s *state) alreadyResolved(R, focus int, t ident) bool {
	for i := len(s.recs) - 1; i >= 0; i-- {
		rec := s.recs[i]
		if rec.focus != focus || rec.target != R {
			continue
		}
		cs := slices.DeleteFunc(slices.Clone(rec.cands), func(c cand) bool { return s.failsOnMyTurn(rec, c) })
		if len(cs) == 0 {
			cs = rec.fallback
		}
		return len(cs) > 0 && !slices.ContainsFunc(cs, func(c cand) bool { return c.id != t || c.save })
	}
	return false
}

// stomps：提示點到某張「某個提示的可能解讀裡要被 finesse 盲打」的牌（Level 2 Stomping on a Finesse：
// 直接提示本來就會被盲打的牌，浪費一個提示）。例如我收到的提示可能是 Reverse Finesse 下一位，
// 我就該等他盲打，不是直接提示他那張。
// 只看我是接收者、看不到焦點牌的提示（看得到時，計畫裡的 finesse 牌已經記成被 finesse，不會是新牌）。
func (s *state) stomps(touched []int) bool {
	for _, rec := range s.recs {
		if rec.target != s.me {
			continue
		}
		for _, c := range rec.cands {
			for _, d := range c.deps {
				if d.finesse && d.player != s.me && !s.notes[d.card].clued && slices.Contains(touched, d.card) {
					return true
				}
			}
		}
	}
	return false
}

// knowsPlayNow：提示後接收者確定焦點現在就能打（每種解讀都能直接打）。
func (s *state) knowsPlayNow(it interp) bool {
	for _, c := range it.cands {
		if c.save || len(c.deps) > 0 || !s.playableNow(c.id) {
			return false
		}
	}
	return len(it.cands) > 0
}

// chopAfter 回傳 R 在提示點到（或 Chop Move）touched 之後的 chop。
func (s *state) chopAfter(R int, touched []int) int {
	h := s.hands[R]
	for i := len(h) - 1; i >= 0; i-- {
		if !s.gotten(h[i]) && !s.notes[h[i]].cm && !slices.Contains(touched, h[i]) {
			return h[i]
		}
	}
	return -1
}

// focusSingle 回傳接收者在提示後確定的焦點牌面（不確定則為空集合）。
func (s *state) focusSingle(it interp) idSet {
	var u idSet
	for _, c := range it.cands {
		u |= setOf(c.id)
	}
	p := s.notes[it.focus].poss & it.explicit[it.focus]
	if q := p & u; q != 0 {
		p = q
	}
	if f, ok := p.single(); ok {
		return setOf(f)
	}
	return 0
}

// seenAsTrash：提示後接收者推得出這張非焦點新牌不可能有用（只會是垃圾或焦點的重複）。
func (s *state) seenAsTrash(it interp, id int) bool {
	p := s.notes[id].poss & it.explicit[id]
	return p&s.freshUseful(it.receiver, s.me, id)&^s.focusSingle(it) == 0
}

// maybeMine：我手上有被拿到、但我不確定是什麼的牌可能是 c。這時提示 c 可能造成重複，
// 依 Good Touch 也算違反（Level 10 Sarcastic Finesse：「potentially duplicating a card is bad -
// it violates Good Touch Principle」；Level 12 Duplication Responsibility）。
func (s *state) maybeMine(c ident) bool {
	for _, id := range s.hands[s.me] {
		if s.gotten(id) && s.myPoss(id).has(c) {
			return true
		}
	}
	return false
}

// selfPromptTimely：要接收者 self-prompt 時，他只知道被 prompt 的牌是「某幾種之一」；
// 必須每一種現在都能打，他才知道可以立刻打（否則他不知道要等誰，可能打錯）。
// The Prompt：「It is the responsibility of the player giving the clue to ensure that all those
// previous cards will play successfully.」
func (s *state) selfPromptTimely(R int, cands []cand) bool {
	for _, c := range cands {
		for _, d := range c.deps {
			if d.player == R && !s.playableNow(d.id) {
				return false
			}
		}
	}
	return true
}

// misleads：提示後接收者會不會對某張牌寫下錯誤的筆記（實際牌面被排除），之後可能因此打錯；
// danger 表示錯誤的筆記全是現在就能打的牌（他下一回合就會打錯）；costly 表示那張牌還有用。
func (s *state) misleads(R int, it interp, touched []int) (wrong, danger, costly bool) {
	var u idSet
	for _, c := range it.cands {
		u |= 1 << c.id
	}
	check := func(id int, p idSet) {
		if p.has(identOfCard(s.cards[id])) {
			return
		}
		wrong = true
		all := p != 0
		for _, i := range p.list() {
			all = all && s.playableNow(i)
		}
		if all {
			danger = true
			costly = costly || !s.trash(identOfCard(s.cards[id]))
		}
	}
	for _, id := range touched {
		p := s.notes[id].poss & it.explicit[id]
		switch {
		case id == it.focus && p&u != 0:
			p &= u
		case id == it.focus:
			p = s.touchedUseful(R, s.me, id, p)
		case slices.Contains(it.newCards, id) && id != it.focus:
			p = nonFocusPoss(p, s.freshUseful(R, s.me, id), s.focusSingle(it))
		}
		check(id, p)
	}
	// self-prompt / self-finesse：每種解讀都要接收者同一張牌
	card := -1
	var ids idSet
	for _, c := range it.cands {
		i := slices.IndexFunc(c.deps, func(d dep) bool { return d.player == R })
		if i < 0 || s.level < 2 && len(c.deps) != 1 || card >= 0 && c.deps[i].card != card {
			return wrong, danger, costly
		}
		card = c.deps[i].card
		ids |= 1 << c.deps[i].id
	}
	if card >= 0 {
		check(card, s.notes[card].poss&it.explicit[card]&ids)
	}
	return wrong, danger, costly
}

// finesseClear：finesse 對象以外，給提示者與接收者之間（Level 2 起是接收者以外的所有人）
// 沒有別人的 finesse position 也是同一張牌，否則大家會搞不清楚是誰被 finesse。
func (s *state) finesseClear(R int, plan []dep) bool {
	for _, d := range plan {
		if !d.finesse {
			continue
		}
		for k := 1; k < s.n; k++ {
			p := (s.me + k) % s.n
			if p == R {
				if s.level < 2 {
					break
				}
				continue
			}
			if fp := s.finessePos(p); p != d.player && fp >= 0 && identOfCard(s.cards[fp]) == d.id {
				return false
			}
		}
	}
	return true
}

// resolvesToPlay：照實際的連接進行後，接收者的每種剩餘解讀都會變成可打（不需要更多提示）。
func (s *state) resolvesToPlay(cands []cand, plan []dep, t ident) bool {
	after := s.stacks
	after[t.color()] = t.rank() - 1
	inPlan := func(d dep) bool {
		return slices.ContainsFunc(plan, func(p dep) bool { return p.card == d.card })
	}
	mayBeSave := slices.ContainsFunc(cands, func(c cand) bool { return c.save })
	proves := func(d dep) bool { return !mayBeSave || d.finesse }
	alive := map[ident]bool{}
	resolves := map[ident]bool{}
	for _, c := range cands {
		if c.id.rank() <= after[c.id.color()] {
			continue // 連接牌打出後就變垃圾，接收者會排除
		}
		ok := true
		for _, d := range c.deps {
			if !inPlan(d) {
				ok = false
			}
		}
		// 有人照計畫打出連接牌時，不需要那張牌的解讀會被排除。但可能是 save clue 時，prompt 的牌被打出證明不了什麼
		// （save clue 不會觸發 prompt，他可能本來就要打），只有 finesse 的盲打才算（和 checkDeps 一致）
		if slices.ContainsFunc(plan, proves) && !slices.ContainsFunc(c.deps, func(d dep) bool { return inPlan(d) && proves(d) }) {
			continue
		}
		if !ok && slices.ContainsFunc(c.deps, func(d dep) bool { return d.finesse }) {
			continue // 不在計畫中的 finesse 不會發生，接收者會排除
		}
		alive[c.id] = true
		if ok && !c.save {
			resolves[c.id] = true
		}
	}
	for id := range alive {
		if !resolves[id] {
			return false
		}
	}
	return alive[t]
}

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

// saveIsOthersJob：要救的 t 可能和我手上不確定的已提示牌重複（`maybeMine`），而在 R 行動前還有別人
// 手上可能重複的已提示牌比我少：交給他救（Level 12 Duplication Responsibility 第一段：手上有已提示的 2 的人
// 不該負責救 2，以免違反 Good Touch；他看得到我那張，多半根本不用救）。
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
	return false
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
		one := s.level >= 3 && poss&^rankSet(1) == 0
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
