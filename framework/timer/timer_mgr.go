package timer

import (
	"container/heap"
	"sync"
	"sync/atomic"
	"time"
)

type TimerMgr struct {
	timerMap    map[int64]*Timer
	timerHeap   *TimerHeap
	mu          sync.RWMutex
	nextTimerId int64
	stopChan    chan struct{}
	running     bool
}

// TimerHeap 实现 heap.Interface 的小顶堆
type TimerHeap []*Timer

func (h TimerHeap) Len() int           { return len(h) }
func (h TimerHeap) Less(i, j int) bool { return h[i].EndTs < h[j].EndTs }
func (h TimerHeap) Swap(i, j int)      { h[i], h[j] = h[j], h[i] }

func (h *TimerHeap) Push(x any) {
	*h = append(*h, x.(*Timer))
}

func (h *TimerHeap) Pop() any {
	old := *h
	n := len(old)
	x := old[n-1]
	*h = old[0 : n-1]
	return x
}

func NewTimerMgr() *TimerMgr {
	timerHeap := &TimerHeap{}
	heap.Init(timerHeap)

	return &TimerMgr{
		timerMap:    make(map[int64]*Timer),
		timerHeap:   timerHeap,
		nextTimerId: 1,
		stopChan:    make(chan struct{}),
		running:     false,
	}
}

func (tm *TimerMgr) Init() {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	if tm.timerMap == nil {
		tm.timerMap = make(map[int64]*Timer)
	}
	if tm.timerHeap == nil {
		tm.timerHeap = &TimerHeap{}
		heap.Init(tm.timerHeap)
	}
	tm.nextTimerId = 1
	tm.running = false
}

func (tm *TimerMgr) Start() {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	if tm.running {
		return
	}

	tm.running = true
	go tm.timerLoop()
}

func (tm *TimerMgr) Stop() {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	if !tm.running {
		return
	}

	close(tm.stopChan)
	tm.running = false
}

func (tm *TimerMgr) NewTimer(duration int64, data any, callback TimerCallback) int64 {
	return tm.newTimerInternal(duration, data, callback, false, 0, 0)
}

func (tm *TimerMgr) NewRepeatTimer(interval int64, repeatCount int, data any, callback TimerCallback) int64 {
	return tm.newTimerInternal(interval, data, callback, true, interval, repeatCount)
}

func (tm *TimerMgr) newTimerInternal(duration int64, data any, callback TimerCallback, repeat bool, interval int64, repeatCount int) int64 {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	timerId := atomic.AddInt64(&tm.nextTimerId, 1)
	now := time.Now().UnixMilli() // 毫秒时间戳
	endTs := now + duration

	timer := &Timer{
		TimerId:     timerId,
		StartTs:     now,
		EndTs:       endTs,
		TimerData:   data,
		Callback:    callback,
		Repeat:      repeat,
		Interval:    interval,
		RepeatCount: repeatCount,
		Executed:    0,
	}

	tm.timerMap[timerId] = timer
	heap.Push(tm.timerHeap, timer)
	return timerId
}

func (tm *TimerMgr) UpdateTimer(timerId int64, endTs int64) {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	if timer, ok := tm.timerMap[timerId]; ok {
		// 从堆中移除旧的定时器
		tm.removeTimerFromHeap(timerId)

		// 更新结束时间
		timer.EndTs = endTs

		// 重新插入堆中
		heap.Push(tm.timerHeap, timer)
	}
}

func (tm *TimerMgr) CancelTimer(timerId int64) {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	if _, ok := tm.timerMap[timerId]; ok {
		delete(tm.timerMap, timerId)
		// 从堆中移除定时器
		tm.removeTimerFromHeap(timerId)
	}
}

func (tm *TimerMgr) GetTimerCount() int {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	return len(tm.timerMap)
}

func (tm *TimerMgr) GetRemainingTime(timerId int64) int64 {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	if timer, ok := tm.timerMap[timerId]; ok {
		now := time.Now().UnixMilli()
		remaining := timer.EndTs - now
		if remaining < 0 {
			return 0
		}
		return remaining
	}
	return -1 // 定时器不存在
}

func (tm *TimerMgr) ResetTimer(timerId int64, duration int64) bool {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	if timer, ok := tm.timerMap[timerId]; ok {
		// 从堆中移除旧的定时器
		tm.removeTimerFromHeap(timerId)

		now := time.Now().UnixMilli()
		timer.EndTs = now + duration
		timer.Executed = 0 // 重置执行次数

		// 重新插入堆中
		heap.Push(tm.timerHeap, timer)
		return true
	}
	return false
}

func (tm *TimerMgr) TriggerTimer(timerId int64) bool {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	if timer, ok := tm.timerMap[timerId]; ok {
		// 从堆中移除定时器
		tm.removeTimerFromHeap(timerId)

		if timer.Callback != nil {
			go timer.Callback(timer.TimerData)
		}

		// 如果是周期性定时器，增加执行次数
		if timer.Repeat {
			timer.Executed++
			// 检查是否达到重复次数限制
			if timer.RepeatCount > 0 && timer.Executed >= timer.RepeatCount {
				delete(tm.timerMap, timerId)
			} else {
				// 重新调度下一次执行
				now := time.Now().UnixMilli()
				timer.EndTs = now + timer.Interval
				// 重新插入堆中
				heap.Push(tm.timerHeap, timer)
			}
		} else {
			// 一次性定时器，触发后删除
			delete(tm.timerMap, timerId)
		}
		return true
	}
	return false
}

// removeTimerFromHeap 从堆中移除指定的定时器
// 注意：调用此函数前必须持有锁
func (tm *TimerMgr) removeTimerFromHeap(timerId int64) {
	// 线性搜索堆中的定时器
	for i, timer := range *tm.timerHeap {
		if timer.TimerId == timerId {
			heap.Remove(tm.timerHeap, i)
			return
		}
	}
}

func (tm *TimerMgr) ClearAll() {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	tm.timerMap = make(map[int64]*Timer)
	tm.timerHeap = &TimerHeap{}
	heap.Init(tm.timerHeap)
}

func (tm *TimerMgr) timerLoop() {
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-tm.stopChan:
			return
		case <-ticker.C:
			tm.checkTimers()
		}
	}
}

func (tm *TimerMgr) checkTimers() {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	now := time.Now().UnixMilli() // 毫秒时间戳

	// 处理所有已过期的定时器
	for tm.timerHeap.Len() > 0 {
		// 查看堆顶定时器（最早要执行的）
		timer := (*tm.timerHeap)[0]

		// 如果堆顶定时器还未到期，则后面的定时器也都未到期
		if timer.EndTs > now {
			break
		}

		// 弹出堆顶定时器
		heap.Pop(tm.timerHeap)

		// 验证定时器是否还在map中（可能已被取消）
		if _, exists := tm.timerMap[timer.TimerId]; !exists {
			continue
		}

		// 执行回调
		if timer.Callback != nil {
			go timer.Callback(timer.TimerData)
		}

		timer.Executed++

		// 处理周期性定时器
		if timer.Repeat {
			// 检查是否达到重复次数限制
			if timer.RepeatCount > 0 && timer.Executed >= timer.RepeatCount {
				// 达到重复次数，删除定时器
				delete(tm.timerMap, timer.TimerId)
				continue
			}

			// 重新调度下一次执行
			timer.EndTs = now + timer.Interval
			// 将更新后的定时器重新加入堆
			heap.Push(tm.timerHeap, timer)
		} else {
			// 一次性定时器，执行后删除
			delete(tm.timerMap, timer.TimerId)
		}
	}
}
