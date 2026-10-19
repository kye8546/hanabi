package hgroup

import (
	"slices"

	"hanabi/game"
)

// interp 是一次提示（實際發生或模擬）的解讀結果。
type interp struct {
	receiver  int
	focus     int
	tempo     bool // 沒有點到任何新牌（Tempo Clue）
	chopFocus bool
	newCards  []int
	explicit  map[int]idSet // 接收者每張手牌在提示後的 explicit 資訊
	cands     []cand        // 接收者角度的焦點可能解讀
	fallback  []cand        // 需要接收者 self-prompt / self-finesse、因為還有其他解讀而暫不考慮的（Level 2）
	selfRisk  bool          // 我是給提示者：fallback 是否其實可能就是接收者的解讀（見 interpret）

	// Level 3 Fix Clue：這個提示修正了違反 Good Touch 的牌，不是 play / save clue（沒有 cands）
	fix     bool
	fixed   []int         // 因此確定沒用的牌
	fixPoss map[int]idSet // 被修正的牌的新筆記

	// Level 4 Chop Moves：這個提示 Chop Move 了哪些牌（Trash Chop Move 沒有 play / save 解讀；
	// 5's Chop Move 點到的 5 照 5 Save 記成任何 5）
	cmKind int
	cm     []int

	// Level 6 Tempo Clues：valuable = 有價值的 Tempo Clue（當一般 play clue，不 Chop Move）；
	// trashNew = 「Tempo Clue 優先於 Trash Chop Move」時一起點到的新牌（已知垃圾）
	valuable   bool
	trashNew   []int
	tempoExtra int // Tempo Clue 點到的其他舊牌因此確定現在能打的張數（tempoExtraPlays）

	// 我看得到焦點牌時才有
	hasTruth bool
	truth    ident
	saveOK   bool  // 實際牌面符合 save clue 的解讀
	plan     []dep // 實際牌面作為 play clue 需要的連接
	planOK   bool
	agreed   bool // 每個要打連接牌的玩家，從自己角度推出的連接都和 plan 一樣
	stray    bool // 計畫外的玩家會從自己的角度以為要打他的牌（他看不到自己的牌，會打錯）
	// saveView[q]：從看得到焦點的旁觀者 q 的角度，這個提示可能是救它的 save clue
	// （例如 q 看不到自己手上的另一張 2，以為 chop 上的 2 是 2 Save；passMeansMe 用）
	saveView []bool
	free     freeChoice // Rainbow 變體的 Free Choice
}

