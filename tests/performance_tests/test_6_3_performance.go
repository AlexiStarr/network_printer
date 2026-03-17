package main

import (
	"bytes"
	"encoding/binary"
	"encoding/csv"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"sort"
	"sync"
	"time"
)

// ========= 协议约定 =========
const (
	ProtocolMagic         = 0xDEADBEEF
	CmdGetStatus    uint8 = 0x01
	ProtocolVersion uint8 = 1
)

type ProtocolHeader struct {
	Magic    uint32
	Version  uint8
	Command  uint8
	Length   uint16
	Sequence uint32
}

func CalculateChecksum(data []byte) uint32 {
	var sum uint32
	for _, b := range data {
		sum += uint32(b)
		sum = (sum << 1) | (sum >> 31)
	}
	return sum ^ ProtocolMagic
}

func encodeStatusRequest(payloadSize int) []byte {
	// mock payload up to payloadSize
	var payload []byte
	if payloadSize > 12 {
		payloadSize = payloadSize - 12 - 4 // minus header and checksum
		if payloadSize < 0 {
			payloadSize = 0
		}
		payload = make([]byte, payloadSize)
	}

	hdr := ProtocolHeader{
		Magic:    ProtocolMagic,
		Version:  ProtocolVersion,
		Command:  CmdGetStatus,
		Length:   uint16(len(payload)),
		Sequence: 1,
	}

	buf := new(bytes.Buffer)
	binary.Write(buf, binary.LittleEndian, hdr)
	headerBytes := buf.Bytes()

	req := new(bytes.Buffer)
	req.Write(headerBytes)
	req.Write(payload)

	checksum := CalculateChecksum(req.Bytes())
	binary.Write(req, binary.LittleEndian, checksum)
	return req.Bytes()
}

// ========= 核心测评引擎 =========
type Result struct {
	Latencies []time.Duration
	BytesRead int64
	Errors    int64
}

// 6.3.2 运行吞吐量测试 (固定时间 30s)
func runThroughputTest(name string, duration time.Duration, concurrency int, runFunc func() (int, error)) (float64, float64) {
	fmt.Printf("\n[+] 运行吞吐量测试: %s (持续: %v, 并发: %d)\n", name, duration, concurrency)

	var totalReqs int64
	var totalBytes int64
	var wg sync.WaitGroup
	quit := make(chan struct{})

	// start := time.Now() // Wait for signals
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-quit:
					return
				default:
					b, err := runFunc()
					if err == nil {
						// atomic counters are safer but for max speed we can local batch
					}
					// simplify lockless approx
					_ = err
					_ = b
				}
			}
		}()
	}

	// But lockless is harder to collect accurately without sync/atomic. Let's do a channel/atomic approach.
	// Actually we should just use workers that return counts.
	close(quit)
	wg.Wait()
	return float64(totalReqs), float64(totalBytes)
}

// Refactoring: Use a proper worker pool for accuracy
func runBenchmarkTimer(name string, d time.Duration, workers int, payloadSize int, mode string) (int64, int64, int64) {
	fmt.Printf("[+] 运行 %s 吞吐量测试 (%v, %d bytes) ... ", name, d, payloadSize)

	reqData := encodeStatusRequest(payloadSize)
	var totalReqs, totalBytes, errCount int64
	var mu sync.Mutex

	var wg sync.WaitGroup
	deadline := time.Now().Add(d)

	tr := &http.Transport{
		MaxIdleConns:        1000,
		MaxIdleConnsPerHost: workers,
		IdleConnTimeout:     30 * time.Second,
	}
	sharedClient := &http.Client{
		Timeout:   2 * time.Second,
		Transport: tr,
	}

	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			var lreqs, lbytes, lerrs int64

			if mode == "binary" {
				conn, err := net.Dial("tcp", "127.0.0.1:9999")
				if err != nil {
					fmt.Printf("[Error] Binary dial failed: %v\n", err)
					return
				}
				defer conn.Close()
				buf := make([]byte, 4096)

				for time.Now().Before(deadline) {
					_, werr := conn.Write(reqData)
					if werr != nil {
						fmt.Printf("[Error] Binary write failed: %v\n", werr)
					}
					n, err := conn.Read(buf)
					if err != nil {
						fmt.Printf("[Error] Binary read failed: %v\n", err)
						lerrs++
						conn.Close()
						conn, _ = net.Dial("tcp", "127.0.0.1:9999")
						if conn == nil {
							break
						}
					} else {
						lreqs++
						lbytes += int64(len(reqData) + n)
					}
				}
			} else {
				// use shared client
				// create mock payload for JSON
				jsonStr := fmt.Sprintf(`{"padding":"%s"}`, string(make([]byte, payloadSize/2)))
				reqBytes := []byte(jsonStr)

				for time.Now().Before(deadline) {
					req, _ := http.NewRequest("GET", "http://127.0.0.1:8081/api/status", bytes.NewBuffer(reqBytes))
					resp, err := sharedClient.Do(req)
					if err != nil {
						lerrs++
					} else {
						io.Copy(io.Discard, resp.Body)
						resp.Body.Close()
						lreqs++
						lbytes += int64(len(reqBytes) + 500) // approx resp
					}
				}
			}

			mu.Lock()
			totalReqs += lreqs
			totalBytes += lbytes
			errCount += lerrs
			mu.Unlock()
		}()
	}

	wg.Wait()
	qps := totalReqs / int64(d.Seconds())
	mbs := float64(totalBytes) / d.Seconds() / 1024 / 1024
	fmt.Printf("%d req/s, %.2f MB/s, %d errors\n", qps, mbs, errCount)
	return qps, totalBytes, errCount
}

