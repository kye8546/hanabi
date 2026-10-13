// Package game 實作花火 (Hanabi) 的純規則引擎。
//
// 這個套件不做任何 I/O：只接受 Action、產生 Event。顯示層（console、之後的 GUI）
// 與玩家（真人、AI）都在套件外，透過 PlayerView 取得各自能看到的資訊。
package game

import (
	"slices"
	"strconv"
	"strings"
)

// Color 是牌的顏色。
type Color int

const (
	Red Color = iota
	Yellow
	Green
	Blue
	White
	Teal  // 只在 6 Suits 變體出現
	Black // 只在 Black (6 Suits) 變體出現；每個數字只有 1 張
)

// NumColors 是所有變體中的顏色總數（陣列大小用）；一局實際用哪些顏色看 Variant.Colors。
const NumColors = 7

// AllColors 依固定順序列出所有顏色，方便迭代與顯示；一局實際用的是 Variant.Colors。
var AllColors = [NumColors]Color{Red, Yellow, Green, Blue, White, Teal, Black}

var colorLetters = [NumColors]string{"R", "Y", "G", "B", "W", "T", "K"}
var colorNames = [NumColors]string{"紅", "黃", "綠", "藍", "白", "青", "黑"}

// Variant 是 Hanab Live 的變體（https://github.com/Hanabi-Live/hanabi-live/blob/main/docs/variants.md）。
// Hanab Live 的第五色是紫色，這裡沿用實體版的白色。
type Variant int

const (
	NoVariant Variant = iota // 標準 5 色
	SixSuits                 // 6 Suits：多一個青色
	BlackSix                 // Black (6 Suits)：多一個黑色，每個數字只有 1 張（全都是 critical）
)

// Variants 列出所有變體（開局選單的順序）。
var Variants = []Variant{NoVariant, SixSuits, BlackSix}

var variantInfo = []struct {
	name, code string
	colors     []Color
}{
	NoVariant: {"No Variant", "none", AllColors[:5]},
	SixSuits:  {"6 Suits", "6suits", AllColors[:6]},
	BlackSix:  {"Black (6 Suits)", "black6", []Color{Red, Yellow, Green, Blue, White, Black}},
}

// Colors 回傳這個變體用到的顏色。
func (v Variant) Colors() []Color { return variantInfo[v].colors }

// HasColor 回傳這個變體有沒有顏色 c。
func (v Variant) HasColor(c Color) bool { return slices.Contains(v.Colors(), c) }

// MaxScore 回傳這個變體的滿分。
func (v Variant) MaxScore() int { return len(v.Colors()) * int(MaxRank) }

// Name 回傳 Hanab Live 的變體名稱。
func (v Variant) Name() string { return variantInfo[v].name }

// Code 回傳對局紀錄用的代碼（ParseVariant 的反向）。
func (v Variant) Code() string { return variantInfo[v].code }

// ParseVariant 由對局紀錄的代碼找回變體。
func ParseVariant(code string) (Variant, bool) {
	for _, v := range Variants {
		if v.Code() == code {
			return v, true
		}
	}
	return 0, false
}

// Letter 回傳單一英文字母代號 (R/Y/G/B/W/T/K)。
func (c Color) Letter() string { return colorLetters[c] }

// Name 回傳中文名稱。
func (c Color) Name() string { return colorNames[c] }

// ParseColor 接受英文字母（大小寫皆可）或中文顏色名。
func ParseColor(s string) (Color, bool) {
	u := strings.ToUpper(s)
	for _, c := range AllColors {
		if u == c.Letter() || s == c.Name() {
			return c, true
		}
	}
	return 0, false
}

// Rank 是牌的數字 1..5；0 表示未知（只會出現在被遮蔽的牌上）。
type Rank int

// MaxRank 是最大數字。
const MaxRank Rank = 5

// rankCopies[r] 是一般顏色中數字 r 的張數。
var rankCopies = [MaxRank + 1]int{0, 3, 2, 2, 2, 1}

// Copies 回傳顏色 c 數字 r 的張數：一般顏色 1→3、2/3/4→2、5→1；黑色每個數字 1 張。
func Copies(c Color, r Rank) int {
	if c == Black {
		return 1
	}
	return rankCopies[r]
}

// Card 是一張牌。ID 在洗牌後依牌堆位置配發，供 UI 追蹤同一張牌，不洩漏牌面。
type Card struct {
	ID    int
	Color Color
	Rank  Rank
}

// String 回傳像 "R3" 的簡寫。
func (c Card) String() string { return c.Color.Letter() + strconv.Itoa(int(c.Rank)) }
