package main

import (
	"encoding/csv"
	"flag"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"os"
	"time"

	"slices"
	"sync"

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

func buildLevels(maxClients, step int) []int {
	levels := []int{0}
	for i := step; i <= maxClients; i += step {
		levels = append(levels, i)
	}
	return levels
}

func buildMeasurementOrder(levels []int, order string, r *rand.Rand) []int {
	seq := slices.Clone(levels)
	switch order {
	case "random":
		r.Shuffle(len(seq), func(i, j int) {
			seq[i], seq[j] = seq[j], seq[i]
		})
		return seq
	case "symmetric":
		if len(seq) <= 1 {
			return seq
		}
		desc := make([]int, 0, len(seq)-1)
		for i := len(seq) - 2; i >= 0; i-- {
			desc = append(desc, seq[i])
		}
		return append(seq, desc...)
	default:
		return seq
	}
}

func adjustWSConnections(wsConns *[]*websocket.Conn, target int) {
	cur := len(*wsConns)

	if target > cur {
		toAdd := target - cur
		fmt.Printf("\n[+] 建立 %d 个新的 WebSocket 连接...\n", toAdd)
		for i := 0; i < toAdd; i++ {
			c, _, err := websocket.DefaultDialer.Dial("ws://127.0.0.1:8080/ws", nil)
			if err != nil {
				fmt.Printf("[-] 建立 WebSocket 失败 (目前总数 %d): %v\n", len(*wsConns), err)
				break
			}
			*wsConns = append(*wsConns, c)
			go func(conn *websocket.Conn) {
				for {
					if _, _, err := conn.ReadMessage(); err != nil {
						return
					}
				}
			}(c)
		}
		return
	}

	if target < cur {
		toClose := cur - target
		fmt.Printf("\n[-] 关闭 %d 个 WebSocket 连接...\n", toClose)
		for i := cur - 1; i >= target; i-- {
			_ = (*wsConns)[i].Close()
		}
		*wsConns = (*wsConns)[:target]
	}
}

func runLevel(level int, wsConns *[]*websocket.Conn, settle time.Duration, duration time.Duration, pollers int) (float64, int64) {
	adjustWSConnections(wsConns, level)
	time.Sleep(settle)

	fmt.Printf("[*] 当前活跃 WebSocket: %d, 执行吞吐量测试 (%s)...\n", len(*wsConns), duration)
	reqs, errs := runBaseLoad(duration, pollers)
	qps := float64(reqs) / duration.Seconds()
	fmt.Printf("    -> QPS: %.2f, 错误数: %d\n", qps, errs)
	return qps, errs
}

func main() {
	var maxClients int
	var step int
	var outDir string
	var durationSec int
	var pollers int
	var warmupSec int
	var settleMs int
	var order string
	var seed int64
	flag.IntVar(&maxClients, "m", 1000, "Max WebSocket clients to simulate")
	flag.IntVar(&step, "s", 200, "Increase step for clients")
	flag.StringVar(&outDir, "o", "../test_results", "Output directory")
	flag.IntVar(&durationSec, "d", 10, "Measurement duration in seconds")
	flag.IntVar(&pollers, "c", 50, "Concurrent HTTP pollers")
	flag.IntVar(&warmupSec, "warmup", 10, "Warmup duration in seconds (discarded)")
	flag.IntVar(&settleMs, "settle-ms", 2000, "Settle time in milliseconds after each level change")
	flag.StringVar(&order, "order", "random", "Measurement order: random | symmetric | asc")
	flag.Int64Var(&seed, "seed", time.Now().UnixNano(), "Random seed for order=random")
	flag.Parse()

	if step <= 0 {
		fmt.Println("参数错误: -s 必须 > 0")
		return
	}
	if maxClients < 0 {
		fmt.Println("参数错误: -m 不能 < 0")
		return
	}
	if durationSec <= 0 || warmupSec <= 0 {
		fmt.Println("参数错误: -d 与 -warmup 必须 > 0")
		return
	}
	if order != "random" && order != "symmetric" && order != "asc" {
		fmt.Println("参数错误: -order 仅支持 random | symmetric | asc")
		return
	}

	os.MkdirAll(outDir, 0755)

	fmt.Printf("=== 6.3.4 并发扩展性测试 ===\n")
	fmt.Printf("最大 WebSocket 客户端数: %d, 步长: %d\n", maxClients, step)
	fmt.Printf("测量配置: duration=%ds, pollers=%d, warmup=%ds, settle=%dms\n", durationSec, pollers, warmupSec, settleMs)
	fmt.Printf("顺序策略: %s (seed=%d)\n", order, seed)

	f, err := os.Create(fmt.Sprintf("%s/scale.csv", outDir))
	if err != nil {
		fmt.Printf("创建输出文件失败: %v\n", err)
		return
	}
	writer := csv.NewWriter(f)
	_ = writer.Write([]string{"WSClients", "QPS", "Errors", "Order", "Seed"})
	defer f.Close()

	var wsConns []*websocket.Conn

	levels := buildLevels(maxClients, step)
	settle := time.Duration(settleMs) * time.Millisecond
	warmupDur := time.Duration(warmupSec) * time.Second
	measureDur := time.Duration(durationSec) * time.Second
	r := rand.New(rand.NewSource(seed))
	measurementOrder := buildMeasurementOrder(levels, order, r)

	fmt.Println("\n--- 预热轮次（结果丢弃） ---")
	for _, level := range levels {
		_, _ = runLevel(level, &wsConns, settle, warmupDur, pollers)
	}

	fmt.Println("\n--- 正式测量 ---")
	fmt.Printf("测量顺序: %v\n", measurementOrder)
	for _, level := range measurementOrder {
		qps, errs := runLevel(level, &wsConns, settle, measureDur, pollers)
		_ = writer.Write([]string{
			fmt.Sprintf("%d", len(wsConns)),
			fmt.Sprintf("%.2f", qps),
			fmt.Sprintf("%d", errs),
			order,
			fmt.Sprintf("%d", seed),
		})
		writer.Flush()
	}

	// Clean up
	for _, c := range wsConns {
		_ = c.Close()
	}

	fmt.Println("\n=> 并发扩展性测试完成！结果已保存至 test_results/scale.csv")
}
