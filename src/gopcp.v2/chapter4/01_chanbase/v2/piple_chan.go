package main

import (
	"fmt"
	"time"
)

var queueChan = make(chan string, 3)

func main() {
	notifyChan2 := make(chan struct{}, 1)
	waitCompleteCh := make(chan struct{}, 2)
	go receive(queueChan, notifyChan2, waitCompleteCh) // 用于演示接收操作。
	go send(queueChan, notifyChan2, waitCompleteCh)    // 用于演示发送操作。
	<-waitCompleteCh
	<-waitCompleteCh
}

func receive(queueChan <-chan string, notifyChan2 <-chan struct{}, waitCompleteCh chan<- struct{}) {
	<-notifyChan2
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
	waitCompleteCh <- struct{}{}
}

func send(queueChan chan<- string, notifyChan2 chan<- struct{}, waitCompleteCh chan<- struct{}) {
	for _, elem := range []string{"a", "b", "c", "d"} {
		queueChan <- elem
		fmt.Println("Sent:", elem, "[sender]")
		if elem == "c" {
			notifyChan2 <- struct{}{}
			fmt.Println("Sent a sync signal. [sender]")
		}
	}
	fmt.Println("start Wait 2 seconds... [sender]")
	time.Sleep(time.Second * 2)
	close(queueChan)
	waitCompleteCh <- struct{}{}
}
