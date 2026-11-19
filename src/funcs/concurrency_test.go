package funcs

import (
	"fmt"
	"github.com/smartping/smartping/src/g"
	"sync"
	"testing"
)

func TestConcurrentAlertStatusAccess(t *testing.T) {
	g.AlertLock.Lock()
	g.AlertStatus = make(map[string]bool)
	g.AlertLock.Unlock()

	var wg sync.WaitGroup
	workers := 20
	iterations := 100

	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			addr := fmt.Sprintf("10.0.0.%d", workerID%5)
			for j := 0; j < iterations; j++ {
				g.AlertLock.Lock()
				g.AlertStatus[addr] = (j%2 == 0)
				_ = g.AlertStatus[addr]
				g.AlertLock.Unlock()
			}
		}(i)
	}

	wg.Wait()
}

func TestConcurrentToolLimitAccess(t *testing.T) {
	g.ToolLimitLock.Lock()
	g.ToolLimit = make(map[string]int)
	g.ToolLimitLock.Unlock()

	var wg sync.WaitGroup
	workers := 20
	iterations := 100

	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			ip := fmt.Sprintf("192.168.1.%d", workerID%5)
			for j := 0; j < iterations; j++ {
				g.ToolLimitLock.Lock()
				g.ToolLimit[ip] = j
				_ = g.ToolLimit[ip]
				g.ToolLimitLock.Unlock()
			}
		}(i)
	}

	wg.Wait()
}

func TestConcurrentAuthMapAccess(t *testing.T) {
	g.AuthLock.Lock()
	g.AuthUserIpMap = make(map[string]bool)
	g.AuthAgentIpMap = make(map[string]bool)
	g.AuthLock.Unlock()

	var wg sync.WaitGroup
	workers := 10
	iterations := 100

	// Readers
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				g.AuthLock.RLock()
				_ = g.AuthUserIpMap["127.0.0.1"]
				_ = g.AuthAgentIpMap["10.0.0.1"]
				g.AuthLock.RUnlock()
			}
		}(i)
	}

	// Writers
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				g.AuthLock.Lock()
				g.AuthUserIpMap[fmt.Sprintf("192.168.1.%d", j%10)] = true
				g.AuthAgentIpMap[fmt.Sprintf("10.0.0.%d", j%10)] = true
				g.AuthLock.Unlock()
			}
		}(i)
	}

	wg.Wait()
}
