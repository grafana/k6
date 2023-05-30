# Introduce a File API module for k6

|         |         |
| ---------- | ------------ |
| **author** | @oleiade     |
| **status** | 🔧 proposal  |
| **Proof of concept** | [branch](https://github.com/grafana/k6/tree/poc/experimental/fs-module/js/modules/k6/experimental/fs) |
| **references**           | [[#2977](https://github.com/grafana/k6/issues/2977) [[#2974](https://github.com/grafana/k6/issues/2974)            |

## Problem definition

The current version of k6 lets users load file content via [the `open` function](https://k6.io/docs/javascript-api/init-context/open/), which is only accessible in the init context. However, the `open` function diverges from its counterparts in other languages and the Linux stack as it reads the whole file into memory rather than opening it for further interaction. This process leads to considerable memory consumption when loading large binary files (as the content ends up copied in each VUs) or when the `SharedArray` cannot be used.

In line with [our commitment to optimize large file handling in k6](https://github.com/grafana/k6/issues/2974), we propose introducing a new `fs` module. This module is intended to offer an intuitive and user-friendly API for file interactions within k6 scripts. We'll also provide some ideas for efficient file handling to minimize memory consumption during k6 execution.

### Context
Currently, files cannot be opened from within a function executed by a VU, only in the init context.

This is due to k6's design for distributed execution, particularly in the cloud. K6 runs the init context once, gathers resources, including files, and sends them to other instances where VU code runs.

### Assumptions
- The solution should be native to k6, with the code being in the k6 repository and maintained by the k6 OSS team.
- The new functionality's API should be intuitive for users familiar with filesystem APIs from other languages and technologies.
- The module should introduce only essential APIs: read-only operations.
- The proposed API should not affect file handling within HTTP requests.

### Requirements
- The solution should allow file interaction during k6 script execution.
- The solution should resort to asynchronous operations as much as possible.
- The module should operate seamlessly in local and cloud setups.
- The module should strive to optimize memory usage when handling files.

### Scope

#### In scope
- The proposed solution will offer an alternative to the existing `open` function.
- The solution will provide a File concept that supports read-only operations: reading file chunks into a buffer, reading the entire file content, and file seeking.
- The proposal could suggest changes to improve overall memory usage in k6 scripts.

#### Out of scope
- This proposal doesn't aim to solve memory consumption issues when using file content in HTTP requests, although minor improvements might be suggested.
- Changes to how k6 gathers resources, particularly files, are out of scope (ideas are proposed in the "Future Improvements" section).
- Despite the proposed API's potential to support them, write operations are beyond this proposal's scope.

## Proposed solution

We suggest implementing a minimalist, experimental file system (`fs`) module based on [Deno's fs module](https://deno.land/api@v1.32.3?s=Deno.open). The new module will allow users to interact with files, separating text and binary files. The module will provide an `open` function that returns a file handle/view for performing read operations.

The initial API will mostly be asynchronous, except for the `open` functionality which will be synchronous due to the current lack of support for `await` operations within the init context.

The API will have the following characteristics:
- Load file content exactly once into memory.
- Provide each VU a copy of the file handle pointing to a unique memory area to avoid copying the whole buffer for each VU.
- Ensure each file handle has unique offsets allowing each VU to work independently.

A working [proof of concept](https://github.com/grafana/k6/tree/poc/experimental/fs-module/js/modules/k6/experimental/fs) of the new API is available on GitHub.

### Proposed API:

```typescript
/*
 * readFile reads the whole content of a file and
 * returns its content as an `ArrayBuffer`.
 *
 * It effectively is a drop-in replacement for the
 * current `open(filename, 'b')` API.
 */
readFileSync(filename: string): ArrayBuffer

/*
 * readFile reads the whole content of a file and
 * returns its content as a `string`.
 *
 * It effectively is a drop-in replacement for the
 * current `open(filename, 'r')` API.
 */
readTextFileSync(filename: string): string

/*
 * openSync opens a file and returns an instance of a
 * `File`.
 */
openSync(path: string) File

/*
 * File is an abstraction to interact with
 * files which exposes read-only operations.
 */
interface File {
    /*
     * read reads the file into an array buffer.
     * resolves to either the number of bytes read during the operation
     * or `null` if there was nothing to read.
     */
	read(p ArrayBuffer | TypedArray | DataView): Promise<number>

	/*
	 * readAll reads the whole content of the file and
	 * returns a promise that will resolve to its content
	 * as an `ArrayBuffer`.
	 */
	readAll(): Promise<ArrayBuffer>

	/*
	 * Seek to the given `offset` under mode given by `whence`.
	 * The call resolves to the new position within the resource
	 * (bytes from the start).
	 */
	seek(offset: number, whence: SeekMode): Promise<number>

	/*
	 * Resolves to a `FileInfo` describing the file.
	 */
	stat(): Promise<FileInfo>
}

/*
 * FileInfo provides information about a file.
 */
interface FileInfo {
	/*
	 * the filename of the file
	 */
	name: string

	/*
	 * the size of the file, in bytes.
	 */
	size: number
}
```

**N.B**: although text and binary files are distinguished by the top-level `readTextFile` and `readFile` operations, the `File` doesn't offer that distinction at the moment. This is based on the assumption we could somewhat easily add `TextDecoder` support to k6. If this assumption was to be invalidated, we could adopt the same API format and have two different read-operation variants on the File, or even expose two different kinds of files `TextFileHandle` and `BinaryFileHandle` for instance.

## Example usage

```javascript
import { openSync, SeekMode } from 'k6/experimental/fs';

export const options = {
    scenarios: {
      default: {
        executor: 'constant-vus',
        vus: 100,
        duration: '1m',
      },
    },
};

const file = openSync("./data.csv");

export default async function () {
    let resultString = ""

    let buffer = new Uint8Array(10);
    let n = await file.read(buffer);
    resultString += ab2str(buffer)

    // Read the same data again
    n = await file.read(buffer);
    resultString += ab2str(buffer)

    // Read the same data again
    n = await file.read(buffer);
    resultString += ab2str(buffer)

    await file.seek(0, SeekMode.Start);

    console.log(`[vu ${__VU}] resultString: ${resultString}`);
}

function ab2str(buf) {
    return String.fromCharCode.apply(null, new Uint16Array(buf));
}
```

### Potential additions

We have intentionally excluded certain elements from the proposed API. Specifically, the asynchronous `open` function is currently unusable in the init context as it does not support the `await` keyword.

Additionally, while Deno provides synchronous counterparts for its entire API, we may also want to consider doing the same. The primary argument for asynchronous code is to facilitate non-blocking IO.

```typescript
/*
 * Asynchronous counterparts to currently synchronous
 * proposed APIs.
 */
open(path: string): Promise<File>
readFile(filename: string): Promise<ArrayBuffer>
readTextFile(filename: string): Promise<string>

/*
 * Synchronous counterparts to the already
 * proposed APIs.
 */
interface File {
	readSync(p ArrayBuffer | TypedArray | DataView): number
	readAllSync(): ArrayBuffer
	seekSync(offset: number, whence: number): number
	statSync(): FileInfo
}
```

### Implementation details

The proposed API is made feasible through the following implementation aspects:

-   A file's content is loaded into memory only once:
    -   When a file is opened, its content is buffered at the module's root in a dedicated registry, returning a handle with a pointer to that buffer.
    -   Each VU receives a copy of the file handle, enabling them to interact with files using the unique memory area linked to the handle, instead of each receiving a full copy of the buffer.
    -   As each VU receives a unique file handle linked to the same memory area, they each have unique offsets. This setup allows each VU to process file data independently without conflict or race conditions.
    -   If we plan to introduce write operations to this API in the future, a synchronization mechanism would be required to ensure adherence to the multiple reader/single writer architecture constraints.

### Possible future improvements

#### Introduce stream support

The Deno API's `FsFile` our proposal is inspired by exposes a `readable` read-only property which is a Streams API `ReadableStream` allowing to stream the content of the file.

#### Enable file opening within VU context

This is currently unachievable because we must anticipate which files will be opened. While some quick fixes might appear feasible (e.g., parsing the AST before execution to identify files), they quickly fall apart: What if the filename resides in a variable? A plausible solution would involve requiring users to declare necessary resources (files/folders) ahead of time. This approach would ensure these resources are captured and included in the archive for future VU code access.

## Conclusion

We believe the [proof of concept](https://github.com/grafana/k6/tree/poc/experimental/fs-module/js/modules/k6/experimental/fs) developed along this proposal illustrates the feasability and benefits of developing such an API.