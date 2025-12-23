package timer

import (
	"sync"
	"testing"
	"time"
)

func TestTimerMgr_NewTimer(t *testing.T) {
	tm := NewTimerMgr()
	tm.Init()
	tm.Start()
	defer tm.Stop()

	var wg sync.WaitGroup
	wg.Add(1)

	triggered := false
	timerId := tm.NewTimer(100, "test data", func(data any) {
		triggered = true
		if data != "test data" {
			t.Errorf("Expected data 'test data', got %v", data)
		}
		wg.Done()
	})

	if timerId <= 0 {
		t.Error("Timer ID should be positive")
	}

	// 等待定时器触发
	wg.Wait()

	if !triggered {
		t.Error("Timer should have been triggered")
	}
}

func TestTimerMgr_NewRepeatTimer(t *testing.T) {
	tm := NewTimerMgr()
	tm.Init()
	tm.Start()
	defer tm.Stop()

	var wg sync.WaitGroup
	triggerCount := 0
	expectedCount := 3

	wg.Add(expectedCount)

	timerId := tm.NewRepeatTimer(50, expectedCount, nil, func(data any) {
		triggerCount++
		wg.Done()
	})

	if timerId <= 0 {
		t.Error("Timer ID should be positive")
	}

	// 等待所有重复触发完成
	wg.Wait()

	if triggerCount != expectedCount {
		t.Errorf("Expected %d triggers, got %d", expectedCount, triggerCount)
	}

	// 验证定时器已被删除
	time.Sleep(100 * time.Millisecond)
	if tm.GetTimerCount() > 0 {
		t.Error("Repeat timer should have been removed after reaching repeat count")
	}
}

func TestTimerMgr_CancelTimer(t *testing.T) {
	tm := NewTimerMgr()
	tm.Init()
	tm.Start()
	defer tm.Stop()

	triggered := false
	timerId := tm.NewTimer(200, nil, func(data any) {
		triggered = true
	})

	// 立即取消定时器
	tm.CancelTimer(timerId)

	// 等待足够长时间，确保定时器不会触发
	time.Sleep(300 * time.Millisecond)

	if triggered {
		t.Error("Timer should not have been triggered after cancellation")
	}
}

func TestTimerMgr_ResetTimer(t *testing.T) {
	tm := NewTimerMgr()
	tm.Init()
	tm.Start()
	defer tm.Stop()

	var wg sync.WaitGroup
	wg.Add(1)

	startTime := time.Now()
	var triggerTime time.Time

	timerId := tm.NewTimer(200, nil, func(data any) {
		triggerTime = time.Now()
		wg.Done()
	})

	// 在100ms后重置定时器为300ms
	time.Sleep(100 * time.Millisecond)
	if !tm.ResetTimer(timerId, 300) {
		t.Error("Failed to reset timer")
	}

	// 等待定时器触发
	wg.Wait()

	elapsed := triggerTime.Sub(startTime)
	// 总时间应该是100ms + 300ms = 400ms左右，但由于定时器检查间隔100ms，允许更大误差
	// 最小时间：100ms（等待）+ 300ms（重置后时间）- 50ms（误差）= 350ms
	// 最大时间：100ms（等待）+ 300ms（重置后时间）+ 150ms（定时器检查间隔+误差）= 550ms
	if elapsed < 350*time.Millisecond || elapsed > 550*time.Millisecond {
		t.Errorf("Reset timer triggered at wrong time: %v (expected 400ms ± 150ms)", elapsed)
	}
}

func TestTimerMgr_GetRemainingTime(t *testing.T) {
	tm := NewTimerMgr()
	tm.Init()
	tm.Start()
	defer tm.Stop()

	timerId := tm.NewTimer(500, nil, func(data any) {})

	// 立即获取剩余时间
	remaining := tm.GetRemainingTime(timerId)
	if remaining <= 0 || remaining > 500 {
		t.Errorf("Remaining time should be between 0 and 500, got %d", remaining)
	}

	// 获取不存在的定时器
	remaining = tm.GetRemainingTime(99999)
	if remaining != -1 {
		t.Errorf("Expected -1 for non-existent timer, got %d", remaining)
	}
}

func TestTimerMgr_TriggerTimer(t *testing.T) {
	tm := NewTimerMgr()
	tm.Init()
	tm.Start()
	defer tm.Stop()

	var wg sync.WaitGroup
	wg.Add(1)

	triggered := false
	timerId := tm.NewTimer(1000, nil, func(data any) {
		triggered = true
		wg.Done()
	})

	// 手动触发定时器
	if !tm.TriggerTimer(timerId) {
		t.Error("Failed to trigger timer")
	}

	// 等待回调完成
	wg.Wait()

	// 验证定时器已触发
	if !triggered {
		t.Error("Timer should have been triggered manually")
	}

	// 验证定时器已被删除（一次性定时器）
	time.Sleep(50 * time.Millisecond)
	if tm.GetTimerCount() > 0 {
		t.Error("One-shot timer should have been removed after manual trigger")
	}
}

func TestTimerMgr_ClearAll(t *testing.T) {
	tm := NewTimerMgr()
	tm.Init()
	tm.Start()
	defer tm.Stop()

	// 创建多个定时器
	for i := 0; i < 5; i++ {
		tm.NewTimer(100+int64(i*50), i, func(data any) {})
	}

	if tm.GetTimerCount() != 5 {
		t.Errorf("Expected 5 timers, got %d", tm.GetTimerCount())
	}

	// 清除所有定时器
	tm.ClearAll()

	if tm.GetTimerCount() != 0 {
		t.Errorf("Expected 0 timers after ClearAll, got %d", tm.GetTimerCount())
	}
}

func TestTimerMgr_ConcurrentAccess(t *testing.T) {
	tm := NewTimerMgr()
	tm.Init()
	tm.Start()
	defer tm.Stop()

	var wg sync.WaitGroup
	numGoroutines := 10
	timersPerGoroutine := 5

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(goroutineId int) {
			defer wg.Done()

			var localWg sync.WaitGroup
			timerIds := make([]int64, timersPerGoroutine)

			// 创建定时器
			for j := 0; j < timersPerGoroutine; j++ {
				localWg.Add(1)
				timerId := tm.NewTimer(int64(100+goroutineId*10+j*5), goroutineId, func(data any) {
					localWg.Done()
				})
				timerIds[j] = timerId
			}

			// 等待所有定时器触发
			localWg.Wait()

			// 验证定时器已被删除
			for _, timerId := range timerIds {
				if tm.GetRemainingTime(timerId) != -1 {
					t.Errorf("Timer %d should have been removed after triggering", timerId)
				}
			}
		}(i)
	}

	wg.Wait()
}

func TestTimerMgr_StopBeforeStart(t *testing.T) {
	tm := NewTimerMgr()
	tm.Init()

	// 停止未启动的定时器管理器应该不会panic
	tm.Stop()

	// 启动后再停止
	tm.Start()
	tm.Stop()

	// 再次停止应该不会panic
	tm.Stop()
}
