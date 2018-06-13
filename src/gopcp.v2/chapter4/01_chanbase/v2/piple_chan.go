package main

import (
	"fmt"
	"time"
)

var queueChan = make(chan string, 3)

func main() {
	syncChan1 := make(chan struct{}, 1)
	syncChan2 := make(chan struct{}, 2)
	go receive(queueChan, syncChan1, syncChan2) // 用于演示接收操作。
	go send(queueChan, syncChan1, syncChan2)    // 用于演示发送操作。
	<-syncChan2
	<-syncChan2
}

func receive(queueChan <-chan string, syncChan1 <-chan struct{}, syncChan2 chan <- struct{}) {
	<-syncChan1
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
	syncChan2 <- struct{}{}
}

func send(queueChan chan <- string, syncChan1 chan <- struct{}, syncChan2 chan <- struct{}) {
	for _, elem := range []string{"a", "b", "c", "d"} {
		queueChan <- elem
		fmt.Println("Sent:", elem, "[sender]")
		if elem == "c" {
			syncChan1 <- struct{}{}
			fmt.Println("Sent a sync signal. [sender]")
		}
	}
	fmt.Println("start Wait 2 seconds... [sender]")
	time.Sleep(time.Second * 2)
	close(queueChan)
	syncChan2 <- struct{}{}
}
