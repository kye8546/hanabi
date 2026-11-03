package hgroup

// Level 10（Special Discards）：Gentleman's Discard（含 Layered）、Baton Discard、Sarcastic Finesse、
// Certain Finesse / Certain Discard（Composition Finesse 是兩者同時成立，不必另外處理）。
// Sarcastic Discard（Level 3）在 level3.go；給 Sarcastic / Certain Finesse 時的 Good Touch 例外在 evalclue.go。

import (
	"slices"

	"hanabi/game"
)

// gdRec：Gentleman's Discard 之後，被傳到的人該盲打、還沒打出的牌。盲打出別張能打的牌時往右移
// （Layered Gentleman's Discard）。
type gdRec struct {
	player, card int
	id           ident
	seq          int  // 棄牌時的事件序號（之後才抽的牌不在 layer 裡）
	giver        int  // 棄牌的人
	guessed      bool // 我看不到目標、推定是我自己
}

// gdSlot：p 的 finesse position（最左邊沒被拿到、也沒被 Chop Move 的牌）；沒有則回傳 -1。
func (s *state) gdSlot(p int) int {
	for _, id := range s.hands[p] {
		if !s.gotten(id) && !s.notes[id].cm {
			return id
		}
	}
	return -1
}

// gdTarget（obs 角度）：D 棄掉被拿到的 t 是 Gentleman's / Baton Discard 時，傳給誰的哪張牌。從 D 的下一位起照座位順序，
// 找 obs 看得到 finesse position 正好是 t 的玩家；t 現在能打時（Gentleman's Discard）也可以在幾張能打的 layer 後面
// （Layered Gentleman's Discard；Baton Discard 必須正好在 finesse position：「it is illegal to perform a Layered Baton Discard」），
// 正好在 finesse position 的優先。都看不到、而 obs 看不到全部剩下的 t 時，就是 obs 自己的 finesse position。
// 回傳座位、他會先盲打的牌（finesse position）與 layer 張數；找不到時座位是 -1。
func (s *state) gdTarget(obs, D int, t ident, discarded int) (p, card, layers int) {
	for _, layered := range []bool{false, true} {
		if layered && !s.playableNow(t) {
			break
		}
		for k := 1; k < s.n; k++ {
			q := (D + k) % s.n
			if q == obs {
				continue
			}
			var seen []ident
			first := -1
			for _, id := range s.hands[q] {
				if s.gotten(id) || s.notes[id].cm {
					continue
				}
				if first < 0 {
					first = id
				}
				u, ok := s.see(obs, id)
				if !ok {
					break
				}
				if u == t {
					return q, first, len(seen)
				}
				// 不像 Layered Finesse 那樣排除「別處已被拿到的牌」當 layer：那要看各人看得到的牌（棄牌的人看不到自己手上的），
				// 讀法會不一致（自我對局 Rainbow 5 人局 seed 130：layer W3 和棄牌者自己不知道的 W3 重複，其他人都找不到目標、
				// 以為是自己而連續三人盲打錯）
				if !layered || !s.playableNow(u) || slices.Contains(seen, u) {
					break
				}
				seen = append(seen, u)
			}
		}
	}
	if obs == D {
		return -1, -1, 0
	}
	left := t.copies() - s.discarded[t] - 1
	for _, h := range s.hands {
		for _, id := range h {
			if u, ok := s.see(obs, id); ok && u == t && id != discarded {
				left--
			}
		}
	}
	if left <= 0 {
		return -1, -1, 0
	}
	return obs, s.gdSlot(obs), 0
}

