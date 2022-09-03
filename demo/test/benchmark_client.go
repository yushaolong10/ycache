package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math/rand"
	"net/http"
	"strconv"
	"sync/atomic"
	"time"
)

var (
	apiUrl          = "http://127.0.0.1:10001/benchmark"
	costSum   int64 = 0
	costMax   int64 = 0
	costCount int64 = 1
	GoNum           = 2
	QPS       int64 = 0
	Interval        = 1000 * time.Nanosecond //ns
)

func clientDo() {
	val := rand.New(rand.NewSource(time.Now().UnixNano())).Intn(123456)
	index := val % 100
	params := map[string]interface{}{
		"user_id": strconv.FormatInt(int64(index), 10),
	}
	req, _ := json.Marshal(params)

	resp, err := http.DefaultClient.Post(apiUrl, "", bytes.NewReader(req))
	if err != nil {
		fmt.Println("request err", err.Error())
		return
	}
	defer resp.Body.Close()
	_, _ = ioutil.ReadAll(resp.Body)
	//fmt.Println("request body", string(body))
}

func beginExecute() {
	for i := 0; i < GoNum; i++ {
		go func() {
			for {
				begin := time.Now().UnixNano() / 1000
				clientDo()
				end := time.Now().UnixNano() / 1000
				cost := end - begin
				atomic.AddInt64(&costSum, cost)
				atomic.AddInt64(&costCount, 1)
				atomic.AddInt64(&QPS, 1)
				if costMax < cost {
					atomic.StoreInt64(&costMax, cost)
				}
				time.Sleep(Interval)
			}
		}()
	}
}

func monitor() {
	go func() {
		for {
			time.Sleep(time.Second)
			max := atomic.SwapInt64(&costMax, 0)
			count := atomic.SwapInt64(&costCount, 0)
			sum := atomic.SwapInt64(&costSum, 0)
			qps2 := atomic.SwapInt64(&QPS, 0)
			if count == 0 {
				count = 1
			}
			fmt.Printf("[ServerExcute]QPS=%v,CostMax=%vμs,CostAvg=%vμs\n", qps2, max, sum/count)
		}
	}()
}

func main() {
	beginExecute()
	monitor()
	select {}
}
