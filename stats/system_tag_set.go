package stats

import (
	"fmt"
)

// This code is borrowed from "enumer -type=SystemTagSet -transform=snake -trimprefix=Tag"
// with modification to TagSubProto to be represent as subproto instead of sub_proto.
//
// The original code gen is "protosub_proto...", here we want sub_proto to become subproto,
// so we remove the "_" and append a space at the start of string. It ends up making the index
// start at 1 instead of 0. And the accessing the TagProto and TagSubProto become:
//
//    _SystemTagSetName[0:5] -> _SystemTagSetName[1:6]
//    _SystemTagSetName[5:14] -> _SystemTagSetName[6:14]
//
// TODO(cuonglm): Find (or write?) another tool to do this.
//nolint:gochecknoglobals
const _SystemTagSetName = " protosubprotostatusmethodurlnamegroupcheckerrorerror_codetls_versionitervuocsp_statusip"

//nolint:gochecknoglobals
var _SystemTagSetMap = map[SystemTagSet]string{
	1:     _SystemTagSetName[1:6],
	2:     _SystemTagSetName[6:14],
	4:     _SystemTagSetName[14:20],
	8:     _SystemTagSetName[20:26],
	16:    _SystemTagSetName[26:29],
	32:    _SystemTagSetName[29:33],
	64:    _SystemTagSetName[33:38],
	128:   _SystemTagSetName[38:43],
	256:   _SystemTagSetName[43:48],
	512:   _SystemTagSetName[48:58],
	1024:  _SystemTagSetName[58:69],
	2048:  _SystemTagSetName[69:73],
	4096:  _SystemTagSetName[73:75],
	8192:  _SystemTagSetName[75:86],
	16384: _SystemTagSetName[86:88],
}

func (ts SystemTagSet) String() string {
	if str, ok := _SystemTagSetMap[ts]; ok {
		return str
	}
	return fmt.Sprintf("SystemTagSet(%d)", ts)
}

//nolint:gochecknoglobals
var _SystemTagSetValues = []SystemTagSet{1, 2, 4, 8, 16, 32, 64, 128, 256, 512, 1024, 2048, 4096, 8192, 16384}

//nolint:gochecknoglobals
var _SystemTagSetNameToValueMap = map[string]SystemTagSet{
	_SystemTagSetName[1:6]:   1,
	_SystemTagSetName[6:14]:  2,
	_SystemTagSetName[14:20]: 4,
	_SystemTagSetName[20:26]: 8,
	_SystemTagSetName[26:29]: 16,
	_SystemTagSetName[29:33]: 32,
	_SystemTagSetName[33:38]: 64,
	_SystemTagSetName[38:43]: 128,
	_SystemTagSetName[43:48]: 256,
	_SystemTagSetName[48:58]: 512,
	_SystemTagSetName[58:69]: 1024,
	_SystemTagSetName[69:73]: 2048,
	_SystemTagSetName[73:75]: 4096,
	_SystemTagSetName[75:86]: 8192,
	_SystemTagSetName[86:88]: 16384,
}

// SystemTagSetString retrieves an enum value from the enum constants string name.
// Throws an error if the param is not part of the enum.
func SystemTagSetString(s string) (SystemTagSet, error) {
	if val, ok := _SystemTagSetNameToValueMap[s]; ok {
		return val, nil
	}
	return 0, fmt.Errorf("%s does not belong to SystemTagSet values", s)
}

// SystemTagSetValues returns all values of the enum
func SystemTagSetValues() []SystemTagSet {
	return _SystemTagSetValues
}

// IsASystemTagSet returns "true" if the value is listed in the enum definition. "false" otherwise
func (ts SystemTagSet) IsASystemTagSet() bool {
	_, ok := _SystemTagSetMap[ts]
	return ok
}
