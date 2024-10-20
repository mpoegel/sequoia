package collect

type Broker[T any] struct {
	stopC      chan struct{}
	broadcastC chan T
	subC       chan chan T
	unsubC     chan chan T
	isStopped  bool
}

func NewBroker[T any]() *Broker[T] {
	b := &Broker[T]{
		stopC:      make(chan struct{}),
		broadcastC: make(chan T, 1),
		subC:       make(chan chan T, 1),
		unsubC:     make(chan chan T, 1),
		isStopped:  true,
	}
	return b
}

func (b *Broker[T]) Start() {
	b.isStopped = false
	subs := map[chan T]bool{}
	for {
		select {
		case <-b.stopC:
			for c := range subs {
				close(c)
			}
			return
		case newC := <-b.subC:
			subs[newC] = true
		case oldC := <-b.unsubC:
			delete(subs, oldC)
			close(oldC)
		case msg := <-b.broadcastC:
			for subbedC := range subs {
				// non-blocking broadcast
				select {
				case subbedC <- msg:
				default:
				}
			}
		}
	}
}

func (b *Broker[T]) Stop() {
	b.isStopped = true
	close(b.stopC)
	close(b.broadcastC)
	close(b.subC)
	close(b.unsubC)
}

func (b *Broker[T]) Subscribe() chan T {
	if !b.isStopped {
		newC := make(chan T, 5)
		b.subC <- newC
		return newC
	}
	return nil
}

func (b *Broker[T]) Unsubscribe(oldC chan T) {
	if !b.isStopped {
		b.unsubC <- oldC
	}
}

func (b *Broker[T]) Broadcast(msg T) {
	if !b.isStopped {
		b.broadcastC <- msg
	}
}
