package timer

// Callback 定时器回调函数类型
// data 是传递给定时器的参数
type TimerCallback func(data any)

// Timer 定时器结构
type Timer struct {
	TimerId     int64         // 定时器ID
	StartTs     int64         // 定时器开始时间
	EndTs       int64         // 定时器结束时间
	TimerData   any           // 定时器参数
	Callback    TimerCallback // 定时器回调函数
	Repeat      bool          // 是否重复执行
	Interval    int64         // 重复间隔（毫秒）
	RepeatCount int           // 重复次数，0表示无限重复
	Executed    int           // 已执行次数
}
