package web

import (
	"strconv"

	"hanabi/ai/hgroup"
	"hanabi/game"
)

// snapshot 是送給某個座位的完整畫面（JSON）：只由 PlayerView 產生，自己的手牌照樣被遮蔽。
type snapshot struct {
	Seat         int          `json:"seat"`
	Names        []string     `json:"names"`
	Level        int          `json:"level"`
	LevelName    string       `json:"levelName"`
	Variant      string       `json:"variant"`
	VariantName  string       `json:"variantName"`
	Colors       []int        `json:"colors"`     // 這局的顏色（game.Color）
	ClueColors   []int        `json:"clueColors"` // 能拿來提示的顏色
	Seed         string       `json:"seed"`       // uint64 用字串，JavaScript 的數字放不下
	Turn         int          `json:"turn"`
	Current      int          `json:"current"`
	Hints        int          `json:"hints"`
	Strikes      int          `json:"strikes"`
	Deck         int          `json:"deck"`
	FinalTurns   int          `json:"finalTurns"`
	Over         bool         `json:"over"`
	EndReason    int          `json:"endReason"`
	Score        int          `json:"score"`
	MaxScore     int          `json:"maxScore"`
	MaxReachable int          `json:"maxReachable"`
	Pace         *int         `json:"pace"`      // 牌堆空了是 null
	Fireworks    []int        `json:"fireworks"` // 和 Colors 對齊
	Discards     []cardJSON   `json:"discards"`
	Hands        [][]handJSON `json:"hands"`
	Log          []eventJSON  `json:"log"`
	YourTurn     bool         `json:"yourTurn"`
	HintOptions  []hintJSON   `json:"hintOptions"` // 所有會點到牌的提示（不管現在有沒有提示 token）
	Error        string       `json:"error,omitempty"`
}

type cardJSON struct {
	ID    int `json:"id"`
	Color int `json:"color"` // -1 = 看不到
	Rank  int `json:"rank"`  // 0 = 看不到
}

type handJSON struct {
	cardJSON
	Clued bool  `json:"clued"`
	PC    []int `json:"pc"` // 持有者認為還可能的顏色（含反向資訊）
	PR    []int `json:"pr"` // 還可能的數字
	HC    int   `json:"hc"` // 正向提示點到的顏色（兩種以上就是彩虹色），-1 = 沒有
	HR    int   `json:"hr"` // 正向提示點到的數字，0 = 沒有
}

type hintJSON struct {
	Target  int    `json:"target"`
	Kind    string `json:"kind"` // "color" / "rank"
	Color   int    `json:"color"`
	Rank    int    `json:"rank"`
	Touched []int  `json:"touched"` // 0-based 位置
}

type eventJSON struct {
	Type    string    `json:"type"` // play / misplay / discard / hint / draw / over
	Turn    int       `json:"turn"`
	Player  int       `json:"player"`
	Card    *cardJSON `json:"card,omitempty"`
	Slot    int       `json:"slot"`
	Bonus   bool      `json:"bonus,omitempty"`
	Target  int       `json:"target"`
	Hint    *hintJSON `json:"hint,omitempty"`
	Touched []int     `json:"touched,omitempty"`
	Reason  int       `json:"reason,omitempty"`
	Score   int       `json:"score,omitempty"`
}

var eventNames = map[game.EventType]string{
	game.EvPlay: "play", game.EvMisplay: "misplay", game.EvDiscard: "discard",
	game.EvHint: "hint", game.EvDraw: "draw", game.EvGameOver: "over",
}

func colorInts(cs []game.Color) []int {
	out := make([]int, len(cs))
	for i, c := range cs {
		out[i] = int(c)
	}
	return out
}

func toCard(c game.Card, visible bool) cardJSON {
	if !visible {
		return cardJSON{ID: c.ID, Color: -1}
	}
	return cardJSON{ID: c.ID, Color: int(c.Color), Rank: int(c.Rank)}
}

