package sequence

import "container/heap"

type PriorityItem[T any] struct {
	Value    T
	Priority int
	index    int
}

type priorityQueue[T any] struct {
	items []*PriorityItem[T]
}

func (pq *priorityQueue[T]) Len() int {
	return len(pq.items)
}

func (pq *priorityQueue[T]) Less(i, j int) bool {
	return pq.items[i].Priority > pq.items[j].Priority
}

func (pq *priorityQueue[T]) Swap(i, j int) {
	pq.items[i], pq.items[j] = pq.items[j], pq.items[i]
	pq.items[i].index = i
	pq.items[j].index = j
}

func (pq *priorityQueue[T]) Push(x any) {
	item := x.(*PriorityItem[T])
	item.index = len(pq.items)
	pq.items = append(pq.items, item)
}

func (pq *priorityQueue[T]) Pop() any {
	old := pq.items
	n := len(old)
	item := old[n-1]
	old[n-1] = nil  // avoid memory leak
	item.index = -1 // for safety
	pq.items = old[0 : n-1]
	return item
}

type PriorityQueue[T any] struct {
	pq priorityQueue[T]
}

func NewPriorityQueue[T any]() *PriorityQueue[T] {
	pq := &PriorityQueue[T]{}
	heap.Init(&pq.pq)
	return pq
}

func (pq *PriorityQueue[T]) Enqueue(value T, priority int) *PriorityItem[T] {
	item := &PriorityItem[T]{
		Value:    value,
		Priority: priority,
	}
	heap.Push(&pq.pq, item)
	return item
}

func (pq *PriorityQueue[T]) Dequeue() (T, bool) {
	if pq.pq.Len() == 0 {
		var zero T
		return zero, false
	}
	item := heap.Pop(&pq.pq).(*PriorityItem[T])
	return item.Value, true
}

func (pq *PriorityQueue[T]) Peek() (T, bool) {
	if pq.pq.Len() == 0 {
		var zero T
		return zero, false
	}
	return pq.pq.items[0].Value, true
}

func (pq *PriorityQueue[T]) Update(item *PriorityItem[T], value T, priority int) {
	item.Value = value
	item.Priority = priority
	heap.Fix(&pq.pq, item.index)
}

func (pq *PriorityQueue[T]) Len() int {
	return pq.pq.Len()
}

func (pq *PriorityQueue[T]) IsEmpty() bool {
	return pq.pq.Len() == 0
}
