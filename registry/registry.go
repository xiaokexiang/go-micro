package registry

import (
	"encoding/json"
	"errors"
	"log"
	"math/rand"
	"net"
	"net/http"
	"strconv"
	"sync"
	"time"
)

type Registry struct {
	lock    sync.Mutex
	addr    string                  // 注册中心地址
	servers map[string][]ServerItem // 服务注册地址,如果服务名相同，表示这两个服务用做负载均衡，只有registry本身会Refresh，所以不用指针
	timeout time.Duration           // 服务注册时间 + timeout > now() 表示服务超时，移除
	mode    Mode                    // 负载均衡的规则
	index   int                     // 记录上次
}
type ServerItem struct {
	Name, Addr string    // 服务名称、地址
	Time       time.Time // 注册时间
}
type Mode int

const (
	Random Mode = iota
	RobinSelect
)

type ResponseBody struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data"`
}

func Success(w http.ResponseWriter, message string, data any) {
	ExportJson(w, &ResponseBody{
		Code:    200,
		Message: message,
		Data:    data,
	})
}

func Error(w http.ResponseWriter, code int, message string) {
	ExportJson(w, &ResponseBody{
		Code:    code,
		Message: message,
		Data:    nil,
	})
}

func ExportJson(w http.ResponseWriter, r *ResponseBody) {
	w.Header().Set("content-type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(r)
}

func NewRegistry(addr string, timeout time.Duration) *Registry {
	return &Registry{
		timeout: timeout,
		addr:    addr,
		servers: make(map[string][]ServerItem),
		mode:    RobinSelect,
		index:   -1,
	}
}

// Register 注册服务到注册中心
func (r *Registry) Register(name, addr string) {
	r.lock.Lock()
	defer r.lock.Unlock()
	s, ok := r.servers[name]
	if ok {
		var exist bool
		for _, server := range s {
			if server.Addr == addr {
				exist = true
				server.Time = time.Now()
				log.Printf("[Registry] Server(%s-%s) set registration time to: %s", server.Name, server.Addr, server.Time.Format("15:04:05"))
			}
		}
		if !exist {
			server := ServerItem{
				Addr: addr,
				Name: name,
				Time: time.Now(),
			}
			s = append(s, server)
			log.Printf("[Registry] Server(%s-%s) register to registry", server.Name, server.Addr)
			r.servers[name] = s
		}
	} else {
		items := make([]ServerItem, 0)
		server := ServerItem{
			Addr: addr,
			Name: name,
			Time: time.Now(),
		}
		items = append(items, server)
		log.Printf("[Registry] Server(%s-%s) register to registry", server.Name, server.Addr)
		r.servers[name] = items
	}
}

// Get 根据负载均衡类型返回服务端addr
func (r *Registry) Get(name string, mode Mode) (string, error) {
	r.lock.Lock()
	defer r.lock.Unlock()

	if _, ok := r.servers[name]; !ok {
		return "", errors.New("[Registry] No server data matching the name of " + name)
	}
	switch mode {
	case Random:
		rand.Seed(time.Now().UnixNano())
		i := rand.Intn(len(r.servers[name]))
		return (r.servers[name])[i].Addr, nil
	case RobinSelect:
		r.index = (r.index + 1) % len(r.servers[name])
		return (r.servers[name])[r.index].Addr, nil
	default:
		return "", errors.New("[Registry] No mode type meets the mode")
	}
}

// 心跳检测(服务是否可达 & 注册时间是否超过timeout)
func (r *Registry) heartbeat() {
	r.lock.Lock()
	defer r.lock.Unlock()

	var wg sync.WaitGroup
	for name, serverItems := range r.servers {
		if len(serverItems) == 0 {
			continue
		}
		wg.Add(1)
		newItem := make([]ServerItem, 0) // make 关键字能够获取一个初始化后的非零值，适用于引用类型(new 获取的是一个指向零值的指针，适用于值类型)
		ch := make(chan []ServerItem)
		go func(items []ServerItem) {
			defer wg.Done()
			for _, item := range items {
				if _, err := net.DialTimeout("tcp", item.Addr, time.Second*1); err != nil {
					log.Printf("[Registry] Server %s(%s) disconnect from registry via unable to access： %s, registry time: %s", item.Addr, item.Name, r.addr, item.Time.Format("15:04:05"))
					continue
				}
				if time.Now().After(item.Time.Add(r.timeout)) {
					log.Printf("[Registry] Server %s(%s) disconnect from registry via timeout： %s, registry time: %s", item.Addr, item.Name, r.addr, item.Time.Format("15:04:05"))
					continue
				}
				item.Time = time.Now()
				newItem = append(newItem, item)
			}
			ch <- newItem
		}(serverItems)
		select {
		case newItem = <-ch:
			if len(newItem) == 0 {
				delete(r.servers, name) // 如果没有服务，则删除服务名key
			} else {
				r.servers[name] = newItem
			}
		}
	}
	wg.Wait()
}

func (r *Registry) startHeartBeat(interval time.Duration) {
	timer := time.NewTicker(interval)
	go func() {
		for {
			<-timer.C
			log.Printf("[Registry] Heartbeat detection initiated ...")
			r.heartbeat()
			log.Printf("[Registry] Heartbeat detection end ...")
		}
	}()
}

// 服务发现与服务注册
func (r *Registry) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	switch req.Method {
	case http.MethodGet:
		q := req.URL.Query()
		name := q.Get("name")
		mode := q.Get("mode")
		if "" == q.Get("name") {
			Error(w, 500, "Missing param [name]")
			return
		}
		if "" == q.Get("mode") {
			Error(w, 500, "Missing param [mode]")
			return
		}
		number, err := strconv.Atoi(mode)
		if err != nil {
			Error(w, 500, "mode must be a number")
		}
		server, err := r.Get(name, Mode(number))
		if err != nil {
			Error(w, 500, err.Error())
			return
		}
		Success(w, "success", server)
		return
	case http.MethodPost:
		var body map[string]any
		err := json.NewDecoder(req.Body).Decode(&body)
		if err != nil {
			Error(w, 500, "Error decoding request body")
			return
		}
		if _, ok := body["name"]; !ok {
			Error(w, 500, "Missing param [name]")
			return
		}
		if _, ok := body["addr"]; !ok {
			Error(w, 500, "Missing param [addr]")
			return
		}
		name := body["name"].(string)
		addr := body["addr"].(string)
		r.Register(name, addr)
		Success(w, "Register success", nil)
		return
	default:
		Error(w, 500, "Invalid request method")
		return
	}
}

const Addr = "127.0.0.1:9999"

func Start() {
	l, err := net.Listen("tcp", Addr)
	if err != nil {
		log.Fatalf("[Registry] Registry start failed, error: %s", err)
	}
	s := l.Addr().String()
	r := NewRegistry(s, time.Second*30)
	r.startHeartBeat(time.Second * 10)
	http.Handle("/server", r)
	log.Printf("[Registry] Listening on %s", s)
	_ = http.Serve(l, r)
}
