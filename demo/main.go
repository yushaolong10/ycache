package main

import (
	"log"
	_ "net/http/pprof"
	"time"
	"ycache/demo/levelcache"
	"ycache/demo/server"
)

func main() {
	if err := levelcache.Init(); err != nil {
		log.Printf("init level cache err:%s", err.Error())
	}
	log.Printf("init level cache success")
	http := server.NewHttpServer()
	if err := http.ListenAndServe(); err != nil {
		log.Printf("http start err:%s", err.Error())
	}
	log.Printf("server end.")
	time.Sleep(time.Second)
}
