// Level 7（docs/level-7.mdx）：Emergency Discards——Scream Discard Chop Move（含 With Known-Trash）、Shout Discard Chop Move、
// Generation Discard，以及盲打 Chop Moved 的牌。

package hgroup

import (
	"slices"

	"hanabi/game"
)

// screamRec：有人做了 Scream / Shout Discard。下一位（target）這一手不能棄牌、必須給提示；他行動後，
// 除非是 Generation Discard（他的提示點到再下一位的 chop），他棄牌當時的 chop 被 Chop Move。
type screamRec struct {
	from, target int
	chop         int // target 當時的 chop（只跳過被提示與 Chop Move 的牌，publicChop；-1 = 沒有牌可 Chop Move）
	// gen：當時是不是 Generation Discard 的情況（再下一位 chop 上是必須保留的牌、他又沒事可做）：1 是、0 不是、
	// -1 我就是再下一位、看不到自己的 chop，照被叫的人的回應判斷（generationReply）
	gen int
}

// Scream / Shout Discard 要大家看法一致，「大家都知道能打 / 沒用」只看提示本身（explicit 扣掉已打出、打不出去的牌面）。
// 試過用共同筆記或 publicPoss：共同筆記可能多了持有者不知道的推論、publicPoss 各家的解讀也不完全相同，
// 5 人局每 300 局約 50 次有人以為是 Scream、棄牌的人卻不是這個意思，閃電多一倍。

// screamTrash：大家都知道這張已提示的牌沒用（提示本身只剩已打出 / 打不出去的牌面，或已標成沒用）。
func (s *state) screamTrash(id int) bool {
	n := s.notes[id]
	return n.clued && (n.bad || s.screamIdents(id) == 0)
}

// screamPlayable：大家都知道這張已提示的牌現在能打（提示本身扣掉沒用的牌面後全都能打；Good Touch：已提示的牌不是垃圾）。
// 被 finesse 的盲打牌不算——原文 Scream Discard 的例外：沒打的是 finesse 的盲打牌、而它也可能在下一位的
// finesse position 時不是 Scream Discard（挑戰題 Bob is Forgetful：那是 Ambiguous Finesse，下一位該盲打）。
func (s *state) screamPlayable(id int) bool {
	n := s.notes[id]
	if !n.clued || n.bad {
		return false
	}
	ex := s.screamIdents(id)
	return ex != 0 && !slices.ContainsFunc(ex.list(), func(i ident) bool { return !s.playableNow(i) })
}

// screamPublic：screamIdents 也用大家都推得出的 Good Touch 推論（關掉時只看提示本身扣掉沒用的牌面）。
var screamPublic = true

