package console

import (
	"bufio"
	"fmt"
	"io"
)

// WaitEnter 顯示提示並等玩家按 Enter 再結束（在 Windows 直接執行 exe 時，視窗才不會一結束就關掉）。
// 輸入已經結束（例如用管線餵指令）時立刻返回。
func WaitEnter(in *bufio.Scanner, out io.Writer) {
	fmt.Fprint(out, "\n按 Enter 結束…")
	in.Scan()
	fmt.Fprintln(out)
}
