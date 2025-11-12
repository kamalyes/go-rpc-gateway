/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2025-11-07 18:30:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-11-12 01:04:30
 * @FilePath: \go-rpc-gateway\middleware\pprof_scenarios.go
 * @Description: pprof性能测试场景 - 提供各种性能测试场景用于分析
 *
 * Copyright (c) 2024 by kamalyes, All Rights Reserved.
 */

package middleware

import (
	crand "crypto/rand"
	"math/rand"
	"net/http"
	"runtime"
	"sync"
	"time"
	"unsafe"

	"github.com/kamalyes/go-config/pkg/pprof"
	"github.com/kamalyes/go-rpc-gateway/response"
)

// PProfScenarios pprof测试场景集合
type PProfScenarios struct {
	longLivedObjects []*struct{} // 长生命周期对象存储
}

// NewPProfScenarios 创建新的pprof测试场景实例
func NewPProfScenarios() *PProfScenarios {
	return &PProfScenarios{
		longLivedObjects: make([]*struct{}, 0),
	}
}

// RegisterScenarios 注册所有测试场景到pprof配置 (兼容旧接口)
func (ps *PProfScenarios) RegisterScenarios(config *pprof.PProf) {
	// 这个方法保持为空，因为现在使用适配器模式
}

// RegisterScenariosToAdapter 注册所有测试场景到pprof适配器
func (ps *PProfScenarios) RegisterScenariosToAdapter(adapter *PProfConfigAdapter) {
	if adapter.CustomHandlers == nil {
		adapter.CustomHandlers = make(map[string]http.HandlerFunc)
	}

	// 注册各种GC场景
	adapter.RegisterCustomHandler("gc/small-objects", ps.SimulateSmallObjectsGC)
	adapter.RegisterCustomHandler("gc/large-objects", ps.SimulateLargeObjectsGC)
	adapter.RegisterCustomHandler("gc/high-cpu", ps.SimulateHighCPUUsageGC)
	adapter.RegisterCustomHandler("gc/cyclic-objects", ps.SimulateCyclicGC)
	adapter.RegisterCustomHandler("gc/short-lived-objects", ps.SimulateShortLivedObjectsGC)
	adapter.RegisterCustomHandler("gc/long-lived-objects", ps.SimulateLongLivedObjectsGC)
	adapter.RegisterCustomHandler("gc/complex-structure", ps.SimulateComplexStructureGC)
	adapter.RegisterCustomHandler("gc/concurrent", ps.SimulateConcurrentGC)

	// 注册内存测试场景
	adapter.RegisterCustomHandler("memory/allocate", ps.SimulateMemoryAllocate)
	adapter.RegisterCustomHandler("memory/leak", ps.SimulateMemoryLeak)
	adapter.RegisterCustomHandler("memory/fragmentation", ps.SimulateMemoryFragmentation)

	// 注册CPU测试场景
	adapter.RegisterCustomHandler("cpu/intensive", ps.SimulateCPUIntensive)
	adapter.RegisterCustomHandler("cpu/recursive", ps.SimulateCPURecursive)

	// 注册并发测试场景
	adapter.RegisterCustomHandler("goroutine/spawn", ps.SimulateGoroutineSpawn)
	adapter.RegisterCustomHandler("goroutine/leak", ps.SimulateGoroutineLeak)
	adapter.RegisterCustomHandler("mutex/contention", ps.SimulateMutexContention)

	// 注册清理场景
	adapter.RegisterCustomHandler("cleanup/all", ps.CleanupAll)
}

// SimulateSmallObjectsGC 模拟大量小对象的GC场景
func (ps *PProfScenarios) SimulateSmallObjectsGC(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	objects := make([]*struct{}, 0, 100000)

	for i := 0; i < 100000; i++ {
		obj := struct{}{}
		objects = append(objects, &obj)
	}
	_ = objects
	duration := time.Since(start)
	response.WriteSuccessResult(w, "Triggered GC after creating 100000 small objects! Duration: "+duration.String())
}

// SimulateLargeObjectsGC 模拟大对象的GC场景
func (ps *PProfScenarios) SimulateLargeObjectsGC(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	objects := make([][]byte, 0, 1000)

	for i := 0; i < 1000; i++ {
		// 创建一个大对象，大小为1MB
		obj := make([]byte, 1<<20) // 1 MB
		// 填充对象以确保它不会被优化掉
		if _, err := crand.Read(obj); err != nil {
			// 在实际应用中应处理错误
		}
		objects = append(objects, obj)
	}
	_ = objects // 标记为已使用，避免linter警告

	duration := time.Since(start)
	response.WriteSuccessResult(w, "Triggered GC after creating 1000 large objects! Duration: "+duration.String())
}

