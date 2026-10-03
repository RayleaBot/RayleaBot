package onebot11

import "time"

type eventExpiry struct {
	id string
	at time.Time
}

// Observations from different transports can reach admission out of timestamp
// order. A min-heap expires only due records without scanning live event IDs.
// The owning Shell protects both this queue and the lookup map with dedupMu.
type eventExpiryQueue []eventExpiry

func (q *eventExpiryQueue) push(entry eventExpiry) {
	*q = append(*q, entry)
	for child := len(*q) - 1; child > 0; {
		parent := (child - 1) / 2
		if !(*q)[child].at.Before((*q)[parent].at) {
			break
		}
		(*q)[child], (*q)[parent] = (*q)[parent], (*q)[child]
		child = parent
	}
}

func (q *eventExpiryQueue) pop() eventExpiry {
	items := *q
	first := items[0]
	last := len(items) - 1
	items[0] = items[last]
	items[last] = eventExpiry{}
	items = items[:last]
	for parent := 0; parent*2+1 < len(items); {
		child := parent*2 + 1
		if right := child + 1; right < len(items) && items[right].at.Before(items[child].at) {
			child = right
		}
		if !items[child].at.Before(items[parent].at) {
			break
		}
		items[child], items[parent] = items[parent], items[child]
		parent = child
	}
	*q = items
	return first
}
