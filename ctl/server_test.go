package ctl

import (
	"context"
	"reflect"
	"testing"
	"time"
)

func TestRangeSelect(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	cases := []reflect.SelectCase{
		{
			Dir:  reflect.SelectRecv,
			Chan: reflect.ValueOf(make(chan int, 1)),
		},
		{
			Dir:  reflect.SelectRecv,
			Chan: reflect.ValueOf(make(chan int, 1)),
		},
		{
			Dir:  reflect.SelectRecv,
			Chan: reflect.ValueOf(ctx.Done()),
		},
	}

	go func() {
		<-time.After(time.Second * 5)
		cancel()
	}()

	start := time.Now()
	idx, v, ok := reflect.Select(cases)

	t.Log(idx, v, ok, time.Since(start))
}
