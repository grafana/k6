import { ReadableStream } from 'k6/streams'
import { sleep } from 'k6'

function numbersStream() {
	let currentNumber = 0

	return new ReadableStream({
		start(controller) {
			while (currentNumber <= 5) {
				controller.enqueue(currentNumber++)
				sleep(1)
			}

			controller.close()
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
