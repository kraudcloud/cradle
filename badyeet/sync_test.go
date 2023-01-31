package badyeet

import (
	"sync"
	"testing"
	"time"
)

func TestSync(t *testing.T) {

	for i := 0; i < 20; i++ {
		testSync(t)
	}
}

func testSync(t *testing.T) {

	var done = false
	go func() {
		time.Sleep(time.Second)
		if !done {
			panic("should be done by now")
		}
	}()
	defer func() { done = true }()

	sockA, sockB := pipe()
	defer sockA.Close()
	defer sockB.Close()

	sockA.Write([]byte("junk"))

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		err := Sync(sockA)
		if err != nil {
			panic(err)
		}
	}()

	go func() {
		defer wg.Done()
		err := Sync(sockB)
		if err != nil {
			panic(err)
		}
	}()

	wg.Wait()
}
