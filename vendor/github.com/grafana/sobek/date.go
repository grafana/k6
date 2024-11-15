package sobek

import (
	"math"
	"reflect"
	"time"
)

const (
	dateTimeLayout       = "Mon Jan 02 2006 15:04:05 GMT-0700 (MST)"
	utcDateTimeLayout    = "Mon, 02 Jan 2006 15:04:05 GMT"
	isoDateTimeLayout    = "2006-01-02T15:04:05.000Z"
	dateLayout           = "Mon Jan 02 2006"
	timeLayout           = "15:04:05 GMT-0700 (MST)"
	datetimeLayout_en_GB = "01/02/2006, 15:04:05"
	dateLayout_en_GB     = "01/02/2006"
	timeLayout_en_GB     = "15:04:05"

	maxTime   = 8.64e15
	timeUnset = math.MinInt64
)

type dateObject struct {
	baseObject
	msec int64
}

func dateParse(date string) (t time.Time, ok bool) {
	d, ok := parseDateISOString(date)
	if !ok {
		d, ok = parseDateOtherString(date)
	}
	if !ok {
		return
	}
	if d.month > 12 ||
		d.day > 31 ||
		d.hour > 24 ||
		d.min > 59 ||
		d.sec > 59 ||
		// special case 24:00:00.000
		(d.hour == 24 && (d.min != 0 || d.sec != 0 || d.msec != 0)) {
		ok = false
		return
	}
	var loc *time.Location
	if d.isLocal {
		loc = time.Local
	} else {
		loc = time.FixedZone("", d.timeZoneOffset*60)
	}
	t = time.Date(d.year, time.Month(d.month), d.day, d.hour, d.min, d.sec, d.msec*1e6, loc)
	unixMilli := t.UnixMilli()
	ok = unixMilli >= -maxTime && unixMilli <= maxTime
	return
}

func (r *Runtime) newDateObject(t time.Time, isSet bool, proto *Object) *Object {
	v := &Object{runtime: r}
	d := &dateObject{}
	v.self = d
	d.val = v
	d.class = classDate
	d.prototype = proto
	d.extensible = true
	d.init()
	if isSet {
		d.msec = timeToMsec(t)
	} else {
		d.msec = timeUnset
	}
	return v
}

func dateFormat(t time.Time) string {
	return t.Local().Format(dateTimeLayout)
}

func timeFromMsec(msec int64) time.Time {
	sec := msec / 1000
	nsec := (msec % 1000) * 1e6
	return time.Unix(sec, nsec)
}

func timeToMsec(t time.Time) int64 {
	return t.Unix()*1000 + int64(t.Nanosecond())/1e6
}

func (d *dateObject) exportType() reflect.Type {
	return typeTime
}

func (d *dateObject) export(*objectExportCtx) interface{} {
	if d.isSet() {
		return d.time()
	}
	return nil
}

func (d *dateObject) setTimeMs(ms int64) Value {
	if ms >= 0 && ms <= maxTime || ms < 0 && ms >= -maxTime {
		d.msec = ms
		return intToValue(ms)
	}

	d.unset()
	return _NaN
}

func (d *dateObject) isSet() bool {
	return d.msec != timeUnset
}

func (d *dateObject) unset() {
	d.msec = timeUnset
}

func (d *dateObject) time() time.Time {
	return timeFromMsec(d.msec)
}

func (d *dateObject) timeUTC() time.Time {
	return timeFromMsec(d.msec).In(time.UTC)
}
