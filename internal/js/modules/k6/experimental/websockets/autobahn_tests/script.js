import { WebSocket } from "k6/x/websockets"
import { sleep, check } from "k6"
import exec from 'k6/execution'

export let options = {
	iterations: 247, // get this value from the Autobahn server
	vus: 3,
}

const base = `ws://127.0.0.1:9001`
const agent = "k6v383"

export default function() {
	let testCase = exec.scenario.iterationInTest+1
	let url = `${base}/runCase?case=${testCase}&agent=${agent}`;
	let ws = new WebSocket(url);

	ws.addEventListener("open", () => {
		console.log(`Testing case #${testCase}`)
	});

	ws.addEventListener("message", (e) => {
		if (e.event === 'ERROR') {
			console.log(`VU ${__VU}: test: #${testCase} error:`, e.data, `and message:`, e.message)
			return
		}
		ws.send(e.data)
	})

	ws.addEventListener("error", (e) => {
		console.error(`test: #${testCase} error:`, e)
		ws.close()
	})
}

export function teardown() {
	let ws = new WebSocket(`${base}/updateReports?agent=${agent}`)
	ws.addEventListener("open", (e) => {
		console.log("Updating the report")
	});

	ws.addEventListener("error", (e) => {
		console.error("Updating the report failed:", e)
		ws.close()
	});
}
