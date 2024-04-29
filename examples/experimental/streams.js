import { ReadableStream } from 'k6/experimental/streams'
import { setTimeout } from 'k6/timers'

function numbersStream() {
	let currentNumber = 0

	return new ReadableStream({
		start(controller) {
			const fn = () => {
				if (currentNumber < 5) {
					controller.enqueue(++currentNumber)
					setTimeout(fn, 1000)
					return;
				}

				controller.close()
			}
			setTimeout(fn, 1000)
		},
	})
}

export default async function () {
	const stream = numbersStream()
	const reader = stream.getReader()

	while (true) {
		const { done, value } = await reader.read()
		if (done) break
		console.log(`received number ${value} from stream`)
	}

	console.log('we are done')
}
