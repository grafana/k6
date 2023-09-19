package goja

import (
	"fmt"
	"math"
	"sync"
	"time"
)

func (r *Runtime) makeDate(args []Value, utc bool) (t time.Time, valid bool) {
	switch {
	case len(args) >= 2:
		t = time.Date(1970, time.January, 1, 0, 0, 0, 0, time.Local)
		t, valid = _dateSetYear(t, FunctionCall{Arguments: args}, 0, utc)
	case len(args) == 0:
		t = r.now()
		valid = true
	default: // one argument
		if o, ok := args[0].(*Object); ok {
			if d, ok := o.self.(*dateObject); ok {
				t = d.time()
				valid = true
			}
		}
		if !valid {
			pv := toPrimitive(args[0])
			if val, ok := pv.(String); ok {
				return dateParse(val.String())
			}
			pv = pv.ToNumber()
			var n int64
			if i, ok := pv.(valueInt); ok {
				n = int64(i)
			} else if f, ok := pv.(valueFloat); ok {
				f := float64(f)
				if math.IsNaN(f) || math.IsInf(f, 0) {
					return
				}
				if math.Abs(f) > maxTime {
					return
				}
				n = int64(f)
			} else {
				n = pv.ToInteger()
			}
			t = timeFromMsec(n)
			valid = true
		}
	}
	if valid {
		msec := t.Unix()*1000 + int64(t.Nanosecond()/1e6)
		if msec < 0 {
			msec = -msec
		}
		if msec > maxTime {
			valid = false
		}
	}
	return
}

func (r *Runtime) newDateTime(args []Value, proto *Object) *Object {
	t, isSet := r.makeDate(args, false)
	return r.newDateObject(t, isSet, proto)
}

func (r *Runtime) builtin_newDate(args []Value, proto *Object) *Object {
	return r.newDateTime(args, proto)
}

func (r *Runtime) builtin_date(FunctionCall) Value {
	return asciiString(dateFormat(r.now()))
}

func (r *Runtime) date_parse(call FunctionCall) Value {
	t, set := dateParse(call.Argument(0).toString().String())
	if set {
		return intToValue(timeToMsec(t))
	}
	return _NaN
}

func (r *Runtime) date_UTC(call FunctionCall) Value {
	var args []Value
	if len(call.Arguments) < 2 {
		args = []Value{call.Argument(0), _positiveZero}
	} else {
		args = call.Arguments
	}
	t, valid := r.makeDate(args, true)
	if !valid {
		return _NaN
	}
	return intToValue(timeToMsec(t))
}

func (r *Runtime) date_now(FunctionCall) Value {
	return intToValue(timeToMsec(r.now()))
}

func (r *Runtime) dateproto_toString(call FunctionCall) Value {
	obj := r.toObject(call.This)
	if d, ok := obj.self.(*dateObject); ok {
		if d.isSet() {
			return asciiString(d.time().Format(dateTimeLayout))
		} else {
			return stringInvalidDate
		}
	}
	panic(r.NewTypeError("Method Date.prototype.toString is called on incompatible receiver"))
}

func (r *Runtime) dateproto_toUTCString(call FunctionCall) Value {
	obj := r.toObject(call.This)
	if d, ok := obj.self.(*dateObject); ok {
		if d.isSet() {
			return asciiString(d.timeUTC().Format(utcDateTimeLayout))
		} else {
			return stringInvalidDate
		}
	}
	panic(r.NewTypeError("Method Date.prototype.toUTCString is called on incompatible receiver"))
}

func (r *Runtime) dateproto_toISOString(call FunctionCall) Value {
	obj := r.toObject(call.This)
	if d, ok := obj.self.(*dateObject); ok {
		if d.isSet() {
			utc := d.timeUTC()
			year := utc.Year()
			if year >= -9999 && year <= 9999 {
				return asciiString(utc.Format(isoDateTimeLayout))
			}
			// extended year
			return asciiString(fmt.Sprintf("%+06d-", year) + utc.Format(isoDateTimeLayout[5:]))
		} else {
			panic(r.newError(r.getRangeError(), "Invalid time value"))
		}
	}
	panic(r.NewTypeError("Method Date.prototype.toISOString is called on incompatible receiver"))
}

func (r *Runtime) dateproto_toJSON(call FunctionCall) Value {
	obj := call.This.ToObject(r)
	tv := obj.toPrimitiveNumber()
	if f, ok := tv.(valueFloat); ok {
		f := float64(f)
		if math.IsNaN(f) || math.IsInf(f, 0) {
			return _null
		}
	}

	if toISO, ok := obj.self.getStr("toISOString", nil).(*Object); ok {
		if toISO, ok := toISO.self.assertCallable(); ok {
			return toISO(FunctionCall{
				This: obj,
			})
		}
	}

	panic(r.NewTypeError("toISOString is not a function"))
}

