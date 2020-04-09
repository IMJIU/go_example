package main

import (
	"fmt"
	"time"
)

// Counter 代表计数器的类型。
type Counter struct {
	count int
}

var mapChan = make(chan map[string]Counter, 1)

func main() {
	syncChan := make(chan struct{}, 2)
	go func() { // 接收操作。
		for {
			if elem, ok := <-mapChan; ok {
				counter := elem["count"]
				fmt.Printf("The count map: %v. [receiver]\n", counter)
				counter.count++
			} else {
				break
			}
		}
		fmt.Println("Stopped. [receiver]")
		syncChan <- struct{}{}
	}()
	go func() { // 发送操作。
		countMap := map[string]Counter{
			"count": Counter{},
		}
		for i := 0; i < 5; i++ {
			mapChan <- countMap
			time.Sleep(time.Millisecond)
			fmt.Printf("The count map: %v. [sender]\n", countMap)
		}
		close(mapChan)
		syncChan <- struct{}{}
	}()
	<-syncChan
	<-syncChan
}
