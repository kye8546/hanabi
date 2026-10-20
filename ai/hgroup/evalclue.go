// 給提示前的評估：模擬接收者與旁觀者的解讀，判斷會不會誤導、能讓幾張牌打出，並打分數（evalClue）。

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
			if c == game.Rainbow {
				continue // 彩虹色不能提示
			}
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
	if it.cmKind == cmTrash || it.cmKind == cmFive {
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
		case s.trash(c) || seen.has(c) || s.gottenAs(s.me, s.me, c, id) || s.maybeMine(c) || s.awaited(c, id) || s.pendingLayer(c, id):
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
	if s.focusUnclear(touched) {
		ev.misleads = true
		return ev, true
	}
	if s.knows(lvFixClue) && len(it.newCards) == 0 && !slices.ContainsFunc(touched, func(id int) bool { return it.explicit[id] != s.notes[id].explicit }) {
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
	if it.cmKind == cmTempo {
		// Level 6 Tempo Clue Chop Move：焦點照常打出，另外 Chop Move 接收者的 chop
		plays := ev.understood && it.planOK && !ev.danger
		ev = s.evalChopMove(it, ev, h, dist)
		ev.understood = ev.understood && plays
		if plays {
			ev.gets = 1 + len(it.plan)
			ev.score += scoreTempoCMPlay
		}
		return ev, true
	}
	// Level 6 The Valuable Tempo Clue：有價值的 Tempo Clue 和一般 play clue 一樣（可以隨時給）
	valuable := s.knows(lvTempo) && it.valuable && ev.understood && it.planOK && !s.alreadyResolved(R, it.focus, t)
	if it.tempo && !valuable && !(ev.understood && it.planOK && len(it.plan) > 0 && !s.alreadyResolved(R, it.focus, t)) {
		ev.kind = kindTempo
		ev.knowsPlay = ev.understood && s.playableNow(t)
		for _, c := range it.cands {
			ev.knowsPlay = ev.knowsPlay && s.playableNow(c.id)
		}
		ev.score = -dist
		return ev, true
	}
	if !ev.understood {
		return ev, true
	}
	if s.stomps(touched) {
		// Level 5：Layered / Hidden Finesse 中，真正要盲打的牌被直接提示後打出，會被當成那個 finesse 兌現了
		// （例如 Clandestine Finesse 的接收者以為是另一種解讀），拖延時也不能用
		ev.misleads = ev.misleads || s.knows(lvLayeredFinesse)
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
	if bad := s.unsafeFinesse(R, plan); bad || s.layerMisread(it, plan) {
		return ev, true
	}
	// 被 prompt / finesse 叫出來的連接牌，若可能和我手上不確定的已提示牌重複，同樣違反 Good Touch
	// （Level 10 Sarcastic Finesse：「Normally, potentially duplicating a card is bad」；那種刻意的用法是 Level 10）
	for _, d := range plan {
		if s.holder(d.card) != s.me && (s.maybeMine(d.id) || s.awaited(d.id, d.card) || s.pendingLayer(d.id, d.card)) {
			ev.bad++
		}
	}
	soon := playable && s.resolvesToPlay(it.cands, plan, t)

	switch {
	case ev.danger:
		return ev, true
	case soon && ev.bad == 0:
		ev.kind = kindPlay
		ev.gets = 1 + len(plan) + it.tempoExtra
		ev.plan = plan
		ev.plays = s.chainPlays(t, plan, it.newCards, it.focus)
		ev.immediate = len(plan) == 0 && s.playableNow(t)
		next := (s.me + 1) % s.n
		ev.nextPlays = R == next && ev.immediate
		for _, d := range plan {
			ev.nextPlays = ev.nextPlays || d.player == next && s.playableNow(d.id)
		}
		ev.score = scorePerPlay*ev.gets + scoreNewGood*ev.newGood + scoreTrashMarked*ev.trashMarked
		// 接收者自己就知道能接著打的非焦點新牌：比只是「有用」的新牌值錢（例如「1」一次讓兩張 1 能打，勝過藍色點到 b1 與 b4）
		ev.score += scoreExtraPlay * s.knownExtraPlays(it, t, plan)
		if it.chopFocus && s.needsSave(R, it.focus) {
			ev.score += scoreChopSaved
		}
		if len(plan) == 0 && s.playableNow(t) {
			ev.score += scoreImmediate
		}
		if s.knows(lvTempo) {
			// Level 6 Clarity Principle：結果相同時選比較不複雜的（finesse 可能沒被接到）
			for _, d := range plan {
				if d.finesse {
					ev.score -= scoreFinesseClarity
				}
			}
		}
	case it.chopFocus && s.needsSave(R, it.focus) && !s.saveIsOthersJob(R, t):
		// 接收者知道要保留（就算還不確定是 play 還是 save）就算救下來了
		ev.kind = kindSave
		ev.score = scoreSave + scoreNewGood*ev.newGood - scoreBadTouch*ev.bad
		if ev.misleads {
			ev.score -= scoreSaveMisleads
		}
	case it.saveOK && !it.chopFocus && ev.bad == 0 && !ev.misleads:
		// 5 Stall：點到的都是將來要用的 5，不會被誤會成 play clue
		ev.kind = kindStall
		ev.score = scoreStall * (1 + ev.newGood)
	default:
		return ev, true
	}
	if h.Kind == game.HintColor {
		ev.score += scoreColor
	}
	// 給下一位的提示：提示後他的 chop 往左移到必須保留的牌上，而他又沒有能立刻打的牌，
	// 他下一手就會棄掉那張牌（Check Team Chops 要看提示之後的 chop）
	if R == (s.me+1)%s.n && !s.keepsChop(R) && !s.knowsPlayNow(it) {
		if nc := s.chopAfter(R, touched); nc >= 0 && s.needsSave(R, nc) {
			ev.score -= scoreNextChopNeeds
		}
	}
	ev.score -= dist
	return ev, true
}

// layerMisread（Level 5 Layered Finesse）：layer 打出後，接收者會以為焦點是接在 layer 後面的那張
// （提示資訊上可能是它，例如「2」的焦點是 g2，layer 是 y1，接收者就以為是 y2）。原文的 Bob is Lazy 也是這樣讀；
// 盲打的人看得到焦點會再打下一張，但在那之前接收者的筆記是錯的，中間別人的提示就可能讓他打錯
// （自我對局 seed 492：之後綠色點到他的 g2 與 chop 上的 g3，他把 g3 讀成 g1 打出）。Clarity Principle：不給。
func (s *state) layerMisread(it interp, plan []dep) bool {
	maybe := it.explicit[it.focus] & s.useful()
	for _, d := range plan {
		if !d.finesse || d.layers == 0 {
			continue
		}
		cards := s.depCards(d)
		for _, l := range cards[:len(cards)-1] {
			if c := identOfCard(s.cards[l]); c.rank() < game.MaxRank && maybe.has(identOf(c.color(), c.rank()+1)) {
				return true
			}
		}
	}
	return false
}

// focusUnclear：點到的牌中有被 finesse、還沒被提示的牌，而有人不知道它被 finesse（他是那個 finesse 提示的接收者，
// 看不到焦點，還有直接的解讀）：他會把它當成新牌，判斷出的焦點和別人不同，該盲打的連接牌他就不會打
// （自我對局 seed 393：「3」點到被 finesse 的 r3 與新的 y3，大家讀成 y3、要 Reverse Finesse Alice 的 y2，
// Alice 不知道 r3 被 finesse，以為焦點是能打的 r3 而不盲打，後面的人又依 Ambiguous Finesse 以為是自己而打錯）。
func (s *state) focusUnclear(touched []int) bool {
	for _, id := range touched {
		if n := s.notes[id]; n.finessed && !n.clued {
			for q := range s.n {
				if !s.knowsFinessed(q, id) {
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

// 提示的評分。同一步驟裡選分數最高的提示；不同種類（play / save / fix…）的先後由 decide 的步驟順序決定，
// 不靠分數互相比較。dist（接收者離我幾個座位）最後再扣，同分時先給近的人。
const (
	scorePerPlay        = 100 // play clue：每張會被打出的牌（焦點 + prompt / finesse）
	scoreExtraPlay      = 50  // 接收者自己就知道能接著打的非焦點新牌（另外已算在 newGood 裡）
	scoreNewGood        = 10  // 其他有用的非焦點新牌（各種提示共用）
	scoreTrashMarked    = 3   // 接收者推得出是垃圾的非焦點新牌：多知道一張可以先棄的牌
	scoreChopSaved      = 30  // play clue 順便救下 chop 上必須保留的牌
	scoreImmediate      = 3   // play clue 的焦點現在就能打（不用等連接牌）
	scoreSave           = 50  // save clue 的底分
	scoreSaveMisleads   = 10  // save clue 會讓接收者對某張牌寫下錯誤筆記
	scoreBadTouch       = 15  // 每張違反 Good Touch 的新牌
	scoreStall          = 10  // 5 Stall：（1 + 有用的新牌）× 這個分數
	scoreColor          = 2   // 能選的話優先用顏色提示
	scoreNextChopNeeds  = 100 // 給下一位的提示讓他的新 chop 落在必須保留的牌上、他又不能立刻打牌
	scoreFixNow         = 100 // Fix Clue：每張原本會被打錯的牌
	scoreFix            = 30  // Fix Clue：每張修正的牌
	scoreCMFirstSave    = 45  // Chop Move：保護到 chop 上必須保留的牌（第一張）
	scoreCMMoreSave     = 40  // Chop Move：每多保護一張必須保留的牌
	scoreCMUseful       = 10  // Chop Move：其他被保護的有用牌
	scoreCMWaste        = 20  // Chop Move：每張白白被保護的沒用牌
	scoreTempoCMPlay    = 30  // Level 6 Tempo Clue Chop Move：除了 Chop Move，焦點也會打出
	scoreFinesseClarity = 1   // Level 6 Clarity Principle：每個要盲打的 finesse（同分時選比較單純的提示）
)

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
		if i < 0 || !s.knows(lvMultiConnect) && len(c.deps) != 1 || card >= 0 && c.deps[i].card != card {
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
				if !s.knows(lvReverseFinesse) {
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

// unsafeFinesse：計畫裡的 finesse 雖然照慣例解讀得通，但這樣給容易讓人搞錯，不給。
//   - 我收到的提示還在等某人盲打時，不給要他盲打的提示：他的 finesse position 有沒有被佔走還不確定，
//     我以為的位置可能和他自己認定的不同（等待時只做無關的動作，所有等級）。
//   - Level 5 Queued Finesse（排在同一人另一個還沒打的 finesse 後面）只解讀、不主動給：前一個若是 Layered Finesse，
//     給提示者知道哪幾張是 layer、別人不知道，「下一張 finesse position」各說各話（自我對局 5 人局 -0.08，打錯變多）。
//   - Layered Finesse 只在 layer 後面那張一輪到就能打時給（他得先等別的牌時，layer 可能先被別人打掉或提示），
//     而且不用在接收者自己身上（他看不到自己的 layer，盲打出它就會以為焦點是接上它的那張，Bob is Lazy）。
func (s *state) unsafeFinesse(R int, plan []dep) bool {
	return slices.ContainsFunc(plan, func(d dep) bool {
		if !d.finesse {
			return false
		}
		if s.awaitedFinesse(d.player) {
			return true
		}
		if !s.knows(lvLayeredFinesse) {
			return false
		}
		return s.queuedBehind(d) || d.layers > 0 && (!s.playableNow(d.id) || d.player == R)
	})
}
