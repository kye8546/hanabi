package hgroup

import (
	"os"
	"runtime/debug"
	"testing"
)

// TestMain：自我對局一次跑上千局，每局都配置很多短命的小物件；預設的 GC 門檻（100%）讓 GC 跑得太頻繁，
// 一半以上的 CPU 花在 GC 與執行緒互等上。存活的資料很少（峰值約 150 MB），調成 800% 時同樣的對局快兩倍多。
func TestMain(m *testing.M) {
	debug.SetGCPercent(800)
	os.Exit(m.Run())
}
