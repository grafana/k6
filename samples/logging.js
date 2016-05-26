var vu = require('vu');
var log = require('log');
log.debug("debug", {id: vu.id()});
log.info("info", {id: vu.id()});
log.warn("warn", {id: vu.id()});
log.error("error", {id: vu.id()});
