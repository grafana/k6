package dns

import (
	"context"
	"math/rand"
	"sync"
	"time"
)

// Cache is a DNS query cache handler.
type Cache struct {
	mu    sync.RWMutex
	cache map[Question]*Message
}

// ServeDNS answers query questions from a local cache, and forwards unanswered
// questions upstream, then caches the answers from the response.
func (c *Cache) ServeDNS(ctx context.Context, w MessageWriter, r *Query) {
	var (
		miss bool

		now = time.Now()
	)

	c.mu.RLock()
	for _, q := range r.Questions {
		if hit := c.lookup(q, w, now); !hit {
			miss = true
		}
	}
	c.mu.RUnlock()

	if !miss {
		return
	}

	msg, err := w.Recur(ctx)
	if err != nil || msg == nil {
		w.Status(ServFail)
		return
	}
	if msg.RCode == NoError {
		c.insert(msg, now)
	}
	writeMessage(w, msg)
}

// c.mu.RLock held
func (c *Cache) lookup(q Question, w MessageWriter, now time.Time) bool {
	msg, ok := c.cache[q]
	if !ok {
		return false
	}

	var answers, authorities, additionals []Resource

	for _, res := range msg.Answers {
		if res.TTL = cacheTTL(res.TTL, now); res.TTL <= 0 {
			return false
		}

		answers = append(answers, res)
	}
	for _, res := range msg.Authorities {
		if res.TTL = cacheTTL(res.TTL, now); res.TTL <= 0 {
			return false
		}

		authorities = append(authorities, res)
	}
	for _, res := range msg.Additionals {
		if res.TTL = cacheTTL(res.TTL, now); res.TTL <= 0 {
			return false
		}

		additionals = append(additionals, res)
	}

	randomize(answers)
	for _, res := range answers {
		w.Answer(res.Name, res.TTL, res.Record)
	}
	for _, res := range authorities {
		w.Authority(res.Name, res.TTL, res.Record)
	}
	for _, res := range additionals {
		w.Additional(res.Name, res.TTL, res.Record)
	}

	return true
}

func (c *Cache) insert(msg *Message, now time.Time) {
	cache := make(map[Question]*Message, len(msg.Questions))
	for _, q := range msg.Questions {
		m := new(Message)
		for _, res := range msg.Answers {
			res.TTL = cacheEpoch(res.TTL, now)
			m.Answers = append(m.Answers, res)
		}
		for _, res := range msg.Authorities {
			res.TTL = cacheEpoch(res.TTL, now)
			m.Authorities = append(m.Authorities, res)
		}
		for _, res := range msg.Additionals {
			res.TTL = cacheEpoch(res.TTL, now)
			m.Additionals = append(m.Additionals, res)
		}

		cache[q] = m
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if c.cache == nil {
		c.cache = cache
		return
	}

	for q, m := range cache {
		c.cache[q] = m
	}
}

func cacheEpoch(ttl time.Duration, now time.Time) time.Duration {
	return time.Duration(now.Add(ttl).UnixNano())
}

func cacheTTL(epoch time.Duration, now time.Time) time.Duration {
	return time.Unix(0, int64(epoch)).Sub(now)
}

// randomize shuffles contigous groups of resourcesfor the same name.
func randomize(s []Resource) {
	var low, high int
	for low = 0; low < len(s)-1; low++ {
		for high = low + 1; high < len(s) && s[low].Name == s[high].Name; high++ {
		}

		shuffle(s[low:high])
		low = high
	}
}

func shuffle(s []Resource) {
	if len(s) < 2 {
		return
	}

	for i := len(s) - 1; i > 0; i-- {
		j := rand.Intn(i + 1)
		s[i], s[j] = s[j], s[i]
	}
}
