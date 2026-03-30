package main

import (
	"bytes"
	"encoding/binary"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"sort"
	"sync"
	"time"
)

const (
	benchMagic   uint32 = 0xDEADBEEF
	benchVersion uint8  = 1
	benchCmdGet  uint8  = 0x01
)

type Job struct {
	ID    int    `json:"id"`
	Name  string `json:"name"`
	Pages int    `json:"pages"`
}

type ProxyRequest struct {
	Cmd  string `json:"cmd"`
	Jobs []Job  `json:"jobs,omitempty"`
}

func buildBinaryFrame(totalSize int, seq uint32) []byte {
	payloadLen := totalSize - 16
	if payloadLen < 0 {
		payloadLen = 0
	}
	payload := make([]byte, payloadLen)

	buf := new(bytes.Buffer)
	binary.Write(buf, binary.LittleEndian, benchMagic)
	buf.WriteByte(benchVersion)
	buf.WriteByte(benchCmdGet)
	binary.Write(buf, binary.LittleEndian, uint16(payloadLen))
	binary.Write(buf, binary.LittleEndian, seq)
	buf.Write(payload)

	data := buf.Bytes()
	var cs uint32
	for _, b := range data {
		cs += uint32(b)
		cs = (cs << 1) | (cs >> 31)
	}
	cs ^= benchMagic
	binary.Write(buf, binary.LittleEndian, cs)
	return buf.Bytes()
}

func updateBinaryFrameSeq(frame []byte, seq uint32) {
	binary.LittleEndian.PutUint32(frame[8:12], seq)
	var cs uint32
	for _, b := range frame[:len(frame)-4] {
		cs += uint32(b)
		cs = (cs << 1) | (cs >> 31)
	}
	cs ^= benchMagic
	binary.LittleEndian.PutUint32(frame[len(frame)-4:], cs)
}

func readBinaryResponse(conn net.Conn, buf []byte) (int, error) {
	if _, err := io.ReadFull(conn, buf[:12]); err != nil {
		return 0, err
	}
	dataLen := int(binary.LittleEndian.Uint16(buf[6:8]))
	remaining := dataLen + 4
	if 12+remaining > len(buf) {
		return 0, fmt.Errorf("超出缓冲区：%d", 12+remaining)
	}
	if _, err := io.ReadFull(conn, buf[12:12+remaining]); err != nil {
		return 0, err
	}
	return 12 + remaining, nil
}

func buildJSONPayload(targetSize int) []byte {
	req := ProxyRequest{Cmd: "get_status"}
	b, _ := json.Marshal(req)
	for len(b) < targetSize {
		req.Jobs = append(req.Jobs, Job{ID: 100, Name: "doc.pdf", Pages: 50})
		b, _ = json.Marshal(req)
	}
	return b
}

func runThroughput(mode string, duration time.Duration, workers, payloadSize int) (qps int64, mbps float64) {
	var totalReqs, totalBytes int64
	var mu sync.Mutex
	var wg sync.WaitGroup
	deadline := time.Now().Add(duration)

	if mode == "binary" {
		addr := "127.0.0.1:9997"
		for i := 0; i < workers; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				conn, err := net.DialTimeout("tcp", addr, 5*time.Second)
				if err != nil {
					return
				}
				defer conn.Close()

				var lReqs, lBytes int64
				recvBuf := make([]byte, 4096)
				reqFrame := buildBinaryFrame(payloadSize, 0)
				var seq uint32

				for time.Now().Before(deadline) {
					seq++
					updateBinaryFrameSeq(reqFrame, seq)
					if _, err := conn.Write(reqFrame); err != nil {
						return
					}
					n, err := readBinaryResponse(conn, recvBuf)
					if err != nil {
						return
					}
					lReqs++
					lBytes += int64(len(reqFrame) + n)
				}
				mu.Lock()
				totalReqs += lReqs
				totalBytes += lBytes
				mu.Unlock()
			}()
		}
	} else {
		addr := "http://127.0.0.1:9998/api/command"
		tr := &http.Transport{
			MaxIdleConns:        workers,
			MaxIdleConnsPerHost: workers,
			IdleConnTimeout:     90 * time.Second,
		}
		client := &http.Client{Transport: tr, Timeout: 5 * time.Second}
		reqPayload := buildJSONPayload(payloadSize)

		for i := 0; i < workers; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				var lReqs, lBytes int64

				for time.Now().Before(deadline) {
					req, err := http.NewRequest("POST", addr, bytes.NewReader(reqPayload))
					if err != nil {
						return
					}
					req.Header.Set("Content-Type", "application/json")
					resp, err := client.Do(req)
					if err != nil {
						return
					}

					body, _ := io.ReadAll(resp.Body)
					resp.Body.Close()

					lReqs++
					lBytes += int64(len(reqPayload) + len(body) + 100)
				}
				mu.Lock()
				totalReqs += lReqs
				totalBytes += lBytes
				mu.Unlock()
			}()
		}
	}

	wg.Wait()

	secs := int64(duration.Seconds())
	if secs == 0 {
		secs = 1
	}
	qps = totalReqs / secs
	mbps = float64(totalBytes) / duration.Seconds() / 1024 / 1024
	return
}