func (r *Runtime) dateproto_toPrimitive(call FunctionCall) Value {
	o := r.toObject(call.This)
	arg := call.Argument(0)

	if asciiString("string").StrictEquals(arg) || asciiString("default").StrictEquals(arg) {
		return o.ordinaryToPrimitiveString()
	}
	if asciiString("number").StrictEquals(arg) {
		return o.ordinaryToPrimitiveNumber()
	}
	panic(r.NewTypeError("Invalid hint: %s", arg))
}

func (r *Runtime) dateproto_toDateString(call FunctionCall) Value {
	obj := r.toObject(call.This)
	if d, ok := obj.self.(*dateObject); ok {
		if d.isSet() {
			return asciiString(d.time().Format(dateLayout))
		} else {
			return stringInvalidDate
		}
	}
	panic(r.NewTypeError("Method Date.prototype.toDateString is called on incompatible receiver"))
}

func (r *Runtime) dateproto_toTimeString(call FunctionCall) Value {
	obj := r.toObject(call.This)
	if d, ok := obj.self.(*dateObject); ok {
		if d.isSet() {
			return asciiString(d.time().Format(timeLayout))
		} else {
			return stringInvalidDate
		}
	}
	panic(r.NewTypeError("Method Date.prototype.toTimeString is called on incompatible receiver"))
}

func (r *Runtime) dateproto_toLocaleString(call FunctionCall) Value {
	obj := r.toObject(call.This)
	if d, ok := obj.self.(*dateObject); ok {
		if d.isSet() {
			return asciiString(d.time().Format(datetimeLayout_en_GB))
		} else {
			return stringInvalidDate
		}
	}
	panic(r.NewTypeError("Method Date.prototype.toLocaleString is called on incompatible receiver"))
}

func (r *Runtime) dateproto_toLocaleDateString(call FunctionCall) Value {
	obj := r.toObject(call.This)
	if d, ok := obj.self.(*dateObject); ok {
		if d.isSet() {
			return asciiString(d.time().Format(dateLayout_en_GB))
		} else {
			return stringInvalidDate
		}
	}
	panic(r.NewTypeError("Method Date.prototype.toLocaleDateString is called on incompatible receiver"))
}

func (r *Runtime) dateproto_toLocaleTimeString(call FunctionCall) Value {
	obj := r.toObject(call.This)
	if d, ok := obj.self.(*dateObject); ok {
		if d.isSet() {
			return asciiString(d.time().Format(timeLayout_en_GB))
		} else {
			return stringInvalidDate
		}
	}
	panic(r.NewTypeError("Method Date.prototype.toLocaleTimeString is called on incompatible receiver"))
}

func (r *Runtime) dateproto_valueOf(call FunctionCall) Value {
	obj := r.toObject(call.This)
	if d, ok := obj.self.(*dateObject); ok {
		if d.isSet() {
			return intToValue(d.msec)
		} else {
			return _NaN
		}
	}
	panic(r.NewTypeError("Method Date.prototype.valueOf is called on incompatible receiver"))
}

func (r *Runtime) dateproto_getTime(call FunctionCall) Value {
	obj := r.toObject(call.This)
	if d, ok := obj.self.(*dateObject); ok {
		if d.isSet() {
			return intToValue(d.msec)
		} else {
			return _NaN
		}
	}
	panic(r.NewTypeError("Method Date.prototype.getTime is called on incompatible receiver"))
}

func (r *Runtime) dateproto_getFullYear(call FunctionCall) Value {
	obj := r.toObject(call.This)
	if d, ok := obj.self.(*dateObject); ok {
		if d.isSet() {
			return intToValue(int64(d.time().Year()))
		} else {
			return _NaN
		}
	}
	panic(r.NewTypeError("Method Date.prototype.getFullYear is called on incompatible receiver"))
}

func (r *Runtime) dateproto_getUTCFullYear(call FunctionCall) Value {
	obj := r.toObject(call.This)
	if d, ok := obj.self.(*dateObject); ok {
		if d.isSet() {
			return intToValue(int64(d.timeUTC().Year()))
		} else {
			return _NaN
		}
	}
	panic(r.NewTypeError("Method Date.prototype.getUTCFullYear is called on incompatible receiver"))
}

func (r *Runtime) dateproto_getMonth(call FunctionCall) Value {
	obj := r.toObject(call.This)
	if d, ok := obj.self.(*dateObject); ok {
		if d.isSet() {
			return intToValue(int64(d.time().Month()) - 1)
		} else {
			return _NaN
		}
	}
	panic(r.NewTypeError("Method Date.prototype.getMonth is called on incompatible receiver"))
}

func (r *Runtime) dateproto_getUTCMonth(call FunctionCall) Value {
	obj := r.toObject(call.This)
	if d, ok := obj.self.(*dateObject); ok {
		if d.isSet() {
			return intToValue(int64(d.timeUTC().Month()) - 1)
		} else {
			return _NaN
		}
	}
	panic(r.NewTypeError("Method Date.prototype.getUTCMonth is called on incompatible receiver"))
}

