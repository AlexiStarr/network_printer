package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
)

type JSONHTTPProxy struct {
	listenAddr string
	driverAddr string
	connPool   chan net.Conn
}

func NewJSONHTTPProxy(listenAddr, driverAddr string) *JSONHTTPProxy {
	return &JSONHTTPProxy{
		listenAddr: listenAddr,
		driverAddr: driverAddr,
		connPool:   make(chan net.Conn, 1000), // 大容量连接池
	}
}

type Job struct {
	ID    int    `json:"id"`
	Name  string `json:"name"`
	Pages int    `json:"pages"`
}

type ProxyRequest struct {
	Cmd  string `json:"cmd"`
	Jobs []Job  `json:"jobs,omitempty"`
}

type ProxyResponse struct {
	Status string      `json:"status"`
	Data   interface{} `json:"data,omitempty"`
	Jobs   []Job       `json:"jobs,omitempty"`
}

func (p *JSONHTTPProxy) getDriverConn() (net.Conn, error) {
	select {
	case conn := <-p.connPool:
		return conn, nil
	default:
		return net.Dial("tcp", p.driverAddr)
	}
}

func (p *JSONHTTPProxy) putDriverConn(conn net.Conn) {
	select {
	case p.connPool <- conn:
	default:
		conn.Close()
	}
}

func (p *JSONHTTPProxy) handleCommand(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Only POST", http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Read err", http.StatusBadRequest)
		return
	}
	var req ProxyRequest
	if err := json.Unmarshal(body, &req); err != nil {
		http.Error(w, "JSON err", http.StatusBadRequest)
		return
	}

	driverC, err := p.getDriverConn()
	if err != nil {
		http.Error(w, "Driver unreachable", http.StatusBadGateway)
		return
	}

	cmdFrame := encodeGetStatusRequest(1)
	if _, err := driverC.Write(cmdFrame); err != nil {
		driverC.Close()
		http.Error(w, "Write fail", http.StatusBadGateway)
		return
	}

	respFrame, err := readFullPacket(driverC)
	if err != nil {
		driverC.Close()
		http.Error(w, "Read fail", http.StatusBadGateway)
		return
	}
	p.putDriverConn(driverC) // 连接复用

	resultMap, _ := parseBinaryResponse(respFrame)

	resp := ProxyResponse{
		Status: "ok",
		Data:   resultMap["status"], // parseBinaryResponse returns map[string]interface{}, status is fine
		Jobs:   req.Jobs,
	}

	outHTML, _ := json.Marshal(resp)
	w.Header().Set("Content-Type", "application/json")
	w.Write(outHTML)
}

func (p *JSONHTTPProxy) Start() {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/command", p.handleCommand)
	server := &http.Server{Addr: p.listenAddr, Handler: mux}
	fmt.Printf("[JSONProxy] JSON-over-HTTP 启动：%s → %s\n", p.listenAddr, p.driverAddr)
	if err := server.ListenAndServe(); err != nil {
		log.Fatalf("[JSONProxy] 错误: %v", err)
	}
}