// gentleman（Level 10 The Gentleman's Discard / The Baton Discard）：棄掉一張被拿到、還有用的牌 t，而別人手上沒有已被拿到、
// 可能是 t 的牌（那是 Sarcastic Discard，優先；呼叫前 sarcastic 已確認）：t 傳到另一張 t 所在的 finesse position。
// t 現在能打就是 Gentleman's Discard，持有者馬上盲打（可以有 layer）；還不能打就是 Baton Discard，那張視同被提示、確定是 t
// （「All players should write a note of exactly the card transferred」）。只轉移和棄掉的牌面一樣的那一張——就算看起來
// 棄牌的人不可能知道它是哪張（Double Gentleman's Discard 不合法）。剩下的 t 都已被棄掉時不是。
func (s *state) gentleman(e game.Event) bool {
	t := identOfCard(e.Card)
	if !s.knows(lvGentleman) || t.copies()-s.discarded[t] < 2 || s.notes[e.Card.ID].poss != setOf(t) {
		// 只在大家都看得出他知道那張就是 t 時才讀成 Gentleman's / Baton Discard。看不到目標的人都會以為是自己，
		// 棄牌的人其實是失誤（例如私下以為那張是垃圾）時，好幾個人會同時盲打錯（自我對局 Rainbow 5 人局 seed 393：
		// 能打的 R4 被當成已知垃圾棄掉，其他三人都以為傳給自己）。原文 Double Gentleman's Discard 那種「他私下推得出」的
		// 情況就不讀，只是少打一張，不會打錯
		return false
	}
	// 他自己手上還有可能是 t 的已提示牌：他只是棄掉自己手上重複的那張（自我對局 Rainbow 5 人局 seed 376：
	// 兩張都被記成 G2，他棄掉一張，被讀成傳給 P4 的 Baton Discard）
	if s.pendingAs(t) || slices.ContainsFunc(s.hands[e.Player], func(id int) bool {
		return id != e.Card.ID && s.live(id) && s.notes[id].poss.has(t)
	}) {
		return false
	}
	// 我手上有提示資訊上可能是 t 的已提示牌：棄牌的人看得到它，多半是對我 Sarcastic Discard
	// （我自己的推論排除了 t 不代表他知道），不盲打 finesse position
	if e.Player != s.me && slices.ContainsFunc(s.hands[s.me], func(id int) bool { return s.live(id) && s.notes[id].explicit.has(t) }) {
		return false
	}
	p, card, _ := s.gdTarget(s.me, e.Player, t, e.Card.ID)
	if p < 0 || card < 0 {
		return false
	}
	n := s.notes[card]
	n.poss = setOf(t)
	s.got(card, e.Player)
	n.finessed, n.willPlay, n.finRec = true, true, nil
	if s.playableNow(t) || p == s.me {
		s.gds = append(s.gds, gdRec{player: p, card: card, id: t, seq: s.seq, giver: e.Player, guessed: p == s.me})
	}
	return true
}

// pendingAs：還有提示的某種解讀要某張牌（prompt / finesse）當 t 打出。棄掉 t 的人可能是照那個解讀認為 t 已經有人會打
// （對他是 Sarcastic Discard 或棄重複的牌），各人採用的解讀不一定相同，不能當成 Gentleman's / Baton Discard
// （自我對局 Rainbow 5 人局 seed 376：G2 被當成已知垃圾棄掉，被讀成傳給 P4 的 Baton Discard，G1 打出後他盲打錯）。
func (s *state) pendingAs(t ident) bool {
	for _, rec := range s.recs {
		for _, ds := range append(append([][]dep{rec.plan}, depsOf(rec.cands)...), depsOf(rec.fallback)...) {
			if slices.ContainsFunc(ds, func(d dep) bool { return d.id == t && s.holder(d.card) >= 0 }) {
				return true
			}
		}
	}
	return false
}

// gdCard：這張牌是 Gentleman's Discard 要持有者盲打的牌（checkDeps 不拿它的盲打當其他提示的證據）。
func (s *state) gdCard(id int) bool {
	return slices.ContainsFunc(s.gds, func(g gdRec) bool { return g.card == id })
}

// stepGD：Gentleman's Discard 的持有者打出了該盲打的牌。是 t 就完成；是別張能打的牌就是 layer，承諾移到下一張
// （原文 Layered Gentleman's Discard：「Alice knows that she is promised the red 1, so she will blind-play her slot 2」）；
// 打錯或棄掉就作廢。t 已經被打出（例如 1 的另一張）也作廢。
func (s *state) stepGD(e game.Event) {
	// 我推定自己是目標，卻有別人（不是棄牌的人）先盲打了沒被拿到的牌：他也以為是自己，表示棄牌的人多半是失誤
	// （看不到目標的人都會以為是自己），我就不盲打了，免得連續打錯（自我對局 Rainbow 5 人局 seed 393）
	blind := (e.Type == game.EvPlay || e.Type == game.EvMisplay) && e.Player != s.me && !s.gotten(e.Card.ID)
	s.gds = slices.DeleteFunc(s.gds, func(g gdRec) bool {
		if g.guessed && blind && e.Player != g.giver && slices.Contains(s.hands[g.player], g.card) {
			s.resetNote(g.card)
			return true
		}
		return s.trash(g.id) || !slices.Contains(s.hands[g.player], g.card)
	})
	for i := range s.gds {
		g := &s.gds[i]
		if g.card != e.Card.ID {
			continue
		}
		if e.Type != game.EvPlay || identOfCard(e.Card) == g.id {
			g.card = -1
			continue
		}
		next := -1
		for _, id := range s.hands[g.player] {
			if id != e.Card.ID && !s.gotten(id) && !s.notes[id].cm && s.notes[id].drawn <= g.seq {
				next = id
				break
			}
		}
		g.card = next
		if next >= 0 {
			n := s.notes[next]
			n.poss = setOf(g.id)
			s.got(next, g.player)
			n.finessed, n.willPlay, n.finRec = true, true, nil
		}
	}
	s.gds = slices.DeleteFunc(s.gds, func(g gdRec) bool { return g.card < 0 })
}