func (r *Runtime) dateproto_getHours(call FunctionCall) Value {
	obj := r.toObject(call.This)
	if d, ok := obj.self.(*dateObject); ok {
		if d.isSet() {
			return intToValue(int64(d.time().Hour()))
		} else {
			return _NaN
		}
	}
	panic(r.NewTypeError("Method Date.prototype.getHours is called on incompatible receiver"))
}

func (r *Runtime) dateproto_getUTCHours(call FunctionCall) Value {
	obj := r.toObject(call.This)
	if d, ok := obj.self.(*dateObject); ok {
		if d.isSet() {
			return intToValue(int64(d.timeUTC().Hour()))
		} else {
			return _NaN
		}
	}
	panic(r.NewTypeError("Method Date.prototype.getUTCHours is called on incompatible receiver"))
}

func (r *Runtime) dateproto_getDate(call FunctionCall) Value {
	obj := r.toObject(call.This)
	if d, ok := obj.self.(*dateObject); ok {
		if d.isSet() {
			return intToValue(int64(d.time().Day()))
		} else {
			return _NaN
		}
	}
	panic(r.NewTypeError("Method Date.prototype.getDate is called on incompatible receiver"))
}

func (r *Runtime) dateproto_getUTCDate(call FunctionCall) Value {
	obj := r.toObject(call.This)
	if d, ok := obj.self.(*dateObject); ok {
		if d.isSet() {
			return intToValue(int64(d.timeUTC().Day()))
		} else {
			return _NaN
		}
	}
	panic(r.NewTypeError("Method Date.prototype.getUTCDate is called on incompatible receiver"))
}

func (r *Runtime) dateproto_getDay(call FunctionCall) Value {
	obj := r.toObject(call.This)
	if d, ok := obj.self.(*dateObject); ok {
		if d.isSet() {
			return intToValue(int64(d.time().Weekday()))
		} else {
			return _NaN
		}
	}
	panic(r.NewTypeError("Method Date.prototype.getDay is called on incompatible receiver"))
}

func (r *Runtime) dateproto_getUTCDay(call FunctionCall) Value {
	obj := r.toObject(call.This)
	if d, ok := obj.self.(*dateObject); ok {
		if d.isSet() {
			return intToValue(int64(d.timeUTC().Weekday()))
		} else {
			return _NaN
		}
	}
	panic(r.NewTypeError("Method Date.prototype.getUTCDay is called on incompatible receiver"))
}

func (r *Runtime) dateproto_getMinutes(call FunctionCall) Value {
	obj := r.toObject(call.This)
	if d, ok := obj.self.(*dateObject); ok {
		if d.isSet() {
			return intToValue(int64(d.time().Minute()))
		} else {
			return _NaN
		}
	}
	panic(r.NewTypeError("Method Date.prototype.getMinutes is called on incompatible receiver"))
}

func (r *Runtime) dateproto_getUTCMinutes(call FunctionCall) Value {
	obj := r.toObject(call.This)
	if d, ok := obj.self.(*dateObject); ok {
		if d.isSet() {
			return intToValue(int64(d.timeUTC().Minute()))
		} else {
			return _NaN
		}
	}
	panic(r.NewTypeError("Method Date.prototype.getUTCMinutes is called on incompatible receiver"))
}

func (r *Runtime) dateproto_getSeconds(call FunctionCall) Value {
	obj := r.toObject(call.This)
	if d, ok := obj.self.(*dateObject); ok {
		if d.isSet() {
			return intToValue(int64(d.time().Second()))
		} else {
			return _NaN
		}
	}
	panic(r.NewTypeError("Method Date.prototype.getSeconds is called on incompatible receiver"))
}

func (r *Runtime) dateproto_getUTCSeconds(call FunctionCall) Value {
	obj := r.toObject(call.This)
	if d, ok := obj.self.(*dateObject); ok {
		if d.isSet() {
			return intToValue(int64(d.timeUTC().Second()))
		} else {
			return _NaN
		}
	}
	panic(r.NewTypeError("Method Date.prototype.getUTCSeconds is called on incompatible receiver"))
}

func (r *Runtime) dateproto_getMilliseconds(call FunctionCall) Value {
	obj := r.toObject(call.This)
	if d, ok := obj.self.(*dateObject); ok {
		if d.isSet() {
			return intToValue(int64(d.time().Nanosecond() / 1e6))
		} else {
			return _NaN
		}
	}
	panic(r.NewTypeError("Method Date.prototype.getMilliseconds is called on incompatible receiver"))
}

func (r *Runtime) dateproto_getUTCMilliseconds(call FunctionCall) Value {
	obj := r.toObject(call.This)
	if d, ok := obj.self.(*dateObject); ok {
		if d.isSet() {
			return intToValue(int64(d.timeUTC().Nanosecond() / 1e6))
		} else {
			return _NaN
		}
	}
	panic(r.NewTypeError("Method Date.prototype.getUTCMilliseconds is called on incompatible receiver"))
}

