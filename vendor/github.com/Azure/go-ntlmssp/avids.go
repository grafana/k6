// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package ntlmssp

type avID uint16

const (
	avIDMsvAvEOL avID = iota
	avIDMsvAvNbComputerName
	avIDMsvAvNbDomainName
	avIDMsvAvDNSComputerName
	avIDMsvAvDNSDomainName
	avIDMsvAvDNSTreeName
	avIDMsvAvFlags
	avIDMsvAvTimestamp
	avIDMsvAvSingleHost
	avIDMsvAvTargetName
	avIDMsvChannelBindings
)
