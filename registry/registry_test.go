package registry

import (
	"testing"
	"time"
)

func Test(t *testing.T) {
	r := NewRegistry("192.18.10.10:30001", time.Second*1)
	r.Register("server1", "10.50.8.10")
	r.Register("server1", "10.10.10.100")
	r.Register("server1", "10.50.8.10")
	r.Register("server2", "10.10.10.11")
	r.Get("name", 0)
}
