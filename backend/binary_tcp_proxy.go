package main

import (
	"fmt"
	"io"
	"log"
	"net"
)

type BinaryTCPProxy struct {
	listenAddr string
	driverAddr string
}

func NewBinaryTCPProxy(listenAddr, driverAddr string) *BinaryTCPProxy {
	return &BinaryTCPProxy{
		listenAddr: listenAddr,
		driverAddr: driverAddr,
	}
}

func (p *BinaryTCPProxy) Start() {
	ln, err := net.Listen("tcp", p.listenAddr)
	if err != nil {
		log.Fatalf("[BinaryProxy] 错误: %v", err)
	}
	defer ln.Close()

	fmt.Printf("[BinaryProxy] Binary-TCP 启动：%s → %s\n", p.listenAddr, p.driverAddr)

	for {
		clientConn, err := ln.Accept()
		if err != nil {
			log.Printf("[BinaryProxy] Accept 错误: %v", err)
			continue
		}

		go p.handleConnection(clientConn)
	}
}

func (p *BinaryTCPProxy) handleConnection(clientConn net.Conn) {
	defer clientConn.Close()

	driverConn, err := net.Dial("tcp", p.driverAddr)
	if err != nil {
		log.Printf("[BinaryProxy] 无法连接 C 驱动: %v", err)
		return
	}
	defer driverConn.Close()

	// 简单的全双工透传
	go func() {
		io.Copy(driverConn, clientConn)
	}()
	io.Copy(clientConn, driverConn)
}
