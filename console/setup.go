package console

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"hanabi/game"
)

// Settings 是開局前玩家必須選定的設定。
type Settings struct {
	Players int          // 總人數（含自己）
	Level   int          // H-Group 慣例等級
	Variant game.Variant // 變體（Hanab Live 的 No Variant、6 Suits、Black (6 Suits)）
	Seed    uint64       // 洗牌種子；0 = 隨機
}

// AskSetup 在開局前依序詢問人數、慣例等級、變體與洗牌種子，輸入不合法就重問。
// 人數與等級必須選；變體直接按 Enter 是標準 5 色，種子直接按 Enter 是隨機（Seed = 0）。seed 不是 0（啟動參數已指定）時不問種子。
// levels 是各等級名稱（levels[0] = Lv1），maxLevel 是目前已實作的最高等級。
func AskSetup(in *bufio.Scanner, out io.Writer, levels []string, maxLevel int, seed uint64) Settings {
	st := Settings{Seed: seed}
	st.Players = askInt(in, out, fmt.Sprintf("玩家人數（%d–%d，含你自己）", game.MinPlayers, game.MaxPlayers),
		func(n int) error {
			if n < game.MinPlayers || n > game.MaxPlayers {
				return fmt.Errorf("請輸入 %d–%d", game.MinPlayers, game.MaxPlayers)
			}
			return nil
		})

	fmt.Fprintln(out, "\n H-Group 慣例等級（https://hanabi.github.io/），✓ = AI 已實作")
	fmt.Fprintln(out, " AI 會照選定等級的規範給提示、解讀提示；你也應該照同一個等級玩。")
	half := (len(levels) + 1) / 2
	for i := range half {
		left := levelLine(i+1, levels[i], maxLevel)
		right := ""
		if j := i + half; j < len(levels) {
			right = levelLine(j+1, levels[j], maxLevel)
		}
		fmt.Fprintf(out, " %s%s\n", padRight(left, 34), right)
	}
	st.Level = askInt(in, out, fmt.Sprintf("慣例等級（1–%d）", len(levels)), func(n int) error {
		switch {
		case n < 1 || n > len(levels):
			return fmt.Errorf("請輸入 1–%d", len(levels))
		case n > maxLevel:
			return fmt.Errorf("Lv%d 的 AI 尚未實作，目前只能選 Lv1–Lv%d", n, maxLevel)
		}
		return nil
	})
	st.Variant = askVariant(in, out)
	if st.Seed == 0 {
		st.Seed = askSeed(in, out)
	}
	return st
}

// variants 是可選的變體（Hanab Live 的名稱，https://github.com/Hanabi-Live/hanabi-live/blob/main/docs/variants.md）。
var variants = []struct {
	name    string
	variant game.Variant
}{
	{"標準（No Variant）：5 色、滿分 25", game.NoVariant},
	{"6 Suits：多一個青色（T），每色 10 張、共 60 張，滿分 30", game.SixSuits},
	{"Black (6 Suits)：多一個黑色（K），每個數字只有 1 張（全都是 critical）、共 55 張，滿分 30", game.BlackSix},
}

// askVariant 詢問變體；空白 = 標準。
func askVariant(in *bufio.Scanner, out io.Writer) game.Variant {
	fmt.Fprintln(out, "\n 變體")
	for i, va := range variants {
		fmt.Fprintf(out, " %d. %s\n", i+1, va.name)
	}
	for {
		fmt.Fprintf(out, "變體（1–%d，直接按 Enter = 標準）：", len(variants))
		if !in.Scan() {
			fmt.Fprintln(out, "\n再見！")
			os.Exit(0)
		}
		t := strings.TrimSpace(in.Text())
		if t == "" {
			return game.NoVariant
		}
		n, err := strconv.Atoi(t)
		if err != nil || n < 1 || n > len(variants) {
			fmt.Fprintf(out, "✗ 請輸入 1–%d，或直接按 Enter\n", len(variants))
			continue
		}
		return variants[n-1].variant
	}
}

// askSeed 詢問洗牌種子：空白 = 隨機（回傳 0），否則必須是正整數。
func askSeed(in *bufio.Scanner, out io.Writer) uint64 {
	for {
		fmt.Fprint(out, "洗牌種子（相同種子可重玩同一副牌；直接按 Enter = 隨機）：")
		if !in.Scan() {
			fmt.Fprintln(out, "\n再見！")
			os.Exit(0)
		}
		t := strings.TrimSpace(in.Text())
		if t == "" {
			return 0
		}
		n, err := strconv.ParseUint(t, 10, 64)
		if err != nil || n == 0 {
			fmt.Fprintln(out, "✗ 請輸入正整數，或直接按 Enter")
			continue
		}
		return n
	}
}

func levelLine(n int, name string, maxLevel int) string {
	mark := " "
	if n <= maxLevel {
		mark = "✓"
	}
	return fmt.Sprintf("%s %2d. %s", mark, n, name)
}

// padRight 以終端顯示寬度（中文字算 2 格）補空白。
func padRight(s string, width int) string {
	w := 0
	for _, r := range s {
		if r >= 0x2E80 && r != '✓' {
			w += 2
		} else {
			w++
		}
	}
	return s + strings.Repeat(" ", max(1, width-w))
}

// askInt 重複詢問直到拿到通過 check 的整數；輸入結束就離開程式。
func askInt(in *bufio.Scanner, out io.Writer, prompt string, check func(int) error) int {
	for {
		fmt.Fprintf(out, "%s：", prompt)
		if !in.Scan() {
			fmt.Fprintln(out, "\n再見！")
			os.Exit(0)
		}
		n, err := strconv.Atoi(strings.TrimSpace(in.Text()))
		if err != nil {
			fmt.Fprintln(out, "✗ 請輸入數字")
			continue
		}
		if err := check(n); err != nil {
			fmt.Fprintf(out, "✗ %v\n", err)
			continue
		}
		return n
	}
}
