package game

import (
	"errors"
	"fmt"
)

var (
	ErrGameOver        = errors.New("遊戲已經結束")
	ErrBadSlot         = errors.New("手牌位置不存在")
	ErrHintsFull       = errors.New("提示 token 已滿 (8)，不能棄牌")
	ErrNoHints         = errors.New("沒有提示 token 了，不能給提示")
	ErrHintSelf        = errors.New("不能提示自己")
	ErrBadTarget       = errors.New("沒有這個玩家")
	ErrBadHint         = errors.New("提示內容不合法")
	ErrHintTouchesNone = errors.New("提示必須至少點到對方一張牌")
	ErrBadAction       = errors.New("未知的行動種類")
)

// Validate 檢查目前玩家能否執行 a；不合法時回傳說明原因的錯誤。
func (g *Game) Validate(a Action) error {
	if g.Over {
		return ErrGameOver
	}
	switch a.Type {
	case ActPlay, ActDiscard:
		if n := len(g.Hands[g.Current]); a.Slot < 0 || a.Slot >= n {
			return fmt.Errorf("%w（手牌只有 %d 張）", ErrBadSlot, n)
		}
		if a.Type == ActDiscard && g.Hints >= MaxHints {
			return ErrHintsFull
		}
	case ActHint:
		if g.Hints <= 0 {
			return ErrNoHints
		}
		if a.Target < 0 || a.Target >= g.NumPlayers {
			return ErrBadTarget
		}
		if a.Target == g.Current {
			return ErrHintSelf
		}
		if !validHint(a.Hint, g.Variant) {
			return ErrBadHint
		}
		if len(touchedSlots(cardsOf(g.Hands[a.Target]), a.Hint)) == 0 {
			return ErrHintTouchesNone
		}
	default:
		return ErrBadAction
	}
	return nil
}

func validHint(h Hint, va Variant) bool {
	switch h.Kind {
	case HintColor:
		return va.CanClue(h.Color)
	case HintRank:
		return h.Rank >= 1 && h.Rank <= MaxRank
	}
	return false
}

func cardsOf(hand []HandCard) []Card {
	cs := make([]Card, len(hand))
	for i, hc := range hand {
		cs[i] = hc.Card
	}
	return cs
}

// touchedSlots 回傳提示 h 會點到的手牌位置。
func touchedSlots(hand []Card, h Hint) []int {
	var out []int
	for i, c := range hand {
		if h.Matches(c) {
			out = append(out, i)
		}
	}
	return out
}

// Apply 執行目前玩家的行動、推進回合並回傳這一步產生的事件。
// 行動不合法時不改變狀態並回傳錯誤。
func (g *Game) Apply(a Action) ([]Event, error) {
	if err := g.Validate(a); err != nil {
		return nil, err
	}
	p := g.Current
	base := Event{Turn: g.Turn, Player: p}
	var evs []Event

	switch a.Type {
	case ActPlay:
		hc := g.removeCard(p, a.Slot)
		e := base
		e.Card, e.Slot = hc.Card, a.Slot
		if g.Fireworks[hc.Card.Color]+1 == hc.Card.Rank {
			g.Fireworks[hc.Card.Color] = hc.Card.Rank
			e.Type = EvPlay
			if hc.Card.Rank == MaxRank && g.Hints < MaxHints {
				g.Hints++
				e.Bonus = true
			}
		} else {
			g.Discards = append(g.Discards, hc.Card)
			g.Strikes++
			e.Type = EvMisplay
		}
		evs = append(evs, e)
		evs = g.drawFor(p, base, evs)

	case ActDiscard:
		hc := g.removeCard(p, a.Slot)
		g.Discards = append(g.Discards, hc.Card)
		g.Hints++
		e := base
		e.Type, e.Card, e.Slot = EvDiscard, hc.Card, a.Slot
		evs = append(evs, e)
		evs = g.drawFor(p, base, evs)

	case ActHint:
		g.Hints--
		hand := g.Hands[a.Target]
		touched := touchedSlots(cardsOf(hand), a.Hint)
		for i := range hand {
			hand[i].Know.apply(a.Hint, a.Hint.Matches(hand[i].Card))
		}
		e := base
		e.Type, e.Target, e.Hint, e.Touched = EvHint, a.Target, a.Hint, touched
		evs = append(evs, e)
	}

	g.Turn++
	switch {
	case g.Strikes >= MaxStrikes:
		g.end(EndStrikes)
	case g.FireworkScore() == g.Variant.MaxScore():
		g.end(EndPerfect)
	case g.FinalTurnsLeft > 0:
		g.FinalTurnsLeft--
		if g.FinalTurnsLeft == 0 {
			g.end(EndDeckOut)
		}
	case g.FinalTurnsLeft < 0 && len(g.Deck) == 0:
		// 剛抽走最後一張：包含抽牌者在內，每人再各一回合
		g.FinalTurnsLeft = g.NumPlayers
	}

	if g.Over {
		evs = append(evs, Event{Type: EvGameOver, Turn: base.Turn, Player: p, Reason: g.EndReason, Score: g.Score()})
	} else {
		g.Current = (g.Current + 1) % g.NumPlayers
	}
	g.Log = append(g.Log, evs...)
	return evs, nil
}

func (g *Game) drawFor(p int, base Event, evs []Event) []Event {
	if c, ok := g.draw(p); ok {
		e := base
		e.Type, e.Card, e.Slot = EvDraw, c, 0
		evs = append(evs, e)
	}
	return evs
}

func (g *Game) end(r EndReason) {
	g.Over = true
	g.EndReason = r
}
