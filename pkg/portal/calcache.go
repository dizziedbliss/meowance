package portal

import (
	"sync"
	"time"
)

// calendarTTL is how long a warm instance reuses a semester's calendar. The
// calendar of events is the same page for every student of that semester and
// holds nothing personal (dates, holidays, exams, events), so only the first
// login per semester pays the two extra page fetches.
const calendarTTL = 6 * time.Hour

// calendarCache keeps parsed calendars by which COE page they came from. The
// cached *Calendar is shared between responses and never modified.
type calendarCache struct {
	mu sync.Mutex
	m  map[string]cachedCalendar
}

type cachedCalendar struct {
	cal *Calendar
	at  time.Time
}

func (cc *calendarCache) get(key string, now time.Time) *Calendar {
	cc.mu.Lock()
	defer cc.mu.Unlock()
	if e, ok := cc.m[key]; ok && now.Sub(e.at) < calendarTTL {
		return e.cal
	}
	return nil
}

// put stores a parsed calendar; a failed parse (nil) is never cached.
func (cc *calendarCache) put(key string, cal *Calendar, now time.Time) {
	if cal == nil {
		return
	}
	cc.mu.Lock()
	defer cc.mu.Unlock()
	if cc.m == nil {
		cc.m = map[string]cachedCalendar{}
	}
	for k, e := range cc.m { // a handful of semesters at most; drop the expired ones
		if now.Sub(e.at) >= calendarTTL {
			delete(cc.m, k)
		}
	}
	cc.m[key] = cachedCalendar{cal, now}
}
