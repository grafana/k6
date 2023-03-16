;; $wasi_arg is a WASI command which copies null-terminated args to stdout.
(module $wasi_arg
	;; args_get reads command-line argument data.
	;;
	;; See https://github.com/WebAssembly/WASI/blob/snapshot-01/phases/snapshot/docs.md#-args_getargv-pointerpointeru8-argv_buf-pointeru8---errno
    (import "wasi_snapshot_preview1" "args_get"
        (func $wasi.args_get (param $argv i32) (param $argv_buf i32) (result (;errno;) i32)))

	;; args_sizes_get returns command-line argument data sizes.
	;;
	;; See https://github.com/WebAssembly/WASI/blob/snapshot-01/phases/snapshot/docs.md#-args_sizes_get---errno-size-size
    (import "wasi_snapshot_preview1" "args_sizes_get"
        (func $wasi.args_sizes_get (param $result.argc i32) (param $result.argv_len i32) (result (;errno;) i32)))

    ;; fd_write write bytes to a file descriptor.
    ;;
    ;; See https://github.com/WebAssembly/WASI/blob/snapshot-01/phases/snapshot/docs.md#fd_write
    (import "wasi_snapshot_preview1" "fd_write"
        (func $wasi.fd_write (param $fd i32) (param $iovs i32) (param $iovs_len i32) (param $result.size i32) (result (;errno;) i32)))

    ;; WASI commands are required to export "memory". Particularly, imported functions mutate this.
    ;;
    ;; Note: 1 is the size in pages (64KB), not bytes!
    ;; See https://www.w3.org/TR/2019/REC-wasm-core-1-20191205/#memories%E2%91%A7
    (memory (export "memory") 1)

    ;; $iovs are offset/length pairs in memory fd_write copies to the file descriptor.
    ;; $main will only write one offset/length pair, corresponding to null-terminated args.
    (global $iovs i32 i32.const 1024) ;; 1024 is an arbitrary offset larger than the args.

    ;; WASI parameters are usually memory offsets, you can ignore values by writing them to an unread offset.
    (global $ignored i32 i32.const 32768)

    ;; _start is a special function defined by a WASI Command that runs like a main function would.
    ;;
    ;; See https://github.com/WebAssembly/WASI/blob/snapshot-01/design/application-abi.md#current-unstable-abi
    (func $main (export "_start")
        ;; To copy an argument to a file, we first need to load it into memory.
        (call $wasi.args_get
            (global.get $ignored) ;; ignore $argv as we only read the argv_buf
            (i32.const 0) ;; Write $argv_buf (null-terminated args) to memory offset zero.
        )
        drop ;; ignore the errno returned

        ;; Next, we need to know how many bytes were loaded, as that's how much we'll copy to the file.
        (call $wasi.args_sizes_get
            (global.get $ignored) ;; ignore $result.argc as we only read the argv_buf.
            (i32.add (global.get $iovs) (i32.const 4)) ;; store $result.argv_len as the length to copy
        )
        drop ;; ignore the errno returned

        ;; Finally, write the memory region to the file.
        (call $wasi.fd_write
            (i32.const 1) ;; $fd is a file descriptor and 1 is stdout (console).
            (global.get $iovs) ;; $iovs is the start offset of the IO vectors to copy.
            (i32.const 1) ;; $iovs_len is the count of offset/length pairs to copy to memory.
            (global.get $ignored) ;; ignore $result.size as we aren't verifying it.
        )
        drop ;; ignore the errno returned
    )
)
