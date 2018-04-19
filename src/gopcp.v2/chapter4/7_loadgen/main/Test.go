package main

import (
	"time"
	. "gopcp.v2/chapter4/7_loadgen"
	loadgenlib "gopcp.v2/chapter4/7_loadgen/lib"
	helper "gopcp.v2/chapter4/7_loadgen/testhelper"
	"gopcp.v2/helper/log"
	"fmt"
)

// printDetail 代表是否打印详细结果。
var printDetail = false

func main() {
	Test_Start()
	fmt.Println("=====================")
	//TestStop()
}
func Test_Start() {
	t := log.DLogger();
	// 初始化服务器。
	server := helper.NewTCPServer()
	defer server.Close()
	serverAddr := "127.0.0.1:8080"
	t.Info("Startup TCP server(%s)...\n", serverAddr)
	err := server.Listen(serverAddr)
	if err != nil {
		t.Fatalf("TCP Server startup failing! (addr=%s)!\n", serverAddr)
		//t.FailNow()
	}

	// 初始化载荷发生器。
	pset := ParamSet{
		Caller:     helper.NewTCPComm(serverAddr),
		TimeoutNS:  50 * time.Millisecond,
		LPS:        uint32(1000),
		DurationNS: 10 * time.Second,
		ResultCh:   make(chan *loadgenlib.CallResult, 50),
	}
	t.Info("Initialize load generator (timeoutNS=%v, lps=%d, durationNS=%v)...",
		pset.TimeoutNS, pset.LPS, pset.DurationNS)
	gen, err := NewGenerator(pset)
	if err != nil {
		t.Fatalf("Load generator initialization failing: %s\n",err)
		//t.FailNow()
	}

	// 开始！
	t.Info("Start load generator...")
	gen.Start()

	// 显示结果。
	countMap := make(map[loadgenlib.RetCode]int)
	for r := range pset.ResultCh {
		countMap[r.Code] = countMap[r.Code] + 1
		if printDetail {
			t.Infof("Result: ID=%d, Code=%d, Msg=%s, Elapse=%v.\n",
				r.ID, r.Code, r.Msg, r.Elapse)
		}
	}

	var total int
	t.Info("RetCode Count:")
	for k, v := range countMap {
		codePlain := loadgenlib.GetRetCodePlain(k)
		t.Infof("  Code plain: %s (%d), Count: %d.\n",
			codePlain, k, v)
		total += v
	}

	t.Infof("Total: %d.\n", total)
	successCount := countMap[loadgenlib.RET_CODE_SUCCESS]
	tps := float64(successCount) / float64(pset.DurationNS / 1e9)
	t.Infof("Loads per second: %d; Treatments per second: %f.\n", pset.LPS, tps)
}

func TestStop() {
	t := log.DLogger()
	// 初始化服务器。
	server := helper.NewTCPServer()
	defer server.Close()
	serverAddr := "127.0.0.1:8081"
	t.Infof("Startup TCP server(%s)...\n", serverAddr)
	err := server.Listen(serverAddr)
	if err != nil {
		t.Fatalf("TCP Server startup failing! (addr=%s)!\n", serverAddr)
		//t.FailNow()
	}

	// 初始化载荷发生器。
	pset := ParamSet{
		Caller:     helper.NewTCPComm(serverAddr),
		TimeoutNS:  50 * time.Millisecond,
		LPS:        uint32(1000),
		DurationNS: 10 * time.Second,
		ResultCh:   make(chan *loadgenlib.CallResult, 50),
	}
	t.Infof("Initialize load generator (timeoutNS=%v, lps=%d, durationNS=%v)...",
		pset.TimeoutNS, pset.LPS, pset.DurationNS)
	gen, err := NewGenerator(pset)
	if err != nil {
		t.Fatalf("Load generator initialization failing: %s.\n", err)
		//t.FailNow()
	}

	// 开始！
	t.Info("Start load generator...")
	gen.Start()
	timeoutNS := 2 * time.Second
	time.AfterFunc(timeoutNS, func() {
		gen.Stop()
	})

	// 显示调用结果。
	countMap := make(map[loadgenlib.RetCode]int)
	count := 0
	for r := range pset.ResultCh {
		countMap[r.Code] = countMap[r.Code] + 1
		if printDetail {
			t.Infof("Result: ID=%d, Code=%d, Msg=%s, Elapse=%v.\n",
				r.ID, r.Code, r.Msg, r.Elapse)
		}
		count++
	}

	var total int
	t.Info("RetCode Count:")
	for k, v := range countMap {
		codePlain := loadgenlib.GetRetCodePlain(k)
		t.Infof("  Code plain: %s (%d), Count: %d.\n",
			codePlain, k, v)
		total += v
	}

	t.Infof("Total: %d.\n", total)
	successCount := countMap[loadgenlib.RET_CODE_SUCCESS]
	tps := float64(successCount) / float64(timeoutNS / 1e9)
	t.Infof("Loads per second: %d; Treatments per second: %f.\n", pset.LPS, tps)
}
