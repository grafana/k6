package compiler

import (
	"github.com/tetratelabs/wazero/internal/asm"
	"github.com/tetratelabs/wazero/internal/wazeroir"
)

// compileDropRange adds instruction to drop the values on the target range
// in an architecture independent way.
func compileDropRange(c compiler, r *wazeroir.InclusiveRange) (err error) {
	locationStack := c.runtimeValueLocationStack()
	if r == nil {
		return
	} else if r.Start == 0 {
		for i := 0; i <= r.End; i++ {
			if loc := locationStack.pop(); loc.onRegister() {
				locationStack.releaseRegister(loc)
			}
		}
		return
	}

	// If the top value is alive, we must ensure that it is not located as a conditional.
	// Otherwise, the conditional flag might end up modified by the following operation.
	if err = c.maybeCompileMoveTopConditionalToGeneralPurposeRegister(); err != nil {
		return
	}

	dropValues, liveValues := locationStack.dropsLivesForInclusiveRange(r)

	// Frees all the registers used by drop target values.
	for i := range dropValues {
		dv := &dropValues[i]
		if dv.onRegister() {
			locationStack.releaseRegister(dv)
		}
	}

	// These registers are not nil if a live value of that type is currently located on the memory stack.
	// In order to migrate these values, we can use them below.
	gpTmp, vecTmp, err := getTemporariesForStackedLiveValues(c, liveValues)
	if err != nil {
		return err
	}

	// Reset the stack pointer below the end.
	locationStack.sp -= uint64(len(liveValues) + len(dropValues))

	// Push back the live values again.
	for i := range liveValues {
		live := &liveValues[i]
		migrateLiveValue(c, live, gpTmp, vecTmp)
	}
	return
}

// migrateLiveValue migrates the live value `live` into the top of the stack. It might be located on the stack
// and in that case, we have to load it into either `generalPurposeTmpReg` or `vectorTmpReg` temporarily, and
// write it back into the *new* stack location.
func migrateLiveValue(c compiler, live *runtimeValueLocation, generalPurposeTmpReg, vectorTmpReg asm.Register) {
	if live.valueType == runtimeValueTypeV128Hi {
		// Higher bits of vector was already handled together with the lower bits.
		return
	}

	previouslyOnStack := live.onStack()
	if previouslyOnStack {
		// If the value is on the stack, load the value on the old location into the temporary value,
		// and then write it back to the new memory location below.
		switch live.getRegisterType() {
		case registerTypeGeneralPurpose:
			live.setRegister(generalPurposeTmpReg)
		case registerTypeVector:
			live.setRegister(vectorTmpReg)
		}
		// Load the value into tmp.
		c.compileLoadValueOnStackToRegister(live)
	}

	var newLocation *runtimeValueLocation
	if live.valueType == runtimeValueTypeV128Lo {
		newLocation = c.pushVectorRuntimeValueLocationOnRegister(live.register)
	} else {
		newLocation = c.pushRuntimeValueLocationOnRegister(live.register, live.valueType)
	}

	if previouslyOnStack {
		// This case, the location is on the temporary register. Therefore,
		// we have to release the value there into the *new* memory location
		// so that the tmp can be used for subsequent live value migrations.
		c.compileReleaseRegisterToStack(newLocation)
	}
}

func getTemporariesForStackedLiveValues(c compiler, liveValues []runtimeValueLocation) (gpTmp, vecTmp asm.Register, err error) {
	gpTmp, vecTmp = asm.NilRegister, asm.NilRegister
	for i := range liveValues {
		l := &liveValues[i]
		if l.onStack() {
			if rt := l.getRegisterType(); rt == registerTypeGeneralPurpose && gpTmp == asm.NilRegister {
				gpTmp, err = c.allocateRegister(registerTypeGeneralPurpose)
				if err != nil {
					return
				}
			} else if rt == registerTypeVector && vecTmp == asm.NilRegister {
				vecTmp, err = c.allocateRegister(registerTypeVector)
				if err != nil {
					return
				}
			}
		}
	}
	return
}

// dropsLivesForInclusiveRange returns the live and drop target values for the given wazeroir.InclusiveRange.
func (v *runtimeValueLocationStack) dropsLivesForInclusiveRange(
	r *wazeroir.InclusiveRange,
) (dropValues, liveValues []runtimeValueLocation) {
	// liveValues are must be pushed backed after dropping the target range.
	liveValues = v.stack[v.sp-uint64(r.Start) : v.sp]
	// dropValues are the values on the drop target range.
	dropValues = v.stack[v.sp-uint64(r.End)-1 : v.sp-uint64(r.Start)]
	return
}
