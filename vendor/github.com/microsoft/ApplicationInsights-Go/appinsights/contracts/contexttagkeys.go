package contracts

// NOTE: This file was automatically generated.

import "strconv"

const (
	// Application version. Information in the application context fields is
	// always about the application that is sending the telemetry.
	ApplicationVersion string = "ai.application.ver"

	// Unique client device id. Computer name in most cases.
	DeviceId string = "ai.device.id"

	// Device locale using <language>-<REGION> pattern, following RFC 5646.
	// Example 'en-US'.
	DeviceLocale string = "ai.device.locale"

	// Model of the device the end user of the application is using. Used for
	// client scenarios. If this field is empty then it is derived from the user
	// agent.
	DeviceModel string = "ai.device.model"

	// Client device OEM name taken from the browser.
	DeviceOEMName string = "ai.device.oemName"

	// Operating system name and version of the device the end user of the
	// application is using. If this field is empty then it is derived from the
	// user agent. Example 'Windows 10 Pro 10.0.10586.0'
	DeviceOSVersion string = "ai.device.osVersion"

	// The type of the device the end user of the application is using. Used
	// primarily to distinguish JavaScript telemetry from server side telemetry.
	// Examples: 'PC', 'Phone', 'Browser'. 'PC' is the default value.
	DeviceType string = "ai.device.type"

	// The IP address of the client device. IPv4 and IPv6 are supported.
	// Information in the location context fields is always about the end user.
	// When telemetry is sent from a service, the location context is about the
	// user that initiated the operation in the service.
	LocationIp string = "ai.location.ip"

	// A unique identifier for the operation instance. The operation.id is created
	// by either a request or a page view. All other telemetry sets this to the
	// value for the containing request or page view. Operation.id is used for
	// finding all the telemetry items for a specific operation instance.
	OperationId string = "ai.operation.id"

	// The name (group) of the operation. The operation.name is created by either
	// a request or a page view. All other telemetry items set this to the value
	// for the containing request or page view. Operation.name is used for finding
	// all the telemetry items for a group of operations (i.e. 'GET Home/Index').
	OperationName string = "ai.operation.name"

	// The unique identifier of the telemetry item's immediate parent.
	OperationParentId string = "ai.operation.parentId"

	// Name of synthetic source. Some telemetry from the application may represent
	// a synthetic traffic. It may be web crawler indexing the web site, site
	// availability tests or traces from diagnostic libraries like Application
	// Insights SDK itself.
	OperationSyntheticSource string = "ai.operation.syntheticSource"

	// The correlation vector is a light weight vector clock which can be used to
	// identify and order related events across clients and services.
	OperationCorrelationVector string = "ai.operation.correlationVector"

	// Session ID - the instance of the user's interaction with the app.
	// Information in the session context fields is always about the end user.
	// When telemetry is sent from a service, the session context is about the
	// user that initiated the operation in the service.
	SessionId string = "ai.session.id"

	// Boolean value indicating whether the session identified by ai.session.id is
	// first for the user or not.
	SessionIsFirst string = "ai.session.isFirst"

	// In multi-tenant applications this is the account ID or name which the user
	// is acting with. Examples may be subscription ID for Azure portal or blog
	// name blogging platform.
	UserAccountId string = "ai.user.accountId"

	// Anonymous user id. Represents the end user of the application. When
	// telemetry is sent from a service, the user context is about the user that
	// initiated the operation in the service.
	UserId string = "ai.user.id"

	// Authenticated user id. The opposite of ai.user.id, this represents the user
	// with a friendly name. Since it's PII information it is not collected by
	// default by most SDKs.
	UserAuthUserId string = "ai.user.authUserId"

	// Name of the role the application is a part of. Maps directly to the role
	// name in azure.
	CloudRole string = "ai.cloud.role"

	// Name of the instance where the application is running. Computer name for
	// on-premisis, instance name for Azure.
	CloudRoleInstance string = "ai.cloud.roleInstance"

	// SDK version. See
	// https://github.com/microsoft/ApplicationInsights-Home/blob/master/SDK-AUTHORING.md#sdk-version-specification
	// for information.
	InternalSdkVersion string = "ai.internal.sdkVersion"

	// Agent version. Used to indicate the version of StatusMonitor installed on
	// the computer if it is used for data collection.
	InternalAgentVersion string = "ai.internal.agentVersion"

	// This is the node name used for billing purposes. Use it to override the
	// standard detection of nodes.
	InternalNodeName string = "ai.internal.nodeName"
)

var tagMaxLengths = map[string]int{
	"ai.application.ver":             1024,
	"ai.device.id":                   1024,
	"ai.device.locale":               64,
	"ai.device.model":                256,
	"ai.device.oemName":              256,
	"ai.device.osVersion":            256,
	"ai.device.type":                 64,
	"ai.location.ip":                 46,
	"ai.operation.id":                128,
	"ai.operation.name":              1024,
	"ai.operation.parentId":          128,
	"ai.operation.syntheticSource":   1024,
	"ai.operation.correlationVector": 64,
	"ai.session.id":                  64,
	"ai.session.isFirst":             5,
	"ai.user.accountId":              1024,
	"ai.user.id":                     128,
	"ai.user.authUserId":             1024,
	"ai.cloud.role":                  256,
	"ai.cloud.roleInstance":          256,
	"ai.internal.sdkVersion":         64,
	"ai.internal.agentVersion":       64,
	"ai.internal.nodeName":           256,
}

// Truncates tag values that exceed their maximum supported lengths.  Returns
// warnings for each affected field.
func SanitizeTags(tags map[string]string) []string {
	var warnings []string
	for k, v := range tags {
		if maxlen, ok := tagMaxLengths[k]; ok && len(v) > maxlen {
			tags[k] = v[:maxlen]
			warnings = append(warnings, "Value for "+k+" exceeded maximum length of "+strconv.Itoa(maxlen))
		}
	}

	return warnings
}
