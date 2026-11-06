// Level 11（docs/level-11.mdx）：Bluffs。
//
// Bluff 只能從 Bluff Seat 給（給提示者的下一位盲打），而且他必須在下一手就盲打（Lie Principle）：所以 Bluff 一定在
// 提示之後的第一個行動兌現，在那之前沒有別人行動。提示當下大家照原本的方式解讀（Bob's Truth Principle：被 Bluff 的人
// 看不到自己的牌，只會以為是 finesse 自己；看得到別人有連接牌就當成 finesse 他），等他盲打出牌後再照 Cathy's Connecting
// Principle 判斷：接得上提示就是 finesse，接不上（或接得上但不可能是 finesse）而焦點可能是差一張就能打的牌，就是 Bluff
// （detectBluff），焦點記成那些牌，他不再往下盲打（Bluff 優先於 Layered Finesse）。

package hgroup

import (
	"slices"

	"hanabi/game"
)

// bluffSeat：giver 坐在 p 的 Bluff Seat（p 是他的下一位）。
func (s *state) bluffSeat(giver, p int) bool { return (giver+1)%s.n == p }

// bluffSlot：提示後 B 被 finesse / Bluff 時會盲打的牌：最左邊沒被提示、沒被 Chop Move、不是這次點到的牌。
// 他手上有別的提示要盲打、還沒打的牌時回傳 -1——Queued Bluffs 不合法：他會先打那張，Bluff 不會在下一手兌現。
// rec 是這次提示（被它 finesse 的牌不算別的提示的；給提示前模擬時是 nil）。
func (s *state) bluffSlot(rec *clueRec, B int, touched []int) int {
	fp := -1
	for _, id := range s.hands[B] {
		n := s.notes[id]
		if n.finessed && !n.clued && (rec == nil || n.finRec != rec) {
			return -1
		}
		if fp < 0 && !n.clued && !n.cm && !slices.Contains(touched, id) {
			fp = id
		}
	}
	return fp
}

// bluffBlocked：B 從 finesse position 盲打出 x（played）不能當成 rec 的 Bluff——他多半是在接別的提示：
// 別的提示要他盲打這張，或他從較早的某個提示之後還沒輪過、x 可能是那個提示的焦點的連接牌（從我的角度那個提示
// 也許不需要他——例如我以為自己的已提示牌就是連接牌——但他看得到我的牌）。自我對局 4 人局 seed 159：綠色要 AI-1 盲打 g1，
// AI-3 以為自己被「1」點到的牌可能是 g1；AI-0 接著用「3」點 AI-1，AI-1 盲打出 g1，被讀成「3」的 Bluff。
// rec 是 nil 時是給提示前的模擬（所有已記錄的提示都比它早）。
func (s *state) bluffBlocked(rec *clueRec, B, played int, x ident) bool {
	for _, r := range s.recs {
		if r == rec {
			continue
		}
		// 接收者自己的 Self-Finesse 沒有 layer（盲打出什麼焦點就要對上它）：盲打出的不是那張，就不是在接它
		uses := func(d dep) bool { return d.card == played && (d.player != r.target || d.id == x) }
		usesAll := func(cs []cand) bool {
			return len(cs) > 0 && !slices.ContainsFunc(cs, func(c cand) bool { return !slices.ContainsFunc(c.deps, uses) })
		}
		// 計畫要他盲打這張，或那個提示的每種解讀都要他盲打這張（只有部分解讀要時，看得到焦點的他多半知道不是）
		if slices.ContainsFunc(r.plan, uses) || usesAll(r.cands) || len(r.cands) == 0 && usesAll(r.fallback) {
			return true
		}
	}
	for _, r := range s.recs {
		if r == rec || r.seq <= s.acted[B] || s.holder(r.focus) < 0 {
			continue
		}
		ids := s.notes[r.focus].poss
		if t, ok := s.see(s.me, r.focus); ok && rec != nil {
			ids = setOf(t) // 給提示前模擬時保守一點：看不到焦點的人（例如它的持有者）只知道共同筆記
		}
		if slices.ContainsFunc(ids.list(), func(t ident) bool { return t.color() == x.color() && t.rank() > x.rank() }) {
			return true
		}
	}
	return false
}

