const ping = require("k6/x/icmp").ping
const assert = require("k6/x/assert")

module.exports = () => {
  assert.true(ping("127.0.0.1"), "Loopback host should be alive")
  assert.false(ping("192.0.2.1", { timeout: "500ms" }), "TEST-NET-1 IP should be unreachable")
}
