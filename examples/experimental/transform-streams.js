import { ReadableStream, TransformStream } from 'k6/experimental/streams'
import { setTimeout } from 'k6/timers'

// numbersStream returns a ReadableStream that emits the numbers 1 through 5, one per tick.
function numbersStream() {
	let current = 0

	return new ReadableStream({
		start(controller) {
			const fn = () => {
				if (current < 5) {
					controller.enqueue(++current)
					setTimeout(fn, 100)
					return
				}

				controller.close()
			}
			setTimeout(fn, 100)
		},
	})
}

// doubler is a TransformStream that doubles every number written to its writable side, and
// makes the result available on its readable side.
function doubler() {
	return new TransformStream({
		transform(chunk, controller) {
			controller.enqueue(chunk * 2)
		},
	})
}

export default async function () {
	const source = numbersStream()
	const transform = doubler()

	// Pump the source into the transform stream's writable side, respecting backpressure.
	const writer = transform.writable.getWriter()
	const reader = source.getReader()
	const pump = (async () => {
		for (;;) {
			const { done, value } = await reader.read()
			if (done) {
				await writer.close()
				return
			}
			await writer.ready
			await writer.write(value)
		}
	})()

	// Consume the transform stream's readable side.
	const transformed = transform.readable.getReader()
	for (;;) {
		const { done, value } = await transformed.read()
		if (done) break
		console.log(`doubled number: ${value}`)
	}

	await pump
	console.log('we are done')
}
