package antch

import (
	"io"
	"sync"
)

// Queue is a URLs list manager.
type Queue interface {
	Enqueue(string)
	Dequeue() (string, error)
}

// SimpleHeapQueue is a simple FIFO URLs queue.
type SimpleHeapQueue struct {
	mu   sync.Mutex
	data []string
}

func (q *SimpleHeapQueue) Enqueue(urlStr string) {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.data = append(q.data, urlStr)
}

func (q *SimpleHeapQueue) Dequeue() (string, error) {
	q.mu.Lock()
	defer q.mu.Unlock()

	if len(q.data) == 0 {
		return "", io.EOF
	}

	var urlStr string
	urlStr, q.data = q.data[0], q.data[1:]
	return urlStr, nil
}
