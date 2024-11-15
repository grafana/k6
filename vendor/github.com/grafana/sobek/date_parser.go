package sobek

import (
	"strings"
)

type date struct {
	year, month, day     int
	hour, min, sec, msec int
	timeZoneOffset       int // time zone offset in minutes
	isLocal              bool
}

func skip(s string, c byte) (string, bool) {
	if len(s) > 0 && s[0] == c {
		return s[1:], true
	}
	return s, false
}

func skipSpaces(s string) string {
	for len(s) > 0 && s[0] == ' ' {
		s = s[1:]
	}
	return s
}

func skipUntil(s string, stopList string) string {
	for len(s) > 0 && !strings.ContainsRune(stopList, rune(s[0])) {
		s = s[1:]
	}
	return s
}

func match(s string, lower string) (string, bool) {
	if len(s) < len(lower) {
		return s, false
	}
	for i := 0; i < len(lower); i++ {
		c1 := s[i]
		c2 := lower[i]
		if c1 != c2 {
			// switch to lower-case; 'a'-'A' is known to be a single bit
			c1 |= 'a' - 'A'
			if c1 != c2 || c1 < 'a' || c1 > 'z' {
				return s, false
			}
		}
	}
	return s[len(lower):], true
}

func getDigits(s string, minDigits, maxDigits int) (int, string, bool) {
	var i, v int
	for i < len(s) && i < maxDigits && s[i] >= '0' && s[i] <= '9' {
		v = v*10 + int(s[i]-'0')
		i++
	}
	if i < minDigits {
		return 0, s, false
	}
	return v, s[i:], true
}

func getMilliseconds(s string) (int, string) {
	mul, v := 100, 0
	if len(s) > 0 && (s[0] == '.' || s[0] == ',') {
		const I_START = 1
		i := I_START
		for i < len(s) && i-I_START < 9 && s[i] >= '0' && s[i] <= '9' {
			v += int(s[i]-'0') * mul
			mul /= 10
			i++
		}
		if i > I_START {
			// only consume the separator if digits are present
			return v, s[i:]
		}
	}
	return 0, s
}

// [+-]HH:mm or [+-]HHmm or Z
func getTimeZoneOffset(s string, strict bool) (int, string, bool) {
	if len(s) == 0 {
		return 0, s, false
	}
	sign := s[0]
	if sign == '+' || sign == '-' {
		var hh, mm, v int
		var ok bool
		t := s[1:]
		n := len(t)
		if hh, t, ok = getDigits(t, 1, 9); !ok {
			return 0, s, false
		}
		n -= len(t)
		if strict && n != 2 && n != 4 {
			return 0, s, false
		}
		for n > 4 {
			n -= 2
			hh /= 100
		}
		if n > 2 {
			mm = hh % 100
			hh = hh / 100
		} else if t, ok = skip(t, ':'); ok {
			if mm, t, ok = getDigits(t, 2, 2); !ok {
				return 0, s, false
			}
		}
		if hh > 23 || mm > 59 {
			return 0, s, false
		}
		v = hh*60 + mm
		if sign == '-' {
			v = -v
		}
		return v, t, true
	} else if sign == 'Z' {
		return 0, s[1:], true
	}
	return 0, s, false
}

var tzAbbrs = []struct {
	nameLower string
	offset    int
}{
	{"gmt", 0},        // Greenwich Mean Time
	{"utc", 0},        // Coordinated Universal Time
	{"ut", 0},         // Universal Time
	{"z", 0},          // Zulu Time
	{"edt", -4 * 60},  // Eastern Daylight Time
	{"est", -5 * 60},  // Eastern Standard Time
	{"cdt", -5 * 60},  // Central Daylight Time
	{"cst", -6 * 60},  // Central Standard Time
	{"mdt", -6 * 60},  // Mountain Daylight Time
	{"mst", -7 * 60},  // Mountain Standard Time
	{"pdt", -7 * 60},  // Pacific Daylight Time
	{"pst", -8 * 60},  // Pacific Standard Time
	{"wet", +0 * 60},  // Western European Time
	{"west", +1 * 60}, // Western European Summer Time
	{"cet", +1 * 60},  // Central European Time
	{"cest", +2 * 60}, // Central European Summer Time
	{"eet", +2 * 60},  // Eastern European Time
	{"eest", +3 * 60}, // Eastern European Summer Time
}

func getTimeZoneAbbr(s string) (int, string, bool) {
	for _, tzAbbr := range tzAbbrs {
		if s, ok := match(s, tzAbbr.nameLower); ok {
			return tzAbbr.offset, s, true
		}
	}
	return 0, s, false
}

var monthNamesLower = []string{
	"jan",
	"feb",
	"mar",
	"apr",
	"may",
	"jun",
	"jul",
	"aug",
	"sep",
	"oct",
	"nov",
	"dec",
}

func getMonth(s string) (int, string, bool) {
	for i, monthNameLower := range monthNamesLower {
		if s, ok := match(s, monthNameLower); ok {
			return i + 1, s, true
		}
	}
	return 0, s, false
}

