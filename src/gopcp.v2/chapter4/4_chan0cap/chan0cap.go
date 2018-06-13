package main

import (
	"fmt"
	"time"
)

func main() {
	time1s := time.Second
	time2s := time.Second * 2
	intChan := make(chan int, 0)
	go func() {
		var lastTime, curTime int64
		for i := 1; i <= 5; i++ {
			intChan <- i
			curTime = time.Now().Unix()
			if lastTime == 0 {
				fmt.Println("Sent:", i)
			} else {
				fmt.Printf("Sent: %d [interval: %d s]\n", i, curTime - lastTime)
			}
			lastTime = time.Now().Unix()
			time.Sleep(time1s)
		}
		close(intChan)
	}()
	var lastTime, curTime int64
	Loop:
	for {
		select {
		case v, ok := <-intChan:
			if !ok {
				break Loop
			}
			curTime = time.Now().Unix()
			if lastTime == 0 {
				fmt.Println("Received:", v)
			} else {
				fmt.Printf("Received: %d [interval: %d s]\n", v, curTime - lastTime)
			}
		}
		lastTime = time.Now().Unix()
		time.Sleep(time2s)
	}
	fmt.Println("End.")
}
