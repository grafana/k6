import { WritableStream } from 'k6/experimental/streams'
import { setTimeout } from 'k6/timers'

function delay(milliseconds) {
	return new Promise((resolve) => setTimeout(resolve, milliseconds))
}

// loggingStream returns a WritableStream whose underlying sink writes chunks slowly enough
// for the producer to observe and respect backpressure through writer.ready.
function loggingStream() {
	const chunks = []

	return new WritableStream({
		start() {
			console.log('stream opened')
		},
		async write(chunk) {
			await delay(200)

			chunks.push(chunk)
			console.log(`wrote ${chunk}`)
		},
		close() {
			console.log(`stream closed after receiving: ${chunks.join(', ')}`)
		},
		abort(reason) {
			console.log(`stream aborted: ${reason}`)
		},
	}, { highWaterMark: 1 })
}

export default async function () {
	const stream = loggingStream()
	const writer = stream.getWriter()
	const writes = []

	for (let i = 1; i <= 5; i++) {
		await writer.ready
		writes.push(writer.write(`chunk ${i}`))
	}

	await Promise.all(writes)
	await writer.close()
	await writer.closed

	console.log('we are done')
}