// screamIdents：大家都推得出的已提示牌 id 的牌面，只用公開資訊（各家算法一致）：
//   - 提示本身扣掉沒用的牌面；
//   - 別的已提示牌依提示本身（反覆套用這條規則後）確定是 i 時扣掉 i（Good Touch）；
//   - 同一個提示點到的幾張都只剩同一個牌面、剩下的張數（扣掉棄牌堆）又不夠時，焦點是它，其他是重複（垃圾）。
//
// 以前只看提示本身（試過用共同筆記或 publicPoss 時各家判斷常常不同）。使用者回報 20261110-154627 t31：紅色點到 AI-2 的
// R1、R4 與「5」點過的 R5（R4 已棄一張），大家都推得出 slot 2 是能打的 R4、slot 1 是垃圾，AI-2 應該 Shout Discard
// 棄掉 slot 1，讓 AI-3 Chop Move chop 上的 W5。
func (s *state) screamIdents(id int) idSet {
	n := s.notes[id]
	if !screamPublic {
		return n.explicit & s.useful()
	}
	if !n.clued || n.bad {
		return 0
	}
	var ids []int
	P := map[int]idSet{}
	for _, h := range s.hands {
		for _, o := range h {
			if s.notes[o].clued && s.live(o) {
				ids = append(ids, o)
				P[o] = s.notes[o].explicit & s.useful()
				// Sarcastic Discard 公開揭露的牌面（使用者回報 20261116-101342 t36：AI-1 Sarcastic Discard G4 之後，大家都知道
				// AI-3 被綠色點到的是 G4，AI-3 該 Scream Discard 救使用者 chop 上的 K2，以前只看提示本身「G4 或 G5」）
				if sd := s.notes[o].sdAs; sd != 0 && P[o]&sd != 0 {
					P[o] &= sd
				}
			}
		}
	}
	for changed := true; changed; {
		changed = false
		for _, o := range ids {
			i, ok := P[o].single()
			if !ok {
				continue
			}
			for _, q := range ids {
				if q != o && P[q].has(i) && P[q].count() > 1 {
					P[q] &^= setOf(i)
					changed = true
				}
			}
		}
		// 同一個提示點到的牌都只剩同一個牌面、只剩一張：焦點以外的是重複
		for _, r := range s.recs {
			f, ok := P[r.focus].single()
			if !ok {
				continue
			}
			var same []int
			for _, q := range r.fresh {
				if q != r.focus && P[q] == setOf(f) {
					same = append(same, q)
				}
			}
			// 剩下不只一張時，其中一張可能就是另一張 f，不知道是哪張，不判定
			if len(same) > 0 && f.copies()-s.discarded[f] == 1 {
				for _, q := range same {
					P[q] = 0
				}
				changed = true
			}
		}
	}
	return P[id]
}

// publicPlayable：p 手上（不含 except）有大家都知道能打的已提示牌。
func (s *state) publicPlayable(p int, except int) bool {
	return slices.ContainsFunc(s.hands[p], func(id int) bool { return id != except && s.screamPlayable(id) })
}

// hasPublicTrash：p 手上（不含 except）有大家都知道沒用的已提示牌。
func (s *state) hasPublicTrash(p int, except int) bool {
	return slices.ContainsFunc(s.hands[p], func(id int) bool { return id != except && s.screamTrash(id) })
}

// detectScream（在棄牌事件處理前呼叫）：P 棄掉 id 是不是 Scream / Shout Discard。
//   - Shout Discard：棄掉大家都知道沒用的已提示牌，手上卻有大家都知道能打的牌（任何提示數）。
//   - Scream Discard：提示 0 個時棄掉沒被提示的牌（chop），手上卻有大家都知道能打的牌，或有大家都知道沒用的
//     已提示牌（With Known-Trash：本來該先棄它）。兩者都有時棄 chop 是已知的失誤（原文：illegal），不算。
//
// 原文另一種情況「提示 1 個、而下一位鎖手」各家難有共識，不採用。Level 8 終局不 Chop Move，也就沒有 Scream / Shout Discard。
func (s *state) detectScream(P, id int) *screamRec {
	if !s.knows(lvScream) || s.endGame() { // Level 8 終局不 Chop Move
		return nil
	}
	n := s.notes[id]
	playable, trash := s.publicPlayable(P, id), s.hasPublicTrash(P, id)
	switch {
	case s.screamTrash(id) && playable:
	case !n.clued && !n.cm && s.hints == 0 && playable != trash:
	default:
		return nil
	}
	N := (P + 1) % s.n
	sc := &screamRec{from: P, target: N, chop: s.publicChop(N, nil)}
	// 原文：「It is not a Scream Discard if the next player has to generate a clue for cluing the next player after that」——
	// 是不是 Generation Discard 看當時的情況，不看被叫的人怎麼回應。看得到再下一位 chop 的人照情況判斷
	// （使用者回報 20261120-150449：t16 AI-3 做 Generation Discard 讓使用者救 AI-1 chop 上的 R1，使用者用「2」點 AI-2 的 R2、
	// finesse AI-1 的 R1，以前因為沒點到 AI-1 的 chop 就當成 Scream，Chop Move 了使用者 chop 上的 W5，t23 的白色焦點因此是 slot 2）
	if P3 := (N + 1) % s.n; s.n >= 3 {
		switch {
		case P3 == s.me:
			sc.gen = -1
		case s.generationSituation(P3):
			sc.gen = 1
		}
	}
	return sc
}

