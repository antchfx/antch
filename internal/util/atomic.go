package util

import (
	"strconv"
	"sync/atomic"
)

// An AtomicBool is an boolean value updated atomically.
type AtomicBool int32

func (b *AtomicBool) IsSet() bool {
	return atomic.LoadInt32((*int32)(b)) != 0
}

func (b *AtomicBool) SetTrue() {
	atomic.StoreInt32((*int32)(b), 1)
}

// An AtomicInt is an int64 to be accessed atomically.
type AtomicInt int64

func (i *AtomicInt) Add(n int64) {
	atomic.AddInt64((*int64)(i), n)
}

func (i *AtomicInt) Get() int64 {
	return atomic.LoadInt64((*int64)(i))
}

func (i *AtomicInt) String() string {
	return strconv.FormatInt(i.Get(), 10)
}
