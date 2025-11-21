// Source: https://github.com/web-platform-tests/wpt/blob/f5cac385941da42024176a1763f3941e33bb7bb0/encoding/textdecoder-copy.any.js
//
// Changes we made to adapt it to k6's environment:
// - Dropped SharedArrayBuffer from the test cases.
// - Used a custom `createBuffer` function to avoid all the WPT specific setup.
// - Dropped the usage of the WPT `test` function.
// - Commented out the streams specific tests, as we do not support it yet.

function createBuffer(_, size) {
    return new ArrayBuffer(size);
}

// ["ArrayBuffer", "SharedArrayBuffer"].forEach(arrayBufferOrSharedArrayBuffer => {
["ArrayBuffer", ].forEach(arrayBuffer => {
    test(() => {
        const buf = createBuffer(arrayBuffer, 2);
        const view = new Uint8Array(buf);
        const buf2 = createBuffer(arrayBuffer, 2);
        const view2 = new Uint8Array(buf2);
        const decoder = new TextDecoder("utf-8");
        view[0] = 0xEF;
        view[1] = 0xBB;
        view2[0] = 0xBF;
        view2[1] = 0x40;
        assert_equals(decoder.decode(buf, {stream:true}), "");
        view[0] = 0x01;
        view[1] = 0x02;
        assert_equals(decoder.decode(buf2), "@");
    }, "Modify buffer after passing it in (" + arrayBuffer  + ")");
});