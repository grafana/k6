package tests

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"
	"go.k6.io/k6/internal/cmd"
)

func TestEventLoop(t *testing.T) {
	t.Parallel()
	script := []byte(`
		setTimeout(()=> {console.log("initcontext setTimeout")}, 200)
		console.log("initcontext");
		export default function() {
			setTimeout(()=> {console.log("default setTimeout")}, 200)
			console.log("default");
		};
		export function setup() {
			setTimeout(()=> {console.log("setup setTimeout")}, 200)
			console.log("setup");
		};
		export function teardown() {
			setTimeout(()=> {console.log("teardown setTimeout")}, 200)
			console.log("teardown");
		};
		export function handleSummary() {
			setTimeout(()=> {console.log("handleSummary setTimeout")}, 200)
			console.log("handleSummary");
		};
`)
	eventLoopTest(t, script, func(logLines []string) {
		require.Equal(t, []string{
			"initcontext", // first initialization
			"initcontext setTimeout",
			"initcontext", // for vu
			"initcontext setTimeout",
			"initcontext", // for setup
			"initcontext setTimeout",
			"setup", // setup
			"setup setTimeout",
			"default", // one iteration
			"default setTimeout",
			"initcontext", // for teardown
			"initcontext setTimeout",
			"teardown", // teardown
			"teardown setTimeout",
			"initcontext", // for handleSummary
			"initcontext setTimeout",
			"handleSummary", // handleSummary
			"handleSummary setTimeout",
		}, logLines)
	})
}

func TestEventLoopCrossScenario(t *testing.T) {
	t.Parallel()
	script := []byte(`
		import exec from "k6/execution"
		export const options = {
			scenarios: {
				"first":{
					executor: "shared-iterations",
					maxDuration: "1s",
					iterations: 1,
					vus: 1,
					gracefulStop:"1s",
				},
				"second": {
					executor: "shared-iterations",
					maxDuration: "1s",
					iterations: 1,
					vus: 1,
					startTime: "3s",
				}
			}
		}
		export default function() {
			let i = exec.scenario.name
			setTimeout(()=> {console.log(i)}, 3000)
		}
`)

	eventLoopTest(t, script, func(logLines []string) {
		require.Equal(t, []string{
			"setTimeout 1 was stopped because the VU iteration was interrupted",
			"second",
		}, logLines)
	})
}

func TestEventLoopDoesntCrossIterations(t *testing.T) {
	t.Parallel()
	script := []byte(`
		import { sleep } from "k6"
		export const options = {
			iterations: 2,
			vus: 1,
		}

		export default function() {
			let i = __ITER;
			setTimeout(()=> { console.log(i) }, 1000)
			if (__ITER == 0) {
				throw "just error"
			} else {
				sleep(1)
			}
		}
`)

	eventLoopTest(t, script, func(logLines []string) {
		require.Equal(t, []string{
			"setTimeout 1 was stopped because the VU iteration was interrupted",
			"just error\n\tat default (file:///-:13:5(14))\n", "1",
		}, logLines)
	})
}

func eventLoopTest(t *testing.T, script []byte, testHandle func(logLines []string)) {
	ts := NewGlobalTestState(t)
	ts.CmdArgs = []string{"k6", "--quiet", "run", "-"}
	ts.Stdin = bytes.NewBuffer(
		append([]byte("import { setTimeout } from 'k6/timers';\n"), script...),
	)

	cmd.ExecuteWithGlobalState(ts.GlobalState)

	testHandle(ts.LoggerHook.Lines())
}
