package game

// ActionType 是行動種類。
type ActionType int

const (
	ActPlay ActionType = iota
	ActDiscard
	ActHint
)

// HintKind 是提示種類：顏色或數字。
type HintKind int

const (
	HintColor HintKind = iota
	HintRank
)

// Hint 是一次提示的內容。
type Hint struct {
	Kind  HintKind
	Color Color // Kind == HintColor 時有效
	Rank  Rank  // Kind == HintRank 時有效
}

// Matches 回傳這個提示是否會點到該牌。彩虹色的牌被每種顏色提示點到。
func (h Hint) Matches(c Card) bool {
	if h.Kind == HintColor {
		return c.Color == h.Color || c.Color == Rainbow
	}
	return c.Rank == h.Rank
}

// Action 是玩家在自己回合做的事。Slot 為 0-based 手牌位置（0 = 最左、最新抽到的牌）。
type Action struct {
	Type   ActionType
	Slot   int  // ActPlay / ActDiscard
	Target int  // ActHint：被提示的座位
	Hint   Hint // ActHint
}

func Play(slot int) Action    { return Action{Type: ActPlay, Slot: slot} }
func Discard(slot int) Action { return Action{Type: ActDiscard, Slot: slot} }

func HintColorTo(target int, c Color) Action {
	return Action{Type: ActHint, Target: target, Hint: Hint{Kind: HintColor, Color: c}}
}

func HintRankTo(target int, r Rank) Action {
	return Action{Type: ActHint, Target: target, Hint: Hint{Kind: HintRank, Rank: r}}
}
