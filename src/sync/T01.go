package main

import (
	"sync"
	"fmt"
	"context"
	"time"
	"net/http"
)

func main() {
	//test_wg()
	test_cancel()
}
func test_wg() {
	wg := sync.WaitGroup{}
	wg.Add(3)
	go func() {
		defer wg.Done()
		fmt.Printf(" hello ")
	}()
	go func() {
		defer wg.Done()
		fmt.Printf(" world ")
	}()
	go func() {
		defer wg.Done()
		fmt.Printf(" !")
	}()
	wg.Wait()
	fmt.Printf(" all done")
}

func test_cancel() {
	ctx := context.Background()
	ctx, cancel := context.WithCancel(ctx)
	go Proc(ctx)
	go Proc(ctx)
	go Proc(ctx)
	fmt.Printf("sleep()!\n")
	time.Sleep(time.Second)
	cancel()
	fmt.Printf("cancel()!\n")
}
func Proc(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
			time.Sleep(time.Duration(1) * time.Second)
			fmt.Printf("default\n")
		}
	}
}

func Handler(r *http.Request) {
	//timeout := r.Value("timeout")
	timeout := time.Duration(1000)
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	done := make(chan struct{}, 1)
	go func() {
		//RPC(ctx,...)
		fmt.Println("ok")
		done <- struct{}{}
	}()
	select {
	case <-done:
	//nice
	case <-ctx.Done():
	//timeout
	}

}