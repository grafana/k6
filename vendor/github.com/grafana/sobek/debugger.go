package sobek

import (
	"bufio"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/grafana/sobek/parser"
	"github.com/grafana/sobek/unistring"
)

type Debugger struct {
	vm *vm

	currentLine     int
	lastLine        int
	breakpoints     map[string][]int
	breakpointMutex sync.RWMutex
	breakpointIDs   map[string]int
	breakPointCount int
	activationCh    chan chan DebuggerActivation
	currentCh       chan DebuggerActivation
	active          bool
	lastBreakpoint  struct {
		filename   string
		line       int
		stackDepth int
	}
}

func newDebugger(vm *vm) *Debugger {
	dbg := &Debugger{
		vm:            vm,
		activationCh:  make(chan chan DebuggerActivation),
		active:        false,
		breakpoints:   make(map[string][]int),
		breakpointIDs: make(map[string]int),
		lastLine:      0,
	}
	return dbg
}

type ActivationReason string

const (
	ProgramStartActivation      ActivationReason = "start"
	DebuggerStatementActivation ActivationReason = "debugger"
	BreakpointActivation        ActivationReason = "breakpoint"
)

type DebuggerActivation struct {
	Reason   ActivationReason
	Filename string
	Line     int
	ID       int
}

var globalBuiltinKeys = map[string]bool{"Object": true, "Function": true, "Array": true, "String": true, "globalThis": true, "NaN": true, "undefined": true, "Infinity": true, "isNaN": true, "parseInt": true, "parseFloat": true, "isFinite": true, "decodeURI": true, "decodeURIComponent": true, "encodeURI": true, "encodeURIComponent": true, "escape": true, "unescape": true, "Number": true, "RegExp": true, "Date": true, "Boolean": true, "Proxy": true, "Reflect": true, "Error": true, "AggregateError": true, "TypeError": true, "ReferenceError": true, "SyntaxError": true, "RangeError": true, "EvalError": true, "URIError": true, "GoError": true, "eval": true, "Math": true, "JSON": true, "ArrayBuffer": true, "DataView": true, "Uint8Array": true, "Uint8ClampedArray": true, "Int8Array": true, "Uint16Array": true, "Int16Array": true, "Uint32Array": true, "Int32Array": true, "Float32Array": true, "Float64Array": true, "Symbol": true, "WeakSet": true, "WeakMap": true, "Map": true, "Set": true, "Promise": true}

func (dbg *Debugger) activate(reason ActivationReason, filename string, line int) {
	dbg.active = true
	ch := <-dbg.activationCh // get channel from waiter

	id := 0
	if reason != DebuggerStatementActivation {
		dbg.breakpointMutex.RLock()
		id = dbg.breakpointIDs[filename+":"+strconv.Itoa(line)]
		dbg.breakpointMutex.RUnlock()
	}

	ch <- DebuggerActivation{
		Reason:   reason,
		Filename: filename,
		Line:     line,
		ID:       id,
	} // send what activated it
	<-ch // wait for deactivation
	dbg.active = false
}

// Continue unblocks the goja runtime to run code as is and will return the reason why it blocked again.
func (dbg *Debugger) Continue() DebuggerActivation {
	if dbg.currentCh != nil {
		close(dbg.currentCh)
	}
	dbg.currentCh = make(chan DebuggerActivation)
	dbg.activationCh <- dbg.currentCh
	activation := <-dbg.currentCh
	return activation
}

func (dbg *Debugger) PC() int {
	return dbg.vm.pc
}

// Detach the debugger, after this call this instance of the debugger should *not* be used.
// This also disables debug mode for the runtime
func (dbg *Debugger) Detach() { // TODO return an error?
	dbg.vm.debugger = nil
	dbg.vm.debugMode = false
	dbg.vm = nil
	dbg.active = false
	if dbg.currentCh != nil {
		close(dbg.currentCh)
		dbg.currentCh = nil
	}
}

