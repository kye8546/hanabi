// Package console 是花火的文字介面：畫面輸出與真人玩家輸入。
// 之後換成 GUI 時，只需要取代這個套件。
package console

import (
	"fmt"
	"io"
	"slices"
	"strconv"
	"strings"

	"hanabi/game"
)

const (
	cellWidth = 7
	rule      = "────────────────────────────────────────────────────"
	ansiReset = "\x1b[0m"
	ansiDim   = "\x1b[2m"
	ansiBold  = "\x1b[1m"
)

var ansiColors = [game.NumColors]string{
	"\x1b[1;31m", // 紅
	"\x1b[1;33m", // 黃
	"\x1b[1;32m", // 綠
	"\x1b[1;94m", // 藍（亮藍在深色終端較好讀）
	"\x1b[1;97m", // 白
	"\x1b[1;36m", // 青
	"\x1b[1;90m", // 黑（亮黑 = 深灰，深色與淺色終端都看得到）
}

// Renderer 負責把 PlayerView 與事件印成文字。
type Renderer struct {
	w     io.Writer
	names []string
	color bool
	// showNeg 為 false 時手牌認知只顯示正向提示（被點到的值），
	// 不顯示「不是紅色」這類由沒被點到推出的反向資訊
	showNeg bool
}

func NewRenderer(w io.Writer, names []string, color, showNeg bool) *Renderer {
	return &Renderer{w: w, names: names, color: color, showNeg: showNeg}
}

func (r *Renderer) printf(format string, a ...any) { fmt.Fprintf(r.w, format, a...) }

func (r *Renderer) style(code, s string) string {
	if !r.color {
		return s
	}
	return code + s + ansiReset
}

func (r *Renderer) paint(c game.Color, s string) string { return r.style(ansiColors[c], s) }
func (r *Renderer) card(c game.Card) string             { return r.paint(c.Color, c.String()) }

// cell 把已上色的字串補空白到固定寬度；plainLen 是不含色碼的可見長度。
func cell(styled string, plainLen int) string {
	return styled + strings.Repeat(" ", max(1, cellWidth-plainLen))
}

func (r *Renderer) seatLabel(seat int) string { return fmt.Sprintf("(%d) %s", seat, r.names[seat]) }

// Board 印出 v 視角的完整盤面。
func (r *Renderer) Board(v game.PlayerView) {
	r.printf("\n%s\n", strings.ReplaceAll(rule, "─", "═"))
	title := fmt.Sprintf(" 花火 · 第 %d 回合", v.Turn)
	if v.FinalTurnsLeft > 0 {
		title += r.style(ansiBold, fmt.Sprintf("    ⚠ 最後一輪：剩 %d 回合", v.FinalTurnsLeft))
	}
	r.printf("%s\n", title)

	var fw []string
	for _, c := range v.Colors() {
		fw = append(fw, r.paint(c, fmt.Sprintf("%s%d", c.Letter(), v.Fireworks[c])))
	}
	r.printf(" 煙火  %s      分數 %d/%d\n", strings.Join(fw, "  "), v.Score(), v.MaxScore())

	r.printf(" 提示  %s%s %d/%d    閃電  %s%s %d/%d    牌堆 %d 張\n",
		strings.Repeat("●", v.Hints), strings.Repeat("○", game.MaxHints-v.Hints), v.Hints, game.MaxHints,
		strings.Repeat("✗", v.Strikes), strings.Repeat("○", game.MaxStrikes-v.Strikes), v.Strikes, game.MaxStrikes,
		v.DeckSize)
	r.printf(" 棄牌  %s\n", r.discards(v.Discards))
	r.printf("%s\n", rule)

	// 依行動順序：從自己的下一位開始，自己放最後（最靠近輸入提示）
	for i := 1; i <= v.NumPlayers; i++ {
		seat := (v.Seat + i) % v.NumPlayers
		r.hand(v, seat)
	}
	r.printf("%s\n", rule)
}

func (r *Renderer) discards(ds []game.Card) string {
	if len(ds) == 0 {
		return "（無）"
	}
	var parts []string
	for _, c := range game.AllColors {
		var ranks []int
		for _, d := range ds {
			if d.Color == c {
				ranks = append(ranks, int(d.Rank))
			}
		}
		if len(ranks) == 0 {
			continue
		}
		slices.Sort(ranks)
		s := make([]string, len(ranks))
		for i, rk := range ranks {
			s[i] = strconv.Itoa(rk)
		}
		parts = append(parts, r.paint(c, c.Letter()+":"+strings.Join(s, " ")))
	}
	return strings.Join(parts, "  ")
}

func (r *Renderer) hand(v game.PlayerView, seat int) {
	label := r.seatLabel(seat)
	if seat == v.Current && !v.Over {
		label += "  ◀ 輪到" + map[bool]string{true: "你", false: "他"}[seat == v.Seat]
	}
	r.printf(" %s\n", label)

	cards := v.Hands[seat]
	var pos strings.Builder
	for i, vc := range cards {
		s := strconv.Itoa(i + 1)
		if vc.Know.Clued {
			s += "*"
		}
		pos.WriteString(cell(s, len(s)))
	}
	r.printf("     位置  %s\n", pos.String())

	if seat == v.Seat {
		// 自己：列出每張牌仍可能的顏色與數字（隱藏反向資訊時，沒被點到就是 ?）
		var cols, ranks strings.Builder
		for _, vc := range cards {
			pc := r.colorsShown(vc.Know)
			var s strings.Builder
			for _, c := range pc {
				s.WriteString(r.paint(c, c.Letter()))
			}
			if len(pc) == 0 {
				cols.WriteString(cell(r.style(ansiDim, "?"), 1))
			} else {
				cols.WriteString(cell(s.String(), len(pc)))
			}
			var rs string
			for _, rk := range r.ranksShown(vc.Know) {
				rs += strconv.Itoa(int(rk))
			}
			if rs == "" {
				ranks.WriteString(cell(r.style(ansiDim, "?"), 1))
			} else {
				ranks.WriteString(cell(rs, len(rs)))
			}
		}
		r.printf("     顏色  %s\n", cols.String())
		r.printf("     數字  %s\n", ranks.String())
		return
	}

	// 別人：實際牌面，以及他自己目前知道什麼
	var actual, known strings.Builder
	for _, vc := range cards {
		actual.WriteString(cell(r.card(vc.Card), 2))
		known.WriteString(cell(r.knownShort(vc.Know), 2))
	}
	r.printf("     手牌  %s\n", actual.String())
	r.printf("     他知  %s\n", known.String())
}

