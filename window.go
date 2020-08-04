package goutils

import (
	"sync"
	"sync/atomic"
	"time"
)

const (
	DEFAULT_BUCKET_DURATION = time.Millisecond * 50
	DEFAULT_BUCKET_NUMS     = 100
)

// bucket有3态：成功/失败/超时
type bucket struct {
	success int64
	fail    int64
	timeout int64
}

func (b *bucket) Succeed() {
	atomic.AddInt64(&b.success, 1)
}

func (b *bucket) SuccessCount() int64 {
	return atomic.LoadInt64(&b.success)
}

func (b *bucket) Fail() {
	atomic.AddInt64(&b.fail, 1)
}

func (b *bucket) FailCount() int64 {
	return atomic.LoadInt64(&b.fail)
}

func (b *bucket) Timeout() {
	atomic.AddInt64(&b.timeout, 1)
}

func (b *bucket) TimeoutCount() int64 {
	return atomic.LoadInt64(&b.timeout)
}

func (b *bucket) Reset() {
	atomic.StoreInt64(&b.success, 0)
	atomic.StoreInt64(&b.fail, 0)
	atomic.StoreInt64(&b.timeout, 0)
}

// 将时间分成一个个区间，每个区间单独计数，取一定区间计数可以统计出一段时间的状态，循环数组模拟时间流动
type Window struct {
	mutex          sync.RWMutex
	buckets        []bucket
	oldest         int
	latest         int
	bucketDuration time.Duration
	bucketNum      int

	// 定时更新
	ticker    *time.Ticker
	stop      chan struct{}
	firstTrip bool

	// 不含当前bucket的统计值
	preSuccess int64
	preFail    int64
	preTimeout int64
}

// 默认统计区间是5s
func NewWindow() *Window {
	return NewWindowWithOption(DEFAULT_BUCKET_DURATION, DEFAULT_BUCKET_NUMS)
}

// 每个统计区间是DEFAULT_BUCKET_DURATION*DEFAULT_BUCKET_NUMS
func NewWindowWithOption(bucketDuration time.Duration, bucketNum int) *Window {
	w := &Window{
		mutex:          sync.RWMutex{},
		buckets:        make([]bucket, bucketNum+2), // +2保证不重叠
		oldest:         0,
		latest:         0,
		firstTrip:      true,
		bucketDuration: bucketDuration,
		bucketNum:      bucketNum,
		ticker:         time.NewTicker(bucketDuration),
	}
	go w.autoLoad()
	return w
}

func (w *Window) Stop() {
	w.stop <- struct{}{}
}

func (w *Window) autoLoad() {
	for {
		select {
		case <-w.ticker.C:
			w.refresh()
		case <-w.stop:
			w.ticker.Stop()
			break
		}
	}
}

func (w *Window) refresh() {
	w.mutex.Lock()
	// latest向前
	w.preSuccess += w.buckets[w.latest].SuccessCount()
	w.preFail += w.buckets[w.latest].FailCount()
	w.preTimeout += w.buckets[w.latest].TimeoutCount()
	w.latest++
	if w.firstTrip && w.latest >= w.bucketNum {
		w.firstTrip = false
	}
	if w.latest >= len(w.buckets) {
		w.latest -= len(w.buckets)
	}
	w.buckets[w.latest].Reset() // 新bucket归零

	// oldest向前
	if !w.firstTrip {
		w.preSuccess -= w.buckets[w.oldest].SuccessCount()
		w.preFail -= w.buckets[w.oldest].FailCount()
		w.preTimeout -= w.buckets[w.oldest].TimeoutCount()
		w.oldest++
		if w.oldest >= len(w.buckets) {
			w.oldest -= len(w.buckets)
		}
	}
	w.mutex.Unlock()
}

func (w *Window) Succeed() {
	w.mutex.RLock()
	w.buckets[w.latest].Succeed()
	w.mutex.RUnlock()
}

func (w *Window) SuccessCount() int64 {
	w.mutex.RLock()
	ret := w.buckets[w.latest].SuccessCount() + w.preSuccess
	w.mutex.RUnlock()
	return ret
}

func (w *Window) Fail() {
	w.mutex.RLock()
	w.buckets[w.latest].Fail()
	w.mutex.RUnlock()
}

func (w *Window) FailCount() int64 {
	w.mutex.RLock()
	ret := w.buckets[w.latest].FailCount() + w.preFail
	w.mutex.RUnlock()
	return ret
}

func (w *Window) Timeout() {
	w.mutex.RLock()
	w.buckets[w.latest].Timeout()
	w.mutex.RUnlock()
}

func (w *Window) TimeoutCount() int64 {
	w.mutex.RLock()
	ret := w.buckets[w.latest].TimeoutCount() + w.preTimeout
	w.mutex.RUnlock()
	return ret
}

// SuccessRate = success / total
func (w *Window) SuccessRate() float32 {
	w.mutex.RLock()
	defer w.mutex.RUnlock()
	succ := w.buckets[w.latest].SuccessCount() + w.preSuccess
	fail := w.buckets[w.latest].FailCount() + w.preFail
	timeout := w.buckets[w.latest].TimeoutCount() + w.preTimeout
	total := succ + fail + timeout
	if total == 0 {
		return float32(0)
	}
	ret := float32(succ) / float32(total)
	return ret
}

// ErrorRate = (fail + timeout) / total
func (w *Window) ErrorRate() float32 {
	w.mutex.RLock()
	defer w.mutex.RUnlock()
	succ := w.buckets[w.latest].SuccessCount() + w.preSuccess
	fail := w.buckets[w.latest].FailCount() + w.preFail
	timeout := w.buckets[w.latest].TimeoutCount() + w.preTimeout
	total := succ + fail + timeout
	if total == 0 {
		return float32(0)
	}
	ret := float32(fail+timeout) / float32(total)
	return ret
}
