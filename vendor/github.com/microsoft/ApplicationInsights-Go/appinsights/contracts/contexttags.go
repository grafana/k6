package contracts

// NOTE: This file was automatically generated.

type ContextTags map[string]string

// Helper type that provides access to context fields grouped under 'application'.
// This is returned by TelemetryContext.Tags.Application()
type ApplicationContextTags ContextTags

// Helper type that provides access to context fields grouped under 'device'.
// This is returned by TelemetryContext.Tags.Device()
type DeviceContextTags ContextTags

// Helper type that provides access to context fields grouped under 'location'.
// This is returned by TelemetryContext.Tags.Location()
type LocationContextTags ContextTags

// Helper type that provides access to context fields grouped under 'operation'.
// This is returned by TelemetryContext.Tags.Operation()
type OperationContextTags ContextTags

// Helper type that provides access to context fields grouped under 'session'.
// This is returned by TelemetryContext.Tags.Session()
type SessionContextTags ContextTags

// Helper type that provides access to context fields grouped under 'user'.
// This is returned by TelemetryContext.Tags.User()
type UserContextTags ContextTags

// Helper type that provides access to context fields grouped under 'cloud'.
// This is returned by TelemetryContext.Tags.Cloud()
type CloudContextTags ContextTags

// Helper type that provides access to context fields grouped under 'internal'.
// This is returned by TelemetryContext.Tags.Internal()
type InternalContextTags ContextTags

// Returns a helper to access context fields grouped under 'application'.
func (tags ContextTags) Application() ApplicationContextTags {
	return ApplicationContextTags(tags)
}

// Returns a helper to access context fields grouped under 'device'.
func (tags ContextTags) Device() DeviceContextTags {
	return DeviceContextTags(tags)
}

// Returns a helper to access context fields grouped under 'location'.
func (tags ContextTags) Location() LocationContextTags {
	return LocationContextTags(tags)
}

// Returns a helper to access context fields grouped under 'operation'.
func (tags ContextTags) Operation() OperationContextTags {
	return OperationContextTags(tags)
}

// Returns a helper to access context fields grouped under 'session'.
func (tags ContextTags) Session() SessionContextTags {
	return SessionContextTags(tags)
}

// Returns a helper to access context fields grouped under 'user'.
func (tags ContextTags) User() UserContextTags {
	return UserContextTags(tags)
}

// Returns a helper to access context fields grouped under 'cloud'.
func (tags ContextTags) Cloud() CloudContextTags {
	return CloudContextTags(tags)
}

// Returns a helper to access context fields grouped under 'internal'.
func (tags ContextTags) Internal() InternalContextTags {
	return InternalContextTags(tags)
}

// Application version. Information in the application context fields is
// always about the application that is sending the telemetry.
func (tags ApplicationContextTags) GetVer() string {
	if result, ok := tags["ai.application.ver"]; ok {
		return result
	}

	return ""
}

// Application version. Information in the application context fields is
// always about the application that is sending the telemetry.
func (tags ApplicationContextTags) SetVer(value string) {
	if value != "" {
		tags["ai.application.ver"] = value
	} else {
		delete(tags, "ai.application.ver")
	}
}

// Unique client device id. Computer name in most cases.
func (tags DeviceContextTags) GetId() string {
	if result, ok := tags["ai.device.id"]; ok {
		return result
	}

	return ""
}

// Unique client device id. Computer name in most cases.
func (tags DeviceContextTags) SetId(value string) {
	if value != "" {
		tags["ai.device.id"] = value
	} else {
		delete(tags, "ai.device.id")
	}
}

// Device locale using <language>-<REGION> pattern, following RFC 5646.
// Example 'en-US'.
func (tags DeviceContextTags) GetLocale() string {
	if result, ok := tags["ai.device.locale"]; ok {
		return result
	}

	return ""
}

// Device locale using <language>-<REGION> pattern, following RFC 5646.
// Example 'en-US'.
func (tags DeviceContextTags) SetLocale(value string) {
	if value != "" {
		tags["ai.device.locale"] = value
	} else {
		delete(tags, "ai.device.locale")
	}
}