func calculatePercentiles(latencies []time.Duration) (p50, p95, p99, p999 time.Duration) {
	if len(latencies) == 0 {
		return 0, 0, 0, 0
	}
	sort.Slice(latencies, func(i, j int) bool {
		return latencies[i] < latencies[j]
	})
	p50 = latencies[int(float64(len(latencies))*0.50)]
	p95 = latencies[int(float64(len(latencies))*0.95)]
	p99 = latencies[int(float64(len(latencies))*0.99)]
	p999 = latencies[int(float64(len(latencies))*0.999)]
	return
}

// 6.3.3 运行延迟测试 (固定请求数 100,000)
func runLatencyBenchmark(name string, totalRequests int, workers int, mode string, outPath string) []time.Duration {
	fmt.Printf("[+] 运行 %s 延迟测试 (样本量: %d) ... ", name, totalRequests)

	reqData := encodeStatusRequest(256)
	latencies := make(chan time.Duration, totalRequests)
	var wg sync.WaitGroup
	var errCount int64
	var mu sync.Mutex

	jobs := make(chan int, totalRequests)
	for i := 0; i < totalRequests; i++ {
		jobs <- i
	}
	close(jobs)

	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			var conn net.Conn
			var client *http.Client

			if mode == "binary" {
				conn, _ = net.Dial("tcp", "127.0.0.1:9999")
				if conn != nil {
					defer conn.Close()
				}
			} else {
				client = &http.Client{
					Transport: &http.Transport{
						MaxIdleConnsPerHost: 100,
					},
					Timeout: 5 * time.Second,
				}
			}

			buf := make([]byte, 4096)

			for range jobs {
				start := time.Now()
				if mode == "binary" {
					if conn == nil {
						continue
					}
					conn.Write(reqData)
					_, err := conn.Read(buf)
					if err != nil {
						mu.Lock()
						errCount++
						mu.Unlock()
						conn.Close()
						conn, _ = net.Dial("tcp", "127.0.0.1:9999")
					} else {
						latencies <- time.Since(start)
					}
				} else {
					req, _ := http.NewRequest("GET", "http://127.0.0.1:8081/api/status", nil)
					resp, err := client.Do(req)
					if err != nil {
						mu.Lock()
						errCount++
						mu.Unlock()
					} else {
						io.Copy(io.Discard, resp.Body)
						resp.Body.Close()
						latencies <- time.Since(start)
					}
				}
			}
		}()
	}

	wg.Wait()
	close(latencies)

	var results []time.Duration
	for l := range latencies {
		results = append(results, l)
	}

	p50, p95, p99, p999 := calculatePercentiles(results)
	fmt.Printf("P50: %v, P95: %v, P99: %v, P99.9: %v (Errors: %d)\n", p50, p95, p99, p999, errCount)

	// Dump specific percentiles info to csv
	f, _ := os.OpenFile(outPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	defer f.Close()
	writer := csv.NewWriter(f)
	writer.Write([]string{
		name,
		fmt.Sprintf("%.3f", float64(p50.Microseconds())/1000.0),
		fmt.Sprintf("%.3f", float64(p95.Microseconds())/1000.0),
		fmt.Sprintf("%.3f", float64(p99.Microseconds())/1000.0),
		fmt.Sprintf("%.3f", float64(p999.Microseconds())/1000.0),
	})
	writer.Flush()

	return results
}

func main() {
	var workers int
	var duration int
	var outDir string
	flag.IntVar(&workers, "c", 100, "concurrency level")
	flag.IntVar(&duration, "d", 10, "duration in seconds (for throughput test)")
	flag.StringVar(&outDir, "o", "../test_results", "output directory for csv")
	flag.Parse()

	os.MkdirAll(outDir, 0755)

	fmt.Println("=== 6.3 性能基准对比测试 (Binary vs JSON over HTTP) ===")

	// 6.3.2 吞吐量测试
	fmt.Println("\n--- 6.3.2 吞吐量对比测试 ---")
	throughputCsv := fmt.Sprintf("%s/throughput.csv", outDir)
	f, _ := os.Create(throughputCsv)
	tf := csv.NewWriter(f)
	tf.Write([]string{"Protocol", "PayloadSize", "QPS", "MB_s"})

	payloads := []int{128, 256, 512}

	for _, size := range payloads {
		// Binary
		qpsB, tbB, _ := runBenchmarkTimer("Binary", time.Duration(duration)*time.Second, workers, size, "binary")
		mbB := float64(tbB) / float64(duration) / 1024 / 1024
		tf.Write([]string{"Binary", fmt.Sprintf("%d", size), fmt.Sprintf("%d", qpsB), fmt.Sprintf("%.2f", mbB)})

		// JSON
		qpsJ, tbJ, _ := runBenchmarkTimer("JSON_HTTP", time.Duration(duration)*time.Second, workers, size, "json")
		mbJ := float64(tbJ) / float64(duration) / 1024 / 1024
		tf.Write([]string{"JSON_HTTP", fmt.Sprintf("%d", size), fmt.Sprintf("%d", qpsJ), fmt.Sprintf("%.2f", mbJ)})
	}
	tf.Flush()
	f.Close()

	// 6.3.3 延迟分位数对比测试
	fmt.Println("\n--- 6.3.3 延迟分位数对比测试 ---")
	latencyCsv := fmt.Sprintf("%s/latency.csv", outDir)
	lf, _ := os.Create(latencyCsv)
	lf.WriteString("Protocol,P50_ms,P95_ms,P99_ms,P99.9_ms\n")
	lf.Close() // Will append in func

	// 100k requests each
	runLatencyBenchmark("Binary", 100000, workers, "binary", latencyCsv)
	runLatencyBenchmark("JSON_HTTP", 100000, workers, "json", latencyCsv)

	fmt.Println("\n=> 性能测试完成！结果已写入", outDir)
}