// bluffConnects（Cathy's Connecting Principle）：盲打出的 x 和提示 h「接得上」——顏色提示點得到 x（彩虹牌接得上任何顏色，
// 原文 Example 7），數字提示 x 正好小一號（Example 3：3 接得上 4；Example 4：1 接不上 4，就算可能是 Triple Finesse）。
// 只照原文：中間隔著已提示的牌（例如「4」、別人已提示的 b3、盲打出 b2）也算接不上——那是 Bluff Through Already-Clued
// Cards，bluffTargets 照樣找得到 b4；「中間的牌 prompt 得到」要看各人看不到的手牌，判斷會不一致
// （自我對局 4 人局 seed 42：「5」點 W5、盲打出 W3，給提示者以為自己可能有已提示的 W4，讀成 finesse 而盲打錯）。
func (s *state) bluffConnects(rec *clueRec, x ident) bool {
	h := rec.hint
	if h.Kind == game.HintColor {
		return hintSet(h).has(x)
	}
	return x.rank()+1 == h.Rank
}

// bluffFinesse：焦點可能是 x 那一色、比 x 大的牌（x 可以是它的連接牌，finesse 說得通）。只看提示資訊與接收者看得到的牌——
// 旁觀者替接收者算的解讀會把連接牌放在自己看不到的 finesse position 上，不能拿來判斷（原文 Example 6 / 8：
// 接得上，但焦點的負向資訊排除了那一色，就是 Bluff）。
func (s *state) bluffFinesse(rec *clueRec, ex idSet, x ident) bool {
	for _, t := range (ex & s.useful()).list() {
		if t.color() == x.color() && t.rank() > x.rank() && !s.gottenAs(rec.target, rec.giver, t, rec.focus) && s.remainingFor(rec.target, t) > 0 {
			return true
		}
	}
	return false
}

// bluffTargets：Bluff 的盲打 x 打出後，從接收者 R 的角度，焦點可能是哪些差一張就能打的牌（Bluff 的目標）：
// 下一張該打的牌沒有人有（沒被拿到、也 prompt 不到——那張就是 Bluff 少的那一張），中間其他的牌都已被拿到或 prompt 得到
// （Bluffs Through Already-Clued Cards）。差兩張以上的牌不能當 Bluff 的目標（原文：「Bluffs on two-away-from-playable cards
// are normally illegal」）。ex 是焦點的提示資訊；assigns[t] 是焦點是 t 時中間被 prompt 到的已提示牌（它們就是那幾張）。
func (s *state) bluffTargets(rec *clueRec, ex idSet, played int, x ident) (idSet, map[ident][]dep) {
	defer s.saveSim()()
	s.sameClue, s.free, s.clueSeq = rec.fresh, rec.free, rec.seq
	s.skipCards = append(slices.Clone(s.skipCards), played)
	// 「差一張就能打」照盲打之前的煙火判斷：原文 Level 13 The 3 Bluff 開頭說 Bluff 點到的牌是差一張就能打的，
	// 它的 Example 5（「3」點到 r3、Bob 盲打出 r1）是 Level 13 另外約定的 3 Bluff——提示當下差兩張、只因為盲打出
	// 同色的牌才變成差一張的，不是 Level 11 的 Bluff 目標（使用者回報 20261106-123034 t15：AI-2 用「3」點使用者 chop 上
	// 的 Y3、要他盲打出 Y1，那時 Y3 還差 Y1、Y2 兩張）
	saved := s.stacks
	defer func() { s.stacks = saved }()
	s.stacks[x.color()] = x.rank() - 1

	R, giver := rec.target, rec.giver
	var targets idSet
	assigns := map[ident][]dep{}
	for _, t := range (ex & s.useful()).list() {
		if s.playableNow(t) || s.gottenAs(R, giver, t, rec.focus) || s.remainingFor(R, t) <= 0 {
			continue
		}
		c := t.color()
		used := append([]int{rec.focus, played}, rec.fresh...)
		var deps []dep
		ok := true
		for r := s.stacks[c] + 1; r < t.rank() && ok; r++ {
			m := identOf(c, r)
			next := r == s.stacks[c]+1
			restore := s.saveSim()
			if next {
				// 下一張該打的牌：B 看得到我的手牌，我手上已提示的牌若是它，他會當成 prompt、不會盲打——他盲打了，
				// 我手上的就不是它（我看不到自己的牌，筆記上的「可能」不算；自我對局 4 人局 seed 13）
				s.skipCards = append(slices.Clone(s.skipCards), s.hands[s.me]...)
				s.notPrompt = append(slices.Clone(s.notPrompt), s.hands[s.me]...)
			}
			got := s.alreadyGotten(R, giver, m, used) >= 0
			var d dep
			var took []int
			prompt := false
			if !got {
				d, took, prompt = s.findPrompt(R, giver, m, used)
			}
			restore()
			switch {
			case next:
				ok = !got && !prompt // 下一張該打的牌就是 Bluff 少的那一張
			case prompt:
				deps = append(deps, d)
				used = append(used, took...)
			case !got:
				ok = false // 還差第二張：不是 Bluff 的目標
			}
		}
		if ok {
			targets |= setOf(t)
			assigns[t] = deps
		}
	}
	return targets, assigns
}