// SimulateHighCPUUsageGC 模拟高CPU使用的场景
func (ps *PProfScenarios) SimulateHighCPUUsageGC(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	const iterations = 1_000_000
	var result int64
	var wg sync.WaitGroup

	// 启动多个goroutine来增加CPU使用率
	numWorkers := 4
	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				result += int64(rand.Intn(100)) // 进行一些计算，增加CPU使用率
			}
		}()
	}

	wg.Wait() // 等待所有goroutine完成
	duration := time.Since(start)

	response.WriteSuccessResult(w, "Triggered high CPU usage! Duration: "+duration.String())
}

// SimulateCyclicGC 模拟周期性GC场景
func (ps *PProfScenarios) SimulateCyclicGC(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	type Node struct {
		next *Node
		// data [100]byte // 增加节点大小
	}

	head := &Node{}
	current := head

	nodeCount := 10000
	for i := 0; i < nodeCount; i++ {
		current.next = &Node{}
		current = current.next
	}

	// 创建循环引用
	current.next = head

	duration := time.Since(start)
	response.WriteSuccessResult(w, "Triggered GC after creating cyclic objects! Duration: "+duration.String())
}

// SimulateShortLivedObjectsGC 模拟短生命周期对象的GC场景
func (ps *PProfScenarios) SimulateShortLivedObjectsGC(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	objectCount := 100000

	for i := 0; i < objectCount; i++ {
		_ = struct{}{} // 创建一个短生命周期的对象
	}

	duration := time.Since(start)
	response.WriteSuccessResult(w, "Triggered GC after creating many short-lived objects! Duration: "+duration.String())
}

// SimulateLongLivedObjectsGC 模拟长时间持有对象的GC场景
func (ps *PProfScenarios) SimulateLongLivedObjectsGC(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	objectCount := 1000

	for i := 0; i < objectCount; i++ {
		obj := struct{}{}
		ps.longLivedObjects = append(ps.longLivedObjects, &obj) // 持有对象的引用
	}

	duration := time.Since(start)
	response.WriteSuccessResult(w, "Triggered GC after holding many long-lived objects! Duration: "+duration.String())
}

// TreeNode 树节点结构
type TreeNode struct {
	value int
	left  *TreeNode
	right *TreeNode
	// data  [256]byte // 增加节点大小
}

// SimulateComplexStructureGC 模拟复杂数据结构的GC场景
func (ps *PProfScenarios) SimulateComplexStructureGC(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	root := &TreeNode{value: 0}
	current := root
	nodeCount := 1000

	for i := 1; i < nodeCount; i++ {
		node := &TreeNode{value: i}
		if i%2 == 0 {
			current.left = node
		} else {
			current.right = node
		}
		current = node
	}

	duration := time.Since(start)
	response.WriteSuccessResult(w, "Triggered GC after creating a complex tree structure! Duration: "+duration.String())
}

// SimulateConcurrentGC 模拟并发场景的GC场景
func (ps *PProfScenarios) SimulateConcurrentGC(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	var wg sync.WaitGroup
	numWorkers := 10
	objectsPerWorker := 10000

	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < objectsPerWorker; j++ {
				_ = struct{}{} // 创建短生命周期对象
			}
		}()
	}
	wg.Wait()

	duration := time.Since(start)
	response.WriteSuccessResult(w, "Triggered GC after concurrent creation of many objects! Duration: "+duration.String())
}

// SimulateMemoryAllocate 模拟内存分配
func (ps *PProfScenarios) SimulateMemoryAllocate(w http.ResponseWriter, r *http.Request) {
	start := time.Now()

	// 分配不同大小的内存块
	var allocations [][]byte
	sizes := []int{1024, 4096, 65536, 1048576} // 1KB, 4KB, 64KB, 1MB

	for _, size := range sizes {
		for i := 0; i < 100; i++ {
			allocation := make([]byte, size)
			allocations = append(allocations, allocation)
		}
	}

	duration := time.Since(start)
	response.WriteSuccessResult(w, "Memory allocation test completed! Duration: "+duration.String())
}

