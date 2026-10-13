package game

import (
	"fmt"
	"math/rand/v2"
)

const (
	MinPlayers = 2
	MaxPlayers = 5
	MaxHints   = 8 // 提示 token 上限（也是初始值）
	MaxStrikes = 3 // 第 3 道閃電即結束
)

// HandSize 回傳 n 人局的手牌數：2–3 人 5 張、4–5 人 4 張。
func HandSize(n int) int {
	if n <= 3 {
		return 5
	}
	return 4
}

// HandCard 是手上的一張牌，連同持有者對它的認知。
type HandCard struct {
	Card Card
	Know Knowledge
}

// Game 是完整（全知）的遊戲狀態。玩家不應直接讀取，而是透過 View 取得各自的視角。
type Game struct {
	NumPlayers     int
	Variant        Variant      // 變體（決定有哪些顏色、每張牌幾張）
	Hands          [][]HandCard // Hands[座位][位置]，位置 0 = 最左、最新
	Deck           []Card       // 牌堆頂端在尾端
	Fireworks      [NumColors]Rank
	Discards       []Card // 棄牌與打出失敗的牌，依發生順序
	Hints          int
	Strikes        int
	Current        int // 輪到的座位
	Turn           int // 從 1 開始
	FinalTurnsLeft int // 牌堆抽完後剩餘回合數；-1 表示牌堆尚未抽完
	Over           bool
	EndReason      EndReason
	Log            []Event
}

// New 建立一局標準 5 色的新遊戲。相同 seed 會得到相同牌序，方便重現牌局。
func New(numPlayers int, seed uint64) (*Game, error) { return NewVariant(numPlayers, NoVariant, seed) }

// NewVariant 建立一局變體 va 的新遊戲。標準 5 色時牌序和 New 完全相同。
func NewVariant(numPlayers int, va Variant, seed uint64) (*Game, error) {
	if numPlayers < MinPlayers || numPlayers > MaxPlayers {
		return nil, fmt.Errorf("玩家人數必須是 %d–%d 人，收到 %d", MinPlayers, MaxPlayers, numPlayers)
	}
	if va < 0 || int(va) >= len(variantInfo) {
		return nil, fmt.Errorf("沒有這個變體：%d", va)
	}
	rng := rand.New(rand.NewPCG(seed, seed^0x9E3779B97F4A7C15))
	g := &Game{
		NumPlayers:     numPlayers,
		Variant:        va,
		Hands:          make([][]HandCard, numPlayers),
		Deck:           newDeck(rng, va),
		Hints:          MaxHints,
		Turn:           1,
		FinalTurnsLeft: -1,
	}
	for range HandSize(numPlayers) {
		for p := range numPlayers {
			g.draw(p)
		}
	}
	return g, nil
}

// FireworkScore 回傳目前煙火總進度（不考慮閃電規則）。
func (g *Game) FireworkScore() int {
	s := 0
	for _, r := range g.Fireworks {
		s += int(r)
	}
	return s
}

// Score 回傳官方計分：三道閃電結束得 0 分，否則為煙火總進度。
func (g *Game) Score() int {
	if g.Strikes >= MaxStrikes {
		return 0
	}
	return g.FireworkScore()
}

// draw 從牌堆抽一張放到 p 的手牌最左邊。牌堆空時回傳 false。
func (g *Game) draw(p int) (Card, bool) {
	if len(g.Deck) == 0 {
		return Card{}, false
	}
	c := g.Deck[len(g.Deck)-1]
	g.Deck = g.Deck[:len(g.Deck)-1]
	g.Hands[p] = append([]HandCard{{Card: c, Know: NewKnowledge(g.Variant)}}, g.Hands[p]...)
	return c, true
}

// removeCard 取出 p 手牌中 slot 位置的牌。
func (g *Game) removeCard(p, slot int) HandCard {
	hc := g.Hands[p][slot]
	g.Hands[p] = append(g.Hands[p][:slot:slot], g.Hands[p][slot+1:]...)
	return hc
}