// interpret 依 H-Group 慣例（目前的等級）解讀 giver 給 R 的提示。touched 是被點到的牌 ID。
// 焦點與可能解讀都以接收者 R 的角度計算；plan 則以我的角度（看得到焦點時）計算。
func (s *state) interpret(giver, R int, h game.Hint, touched []int) interp {
	it := interp{receiver: R, explicit: map[int]idSet{}}
	hs := hintSet(h)
	chop := s.chop(R)
	// fresh：新牌中沒被 Chop Move 的。Level 4 Chop Moves & New Clues：判斷焦點時 Chop Moved 的牌算舊牌
	var fresh []int
	for _, id := range s.hands[R] {
		ex := s.notes[id].explicit
		if slices.Contains(touched, id) {
			ex &= hs
			if !s.gotten(id) {
				it.newCards = append(it.newCards, id)
				if !s.notes[id].cm {
					fresh = append(fresh, id)
				}
			}
		} else {
			ex &^= hs
		}
		it.explicit[id] = ex
	}

	// 焦點：沒有新牌 → 最左邊被重複點到的牌；點到 chop → chop；否則最左邊的新牌
	switch {
	case len(it.newCards) == 0:
		// Level 6 Focus Shifting：最左邊那張已經知道能打時，焦點移到下一張不知道能打的
		it.tempo = true
		it.focus = s.tempoFocus(R, touched)
	case len(fresh) == 0:
		// Level 4：只點到舊牌、而且第一次點到 Chop Moved 的牌時，焦點是最左邊那張 Chop Moved 的牌
		it.focus = it.newCards[0]
	case slices.Contains(fresh, chop):
		it.focus, it.chopFocus = chop, true
	case s.knows(lvMultipleOnes) && h.Kind == game.HintRank && h.Rank == 1 && !slices.ContainsFunc(fresh, s.fresh):
		// Level 3 Playing Multiple 1's：焦點是最先打的那張 1；點到的全是起手牌時是最右邊（最舊）那張
		// （新抽的牌一定在起手牌左邊，有新抽的 1 時就是最左邊的新牌，和一般規則一樣）
		it.focus = fresh[len(fresh)-1]
	default:
		it.focus = fresh[0]
	}

	if s.findFix(giver, R, h, touched, &it); it.fix {
		return it
	}
	// Level 6：同時像 Tempo Clue 和 Trash Chop Move 時當成 Tempo Clue
	if !s.tempoOverTrash(R, touched, fresh, &it) {
		if s.findChopMove(giver, R, h, touched, fresh, &it); it.cmKind == cmTrash {
			return it
		}
	}

	defer s.saveSim()()
	// 我看得到被點到的牌時：這次提示的 Good Touch 推論立刻生效（給提示者看得到我的已提示牌，
	// 就不會點到和它們一樣的牌），算解讀時就用得上
	if R != s.me {
		s.pending, s.pendingGiver = touched, giver
	}

	// 這次被點到的牌不在 finesse position 上（finesseSlot 會跳過）
	s.touchedNow = touched
	it.free = s.freeChoiceOf(R, h, touched)
	s.free = it.free

	// 計算期間把接收者的筆記換成「提示後」的樣子（負向資訊會影響 prompt 候選）
	s.overlay = map[int]idSet{}
	for id, ex := range it.explicit {
		s.overlay[id] = s.notes[id].poss & ex
	}
	if giver == s.me {
		// 我給這個提示就表示這手不盲打：之前的提示裡要我盲打、但我知道不是的解讀，在大家看來也就被排除了
		// （例如 Cathy「b3 或 b4」的牌，b4 那種要我盲打 b3；我改給提示後她就知道是 b3）
		for _, rec := range s.recs {
			if s.holder(rec.focus) < 0 || rec.target == s.me {
				continue
			}
			if _, ok := s.overlay[rec.focus]; ok {
				continue
			}
			var u idSet
			dropped := false
			for _, c := range rec.cands {
				if s.failsOnMyTurn(rec, c) {
					dropped = true
					continue
				}
				u |= setOf(c.id)
			}
			if p := s.notes[rec.focus].poss & u; dropped && p != 0 {
				s.overlay[rec.focus] = p
			}
		}
	}

	base := s.overlay[it.focus]
	maySave := it.chopFocus || it.cmKind == cmFive || s.fiveStall(giver, h) && (len(it.newCards) > 0 || s.mustStall(giver))
	candsOf := func() []cand {
		cs := s.playCands(R, giver, R, it.focus, base)
		if maySave {
			cs = append(cs, s.saveCands(R, giver, R, &it, h, base)...)
		}
		return cs
	}
	it.cands = candsOf()
	// 接收者看得到的牌（別人手牌、棄牌、煙火）已經用完所有張數的牌面，焦點不可能是它
	gone := func(c cand) bool { return s.remainingFor(R, c.id) <= 0 }
	if slices.ContainsFunc(it.cands, func(c cand) bool { return !gone(c) }) {
		it.cands = slices.DeleteFunc(it.cands, gone)
	}
	// Self-prompt 只在沒有其他解讀時才成立（Beginner's Guide: The Prompt
	// 「There is no 4 that could possibly be playable right now. Thus, this must be a Self-Prompt」）。
	// 我是給提示者時看不到自己的牌，接收者卻看得到：也要考慮它們讓接收者有更簡單的解讀
	selfDep := func(c cand) bool { return slices.ContainsFunc(c.deps, func(d dep) bool { return d.player == R }) }
	alt := it.cands
	if giver == s.me && R != s.me {
		restore := s.saveSim()
		s.optimistic = true
		alt = candsOf()
		restore()
	}
	notSelf := func(c cand) bool { return !selfDep(c) }
	if slices.ContainsFunc(it.cands, notSelf) || slices.ContainsFunc(alt, notSelf) {
		// 只有「樂觀地假設接收者看得到我手上的連接牌」時才有其他解讀：他實際上很可能只剩 self 解讀
		it.selfRisk = s.knows(lvSelfFinesse) && !slices.ContainsFunc(it.cands, notSelf)
		// 其他解讀被證實不成立（該打的人沒打）後，才輪到 self-prompt / self-finesse（Lv1 只會有 self-prompt）
		it.fallback = slices.DeleteFunc(slices.Clone(it.cands), notSelf)
		it.cands = slices.DeleteFunc(it.cands, selfDep)
	}
	// 只剩要接收者自己的解讀時，Prompts > Finesses：有只靠 self-prompt（自己已被提示的牌）的解讀，
	// 就不考慮要 self-finesse 的（Level 2 The Self-Finesse：「If the clue looks like it could be a Prompt,
	// then the clue receiver will not play anything and assume that it is a Prompt」）
	selfFin := func(c cand) bool {
		return slices.ContainsFunc(c.deps, func(d dep) bool { return d.player == R && d.finesse })
	}
	if s.knows(lvSelfFinesse) && slices.ContainsFunc(it.cands, selfFin) && slices.ContainsFunc(it.cands, func(c cand) bool { return !selfFin(c) }) {
		it.fallback = append(it.fallback, slices.DeleteFunc(slices.Clone(it.cands), func(c cand) bool { return !selfFin(c) })...)
		it.cands = slices.DeleteFunc(it.cands, selfFin)
	}
	// Level 6：沒有價值、又不是拖延時給的 Tempo Clue 另外 Chop Move 接收者的 chop
	if it.tempo && s.knows(lvTempo) {
		it.tempoExtra = s.tempoExtraPlays(&it, touched)
		s.tempoChopMove(giver, R, &it, touched)
	}

	if t, ok := s.see(s.me, it.focus); ok {
		it.hasTruth, it.truth = true, t
		for _, c := range it.cands {
			if c.save && c.id == t {
				it.saveOK = true
			}
		}
		if maySave {
			it.saveView = make([]bool, s.n)
			for q := range s.n {
				if q != giver && q != R && q != s.me {
					it.saveView[q] = len(s.saveCands(q, giver, R, &it, h, setOf(t))) > 0
				}
			}
		}
		it.plan, it.planOK = s.connect(s.me, giver, R, it.focus, t)
		it.agreed = it.planOK
		for _, d := range it.plan {
			if d.player == s.me {
				continue
			}
			// 他看不到自己的牌：例如他手上有可能是連接牌的已提示牌時，他會以為是 prompt 而不盲打
			theirs, ok := s.connect(d.player, giver, R, it.focus, t)
			if !ok || !sameDeps(theirs, it.plan) {
				it.agreed = false
			}
		}
		// 計畫外的玩家看不到自己的牌：他若從自己的角度推出「要我打牌」（例如手上已提示的牌可能就是連接牌，
		// Prompts > Finesses），就會打錯。提示本身根本沒有可行的計畫時也一樣（例如鎖手的人拖延時重新點已知的 y5，
		// 看不到自己手牌的旁觀者會以為是 Reverse Finesse 自己的 y1–y4 而盲打）；可能是 save clue 時不會有人盲打
		mayBeSave := slices.ContainsFunc(it.cands, func(c cand) bool { return c.save })
		planned := it.agreed
		for q := range s.n {
			if !planned && mayBeSave || q == giver || q == R || q == s.me || slices.ContainsFunc(it.plan, func(d dep) bool { return d.player == q }) {
				continue
			}
			if theirs, ok := s.connect(q, giver, R, it.focus, t); ok && slices.ContainsFunc(theirs, func(d dep) bool { return d.player == q }) {
				it.agreed = false
				it.stray = it.stray || !planned
			}
		}
	}
	return it
}

