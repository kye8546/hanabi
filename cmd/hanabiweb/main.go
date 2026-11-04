// 花火網頁版：啟動 HTTP 伺服器，用瀏覽器玩。console 版（cmd/hanabi）不受影響。
package main

import (
	"flag"
	"log"
	"net/http"
	"os"
	"time"

	"hanabi/web"
)

func main() {
	addr := flag.String("addr", "", "監聽位址；空的 = :$PORT（雲端主機多半用這個環境變數），沒有就是 localhost:8080")
	logDir := flag.String("logs", "weblogs", "對局紀錄的資料夾（和 console 版的 logs/ 分開，不影響 AI 的重播基準）")
	aiDelay := flag.Duration("aidelay", 700*time.Millisecond, "AI 每一手至少間隔多久（讓人看得出發生了什麼）")
	flag.Parse()

	if *addr == "" {
		*addr = "localhost:8080"
		if p := os.Getenv("PORT"); p != "" {
			*addr = ":" + p
		}
	}
	srv := web.New(*logDir, *aiDelay)
	log.Printf("花火網頁版：http://%s （對局紀錄存到 %s/）", *addr, *logDir)
	log.Fatal(http.ListenAndServe(*addr, srv.Handler()))
}
