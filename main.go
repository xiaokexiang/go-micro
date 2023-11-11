package main

import (
	"client"
	"log"
	"registry"
	"server"
	"service"
	"time"
)

func main() {
	var svc service.Service
	s := service.NewService(&svc)
	go registry.Start()
	go server.Start("server1", time.Second*5, s)
	go server.Start("server1", time.Second*5, s)
	time.Sleep(time.Second * 2)
	r, _ := client.Call(registry.Addr, "server1", "Square", 2)
	log.Printf("result: %#v\n", r)
}