func (r *Runtime) dateproto_getTimezoneOffset(call FunctionCall) Value {
	obj := r.toObject(call.This)
	if d, ok := obj.self.(*dateObject); ok {
		if d.isSet() {
			_, offset := d.time().Zone()
			return floatToValue(float64(-offset) / 60)
		} else {
			return _NaN
		}
	}
	panic(r.NewTypeError("Method Date.prototype.getTimezoneOffset is called on incompatible receiver"))
}

func (r *Runtime) dateproto_setTime(call FunctionCall) Value {
	obj := r.toObject(call.This)
	if d, ok := obj.self.(*dateObject); ok {
		n := call.Argument(0).ToNumber()
		if IsNaN(n) {
			d.unset()
			return _NaN
		}
		return d.setTimeMs(n.ToInteger())
	}
	panic(r.NewTypeError("Method Date.prototype.setTime is called on incompatible receiver"))
}

// _norm returns nhi, nlo such that
//
//	hi * base + lo == nhi * base + nlo
//	0 <= nlo < base
func _norm(hi, lo, base int64) (nhi, nlo int64, ok bool) {
	if lo < 0 {
		if hi == math.MinInt64 && lo <= -base {
			// underflow
			ok = false
			return
		}
		n := (-lo-1)/base + 1
		hi -= n
		lo += n * base
	}
	if lo >= base {
		if hi == math.MaxInt64 {
			// overflow
			ok = false
			return
		}
		n := lo / base
		hi += n
		lo -= n * base
	}
	return hi, lo, true
}

func mkTime(year, m, day, hour, min, sec, nsec int64, loc *time.Location) (t time.Time, ok bool) {
	year, m, ok = _norm(year, m, 12)
	if !ok {
		return
	}

	// Normalise nsec, sec, min, hour, overflowing into day.
	sec, nsec, ok = _norm(sec, nsec, 1e9)
	if !ok {
		return
	}
	min, sec, ok = _norm(min, sec, 60)
	if !ok {
		return
	}
	hour, min, ok = _norm(hour, min, 60)
	if !ok {
		return
	}
	day, hour, ok = _norm(day, hour, 24)
	if !ok {
		return
	}
	if year > math.MaxInt32 || year < math.MinInt32 ||
		day > math.MaxInt32 || day < math.MinInt32 ||
		m >= math.MaxInt32 || m < math.MinInt32-1 {
		return time.Time{}, false
	}
	month := time.Month(m) + 1
	return time.Date(int(year), month, int(day), int(hour), int(min), int(sec), int(nsec), loc), true
}

func _intArg(call FunctionCall, argNum int) (int64, bool) {
	n := call.Argument(argNum).ToNumber()
	if IsNaN(n) {
		return 0, false
	}
	return n.ToInteger(), true
}

func _dateSetYear(t time.Time, call FunctionCall, argNum int, utc bool) (time.Time, bool) {
	var year int64
	if argNum == 0 || argNum > 0 && argNum < len(call.Arguments) {
		var ok bool
		year, ok = _intArg(call, argNum)
		if !ok {
			return time.Time{}, false
		}
		if year >= 0 && year <= 99 {
			year += 1900
		}
	} else {
		year = int64(t.Year())
	}

	return _dateSetMonth(year, t, call, argNum+1, utc)
}

func _dateSetFullYear(t time.Time, call FunctionCall, argNum int, utc bool) (time.Time, bool) {
	var year int64
	if argNum == 0 || argNum > 0 && argNum < len(call.Arguments) {
		var ok bool
		year, ok = _intArg(call, argNum)
		if !ok {
			return time.Time{}, false
		}
	} else {
		year = int64(t.Year())
	}
	return _dateSetMonth(year, t, call, argNum+1, utc)
}

func _dateSetMonth(year int64, t time.Time, call FunctionCall, argNum int, utc bool) (time.Time, bool) {
	var mon int64
	if argNum == 0 || argNum > 0 && argNum < len(call.Arguments) {
		var ok bool
		mon, ok = _intArg(call, argNum)
		if !ok {
			return time.Time{}, false
		}
	} else {
		mon = int64(t.Month()) - 1
	}

	return _dateSetDay(year, mon, t, call, argNum+1, utc)
}

func _dateSetDay(year, mon int64, t time.Time, call FunctionCall, argNum int, utc bool) (time.Time, bool) {
	var day int64
	if argNum == 0 || argNum > 0 && argNum < len(call.Arguments) {
		var ok bool
		day, ok = _intArg(call, argNum)
		if !ok {
			return time.Time{}, false
		}
	} else {
		day = int64(t.Day())
	}

	return _dateSetHours(year, mon, day, t, call, argNum+1, utc)
}

