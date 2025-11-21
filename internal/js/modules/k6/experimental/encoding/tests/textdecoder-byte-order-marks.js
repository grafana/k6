var testCases = [
  {
    encoding: "utf-8",
    bom: [0xef, 0xbb, 0xbf],
    bytes: [
      0x7a, 0xc2, 0xa2, 0xe6, 0xb0, 0xb4, 0xf0, 0x9d, 0x84, 0x9e, 0xf4, 0x8f,
      0xbf, 0xbd,
    ],
  },
    {
      encoding: "utf-16le",
      bom: [0xff, 0xfe],
      bytes: [
        0x7a, 0x00, 0xa2, 0x00, 0x34, 0x6c, 0x34, 0xd8, 0x1e, 0xdd, 0xff, 0xdb,
        0xfd, 0xdf,
      ],
    },
    {
      encoding: "utf-16be",
      bom: [0xfe, 0xff],
      bytes: [
        0x00, 0x7a, 0x00, 0xa2, 0x6c, 0x34, 0xd8, 0x34, 0xdd, 0x1e, 0xdb, 0xff,
        0xdf, 0xfd,
      ],
    },
];

var string = "z\xA2\u6C34\uD834\uDD1E\uDBFF\uDFFD"; // z, cent, CJK water, G-Clef, Private-use character

testCases.forEach(function (t) {
  test(function () {
    var decoder = new TextDecoder(t.encoding);
    assert_equals(
      decoder.decode(new Uint8Array(t.bytes)),
      string,
      "Sequence without BOM should decode successfully"
    );

    assert_equals(
      decoder.decode(new Uint8Array(t.bom.concat(t.bytes))),
      string,
      "Sequence with BOM should decode successfully (with no BOM present in output)"
    );

      testCases.forEach(function (o) {
        if (o === t) return;

        assert_not_equals(
          decoder.decode(new Uint8Array(o.bom.concat(t.bytes))),
          string,
          "Mismatching BOM should not be ignored - treated as garbage bytes."
        );
      });
  }, "Byte-order marks: " + t.encoding);
});