// detectBluff（Level 11）：上一個行動是 giver 給的提示 rec，他的下一位（Bluff Seat）接著從 finesse position 盲打出 x。
// 照 Cathy's Connecting Principle：x 接得上提示、也有解讀要它（finesse）就照一般流程；否則焦點可能是差一張就能打的牌時
// 就是 Bluff——焦點記成那些牌，為這個提示寫下的其他連接（例如以為是 Layered Finesse、要他繼續盲打，或旁觀者以為要
// finesse 自己）全部作廢。Self-Bluff 只能用數字提示（顏色的 Self Color Bluff 是更高等級的技巧）。
func (s *state) detectBluff(e game.Event, rec *clueRec) bool {
	if !s.knows(lvBluff) || rec == nil || !s.bluffSeat(rec.giver, e.Player) || s.holder(rec.focus) != rec.target ||
		e.Player == rec.target && rec.hint.Kind != game.HintRank {
		return false
	}
	played := e.Card.ID
	if s.bluffSlot(rec, e.Player, nil) != played || s.gdCard(played) {
		return false
	}
	x := identOfCard(e.Card)
	if s.bluffBlocked(rec, e.Player, played, x) {
		return false
	}
	if s.bluffConnects(rec, x) && s.bluffFinesse(rec, s.notes[rec.focus].explicit, x) {
		return false // 接得上，是 finesse
	}
	// 先撤掉為這個提示寫下的連接筆記（例如旁觀者以為要 finesse 自己、記成被 finesse 的 r2，或被 prompt 成 r3 的已提示牌），
	// 照沒有這個提示的連接來找目標；不是 Bluff 就還原
	saved := map[int]note{}
	for _, ds := range append([][]dep{rec.plan}, append(depsOf(rec.cands), depsOf(rec.fallback)...)...) {
		for _, d := range ds {
			if _, done := saved[d.card]; done || d.card == played || d.card == rec.focus || s.holder(d.card) < 0 || !slices.Contains(rec.changed, d.card) {
				continue
			}
			if n := s.notes[d.card]; n.finRec == rec || !n.finessed && n.poss == setOf(d.id) {
				saved[d.card] = *n
				s.resetNote(d.card)
			}
		}
	}
	targets, assigns := s.bluffTargets(rec, s.notes[rec.focus].explicit, played, x)
	if targets == 0 {
		for id, n := range saved {
			*s.notes[id] = n
		}
		return false // 沒有可以當 Bluff 目標的牌（原文 Example 5：那就是 finesse，例如 Layered / Triple Finesse）
	}
	s.applyBluff(rec, targets, assigns)
	// 盲打出的牌不算被這個提示拿到：只有給提示者和他自己（還有以為是 finesse 他的人）記過它被 finesse，
	// 照它做 Good Touch 排除（revealGTP）會讓各家筆記不一致（自我對局 5 人局 seed 201：Bluff 盲打出 w1 後，
	// 只有其中兩人把別人被「1」點到的牌排除了 w1，出牌順序與 Order Chop Move 的判斷就不同）
	if n := s.notes[played]; !n.clued {
		n.finessed, n.finRec, n.giver = false, nil, -1
	}
	return true
}