func (dbg *Debugger) SetBreakpoint(filename string, line int) (id int, err error) {
	idx := sort.SearchInts(dbg.breakpoints[filename], line)
	if idx < len(dbg.breakpoints[filename]) && dbg.breakpoints[filename][idx] == line {
		err = errors.New("breakpoint exists")
	} else {
		dbg.breakpoints[filename] = append(dbg.breakpoints[filename], line)
		if len(dbg.breakpoints[filename]) > 1 {
			sort.Ints(dbg.breakpoints[filename])
		}
		dbg.breakpointMutex.Lock()
		id = dbg.breakPointCount
		dbg.breakPointCount++
		dbg.breakpointIDs[filename+":"+strconv.Itoa(line)] = id
		dbg.breakpointMutex.Unlock()
	}
	return
}

func (dbg *Debugger) ClearBreakpoint(filename string, line int) (err error) {
	if len(dbg.breakpoints[filename]) == 0 {
		return errors.New("no breakpoints")
	}

	idx := sort.SearchInts(dbg.breakpoints[filename], line)
	if idx < len(dbg.breakpoints[filename]) && dbg.breakpoints[filename][idx] == line {
		dbg.breakpoints[filename] = append(dbg.breakpoints[filename][:idx], dbg.breakpoints[filename][idx+1:]...)
		if len(dbg.breakpoints[filename]) == 0 {
			delete(dbg.breakpoints, filename)
		}
	} else {
		err = errors.New("breakpoint doesn't exist")
	}
	return
}

func (dbg *Debugger) Breakpoints() (map[string][]int, error) {
	if len(dbg.breakpoints) == 0 {
		return nil, errors.New("no breakpoints")
	}

	return dbg.breakpoints, nil
}

func (dbg *Debugger) StepIn() error {
	// TODO: implement proper error propagation
	lastLine := dbg.Line()
	dbg.updateCurrentLine()
	if dbg.safeToRun() {
		dbg.updateCurrentLine()
		dbg.vm.prg.code[dbg.vm.pc].exec(dbg.vm)
		dbg.updateLastLine(lastLine)
	}
	return nil
}

func (dbg *Debugger) Next() error {
	// TODO: implement proper error propagation
	lastLine := dbg.Line()
	dbg.updateCurrentLine()
	if dbg.getLastLine() != dbg.Line() {
		nextLine := dbg.getNextLine()
		for dbg.safeToRun() && nextLine > 0 && dbg.Line() != nextLine {
			dbg.updateCurrentLine()
			dbg.vm.prg.code[dbg.vm.pc].exec(dbg.vm)
		}
		dbg.updateLastLine(lastLine)
	} else if dbg.getNextLine() == 0 {
		// Step out of functions
		return errors.New("exhausted")
	}
	return nil
}

func (dbg *Debugger) Exec(expr string) (Value, error) {
	if expr == "" {
		return nil, errors.New("nothing to execute")
	}
	val, err := dbg.eval(expr)

	lastLine := dbg.Line()
	dbg.updateLastLine(lastLine)
	return val, err
}

func (dbg *Debugger) Print(varName string) (string, error) {
	if varName == "" {
		return "", errors.New("please specify variable name")
	}
	val, err := dbg.getValue(varName)

	if val == Undefined() {
		return fmt.Sprint(dbg.vm.stash.values), err
	} else {
		// FIXME: val.ToString() causes debugger to exit abruptly
		return fmt.Sprint(val), err
	}
}

func (dbg *Debugger) List() ([]string, error) {
	// TODO probably better to get only some of the lines, but fine for now
	return stringToLines(dbg.vm.prg.src.Source())
}

func stringToLines(s string) (lines []string, err error) {
	scanner := bufio.NewScanner(strings.NewReader(s))
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	err = scanner.Err()
	return
}

func (dbg *Debugger) breakpoint() bool {
	if dbg.vm.prg == nil {
		return false
	}
	filename := dbg.Filename()
	line := dbg.Line()

	idx := sort.SearchInts(dbg.breakpoints[filename], line)
	if idx < len(dbg.breakpoints[filename]) && dbg.breakpoints[filename][idx] == line {
		return true
	} else {
		return false
	}
}

func (dbg *Debugger) getLastLine() int {
	if dbg.lastLine >= 0 {
		return dbg.lastLine
	}
	// First executed line (current line) is considered the last line
	return dbg.Line()
}

func (dbg *Debugger) updateLastLine(lineNumber int) {
	if dbg.lastLine != lineNumber {
		dbg.lastLine = lineNumber
	}
}

