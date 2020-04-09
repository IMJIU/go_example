package main

import (
	"fmt"
	"time"
	//"time"
)

var mapChan = make(chan map[string]int, 1)

//传map会有引用
func main() {
	waitChan := make(chan struct{}, 2)
	go func() { // 接收操作。
		for {
			if elem, ok := <-mapChan; ok {
				elem["count"]++
				fmt.Printf("receive count: %v.\n", elem)
			} else {
				break
			}
		}
		fmt.Println("Stopped. [receiver]")
		waitChan <- struct{}{}
	}()
	go func() { // 发送操作。
		countMap := make(map[string]int)
		for i := 0; i < 5; i++ {
			mapChan <- countMap
			time.Sleep(time.Millisecond)
			fmt.Printf("[sender]: %v. \n", countMap)
		}
		close(mapChan)
		waitChan <- struct{}{}
	}()
	<-waitChan
	<-waitChan
}