// sameDeps：兩個計畫要同樣的人打同樣的牌。layer 的數目不算（Level 5 Layered / Hidden Finesse：盲打的人
// 看不到自己的牌，不知道前面有 layer，但他第一張打的牌和看得到的人預期的一樣）。
func sameDeps(a, b []dep) bool {
	return slices.EqualFunc(a, b, func(x, y dep) bool { x.layers, y.layers = 0, 0; return x == y })
}

// fresh：不是起手牌（Level 3 Fresh 1's）。
func (s *state) fresh(id int) bool { return !s.notes[id].start }

// playCands 列出焦點作為 play clue（含 delayed play clue）時，obs 認為可能的牌面。
func (s *state) playCands(obs, giver, R, focus int, base idSet) []cand {
	var out []cand
	for _, t := range base.list() {
		// Good Touch：已打出、打不出去、或別處已被拿到的牌不會是焦點
		if s.trash(t) || s.gottenAs(obs, giver, t, focus) {
			continue
		}
		if deps, ok := s.connect(obs, giver, R, focus, t); ok {
			out = append(out, cand{id: t, deps: deps})
		}
	}
	return out
}

// saveCands 列出焦點在 chop 上時，作為 save clue 可能的牌面：
// 數字 5 → 任何 5（5 Save；不在 chop 上的是 5 Stall）；數字 2 → 任何 2（2 Save，依 Visible Rule 排除別人手上看得到的）；
// 其他 → critical 牌（5 只能用數字 5 救）。黑色的牌另有慣例（blackColorSave、florratSave）。
// 現在就能打的牌面不算 save：那種可能本身就是 play clue（playCands 已列入），接收者會直接打出，
// 不需要「留著」——例如「1」點到 chop 永遠不是 save clue，就算 Black 變體的 k1 是 critical。
func (s *state) saveCands(obs, giver, R int, it *interp, h game.Hint, base idSet) []cand {
	var out []cand
	focus := it.focus
	for _, t := range base.list() {
		if s.trash(t) || s.playableNow(t) || s.gottenAs(obs, giver, t, focus) {
			continue
		}
		ok := false
		switch {
		case t.color() == game.Black && h.Kind == game.HintColor && (t.rank() == 2 || t.rank() == game.MaxRank):
			ok = s.blackColorSave(it)
		case t.color() == game.Black && h.Kind == game.HintRank && (t.rank() == 3 || t.rank() == 4):
			ok = s.florratSave(t, it)
		case h.Kind == game.HintRank && h.Rank == 5:
			ok = true
		case h.Kind == game.HintRank && h.Rank == 2:
			ok = s.critical(t) || !s.twoVisible(obs, giver, R, focus, t)
		default:
			ok = s.critical(t) && t.rank() != 5
		}
		if ok {
			out = append(out, cand{id: t, save: true})
		}
	}
	return out
}