// generationSituation：棄牌時 P3（再下一位）的情況是 Generation Discard 要救的——只用大家（P3 以外）都看得到的資訊，
// 各家判斷才一致（試過用 screamWorthy：裡面有各家私下的推論——finesse、自己手上可能重複的牌、2 看不看得到——
// Chop Move 看法不一致的張數變多）：他的 chop（publicChop）是 5、critical 或現在就能打的牌，而他手上沒有大家都知道
// 能打或沒用的牌（沒事可做，會棄 chop）。
func (s *state) generationSituation(P3 int) bool {
	c := s.publicChop(P3, nil)
	if c < 0 || s.publicPlayable(P3, -1) || s.hasPublicTrash(P3, -1) {
		return false
	}
	t := identOfCard(s.cards[c])
	return !s.trash(t) && (t.rank() == game.MaxRank || s.critical(t) || s.playableNow(t))
}

// screamed：p 這一手是被 Scream / Shout 的人，不能棄牌、必須給提示（只能拖延時可以 5 Stall 或補已提示牌的資訊）。
func (s *state) screamed(p int) bool { return s.scream != nil && s.scream.target == p }

// resolveScream：被 Scream 的人行動了（e），決定 Chop Move。Generation Discard（棄牌只是讓他有提示可以救再下一位的 chop，
// 挑戰題 Too Few Clues）不 Chop Move：看得到再下一位 chop 的人照當時的情況（screamRec.gen），再下一位自己照回應
// （generationReply）。chop 只跳過被提示與 Chop Move 的牌（被 finesse 的牌不是每個人都知道，publicChop）。
// 被叫的人自己判斷不是 Generation Discard 時，就不給再下一位提示（decide 的 misreadAsGeneration），筆記才會一致。
func (s *state) resolveScream(sc *screamRec, e game.Event, p3Chop int, p3Fin bool) {
	if sc.gen == 1 || sc.gen == -1 && s.generationReply(sc, e, p3Chop, p3Fin) {
		return
	}
	if c := sc.chop; c >= 0 && s.holder(c) == sc.target && !s.gotten(c) && !s.notes[c].cm {
		s.chopMove(sc.target, []int{c})
	}
}

// generationReply：被叫的人的行動 e 是回應 Generation Discard——提示再下一位。
// 只有看不到自己 chop 的再下一位照回應判斷（其他人照當時的情況，screamRec.gen）。原文：棄牌「is just for the purposes of
// generating a clue so that player 2 can clue player 3」，而 player 3 有事可做（例如打牌）就不必救——所以任何給我的提示都算回應
// （例如讓我下一手打牌的 play clue；重播 20261109-121725 t32：AI-1 用「3」讓 AI-2 打 Y3，看得到 AI-2 chop 的人當成 Generation
// Discard，只照「點到 chop」判斷的 AI-2 卻以為是 Scream），提示要我盲打 chop 上那張（我的 finesse position 就是 chop）也算。
func (s *state) generationReply(sc *screamRec, e game.Event, p3Chop int, p3Fin bool) bool {
	P3 := (sc.target + 1) % s.n
	if e.Type != game.EvHint || e.Player != sc.target || s.n < 3 || p3Chop < 0 {
		return false
	}
	return e.Target == P3 || !p3Fin && s.holder(p3Chop) == P3 && s.notes[p3Chop].finessed
}

