package a

import "time"

func use(...any) {}

func ceremony(done chan int) {
	timer := time.NewTimer(time.Second)
	defer timer.Stop()
	select {
	case v := <-done:
		use(v)
	case <-timer.C: // want `timer ceremony can be replaced by time.After`
		use("timeout")
	}
}

func noStop(done chan int) {
	timer := time.NewTimer(time.Second)
	select {
	case v := <-done:
		use(v)
	case <-timer.C: // want `timer ceremony can be replaced by time.After`
		use("timeout")
	}
}

func reset(done chan int) {
	timer := time.NewTimer(time.Second)
	defer timer.Stop()
	for {
		select {
		case v := <-done:
			use(v)
			timer.Reset(time.Second)
		case <-timer.C:
			return
		}
	}
}

func plainRecv() {
	timer := time.NewTimer(time.Second)
	<-timer.C
}

func bodyRecv(quit chan int) {
	timer := time.NewTimer(time.Second)
	defer timer.Stop()
	select {
	case <-quit:
		<-timer.C
	}
}

func nested(done chan int) {
	timer := time.NewTimer(time.Second)
	defer timer.Stop()
	for {
		select {
		case v := <-done:
			use(v)
		case <-timer.C:
			return
		}
	}
}

func resetSameLevel(done chan int) {
	timer := time.NewTimer(time.Second)
	timer.Reset(time.Second)
	select {
	case v := <-done:
		use(v)
	case <-timer.C:
	}
}

func condStop(done chan int, cond bool) {
	timer := time.NewTimer(time.Second)
	if cond {
		defer timer.Stop()
	}
	select {
	case v := <-done:
		use(v)
	case <-timer.C:
	}
}

func twoRecvs(done chan int) {
	timer := time.NewTimer(time.Second)
	select {
	case v := <-done:
		use(v)
	case <-timer.C:
	}
	select {
	case <-timer.C:
	}
}

func drain(done chan int) {
	timer := time.NewTimer(time.Second)
	if !timer.Stop() {
		<-timer.C
	}
	use(done)
}

func flows(done chan int) {
	timer := time.NewTimer(time.Second)
	defer use(timer.Stop)
	select {
	case v := <-done:
		use(v)
	case <-timer.C:
	}
}

func doubleStop(done chan int) {
	timer := time.NewTimer(time.Second)
	defer timer.Stop()
	defer timer.Stop()
	select {
	case v := <-done:
		use(v)
	case <-timer.C: // want `timer ceremony can be replaced by time.After`
	}
}

func addr() {
	timer := time.NewTimer(time.Second)
	use(&timer.C)
}

func varForm(done chan int) {
	var timer = time.NewTimer(time.Second)
	defer timer.Stop()
	select {
	case v := <-done:
		use(v)
	case <-timer.C:
	}
}

func reassigned(done chan int, timer *time.Timer) {
	timer = time.NewTimer(time.Second)
	select {
	case v := <-done:
		use(v)
	case <-timer.C:
	}
}

func intervening(done chan int) {
	timer := time.NewTimer(time.Second)
	defer timer.Stop()
	use(done)
	select {
	case v := <-done:
		use(v)
	case <-timer.C: // want `timer ceremony can be replaced by time.After`
	}
}

func stopOnly() {
	timer := time.NewTimer(time.Second)
	defer timer.Stop()
}

func sendClause(ch chan *<-chan time.Time) {
	timer := time.NewTimer(time.Second)
	defer timer.Stop()
	select {
	case ch <- &timer.C:
	}
}
