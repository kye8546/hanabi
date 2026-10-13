package game

import "math/rand/v2"

// newDeck 產生洗好的牌組（一般顏色每色 10 張、黑色 5 張，標準 5 色共 50 張）。牌堆頂端在 slice 尾端。
func newDeck(rng *rand.Rand, va Variant) []Card {
	var deck []Card
	for _, c := range va.Colors() {
		for r := Rank(1); r <= MaxRank; r++ {
			for range Copies(c, r) {
				deck = append(deck, Card{Color: c, Rank: r})
			}
		}
	}
	rng.Shuffle(len(deck), func(i, j int) { deck[i], deck[j] = deck[j], deck[i] })
	// 洗牌後才配 ID，ID 只反映抽牌順序，不洩漏牌面
	for i := range deck {
		deck[i].ID = len(deck) - 1 - i
	}
	return deck
}
