package window

import (
	"fmt"
	"testing"
	"time"
)

func TestWindow(t *testing.T) {
	w := NewWindowWithOption(time.Millisecond*500, 10)
	for i := 0; i < 500; i++ {
		w.Succeed()
		if i%2 == 0 {
			w.Fail()
		}
		if i%4 == 0 {
			w.Timeout()
		}
		time.Sleep(time.Millisecond * 100)
		fmt.Println(w.SuccessCount(), w.FailCount(), w.TimeoutCount(), w.SuccessRate(), w.ErrorRate())
	}
}

func BenchmarkWindow(b *testing.B) {
	w := NewWindow()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			w.Succeed()
			w.Fail()
			w.Timeout()
			_ = w.SuccessCount()
			_ = w.FailCount()
			_ = w.TimeoutCount()
			_ = w.SuccessRate()
			_ = w.ErrorRate()
		}
	})
}