func _dateSetHours(year, mon, day int64, t time.Time, call FunctionCall, argNum int, utc bool) (time.Time, bool) {
	var hours int64
	if argNum == 0 || argNum > 0 && argNum < len(call.Arguments) {
		var ok bool
		hours, ok = _intArg(call, argNum)
		if !ok {
			return time.Time{}, false
		}
	} else {
		hours = int64(t.Hour())
	}
	return _dateSetMinutes(year, mon, day, hours, t, call, argNum+1, utc)
}

func _dateSetMinutes(year, mon, day, hours int64, t time.Time, call FunctionCall, argNum int, utc bool) (time.Time, bool) {
	var min int64
	if argNum == 0 || argNum > 0 && argNum < len(call.Arguments) {
		var ok bool
		min, ok = _intArg(call, argNum)
		if !ok {
			return time.Time{}, false
		}
	} else {
		min = int64(t.Minute())
	}
	return _dateSetSeconds(year, mon, day, hours, min, t, call, argNum+1, utc)
}

func _dateSetSeconds(year, mon, day, hours, min int64, t time.Time, call FunctionCall, argNum int, utc bool) (time.Time, bool) {
	var sec int64
	if argNum == 0 || argNum > 0 && argNum < len(call.Arguments) {
		var ok bool
		sec, ok = _intArg(call, argNum)
		if !ok {
			return time.Time{}, false
		}
	} else {
		sec = int64(t.Second())
	}
	return _dateSetMilliseconds(year, mon, day, hours, min, sec, t, call, argNum+1, utc)
}

func _dateSetMilliseconds(year, mon, day, hours, min, sec int64, t time.Time, call FunctionCall, argNum int, utc bool) (time.Time, bool) {
	var msec int64
	if argNum == 0 || argNum > 0 && argNum < len(call.Arguments) {
		var ok bool
		msec, ok = _intArg(call, argNum)
		if !ok {
			return time.Time{}, false
		}
	} else {
		msec = int64(t.Nanosecond() / 1e6)
	}
	var ok bool
	sec, msec, ok = _norm(sec, msec, 1e3)
	if !ok {
		return time.Time{}, false
	}

	var loc *time.Location
	if utc {
		loc = time.UTC
	} else {
		loc = time.Local
	}
	r, ok := mkTime(year, mon, day, hours, min, sec, msec*1e6, loc)
	if !ok {
		return time.Time{}, false
	}
	if utc {
		return r.In(time.Local), true
	}
	return r, true
}

func (r *Runtime) dateproto_setMilliseconds(call FunctionCall) Value {
	obj := r.toObject(call.This)
	if d, ok := obj.self.(*dateObject); ok {
		n := call.Argument(0).ToNumber()
		if IsNaN(n) {
			d.unset()
			return _NaN
		}
		msec := n.ToInteger()
		sec := d.msec / 1e3
		var ok bool
		sec, msec, ok = _norm(sec, msec, 1e3)
		if !ok {
			d.unset()
			return _NaN
		}
		if d.isSet() {
			return d.setTimeMs(sec*1e3 + msec)
		} else {
			return _NaN
		}
	}
	panic(r.NewTypeError("Method Date.prototype.setMilliseconds is called on incompatible receiver"))
}

func (r *Runtime) dateproto_setUTCMilliseconds(call FunctionCall) Value {
	obj := r.toObject(call.This)
	if d, ok := obj.self.(*dateObject); ok {
		n := call.Argument(0).ToNumber()
		if IsNaN(n) {
			d.unset()
			return _NaN
		}
		msec := n.ToInteger()
		sec := d.msec / 1e3
		var ok bool
		sec, msec, ok = _norm(sec, msec, 1e3)
		if !ok {
			d.unset()
			return _NaN
		}
		if d.isSet() {
			return d.setTimeMs(sec*1e3 + msec)
		} else {
			return _NaN
		}
	}
	panic(r.NewTypeError("Method Date.prototype.setUTCMilliseconds is called on incompatible receiver"))
}

func (r *Runtime) dateproto_setSeconds(call FunctionCall) Value {
	obj := r.toObject(call.This)
	if d, ok := obj.self.(*dateObject); ok {
		t, ok := _dateSetFullYear(d.time(), call, -5, false)
		if !ok {
			d.unset()
			return _NaN
		}
		if d.isSet() {
			return d.setTimeMs(timeToMsec(t))
		} else {
			return _NaN
		}
	}
	panic(r.NewTypeError("Method Date.prototype.setSeconds is called on incompatible receiver"))
}

func (r *Runtime) dateproto_setUTCSeconds(call FunctionCall) Value {
	obj := r.toObject(call.This)
	if d, ok := obj.self.(*dateObject); ok {
		t, ok := _dateSetFullYear(d.timeUTC(), call, -5, true)
		if !ok {
			d.unset()
			return _NaN
		}
		if d.isSet() {
			return d.setTimeMs(timeToMsec(t))
		} else {
			return _NaN
		}
	}
	panic(r.NewTypeError("Method Date.prototype.setUTCSeconds is called on incompatible receiver"))
}