func (dbg *Debugger) callStackDepth() int {
	return len(dbg.vm.callStack)
}

func (dbg *Debugger) Line() int {
	// FIXME: Some lines are skipped, which causes this function to report incorrect lines
	// TODO: lines inside function are reported differently and the vm.pc is reset from the start
	// of each function, so account for functions (ref: TestDebuggerStepIn)
	return dbg.vm.prg.src.Position(dbg.vm.prg.sourceOffset(dbg.vm.pc)).Line
}

func (dbg *Debugger) Filename() string {
	return dbg.vm.prg.src.Name()
}

func (dbg *Debugger) updateCurrentLine() {
	dbg.currentLine = dbg.Line()
}

func (dbg *Debugger) getNextLine() int {
	for idx := range dbg.vm.prg.code[dbg.vm.pc:] {
		nextLine := dbg.vm.prg.src.Position(dbg.vm.prg.sourceOffset(dbg.vm.pc + idx + 1)).Line
		if nextLine > dbg.Line() {
			return nextLine
		}
	}
	return 0
}

func (dbg *Debugger) safeToRun() bool {
	return dbg.vm.pc < len(dbg.vm.prg.code)
}

func (dbg *Debugger) eval(expr string) (v Value, err error) {
	prg, err := parser.ParseFile(nil, "<eval>", expr, 0)
	if err != nil {
		return nil, &CompilerSyntaxError{
			CompilerError: CompilerError{
				Message: err.Error(),
			},
		}
	}

	c := newCompiler(true)

	defer func() {
		if x := recover(); x != nil {
			c.p = nil
			switch ex := x.(type) {
			case *CompilerSyntaxError:
				err = ex
			default:
				err = fmt.Errorf("cannot recover from exception %s", ex)
			}
		}
	}()

	var this Value
	if dbg.vm.sb >= 0 {
		this = dbg.vm.stack[dbg.vm.sb]
	} else {
		this = dbg.vm.r.globalObject
	}

	c.compile(prg, false, true, c.evalVM)

	defer func() {
		if x := recover(); x != nil {
			if ex, ok := x.(uncatchableException); ok {
				err = ex
			} else {
				err = fmt.Errorf("cannot recover from exception %s", x)
			}
		}
		dbg.vm.popCtx()
		dbg.vm.sp -= 1
	}()

	dbg.vm.pushCtx()
	dbg.vm.prg = c.p
	dbg.vm.pc = 0
	dbg.vm.args = 0
	dbg.vm.result = _undefined
	dbg.vm.sb = dbg.vm.sp
	dbg.vm.push(this)
	dbg.vm.run()
	v = dbg.vm.result
	return v, err
}

func (dbg *Debugger) getValue(varName string) (val Value, err error) {
	defer func() {
		if err := recover(); err != nil {
			return
		}
	}()

	// copied from loadDynamicRef
	name := unistring.String(varName)
	for stash := dbg.vm.stash; stash != nil; stash = stash.outer {
		if v, exists := stash.getByName(name); exists {
			val = v
			break
		}
	}
	if val == nil {
		val = dbg.vm.r.globalObject.self.getStr(name, nil)
		if val == nil {
			val = valueUnresolved{r: dbg.vm.r, ref: name}
		}
	}
	return val, nil
}

func (dbg *Debugger) GetGlobalVariables() (map[string]Value, error) {
	defer func() {
		if err := recover(); err != nil {
			return
		}
	}()

	globals := make(map[string]Value)
	for _, name := range dbg.vm.r.globalObject.self.stringKeys(true, nil) {
		nameStr := name.String()
		if globalBuiltinKeys[nameStr] {
			continue
		}
		val := dbg.vm.r.globalObject.self.getStr(unistring.String(nameStr), nil)
		if val != nil {
			globals[nameStr] = val
		}
	}
	return globals, nil
}

func (dbg *Debugger) GetLocalVariables() (map[string]Value, error) {
	defer func() {
		if err := recover(); err != nil {
			return
		}
	}()

	locals := make(map[string]Value)
	for name := range dbg.vm.stash.names {
		val, _ := dbg.getValue(name.String())
		if val == nil {
			locals[name.String()] = Undefined()
		}
		locals[name.String()] = val
	}
	return locals, nil
}
