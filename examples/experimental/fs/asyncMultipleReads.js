import { open } from "k6/experimental/fs";

// export const options = {
//     vus: 2,
//     iterations: 2,
// }

let file;
(async function () {
	file = await open("bonjour.txt");
})();

export default async function() {
    let buffer = new Uint8Array(5);
    let readPromise = file.read(buffer);
    buffer[0] = 3;

    console.log(`buffer right after read: ${buffer}`)

    await readPromise;

    console.log(`buffer after await: ${buffer}`)

    // throw 'buffer before awaited read:' + bufferCopy + ', buffer after await: ' + buffer
}