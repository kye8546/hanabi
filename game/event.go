package game

// EventType 是事件種類。
type EventType int

const (
	EvPlay     EventType = iota // 成功打出
	EvMisplay                   // 打出失敗（閃電 +1）
	EvDiscard                   // 棄牌
	EvHint                      // 給提示
	EvDraw                      // 抽牌
	EvGameOver                  // 遊戲結束
)

// EndReason 是遊戲結束原因。
type EndReason int

const (
	EndNone    EndReason = iota
	EndStrikes           // 三道閃電
	EndPerfect           // 滿分（5 色 25 分、6 色 30 分）
	EndDeckOut           // 牌堆抽完後的最後一輪結束
)

// Event 描述一件已發生的事，供顯示層與 AI 回顧。依 Type 不同只有部分欄位有效。
type Event struct {
	Type    EventType
	Turn    int
	Player  int  // 行動者
	Card    Card // Play/Misplay/Discard：被翻開的牌；Draw：抽到的牌（對抽牌者本人會被遮蔽）
	Slot    int  // Play/Misplay/Discard：原本的手牌位置
	Bonus   bool // EvPlay：完成一色煙火並回收提示
	Target  int  // EvHint：被提示的座位
	Hint    Hint // EvHint
	Touched []int
	Reason  EndReason // EvGameOver
	Score   int       // EvGameOver
}