func runLatency(mode string, totalRequests, workers int) []time.Duration {
	jobs := make(chan struct{}, totalRequests)
	for i := 0; i < totalRequests; i++ {
		jobs <- struct{}{}
	}
	close(jobs)

	latCh := make(chan time.Duration, totalRequests)
	var wg sync.WaitGroup

	if mode == "binary" {
		addr := "127.0.0.1:9997"
		for i := 0; i < workers; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				conn, err := net.DialTimeout("tcp", addr, 5*time.Second)
				if err != nil {
					for range jobs {
					}
					return
				}
				defer conn.Close()

				recvBuf := make([]byte, 4096)
				reqFrame := buildBinaryFrame(256, 0)
				var seq uint32

				for range jobs {
					seq++
					updateBinaryFrameSeq(reqFrame, seq)
					start := time.Now()
					if _, err := conn.Write(reqFrame); err != nil {
						return
					}
					if _, err := readBinaryResponse(conn, recvBuf); err != nil {
						return
					}
					latCh <- time.Since(start)
				}
			}()
		}
	} else {
		addr := "http://127.0.0.1:9998/api/command"
		tr := &http.Transport{
			MaxIdleConns:        workers,
			MaxIdleConnsPerHost: workers,
			IdleConnTimeout:     90 * time.Second,
		}
		client := &http.Client{Transport: tr, Timeout: 5 * time.Second}
		reqPayload := buildJSONPayload(256)

		for i := 0; i < workers; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for range jobs {
					start := time.Now()
					req, _ := http.NewRequest("POST", addr, bytes.NewReader(reqPayload))
					req.Header.Set("Content-Type", "application/json")
					resp, err := client.Do(req)
					if err == nil {
						io.ReadAll(resp.Body)
						resp.Body.Close()
						latCh <- time.Since(start)
					}
				}
			}()
		}
	}

	wg.Wait()
	close(latCh)

	var lats []time.Duration
	for l := range latCh {
		lats = append(lats, l)
	}
	return lats
}

func calcPercentiles(lats []time.Duration) (p50, p95, p99, p999 time.Duration) {
	if len(lats) == 0 {
		return
	}
	sort.Slice(lats, func(i, j int) bool { return lats[i] < lats[j] })
	n := float64(len(lats))
	p50 = lats[int(n*0.50)]
	p95 = lats[int(n*0.95)]
	p99 = lats[int(n*0.99)]
	p999 = lats[int(n*0.999)]
	return
}

func main() {
	var workers = 100
	var duration = 5
	var outDir = "../test_results"

	os.MkdirAll(outDir, 0755)

	fmt.Println("═══════════════════════════════════════════════════════════")
	fmt.Println("  课题 6.3 节：受控性能基准对比测试（优化版）")
	fmt.Println("═══════════════════════════════════════════════════════════")

	tpCSV := outDir + "/throughput.csv"
	tf, _ := os.Create(tpCSV)
	tw := csv.NewWriter(tf)
	tw.Write([]string{"Protocol", "PayloadSize_B", "QPS", "MB_s"})

	fmt.Printf("\n─── 6.3.2 吞吐量测试 (%ds, %d并发) ─────────────────────────\n", duration, workers)
	for _, size := range []int{128, 256, 1024} {
		fmt.Printf("\n  载荷 %4d B\n", size)
		for _, mode := range []string{"binary", "json"} {
			label := "Binary-TCP"
			if mode == "json" {
				label = "JSON-HTTP "
			}
			fmt.Printf("    %-12s ... ", label)
			q, m := runThroughput(mode, time.Duration(duration)*time.Second, workers, size)
			fmt.Printf("%7d req/s  %6.2f MB/s\n", q, m)
			proto := "Binary_TCP"
			if mode == "json" {
				proto = "JSON_HTTP"
			}
			tw.Write([]string{proto, fmt.Sprintf("%d", size), fmt.Sprintf("%d", q), fmt.Sprintf("%.2f", m)})
		}
	}
	tw.Flush()
	tf.Close()
	fmt.Printf("\n✓ 吞吐量 CSV → %s\n", tpCSV)

	fmt.Println("\n─── 6.3.3 延迟分位数对比 ───────────────────────────────────")
	latCSV := outDir + "/latency.csv"
	lf, _ := os.Create(latCSV)
	lw := csv.NewWriter(lf)
	lw.Write([]string{"Protocol", "P50_ms", "P95_ms", "P99_ms", "P999_ms", "Samples"})

	samples := 50000
	fmt.Printf("样本量：%d 次（256B 载荷）\n\n", samples)
	for _, mode := range []string{"binary", "json"} {
		label := "Binary-TCP"
		if mode == "json" {
			label = "JSON-HTTP "
		}
		fmt.Printf("  %-12s ... ", label)
		lats := runLatency(mode, samples, workers)
		p50, p95, p99, p999 := calcPercentiles(lats)
		ms := func(d time.Duration) float64 { return float64(d.Microseconds()) / 1000.0 }
		fmt.Printf("P50=%6.3fms  P95=%6.3fms  P99=%6.3fms  P99.9=%7.3fms  (n=%d)\n",
			ms(p50), ms(p95), ms(p99), ms(p999), len(lats))
		proto := "Binary_TCP"
		if mode == "json" {
			proto = "JSON_HTTP"
		}
		lw.Write([]string{
			proto,
			fmt.Sprintf("%.3f", ms(p50)),
			fmt.Sprintf("%.3f", ms(p95)),
			fmt.Sprintf("%.3f", ms(p99)),
			fmt.Sprintf("%.3f", ms(p999)),
			fmt.Sprintf("%d", len(lats)),
		})
	}
	lw.Flush()
	lf.Close()
	fmt.Printf("\n✓ 延迟 CSV → %s\n", latCSV)
	fmt.Println("\n═══════════════════════════════════════════════════════════")
}
