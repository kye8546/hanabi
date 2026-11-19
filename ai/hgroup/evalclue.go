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
	exposesNext bool  // 給下一位的提示讓他的新 chop 落在必須保留的牌上、他又不能立刻打牌（scoreNextChopNeeds）
	exposesLate bool  // 給更後面玩家的提示讓他的新 chop 落在必須保留的牌上、他又不能立刻打牌（exposesLaterChop）
	targetPlays bool  // play clue：接收者輪到時就會打牌（自己就知道能打，或連接牌都在他之前打出），那一手不會棄 chop
	touched     []int
	newGood     int   // 非焦點、有用的新牌
	plays       int   // 照計畫進行後會依序打出的牌數（焦點、連接牌、接得上的非焦點新牌）
	trashMarked int   // 非焦點、接收者推得出是垃圾的新牌（不算違反 Good Touch）
	bad         int   // 違反 Good Touch 的新牌（垃圾或重複）
	fixes       int   // Fix Clue：修正了幾張壞掉的牌
	fixesNow    int   // 其中有幾張原本會被打錯
	bluff       bool  // Level 11 Bluff：下一位盲打出別張能打的牌，焦點是差一張就能打的牌
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
	if s.ddaMisread(R, h, touched, it) {
		ev.misleads = true
		return ev, true
	}
	if it.fix {
		return s.evalFix(it, ev, h, dist), true
	}
	if it.cmKind == cmTrash || it.cmKind == cmFive {
		return s.evalChopMove(it, ev, h, dist), true
	}
	t := it.truth

	var seen idSet
	var side []int
	// 計畫要打出的連接牌（prompt / finesse）也算已被拿到：點到和它們一樣的牌違反 Good Touch
	// （使用者回報 20261021-174911 t2：紅色點到 AI-2 的 r2、r1，要 finesse 使用者的 r1，AI-2 的 r1 就重複了；以前算成有用的新牌）
	var planned idSet
	for _, d := range it.plan {
		planned |= setOf(d.id)
	}
	// 焦點先算：和焦點一樣的非焦點新牌是重複的那張（例如其他 1 都打出了，「1」點到兩張 w1）
	order := it.newCards
	if i := slices.Index(order, it.focus); i > 0 {
		order = append([]int{it.focus}, slices.Delete(slices.Clone(order), i, i+1)...)
	}
	// Level 8 Distribution Clue：值得給時，焦點和別處已被拿到的牌重複是刻意的，不算違反 Good Touch
	distWorth := s.distributionWorth(R, it, t)
	// Level 10 The Sarcastic Finesse：finesse 別人時，焦點可能和我手上不確定的已提示牌重複也可以（dupFinesseOK）
	// 接收者要把實際牌面讀成 play clue 才是 finesse：只可能是 save 時（例如鎖手時點 chop 的 Locked Hand Save）沒有人會盲打
	// （使用者回報 20261110-122845 t22：AI-1 鎖手、綠色那張是「G2 或 G3」，正等 AI-3 盲打 G2（Reverse Finesse），卻用「3」
	// 點 AI-3 chop 上的 G3，AI-3 照 Good Touch 推定 AI-1 那張是 G2，不盲打了）
	sarcastic := slices.ContainsFunc(it.plan, func(d dep) bool { return d.finesse && d.player != s.me }) &&
		slices.ContainsFunc(it.cands, func(c cand) bool { return c.id == t && !c.save })
	for _, id := range order {
		c := identOfCard(s.cards[id])
		switch {
		case id == it.focus && distWorth:
			seen |= 1 << c
			continue
		case slices.Contains(it.staleBad, id) && !s.trash(c) && !seen.has(c) && !s.gottenAs(s.me, s.me, c, id):
			// Level 12 Stale 1's Clue：接收者會把照順序先打的那張 1 當成壞的棄掉，它實際卻是有用的牌
			ev.bad++
			ev.misleads = true
		case id != it.focus && (s.trash(c) || seen.has(c)) && s.seenAsTrash(it, id):
			// 接收者推得出它不可能有用（垃圾，或只可能是焦點的重複）：不會造成誤解，還多知道一張可以先棄的牌
			ev.trashMarked++
		case s.trash(c) || seen.has(c) || id != it.focus && planned.has(c) || s.gottenAs(s.me, s.me, c, id) || s.awaited(c, id) || s.pendingLayer(c, id) ||
			s.maybeMine(c) && !(id == it.focus && sarcastic && s.dupFinesseOK(c)):
			ev.bad++
			if id != it.focus && (s.trash(c) || seen.has(c)) {
				side = append(side, id) // 一起點到的垃圾（或焦點的重複）：終局可以容忍（goodTouchWaived）
			}
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
	waived := s.goodTouchWaived(R, it, touched, side, ev.bad, &ev)
	if it.stray {
		ev.misleads, ev.danger = true, true // 計畫外的玩家會盲打（或打已提示的牌），多半打錯
	}
	if s.focusUnclear(touched) {
		ev.misleads = true
		return ev, true
	}
	if s.knows(lvFixClue) && len(it.newCards) == 0 && !s.retouch &&
		!slices.ContainsFunc(touched, func(id int) bool { return it.explicit[id] != s.notes[id].explicit }) &&
		!s.finessingTempo(it, touched, ev) {
		// 不給任何新資訊的提示：接收者若已經（私下推得）知道其中某張是他下一手要打的牌，就會當成 Stop Clue
		// 而棄掉它（Level 3 Fix Clue）。我看不到他的私下推論，不冒這個險。
		// 例外（discard 的 retouch）：我鎖手、又只能棄很可能是 critical 的已提示牌時，鎖手的人重新點已提示的牌
		// 是拖延，不是 Stop Clue（findFix）；我在 Double Discard Situation 時 Stall Table 也允許 Hard Burn。照一般的解讀評估
		// （點到接收者下一手要打的牌時他照樣會讀成 Stop Clue，模擬得出來）
		ev.misleads = true
		return ev, true
	}
	if bev, ok := s.evalBluff(R, h, it, touched, ev, dist); ok {
		return bev, true // Level 11：Bluff（實際焦點接不上，靠下一位盲打出別張能打的牌）
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
			// 他本來就會打的牌（已知能打、是之前的提示要打的牌，例如只剩 self-prompt 的解讀）不算這個提示讓他打的
			anyway := s.alreadyResolved(R, it.focus, t) || s.holderKnowsPlayable(it.focus) || s.notes[it.focus].willPlay || s.plannedPlay(it.focus)
			// 他本來就確定這張是 t（還不能打，等前一張打出就會打）：這個提示也沒讓他多打牌（使用者回報 20261109-122807 t38：
			// AI-2 的 W3 早就確定、W2 也確定能打，白色只剩 Chop Move，直接用「5」救 chop 上的 R5 有正向資訊、比較好）
			if u, ok := s.possOf(it.focus).single(); ok && u == t {
				anyway = true
			}
			if !anyway {
				// 焦點本來就會打出時不加分：這個提示只剩 Chop Move，直接救 chop 有正向資訊、比較好（使用者回報 20261105-113110 t5：
				// 「4」已要 prompt AI-2 的 R2、R3，AI-1 用「3」點 R3 做 TCCM 保護 chop 上的 Y2，被算成勝過直接的 2 Save）
				ev.score += scoreTempoCMPlay
			}
			ev.nextPlays = R == (s.me+1)%s.n && s.knowsPlayNow(it) && !anyway
		}
		return ev, true
	}
	// Level 6 The Valuable Tempo Clue：有價值的 Tempo Clue 和一般 play clue 一樣（可以隨時給）
	valuable := s.knows(lvTempo) && it.valuable && ev.understood && it.planOK && !s.alreadyResolved(R, it.focus, t)
	if it.tempo && !valuable && !(ev.understood && it.planOK && len(it.plan) > 0 && !s.alreadyResolved(R, it.focus, t)) {
		knowsPlay := ev.understood && s.playableNow(t)
		for _, c := range it.cands {
			knowsPlay = knowsPlay && s.playableNow(c.id)
		}
		// 接收者鎖手（沒有 chop 可 Chop Move，不會讀成 Tempo Clue Chop Move）、這個提示讓他確定焦點現在就能打、
		// 他本來不知道：就是一般的 play clue，照打出一張算（使用者回報 20261113-165056：AI-3 鎖手，被「1」點到的 K1
		// 記成「R1 或 K1」，用黑色補資訊就能打出，以前只在拖延時才給這種提示）
		// Level 8 終局不 Chop Move（tempoChopMove）：Tempo Clue 只是讓焦點打出，同樣照一般的 play clue 算（使用者回報
		// 20261118-141043 t50：只差 R4、R5、G5，你被「5」點到的兩張分不出哪張是 G5，AI-1 把綠色當成沒價值的拖延，改給沒用的「5」）
		anyway := s.alreadyResolved(R, it.focus, t) || s.holderKnowsPlayable(it.focus) || s.notes[it.focus].willPlay || s.plannedPlay(it.focus)
		// 終局這種提示會被接受，就要確定他自己還推不出（從他看得到的牌，holderView；例如 Black 的 W2 / K2，他看得到 K2 在我手上）
		heKnows := func() bool {
			v := s.holderView(it.focus, s.certainMine())
			return v != 0 && !slices.ContainsFunc(v.list(), func(i ident) bool { return !s.playableNow(i) })
		}
		if !(knowsPlay && !anyway && (s.publicChop(R, nil) < 0 || s.endGame() && !heKnows())) {
			ev.kind = kindTempo
			ev.knowsPlay = knowsPlay
			ev.score = -dist
			return ev, true
		}
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
		if !s.finesseClear(R, it.focus, t, plan) || !it.agreed || !s.selfPromptTimely(R, it.cands) {
			// 接收者看得懂，但要打連接牌的人會搞錯是誰、打哪張（例如 prompt 到另一張已提示牌），拖延時也不能用
			ev.misleads = true
			return ev, true
		}
	}
	if playable && s.knows(lvBluff) {
		if targets, _, fp, ok := s.bluffRead(R, h, it, touched, plan); ok && (targets.has(t) || !slices.ContainsFunc(it.cands, func(c cand) bool {
			return !targets.has(c.id) && slices.ContainsFunc(c.deps, func(d dep) bool { return d.card == fp && d.finesse })
		})) {
			// Level 11：下一位照計畫盲打出的牌接不上提示，大家會讀成 Bluff（例如 Bluff Seat 上的 Layered Finesse、
			// 用數字提示的 Double Finesse），不是計畫的意思。焦點不是合法的 Bluff 目標、接收者也看得出可能是 Layered Finesse
			// 時不會（看得到焦點的人照 Layered Finesse 讀，接收者等盲打的人下一手揭曉，detectBluff）
			ev.misleads = true
			return ev, true
		}
	}
	if bad := s.unsafeFinesse(R, plan); bad || s.layerMisread(it, plan) {
		return ev, true
	}
	// 被 prompt / finesse 叫出來的連接牌，若可能和我手上不確定的已提示牌重複，同樣違反 Good Touch
	// （Level 10 Sarcastic Finesse：「Normally, potentially duplicating a card is bad」）。Level 10 The Certain Finesse 例外：
	// 正好在 finesse position 上（不能有 layer：「performing a Layered Finesse on a card that could potentially be clued in your own hand
	// is illegal」）、不是接收者自己的牌——他看到我那張就是它時會 Certain Discard 傳回給我
	for _, d := range plan {
		certain := d.finesse && d.layers == 0 && d.player != R && s.dupFinesseOK(d.id)
		if s.holder(d.card) != s.me && (s.maybeMine(d.id) && !certain || s.awaited(d.id, d.card) || s.pendingLayer(d.id, d.card)) {
			ev.bad++
		}
	}
	soon := playable && s.resolvesToPlay(R, it.cands, plan, t)
	if soon && s.unlocksMisplay(t, plan) {
		ev.misleads, ev.danger = true, true
		return ev, true
	}

	switch {
	case ev.danger:
		return ev, true
	case soon && (ev.bad == 0 || waived) && s.timelyPlays(R, t, plan, touched)+it.tempoExtra == 0:
		return ev, true // 遊戲結束前一張都打不到：不是 play clue，只能當拖延
	case soon && (ev.bad == 0 || waived):
		ev.kind = kindPlay
		ev.gets = s.timelyPlays(R, t, plan, touched) + it.tempoExtra
		ev.plan = plan
		ev.plays = s.chainPlays(t, plan, it.newCards, it.focus)
		ev.immediate = len(plan) == 0 && s.playableNow(t)
		next := (s.me + 1) % s.n
		// 他自己的每種解讀都直接能打才會下一手就打；還有要等別人（例如 Reverse Finesse 後面的人）的解讀時，他會先等
		// （使用者回報 20261022-175528 t11：「3」點到 AI-2 的 g3，也可能是要使用者 Reverse Finesse m2 的 m3，AI-2 先等、棄掉 chop 上的 g4）
		ev.nextPlays = R == next && ev.immediate && s.knowsPlayNow(it)
		for _, d := range plan {
			ev.nextPlays = ev.nextPlays || d.player == next && s.playableNow(d.id)
		}
		ev.score = scorePerPlay*ev.gets + scoreNewGood*ev.newGood + scoreTrashMarked*ev.trashMarked - scoreBadTouch*ev.bad
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
	ev.targetPlays = ev.kind == kindPlay && (s.knowsPlayNow(it) || s.playsByTurn(it, R))
	// 給下一位的提示：提示後他的 chop 往左移到必須保留的牌上，而他又沒有能立刻打的牌，
	// 他下一手就會棄掉那張牌（Check Team Chops 要看提示之後的 chop）
	if !s.keepsChop(R) && !s.knowsPlayNow(it) {
		if nc := s.chopAfter(R, touched); nc >= 0 && s.needsSave(R, nc) {
			if R == (s.me+1)%s.n {
				ev.score -= scoreNextChopNeeds
				ev.exposesNext = true
			} else if nc != s.chop(R) && !s.playsByTurn(it, R) {
				// chop 本來就在那張牌上時由 doomsLaterChop 處理（改給救它的提示）
				ev.exposesLate = true
			}
		}
	}
	ev.score -= dist
	return ev, true
}

// finessingTempo（Level 6）：不給新資訊的 Tempo Clue 也可以給——它會 finesse / prompt 出別人的牌（The Valuable Tempo Clue、
// Minimum Clue Value Principle），而接收者推不出被點到的牌現在就能打（他不會以為那是他下一手要打的牌而讀成 Stop Clue；
// 照他看得到的牌推，holderView）。使用者回報 20261110-173602 t12：AI-2 被白色點過的 W4，重新點白色就 finesse AI-1 的 W3，
// 一個提示打出兩張，以前因為不給新資訊而不給。
func (s *state) finessingTempo(it interp, touched []int, ev clueEval) bool {
	if !s.knows(lvTempo) || !ev.understood || !it.planOK || it.saveOK || len(it.plan) == 0 ||
		!slices.ContainsFunc(it.plan, func(d dep) bool { return d.player != it.receiver }) {
		return false
	}
	// 只限一般的 finesse：要盲打的人都在我和接收者之間（他們看得到焦點，下一手就盲打）。放行 Reverse Finesse 或 prompt 時
	// 3000 局 Lv11 4–5 人局 -0.04–0.07、Black 3 人局 -0.07
	k := (it.receiver - s.me + s.n) % s.n
	if slices.ContainsFunc(it.plan, func(d dep) bool {
		j := (d.player - s.me + s.n) % s.n
		return !d.finesse || j == 0 || j >= k
	}) {
		return false
	}
	mine := s.certainMine()
	for _, id := range touched {
		v := s.holderView(id, mine) & s.useful()
		if v != 0 && !slices.ContainsFunc(v.list(), func(i ident) bool { return !s.playableNow(i) }) {
			return false // 他可能以為這張就是他下一手要打的牌：讀成 Stop Clue
		}
	}
	return true
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

// goodTouchWaived：Level 8 的終局（pace < 人數，或牌堆已空）中，play clue 一起點到的非焦點新牌只是垃圾（或焦點的重複）時，
// 照 play clue 給，每張扣 scoreBadTouch（策略）。只在 Lv8 起：Lv1 3000 局閃電明顯變多（5 色 5 人局 0.10 → 0.15，
// Rainbow 3 / 5 人局 -0.06）——沒有 Fix Clue，被記錯的垃圾之後常被打錯。原文 Save Principle「Violating Good Touch Principle」：
// 為了不失去滿分的機會，違反 Good Touch 是兩害相權取其輕；終局能打的牌比塞住手牌的垃圾重要。
// 使用者回報 20261106-181639 t53：使用者 slot 1 的 R4 是最後一張，能點到它的只有紅色（一起點到已打出的 R2、R1），
// AI-1 不給而棄牌，牌堆抽完後 R4、R5 都打不出來；給紅色可以拿到 25 分。
// 那幾張被記錯（例如記成 R5）不算誤導；但焦點與連接牌打出後它們可能被當成能打（接收者打錯）時，
// 只在閃電少於 2 個時容忍。成立時把 ev 的誤導判斷改成不看那幾張。
func (s *state) goodTouchWaived(R int, it interp, touched, side []int, bad int, ev *clueEval) bool {
	if len(side) == 0 || len(side) != bad || !s.endGame() {
		return false
	}
	rest := slices.DeleteFunc(slices.Clone(touched), func(id int) bool { return slices.Contains(side, id) })
	m, d, c := s.misleads(R, it, rest)
	if d || s.strikes >= 2 && s.sideMayPlay(R, it, side) {
		return false
	}
	ev.misleads, ev.danger, ev.costly = m, d, c
	return true
}

// sideMayPlay：一起點到的垃圾 side，接收者照 Good Touch 記下的可能，在焦點與連接牌打出後全都能打（他可能打錯）。
func (s *state) sideMayPlay(R int, it interp, side []int) bool {
	var ps []idSet
	for _, id := range side {
		p := s.notes[id].poss & it.explicit[id]
		if p == 0 {
			p = it.explicit[id]
		}
		ps = append(ps, nonFocusPoss(p, s.freshUseful(R, s.me, id), s.focusSingle(it)))
	}
	saved := s.stacks
	defer func() { s.stacks = saved }()
	if it.hasTruth {
		for _, u := range append([]ident{it.truth}, depIDs(it.plan)...) {
			s.stacks[u.color()] = max(s.stacks[u.color()], u.rank())
		}
	}
	for _, p := range ps {
		all := p != 0
		for _, i := range p.list() {
			all = all && s.playableNow(i)
		}
		if all {
			return true
		}
	}
	return false
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
		if p == 0 {
			// 筆記和提示矛盾：接收者會改成只看提示本身的資訊（onClue），不是寫下錯誤的筆記（使用者回報 20261021-094401 t29：
			// AI-2 的紅牌被記成 r1，「4」讓它變成 r4 就被當成誤導，AI-1 改用 Trash Chop Move 救本來能直接用「4」救的 g4）
			p = it.explicit[id]
		}
		switch {
		case id == it.focus && p&u != 0:
			p &= u
		case id == it.focus:
			p = s.touchedUseful(R, s.me, id, p)
		case slices.Contains(it.newCards, id) && id != it.focus:
			p = nonFocusPoss(p, s.freshUseful(R, s.me, id)&^s.clueSettled(it, id), s.focusSingle(it))
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
// 例外是能照座位順序解開的 Ambiguous Finesse（ambiguousOK）。
func (s *state) finesseClear(R, focus int, t ident, plan []dep) bool {
	for _, d := range plan {
		if !d.finesse {
			continue
		}
		var others []int
		for k := 1; k < s.n; k++ {
			p := (s.me + k) % s.n
			if p == R {
				if !s.knows(lvReverseFinesse) {
					break
				}
				continue
			}
			if fp := s.finessePos(p); p != d.player && fp >= 0 && identOfCard(s.cards[fp]) == d.id {
				others = append(others, p)
			}
		}
		if len(others) > 0 && !s.ambiguousOK(s.me, R, focus, t, plan, d, others) {
			return false
		}
	}
	return true
}

// ambiguousOK：Ambiguous Finesse——計畫要 d.player 盲打的牌 m，正好也在之後另一位 q 的 finesse position 上（都沒有 layer）。
// d.player 看得到 q 的 m，會讓給他（Level 2：看得到的 finesse position 優先）；q 看到他沒打，就知道是自己
// （passMeansMe：d.player 在接收者之前、q 在接收者之後是 Level 2 的 Reverse Finesse 就解得開，兩人在接收者同一側要 Level 5）。
// 實際盲打的是 q，牌面一樣，打出的牌和計畫相同，只是晚一些。使用者的判斷：這是正常的操作，不能禁止
// （使用者回報 20261113-143339 t14：AI-2 和使用者的 finesse position 都是 B1，藍色點 AI-3 chop 上的 B2 一次打出兩張、
// 也救下 B2，AI-1 以前因此不給，改給只打一張的藍色）。
func (s *state) ambiguousOK(giver, R, focus int, t ident, plan []dep, d dep, others []int) bool {
	if len(others) != 1 || d.layers != 0 {
		return false
	}
	q := others[0]
	// q 的 finesse position 本身就是 m（不是藏在 layer 後面；自我對局 Rainbow 5 人局 seed 50：d.player 以為 q 的 M5 後面
	// 藏著 W2、讓給他，q 盲打出 layer 後一路打錯）
	if fp := s.finessePos(q); fp < 0 || identOfCard(s.cards[fp]) != d.id {
		return false
	}
	// 接收者自己的 finesse position 也是 m 時是三方都可能（自我對局 Rainbow 5 人局 seed 1633：P3、P4 和接收者都有 Y1，
	// 一直沒人盲打），不是原文的情況
	if fp := s.finessePos(R); fp >= 0 && identOfCard(s.cards[fp]) == d.id {
		return false
	}
	pos := func(p int) int { return (p - s.me + s.n) % s.n }
	if pos(q) < pos(d.player) {
		return false // 比他早行動的人看得到他的 m，會讓給他，不是計畫的人；不處理
	}
	// 兩人中間只能隔著接收者：中間的旁觀者看不到自己的 finesse position，d.player 不打時他分不出 d.player 是看到他的
	// 還是 q 的 m（自我對局 Rainbow 5 人局 seed 225：P0、P2 都有 Y3，中間的 P1 以為可能是自己，又用「3」點到 P0 的 Y3，
	// P2 盲打出 Y3 後 P0 那張成了重複牌而被打錯）。原文的例子兩人相鄰
	for p := (d.player + 1) % s.n; p != q; p = (p + 1) % s.n {
		if p != R {
			return false
		}
	}
	// 計畫裡只有這一張要盲打，q 也不在計畫裡（否則誰讓給誰會牽動其他連接）
	for _, e := range plan {
		if e.player == q || e.finesse && e != d {
			return false
		}
	}
	if sameSide := (pos(d.player) < pos(R)) == (pos(q) < pos(R)); sameSide && !s.knows(lvUrgency) {
		return false
	}
	// q 從自己的角度也要認為是 finesse d.player：否則 d.player 不打時，他不會想到是自己（自我對局 5 人局 seed 1464：
	// q 手上被「1」點到的牌可能是 W1，他以為 W2 接在那張上，W1 一直沒人盲打，接收者打錯 W2）
	theirs, ok := s.connect(q, giver, R, focus, t)
	return ok && sameDeps(theirs, plan)
}

// ambiguousSwap：theirs 和 plan 只差在一張 finesse 換成另一位 q 的 finesse position 上同一張牌，而且照座位順序
// 解得開（ambiguousOK）——要盲打的人看得到 q 的那張，會讓給他，打出的牌和計畫一樣。
func (s *state) ambiguousSwap(giver, R, focus int, t ident, plan, theirs []dep) bool {
	if len(plan) != len(theirs) {
		return false
	}
	swapped := false
	for i, d := range plan {
		e := theirs[i]
		layered := e.layers != 0
		e.layers, d.layers = 0, 0
		if e == d {
			continue
		}
		if layered {
			return false
		}
		// 換成接收者自己（他看到接收者的 finesse position 也是那張，以為是 Self-Finesse）不算：那是另一種解法，原文的例子沒有
		if swapped || !d.finesse || !e.finesse || e.id != d.id || e.player == d.player || e.player == R || e.card != s.finessePos(e.player) ||
			!s.ambiguousOK(giver, R, focus, t, plan, plan[i], []int{e.player}) {
			return false
		}
		swapped = true
	}
	return swapped
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
func (s *state) resolvesToPlay(R int, cands []cand, plan []dep, t ident) bool {
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
		// Level 2：不在計畫中的 prompt（要別人、不是接收者自己打出已提示的牌）也不會發生：他不打而棄牌時，接收者就排除（promptPassed）
		if !ok && s.knows(lvSelfFinesse) && slices.ContainsFunc(c.deps, func(d dep) bool { return !inPlan(d) && !d.finesse && d.player != R }) {
			continue
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
		// 要我盲打的解讀，焦點實際不是它時（我看得到焦點）也不成立：接收者看得到我的牌，我也不會在輪到時盲打
		// （使用者回報 20261027-145109 t35：黃色的 Y4 也可能是「Y5，之後 finesse AI-1 的 Y4」，Y4 還不能打，
		// failsOnMyTurn 排除不了，AI-1 就以為要再點一次「4」才確定）
		mine := func(c cand) bool {
			return c.id != t && slices.ContainsFunc(c.deps, func(d dep) bool { return d.player == s.me && d.finesse })
		}
		cs := slices.DeleteFunc(slices.Clone(rec.cands), func(c cand) bool { return s.failsOnMyTurn(rec, c) || mine(c) })
		if len(cs) == 0 {
			cs = rec.fallback
		}
		return len(cs) > 0 && !slices.ContainsFunc(cs, func(c cand) bool { return c.id != t || c.save })
	}
	return false
}

// plannedPlay：某個提示要持有者打出這張牌當連接牌，他本來就會打——我推得的計畫要它，或接收者角度的解讀
// （扣掉要我盲打、但我這手不會打的；都扣光時看 fallback，例如只剩 self-prompt 的解讀）每一種都要它。
func (s *state) plannedPlay(id int) bool {
	uses := func(c cand) bool { return slices.ContainsFunc(c.deps, func(d dep) bool { return d.card == id }) }
	for _, rec := range s.recs {
		if slices.ContainsFunc(rec.plan, func(d dep) bool { return d.card == id }) {
			return true
		}
		cs := slices.DeleteFunc(slices.Clone(rec.cands), func(c cand) bool { return s.failsOnMyTurn(rec, c) })
		if len(cs) == 0 {
			cs = rec.fallback
		}
		if len(cs) > 0 && !slices.ContainsFunc(cs, func(c cand) bool { return !uses(c) }) {
			return true
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

// playsByTurn：接收者 R（在我之後第二位以後）的每種解讀，連接牌都由我和他之間的玩家（或他自己盲打）打出，
// 輪到他時他就會打牌、不會棄 chop。
func (s *state) playsByTurn(it interp, R int) bool {
	k := (R - s.me + s.n) % s.n
	for _, c := range it.cands {
		if c.save {
			return false
		}
		var have idSet
		for _, d := range c.deps {
			if j := (d.player - s.me + s.n) % s.n; j == 0 || j > k {
				return false
			}
			have |= setOf(d.id)
		}
		// 中間缺的牌都要由這些連接牌補上（delayed play clue 等的已提示牌在別人手上，他不一定先打）
		for r := s.stacks[c.id.color()] + 1; r < c.id.rank(); r++ {
			if !have.has(identOf(c.id.color(), game.Rank(r))) {
				return false
			}
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

// ddaMisread（Level 9）：我看得到上一位棄掉的那張的最後一張在第三人 H 手上，所以我不在 Double Discard Situation；
// 但 H 看不到自己的牌，以為我在，會把我的提示照拖延讀（5 Stall、Tempo Clue 不 Chop Move、Fill-In 不讓人盲打）。
// 這個提示要 H 行動（他是接收者，或要他打連接牌），而照他的讀法意思不同時，算誤導
// （自我對局 4 人局 seed 212：P3 的「5」對別人是 finesse，持有最後一張 Y4 的 P1 讀成 5 Stall 而沒盲打）。
func (s *state) ddaMisread(R int, h game.Hint, touched []int, it interp) bool {
	if s.dda == nil || s.dda.target != s.me || s.dda.holder < 0 || s.ddaHolderView {
		return false
	}
	H := s.dda.holder
	uses := func(cs []cand) bool {
		return slices.ContainsFunc(cs, func(c cand) bool {
			return slices.ContainsFunc(c.deps, func(d dep) bool { return d.player == H })
		})
	}
	if R != H && !uses(it.cands) && !slices.ContainsFunc(it.plan, func(d dep) bool { return d.player == H }) {
		return false
	}
	restore := s.saveSim()
	s.ddaHolderView = true
	alt := s.interpret(s.me, R, h, touched)
	restore()
	ids := func(cs []cand) []ident {
		var out []ident
		for _, c := range cs {
			out = append(out, c.id)
		}
		return out
	}
	return alt.fix != it.fix || alt.cmKind != it.cmKind || !slices.Equal(alt.cm, it.cm) ||
		!slices.Equal(ids(alt.cands), ids(it.cands)) || !sameDeps(alt.plan, it.plan)
}
