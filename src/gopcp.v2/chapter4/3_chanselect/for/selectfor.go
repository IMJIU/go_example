package main

import "fmt"

func main() {
	ich := make(chan int, 10)
	for i := 0; i < 10; i++ {
		ich <- i
	}
	close(ich)
	countDownCh := make(chan struct{}, 1)
	go func() {
		Loop:
		for {
			select {
			case e, ok := <-ich:
				if !ok {
					fmt.Println("End.")
					break Loop
				}
				fmt.Printf("Received: %v\n", e)
			default:
				fmt.Println("default")
			}
		}
		countDownCh <- struct{}{}
	}()
	<-countDownCh
}