// applyBluff：rec 確定是 Bluff（連接筆記已撤掉）。焦點記成 Bluff 的目標；中間被 prompt 到的已提示牌，
// 目標只剩一種（或我看得到焦點）時記成那幾張（Bluffs Through Already-Clued Cards：Cathy 的 3 就是 r3）。
func (s *state) applyBluff(rec *clueRec, targets idSet, assigns map[ident][]dep) {
	rec.plan, rec.fallback, rec.cands, rec.bluff = nil, nil, nil, true
	for _, t := range targets.list() {
		rec.cands = append(rec.cands, cand{id: t})
	}
	n := s.notes[rec.focus]
	n.willPlay = true
	truth, see := s.see(s.me, rec.focus)
	if see && !targets.has(truth) {
		// 我看得到的實際牌面不在推得的目標中（給提示者弄錯了）。Bluff 的讀法只看大家都知道的資訊，接收者一定照它記，
		// 所以照樣寫進共同筆記，Fix Clue 才看得出他記錯、會打錯（使用者回報 20261106-151527：t16 使用者紅色點 AI-2 的 R4，
		// AI-1 盲打出 G3，大家讀成 Bluff、R3；AI-1 看得到是 R4 就不寫，一直不知道 AI-2 以為是 R3，t29 還提示使用者的 R2，
		// t33 AI-2 打錯 R4）。中間的牌（assigns）不寫：那是照錯的目標推的
		if p := n.explicit & targets; p != 0 {
			n.poss = p
		}
		return
	}
	t, ok := targets.single()
	if see {
		t, ok = truth, true
	}
	if ok {
		for _, d := range assigns[t] {
			s.markDep(rec, d)
		}
		// 記下中間的牌之後，別處已被拿到的牌面就不是焦點（Cathy 推得自己的 3 是 r3 後，Donald 的紅牌就不是 r3）
		for _, u := range targets.list() {
			if u != t && s.gottenAs(rec.target, rec.giver, u, rec.focus) {
				targets &^= setOf(u)
			}
		}
	}
	if p := n.explicit & targets; p != 0 {
		n.poss = p
	}
}

// guided（Level 11 Guide Principle）：上一位剛給的提示要我盲打（我坐在他的 Bluff Seat；finesse 或 Bluff 都一樣）。
// 他在引導我現在就盲打，不必先替下一位救 chop——他需要我救的話會晚點再給（Lie Principle：Bluff 不在下一手兌現就不成立）。
func (s *state) guided() bool {
	rec := s.lastClue
	return s.knows(lvBluff) && rec != nil && s.bluffSeat(rec.giver, s.me) && slices.ContainsFunc(s.hands[s.me], func(id int) bool {
		n := s.notes[id]
		return n.finessed && !n.clued && n.finRec == rec
	})
}

// bluffRead（Level 11，給提示前模擬）：照計畫 plan，我的下一位 B 下一手就會從 finesse position 盲打出 x（finesse 或 Layered
// Finesse 的 layer）；大家照 Cathy's Connecting Principle 會不會把它讀成 Bluff（焦點是差一張就能打的牌）。
// 回傳讀成 Bluff 時焦點的目標、中間被 prompt 的牌，以及 B 盲打的那張牌。
func (s *state) bluffRead(R int, h game.Hint, it interp, touched []int, plan []dep) (idSet, map[ident][]dep, int, bool) {
	B := (s.me + 1) % s.n
	i := slices.IndexFunc(plan, func(d dep) bool { return d.player == B })
	if i < 0 || !plan[i].finesse || !s.playableNow(plan[i].id) || B == R && h.Kind != game.HintRank {
		return 0, nil, -1, false
	}
	fp := plan[i].card
	if s.bluffSlot(nil, B, touched) != fp {
		return 0, nil, -1, false
	}
	c, ok := s.card(fp)
	if !ok || !s.playableNow(identOfCard(c)) {
		return 0, nil, -1, false
	}
	x := identOfCard(c)
	defer s.saveSim()()
	s.pending, s.pendingGiver, s.touchedNow = touched, s.me, touched
	s.overlay = map[int]idSet{}
	for id, ex := range it.explicit {
		s.overlay[id] = s.notes[id].poss & ex
	}
	rec := &clueRec{giver: s.me, target: R, focus: it.focus, fresh: it.newCards, free: it.free, seq: s.seq, hint: h}
	ex := it.explicit[it.focus]
	if s.bluffConnects(rec, x) && s.bluffFinesse(rec, ex, x) {
		return 0, nil, -1, false
	}
	targets, assigns := s.bluffTargets(rec, ex, fp, x)
	return targets, assigns, fp, targets != 0
}