// misreadAsGeneration：我被 Scream 叫到時，這個提示會讓看不到自己 chop 的再下一位讀錯（他照回應判斷，generationReply）：
//   - 不是 Generation Discard 卻提示他：他會以為是回應，不 Chop Move 我的 chop；
//   - 是 Generation Discard 卻不提示他：他會以為是 Scream，把我的 chop 記成被 Chop Move。原文也是要我提示他
//     （「the discard is just for the purposes of generating a clue so that player 2 can clue player 3」）。
func (s *state) misreadAsGeneration(a game.Action) bool {
	if a.Type != game.ActHint || s.n < 3 || s.scream == nil || s.scream.target != s.me || s.scream.gen < 0 {
		return false
	}
	return (a.Target == (s.me+1)%s.n) != (s.scream.gen == 1)
}

// screamWorthy（Level 7）：p 的 chop 值得用 Scream Discard 救——是現在能打或 critical 的牌（或別處看不到的 2），
// 也就是 Save Principle 要保留的牌（needsSave；差一張就能打的牌不行，挑戰題 Another Scream Discard），
// 而他下一手會棄 chop（沒有確定能打的牌、也沒有已知垃圾可棄，我打出能打的牌後也不會有；挑戰題 Screaming into the Void）。
func (s *state) screamWorthy(p int) bool {
	c := s.chop(p)
	if c < 0 || !s.needsSave(p, c) || s.keepsChop(p) || s.holderMayAct(p) {
		return false
	}
	// 我打出能打的牌後他就有事可做：有牌可打，或我打出 5 回收了提示、他有明顯的 play clue 可給（不算給我的，All 4's Test）
	// ——不是最後手段（使用者回報 20261021-165112 t36：AI-2 有 B5，打出後使用者有提示可以讓 AI-1 打 G5，不會棄 chop 上的 Y5）
	return !(p == (s.me+1)%s.n && (s.playLetsNextPlay(p) || s.playRefillsClue() && s.obviousClue(p)))
}

// playRefillsClue：提示 0 個時，我確定能打的牌是 5，打出後回收一個提示。
func (s *state) playRefillsClue() bool {
	slot := s.bestPlay()
	if slot < 0 || s.hints >= game.MaxHints {
		return false
	}
	t, ok := s.myPoss(s.hands[s.me][slot]).single()
	return ok && t.rank() == game.MaxRank
}

// holderMayAct：p 自己可能知道手上有牌可打或有已知垃圾——共同筆記扣掉他看得到所有剩下張數的牌面後（holderView），
// 某張已被拿到的牌只剩能打的、或只剩沒用的牌面（例如共同筆記是「b4 或 b5」，他看得到 b5 在別人手上）。
func (s *state) holderMayAct(p int) bool {
	mine := s.certainMine()
	for _, id := range s.hands[p] {
		if !s.gotten(id) {
			continue
		}
		v := s.holderView(id, mine)
		if v == 0 {
			continue
		}
		list := v.list()
		if !slices.ContainsFunc(list, func(i ident) bool { return !s.playableNow(i) }) ||
			!slices.ContainsFunc(list, func(i ident) bool { return !s.trash(i) }) {
			return true
		}
	}
	return false
}

// myTrashSlot：我手上大家都知道沒用的已提示牌（由左而右第一張——原文 summary：好幾張已知垃圾時照慣例由左而右棄，
// Level 14 的 Trash Order Chop Move 就是跳過它；Level 8 Positional Discard「該棄的牌」也照這個順序）；沒有則回傳 -1。
func (s *state) myTrashSlot() int {
	return slices.IndexFunc(s.hands[s.me], s.screamTrash)
}

