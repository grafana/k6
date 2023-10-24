import { open } from "k6/experimental/fs";


export const options = {
    vus: 100,
    iterations: 1000,
}

let file;
(async function () {
    // 5 bytes "hello"
	file = await open("bonjour.txt");
})();

export default async function() {
    // let buffer = new Uint8Array(6);
    // console.log(`initial buffer: ${buffer}`)

    // let bytesRead = await file.read(buffer);
    // console.log(`read the ${bytesRead} bytes: ${buffer}`);

    // // bytesRead = await file.read(buffer);
    // // console.log(`read the ${bytesRead} bytes: ${buffer}`);

    // await file.read(buffer);

    let buffer = new Uint8Array(4)
    let p1 = file.read(buffer);
    let p2 = file.read(buffer);

    await Promise.all([p1, p2]);
}


// In JS runtime, is there a guarantee that promises are executed in the order they were instantiated/called?
// 
// We need to ensure, somehow, that the modification of the buffer, as a result of the `file.read` operation is done sequentially/synchronously
//   - What are the user expectations?
//   - What guarantees can we give?
//   - What guarantees can we not give?
//   - What solution, and where should it be implemented? (Task Queue? FileApi-specific command-based operations queue?)