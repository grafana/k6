package taskqueue_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"go.k6.io/k6/v2/internal/js/taskqueue"
	"go.k6.io/k6/v2/js/modulestest"
)

func TestTaskQueue(t *testing.T) {
	// really basic test
	t.Parallel()
	tr := modulestest.NewRuntime(t)
	fq := taskqueue.New(tr.EventLoop.RegisterCallback)
	var i int
	require.NoError(t, tr.VU.Runtime().Set("a", func() {
		fq.Queue(func() error {
			fq.Queue(func() error {
				fq.Queue(func() error {
					i++
					fq.Close()
					return nil
				})
				i++
				return nil
			})
			i++
			return nil
		})
	}))

	err := tr.EventLoop.Start(func() error {
		_, err := tr.VU.Runtime().RunString(`a()`)
		return err
	})
	require.NoError(t, err)
	require.Equal(t, i, 3)
}

func TestTwoTaskQueues(t *testing.T) {
	// try to find any kind of races through running multiple queues and having them race with each other
	t.Parallel()
	tr := modulestest.NewRuntime(t)
	ctx, cancel := context.WithTimeout(tr.VU.Context(), time.Millisecond*100)
	t.Cleanup(cancel)
	tr.VU.CtxField = ctx

	rt := tr.VU.Runtime()
	fq := taskqueue.New(tr.EventLoop.RegisterCallback)
	fq2 := taskqueue.New(tr.EventLoop.RegisterCallback)
	var i int
	incrimentI := func() { i++ }
	var j int
	incrimentJ := func() { j++ }
	var k int
	incrimentK := func() { k++ }

	require.NoError(t, rt.Set("a", func() {
		for range 5 { // make multiple goroutines
			go func() {
				for range 1000000 {
					fq.Queue(func() error { // queue a task to increment integers
						incrimentI()
						incrimentJ()
						return nil
					})
					time.Sleep(time.Millisecond) // this is here mostly to not get a goroutine that just loops
					select {
					case <-ctx.Done():
						return
					default:
					}
				}
			}()
			go func() { // same as above but with the other queue
				for range 1000000 {
					fq2.Queue(func() error {
						incrimentI()
						incrimentK()
						return nil
					})
					time.Sleep(time.Millisecond)
					select {
					case <-ctx.Done():
						return
					default:
					}
				}
			}()
		}
	}))

	go func() {
		<-ctx.Done()
		fq.Close()
		fq2.Close()
	}()

	err := tr.EventLoop.Start(func() error {
		_, err := tr.VU.Runtime().RunString(`a()`)
		return err
	})
	require.NoError(t, err)
	tr.EventLoop.WaitOnRegistered()
	require.Equal(t, i, k+j)
	require.Greater(t, k, 100)
	require.Greater(t, j, 100)
}