// stepEmergencyDiscard（Level 7）：提示 0 個時用棄牌救人（最後手段，Order Chop Move 等不花提示的救法在前面）：
//   - Generation Discard：我打出能打的牌後，再下一位 chop 上必須保留的牌就沒人能救（下一位拿不到提示）：
//     棄牌換一個提示給下一位，讓他救（3 人以上）。下一位的行動會說明是不是這樣，不 Chop Move。
//   - Scream Discard Chop Move：下一位會棄掉 chop 上必須保留的牌：棄 chop（手上有大家都知道沒用的牌時棄它，
//     那是 Shout Discard），讓他 Chop Move 自己的 chop、這一手改給提示。
func (s *state) stepEmergencyDiscard(t *turn) (game.Action, string, bool) {
	if !s.knows(lvScream) || t.v.Hints != 0 || s.endGame() {
		return game.Action{}, "", false
	}
	playable := s.publicPlayable(s.me, -1)
	trash := s.myTrashSlot()
	chop := slices.Index(s.hands[s.me], s.chop(s.me))
	if playable && s.generationWorthy() {
		if trash >= 0 {
			return game.Discard(trash), "Generation Discard：棄牌換提示，讓下一位救再下一位 chop 上必須保留的牌", true
		}
		if chop >= 0 {
			return game.Discard(chop), "Generation Discard：棄牌換提示，讓下一位救再下一位 chop 上必須保留的牌", true
		}
	}
	if !s.screamWorthy((s.me + 1) % s.n) {
		return game.Action{}, "", false
	}
	switch {
	case playable && trash >= 0:
		return game.Discard(trash), "Shout Discard Chop Move：不打能打的牌、棄已知垃圾，要下一位 Chop Move 他 chop 上必須保留的牌", true
	case (playable || trash >= 0) && chop >= 0:
		return game.Discard(chop), "Scream Discard Chop Move：不打能打的牌（或不棄已知垃圾）、改棄 chop，要下一位 Chop Move 他 chop 上必須保留的牌", true
	}
	return game.Action{}, "", false
}

// shoutSave（Level 7 The Shout Discard Chop Move）：我手上有大家都知道能打的牌和沒用的牌時，棄掉沒用的那張就能 Chop Move
// 下一位的 chop（任何提示數都可以），不花提示。回傳要棄的位置；不能時回傳 -1。
func (s *state) shoutSave(hints int) int {
	if !s.knows(lvScream) || hints >= game.MaxHints || !s.publicPlayable(s.me, -1) || s.endGame() {
		return -1
	}
	return s.myTrashSlot()
}

// shoutOverClue：提示不是 0 個時，下一位的 chop 需要救，也用 Shout Discard 代替 save clue。
var shoutOverClue = true

// unintendedScream：我這次棄牌（slot）會被大家當成 Scream / Shout Discard，但我不是要救人（下一位的 chop 不必救，
// 也不是 Generation Discard）。
func (s *state) unintendedScream(slot int) bool {
	if !s.knows(lvScream) {
		return false
	}
	return s.detectScream(s.me, s.hands[s.me][slot]) != nil && !s.screamWorthy((s.me+1)%s.n) && !s.generationWorthy()
}

// screamAvoidPlay：不想做 Scream Discard 時改打的牌：確定能打的牌（不會被當成 Order Chop Move 的優先），或依 Good Touch（已提示的牌不是垃圾）
// 確定能打、大家也都認為能打的已提示牌（例如「r2 或 g2」而 r2 已打出）。沒有則回傳 -1。
func (s *state) screamAvoidPlay() int {
	// 不照順序打 1 會被當成 Order Chop Move，先找不會的
	order := s.playOrder(s.me)
	for _, id := range order {
		if s.ocmSkip(s.me, id) == 0 {
			return slices.Index(s.hands[s.me], id)
		}
	}
	if len(order) > 0 {
		return slices.Index(s.hands[s.me], order[0])
	}
	for slot, id := range s.hands[s.me] {
		if !s.screamPlayable(id) {
			continue
		}
		if p := s.myPoss(id) & s.useful(); p != 0 && !slices.ContainsFunc(p.list(), func(i ident) bool { return !s.playableNow(i) }) {
			return slot
		}
	}
	return -1
}