func (r *Runtime) dateproto_setMinutes(call FunctionCall) Value {
	obj := r.toObject(call.This)
	if d, ok := obj.self.(*dateObject); ok {
		t, ok := _dateSetFullYear(d.time(), call, -4, false)
		if !ok {
			d.unset()
			return _NaN
		}
		if d.isSet() {
			return d.setTimeMs(timeToMsec(t))
		} else {
			return _NaN
		}
	}
	panic(r.NewTypeError("Method Date.prototype.setMinutes is called on incompatible receiver"))
}

func (r *Runtime) dateproto_setUTCMinutes(call FunctionCall) Value {
	obj := r.toObject(call.This)
	if d, ok := obj.self.(*dateObject); ok {
		t, ok := _dateSetFullYear(d.timeUTC(), call, -4, true)
		if !ok {
			d.unset()
			return _NaN
		}
		if d.isSet() {
			return d.setTimeMs(timeToMsec(t))
		} else {
			return _NaN
		}
	}
	panic(r.NewTypeError("Method Date.prototype.setUTCMinutes is called on incompatible receiver"))
}

func (r *Runtime) dateproto_setHours(call FunctionCall) Value {
	obj := r.toObject(call.This)
	if d, ok := obj.self.(*dateObject); ok {
		t, ok := _dateSetFullYear(d.time(), call, -3, false)
		if !ok {
			d.unset()
			return _NaN
		}
		if d.isSet() {
			return d.setTimeMs(timeToMsec(t))
		} else {
			return _NaN
		}
	}
	panic(r.NewTypeError("Method Date.prototype.setHours is called on incompatible receiver"))
}

func (r *Runtime) dateproto_setUTCHours(call FunctionCall) Value {
	obj := r.toObject(call.This)
	if d, ok := obj.self.(*dateObject); ok {
		t, ok := _dateSetFullYear(d.timeUTC(), call, -3, true)
		if !ok {
			d.unset()
			return _NaN
		}
		if d.isSet() {
			return d.setTimeMs(timeToMsec(t))
		} else {
			return _NaN
		}
	}
	panic(r.NewTypeError("Method Date.prototype.setUTCHours is called on incompatible receiver"))
}

func (r *Runtime) dateproto_setDate(call FunctionCall) Value {
	obj := r.toObject(call.This)
	if d, ok := obj.self.(*dateObject); ok {
		t, ok := _dateSetFullYear(d.time(), limitCallArgs(call, 1), -2, false)
		if !ok {
			d.unset()
			return _NaN
		}
		if d.isSet() {
			return d.setTimeMs(timeToMsec(t))
		} else {
			return _NaN
		}
	}
	panic(r.NewTypeError("Method Date.prototype.setDate is called on incompatible receiver"))
}

func (r *Runtime) dateproto_setUTCDate(call FunctionCall) Value {
	obj := r.toObject(call.This)
	if d, ok := obj.self.(*dateObject); ok {
		t, ok := _dateSetFullYear(d.timeUTC(), limitCallArgs(call, 1), -2, true)
		if !ok {
			d.unset()
			return _NaN
		}
		if d.isSet() {
			return d.setTimeMs(timeToMsec(t))
		} else {
			return _NaN
		}
	}
	panic(r.NewTypeError("Method Date.prototype.setUTCDate is called on incompatible receiver"))
}

func (r *Runtime) dateproto_setMonth(call FunctionCall) Value {
	obj := r.toObject(call.This)
	if d, ok := obj.self.(*dateObject); ok {
		t, ok := _dateSetFullYear(d.time(), limitCallArgs(call, 2), -1, false)
		if !ok {
			d.unset()
			return _NaN
		}
		if d.isSet() {
			return d.setTimeMs(timeToMsec(t))
		} else {
			return _NaN
		}
	}
	panic(r.NewTypeError("Method Date.prototype.setMonth is called on incompatible receiver"))
}

func (r *Runtime) dateproto_setUTCMonth(call FunctionCall) Value {
	obj := r.toObject(call.This)
	if d, ok := obj.self.(*dateObject); ok {
		t, ok := _dateSetFullYear(d.timeUTC(), limitCallArgs(call, 2), -1, true)
		if !ok {
			d.unset()
			return _NaN
		}
		if d.isSet() {
			return d.setTimeMs(timeToMsec(t))
		} else {
			return _NaN
		}
	}
	panic(r.NewTypeError("Method Date.prototype.setUTCMonth is called on incompatible receiver"))
}

func (r *Runtime) dateproto_setFullYear(call FunctionCall) Value {
	obj := r.toObject(call.This)
	if d, ok := obj.self.(*dateObject); ok {
		var t time.Time
		if d.isSet() {
			t = d.time()
		} else {
			t = time.Date(1970, time.January, 1, 0, 0, 0, 0, time.Local)
		}
		t, ok := _dateSetFullYear(t, limitCallArgs(call, 3), 0, false)
		if !ok {
			d.unset()
			return _NaN
		}
		return d.setTimeMs(timeToMsec(t))
	}
	panic(r.NewTypeError("Method Date.prototype.setFullYear is called on incompatible receiver"))
}