// Model of the device the end user of the application is using. Used for
// client scenarios. If this field is empty then it is derived from the user
// agent.
func (tags DeviceContextTags) GetModel() string {
	if result, ok := tags["ai.device.model"]; ok {
		return result
	}

	return ""
}

// Model of the device the end user of the application is using. Used for
// client scenarios. If this field is empty then it is derived from the user
// agent.
func (tags DeviceContextTags) SetModel(value string) {
	if value != "" {
		tags["ai.device.model"] = value
	} else {
		delete(tags, "ai.device.model")
	}
}

// Client device OEM name taken from the browser.
func (tags DeviceContextTags) GetOemName() string {
	if result, ok := tags["ai.device.oemName"]; ok {
		return result
	}

	return ""
}

// Client device OEM name taken from the browser.
func (tags DeviceContextTags) SetOemName(value string) {
	if value != "" {
		tags["ai.device.oemName"] = value
	} else {
		delete(tags, "ai.device.oemName")
	}
}

// Operating system name and version of the device the end user of the
// application is using. If this field is empty then it is derived from the
// user agent. Example 'Windows 10 Pro 10.0.10586.0'
func (tags DeviceContextTags) GetOsVersion() string {
	if result, ok := tags["ai.device.osVersion"]; ok {
		return result
	}

	return ""
}

// Operating system name and version of the device the end user of the
// application is using. If this field is empty then it is derived from the
// user agent. Example 'Windows 10 Pro 10.0.10586.0'
func (tags DeviceContextTags) SetOsVersion(value string) {
	if value != "" {
		tags["ai.device.osVersion"] = value
	} else {
		delete(tags, "ai.device.osVersion")
	}
}

// The type of the device the end user of the application is using. Used
// primarily to distinguish JavaScript telemetry from server side telemetry.
// Examples: 'PC', 'Phone', 'Browser'. 'PC' is the default value.
func (tags DeviceContextTags) GetType() string {
	if result, ok := tags["ai.device.type"]; ok {
		return result
	}

	return ""
}

// The type of the device the end user of the application is using. Used
// primarily to distinguish JavaScript telemetry from server side telemetry.
// Examples: 'PC', 'Phone', 'Browser'. 'PC' is the default value.
func (tags DeviceContextTags) SetType(value string) {
	if value != "" {
		tags["ai.device.type"] = value
	} else {
		delete(tags, "ai.device.type")
	}
}

// The IP address of the client device. IPv4 and IPv6 are supported.
// Information in the location context fields is always about the end user.
// When telemetry is sent from a service, the location context is about the
// user that initiated the operation in the service.
func (tags LocationContextTags) GetIp() string {
	if result, ok := tags["ai.location.ip"]; ok {
		return result
	}

	return ""
}

// The IP address of the client device. IPv4 and IPv6 are supported.
// Information in the location context fields is always about the end user.
// When telemetry is sent from a service, the location context is about the
// user that initiated the operation in the service.
func (tags LocationContextTags) SetIp(value string) {
	if value != "" {
		tags["ai.location.ip"] = value
	} else {
		delete(tags, "ai.location.ip")
	}
}

// A unique identifier for the operation instance. The operation.id is created
// by either a request or a page view. All other telemetry sets this to the
// value for the containing request or page view. Operation.id is used for
// finding all the telemetry items for a specific operation instance.
func (tags OperationContextTags) GetId() string {
	if result, ok := tags["ai.operation.id"]; ok {
		return result
	}

	return ""
}

// A unique identifier for the operation instance. The operation.id is created
// by either a request or a page view. All other telemetry sets this to the
// value for the containing request or page view. Operation.id is used for
// finding all the telemetry items for a specific operation instance.
func (tags OperationContextTags) SetId(value string) {
	if value != "" {
		tags["ai.operation.id"] = value
	} else {
		delete(tags, "ai.operation.id")
	}
}

// The name (group) of the operation. The operation.name is created by either
// a request or a page view. All other telemetry items set this to the value
// for the containing request or page view. Operation.name is used for finding
// all the telemetry items for a group of operations (i.e. 'GET Home/Index').
func (tags OperationContextTags) GetName() string {
	if result, ok := tags["ai.operation.name"]; ok {
		return result
	}

	return ""
}