// evalBluff（Level 11，給提示）：實際焦點 t 照一般解讀接不上（沒有計畫），但我的下一位 B（我坐在他的 Bluff Seat）的
// finesse position 是現在就能打的 x：給這個提示，B 照 Bob's Truth Principle 以為是 finesse 自己而盲打出 x，大家照 Cathy's
// Connecting Principle 讀成 Bluff，焦點記成差一張就能打的牌——t 要在其中。B 是接收者時是 Self-Bluff，只能用數字提示。
// 價值：x 打出（一張），焦點 t 也被拿到；中間被 prompt 的已提示牌（Bluffs Through Already-Clued Cards）多知道是哪張。
func (s *state) evalBluff(R int, h game.Hint, it interp, touched []int, ev clueEval, dist int) (clueEval, bool) {
	B := (s.me + 1) % s.n
	if !s.knows(lvBluff) || !it.hasTruth || it.planOK || it.tempo || it.anySave || it.distribution || ev.bad > 0 ||
		B == R && h.Kind != game.HintRank || slices.ContainsFunc(it.cands, func(c cand) bool { return c.save }) {
		return ev, false
	}
	fp := s.bluffSlot(nil, B, touched)
	if fp < 0 {
		return ev, false
	}
	c, ok := s.card(fp)
	if !ok {
		return ev, false
	}
	x := identOfCard(c)
	if !s.playableNow(x) || s.gottenAs(s.me, s.me, x, fp) || s.maybeMine(x) || s.awaited(x, fp) || s.pendingLayer(x, fp) ||
		s.plannedPlay(fp) || s.bluffBlocked(nil, B, fp, x) || !s.bluffeeBlindPlays(B, R, it, touched, fp) {
		return ev, false
	}
	// 之前某個還沒結束的提示有解讀要 B 盲打（例如我是那個提示的接收者，看不到焦點，不知道是不是 Layered Finesse 他）：
	// 他可能先打那張，Queued Bluff 不合法（自我對局 4 人局 seed 238：Alice 綠色 Layered Finesse Cathy 的 b1 → g1，
	// 接收者 Bob 以為是直接的 g1，接著用紅色 Bluff Cathy，看得到焦點的人都以為 Cathy 打出 b1 是在接綠色）
	for _, r := range s.recs {
		if s.holder(r.focus) < 0 {
			continue
		}
		onB := func(d dep) bool { return d.player == B && d.finesse }
		if slices.ContainsFunc(r.plan, onB) || slices.ContainsFunc(depsOf(append(slices.Clone(r.cands), r.fallback...)), func(ds []dep) bool {
			return slices.ContainsFunc(ds, onB)
		}) {
			return ev, false
		}
	}
	// 這個提示用掉最後一個提示、B 的下一位又鎖手時，B 會先棄牌換提示給他（Anxiety Play），不盲打
	if C := (B + 1) % s.n; s.hints <= 1 && C != s.me {
		var tc []int
		if C == R {
			tc = append(slices.Clone(touched), it.cm...)
		}
		if s.publicChop(C, tc) < 0 {
			return ev, false
		}
	}
	// 這個提示自己點到焦點那一色、比它小的牌（例如白色同時點到 w1 與 chop 上的 w2）：Bluff 之後焦點記成差一張就能打，
	// 一起點到的那張只知道是有用的白牌，反而打不出去（自我對局 5 人局 seed 196）
	for _, id := range touched {
		if c := identOfCard(s.cards[id]); id != it.focus && c.color() == it.truth.color() && c.rank() < it.truth.rank() {
			return ev, false
		}
	}
	bd := dep{player: B, card: fp, id: x, finesse: true}
	targets, assigns, _, ok := s.bluffRead(R, h, it, touched, []dep{bd})
	t := it.truth
	if !ok || !targets.has(t) {
		return ev, false
	}
	// 中間的牌：被 prompt 的已提示牌要真的是那張（我手上的看不到，不用）；缺的那張和中間的牌都不能可能在我手上
	// （接收者看得到我的牌，會以為它已被拿到或是連接牌，算出別的目標；Certain Discard 也可能把 Bluff 傳回給我）
	for _, d := range assigns[t] {
		if c, ok := s.card(d.card); d.player == s.me || !ok || identOfCard(c) != d.id {
			return ev, false
		}
	}
	for r := max(s.stacks[t.color()], rankIf(x.color() == t.color(), x.rank())) + 1; r < t.rank(); r++ {
		if s.maybeMine(identOf(t.color(), r)) {
			return ev, false
		}
	}
	ev.kind, ev.understood, ev.misleads, ev.danger, ev.costly = kindPlay, true, false, false, false
	ev.gets, ev.plays, ev.plan, ev.nextPlays, ev.bluff = 1, 1, []dep{bd}, true, true
	ev.score = scorePerPlay + scoreNewGood*(1+ev.newGood+len(assigns[t])) + scoreTrashMarked*ev.trashMarked - scoreFinesseClarity
	if it.chopFocus && s.needsSave(R, it.focus) {
		ev.score += scoreChopSaved
	}
	if h.Kind == game.HintColor {
		ev.score += scoreColor
	}
	ev.score -= dist
	return ev, true
}

