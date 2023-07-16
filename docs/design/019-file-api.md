# Introduce a File API module for k6

|                      |                                                                                                                                                                                                           |
| :------------------- | :-------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| **author**           | @oleiade                                                                                                                                                                                                  |
| **status**           | 🔧 proposal                                                                                                                                                                                              |
| **revisions**        | [previous](https://github.com/grafana/k6/pull/3101/commits/e2b8ddad40d013b56789cb4c89dd8f9c338f42d4), [initial](https://github.com/grafana/k6/pull/3101/commits/0669d16e76791241b75a2622729327880cd814e2) |
| **Proof of concept** | [branch](https://github.com/grafana/k6/tree/poc/experimental/fs-module/js/modules/k6/experimental/fs)                                                                                                     |
| **references**       | [#2977](https://github.com/grafana/k6/issues/2977) [#2974](https://github.com/grafana/k6/issues/2974)                                                                                                   |

## Problem definition

The current version of k6 lets users load file content via [the `open` function](https://k6.io/docs/javascript-api/init-context/open/), which is only accessible in the init context. However, the `open` function diverges from its counterparts in other languages and the Linux stack as it reads the whole file into memory rather than opening it for further interaction. This process leads to considerable memory consumption when loading large binary files (as the content ends up copied in each VUs) or when the `SharedArray` cannot be used.

In line with [our commitment to optimize large file handling in k6](https://github.com/grafana/k6/issues/2974), we propose introducing a new `fs` module. This module is intended to offer an intuitive and user-friendly API for file interactions within k6 scripts. We'll also provide some ideas for efficient file handling to minimize memory consumption during k6 execution.

### Context

Currently, files cannot be opened from within a function executed by a VU, only in the init context.

This is due to k6's design for distributed execution, particularly in the cloud. K6 runs the init context once, gathers resources, including files, and sends them to other instances where VU code runs.

### Requirements

- The solution must be native to k6, with the code being in the k6 repository and maintained by the k6 OSS team.
- The solution must use asynchronous operations.
- The solution restricts opening files in the init context only.
- The solution allows read-only operations on files in the init context and the vu, setup and teardown stages.
- The solution must provide support for reading text and binary files.
- The solution must support local and cloud execution seamlessly.
- The solution must provide substantial improvements to memory usage, compared to using the current `open()` function.
- The solution's API should be intuitive and hard to misuse for users familiar with filesystem APIs from other languages and technologies.

### Scope

#### In scope
- The proposed solution will offer an alternative to the existing `open` function.
- The solution will provide a File concept that supports read-only operations: reading file chunks into a buffer, reading the entire file content, and file seeking.
- The proposal could suggest changes to improve overall memory usage in k6 scripts.

#### Out of scope
- This proposal doesn't aim to solve memory consumption issues when using file content in HTTP requests, although minor improvements might be suggested.
- Changes to how k6 gathers resources, particularly files, are out of scope (ideas are proposed in the "Future Improvements" section).
- Despite the proposed API's potential to support them, write operations are beyond this proposal's scope:
  - The scope of this proposal is to be a drop-in replacement for the current `open` function.
  - The support of write operations would require substantial implementation effort and is out of the scope of this proposal.
  - There is no immediate need for k6 to support write operations on files.

## Proposed solution

We suggest implementing a minimalist, experimental file system (`fs`) module based on [Deno's fs module](https://deno.land/api@v1.32.3?s=Deno.open). The new module will allow users to interact with files, separating text and binary files. The module will provide an `open` function that returns a file handle/view for performing read operations.

The initial API will mostly be asynchronous, except for the `open` functionality which will be synchronous due to the current lack of support for `await` operations within the init context.

The API will have the following characteristics:
- Load file content exactly once into memory.
- As opening files is restricted to the init context, each VU receives a copy of each unique file handle, pointing to a unique memory area to avoid copying the whole buffer for each VU.
- Ensure each file handle has unique offsets allowing each VU to work independently.

A working [proof of concept](https://github.com/grafana/k6/tree/poc/experimental/fs-module/js/modules/k6/experimental/fs) of the new API is available on GitHub.

### Proposed API:

```typescript
/*
 * openSync opens a file and returns an instance of a
 * `File`.
 */
openSync(path: string): File

/*
 * open opens a file and resolves to an instance of a
 * `File`.
 *
 * Because the k6 init context does not support using await yet,
 * to use this function, users must use a workaround:
 *
 * ```
 * let f;
 * (async function() {
 *   f = await asyncOpen("./somefile"); // name for emphasis not as a proposal
 * }());
 * ```
 */
open(path: string): Promise<File>

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

    /*
     * close closes the file.
     */
    close(): Promise<void>
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

**N.B**: the `File` operations only support working with `ArrayBuffer` as of this proposal. This is based on the assumption we could somewhat easily add `TextDecoder` support to k6 (see comments [#2291](https://github.com/grafana/k6/issues/2291) and [#2440](https://github.com/grafana/k6/issues/2440)). If this assumption was to be invalidated, we could adopt the same API format and have two different read-operation variants on the File, or even expose two different kinds of files `TextFileHandle` and `BinaryFileHandle` for instance.

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

export default function teardown() {
    file.close();
}

function ab2str(buf) {
    return String.fromCharCode.apply(null, new Uint16Array(buf));
}
```

### Implementation details

The proposed API is made feasible through the following implementation aspects:

-   A file's content is loaded into memory only once:
    -   When a file is opened, its content is buffered at the module's root in a dedicated registry, returning a handle with a pointer to that buffer.
    -   Each VU receives a copy of the file handle, enabling them to interact with files using the unique memory area linked to the handle, instead of each receiving a full copy of the buffer.
    - As each invocation of `open*` receives a unique file handle linked to the same memory area, they each have unique offsets. This setup allows each VU to process file data independently without conflict or race conditions.

### Possible future improvements

#### Introduce stream support

The Deno API's `FsFile` our proposal is inspired by exposes a `readable` read-only property which is a [Streams API](https://developer.mozilla.org/en-US/docs/Web/API/Streams_API) `ReadableStream` allowing to stream the content of the file. We have an open issue tracking the implementation of the Streams API in k6 [#2978](https://github.com/grafana/k6/issues/2978).

#### Enable file opening within VU context

See [#3020](https://github.com/grafana/k6/issues/3020)

This is currently unachievable because we must anticipate which files will be opened. While some quick fixes might appear feasible (e.g., parsing the AST before execution to identify files), they quickly fall apart: What if the filename resides in a variable? A plausible solution would involve requiring users to declare necessary resources (files/folders) ahead of time. This approach would ensure these resources are captured and included in the archive for future VU code access.

## Conclusion

We believe the [proof of concept](https://github.com/grafana/k6/tree/poc/experimental/fs-module/js/modules/k6/experimental/fs) developed with this proposal illustrates the feasibility and benefits of developing such an API.