// generationWorthy：提示 0 個時，Generation Discard 能救再下一位（P3）chop 上必須保留的牌（3 人以上）。只當最後手段：
// 下一位沒牌可打就會棄牌、P3 也就有一個提示可用，P3 有明顯的 play clue 可給時就不會棄 chop（原文：「If player 3 has
// something else to do (like play a card or give some obvious clue), then player 1 should just play」）。
func (s *state) generationWorthy() bool {
	if s.hints != 0 || s.n < 3 {
		return false
	}
	N, P3 := (s.me+1)%s.n, (s.me+2)%s.n
	// 我打出 5 就回收一個提示，下一位直接就能救 P3，不必棄牌換提示；我打出的牌讓 P3 有牌可打（例如他的 delayed play clue
	// 正等我這張）時他也不會棄 chop（使用者回報 20261027-180616 t17：AI-1 有使用者 R3 等著的 R2，卻棄牌換提示）
	return !s.playRefillsClue() && s.screamWorthy(P3) && !s.playLetsNextPlay(P3) && (s.willPlay(N) || !s.obviousClue(P3))
}

// obviousClue：p 有明顯的 play clue 可給——我看得到的其他玩家（不含 p 與我）有沒被提示、現在就能打、別處沒被拿到，
// 而且有不違反 Good Touch 的提示焦點就是它的牌（cleanClueFor）。
func (s *state) obviousClue(p int) bool {
	for q := range s.n {
		if q == s.me || q == p {
			continue
		}
		for _, id := range s.hands[q] {
			if t := identOfCard(s.cards[id]); !s.gotten(id) && s.playableNow(t) && !s.gottenAs(s.me, s.me, t, id) && s.cleanClueFor(q, id) {
				return true
			}
		}
	}
	return false
}

// stepPlayChopMoved（Level 7 Blind-Playing Chop Moved Cards）：Chop Moved 的牌累積了足夠的資訊、很可能能打時，
// 不等別人直接提示，賭它能打。只在還能承受閃電時（原文：0 個閃電可以、1 個也許、2 個絕不），終局提示又不多時才賭
// （遊戲順利時照常棄牌、等別人提示）。它可能是打不出去的 critical 牌時絕不打。
func (s *state) stepPlayChopMoved(t *turn) (game.Action, string, bool) {
	if !s.knows(lvCMBlindPlay) || t.v.Strikes >= game.MaxStrikes-1 || t.v.Hints > cmBlindHints || t.v.DeckSize > cmBlindDeck {
		return game.Action{}, "", false
	}
	need := cmBlindOdds[t.v.Strikes]
	bestSlot, bestP := -1, need
	for slot, id := range s.hands[s.me] {
		if n := s.notes[id]; !n.cm || n.clued {
			continue
		}
		poss := s.myPoss(id)
		// Chop Move 保護的多半是有用的牌（原文：「more likely to be a useful card than not」）
		if u := poss & s.useful(); u != 0 {
			poss = u
		}
		ok, all := 0, 0
		risky := false
		for _, i := range poss.list() {
			k := max(s.remaining(i), 0)
			all += k
			switch {
			case s.playableNow(i):
				ok += k
			case k > 0 && s.critical(i):
				risky = true
			}
		}
		if risky || all == 0 {
			continue
		}
		if p := float64(ok) / float64(all); p >= bestP {
			bestSlot, bestP = slot, p
		}
	}
	if bestSlot < 0 {
		return game.Action{}, "", false
	}
	return game.Play(bestSlot), "盲打 Chop Moved 的牌：很可能能打，還能承受閃電", true
}

// 盲打 Chop Moved 的牌的門檻：牌堆不超過 cmBlindDeck 張、提示不超過 cmBlindHints 個時，
// 能打的機率至少 cmBlindOdds[閃電數]。
var (
	cmBlindDeck  = 10
	cmBlindHints = 2
	cmBlindOdds  = [game.MaxStrikes]float64{0.75, 1, 2}
)