// twoVisible：obs 是否看得到另一張 t（2 Save 的 Visible Rule：另一張還在，晚點再救那張就好）。
// 包含接收者 R 自己手上的其他牌（R 看不到，但給提示者看得到）；給提示者看不到自己的牌，不算。
// 例外：那張也在它持有者的 chop 上（Double Chop 2's）時仍可救。self 是要救的那張。
func (s *state) twoVisible(obs, giver, R, self int, t ident) bool {
	for p, h := range s.hands {
		if p == obs || p == giver {
			continue
		}
		for _, id := range h {
			if c, ok := s.see(obs, id); ok && c == t && id != self && (p == R || id != s.chop(p)) {
				return true
			}
		}
	}
	return false
}

// onClue 處理一次實際發生的提示：更新 explicit 資訊、寫下筆記、記錄待驗證的連接。
func (s *state) onClue(e game.Event) {
	R := e.Target
	var touched []int
	for _, sl := range e.Touched {
		touched = append(touched, s.hands[R][sl])
	}
	it := s.interpret(e.Player, R, e.Hint, touched)
	fpAt := make([]int, s.n)
	for p := range s.n {
		fpAt[p] = s.finessePos(p)
	}

	var wrong idSet // 被這個提示推翻的筆記（原本確定是某張，提示說不是）
	for _, id := range s.hands[R] {
		n := s.notes[id]
		n.explicit = it.explicit[id]
		if t, ok := n.poss.single(); ok && s.gotten(id) && !n.explicit.has(t) {
			wrong |= setOf(t)
		}
		n.poss &= n.explicit
		if n.poss == 0 {
			n.poss = n.explicit
		}
	}
	// 依 Good Touch 因為那張「確定是 t」而排除 t 的其他已提示牌，把 t 補回來（還有其他理由排除時，之後的推論會再刪掉）。
	// 自我對局 seed 2450：Bob 以為自己的藍牌是 b2，另一張藍牌就排除 b2；「3」修正前者是 b3 後，後者仍不可能是 b2，
	// 它實際就是 b2，之後需要 b2 時他改去盲打 finesse position 而打錯
	for _, t := range wrong.list() {
		for _, id := range s.hands[R] {
			if n := s.notes[id]; s.gotten(id) && !n.bad && n.explicit.has(t) && !n.poss.has(t) && !s.trash(t) && n.poss.count() > 1 {
				n.poss |= setOf(t)
			}
		}
	}
	// 給提示者自己手上被提示的牌，當時的解讀（giverKnew 用）
	var own map[int][]cand
	for _, y := range s.hands[e.Player] {
		for i := len(s.recs) - 1; i >= 0; i-- {
			if r := s.recs[i]; r.focus == y && r.target == e.Player {
				if own == nil {
					own = map[int][]cand{}
				}
				for _, c := range r.cands {
					c.deps = slices.Clone(c.deps)
					own[y] = append(own[y], c)
				}
				break
			}
		}
	}
	for _, id := range it.newCards {
		s.notes[id].giverOwn = own
	}
	for _, id := range touched {
		s.got(id, e.Player)
		n := s.notes[id]
		n.clued, n.cm = true, false // 被提示點到後就是一般的已提示牌
		n.touches++
		n.hinted |= hintBit(e.Hint)
	}
	if it.fix {
		s.applyFix(R, e.Player, it)
		return
	}
	if it.cmKind == cmTrash {
		// 點到的是已知垃圾（會先棄），它們右邊的牌被 Chop Move
		for _, id := range touched {
			s.notes[id].bad, s.notes[id].willPlay = true, false
		}
		s.chopMove(R, it.cm)
		return
	}

	rec := &clueRec{giver: e.Player, target: R, focus: it.focus, cands: it.cands, fallback: it.fallback,
		fpAt: fpAt, changed: []int{it.focus}, chop: it.chopFocus, fresh: it.newCards, seq: s.seq, saveView: it.saveView, free: it.free}
	var u idSet
	for _, c := range it.cands {
		u |= 1 << c.id
	}
	fn := s.notes[it.focus]
	fn.willPlay = len(it.cands) > 0 && !slices.ContainsFunc(it.cands, func(c cand) bool { return c.save })
	if it.hasTruth && !u.has(it.truth) {
		// 我看得到的實際牌面不在我推測的解讀中：我的推測不可靠（通常是因為看不到自己的牌），
		// 不把它寫進接收者的筆記，免得之後拿它來排除自己的牌
		u = 0
	}
	if p := fn.poss & u; p != 0 {
		fn.poss = p
	} else {
		fn.poss = s.touchedUseful(R, e.Player, it.focus, fn.poss)
	}
	// Good Touch：非焦點的新牌將來一定有用，而且同一個提示不會點到兩張一樣的牌（焦點確定時不會是它）。
	// 找不到這種可能，就是違反 Good Touch 的牌（垃圾或重複），只留提示本身的資訊，不當成可打
	var focusID idSet
	if f, ok := fn.poss.single(); ok {
		focusID = setOf(f)
	}
	for _, id := range it.newCards {
		if id != it.focus {
			s.notes[id].poss = nonFocusPoss(s.notes[id].poss, s.freshUseful(R, e.Player, id), focusID)
		}
	}

	// 我看得到焦點牌：照實際牌面判斷誰被 prompt / finesse（save clue 不含 finesse）
	if it.hasTruth && it.planOK && !it.saveOK {
		rec.plan = it.plan
		if !slices.ContainsFunc(it.cands, func(c cand) bool { return c.id == it.truth }) {
			// 實際解讀要接收者 self-prompt / self-finesse，但他還有其他解讀：等那些不成立、
			// 他也這麼想之後（selfMark）才寫他那張牌的筆記
			rec.plan = slices.DeleteFunc(slices.Clone(rec.plan), func(d dep) bool { return d.player == R })
		}
		for _, d := range rec.plan {
			s.markDep(rec, d)
		}
	}
	for _, id := range it.trashNew {
		s.notes[id].bad, s.notes[id].willPlay = true, false
	}
	if it.cmKind == cmFive || it.cmKind == cmTempo {
		s.chopMove(R, it.cm)
	}
	if R == s.me {
		s.selfMark(rec, s.me)
		s.markCommon(rec)
	} else {
		s.pruneByMyCards(rec)
		if s.knows(lvSelfFinesse) {
			s.selfMark(rec, R)
		}
		if s.knows(lvInfoLock) {
			s.lockMine(rec, it)
		}
	}
	s.recs = append(s.recs, rec)
}

