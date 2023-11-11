package client

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"registry"
	"time"
)

type Client struct {
	RegistryAddr           string
	ServerName, ServerAddr string
	timeout                time.Duration
}

type Request struct {
	MethodName string
	Seq        int64
	Args       any
}

func NewClient(registryAddr, serverName string) *Client {
	return &Client{
		RegistryAddr: registryAddr,
		ServerName:   serverName,
		timeout:      time.Second * 5,
	}
}

func (c *Client) newRequest(methodName string, args any) *Request {
	return &Request{
		MethodName: methodName,
		Seq:        time.Now().Unix(),
		Args:       args,
	}
}

func (c *Client) findServer() error {
	resp, err := http.Get(fmt.Sprintf("http://%s/server?name=%s&mode=0", c.RegistryAddr, c.ServerName))
	if err != nil {
		return err
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	defer func(Body io.ReadCloser) {
		_ = Body.Close()
	}(resp.Body)
	var response registry.ResponseBody
	_ = json.Unmarshal(body, &response)
	if response.Data == nil {
		return errors.New("no server found")
	}
	c.ServerAddr = response.Data.(string)
	return nil
}

func (c *Client) doCall(request *Request) (result any, err error) {
	conn, err := net.DialTimeout("tcp", c.ServerAddr, c.timeout)
	if err != nil {
		log.Printf("[Client] An error occurred while connecting to the server: %s", err)
		return
	}
	defer func(conn net.Conn) {
		_ = conn.Close()
	}(conn)

	reader := bufio.NewReader(conn)
	writer := bufio.NewWriter(conn)

	p, err := json.Marshal(*request)
	if err != nil {
		log.Printf("[Client] An error occurred while marshalling JSON: %s", err)
		return
	}

	_, err = writer.WriteString(string(p) + "\n")
	if err != nil {
		log.Printf("[Client] An error occurred while writing data: %s", err)
		return
	}
	_ = writer.Flush()

	response, err := reader.ReadString('\n')
	if err != nil {
		log.Printf("[Client] An error occurred while reading response: %s", err)
		return
	}

	var resp map[string]any
	err = json.Unmarshal([]byte(response), &resp)
	if err != nil {
		log.Printf("[Client] An error occurred while unmarshalling JSON: %s", err)
		return
	}
	log.Printf("[Client] Received response JSON: %+v", resp)
	return resp, nil
}

func (c *Client) callSync(request *Request) (result any, error error) {
	return c.doCall(request)
}

func (c *Client) callAsync(ch chan any, request *Request) {
	result, err := c.doCall(request)
	if err != nil {
		ch <- err
	} else {
		ch <- result
	}
}

func Call(registryAddr, serverName, methodName string, args any) (result any, err error) {
	c := NewClient(registryAddr, serverName)
	err = c.findServer()
	if err != nil {
		log.Printf("[Client] Find server error: %s", err.Error())
		return
	}
	request := c.newRequest(methodName, args)
	result, err = c.callSync(request)
	if err != nil {
		log.Printf("[Client] Send request to server[%s] error: %s", serverName, err.Error())
		return
	}
	log.Printf("[Client] Send request to server[%s] success, result: %#v", serverName, result)
	return
}

func CallAsync(registryAddr, serverName, methodName string, args any) (channel <-chan any, err error) {
	ch := make(chan any)
	c := NewClient(registryAddr, serverName)
	err = c.findServer()
	if err != nil {
		log.Printf("[Client] Find server error: %s", err.Error())
		return
	}
	request := c.newRequest(methodName, args)
	go c.callAsync(ch, request)
	return ch, nil
}
