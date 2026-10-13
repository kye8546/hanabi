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

	// 我看得到焦點牌時才有
	hasTruth bool
	truth    ident
	saveOK   bool  // 實際牌面符合 save clue 的解讀
	plan     []dep // 實際牌面作為 play clue 需要的連接
	planOK   bool
	agreed   bool // 每個要打連接牌的玩家，從自己角度推出的連接都和 plan 一樣
	stray    bool // 計畫外的玩家會從自己的角度以為要打他的牌（他看不到自己的牌，會打錯）
}

const (
	cmTrash = 1 + iota // The Trash Chop Move
	cmFive             // The 5's Chop Move
)

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
		it.tempo = true
		for _, id := range s.hands[R] {
			if slices.Contains(touched, id) {
				it.focus = id
				break
			}
		}
	case len(fresh) == 0:
		// Level 4：只點到舊牌、而且第一次點到 Chop Moved 的牌時，焦點是最左邊那張 Chop Moved 的牌
		it.focus = it.newCards[0]
	case slices.Contains(fresh, chop):
		it.focus, it.chopFocus = chop, true
	case s.level >= 3 && h.Kind == game.HintRank && h.Rank == 1 && !slices.ContainsFunc(fresh, s.fresh):
		// Level 3 Playing Multiple 1's：焦點是最先打的那張 1；點到的全是起手牌時是最右邊（最舊）那張
		// （新抽的牌一定在起手牌左邊，有新抽的 1 時就是最左邊的新牌，和一般規則一樣）
		it.focus = fresh[len(fresh)-1]
	default:
		it.focus = fresh[0]
	}

	if s.findFix(giver, R, h, touched, &it); it.fix {
		return it
	}
	if s.findChopMove(giver, R, h, touched, fresh, &it); it.cmKind == cmTrash {
		return it
	}

	// 我看得到被點到的牌時：這次提示的 Good Touch 推論立刻生效（給提示者看得到我的已提示牌，
	// 就不會點到和它們一樣的牌），算解讀時就用得上
	if R != s.me {
		s.pending, s.pendingGiver = touched, giver
		defer func() { s.pending = nil }()
	}

	// 這次被點到的牌不在 finesse position 上（finesseSlot 會跳過）
	s.touchedNow = touched
	defer func() { s.touchedNow = nil }()

	// 計算期間把接收者的筆記換成「提示後」的樣子（負向資訊會影響 prompt 候選）
	s.overlay = map[int]idSet{}
	for id, ex := range it.explicit {
		s.overlay[id] = s.notes[id].poss & ex
	}
	defer func() { s.overlay = nil }()

	base := s.overlay[it.focus]
	candsOf := func() []cand {
		cs := s.playCands(R, giver, R, it.focus, base)
		if it.chopFocus || it.cmKind == cmFive || s.fiveStall(giver, h) && (len(it.newCards) > 0 || s.mustStall(giver)) {
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
		s.optimistic = true
		alt = candsOf()
		s.optimistic = false
	}
	notSelf := func(c cand) bool { return !selfDep(c) }
	if slices.ContainsFunc(it.cands, notSelf) || slices.ContainsFunc(alt, notSelf) {
		// 只有「樂觀地假設接收者看得到我手上的連接牌」時才有其他解讀：他實際上很可能只剩 self 解讀
		it.selfRisk = s.level >= 2 && !slices.ContainsFunc(it.cands, notSelf)
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
	if s.level >= 2 && slices.ContainsFunc(it.cands, selfFin) && slices.ContainsFunc(it.cands, func(c cand) bool { return !selfFin(c) }) {
		it.fallback = append(it.fallback, slices.DeleteFunc(slices.Clone(it.cands), func(c cand) bool { return !selfFin(c) })...)
		it.cands = slices.DeleteFunc(it.cands, selfFin)
	}

	if t, ok := s.see(s.me, it.focus); ok {
		it.hasTruth, it.truth = true, t
		for _, c := range it.cands {
			if c.save && c.id == t {
				it.saveOK = true
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
			if !ok || !slices.Equal(theirs, it.plan) {
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

// fresh：不是起手牌（Level 3 Fresh 1's）。
func (s *state) fresh(id int) bool { return !s.notes[id].start }

// findFix（Level 3 The Fix Clue）：從接收者 R 的角度，這個提示是否在修正違反 Good Touch 的牌。
//   - 補資訊（fill in）：點到 R 手上已被拿到、原本被當成有用的牌，提示後它只可能是沒用的牌（垃圾，或別處已被拿到的重複），
//     或和原本的筆記矛盾（Breaking Information Lock）。
//   - 不給新資訊（Stop Clue）：沒點到新牌、點到的牌也沒多任何資訊，卻點到 R 下一手要打的牌：那張是壞的。
//     （提示滿 8 / 鎖手時重新點 5 是 5 Stall，不算。）
//
// 「A clue cannot be a Play Clue and a Fix Clue at the same time」：是 fix clue 就沒有 play / save 的解讀。
func (s *state) findFix(giver, R int, h game.Hint, touched []int, it *interp) {
	if s.level < 3 {
		return
	}
	good := func(set idSet, y int) idSet {
		var g idSet
		for _, i := range set.list() {
			if !s.trash(i) && !s.gottenAs(R, giver, i, y) {
				g |= setOf(i)
			}
		}
		return g
	}
	mark := func(y int, bad bool, poss idSet) {
		it.fix = true
		if it.fixPoss == nil {
			it.fixPoss = map[int]idSet{}
		}
		it.fixPoss[y] = poss
		if bad {
			it.fixed = append(it.fixed, y)
		}
	}
	for _, y := range touched {
		if !s.live(y) {
			continue
		}
		before := s.notes[y].poss
		if good(before, y) == 0 {
			continue // 本來就知道沒用
		}
		ex := it.explicit[y]
		switch after := before & ex; {
		case after == 0:
			// 筆記被證實是錯的：只剩提示本身的資訊（依 Good Touch 仍先假設是有用的牌）
			if g := good(ex, y); g != 0 {
				mark(y, false, g)
			} else {
				mark(y, true, ex)
			}
		case good(after, y) == 0:
			mark(y, true, after)
		}
	}
	// 補資訊後 R 手上兩張以上已被拿到的牌確定是同一張：揭露了重複（例如三張被提示的 1 被紅色補成全是 r1），
	// 只留一張，其餘是壞的
	same := map[ident][]int{}
	for _, y := range touched {
		if _, done := it.fixPoss[y]; done || !s.live(y) {
			continue
		}
		if t, ok := (s.notes[y].poss & it.explicit[y]).single(); ok {
			same[t] = append(same[t], y)
		}
	}
	for _, ys := range same {
		for _, y := range ys[1:] {
			mark(y, true, s.notes[y].poss&it.explicit[y])
		}
	}
	if it.fix || len(it.newCards) > 0 || s.fiveStall(giver, h) && s.mustStall(giver) {
		return
	}
	for _, y := range touched {
		if it.explicit[y] != s.notes[y].explicit {
			return
		}
	}
	if y := s.nextPlay(R); y >= 0 && slices.Contains(touched, y) {
		// 壞掉的牌：已經沒用，或可能和別處被拿到的牌重複。它根本不可能是壞牌時（例如確定是還沒打出的 5），
		// Fix Clue 沒東西可修，就不是 Stop Clue（多半只是拖延）
		var dup idSet
		for _, i := range s.notes[y].poss.list() {
			if s.trash(i) || s.gottenAs(R, giver, i, y) || s.mayDuplicate(R, y, i) {
				dup |= setOf(i)
			}
		}
		if dup != 0 {
			mark(y, true, dup)
		}
	}
}

// findChopMove（Level 4）：從接收者 R 的角度，這個提示是否是 Chop Move。fresh 是沒被 Chop Move 的新牌。
//   - The Trash Chop Move：點到的牌全是 R 推得出的垃圾（已打出、打不出去，或別處已被拿到 / 張數已用完）。
//     最右邊那張新牌右邊、沒被提示的牌全部 Chop Move（垃圾就在 chop 上時沒有牌可 Chop Move，不算；那是 Level 14 的 Trash Push）。
//   - The 5's Chop Move：不是拖延的情況下（5 Stall 優先），數字 5 點到的最右邊那張新的 5 正好在 chop 左邊一張：
//     Chop Move chop 上那一張（只有一張）。點到 chop 的是 5 Save；離 chop 兩張以上的 5 仍是 play clue。
func (s *state) findChopMove(giver, R int, h game.Hint, touched, fresh []int, it *interp) {
	if s.level < 4 || len(fresh) == 0 {
		return
	}
	// 只用大家都知道的資訊（提示本身、被提示與 Chop Move 的牌）：私下的推論或只有部分人知道的 finesse
	// 會讓各家對「是不是 Chop Move、Chop Move 哪幾張」看法不一
	allTrash := true
	for _, y := range touched {
		// 點到還被當成有用的舊牌是 Fix Clue 的範圍（接收者私下可能早就知道它沒用，各家看法會不一樣）
		if s.live(y) || !s.trashFor(y, it.explicit[y]) {
			allTrash = false
			break
		}
	}
	last := fresh[len(fresh)-1]
	if allTrash {
		var cm []int
		for _, id := range s.hands[R][slices.Index(s.hands[R], last)+1:] {
			if n := s.notes[id]; !n.clued && !n.cm {
				cm = append(cm, id)
			}
		}
		if len(cm) > 0 {
			it.cmKind, it.cm, it.focus = cmTrash, cm, last
		}
		return
	}
	if h.Kind != game.HintRank || h.Rank != game.MaxRank || s.fiveStall(giver, h) {
		return
	}
	var unclued []int // 由右而左
	for i := len(s.hands[R]) - 1; i >= 0; i-- {
		if n := s.notes[s.hands[R][i]]; !n.clued && !n.cm {
			unclued = append(unclued, s.hands[R][i])
		}
	}
	if len(unclued) >= 2 && last == unclued[1] && !slices.Contains(touched, unclued[0]) {
		it.cmKind, it.cm, it.focus = cmFive, []int{unclued[0]}, last
	}
}

// trashFor：牌 y 的可能 p（提示本身的資訊）全是大家都知道的垃圾：已打出、打不出去，或別處有提示本身就確定是那張的牌
// （Trash Chop Move 原文的「already played or accounted for」）。只用共同資訊：要是只有接收者推得出是垃圾
// （例如靠旁觀者自己看不到的牌），旁觀者會把它當成 play clue 而盲打。
func (s *state) trashFor(y int, p idSet) bool {
	if p == 0 {
		return false
	}
	for _, i := range p.list() {
		if !s.trash(i) && !s.knownElsewhere(i, y) {
			return false
		}
	}
	return true
}

// knownElsewhere：除了 y 以外，有提示本身就確定是 i 的已提示牌（大家都知道）。
func (s *state) knownElsewhere(i ident, y int) bool {
	for _, h := range s.hands {
		for _, id := range h {
			if id == y || !s.live(id) {
				continue
			}
			if t, ok := s.notes[id].explicit.single(); ok && t == i {
				return true
			}
		}
	}
	return false
}

// mayDuplicate：（R 的角度）R 的牌 y 若是 i，可能和另一張被拿到的牌重複：R 看得到那張就是 i，
// 或那張是 R 自己看不到的牌、可能是 i，而 i 還剩至少兩張（例如兩張被提示的 1）。
func (s *state) mayDuplicate(R, y int, i ident) bool {
	for _, h := range s.hands {
		for _, z := range h {
			if z == y || !s.live(z) {
				continue
			}
			if t, ok := s.see(R, z); ok {
				if t == i {
					return true
				}
			} else if s.holder(z) == R && s.notes[z].poss.has(i) && s.remainingFor(R, i) >= 2 {
				return true
			}
		}
	}
	return false
}

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
func (s *state) saveCands(obs, giver, R int, it *interp, h game.Hint, base idSet) []cand {
	var out []cand
	focus := it.focus
	for _, t := range base.list() {
		if s.trash(t) || s.gottenAs(obs, giver, t, focus) {
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

// 黑色（每個數字只有 1 張、全都是 critical）的 save clue 慣例（https://hanabi.github.io/variant-specific/black）：
// 黑 2、黑 5 用數字救，黑 3、黑 4 用顏色救（縮小被救的是哪張）；用另一種提示點到 chop 上的它們，
// 不符合例外時就是 play clue（接收者照 play clue 找連接牌，找不到就排除那張）。

// blackColorSave：黑色提示能不能是救黑 2 / 黑 5 的 save clue。例外只算大家都看得出的兩種：
// 點到兩張以上之前沒被提示的牌（都是黑色），或補上一張已提示牌的資訊（filled in an ancillary card）。
// 「為了不違反 Good Touch」「給了重要的負向資訊」要看接收者看不到的牌、或靠主觀判斷，接收者無從確認，不算
// （AI 也就不會那樣用黑色救，改用數字）。
func (s *state) blackColorSave(it *interp) bool {
	if len(it.newCards) >= 2 {
		return true
	}
	for _, id := range s.touchedNow {
		if n := s.notes[id]; n.clued && it.explicit[id] != n.explicit {
			return true
		}
	}
	return false
}

// florratSave：數字 3 / 4 能不能是救黑 3 / 黑 4 的 save clue（Florrat Save）：還點到另一張新牌，
// 依提示本身它可能是 critical（只用公開的棄牌堆判斷，大家看法一致）。
func (s *state) florratSave(t ident, it *interp) bool {
	for _, id := range it.newCards {
		if id == it.focus {
			continue
		}
		for _, x := range (it.explicit[id] & s.all).list() {
			if x != t && s.critical(x) {
				return true
			}
		}
	}
	return false
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
		if len(deps) > 0 && s.level < 2 {
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
	old := s.sameClue
	s.sameClue = rec.fresh
	defer func() { s.sameClue = old }()
	return s.connect(obs, rec.giver, rec.target, rec.focus, t)
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
		if n := s.notes[id]; n.finessed && !n.clued && s.level >= 2 && s.level < 5 {
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
			if s.level < 2 {
				break
			}
			continue
		}
		order = append(order, p)
	}
	if s.level >= 2 {
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

// mustStall：給提示者這一手不能棄牌（提示滿 8，或手牌全被拿到），只能給拖延提示。
func (s *state) mustStall(giver int) bool { return s.hints >= game.MaxHints || s.chop(giver) < 0 }

// fiveStall：數字 5 提示點到不在 chop 上的新的 5 時，可以當成 5 Stall（Level 2）。只重新點到舊的 5 是 tempo clue
// （這張能打），除非給提示者這一手不能棄牌（那只是拖延）：
// Early Game 中，或給提示者不能棄牌（提示滿 8、或手牌全被拿到）時，只是救下這些 5、拖延時間。
func (s *state) fiveStall(giver int, h game.Hint) bool {
	if s.level < 2 || h.Kind != game.HintRank || h.Rank != game.MaxRank {
		return false
	}
	return !s.midGame || s.hints >= game.MaxHints || s.chop(giver) < 0
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

	for _, id := range s.hands[R] {
		n := s.notes[id]
		n.explicit = it.explicit[id]
		n.poss &= n.explicit
		if n.poss == 0 {
			n.poss = n.explicit
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
		fpAt: fpAt, changed: []int{it.focus}, chop: it.chopFocus, fresh: it.newCards}
	mayBeSave := slices.ContainsFunc(it.cands, func(c cand) bool { return c.save })
	for _, id := range it.newCards {
		s.notes[id].bySave = mayBeSave
	}
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
	if it.cmKind == cmFive {
		s.chopMove(R, it.cm)
	}
	if R == s.me {
		s.selfMark(rec, s.me)
		s.markCommon(rec)
	} else {
		s.pruneByMyCards(rec)
		if s.level >= 2 {
			s.selfMark(rec, R)
		}
		if s.level >= 3 {
			s.lockMine(rec, it)
		}
	}
	s.recs = append(s.recs, rec)
}

// lockMine（Level 3 Information Lock on In-Between Cards）：我看得到焦點是 t、它現在還不能打，中間需要的某張 m
// 不在計畫的 prompt / finesse 裡、別人手上也沒有已被拿到的 m——那就是我手上那張可能是 m 的已提示牌
// （例如 Cathy 的 r4 被提示，Bob 手上的 2 就鎖定成 r2）。只有一張候選時才鎖定。
func (s *state) lockMine(rec *clueRec, it interp) {
	if !it.hasTruth || !it.planOK || it.saveOK || !slices.ContainsFunc(it.cands, func(c cand) bool { return c.id == it.truth }) {
		return
	}
	t := it.truth
	var chain idSet
	for r := s.stacks[t.color()] + 1; r <= t.rank(); r++ {
		chain |= setOf(identOf(t.color(), r))
	}
	for r := s.stacks[t.color()] + 1; r < t.rank(); r++ {
		m := identOf(t.color(), r)
		if slices.ContainsFunc(it.plan, func(d dep) bool { return d.id == m }) {
			continue
		}
		elsewhere := false
		for p, h := range s.hands {
			for _, id := range h {
				if p == s.me || !s.live(id) {
					continue
				}
				if c, ok := s.see(s.me, id); ok && c == m || s.notes[id].poss == setOf(m) {
					elsewhere = true
				}
			}
		}
		// 給提示者看不到自己的牌：他手上可能是 m 的已提示牌，他也許以為就是它，連接牌就不一定是我的
		for _, id := range s.hands[rec.giver] {
			if rec.giver != s.me && s.live(id) && s.notes[id].poss.has(m) {
				elsewhere = true
			}
		}
		if elsewhere {
			continue
		}
		var mine []int
		for _, id := range s.hands[s.me] {
			if s.live(id) && s.myPoss(id).has(m) {
				mine = append(mine, id)
			}
		}
		// 我那張也可能是鏈上的其他牌（例如 g2 或 g4，焦點是 g5）時，不知道它補的是哪一張
		if len(mine) == 1 && s.myPoss(mine[0])&chain == setOf(m) {
			s.notes[mine[0]].poss = setOf(m)
			rec.changed = append(rec.changed, mine[0])
		}
	}
}

// applyFix：Fix Clue 只修正違反 Good Touch 的牌。被修正的牌寫下新筆記（確定沒用的標成 bad，會先棄掉）；
// 一起點到的新牌依 Good Touch 是有用的牌，但不代表能打（不是 play clue）。
func (s *state) applyFix(R, giver int, it interp) {
	for y, p := range it.fixPoss {
		n := s.notes[y]
		if p != 0 {
			n.poss = p
		}
		n.willPlay = false
		n.bad = slices.Contains(it.fixed, y)
	}
	for _, id := range it.newCards {
		s.notes[id].poss = nonFocusPoss(s.notes[id].poss, s.freshUseful(R, giver, id), 0)
	}
}

// touchedUseful：提示沒有任何解讀時，依 Good Touch 被點到的牌仍是將來有用、而且別處沒被拿到過的牌。
// 一種都找不到就是違反 Good Touch 的牌（垃圾或重複），維持原本的筆記（之後會被推論成已知沒用的牌）。
func (s *state) touchedUseful(obs, giver, id int, p idSet) idSet {
	var q idSet
	for _, i := range (p & s.useful()).list() {
		if !s.gottenAs(obs, giver, i, id) {
			q |= setOf(i)
		}
	}
	if q != 0 {
		return q
	}
	return p
}

// pruneByMyCards：算接收者的解讀時我看不到自己的牌；現在若推論出我手上某張「被提示的牌」就是 x，
// 接收者其實看得到它，依 Good Touch 不會把焦點當成 x（給提示者看不到自己的牌，所以不算給提示者手上的）。
func (s *state) pruneByMyCards(rec *clueRec) {
	if s.me == rec.giver {
		return
	}
	var mine idSet
	for _, id := range s.hands[s.me] {
		if n := s.notes[id]; n.clued {
			if t, ok := n.poss.single(); ok {
				mine |= setOf(t)
			}
		}
	}
	kept := slices.DeleteFunc(slices.Clone(rec.cands), func(c cand) bool { return mine.has(c.id) })
	if len(kept) > 0 && len(kept) < len(rec.cands) {
		rec.cands = kept
		s.refreshFocus(rec)
	}
}

// freshUseful：（obs 角度）將來有用、而且別處沒有被拿到的牌面。依 Good Touch，被提示的新牌只會是這些
// （別處已被提示的牌面，再點到就是重複）。exclude 是正在推論的那張牌。
func (s *state) freshUseful(obs, giver, exclude int) idSet {
	var u idSet
	for _, i := range s.useful().list() {
		if !s.gottenAs(obs, giver, i, exclude) {
			u |= setOf(i)
		}
	}
	return u
}

// nonFocusPoss 是被提示點到的非焦點新牌的筆記：依 Good Touch 將來一定有用（useful 應已扣掉別處被拿到的牌面）、
// 也不會和焦點重複。
// 找不到這種可能時，它就是看得出來的違反 Good Touch 的牌：只會是垃圾或焦點的重複
// （焦點打出後就是已知沒用的牌，會先棄它而不是 chop）。
func nonFocusPoss(poss, useful, focus idSet) idSet {
	if p := poss & useful &^ focus; p != 0 {
		return p
	}
	if p := poss&^useful | poss&focus; p != 0 {
		return p
	}
	return poss
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
		if i < 0 || s.level < 2 && len(c.deps) != 1 {
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
	if s.level < 2 || len(rec.cands) == 0 {
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
		if s.level >= 2 && e.Player != rec.target && e.Player != s.me && !slices.Contains(rec.passed, e.Player) {
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
	defer func() { s.skipFinesse, s.skipCards = nil, nil }()
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
				if t, ok := s.see(rec.target, d.card); !ok || t != d.id || d.player == s.me {
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
	if t, ok := s.see(s.me, rec.focus); ok && slices.Contains(lost, t) && s.passMeansMe(rec, e.Player, t) {
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

// passMeansMe：從沒盲打的 p 的角度（不跳過他自己）找 t 的連接時，需要的 finesse 落在他自己或我身上。
func (s *state) passMeansMe(rec *clueRec, p int, t ident) bool {
	// 只處理「他在給提示者與接收者之間、我在接收者之後」：他沒打，是因為看到我可能被逆飛
	pos := func(q int) int { return (q - rec.giver + s.n) % s.n }
	if !(pos(p) < pos(rec.target) && pos(rec.target) < pos(s.me)) {
		return false
	}
	skip := s.skipFinesse
	s.skipFinesse = nil // 他自己不跳過；他沒打的那張仍不算「已經有人會打」（skipCards 保留）
	defer func() { s.skipFinesse = skip }()
	deps, ok := s.connectRec(rec, p, t)
	if !ok {
		return false
	}
	for _, d := range deps {
		if d.finesse && (d.player == p || d.player == s.me) {
			return true
		}
	}
	return false
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

func depsOf(cs []cand) [][]dep {
	var out [][]dep
	for _, c := range cs {
		out = append(out, c.deps)
	}
	return out
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

// refreshFocus 依剩下的解讀更新焦點牌的筆記。
func (s *state) refreshFocus(rec *clueRec) {
	n, ok := s.notes[rec.focus]
	if !ok || s.holder(rec.focus) < 0 {
		return
	}
	var u idSet
	for _, c := range rec.cands {
		u |= 1 << c.id
	}
	if p := n.poss & u; p != 0 {
		n.poss = p
	}
}

// resetNote 清掉一張牌的隱含筆記，只留 explicit 資訊（被提示過的牌再套 Good Touch）。
func (s *state) resetNote(id int) {
	n, ok := s.notes[id]
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
