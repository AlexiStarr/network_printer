package main

import (
	"encoding/csv"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// 运行基础吞吐量测试
func runBaseLoad(d time.Duration, concurrency int) (int64, int64) {
	var totalReqs int64
	var errCount int64
	var wg sync.WaitGroup
	var mu sync.Mutex

	deadline := time.Now().Add(d)

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			var lreqs, lerrs int64
			client := http.Client{
				Transport: &http.Transport{
					MaxIdleConnsPerHost: 100,
				},
				Timeout: 2 * time.Second,
			}
			for time.Now().Before(deadline) {
				req, _ := http.NewRequest("GET", "http://127.0.0.1:8080/api/status", nil)
				resp, err := client.Do(req)
				if err != nil {
					lerrs++
				} else {
					io.Copy(io.Discard, resp.Body)
					resp.Body.Close()
					lreqs++
				}
			}
			mu.Lock()
			totalReqs += lreqs
			errCount += lerrs
			mu.Unlock()
		}()
	}

	wg.Wait()
	return totalReqs, errCount
}

func main() {
	var maxClients int
	var step int
	var outDir string
	flag.IntVar(&maxClients, "m", 1000, "Max WebSocket clients to simulate")
	flag.IntVar(&step, "s", 200, "Increase step for clients")
	flag.StringVar(&outDir, "o", "../test_results", "Output directory")
	flag.Parse()

	os.MkdirAll(outDir, 0755)

	fmt.Printf("=== 6.3.4 并发扩展性测试 ===\n")
	fmt.Printf("最大 WebSocket 客户端数: %d, 步长: %d\n", maxClients, step)

	f, _ := os.Create(fmt.Sprintf("%s/scale.csv", outDir))
	writer := csv.NewWriter(f)
	writer.Write([]string{"WSClients", "QPS", "Errors"})
	defer f.Close()

	var wsConns []*websocket.Conn

	// Add step 0 as baseline
	steps := []int{0}
	for i := step; i <= maxClients; i += step {
		steps = append(steps, i)
	}

	for _, clientCount := range steps {
		// Calculate how many we need to add
		toAdd := clientCount - len(wsConns)
		if toAdd > 0 {
			fmt.Printf("\n[+] 建立 %d 个新的 WebSocket 连接...\n", toAdd)
			for i := 0; i < toAdd; i++ {
				c, _, err := websocket.DefaultDialer.Dial("ws://127.0.0.1:8080/ws", nil)
				if err != nil {
					fmt.Printf("[-] 建立 WebSocket 失败 (目前总数 %d): %v\n", len(wsConns), err)
					break
				}
				wsConns = append(wsConns, c)
				// Reader goroutine to keep alive
				go func(conn *websocket.Conn) {
					for {
						if _, _, err := conn.ReadMessage(); err != nil {
							return
						}
					}
				}(c)
			}
			time.Sleep(2 * time.Second) // wait for server to settle
		}

		fmt.Printf("[*] 当前活跃 WebSocket: %d, 执行吞吐量测试 (10s)...\n", len(wsConns))

		d := 10 * time.Second
		reqs, errs := runBaseLoad(d, 50) // 50 concurrent HTTP pollers
		qps := float64(reqs) / d.Seconds()

		fmt.Printf("    -> QPS: %.2f, 错误数: %d\n", qps, errs)
		writer.Write([]string{
			fmt.Sprintf("%d", len(wsConns)),
			fmt.Sprintf("%.2f", qps),
			fmt.Sprintf("%d", errs),
		})
		writer.Flush()
	}

	// Clean up
	for _, c := range wsConns {
		c.Close()
	}

	fmt.Println("\n=> 并发扩展性测试完成！结果已保存至 test_results/scale.csv")
}
