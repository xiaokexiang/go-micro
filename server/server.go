package server

import (
	"bufio"
	"bytes"
	"encoding/json"
	"io"
	"log"
	"net"
	"net/http"
	"registry"
	"service"
	"time"
)

type Server struct {
	Registry string           // 注册中心地址
	Time     time.Duration    // 服务注册到注册中心的时间
	Name     string           // server注册的名称，通过名称也能调用
	Service  *service.Service // 注册在server上的方法
}

func NewServer(registry string, time time.Duration, name string, service *service.Service) *Server {
	return &Server{
		Registry: registry,
		Time:     time,
		Name:     name,
		Service:  service,
	}
}

func (s *Server) handleConnection(conn net.Conn) {
	defer func(conn net.Conn) {
		_ = conn.Close()
	}(conn)

	reader := bufio.NewReader(conn)
	writer := bufio.NewWriter(conn)

	str, err := reader.ReadString('\n')
	if err != nil {
		log.Printf("[Server] An error occurred on the server while reading data: %s", err)
		return
	}
	var msg service.Message
	err = json.Unmarshal([]byte(str), &msg)
	if err != nil {
		log.Printf("[Server] An error occurred on the server while unmarshal data: %s", err)
		return
	}
	log.Printf("[Server] Receive message: %s", str)

	if err = s.Service.Exec(&msg); err != nil {
		log.Printf("[Server] An error occurred on the server while call method: %s", err)
		return
	}

	responseData, err := json.Marshal(&msg)
	if err != nil {
		log.Printf("[Server] An error occurred on the server while marshalling JSON: %s", err)
		return
	}

	_, err = writer.WriteString(string(responseData) + "\n")
	if err != nil {
		log.Printf("[Server] An error occurred on the server while writing response: %s", err)
		return
	}
	_ = writer.Flush()
}

func (s *Server) register(addr string) {
	m := map[string]any{
		"name": s.Name,
		"addr": addr,
	}
	body, _ := json.Marshal(m)
	resp, err := http.Post("http://"+s.Registry+"/server", "application/json", bytes.NewBuffer(body))
	if err != nil {
		log.Fatalf("[Server] Register to Registry failed, error: %s", err.Error())
	}
	defer func(Body io.ReadCloser) {
		_ = Body.Close()
	}(resp.Body)
	if err != nil || resp.StatusCode != 200 {
		log.Fatalf("[Server] Register to Registry failed, error: %s, code: %d, resp: %s", err.Error(), resp.StatusCode, resp.Body)
	}
	body, _ = io.ReadAll(resp.Body)
	r := make(map[string]any)
	if err = json.Unmarshal(body, &r); err != nil {
		log.Fatalf("[Server] Register to Registry failed, error: %s", err.Error())
	}
	if r["code"].(float64) != 200 {
		log.Fatalf("[Server] Register to Registry failed, error: %s", r["message"])
	}
	log.Printf("[Server] %s(%s) Register success!", s.Name, addr)
}

func Start(serverName string, timeout time.Duration, service *service.Service) {
	time.Sleep(time.Second)
	l, err := net.Listen("tcp", "127.0.0.1:")
	if err != nil {
		log.Fatalf("[Server] An error occurred on the server: %s", err.Error())
	}
	a := l.Addr().String()
	log.Printf("[Server] Listening on %s", a)
	s := NewServer(registry.Addr, timeout, serverName, service)
	s.register(a)
	for {
		conn, err := l.Accept()
		if err != nil {
			log.Fatalf("[Server] An error occurred on the server: %s", err.Error())
		}
		go s.handleConnection(conn)
	}
}
