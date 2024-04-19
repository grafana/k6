// Original source file: https://github.com/web-platform-tests/wpt/blob/3ede6629030918b00941c2fb7d176a18cbea16ea/encoding/streams/resources/readable-stream-to-array.js
'use strict';

// NOTE: This is a simplified version of the original implementation
// found at: https://github.com/web-platform-tests/wpt/blob/3ede6629030918b00941c2fb7d176a18cbea16ea/encoding/streams/resources/readable-stream-to-array.js#L3
// that doesn't rely on WritableStreams.
function readableStreamToArray(stream) {
	const reader = stream.getReader();
	const array = [];

	// A function to recursively read chunks from the stream
	function readNextChunk() {
		return reader.read().then(({done, value}) => {
			if (done) {
				// Stream has been fully read
				return array;
			}
			// Add the chunk to the array
			array.push(value);
			// Recursively read the next chunk
			return readNextChunk();
		});
	}

	return readNextChunk();
}