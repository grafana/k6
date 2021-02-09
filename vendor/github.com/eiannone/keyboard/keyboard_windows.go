package keyboard

import (
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	vk_backspace   = 0x8
	vk_tab         = 0x9
	vk_enter       = 0xd
	vk_esc         = 0x1b
	vk_space       = 0x20
	vk_pgup        = 0x21
	vk_pgdn        = 0x22
	vk_end         = 0x23
	vk_home        = 0x24
	vk_arrow_left  = 0x25
	vk_arrow_up    = 0x26
	vk_arrow_right = 0x27
	vk_arrow_down  = 0x28
	vk_insert      = 0x2d
	vk_delete      = 0x2e

	vk_f1  = 0x70
	vk_f2  = 0x71
	vk_f3  = 0x72
	vk_f4  = 0x73
	vk_f5  = 0x74
	vk_f6  = 0x75
	vk_f7  = 0x76
	vk_f8  = 0x77
	vk_f9  = 0x78
	vk_f10 = 0x79
	vk_f11 = 0x7a
	vk_f12 = 0x7b

	right_alt_pressed  = 0x1
	left_alt_pressed   = 0x2
	right_ctrl_pressed = 0x4
	left_ctrl_pressed  = 0x8
	shift_pressed      = 0x10

	k32_keyEvent = 0x1
)

type (
	wchar uint16
	dword uint32
	word  uint16

	k32_event struct {
		key_down          int32
		repeat_count      word
		virtual_key_code  word
		virtual_scan_code word
		unicode_char      wchar
		control_key_state dword
	}
)

var (
	kernel32 = syscall.NewLazyDLL("kernel32.dll")

	k32_WaitForMultipleObjects = kernel32.NewProc("WaitForMultipleObjects")
	k32_ReadConsoleInputW      = kernel32.NewProc("ReadConsoleInputW")

	hConsoleIn syscall.Handle
	hInterrupt windows.Handle

	quit = make(chan bool)

	// This is just to prevent heap allocs at all costs
	tmpArg dword
)

func getError(errno syscall.Errno) error {
	if errno != 0 {
		return error(errno)
	} else {
		return syscall.EINVAL
	}
}

func getKeyEvent(r *k32_event) (KeyEvent, bool) {
	e := KeyEvent{}

	if r.key_down == 0 {
		return e, false
	}

	ctrlPressed := r.control_key_state&(left_ctrl_pressed|right_ctrl_pressed) != 0

	if r.virtual_key_code >= vk_f1 && r.virtual_key_code <= vk_f12 {
		switch r.virtual_key_code {
		case vk_f1:
			e.Key = KeyF1
		case vk_f2:
			e.Key = KeyF2
		case vk_f3:
			e.Key = KeyF3
		case vk_f4:
			e.Key = KeyF4
		case vk_f5:
			e.Key = KeyF5
		case vk_f6:
			e.Key = KeyF6
		case vk_f7:
			e.Key = KeyF7
		case vk_f8:
			e.Key = KeyF8
		case vk_f9:
			e.Key = KeyF9
		case vk_f10:
			e.Key = KeyF10
		case vk_f11:
			e.Key = KeyF11
		case vk_f12:
			e.Key = KeyF12
		default:
			panic("unreachable")
		}

		return e, true
	}

	if r.virtual_key_code <= vk_delete {
		switch r.virtual_key_code {
		case vk_insert:
			e.Key = KeyInsert
		case vk_delete:
			e.Key = KeyDelete
		case vk_home:
			e.Key = KeyHome
		case vk_end:
			e.Key = KeyEnd
		case vk_pgup:
			e.Key = KeyPgup
		case vk_pgdn:
			e.Key = KeyPgdn
		case vk_arrow_up:
			e.Key = KeyArrowUp
		case vk_arrow_down:
			e.Key = KeyArrowDown
		case vk_arrow_left:
			e.Key = KeyArrowLeft
		case vk_arrow_right:
			e.Key = KeyArrowRight
		case vk_backspace:
			if ctrlPressed {
				e.Key = KeyBackspace2
			} else {
				e.Key = KeyBackspace
			}
		case vk_tab:
			e.Key = KeyTab
		case vk_enter:
			e.Key = KeyEnter
		case vk_esc:
			e.Key = KeyEsc
		case vk_space:
			if ctrlPressed {
				// manual return here, because KeyCtrlSpace is zero
				e.Key = KeyCtrlSpace
				return e, true
			} else {
				e.Key = KeySpace
			}
		}

		if e.Key != 0 {
			return e, true
		}
	}

	if ctrlPressed {
		if Key(r.unicode_char) >= KeyCtrlA && Key(r.unicode_char) <= KeyCtrlRsqBracket {
			e.Key = Key(r.unicode_char)
			return e, true
		}
		switch r.virtual_key_code {
		case 192, 50:
			// manual return here, because KeyCtrl2 is zero
			e.Key = KeyCtrl2
			return e, true
		case 51:
			e.Key = KeyCtrl3
		case 52:
			e.Key = KeyCtrl4
		case 53:
			e.Key = KeyCtrl5
		case 54:
			e.Key = KeyCtrl6
		case 189, 191, 55:
			e.Key = KeyCtrl7
		case 8, 56:
			e.Key = KeyCtrl8
		}

		if e.Key != 0 {
			return e, true
		}
	}

	if r.unicode_char != 0 {
		e.Rune = rune(r.unicode_char)
		return e, true
	}

	return e, false
}

func produceEvent(event KeyEvent) bool {
	select {
	case <-quit:
		return false
	case inputComm <- event:
		return true
	}
}

func inputEventsProducer() {
	var input [20]uint16
	for {
		// Wait for events
		// https://docs.microsoft.com/en-us/windows/win32/api/synchapi/nf-synchapi-waitformultipleobjects
		r0, _, e1 := syscall.Syscall6(k32_WaitForMultipleObjects.Addr(), 4,
			uintptr(2), uintptr(unsafe.Pointer(&hConsoleIn)), 0, windows.INFINITE, 0, 0)
		if uint32(r0) == windows.WAIT_FAILED && false == produceEvent(KeyEvent{Err: getError(e1)}) {
			return
		}
		select {
		case <-quit:
			return
		default:
		}

		// Get console input
		r0, _, e1 = syscall.Syscall6(k32_ReadConsoleInputW.Addr(), 4,
			uintptr(hConsoleIn), uintptr(unsafe.Pointer(&input[0])), 1, uintptr(unsafe.Pointer(&tmpArg)), 0, 0)
		if int(r0) == 0 {
			if false == produceEvent(KeyEvent{Err: getError(e1)}) {
				return
			}
		} else if input[0] == k32_keyEvent {
			kEvent := (*k32_event)(unsafe.Pointer(&input[2]))
			ev, ok := getKeyEvent(kEvent)
			if ok {
				for i := 0; i < int(kEvent.repeat_count); i++ {
					if false == produceEvent(ev) {
						return
					}
				}
			}
		}
	}
}

func initConsole() (err error) {
	// Create an interrupt event
	hInterrupt, err = windows.CreateEvent(nil, 0, 0, nil)
	if err != nil {
		return err
	}

	hConsoleIn, err = syscall.Open("CONIN$", windows.O_RDWR, 0)
	if err != nil {
		windows.Close(hInterrupt)
		return
	}

	go inputEventsProducer()
	return
}

func releaseConsole() {
	// Stop events producer
	windows.SetEvent(hInterrupt)
	quit <- true

	syscall.Close(hConsoleIn)
	windows.Close(hInterrupt)
}
