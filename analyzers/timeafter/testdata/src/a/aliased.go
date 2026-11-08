package a

import tt "time"

func aliased(done chan int) {
	timer := tt.NewTimer(tt.Second)
	defer timer.Stop()
	select {
	case v := <-done:
		use(v)
	case <-timer.C: // want `timer ceremony can be replaced by time.After`
	}
}
