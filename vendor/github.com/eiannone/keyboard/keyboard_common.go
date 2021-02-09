package keyboard

import (
	"errors"
	"time"
)

type (
	Key uint16

	KeyEvent struct {
		Key  Key   // One of Key* constants, invalid if 'Ch' is not 0
		Rune rune  // A unicode character
		Err  error // Error in case if input failed
	}
)

// Key constants, see GetKey() function.
const (
	KeyF1 Key = 0xFFFF - iota
	KeyF2
	KeyF3
	KeyF4
	KeyF5
	KeyF6
	KeyF7
	KeyF8
	KeyF9
	KeyF10
	KeyF11
	KeyF12
	KeyInsert
	KeyDelete
	KeyHome
	KeyEnd
	KeyPgup
	KeyPgdn
	KeyArrowUp
	KeyArrowDown
	KeyArrowLeft
	KeyArrowRight
	key_min // see terminfo
)

const (
	KeyCtrlTilde      Key = 0x00
	KeyCtrl2          Key = 0x00
	KeyCtrlSpace      Key = 0x00
	KeyCtrlA          Key = 0x01
	KeyCtrlB          Key = 0x02
	KeyCtrlC          Key = 0x03
	KeyCtrlD          Key = 0x04
	KeyCtrlE          Key = 0x05
	KeyCtrlF          Key = 0x06
	KeyCtrlG          Key = 0x07
	KeyBackspace      Key = 0x08
	KeyCtrlH          Key = 0x08
	KeyTab            Key = 0x09
	KeyCtrlI          Key = 0x09
	KeyCtrlJ          Key = 0x0A
	KeyCtrlK          Key = 0x0B
	KeyCtrlL          Key = 0x0C
	KeyEnter          Key = 0x0D
	KeyCtrlM          Key = 0x0D
	KeyCtrlN          Key = 0x0E
	KeyCtrlO          Key = 0x0F
	KeyCtrlP          Key = 0x10
	KeyCtrlQ          Key = 0x11
	KeyCtrlR          Key = 0x12
	KeyCtrlS          Key = 0x13
	KeyCtrlT          Key = 0x14
	KeyCtrlU          Key = 0x15
	KeyCtrlV          Key = 0x16
	KeyCtrlW          Key = 0x17
	KeyCtrlX          Key = 0x18
	KeyCtrlY          Key = 0x19
	KeyCtrlZ          Key = 0x1A
	KeyEsc            Key = 0x1B
	KeyCtrlLsqBracket Key = 0x1B
	KeyCtrl3          Key = 0x1B
	KeyCtrl4          Key = 0x1C
	KeyCtrlBackslash  Key = 0x1C
	KeyCtrl5          Key = 0x1D
	KeyCtrlRsqBracket Key = 0x1D
	KeyCtrl6          Key = 0x1E
	KeyCtrl7          Key = 0x1F
	KeyCtrlSlash      Key = 0x1F
	KeyCtrlUnderscore Key = 0x1F
	KeySpace          Key = 0x20
	KeyBackspace2     Key = 0x7F
	KeyCtrl8          Key = 0x7F
)

var (
	inputComm chan KeyEvent

	ping          = make(chan bool)
	doneClosing   = make(chan bool, 1)
	busy          = make(chan bool)
	waitingForKey = make(chan bool)
)

func IsStarted(timeout time.Duration) bool {
	select {
	case ping <- true:
		return true
	case <-time.After(timeout):
		return false
	}
}

func GetKeys(bufferSize int) (<-chan KeyEvent, error) {
	if IsStarted(time.Millisecond * 1) {
		if cap(inputComm) == bufferSize {
			return inputComm, nil
		}
		return nil, errors.New("channel already started with a different capacity")
	}
	select {
	case busy <- true:
		return nil, errors.New("cannot open keyboard because program is busy")
	default:
	}
	// Signal busy operation
	go func() {
		for <-busy {
		} // Close the routine when busy is false
	}()

	inputComm = make(chan KeyEvent, bufferSize)
	err := initConsole()
	if err != nil {
		close(inputComm)
		busy <- false
		return nil, err
	}

	// Signal ping subroutine started
	go func() {
		defer func() {
			releaseConsole()
			close(inputComm)
			doneClosing <- true
		}()
		for <-ping {
		} // Close the routine when ping is false
	}()
	busy <- false
	// Wait for ping subroutine to start
	ping <- true

	return inputComm, nil
}

func Open() (err error) {
	_, err = GetKeys(10)
	return
}

// Should be called after successful initialization when functionality isn't required anymore.
func Close() (err error) {
	// Checks if already closing
	select {
	case busy <- true:
		return errors.New("cannot close keyboard because program is busy")
	default:
	}
	// Checks if already closed
	if !IsStarted(time.Millisecond * 1) {
		return
	}

	// Signal busy operation
	go func() {
		for <-busy {
		} // Close the routine when busy is false
	}()

	// Stop responding to ping and closes initial subroutine
	ping <- false

	// Cancel GetKey() operations
	select {
	case waitingForKey <- false:
		break
	default:
	}

	// Wait for closing finished
	<-doneClosing

	busy <- false
	return
}

func GetKey() (rune, Key, error) {
	// Check if opened
	if !IsStarted(time.Millisecond * 50) {
		return 0, 0, errors.New("keyboard not opened")
	}
	// Check if already waiting for key
	select {
	case waitingForKey <- true:
		return 0, 0, errors.New("already waiting for key")
	default:
	}

	for {
		select {
		case ev := <-inputComm:
			return ev.Rune, ev.Key, ev.Err

		case keepAlive := <-waitingForKey:
			if !keepAlive {
				return 0, 0, errors.New("operation canceled")
			}
		}
	}
}

func GetSingleKey() (ch rune, key Key, err error) {
	err = Open()
	if err == nil {
		ch, key, err = GetKey()
		errClosing := Close()
		if err == nil {
			err = errClosing
		}
	}
	return
}
