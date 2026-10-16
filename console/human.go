package console

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"hanabi/game"
	"hanabi/player"
)

const commandHint = "指令： p <位置> 打出 │ d <位置> 棄牌 │ h <玩家> <顏色|數字> 提示 │ u 回溯 │ x 偷看 │ n 反向資訊 │ ? 說明"

const helpText = `
 p <位置>              打出手牌，例如 p 1（位置從左邊 1 開始；新抽的牌放最左邊）
 d <位置>              棄掉手牌，例如 d 5（提示 token 滿 8 個時不能棄）
 h <玩家> <顏色|數字>  給提示，例如 h 1 r（給 1 號提示紅色）、h 2 3（給 2 號提示數字 3）
                       玩家可用編號或名字；顏色 r/y/g/b/w 或 紅/黃/綠/藍/白（6 Suits 多一個 t / 青，Black 多一個 k / 黑；Rainbow 的彩虹色不能提示，每種顏色提示都會點到它）
 u                     回溯：回到你上一次行動之前（撤銷你那一手和之後 AI 的行動），可連續使用
 x                     偷看自己手牌的真實牌面（紀錄檔會註明你在第幾回合偷看過）
 log                   顯示完整紀錄
 b                     重新顯示盤面
 n                     切換是否顯示反向提示資訊（「不是紅色」這類由沒被點到推出的資訊），連 AI 手牌的「他知」一起切換
 q                     離開遊戲

 畫面說明
   位置列的 * 代表這張牌曾被提示點到
   「他知」是對方自己對那張牌的認知（?? = 完全不知道）
   你自己的「顏色 / 數字」是被提示點到的顏色 / 數字（? = 還沒被點到）
   用 n（或啟動時加 -showneg）顯示反向資訊：自己和 AI 的牌都改成列出所有剩下的可能（和 Hanab Live 一樣），
   AI 的「他知」也分成顏色、數字兩列
 AI 依開局選的 H-Group 慣例等級行動，你給的提示也會被照那個等級解讀
`

// Human 從終端讀取指令的真人玩家，實作 player.Player 與 player.Rejecter。
type Human struct {
	name  string
	in    *bufio.Scanner
	out   io.Writer
	r     *Renderer
	names []string
	seen  int  // 已顯示過的事件數
	retry bool // 上一次行動被拒絕，不重畫盤面

	OnQuit func()             // 選用：玩家離開（q 或輸入結束）前呼叫，例如寫完對局紀錄
	Peek   func() []game.Card // 選用：偷看自己手牌的真實牌面（由持有整局遊戲的呼叫端提供）
}

// Rewound 在回溯重建局面後呼叫：之前的事件不再當成「最近發生」重播。
func (h *Human) Rewound(v game.PlayerView) {
	h.seen = len(v.Log)
	h.retry = false
	fmt.Fprintln(h.out, "↶ 已回溯到你上一次行動之前")
}

func (h *Human) quit() {
	if h.OnQuit != nil {
		h.OnQuit()
	}
	os.Exit(0)
}

// NewHuman 建立真人玩家。in 要和開局設定（AskSetup）共用同一個 Scanner，輸入才不會被分開緩衝。
func NewHuman(name string, in *bufio.Scanner, out io.Writer, r *Renderer, names []string) *Human {
	return &Human{name: name, in: in, out: out, r: r, names: names}
}

func (h *Human) Name() string { return h.name }

func (h *Human) Act(v game.PlayerView) game.Action {
	if !h.retry {
		h.showUpdate(v)
		fmt.Fprintln(h.out, commandHint)
	}
	h.retry = false
	for {
		fmt.Fprint(h.out, "> ")
		if !h.in.Scan() {
			fmt.Fprintln(h.out, "\n再見！")
			h.quit()
		}
		a, err := h.parse(h.in.Text(), v)
		if err == nil {
			return a
		}
		if !errors.Is(err, errHandled) {
			fmt.Fprintf(h.out, "✗ %v（輸入 ? 看說明）\n", err)
		}
	}
}

func (h *Human) Reject(_ game.Action, err error) {
	fmt.Fprintf(h.out, "✗ 不行：%v\n", err)
	h.retry = true
}

// Final 在遊戲結束後顯示最終盤面、最後發生的事與分數。
func (h *Human) Final(v game.PlayerView) {
	h.showUpdate(v)
	h.r.Result(v)
}

