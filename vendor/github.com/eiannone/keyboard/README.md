# Keyboard
Simple library to listen for keystrokes from the keyboard

The code is inspired by [termbox-go](https://github.com/nsf/termbox-go) library.

### Installation
Install and update this go package with `go get -u github.com/eiannone/keyboard`

### Usage
Example of getting a single keystroke:

```go
char, _, err := keyboard.GetSingleKey()
if (err != nil) {
    panic(err)
}
fmt.Printf("You pressed: %q\r\n", char)
```

Example of getting a series of keystrokes with a blocking `GetKey()` function:
```go
package main

import (
	"fmt"
	"github.com/eiannone/keyboard"
)

func main() {		
	if err := keyboard.Open(); err != nil {
		panic(err)
	}
	defer func() {
		_ = keyboard.Close()
	}()

	fmt.Println("Press ESC to quit")
	for {
		char, key, err := keyboard.GetKey()
		if err != nil {
			panic(err)
		}
		fmt.Printf("You pressed: rune %q, key %X\r\n", char, key)
        if key == keyboard.KeyEsc {
			break
		}
	}	
}
```

Example of getting a series of keystrokes using a channel:
```go
package main

import (
	"fmt"
	"github.com/eiannone/keyboard"
)

func main() {
	keysEvents, err := keyboard.GetKeys(10)
	if err != nil {
		panic(err)
	}
	defer func() {
		_ = keyboard.Close()
	}()

	fmt.Println("Press ESC to quit")
	for {
		event := <-keysEvents
		if event.Err != nil {
			panic(event.Err)
		}
		fmt.Printf("You pressed: rune %q, key %X\r\n", event.Rune, event.Key)
		if event.Key == keyboard.KeyEsc {
			break
		}
	}
}
```