// certainDiscard（Level 10 The Certain Finesse & The Certain Discard）：被 finesse 要盲打 t 的人把那張棄掉了——
// 他看到給提示者手上不確定的已提示牌正好是 t，把它傳回去（給提示者不會 Layered Finesse 可能和自己已提示的牌重複的牌，
// 所以他確定那張就是 t：「everyone can be certain that they can safely blind-discard a card to pass it back」）。
// 那個連接改成 prompt 給提示者那張（可能是 t 的已提示牌，由左而右第一張），只留需要它的解讀。回傳是不是 Certain Discard。
func (s *state) certainDiscard(e game.Event) bool {
	if !s.knows(lvCertain) || e.Type != game.EvDiscard {
		return false
	}
	t := identOfCard(e.Card)
	// 他棄的是 chop、我也沒把它記成被 finesse：多半只是一般的棄牌（只剩一張沒被提示的牌時 finesse position 就是 chop）；
	// 終局棄 chop 以外的牌是 Positional Discard
	if n := s.notes[e.Card.ID]; !n.finessed && (e.Card.ID == s.chop(e.Player) || s.endGame()) {
		return false
	}
	match := func(d dep) bool { return d.player == e.Player && d.card == e.Card.ID && d.finesse && d.id == t }
	found := false
	for _, rec := range s.recs {
		if rec.giver == e.Player {
			continue
		}
		// 提示資訊上可能是 t 的（共同筆記多半已依 Good Touch 排除了 t：要盲打的那張被記成被 finesse 的 t）
		y := slices.IndexFunc(s.hands[rec.giver], func(id int) bool { return s.live(id) && s.notes[id].explicit.has(t) })
		if y < 0 {
			continue
		}
		y = s.hands[rec.giver][y]
		to := dep{player: rec.giver, card: y, id: t}
		conv := func(cs []cand) []cand {
			var out []cand
			for _, c := range cs {
				if i := slices.IndexFunc(c.deps, match); i >= 0 {
					c.deps = slices.Clone(c.deps)
					c.deps[i] = to
					out = append(out, c)
				}
			}
			return out
		}
		kept, fb := conv(rec.cands), conv(rec.fallback)
		inPlan := slices.IndexFunc(rec.plan, match)
		if len(kept) == 0 && len(fb) == 0 && inPlan < 0 {
			continue
		}
		found = true
		switch {
		case len(kept) > 0:
			rec.cands, rec.fallback = kept, fb
		case len(fb) > 0:
			rec.cands, rec.fallback = fb, nil
		}
		if inPlan >= 0 {
			rec.plan = slices.Clone(rec.plan)
			rec.plan[inPlan] = to
		}
		n := s.notes[y]
		n.poss, n.willPlay = setOf(t), true
		rec.changed = append(rec.changed, y)
		s.refreshFocus(rec)
	}
	return found
}

// certainSlot（Level 10 The Certain Discard）：我被 finesse 要盲打的牌是 t，而給提示者手上不確定的已提示牌正好是 t：
// 棄掉我這張（傳回給他，他就知道自己那張是 t），不盲打——否則他那張成了他不知道的重複牌。提示滿 8 不能棄牌。
func (s *state) certainSlot(v game.PlayerView) int {
	if !s.knows(lvCertain) || v.Hints >= game.MaxHints {
		return -1
	}
	for slot, id := range s.hands[s.me] {
		n := s.notes[id]
		t, ok := s.myPoss(id).single()
		if !n.finessed || n.clued || n.finRec == nil || n.finRec.giver == s.me || !ok || s.trash(t) {
			continue
		}
		for _, y := range s.hands[n.finRec.giver] {
			c, seen := s.card(y)
			// 他的筆記多半已依 Good Touch 排除了 t（我這張被記成被 finesse 的 t），看提示資訊
			if n := s.notes[y]; seen && s.live(y) && identOfCard(c) == t && n.explicit.has(t) && n.poss != setOf(t) {
				return slot
			}
		}
	}
	return -1
}

// dupFinesseOK（Level 10 The Sarcastic Finesse / The Certain Finesse）：提示要打出的牌 c（焦點，或被 finesse 的連接牌）
// 可能和我手上不確定的已提示牌重複，照樣可以給：最好的情況是真的 finesse，最差也是打出一張、我的牌被 Sarcastic / Certain
// Discard 告知是什麼。我手上可能是 c 的已提示牌只能有一張，而且大家都不確定它是 c（「the duplicated card in your own hand must
// be unknown. Otherwise, the clue receiver will simply think that the clued card is the next card in the suit」）。
func (s *state) dupFinesseOK(c ident) bool {
	if !s.knows(lvCertain) {
		return false
	}
	k := 0
	for _, id := range s.hands[s.me] {
		if !s.gotten(id) || !s.myPoss(id).has(c) {
			continue
		}
		if s.myPoss(id) == setOf(c) || s.notes[id].poss == setOf(c) || s.notes[id].bad {
			return false
		}
		k++
	}
	return k == 1
}
