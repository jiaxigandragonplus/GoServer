package timer

type TimerMgr struct {
	timerMap map[int64]*Timer
}

func NewTimerMgr() *TimerMgr {
	return &TimerMgr{}
}

func (tm *TimerMgr) Init() {
	tm.timerMap = make(map[int64]*Timer)
}

func (tm *TimerMgr) Start() {

}

func (tm *TimerMgr) NewTimer(duration int64, data any, callback TimerCallback) int64 {
	return 0
}

func (tm *TimerMgr) UpdateTimer(timerId int64, endTs int64) {

}

func (tm *TimerMgr) CancelTimer(timerId int64) {

}
