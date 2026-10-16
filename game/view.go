package game

// VisibleCard 是某個玩家眼中的一張手牌。Visible 為 false 時 Card 只有 ID 有效。
type VisibleCard struct {
	Card    Card
	Visible bool
	Know    Knowledge // 持有者本人對這張牌的認知（所有人都看得到）
}

// PlayerView 是某個座位能合法看到的全部資訊。玩家（真人或 AI）只會拿到這個。
type PlayerView struct {
	Seat           int
	NumPlayers     int
	Variant        Variant // 變體
	Current        int
	Turn           int
	Hands          [][]VisibleCard
	Fireworks      [NumColors]Rank
	Discards       []Card
	Hints          int
	Strikes        int
	DeckSize       int
	FinalTurnsLeft int
	Over           bool
	EndReason      EndReason
	Log            []Event
}

// View 產生 seat 的視角：自己的手牌與自己抽到的牌都會被遮蔽。
func (g *Game) View(seat int) PlayerView {
	v := PlayerView{
		Seat:           seat,
		NumPlayers:     g.NumPlayers,
		Variant:        g.Variant,
		Current:        g.Current,
		Turn:           g.Turn,
		Hands:          make([][]VisibleCard, g.NumPlayers),
		Fireworks:      g.Fireworks,
		Discards:       append([]Card(nil), g.Discards...),
		Hints:          g.Hints,
		Strikes:        g.Strikes,
		DeckSize:       len(g.Deck),
		FinalTurnsLeft: g.FinalTurnsLeft,
		Over:           g.Over,
		EndReason:      g.EndReason,
		Log:            make([]Event, len(g.Log)),
	}
	for p, hand := range g.Hands {
		vh := make([]VisibleCard, len(hand))
		for i, hc := range hand {
			vh[i] = VisibleCard{Card: hc.Card, Visible: p != seat, Know: hc.Know}
			if p == seat {
				vh[i].Card = Card{ID: hc.Card.ID}
			}
		}
		v.Hands[p] = vh
	}
	for i, e := range g.Log {
		if e.Type == EvDraw && e.Player == seat {
			e.Card = Card{ID: e.Card.ID}
		}
		v.Log[i] = e
	}
	return v
}

// MyHand 回傳自己的（被遮蔽的）手牌。
func (v PlayerView) MyHand() []VisibleCard { return v.Hands[v.Seat] }

// Score 回傳目前煙火總進度。
func (v PlayerView) Score() int {
	s := 0
	for _, r := range v.Fireworks {
		s += int(r)
	}
	return s
}

// Colors 回傳這局用到的顏色。
func (v PlayerView) Colors() []Color { return v.Variant.Colors() }

// MaxScore 回傳這局的滿分。
func (v PlayerView) MaxScore() int { return v.Variant.MaxScore() }

// CanDiscard 回傳目前能否棄牌。
func (v PlayerView) CanDiscard() bool { return v.Hints < MaxHints }

// LegalHints 列出目前所有合法的提示行動（依座位、顏色、數字順序）。
func (v PlayerView) LegalHints() []Action {
	if v.Hints <= 0 {
		return nil
	}
	var out []Action
	for i := 1; i < v.NumPlayers; i++ {
		t := (v.Seat + i) % v.NumPlayers
		hand := make([]Card, len(v.Hands[t]))
		for j, vc := range v.Hands[t] {
			hand[j] = vc.Card
		}
		for _, c := range v.Variant.ClueColors() {
			if a := HintColorTo(t, c); len(touchedSlots(hand, a.Hint)) > 0 {
				out = append(out, a)
			}
		}
		for r := Rank(1); r <= MaxRank; r++ {
			if a := HintRankTo(t, r); len(touchedSlots(hand, a.Hint)) > 0 {
				out = append(out, a)
			}
		}
	}
	return out
}