func parseDateISOString(s string) (date, bool) {
	if len(s) == 0 {
		return date{}, false
	}
	var d = date{month: 1, day: 1}
	var ok bool

	// year is either yyyy digits or [+-]yyyyyy
	sign := s[0]
	if sign == '-' || sign == '+' {
		s = s[1:]
		if d.year, s, ok = getDigits(s, 6, 6); !ok {
			return date{}, false
		}
		if sign == '-' {
			if d.year == 0 {
				// reject -000000
				return date{}, false
			}
			d.year = -d.year
		}
	} else if d.year, s, ok = getDigits(s, 4, 4); !ok {
		return date{}, false
	}
	if s, ok = skip(s, '-'); ok {
		if d.month, s, ok = getDigits(s, 2, 2); !ok || d.month < 1 {
			return date{}, false
		}
		if s, ok = skip(s, '-'); ok {
			if d.day, s, ok = getDigits(s, 2, 2); !ok || d.day < 1 {
				return date{}, false
			}
		}
	}
	if s, ok = skip(s, 'T'); ok {
		if d.hour, s, ok = getDigits(s, 2, 2); !ok {
			return date{}, false
		}
		if s, ok = skip(s, ':'); !ok {
			return date{}, false
		}
		if d.min, s, ok = getDigits(s, 2, 2); !ok {
			return date{}, false
		}
		if s, ok = skip(s, ':'); ok {
			if d.sec, s, ok = getDigits(s, 2, 2); !ok {
				return date{}, false
			}
			d.msec, s = getMilliseconds(s)
		}
		d.isLocal = true
	}
	// parse the time zone offset if present
	if len(s) > 0 {
		if d.timeZoneOffset, s, ok = getTimeZoneOffset(s, true); !ok {
			return date{}, false
		}
		d.isLocal = false
	}
	// error if extraneous characters
	return d, len(s) == 0
}

func parseDateOtherString(s string) (date, bool) {
	var d = date{
		year:    2001,
		month:   1,
		day:     1,
		isLocal: true,
	}
	var nums [3]int
	var numIndex int
	var hasYear, hasMon, hasTime, ok bool
	for {
		s = skipSpaces(s)
		if len(s) == 0 {
			break
		}
		c := s[0]
		n, val := len(s), 0
		if c == '+' || c == '-' {
			if hasTime {
				if val, s, ok = getTimeZoneOffset(s, false); ok {
					d.timeZoneOffset = val
					d.isLocal = false
				}
			}
			if !hasTime || !ok {
				s = s[1:]
				if val, s, ok = getDigits(s, 1, 9); ok {
					d.year = val
					if c == '-' {
						if d.year == 0 {
							return date{}, false
						}
						d.year = -d.year
					}
					hasYear = true
				}
			}
		} else if val, s, ok = getDigits(s, 1, 9); ok {
			if s, ok = skip(s, ':'); ok {
				// time part
				d.hour = val
				if d.min, s, ok = getDigits(s, 1, 2); !ok {
					return date{}, false
				}
				if s, ok = skip(s, ':'); ok {
					if d.sec, s, ok = getDigits(s, 1, 2); !ok {
						return date{}, false
					}
					d.msec, s = getMilliseconds(s)
				}
				hasTime = true
				if t := skipSpaces(s); len(t) > 0 {
					if t, ok = match(t, "pm"); ok {
						if d.hour < 12 {
							d.hour += 12
						}
						s = t
						continue
					} else if t, ok = match(t, "am"); ok {
						if d.hour == 12 {
							d.hour = 0
						}
						s = t
						continue
					}
				}
			} else if n-len(s) > 2 {
				d.year = val
				hasYear = true
			} else if val < 1 || val > 31 {
				d.year = val
				if val < 100 {
					d.year += 1900
				}
				if val < 50 {
					d.year += 100
				}
				hasYear = true
			} else {
				if numIndex == 3 {
					return date{}, false
				}
				nums[numIndex] = val
				numIndex++
			}
		} else if val, s, ok = getMonth(s); ok {
			d.month = val
			hasMon = true
			s = skipUntil(s, "0123456789 -/(")
		} else if val, s, ok = getTimeZoneAbbr(s); ok {
			d.timeZoneOffset = val
			if len(s) > 0 {
				if c := s[0]; (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') {
					return date{}, false
				}
			}
			d.isLocal = false
			continue
		} else if c == '(' {
			// skip parenthesized phrase
			level := 1
			s = s[1:]
			for len(s) > 0 && level != 0 {
				if s[0] == '(' {
					level++
				} else if s[0] == ')' {
					level--
				}
				s = s[1:]
			}
			if level > 0 {
				return date{}, false
			}
		} else if c == ')' {
			return date{}, false
		} else {
			if hasYear || hasMon || hasTime || numIndex > 0 {
				return date{}, false
			}
			// skip a word
			s = skipUntil(s, " -/(")
		}
		for len(s) > 0 && strings.ContainsRune("-/.,", rune(s[0])) {
			s = s[1:]
		}
	}
	n := numIndex
	if hasYear {
		n++
	}
	if hasMon {
		n++
	}
	if n > 3 {
		return date{}, false
	}

	switch numIndex {
	case 0:
		if !hasYear {
			return date{}, false
		}
	case 1:
		if hasMon {
			d.day = nums[0]
		} else {
			d.month = nums[0]
		}
	case 2:
		if hasYear {
			d.month = nums[0]
			d.day = nums[1]
		} else if hasMon {
			d.year = nums[1]
			if nums[1] < 100 {
				d.year += 1900
			}
			if nums[1] < 50 {
				d.year += 100
			}
			d.day = nums[0]
		} else {
			d.month = nums[0]
			d.day = nums[1]
		}
	case 3:
		d.year = nums[2]
		if nums[2] < 100 {
			d.year += 1900
		}
		if nums[2] < 50 {
			d.year += 100
		}
		d.month = nums[0]
		d.day = nums[1]
	default:
		return date{}, false
	}
	return d, d.month > 0 && d.day > 0
}
