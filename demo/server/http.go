package server

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"strconv"
	"time"
	"ycache/demo/levelcache"
	"ycache/demo/prometheus"
)

func NewHttpServer() *http.Server {
	//api
	adaptor := &httpRouterAdaptor{}
	http.Handle("/benchmark", adaptor.wrap(adaptor.benchmark))
	//metric
	http.Handle("/metrics", prometheus.NewHttpHandler())

	server := &http.Server{
		Addr:         fmt.Sprintf(":%d", 10001),
		ReadTimeout:  time.Second * 10,
		WriteTimeout: time.Second * 10,
		IdleTimeout:  time.Second * 30,
	}
	return server
}

//adaptor
type httpRouterAdaptor struct {
}

//wrap middleware
func (adaptor *httpRouterAdaptor) wrap(handler http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		handler.ServeHTTP(w, r)
	}
}

// http response
func (adaptor *httpRouterAdaptor) send(resp http.ResponseWriter, respData *httpResponseData) {
	resp.WriteHeader(200)
	if err := json.NewEncoder(resp).Encode(respData); err != nil {
		http.Error(resp, "could not encode response", http.StatusInternalServerError)
	}
}

//error response
func httpError(requestId string, code int, message string) *httpResponseData {
	return &httpResponseData{
		RequestId: requestId,
		Code:      code,
		Message:   message,
	}
}

//success response
func httpSuccess(requestId string, data interface{}) *httpResponseData {
	return &httpResponseData{
		RequestId: requestId,
		Data:      data,
	}
}

type httpResponseData struct {
	RequestId string      `json:"uuid"`
	Code      int         `json:"code"`
	Message   string      `json:"message,omitempty"`
	Data      interface{} `json:"data,omitempty"`
}

type benchRequest struct {
	UserId string `json:"user_id"`
}

type benchResponse struct {
	UserId   string            `json:"user_id"`
	UserName string            `json:"user_name"`
	Pencils  map[string]string `json:"pencils"`
}

func (adaptor *httpRouterAdaptor) uuid() string {
	return strconv.FormatInt(time.Now().UnixNano(), 10)
}

//api: benchmark
func (adaptor *httpRouterAdaptor) benchmark(w http.ResponseWriter, r *http.Request) {
	var err error
	defer func(begin time.Time) {
		status := 0
		if err != nil {
			status = 1
		}
		interval := time.Since(begin).Microseconds()
		prometheus.UpdateHttp("/benchmark", status, interval)
	}(time.Now())

	var uuid = adaptor.uuid()
	var reqUser = &benchRequest{}
	if err := json.NewDecoder(r.Body).Decode(reqUser); err != nil {
		adaptor.send(w, httpError(uuid, 1000, fmt.Sprintf("params decode err:%s", err.Error())))
		return
	}
	ctx := context.Background()
	name, err := levelcache.Biz1Cache.Get(ctx, "_bench_name_", reqUser.UserId, func(ctx context.Context, key string) ([]byte, error) {
		num := rand.New(rand.NewSource(time.Now().UnixNano())).Intn(10000) + 1000
		time.Sleep(time.Microsecond * time.Duration(num))
		reqId, _ := strconv.ParseInt(key, 10, 64)
		id := reqId % 80
		if id == 0 {
			return []byte("id_0_name"), nil
		} else if id < 5 {
			return []byte("id_less_5_name"), nil
		} else if id < 10 {
			return []byte("id_less_10_name"), nil
		} else if id < 20 {
			return []byte("id_less_20_name"), nil
		} else if id < 30 {
			return []byte("id_less_30_name"), nil
		} else if id < 40 {
			return []byte("id_less_40_name"), nil
		} else if id < 50 {
			return []byte("id_less_50_name"), nil
		} else if id < 60 {
			return []byte("id_less_60_name"), nil
		} else {
			return nil, fmt.Errorf("not support duration")
		}
	})
	if err != nil {
		adaptor.send(w, httpError(uuid, 1001, fmt.Sprintf("get name err:%s", err.Error())))
		return
	}
	pencilRet, err := levelcache.Biz1Cache.Get(ctx, "_bench_pencil_", reqUser.UserId, func(ctx context.Context, key string) ([]byte, error) {
		num := rand.New(rand.NewSource(time.Now().UnixNano())).Intn(10000) + 1000
		time.Sleep(time.Microsecond * time.Duration(num))
		reqId, _ := strconv.ParseInt(key, 10, 64)
		id := reqId % 80
		if id < 20 {
			return []byte(`["sky","plant","water"]`), nil
		} else if id < 60 {
			return []byte(`["hot","wind","cloud","rabbit"]`), nil
		} else if id < 70 {
			return []byte(`["dog","duck","road"]`), nil
		} else {
			return nil, fmt.Errorf("not exist pencils")
		}
	})
	if err != nil {
		adaptor.send(w, httpError(uuid, 1002, fmt.Sprintf("get pencils err:%s", err.Error())))
		return
	}
	pencils := make([]string, 0)
	_ = json.Unmarshal(pencilRet, &pencils)
	colorRet, err := levelcache.Biz1Cache.BatchGet(ctx, "_bench_color_", pencils, func(ctx context.Context, keys []string) (map[string][]byte, error) {
		num := rand.New(rand.NewSource(time.Now().UnixNano())).Intn(10000) + 1000
		time.Sleep(time.Microsecond * time.Duration(num))
		kvs := make(map[string][]byte)
		for _, key := range keys {
			if color, ok := pencilColors[key]; ok {
				kvs[key] = []byte(color)
			}
		}
		return kvs, nil
	})
	if err != nil {
		adaptor.send(w, httpError(uuid, 1003, fmt.Sprintf("get colors err:%s", err.Error())))
		return
	}
	pencilColors := make(map[string]string, 0)
	for _, pencil := range pencils {
		if color, ok := colorRet[pencil]; ok {
			pencilColors[pencil] = string(color)
		}
	}
	//response
	adaptor.send(w, httpSuccess(uuid, &benchResponse{UserId: reqUser.UserId, UserName: string(name), Pencils: pencilColors}))
}

var pencilColors = map[string]string{
	"sky":    "white",
	"plant":  "green",
	"water":  "pink",
	"hot":    "red",
	"wind":   "blue",
	"cloud":  "yellow",
	"rabbit": "orange",
	"dog":    "black",
	"duck":   "cola",
	"road":   "gray",
	"sheep":  "seven",
}
