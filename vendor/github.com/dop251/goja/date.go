package goja

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

type dateLayoutDesc struct {
	layout   string
	dateOnly bool
}

var (
	dateLayoutsNumeric = []dateLayoutDesc{
		{layout: "2006-01-02T15:04:05Z0700"},
		{layout: "2006-01-02T15:04:05"},
		{layout: "2006-01-02", dateOnly: true},
		{layout: "2006-01-02 15:04:05"},

		{layout: "2006", dateOnly: true},
		{layout: "2006-01", dateOnly: true},

		{layout: "2006T15:04"},
		{layout: "2006-01T15:04"},
		{layout: "2006-01-02T15:04"},

		{layout: "2006T15:04:05"},
		{layout: "2006-01T15:04:05"},

		{layout: "2006T15:04Z0700"},
		{layout: "2006-01T15:04Z0700"},
		{layout: "2006-01-02T15:04Z0700"},

		{layout: "2006T15:04:05Z0700"},
		{layout: "2006-01T15:04:05Z0700"},
	}

	dateLayoutsAlpha = []dateLayoutDesc{
		{layout: time.RFC1123},
		{layout: time.RFC1123Z},
		{layout: dateTimeLayout},
		{layout: time.UnixDate},
		{layout: time.ANSIC},
		{layout: time.RubyDate},
		{layout: "Mon, _2 Jan 2006 15:04:05 GMT-0700 (MST)"},
		{layout: "Mon, _2 Jan 2006 15:04:05 -0700 (MST)"},
		{layout: "Jan _2, 2006", dateOnly: true},
	}
)

func dateParse(date string) (time.Time, bool) {
	var t time.Time
	var err error
	var layouts []dateLayoutDesc
	if len(date) > 0 {
		first := date[0]
		if first <= '9' && (first >= '0' || first == '-' || first == '+') {
			layouts = dateLayoutsNumeric
		} else {
			layouts = dateLayoutsAlpha
		}
	} else {
		return time.Time{}, false
	}
	for _, desc := range layouts {
		var defLoc *time.Location
		if desc.dateOnly {
			defLoc = time.UTC
		} else {
			defLoc = time.Local
		}
		t, err = parseDate(desc.layout, date, defLoc)
		if err == nil {
			break
		}
	}
	if err != nil {
		return time.Time{}, false
	}
	unix := timeToMsec(t)
	return t, unix >= -maxTime && unix <= maxTime
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

func (d *dateObject) toPrimitive() Value {
	return d.toPrimitiveString()
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