// colorsShown 回傳要顯示的可能顏色；隱藏反向資訊時只回傳被提示點到的顏色（沒有則為空）。
func (r *Renderer) colorsShown(k game.Knowledge) []game.Color {
	if r.showNeg {
		return k.PossibleColors()
	}
	if c, ok := k.HintedColor(); ok {
		return []game.Color{c}
	}
	return nil
}

// ranksShown 同 colorsShown，針對數字。
func (r *Renderer) ranksShown(k game.Knowledge) []game.Rank {
	if r.showNeg {
		return k.PossibleRanks()
	}
	if rk, ok := k.HintedRank(); ok {
		return []game.Rank{rk}
	}
	return nil
}

// knownShort 把認知縮寫成兩個字元，例如 "R3"、"R?"、"?3"、"??"。
func (r *Renderer) knownShort(k game.Knowledge) string {
	cs, rs := "?", "?"
	if ranks := r.ranksShown(k); len(ranks) == 1 {
		rs = strconv.Itoa(int(ranks[0]))
	}
	if cols := r.colorsShown(k); len(cols) == 1 {
		return r.paint(cols[0], cols[0].Letter()+rs)
	}
	return r.style(ansiDim, cs+rs)
}

func (r *Renderer) hintText(h game.Hint) string {
	if h.Kind == game.HintColor {
		return r.paint(h.Color, h.Color.Name()+"色")
	}
	return fmt.Sprintf("數字 %d", h.Rank)
}

// Describe 把一個事件寫成一句話；回傳空字串表示不需顯示（例如抽牌）。
func (r *Renderer) Describe(e game.Event) string {
	who := r.names[e.Player]
	switch e.Type {
	case game.EvPlay:
		s := fmt.Sprintf("%s 打出 %s（位置 %d）✓ 成功", who, r.card(e.Card), e.Slot+1)
		if e.Card.Rank == game.MaxRank {
			s += "，完成" + r.paint(e.Card.Color, e.Card.Color.Name()+"色") + "煙火！"
			if e.Bonus {
				s += "提示 +1"
			}
		}
		return s
	case game.EvMisplay:
		return fmt.Sprintf("%s 打出 %s（位置 %d）%s", who, r.card(e.Card), e.Slot+1, r.style(ansiBold, "✗ 失敗！閃電 +1"))
	case game.EvDiscard:
		return fmt.Sprintf("%s 棄掉 %s（位置 %d），提示 +1", who, r.card(e.Card), e.Slot+1)
	case game.EvHint:
		slots := make([]string, len(e.Touched))
		for i, s := range e.Touched {
			slots[i] = strconv.Itoa(s + 1)
		}
		return fmt.Sprintf("%s 提示 %s「%s」→ 位置 %s", who, r.names[e.Target], r.hintText(e.Hint), strings.Join(slots, ", "))
	case game.EvGameOver:
		return "遊戲結束：" + endReasonText(e.Reason)
	}
	return ""
}

// Events 逐行印出事件；withTurn 為 true 時前面加上回合數。
func (r *Renderer) Events(evs []game.Event, withTurn bool) {
	for _, e := range evs {
		s := r.Describe(e)
		if s == "" {
			continue
		}
		if withTurn {
			r.printf("  [%3d] %s\n", e.Turn, s)
		} else {
			r.printf("  • %s\n", s)
		}
	}
}

func endReasonText(reason game.EndReason) string {
	switch reason {
	case game.EndStrikes:
		return "三道閃電，煙火秀失敗"
	case game.EndPerfect:
		return "五色煙火全部完成！"
	case game.EndDeckOut:
		return "牌堆用盡，最後一輪結束"
	}
	return ""
}

// rating 是官方規則書的評語。
// rating 照官方評語；6 色變體時 21 分到滿分前都是「非凡」，只有滿分是「傳奇」。
func rating(score, max int) string {
	switch {
	case score <= 5:
		return "慘不忍睹，觀眾噓聲四起"
	case score <= 10:
		return "平庸，只有零星掌聲"
	case score <= 15:
		return "值得尊敬，但觀眾不會記得"
	case score <= 20:
		return "優秀，觀眾很滿意"
	case score < max:
		return "非凡，觀眾將永遠記得"
	}
	return "傳奇！大人小孩都看得目瞪口呆"
}

// Result 印出最終結果。
func (r *Renderer) Result(v game.PlayerView) {
	score := v.Score()
	if v.EndReason == game.EndStrikes {
		r.printf("\n %s（煙火進度 %d，依官方規則得 0 分）\n", endReasonText(v.EndReason), score)
		score = 0
	} else {
		r.printf("\n %s\n", endReasonText(v.EndReason))
	}
	r.printf(" 最終分數：%s  —  %s\n\n", r.style(ansiBold, fmt.Sprintf("%d / %d", score, v.MaxScore())), rating(score, v.MaxScore()))
}