// SimulateMemoryLeak 模拟内存泄漏（注意：这只是演示，不是真正的泄漏）
func (ps *PProfScenarios) SimulateMemoryLeak(w http.ResponseWriter, r *http.Request) {
	start := time.Now()

	// 创建一些不释放的内存
	for i := 0; i < 1000; i++ {
		data := make([]byte, 10240) // 10KB
		ps.longLivedObjects = append(ps.longLivedObjects, (*struct{})(unsafe.Pointer(&data[0])))
	}

	duration := time.Since(start)
	response.WriteSuccessResult(w, "Memory leak simulation completed! Duration: "+duration.String())
}

// SimulateMemoryFragmentation 模拟内存碎片化
func (ps *PProfScenarios) SimulateMemoryFragmentation(w http.ResponseWriter, r *http.Request) {
	start := time.Now()

	var allocations [][]byte

	// 分配大量不同大小的内存块来产生碎片
	for i := 0; i < 10000; i++ {
		size := rand.Intn(8192) + 1 // 1B to 8KB
		allocation := make([]byte, size)
		allocations = append(allocations, allocation)

		// 随机释放一些内存
		if i%3 == 0 && len(allocations) > 100 {
			// 移除中间的元素来模拟碎片化
			idx := rand.Intn(len(allocations))
			allocations = append(allocations[:idx], allocations[idx+1:]...)
		}
	}

	duration := time.Since(start)
	response.WriteSuccessResult(w, "Memory fragmentation test completed! Duration: "+duration.String())
}

// SimulateCPUIntensive 模拟CPU密集型操作
func (ps *PProfScenarios) SimulateCPUIntensive(w http.ResponseWriter, r *http.Request) {
	start := time.Now()

	// 执行一些CPU密集型计算
	var result float64
	for i := 0; i < 10000000; i++ {
		result += float64(i) * 3.14159 / 2.71828
	}

	duration := time.Since(start)
	response.WriteSuccessResult(w, "CPU intensive test completed! Duration: "+duration.String())
}

// fibonacci 递归计算斐波那契数列
func fibonacci(n int) int {
	if n <= 1 {
		return n
	}
	return fibonacci(n-1) + fibonacci(n-2)
}

// SimulateCPURecursive 模拟递归CPU使用
func (ps *PProfScenarios) SimulateCPURecursive(w http.ResponseWriter, r *http.Request) {
	start := time.Now()

	// 计算一个较大的斐波那契数
	_ = fibonacci(35)

	duration := time.Since(start)
	response.WriteSuccessResult(w, "CPU recursive test completed! Duration: "+duration.String())
}

// SimulateGoroutineSpawn 模拟大量Goroutine创建
func (ps *PProfScenarios) SimulateGoroutineSpawn(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	var wg sync.WaitGroup
	numGoroutines := 10000

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			time.Sleep(time.Millisecond * 10) // 短暂睡眠
		}(i)
	}
	wg.Wait()

	duration := time.Since(start)
	response.WriteSuccessResult(w, "Goroutine spawn test completed! Duration: "+duration.String())
}

// SimulateGoroutineLeak 模拟Goroutine泄漏
func (ps *PProfScenarios) SimulateGoroutineLeak(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	numLeakedGoroutines := 100

	for i := 0; i < numLeakedGoroutines; i++ {
		go func() {
			// 无限循环，模拟goroutine泄漏
			select {}
		}()
	}

	duration := time.Since(start)
	response.WriteSuccessResult(w, "Goroutine leak simulation completed! Duration: "+duration.String())
}

// SimulateMutexContention 模拟互斥锁竞争
func (ps *PProfScenarios) SimulateMutexContention(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	var mu sync.Mutex
	var counter int
	var wg sync.WaitGroup
	numWorkers := 50
	iterations := 10000

	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				mu.Lock()
				counter++
				time.Sleep(time.Microsecond) // 增加锁持有时间
				mu.Unlock()
			}
		}()
	}
	wg.Wait()

	duration := time.Since(start)
	response.WriteSuccessResult(w, "Mutex contention test completed! Duration: "+duration.String())
}

// CleanupAll 清理所有持有的对象
func (ps *PProfScenarios) CleanupAll(w http.ResponseWriter, r *http.Request) {
	start := time.Now()

	// 清理长生命周期对象
	ps.longLivedObjects = ps.longLivedObjects[:0]

	// 手动触发GC
	runtime.GC()
	runtime.GC() // 连续调用两次确保清理

	duration := time.Since(start)
	response.WriteSuccessResult(w, "Cleanup completed! Duration: "+duration.String())
}
