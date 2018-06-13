package main

import (
	"fmt"
	//"time"
)

var mapChan = make(chan map[string]int, 1)
//传map会有引用
func main() {
	sync := make(chan struct{}, 2)
	go func() { // 用于演示接收操作。
		for {
			if elem, ok := <-mapChan; ok {
				elem["count"]++
				fmt.Printf("receive count: %v.\n", elem)
			} else {
				break
			}
		}
		fmt.Println("Stopped. [receiver]")
		sync <- struct{}{}
	}()
	go func() { // 用于演示发送操作。
		countMap := make(map[string]int)
		for i := 0; i < 5; i++ {
			mapChan <- countMap
			//time.Sleep(time.Millisecond)
			fmt.Printf("[sender]: %v. \n", countMap)
		}
		close(mapChan)
		sync <- struct{}{}
	}()
	<-sync
	<-sync
}
