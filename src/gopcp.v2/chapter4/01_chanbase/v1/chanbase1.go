package main

import (
	"fmt"
	"time"
)

var queueChan = make(chan string, 3)

//发送chan方，比较快，不符合逻辑-！！
func main() {
	sync1 := make(chan struct{}, 1)
	sync2 := make(chan struct{}, 2)
	go func() { // 用于演示接收操作。
		<-sync1
		fmt.Println("Received a sync signal and wait a second... [receiver]")
		time.Sleep(time.Second)
		for {
			if elem, ok := <-queueChan; ok {
				fmt.Println("Received:", elem, "[receiver]")
			} else {
				break
			}
		}
		fmt.Println("Stopped. [receiver]")
		sync2 <- struct{}{}
	}()
	go func() { // 用于演示发送操作。
		for _, elem := range []string{"a", "b", "c", "d"} {
			queueChan <- elem
			fmt.Println("Sent:", elem, "[sender]")
			if elem == "c" {
				sync1 <- struct{}{}
				fmt.Println("Sent a sync signal. [sender]")
			}
		}
		fmt.Println("Wait 2 seconds... [sender]")
		time.Sleep(time.Second * 2)
		close(queueChan)
		sync2 <- struct{}{}
	}()
	<-sync2
	<-sync2
}