func (h *Human) showUpdate(v game.PlayerView) {
	h.r.Board(v)
	if news := v.Log[h.seen:]; len(news) > 0 {
		fmt.Fprintln(h.out, " 最近發生：")
		h.r.Events(news, false)
		fmt.Fprintln(h.out, rule)
	}
	h.seen = len(v.Log)
}

// errHandled 表示指令已處理完（例如顯示說明），不是行動、也不需要印錯誤。
var errHandled = errors.New("handled")

func (h *Human) parse(line string, v game.PlayerView) (game.Action, error) {
	f := splitCompact(strings.Fields(line))
	if len(f) == 0 {
		return game.Action{}, errHandled
	}
	switch strings.ToLower(f[0]) {
	case "?", "help":
		fmt.Fprint(h.out, helpText)
		return game.Action{}, errHandled
	case "log":
		h.r.Events(v.Log, true)
		return game.Action{}, errHandled
	case "b", "board":
		h.r.Board(v)
		return game.Action{}, errHandled
	case "n", "neg":
		on := h.r.ToggleNeg()
		h.r.Board(v)
		fmt.Fprintf(h.out, " 反向提示資訊：%s\n", map[bool]string{true: "顯示", false: "隱藏"}[on])
		return game.Action{}, errHandled
	case "u", "undo":
		return player.UndoAction, nil
	case "x", "peek":
		if h.Peek == nil {
			return game.Action{}, errors.New("這個模式不能偷看")
		}
		var parts []string
		for i, c := range h.Peek() {
			parts = append(parts, fmt.Sprintf("%d:%s", i+1, h.r.card(c)))
		}
		fmt.Fprintf(h.out, " 你的手牌（偷看）：%s\n", strings.Join(parts, "  "))
		return game.Action{}, errHandled
	case "q", "quit", "exit":
		fmt.Fprintln(h.out, "再見！")
		h.quit()
	case "p", "play", "d", "discard":
		if len(f) != 2 {
			return game.Action{}, errors.New("格式：p <位置> 或 d <位置>")
		}
		n, err := strconv.Atoi(f[1])
		if err != nil {
			return game.Action{}, fmt.Errorf("位置要是數字，收到 %q", f[1])
		}
		if f[0][0] == 'p' {
			return game.Play(n - 1), nil
		}
		return game.Discard(n - 1), nil
	case "h", "hint":
		if len(f) != 3 {
			return game.Action{}, errors.New("格式：h <玩家> <顏色|數字>，例如 h 1 r")
		}
		target, ok := h.parseTarget(f[1])
		if !ok {
			return game.Action{}, fmt.Errorf("找不到玩家 %q", f[1])
		}
		var letters []string
		for _, c := range v.Variant.ClueColors() {
			letters = append(letters, strings.ToLower(c.Letter()))
		}
		if c, ok := game.ParseColor(f[2]); ok {
			if c == game.Rainbow && v.Variant.HasColor(c) {
				return game.Action{}, fmt.Errorf("彩虹色不能提示（每種顏色提示都會點到彩虹色的牌；可用 %s）", strings.Join(letters, "/"))
			}
			if !v.Variant.CanClue(c) {
				return game.Action{}, fmt.Errorf("這局沒有%s色（只有 %s）", c.Name(), strings.Join(letters, "/"))
			}
			return game.HintColorTo(target, c), nil
		}
		if n, err := strconv.Atoi(f[2]); err == nil {
			return game.HintRankTo(target, game.Rank(n)), nil
		}
		return game.Action{}, fmt.Errorf("看不懂提示內容 %q，請用 %s 或 1–5", f[2], strings.Join(letters, "/"))
	}
	return game.Action{}, fmt.Errorf("看不懂指令 %q", f[0])
}

func (h *Human) parseTarget(s string) (int, bool) {
	if n, err := strconv.Atoi(s); err == nil {
		return n, n >= 0 && n < len(h.names)
	}
	for i, name := range h.names {
		if strings.EqualFold(name, s) {
			return i, true
		}
	}
	return 0, false
}

// splitCompact 支援不加空白的簡寫：p3 → p 3、h1r → h 1 r。
func splitCompact(f []string) []string {
	if len(f) != 1 || len(f[0]) < 2 {
		return f
	}
	s := strings.ToLower(f[0])
	switch {
	case (s[0] == 'p' || s[0] == 'd') && isDigits(s[1:]):
		return []string{s[:1], s[1:]}
	case s[0] == 'h' && len(s) == 3 && isDigits(s[1:2]):
		return []string{"h", s[1:2], s[2:]}
	}
	return f
}

func isDigits(s string) bool {
	_, err := strconv.Atoi(s)
	return err == nil
}
