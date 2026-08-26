package icmp

import (
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"log"
	"math"
	"reflect"
	"time"

	"github.com/grafana/sobek"
)

func toInt[T int | int64 | int32 | int16 | int8](value sobek.Value, defValue T) (T, error) {
	if sobek.IsUndefined(value) || sobek.IsNull(value) || value == nil {
		return defValue, nil
	}

	if !sobek.IsNumber(value) {
		return 0, fmt.Errorf("%w: number expected", errInvalidType)
	}

	return T(value.ToNumber().ToInteger()), nil
}

func toDuration(value sobek.Value, defValue time.Duration) (time.Duration, error) {
	if sobek.IsUndefined(value) || sobek.IsNull(value) || value == nil {
		return defValue, nil
	}

	if sobek.IsNumber(value) {
		return time.Duration(value.ToInteger()) * time.Millisecond, nil
	}

	if value.ExportType() == reflect.TypeFor[string]() {
		return time.ParseDuration(value.String())
	}

	return 0, fmt.Errorf("%w: number or string expected", errInvalidType)
}

func toPercent(value sobek.Value, defValue float64) (float64, error) {
	if sobek.IsUndefined(value) || sobek.IsNull(value) || value == nil {
		return defValue, nil
	}

	if !sobek.IsNumber(value) {
		return 0, fmt.Errorf("%w: number expected", errInvalidType)
	}

	pc := value.ToFloat()
	if pc <= 0 || pc > 100 || math.IsNaN(pc) {
		pc = defValue
	}

	return pc, nil
}

func toUint16(value sobek.Value) (int, error) {
	if sobek.IsUndefined(value) || sobek.IsNull(value) || value == nil {
		return randomUint16(), nil
	}

	if !sobek.IsNumber(value) {
		return 0, fmt.Errorf("%w: number expected", errInvalidType)
	}

	return int(value.ToNumber().ToInteger()) & maxUint16, nil
}

func randomUint16() int {
	var n uint16
	if err := binary.Read(rand.Reader, binary.LittleEndian, &n); err != nil {
		log.Fatal(err)
	}

	return int(n)
}
