package main

import "fmt"

func main() {
	count := 5
	intChan := make(chan int, count)
	for i := 0; i < count; i++ {
		select {
		case intChan <- 1:
		case intChan <- 2:
		case intChan <- 3:
		}
	}
	for i := 0; i < count; i++ {
		fmt.Printf("%d\n", <-intChan)
	}
}
