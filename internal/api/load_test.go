package api

import (
	"fmt"
	"net/http"
	"os"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// TestLoad 测试服务器负载能力
func TestLoad(t *testing.T) {
	// 从环境变量获取配置
	token := os.Getenv("TEST_TOKEN")
	if token == "" {
		t.Fatal("TEST_TOKEN environment variable is required")
	}

	concurrentStr := os.Getenv("TEST_CONCURRENT")
	if concurrentStr == "" {
		concurrentStr = "10000"
	}
	concurrent, err := strconv.Atoi(concurrentStr)
	if err != nil {
		t.Fatalf("Invalid TEST_CONCURRENT value: %v", err)
	}

	totalStr := os.Getenv("TEST_TOTAL")
	if totalStr == "" {
		totalStr = "100000"
	}
	total, err := strconv.Atoi(totalStr)
	if err != nil {
		t.Fatalf("Invalid TEST_TOTAL value: %v", err)
	}

	var (
		concurrentRequests = concurrent // 并发请求数
		totalRequests      = total      // 总请求数
	)

	var (
		wg           sync.WaitGroup
		successCount int32
		failureCount int32
		startTime    = time.Now()
	)

	// 准备请求
	req, err := http.NewRequest("GET", "http://localhost:8080/api/health", nil)
	if err != nil {
		t.Fatal(err)
	}

	req.Header.Set("Authorization", "Bearer "+token)

	// 开始压测
	for i := 0; i < totalRequests; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			client := &http.Client{
				Timeout: 5 * time.Second,
			}

			resp, err := client.Do(req)
			if err != nil {
				atomic.AddInt32(&failureCount, 1)
				return
			}

			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				atomic.AddInt32(&failureCount, 1)
				return
			}

			atomic.AddInt32(&successCount, 1)
		}()

		// 控制并发数
		if i%concurrentRequests == 0 && i > 0 {
			wg.Wait()
		}
	}

	// 等待所有请求完成
	wg.Wait()

	// 计算性能指标
	endTime := time.Now()
	duration := endTime.Sub(startTime)
	requestsPerSecond := float64(totalRequests) / duration.Seconds()

	// 输出结果
	fmt.Printf("\nLoad Test Results:\n")
	fmt.Printf("Total Requests: %d\n", totalRequests)
	fmt.Printf("Duration: %v\n", duration)
	fmt.Printf("Requests per Second: %.2f\n", requestsPerSecond)
	fmt.Printf("Success Rate: %.2f%%\n", float64(successCount)*100/float64(totalRequests))
	fmt.Printf("Failure Rate: %.2f%%\n", float64(failureCount)*100/float64(totalRequests))

	// 检查成功率
	if float64(successCount)/float64(totalRequests) < 0.95 {
		t.Errorf("Success rate is too low: %.2f%%", float64(successCount)*100/float64(totalRequests))
	}
}