// rankIf：cond 成立時是 r，否則 0。
func rankIf(cond bool, r game.Rank) game.Rank {
	if cond {
		return r
	}
	return 0
}

// bluffeeBlindPlays（Bob's Truth Principle）：提示後 B 下一手會從 finesse position（fp）盲打。B 不是接收者時，他看得到焦點，
// 從他的角度唯一的連接就是 finesse 自己（看得到別人有連接牌、或可能是自己的已提示牌就不會盲打——Prompt Bluff 不合法）；
// B 是接收者（Self-Bluff）時，他的每種解讀都要先盲打 fp。他以為 fp 是的牌面都要現在就能打（還不能打時他會先等），
// 而且照他自己看得到的牌推不掉（holderView：他看得到別人手上所有剩下的那張時，就知道不是自己；我手上的牌我看不到，
// 這部分只能冒險）。他手上已被拿到、可能是焦點那一色中間某張的牌（照共同筆記或實際牌面）也不行：他私下可能推得出
// 就是那張而當成 prompt（自我對局 4 人局 seed 175：Bob 已提示的 W3 只有他自己確定，W4 的 Bluff 被他讀成 delayed play clue）。
func (s *state) bluffeeBlindPlays(B, R int, it interp, touched []int, fp int) bool {
	var firsts []dep
	if B == R {
		all := append(slices.Clone(it.cands), it.fallback...)
		if len(all) == 0 || it.selfRisk {
			return false
		}
		for _, c := range all {
			if len(c.deps) == 0 {
				return false
			}
			firsts = append(firsts, c.deps[0])
		}
	} else {
		restore := s.saveSim()
		s.pending, s.pendingGiver, s.touchedNow = touched, s.me, touched
		s.overlay = map[int]idSet{}
		for id, ex := range it.explicit {
			s.overlay[id] = s.notes[id].poss & ex
		}
		ds, ok := s.connect(B, s.me, R, it.focus, it.truth)
		restore()
		if !ok || len(ds) == 0 || slices.ContainsFunc(ds[1:], func(d dep) bool { return d.finesse }) {
			return false // 還要別人（例如接收者自己）也盲打時，他不一定先動（自我對局 3 人局 seed 406）
		}
		firsts = ds[:1]
	}
	mine := s.certainMine()
	hv := s.holderView(fp, mine)
	for _, d := range firsts {
		if d.player != B || !d.finesse || d.card != fp || !s.playableNow(d.id) || !hv.has(d.id) || s.allMaybeMine(B, d.id) {
			return false
		}
	}
	t := it.truth
	for _, id := range s.hands[B] {
		if !s.gotten(id) || id == fp {
			continue
		}
		u := s.holderView(id, mine)
		if c, ok := s.card(id); ok {
			u |= setOf(identOfCard(c))
		}
		for _, v := range u.list() {
			if v.color() == t.color() && v.rank() < t.rank() && !s.trash(v) {
				return false
			}
		}
	}
	return true
}

// allMaybeMine：B 看得到的牌之外，m 剩下的幾張很可能全在我手上（我看不到自己的牌）。那樣 B 看得到每一張 m，
// 知道自己沒有，就不會以為被 finesse（自我對局 3 人局 seed 301：Bluff 要 Bob 以為自己有 r2，另一張 r2 在 Alice 手上）。
// 剩下的 m 在 B 手上（他看不到）就沒事；否則它們在我手上或牌堆裡，照張數估全在我手上的機率，超過 10% 就不冒險。
func (s *state) allMaybeMine(B int, m ident) bool {
	left := m.copies() - s.discarded[m]
	for p, h := range s.hands {
		if p == s.me {
			continue
		}
		for _, id := range h {
			if c, ok := s.card(id); ok && identOfCard(c) == m {
				if p == B {
					return false
				}
				left--
			}
		}
	}
	mine := 0
	for _, id := range s.hands[s.me] {
		if s.myPoss(id).has(m) {
			mine++
		}
	}
	if left <= 0 {
		return true
	}
	if left > mine {
		return false
	}
	pr := 1.0
	for i := range left {
		pr *= float64(mine-i) / float64(mine+s.deck-i)
	}
	return pr > 0.1
}
