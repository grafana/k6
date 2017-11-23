package goja

import (
	"regexp"
	"time"
)

const (
	dateTimeLayout       = "Mon Jan 02 2006 15:04:05 GMT-0700 (MST)"
	isoDateTimeLayout    = "2006-01-02T15:04:05.000Z"
	dateLayout           = "Mon Jan 02 2006"
	timeLayout           = "15:04:05 GMT-0700 (MST)"
	datetimeLayout_en_GB = "01/02/2006, 15:04:05"
	dateLayout_en_GB     = "01/02/2006"
	timeLayout_en_GB     = "15:04:05"
)

type dateObject struct {
	baseObject
	time  time.Time
	isSet bool
}

var (
	dateLayoutList = []string{
		"2006",
		"2006-01",
		"2006-01-02",

		"2006T15:04",
		"2006-01T15:04",
		"2006-01-02T15:04",

		"2006T15:04:05",
		"2006-01T15:04:05",
		"2006-01-02T15:04:05",

		"2006T15:04:05.000",
		"2006-01T15:04:05.000",
		"2006-01-02T15:04:05.000",

		"2006T15:04-0700",
		"2006-01T15:04-0700",
		"2006-01-02T15:04-0700",

		"2006T15:04:05-0700",
		"2006-01T15:04:05-0700",
		"2006-01-02T15:04:05-0700",

		"2006T15:04:05.000-0700",
		"2006-01T15:04:05.000-0700",
		"2006-01-02T15:04:05.000-0700",

		time.RFC1123,
		dateTimeLayout,
	}
	matchDateTimeZone = regexp.MustCompile(`^(.*)(?:(Z)|([\+\-]\d{2}):(\d{2}))$`)
)

func dateParse(date string) (time.Time, bool) {
	// YYYY-MM-DDTHH:mm:ss.sssZ
	var t time.Time
	var err error
	{
		date := date
		if match := matchDateTimeZone.FindStringSubmatch(date); match != nil {
			if match[2] == "Z" {
				date = match[1] + "+0000"
			} else {
				date = match[1] + match[3] + match[4]
			}
		}
		for _, layout := range dateLayoutList {
			t, err = time.Parse(layout, date)
			if err == nil {
				break
			}
		}
	}
	return t, err == nil
}

func (r *Runtime) newDateObject(t time.Time, isSet bool) *Object {
	v := &Object{runtime: r}
	d := &dateObject{}
	v.self = d
	d.val = v
	d.class = classDate
	d.prototype = r.global.DatePrototype
	d.extensible = true
	d.init()
	d.time = t.In(time.Local)
	d.isSet = isSet
	return v
}

func dateFormat(t time.Time) string {
	return t.Local().Format(dateTimeLayout)
}

func (d *dateObject) toPrimitive() Value {
	return d.toPrimitiveString()
}

func (d *dateObject) export() interface{} {
	if d.isSet {
		return d.time
	}
	return nil
}
