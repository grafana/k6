# Opening files within the VU stage


| Date | Change |
| --- | --- |
| Authors | @oleiade |
| Status | Draft |
| Issue | [#3020](https://github.com/grafana/k6/issues/3020)

## Problem statement

In its present state, k6's approach to handling external files, both in local/OSS and cloud contexts, restricts the ability to open files within the VU (Virtual User) stage.

Question: **How might we enable k6 to access external resources directly from the VU stage?**


### Rationale

One of k6 main features is the ability to create reproducible, idempotent archives. These archives can be run consistently across various environments, permitting users to draft scripts locally and then either upload or execute them in the cloud. 

To accomplish this, k6 assembles user scripts and their associated assets (which can range from other scripts, as well as text (JSON, CSV, etc.) and binary files (protobuf, images, etc.)) into a specific tar archive format. Furthermore, k6 can initiate a test by unpacking and executing such an archive.

However, when dealing with file access, a significant limitation arises: for k6 to maintain its idempotent behavior, it must preemptively recognize which resources to incorporate in the archive and their locations. 


Yet, if users dynamically determine file paths during script execution (rather than during compilation), k6 can't predict the resources accessed within the VU stage. 

This unpredictability stems from the fact that the VU stage is only evaluated (and any dynamic variables are only assigned) when k6 starts the VU execution of the script. Therefore, k6's current design confines file access to the init stage, which is executed a single time before VU activities commence, and any processes within the init stage are subsequently available to the VU stage.

### Requirements

- **File access in VU stage**: The adopted solution should permit file access both within the VU stage and the init stage.
- **Idempotency and reproducibility**: The adopted solution should ensure that the archives remain reproducible and idempotent, irrespective of where files are accessed.
- **Backward compatibility**: The adopted solution should be compatible with the current k6 archive command and should not require any change to the command itself. Namely, it should remain possible to open files in the init stage.
- **Developer experience**: The adopted solution should improve the developer experience and limit the cost of adopting the new file handling.
- **Error handling**: The adopted solution should allow for and provide precise error handling and transparent reporting, especially when dealing with file access issues or conflicts in the VU stage (which is a strength of the current way of handling files).
- **Backward compatibility**: The adopted solution should refrain from introducing breaking changes to the current solution. 

## Possible Solutions

### Solution A: Prior declaration of resources

#### Using a dedicated API

We can provide an API, possibly as part of the new fs module, for users to explicitly include files.

```javascript
import { open, include } from "k6/experimental/fs";

include('my/source/file.txt')
include('my/dir')

export default function () {
    // succeeds
    await open('my/source/file.txt');
    
    // fails with 'resource csvs/data.csv not included by k6, ...'
    await open('csvs/data.csv')
}
```

**Key Points**:
- The `include` function is only usable in the init stage. Invoking it within VU code will trigger an error.
- Using `include` ensures a file's inclusion during the archive process, making it accessible in the VU stage.
- Opening an un-included file in the VU stage will result in an error.
Files opened in the init stage without prior inclusion should still operate as they currently do (though this is debatable) to maintain backward compatibility.

#### Using dedicated option set

In addition to, or instead of the API, users can specify which files to include via a set of options.

```javascript
export const options = {
    // ...
    
    include: [
        'my/source/file.txt',
        'testdata',
    ]
}

export default function () {
        // succeeds
    await open('my/source/file.txt');
    
    // fails with 'resource csvs/data.csv not included by k6, ...'
    await open('csvs/data.csv')
}
```

**Key Points**:
- The rules from the explicit API still apply: if a file isn't referenced in the options and is opened within the VU stage, an error occurs.
- Attempting to open an un-referenced file within the init stage may not result in an error, ensuring backward compatibility.

### Solution B: Awaiting Proposals

Have a novel idea? Please share it with us in a dedicated commit :bow:

## Technical Decisions

To be agreed upon and decided.