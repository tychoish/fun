package pubsub

import "math"

type queueLimitTracker interface {
	len() int
	cap() int
	add() error
	remove()
}

type queueNoLimitTrackerImpl struct {
	length int
}

func (q *queueNoLimitTrackerImpl) len() int   { return q.length }
func (q *queueNoLimitTrackerImpl) cap() int   { return math.MaxInt }
func (q *queueNoLimitTrackerImpl) add() error { q.length++; return nil }
func (q *queueNoLimitTrackerImpl) remove()    { q.length-- }

type queueHardLimitTracker struct {
	capacity int
	length   int
}

func (q *queueHardLimitTracker) len() int { return q.length }
func (q *queueHardLimitTracker) cap() int { return q.capacity }
func (q *queueHardLimitTracker) remove() {
	if q.length == 0 {
		return
	}

	q.length--
}

func (q *queueHardLimitTracker) add() error {
	if q.length >= q.capacity {
		return ErrQueueFull
	}
	q.length++
	return nil
}

type queueLimitTrackerImpl struct {
	softQuota int     // adjusted dynamically (see Add, Remove)
	hardLimit int     // fixed for the lifespan of the queue
	length    int     // number of entries in the queue list
	credit    float64 // current burst credit
}

func newQueueLimitTracker(opts QueueOptions) queueLimitTracker {
	return &queueLimitTrackerImpl{
		softQuota: opts.SoftQuota,
		hardLimit: opts.HardLimit,
		credit:    opts.BurstCredit,
	}
}

func (q *queueLimitTrackerImpl) cap() int { return q.softQuota }
func (q *queueLimitTrackerImpl) len() int { return q.length }
func (q *queueLimitTrackerImpl) add() error {
	if q.length >= q.softQuota {
		if q.length == q.hardLimit {
			return ErrQueueFull
		} else if q.credit < 1 {
			return ErrQueueNoCredit
		}

		// Successfully exceeding the soft quota deducts burst credit and raises
		// the soft quota. This has the effect of reducing the credit cap and the
		// amount of credit given for removing items to better approximate the
		// rate at which the consumer is servicing the queue.
		q.credit--
		q.softQuota = q.length + 1
	}

	q.length++
	return nil
}

func (q *queueLimitTrackerImpl) remove() {
	q.length--

	if q.length < q.softQuota {
		// Successfully removing items from the queue below half the soft quota
		// lowers the soft quota. This has the effect of increasing the credit cap
		// and the amount of credit given for removing items to better approximate
		// the rate at which the consumer is servicing the queue.
		if q.softQuota > 1 && q.length < q.softQuota/2 {
			q.softQuota--
		}

		// Give credit for being below the soft quota. Note we do this after
		// adjusting the quota so the credit reflects the item we just removed.
		q.credit += float64(q.softQuota-q.length) / float64(q.softQuota)
		if cap := float64(q.hardLimit - q.softQuota); q.credit > cap {
			q.credit = cap
		}
	}

}
