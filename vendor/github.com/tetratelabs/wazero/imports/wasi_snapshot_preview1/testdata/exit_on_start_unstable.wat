(module $exit_on_start
  ;; Old TinyGo (e.g. 0.19) uses wasi_unstable not wasi_snapshot_preview1
  (import "wasi_unstable" "proc_exit"
    (func $wasi.proc_exit (param $rval i32)))

  (func (export "_start")
     i32.const 2           ;; push $rval onto the stack
     call $wasi.proc_exit  ;; return a sys.ExitError to the caller
  )
)