// refreshFocus 依剩下的解讀更新焦點牌的筆記。
func (s *state) refreshFocus(rec *clueRec) {
	n, ok := s.note(rec.focus)
	if !ok || s.holder(rec.focus) < 0 {
		return
	}
	var u idSet
	for _, c := range rec.cands {
		u |= 1 << c.id
	}
	if t, ok := s.see(s.me, rec.focus); ok && !u.has(t) {
		return // 和 onClue 一樣：我看得到的實際牌面不在推測的解讀中，推測不可靠，不寫進筆記
	}
	if p := n.poss & u; p != 0 {
		n.poss = p
	}
}

// promoteFocus：其他解讀都不成立、改用 self-prompt / self-finesse 的解讀（fallback 升格）時，焦點的筆記要換成它們。
// 提示當下的筆記只依當時的解讀寫（例如只有「finesse Bob 的 r1 → r2」），和 fallback 沒有交集，refreshFocus 不會改。
// 我看得到焦點、而它不在這些解讀中時不寫（推測不可靠）。
func (s *state) promoteFocus(rec *clueRec) {
	n, ok := s.note(rec.focus)
	if !ok || s.holder(rec.focus) < 0 {
		return
	}
	var u idSet
	for _, c := range rec.cands {
		u |= 1 << c.id
	}
	if t, ok := s.see(s.me, rec.focus); ok && !u.has(t) {
		return
	}
	if n.poss&u == 0 && n.explicit&u != 0 {
		n.poss = n.explicit & u
	}
}

// resetNote 清掉一張牌的隱含筆記，只留 explicit 資訊（被提示過的牌再套 Good Touch）。
func (s *state) resetNote(id int) {
	n, ok := s.note(id)
	if !ok {
		return
	}
	n.finessed, n.willPlay = false, false
	if !n.clued {
		n.giver = -1
	}
	n.poss = n.explicit
	if n.clued {
		if p := n.explicit & s.useful(); p != 0 {
			n.poss = p
		}
	}
}

// onStrike：打錯牌後，從造成這張牌筆記的那次提示起，所有隱含筆記作廢（What to Do After a Strike）。
func (s *state) onStrike(card int) {
	idx := -1
	for i := len(s.recs) - 1; i >= 0; i-- {
		if slices.Contains(s.recs[i].changed, card) {
			idx = i
			break
		}
	}
	if idx < 0 {
		idx = len(s.recs) - 1
	}
	if idx < 0 {
		return
	}
	for _, rec := range s.recs[idx:] {
		for _, id := range rec.changed {
			if s.holder(id) >= 0 {
				s.resetNote(id)
			}
		}
	}
	s.recs = s.recs[:idx]
}
