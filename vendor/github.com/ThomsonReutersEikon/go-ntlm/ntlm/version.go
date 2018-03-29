//Copyright 2013 Thomson Reuters Global Resources. BSD License please see License file for more information

package ntlm

import (
	"bytes"
	"encoding/binary"
	"fmt"
)

type VersionStruct struct {
	ProductMajorVersion uint8
	ProductMinorVersion uint8
	ProductBuild        uint16
	Reserved            []byte
	NTLMRevisionCurrent uint8
}

func ReadVersionStruct(structSource []byte) (*VersionStruct, error) {
	versionStruct := new(VersionStruct)

	versionStruct.ProductMajorVersion = uint8(structSource[0])
	versionStruct.ProductMinorVersion = uint8(structSource[1])
	versionStruct.ProductBuild = binary.LittleEndian.Uint16(structSource[2:4])
	versionStruct.Reserved = structSource[4:7]
	versionStruct.NTLMRevisionCurrent = uint8(structSource[7])

	return versionStruct, nil
}

func (v *VersionStruct) String() string {
	return fmt.Sprintf("%d.%d.%d Ntlm %d", v.ProductMajorVersion, v.ProductMinorVersion, v.ProductBuild, v.NTLMRevisionCurrent)
}

func (v *VersionStruct) Bytes() []byte {
	dest := make([]byte, 0, 8)
	buffer := bytes.NewBuffer(dest)

	binary.Write(buffer, binary.LittleEndian, v.ProductMajorVersion)
	binary.Write(buffer, binary.LittleEndian, v.ProductMinorVersion)
	binary.Write(buffer, binary.LittleEndian, v.ProductBuild)
	buffer.Write(make([]byte, 3))
	binary.Write(buffer, binary.LittleEndian, uint8(v.NTLMRevisionCurrent))

	return buffer.Bytes()
}