func (r *Runtime) dateproto_setUTCFullYear(call FunctionCall) Value {
	obj := r.toObject(call.This)
	if d, ok := obj.self.(*dateObject); ok {
		var t time.Time
		if d.isSet() {
			t = d.timeUTC()
		} else {
			t = time.Date(1970, time.January, 1, 0, 0, 0, 0, time.UTC)
		}
		t, ok := _dateSetFullYear(t, limitCallArgs(call, 3), 0, true)
		if !ok {
			d.unset()
			return _NaN
		}
		return d.setTimeMs(timeToMsec(t))
	}
	panic(r.NewTypeError("Method Date.prototype.setUTCFullYear is called on incompatible receiver"))
}

var dateTemplate *objectTemplate
var dateTemplateOnce sync.Once

func getDateTemplate() *objectTemplate {
	dateTemplateOnce.Do(func() {
		dateTemplate = createDateTemplate()
	})
	return dateTemplate
}

func createDateTemplate() *objectTemplate {
	t := newObjectTemplate()
	t.protoFactory = func(r *Runtime) *Object {
		return r.getFunctionPrototype()
	}

	t.putStr("name", func(r *Runtime) Value { return valueProp(asciiString("Date"), false, false, true) })
	t.putStr("length", func(r *Runtime) Value { return valueProp(intToValue(7), false, false, true) })

	t.putStr("prototype", func(r *Runtime) Value { return valueProp(r.getDatePrototype(), false, false, false) })

	t.putStr("parse", func(r *Runtime) Value { return r.methodProp(r.date_parse, "parse", 1) })
	t.putStr("UTC", func(r *Runtime) Value { return r.methodProp(r.date_UTC, "UTC", 7) })
	t.putStr("now", func(r *Runtime) Value { return r.methodProp(r.date_now, "now", 0) })

	return t
}

func (r *Runtime) getDate() *Object {
	ret := r.global.Date
	if ret == nil {
		ret = &Object{runtime: r}
		r.global.Date = ret
		r.newTemplatedFuncObject(getDateTemplate(), ret, r.builtin_date,
			r.wrapNativeConstruct(r.builtin_newDate, ret, r.getDatePrototype()))
	}
	return ret
}