func toHint(h game.Hint) hintJSON {
	if h.Kind == game.HintColor {
		return hintJSON{Kind: "color", Color: int(h.Color)}
	}
	return hintJSON{Kind: "rank", Color: -1, Rank: int(h.Rank)}
}

// buildSnapshot 把 v 轉成畫面；yourTurn 是「真人現在可以下指令」。
func buildSnapshot(v game.PlayerView, names []string, level int, seed uint64, yourTurn bool) snapshot {
	s := snapshot{
		Seat:         v.Seat,
		Names:        names,
		Level:        level,
		LevelName:    hgroup.Levels[level-1],
		Variant:      v.Variant.Code(),
		VariantName:  v.Variant.Name(),
		Colors:       colorInts(v.Colors()),
		ClueColors:   colorInts(v.Variant.ClueColors()),
		Seed:         strconv.FormatUint(seed, 10),
		Turn:         v.Turn,
		Current:      v.Current,
		Hints:        v.Hints,
		Strikes:      v.Strikes,
		Deck:         v.DeckSize,
		FinalTurns:   v.FinalTurnsLeft,
		Over:         v.Over,
		EndReason:    int(v.EndReason),
		Score:        v.Score(),
		MaxScore:     v.MaxScore(),
		MaxReachable: v.MaxReachable(),
		YourTurn:     yourTurn && !v.Over && v.Current == v.Seat,
		Discards:     []cardJSON{},
		Log:          []eventJSON{},
		HintOptions:  []hintJSON{},
	}
	if p, ok := v.Pace(); ok {
		s.Pace = &p
	}
	for _, c := range v.Colors() {
		s.Fireworks = append(s.Fireworks, int(v.Fireworks[c]))
	}
	for _, c := range v.Discards {
		s.Discards = append(s.Discards, toCard(c, true))
	}
	for _, hand := range v.Hands {
		var hs []handJSON
		for _, vc := range hand {
			k := vc.Know
			h := handJSON{cardJSON: toCard(vc.Card, vc.Visible), Clued: k.Clued, HC: -1}
			h.PC = colorInts(k.PossibleColors())
			for _, r := range k.PossibleRanks() {
				h.PR = append(h.PR, int(r))
			}
			if c, ok := k.HintedColor(); ok {
				h.HC = int(c)
			}
			if r, ok := k.HintedRank(); ok {
				h.HR = int(r)
			}
			hs = append(hs, h)
		}
		s.Hands = append(s.Hands, hs)
	}
	for _, e := range v.Log {
		ej := eventJSON{Type: eventNames[e.Type], Turn: e.Turn, Player: e.Player, Slot: e.Slot, Bonus: e.Bonus,
			Target: e.Target, Touched: e.Touched, Reason: int(e.Reason), Score: e.Score}
		switch e.Type {
		case game.EvPlay, game.EvMisplay, game.EvDiscard:
			c := toCard(e.Card, true)
			ej.Card = &c
		case game.EvHint:
			h := toHint(e.Hint)
			ej.Hint = &h
		case game.EvDraw:
			continue // 抽牌不顯示（自己抽到什麼本來就看不到）
		}
		s.Log = append(s.Log, ej)
	}
	// 提示選項：照 Hint.Matches 算點到哪些位置（彩虹色的牌被每種顏色點到），畫面不必自己重寫規則
	for i := 1; i < v.NumPlayers; i++ {
		t := (v.Seat + i) % v.NumPlayers
		var hints []game.Hint
		for _, c := range v.Variant.ClueColors() {
			hints = append(hints, game.Hint{Kind: game.HintColor, Color: c})
		}
		for r := game.Rank(1); r <= game.MaxRank; r++ {
			hints = append(hints, game.Hint{Kind: game.HintRank, Rank: r})
		}
		for _, h := range hints {
			var touched []int
			for j, vc := range v.Hands[t] {
				if h.Matches(vc.Card) {
					touched = append(touched, j)
				}
			}
			if len(touched) > 0 {
				hj := toHint(h)
				hj.Target, hj.Touched = t, touched
				s.HintOptions = append(s.HintOptions, hj)
			}
		}
	}
	return s
}