// The name (group) of the operation. The operation.name is created by either
// a request or a page view. All other telemetry items set this to the value
// for the containing request or page view. Operation.name is used for finding
// all the telemetry items for a group of operations (i.e. 'GET Home/Index').
func (tags OperationContextTags) SetName(value string) {
	if value != "" {
		tags["ai.operation.name"] = value
	} else {
		delete(tags, "ai.operation.name")
	}
}

// The unique identifier of the telemetry item's immediate parent.
func (tags OperationContextTags) GetParentId() string {
	if result, ok := tags["ai.operation.parentId"]; ok {
		return result
	}

	return ""
}

// The unique identifier of the telemetry item's immediate parent.
func (tags OperationContextTags) SetParentId(value string) {
	if value != "" {
		tags["ai.operation.parentId"] = value
	} else {
		delete(tags, "ai.operation.parentId")
	}
}

// Name of synthetic source. Some telemetry from the application may represent
// a synthetic traffic. It may be web crawler indexing the web site, site
// availability tests or traces from diagnostic libraries like Application
// Insights SDK itself.
func (tags OperationContextTags) GetSyntheticSource() string {
	if result, ok := tags["ai.operation.syntheticSource"]; ok {
		return result
	}

	return ""
}

// Name of synthetic source. Some telemetry from the application may represent
// a synthetic traffic. It may be web crawler indexing the web site, site
// availability tests or traces from diagnostic libraries like Application
// Insights SDK itself.
func (tags OperationContextTags) SetSyntheticSource(value string) {
	if value != "" {
		tags["ai.operation.syntheticSource"] = value
	} else {
		delete(tags, "ai.operation.syntheticSource")
	}
}

// The correlation vector is a light weight vector clock which can be used to
// identify and order related events across clients and services.
func (tags OperationContextTags) GetCorrelationVector() string {
	if result, ok := tags["ai.operation.correlationVector"]; ok {
		return result
	}

	return ""
}

// The correlation vector is a light weight vector clock which can be used to
// identify and order related events across clients and services.
func (tags OperationContextTags) SetCorrelationVector(value string) {
	if value != "" {
		tags["ai.operation.correlationVector"] = value
	} else {
		delete(tags, "ai.operation.correlationVector")
	}
}

// Session ID - the instance of the user's interaction with the app.
// Information in the session context fields is always about the end user.
// When telemetry is sent from a service, the session context is about the
// user that initiated the operation in the service.
func (tags SessionContextTags) GetId() string {
	if result, ok := tags["ai.session.id"]; ok {
		return result
	}

	return ""
}

// Session ID - the instance of the user's interaction with the app.
// Information in the session context fields is always about the end user.
// When telemetry is sent from a service, the session context is about the
// user that initiated the operation in the service.
func (tags SessionContextTags) SetId(value string) {
	if value != "" {
		tags["ai.session.id"] = value
	} else {
		delete(tags, "ai.session.id")
	}
}

// Boolean value indicating whether the session identified by ai.session.id is
// first for the user or not.
func (tags SessionContextTags) GetIsFirst() string {
	if result, ok := tags["ai.session.isFirst"]; ok {
		return result
	}

	return ""
}

// Boolean value indicating whether the session identified by ai.session.id is
// first for the user or not.
func (tags SessionContextTags) SetIsFirst(value string) {
	if value != "" {
		tags["ai.session.isFirst"] = value
	} else {
		delete(tags, "ai.session.isFirst")
	}
}

// In multi-tenant applications this is the account ID or name which the user
// is acting with. Examples may be subscription ID for Azure portal or blog
// name blogging platform.
func (tags UserContextTags) GetAccountId() string {
	if result, ok := tags["ai.user.accountId"]; ok {
		return result
	}

	return ""
}

// In multi-tenant applications this is the account ID or name which the user
// is acting with. Examples may be subscription ID for Azure portal or blog
// name blogging platform.
func (tags UserContextTags) SetAccountId(value string) {
	if value != "" {
		tags["ai.user.accountId"] = value
	} else {
		delete(tags, "ai.user.accountId")
	}
}