func createDateProtoTemplate() *objectTemplate {
	t := newObjectTemplate()
	t.protoFactory = func(r *Runtime) *Object {
		return r.global.ObjectPrototype
	}

	t.putStr("constructor", func(r *Runtime) Value { return valueProp(r.getDate(), true, false, true) })

	t.putStr("toString", func(r *Runtime) Value { return r.methodProp(r.dateproto_toString, "toString", 0) })
	t.putStr("toDateString", func(r *Runtime) Value { return r.methodProp(r.dateproto_toDateString, "toDateString", 0) })
	t.putStr("toTimeString", func(r *Runtime) Value { return r.methodProp(r.dateproto_toTimeString, "toTimeString", 0) })
	t.putStr("toLocaleString", func(r *Runtime) Value { return r.methodProp(r.dateproto_toLocaleString, "toLocaleString", 0) })
	t.putStr("toLocaleDateString", func(r *Runtime) Value { return r.methodProp(r.dateproto_toLocaleDateString, "toLocaleDateString", 0) })
	t.putStr("toLocaleTimeString", func(r *Runtime) Value { return r.methodProp(r.dateproto_toLocaleTimeString, "toLocaleTimeString", 0) })
	t.putStr("valueOf", func(r *Runtime) Value { return r.methodProp(r.dateproto_valueOf, "valueOf", 0) })
	t.putStr("getTime", func(r *Runtime) Value { return r.methodProp(r.dateproto_getTime, "getTime", 0) })
	t.putStr("getFullYear", func(r *Runtime) Value { return r.methodProp(r.dateproto_getFullYear, "getFullYear", 0) })
	t.putStr("getUTCFullYear", func(r *Runtime) Value { return r.methodProp(r.dateproto_getUTCFullYear, "getUTCFullYear", 0) })
	t.putStr("getMonth", func(r *Runtime) Value { return r.methodProp(r.dateproto_getMonth, "getMonth", 0) })
	t.putStr("getUTCMonth", func(r *Runtime) Value { return r.methodProp(r.dateproto_getUTCMonth, "getUTCMonth", 0) })
	t.putStr("getDate", func(r *Runtime) Value { return r.methodProp(r.dateproto_getDate, "getDate", 0) })
	t.putStr("getUTCDate", func(r *Runtime) Value { return r.methodProp(r.dateproto_getUTCDate, "getUTCDate", 0) })
	t.putStr("getDay", func(r *Runtime) Value { return r.methodProp(r.dateproto_getDay, "getDay", 0) })
	t.putStr("getUTCDay", func(r *Runtime) Value { return r.methodProp(r.dateproto_getUTCDay, "getUTCDay", 0) })
	t.putStr("getHours", func(r *Runtime) Value { return r.methodProp(r.dateproto_getHours, "getHours", 0) })
	t.putStr("getUTCHours", func(r *Runtime) Value { return r.methodProp(r.dateproto_getUTCHours, "getUTCHours", 0) })
	t.putStr("getMinutes", func(r *Runtime) Value { return r.methodProp(r.dateproto_getMinutes, "getMinutes", 0) })
	t.putStr("getUTCMinutes", func(r *Runtime) Value { return r.methodProp(r.dateproto_getUTCMinutes, "getUTCMinutes", 0) })
	t.putStr("getSeconds", func(r *Runtime) Value { return r.methodProp(r.dateproto_getSeconds, "getSeconds", 0) })
	t.putStr("getUTCSeconds", func(r *Runtime) Value { return r.methodProp(r.dateproto_getUTCSeconds, "getUTCSeconds", 0) })
	t.putStr("getMilliseconds", func(r *Runtime) Value { return r.methodProp(r.dateproto_getMilliseconds, "getMilliseconds", 0) })
	t.putStr("getUTCMilliseconds", func(r *Runtime) Value { return r.methodProp(r.dateproto_getUTCMilliseconds, "getUTCMilliseconds", 0) })
	t.putStr("getTimezoneOffset", func(r *Runtime) Value { return r.methodProp(r.dateproto_getTimezoneOffset, "getTimezoneOffset", 0) })
	t.putStr("setTime", func(r *Runtime) Value { return r.methodProp(r.dateproto_setTime, "setTime", 1) })
	t.putStr("setMilliseconds", func(r *Runtime) Value { return r.methodProp(r.dateproto_setMilliseconds, "setMilliseconds", 1) })
	t.putStr("setUTCMilliseconds", func(r *Runtime) Value { return r.methodProp(r.dateproto_setUTCMilliseconds, "setUTCMilliseconds", 1) })
	t.putStr("setSeconds", func(r *Runtime) Value { return r.methodProp(r.dateproto_setSeconds, "setSeconds", 2) })
	t.putStr("setUTCSeconds", func(r *Runtime) Value { return r.methodProp(r.dateproto_setUTCSeconds, "setUTCSeconds", 2) })
	t.putStr("setMinutes", func(r *Runtime) Value { return r.methodProp(r.dateproto_setMinutes, "setMinutes", 3) })
	t.putStr("setUTCMinutes", func(r *Runtime) Value { return r.methodProp(r.dateproto_setUTCMinutes, "setUTCMinutes", 3) })
	t.putStr("setHours", func(r *Runtime) Value { return r.methodProp(r.dateproto_setHours, "setHours", 4) })
	t.putStr("setUTCHours", func(r *Runtime) Value { return r.methodProp(r.dateproto_setUTCHours, "setUTCHours", 4) })
	t.putStr("setDate", func(r *Runtime) Value { return r.methodProp(r.dateproto_setDate, "setDate", 1) })
	t.putStr("setUTCDate", func(r *Runtime) Value { return r.methodProp(r.dateproto_setUTCDate, "setUTCDate", 1) })
	t.putStr("setMonth", func(r *Runtime) Value { return r.methodProp(r.dateproto_setMonth, "setMonth", 2) })
	t.putStr("setUTCMonth", func(r *Runtime) Value { return r.methodProp(r.dateproto_setUTCMonth, "setUTCMonth", 2) })
	t.putStr("setFullYear", func(r *Runtime) Value { return r.methodProp(r.dateproto_setFullYear, "setFullYear", 3) })
	t.putStr("setUTCFullYear", func(r *Runtime) Value { return r.methodProp(r.dateproto_setUTCFullYear, "setUTCFullYear", 3) })
	t.putStr("toUTCString", func(r *Runtime) Value { return r.methodProp(r.dateproto_toUTCString, "toUTCString", 0) })
	t.putStr("toISOString", func(r *Runtime) Value { return r.methodProp(r.dateproto_toISOString, "toISOString", 0) })
	t.putStr("toJSON", func(r *Runtime) Value { return r.methodProp(r.dateproto_toJSON, "toJSON", 1) })

	t.putSym(SymToPrimitive, func(r *Runtime) Value {
		return valueProp(r.newNativeFunc(r.dateproto_toPrimitive, "[Symbol.toPrimitive]", 1), false, false, true)
	})

	return t
}

var dateProtoTemplate *objectTemplate
var dateProtoTemplateOnce sync.Once

func getDateProtoTemplate() *objectTemplate {
	dateProtoTemplateOnce.Do(func() {
		dateProtoTemplate = createDateProtoTemplate()
	})
	return dateProtoTemplate
}

func (r *Runtime) getDatePrototype() *Object {
	ret := r.global.DatePrototype
	if ret == nil {
		ret = &Object{runtime: r}
		r.global.DatePrototype = ret
		r.newTemplatedObject(getDateProtoTemplate(), ret)
	}
	return ret
}
