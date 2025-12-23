package main

import (
	"fmt"
	"sync"
	"time"

	"github.com/GooLuck/GoServer/framework/timer"
)

func main() {
	fmt.Println("=== 定时器管理器示例 ===")

	// 创建定时器管理器
	tm := timer.NewTimerMgr()
	tm.Init()
	tm.Start()
	defer tm.Stop()

	// 示例1: 基本定时器
	fmt.Println("\n--- 示例1: 基本定时器 ---")
	var wg1 sync.WaitGroup
	wg1.Add(1)

	timerId1 := tm.NewTimer(200, "Hello, Timer!", func(data any) {
		fmt.Printf("定时器触发! 数据: %v\n", data)
		wg1.Done()
	})
	fmt.Printf("创建定时器 ID: %d, 将在200ms后触发\n", timerId1)

	wg1.Wait()

	// 示例2: 周期性定时器
	fmt.Println("\n--- 示例2: 周期性定时器 ---")
	var wg2 sync.WaitGroup
	triggerCount := 0
	expectedCount := 3

	wg2.Add(expectedCount)

	timerId2 := tm.NewRepeatTimer(150, expectedCount, "Repeat Data", func(data any) {
		triggerCount++
		fmt.Printf("周期性定时器触发 #%d, 数据: %v\n", triggerCount, data)
		wg2.Done()
	})
	fmt.Printf("创建周期性定时器 ID: %d, 将触发%d次，间隔150ms\n", timerId2, expectedCount)

	wg2.Wait()

	// 示例3: 取消定时器
	fmt.Println("\n--- 示例3: 取消定时器 ---")
	cancelled := false
	timerId3 := tm.NewTimer(300, nil, func(data any) {
		cancelled = true
		fmt.Println("这个定时器不应该触发!")
	})

	fmt.Printf("创建定时器 ID: %d, 将在300ms后触发\n", timerId3)
	time.Sleep(100 * time.Millisecond)
	fmt.Println("取消定时器...")
	tm.CancelTimer(timerId3)

	time.Sleep(400 * time.Millisecond)
	if !cancelled {
		fmt.Println("成功: 定时器被取消，回调未执行")
	}

	// 示例4: 重置定时器
	fmt.Println("\n--- 示例4: 重置定时器 ---")
	var wg4 sync.WaitGroup
	wg4.Add(1)

	startTime := time.Now()
	timerId4 := tm.NewTimer(500, nil, func(data any) {
		elapsed := time.Since(startTime)
		fmt.Printf("重置后的定时器触发! 经过时间: %v\n", elapsed)
		wg4.Done()
	})

	fmt.Printf("创建定时器 ID: %d, 原定500ms后触发\n", timerId4)
	time.Sleep(200 * time.Millisecond)
	fmt.Println("重置定时器为300ms...")
	tm.ResetTimer(timerId4, 300)

	wg4.Wait()

	// 示例5: 获取剩余时间
	fmt.Println("\n--- 示例5: 获取剩余时间 ---")
	timerId5 := tm.NewTimer(800, nil, func(data any) {
		fmt.Println("定时器5触发")
	})

	time.Sleep(200 * time.Millisecond)
	remaining := tm.GetRemainingTime(timerId5)
	fmt.Printf("定时器 ID: %d 剩余时间: %dms\n", timerId5, remaining)

	// 示例6: 手动触发定时器
	fmt.Println("\n--- 示例6: 手动触发定时器 ---")
	var wg6 sync.WaitGroup
	wg6.Add(1)

	timerId6 := tm.NewTimer(1000, "Manual Trigger", func(data any) {
		fmt.Printf("手动触发定时器! 数据: %v\n", data)
		wg6.Done()
	})

	fmt.Printf("创建定时器 ID: %d, 将在1000ms后触发\n", timerId6)
	time.Sleep(100 * time.Millisecond)
	fmt.Println("手动触发定时器...")
	if tm.TriggerTimer(timerId6) {
		fmt.Println("定时器手动触发成功")
	}

	wg6.Wait()

	// 示例7: 并发定时器
	fmt.Println("\n--- 示例7: 并发定时器 ---")
	var wg7 sync.WaitGroup
	numTimers := 5

	wg7.Add(numTimers)
	for i := 1; i <= numTimers; i++ {
		id := i
		delay := int64(i * 100) // 100ms, 200ms, 300ms, 400ms, 500ms

		tm.NewTimer(delay, id, func(data any) {
			timerID := data.(int)
			fmt.Printf("并发定时器 #%d 触发 (延迟: %dms)\n", timerID, delay)
			wg7.Done()
		})
	}

	wg7.Wait()

	// 示例8: 无限重复定时器（带停止）
	fmt.Println("\n--- 示例8: 无限重复定时器 ---")
	repeatCount := 0
	var timerId8 int64
	timerId8 = tm.NewRepeatTimer(100, 0, nil, func(data any) {
		repeatCount++
		fmt.Printf("无限重复定时器触发 #%d\n", repeatCount)
		if repeatCount >= 5 {
			// 达到5次后取消
			tm.CancelTimer(timerId8)
			fmt.Println("已取消无限重复定时器")
		}
	})

	fmt.Printf("创建无限重复定时器 ID: %d, 间隔100ms\n", timerId8)
	time.Sleep(600 * time.Millisecond) // 等待足够时间触发5次

	// 示例9: 获取定时器数量
	fmt.Println("\n--- 示例9: 定时器统计 ---")
	// 创建一些定时器
	for i := 0; i < 3; i++ {
		tm.NewTimer(1000+int64(i*100), i, func(data any) {})
	}
	count := tm.GetTimerCount()
	fmt.Printf("当前活跃定时器数量: %d\n", count)

	// 清除所有定时器
	tm.ClearAll()
	count = tm.GetTimerCount()
	fmt.Printf("清除后定时器数量: %d\n", count)

	fmt.Println("\n=== 所有示例完成 ===")
	time.Sleep(500 * time.Millisecond) // 等待所有goroutine完成
}