// Anonymous user id. Represents the end user of the application. When
// telemetry is sent from a service, the user context is about the user that
// initiated the operation in the service.
func (tags UserContextTags) GetId() string {
	if result, ok := tags["ai.user.id"]; ok {
		return result
	}

	return ""
}

// Anonymous user id. Represents the end user of the application. When
// telemetry is sent from a service, the user context is about the user that
// initiated the operation in the service.
func (tags UserContextTags) SetId(value string) {
	if value != "" {
		tags["ai.user.id"] = value
	} else {
		delete(tags, "ai.user.id")
	}
}

// Authenticated user id. The opposite of ai.user.id, this represents the user
// with a friendly name. Since it's PII information it is not collected by
// default by most SDKs.
func (tags UserContextTags) GetAuthUserId() string {
	if result, ok := tags["ai.user.authUserId"]; ok {
		return result
	}

	return ""
}

// Authenticated user id. The opposite of ai.user.id, this represents the user
// with a friendly name. Since it's PII information it is not collected by
// default by most SDKs.
func (tags UserContextTags) SetAuthUserId(value string) {
	if value != "" {
		tags["ai.user.authUserId"] = value
	} else {
		delete(tags, "ai.user.authUserId")
	}
}

// Name of the role the application is a part of. Maps directly to the role
// name in azure.
func (tags CloudContextTags) GetRole() string {
	if result, ok := tags["ai.cloud.role"]; ok {
		return result
	}

	return ""
}

// Name of the role the application is a part of. Maps directly to the role
// name in azure.
func (tags CloudContextTags) SetRole(value string) {
	if value != "" {
		tags["ai.cloud.role"] = value
	} else {
		delete(tags, "ai.cloud.role")
	}
}

// Name of the instance where the application is running. Computer name for
// on-premisis, instance name for Azure.
func (tags CloudContextTags) GetRoleInstance() string {
	if result, ok := tags["ai.cloud.roleInstance"]; ok {
		return result
	}

	return ""
}

// Name of the instance where the application is running. Computer name for
// on-premisis, instance name for Azure.
func (tags CloudContextTags) SetRoleInstance(value string) {
	if value != "" {
		tags["ai.cloud.roleInstance"] = value
	} else {
		delete(tags, "ai.cloud.roleInstance")
	}
}

// SDK version. See
// https://github.com/microsoft/ApplicationInsights-Home/blob/master/SDK-AUTHORING.md#sdk-version-specification
// for information.
func (tags InternalContextTags) GetSdkVersion() string {
	if result, ok := tags["ai.internal.sdkVersion"]; ok {
		return result
	}

	return ""
}

// SDK version. See
// https://github.com/microsoft/ApplicationInsights-Home/blob/master/SDK-AUTHORING.md#sdk-version-specification
// for information.
func (tags InternalContextTags) SetSdkVersion(value string) {
	if value != "" {
		tags["ai.internal.sdkVersion"] = value
	} else {
		delete(tags, "ai.internal.sdkVersion")
	}
}

// Agent version. Used to indicate the version of StatusMonitor installed on
// the computer if it is used for data collection.
func (tags InternalContextTags) GetAgentVersion() string {
	if result, ok := tags["ai.internal.agentVersion"]; ok {
		return result
	}

	return ""
}

// Agent version. Used to indicate the version of StatusMonitor installed on
// the computer if it is used for data collection.
func (tags InternalContextTags) SetAgentVersion(value string) {
	if value != "" {
		tags["ai.internal.agentVersion"] = value
	} else {
		delete(tags, "ai.internal.agentVersion")
	}
}

// This is the node name used for billing purposes. Use it to override the
// standard detection of nodes.
func (tags InternalContextTags) GetNodeName() string {
	if result, ok := tags["ai.internal.nodeName"]; ok {
		return result
	}

	return ""
}

// This is the node name used for billing purposes. Use it to override the
// standard detection of nodes.
func (tags InternalContextTags) SetNodeName(value string) {
	if value != "" {
		tags["ai.internal.nodeName"] = value
	} else {
		delete(tags, "ai.internal.nodeName")
	}
}
