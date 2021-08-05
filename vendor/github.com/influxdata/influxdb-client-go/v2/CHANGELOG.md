## 2.5.1[2021-09-17]
### Bug fixes
 - [#276](https://github.com/influxdata/influxdb-client-go/pull/276) Synchronized logging methods of _log.Logger_.
 
### Features
## 2.5.0 [2021-08-20]
### Features
 - [#264](https://github.com/influxdata/influxdb-client-go/pull/264) Synced generated server API with the latest [oss.yml](https://github.com/influxdata/openapi/blob/master/contracts/oss.yml). 
 - [#271](https://github.com/influxdata/influxdb-client-go/pull/271) Use exponential _random_ retry strategy 
 - [#273](https://github.com/influxdata/influxdb-client-go/pull/273) Added `WriteFailedCallback` for `WriteAPI` allowing to be _synchronously_ notified about failed writes and decide on further batch processing. 

### Bug fixes
 - [#269](https://github.com/influxdata/influxdb-client-go/pull/269) Synchronized setters of _log.Logger_ to allow concurrent usage  
 - [#270](https://github.com/influxdata/influxdb-client-go/pull/270) Fixed duplicate `Content-Type` header in requests to managemet API  

### Documentation
 - [#261](https://github.com/influxdata/influxdb-client-go/pull/261) Update Line Protocol document link to v2.0
 - [#274](https://github.com/influxdata/influxdb-client-go/pull/274) Documenting proxy configuration and HTTP redirects handling

## 2.4.0 [2021-06-04]
### Features
 - [#256](https://github.com/influxdata/influxdb-client-go/pull/256) Allowing 'Doer' interface for HTTP requests

### Bug fixes
 - [#259](https://github.com/influxdata/influxdb-client-go/pull/259) Fixed leaking connection in case of not reading whole query result on TLS connection  


## 2.3.0 [2021-04-30]
### Breaking change
 - [#253](https://github.com/influxdata/influxdb-client-go/pull/253) Interface 'Logger' extended with 'LogLevel() uint' getter.

### Features
 - [#241](https://github.com/influxdata/influxdb-client-go/pull/241),[#248](https://github.com/influxdata/influxdb-client-go/pull/248) Synced with InfluxDB 2.0.5 swagger:
    - Setup (onboarding) now sends correctly retentionDuration if specified  
    - `RetentionRule` used in `Bucket` now contains `ShardGroupDurationSeconds` to specify the shard group duration.

### Documentation
1. [#242](https://github.com/influxdata/influxdb-client-go/pull/242) Documentation improvements:
 - [Custom server API example](https://pkg.go.dev/github.com/influxdata/influxdb-client-go/v2#example-Client-CustomServerAPICall) now shows how to create DBRP mapping
 - Improved documentation about concurrency
1. [#251](https://github.com/influxdata/influxdb-client-go/pull/251) Fixed Readme.md formatting
 
### Bug fixes
1. [#252](https://github.com/influxdata/influxdb-client-go/pull/252) Fixed panic when getting not present standard Flux columns  
1. [#253](https://github.com/influxdata/influxdb-client-go/pull/253) Conditional debug logging of buffers 
1. [#254](https://github.com/influxdata/influxdb-client-go/pull/254) Fixed golint issues

## 2.2.3 [2021-04-01]
### Bug fixes
1. [#236](https://github.com/influxdata/influxdb-client-go/pull/236) Setting MaxRetries to zero value disables retry strategy.
1. [#239](https://github.com/influxdata/influxdb-client-go/pull/239) Blocking write client doesn't use retry handling.  

## 2.2.2 [2021-01-29]
### Bug fixes
1. [#229](https://github.com/influxdata/influxdb-client-go/pull/229) Connection errors are also subject for retrying.

## 2.2.1 [2020-12-24]
### Bug fixes
1. [#220](https://github.com/influxdata/influxdb-client-go/pull/220) Fixed runtime error occurring when calling v2 API on v1 server.

### Documentation
1. [#218](https://github.com/influxdata/influxdb-client-go/pull/218), [#221](https://github.com/influxdata/influxdb-client-go/pull/221), [#222](https://github.com/influxdata/influxdb-client-go/pull/222), Changed links leading to sources to point to API docs in Readme, fixed broken links to InfluxDB docs.

## 2.2.0 [2020-10-30]
### Features
1. [#206](https://github.com/influxdata/influxdb-client-go/pull/206) Adding TasksAPI for managing tasks and associated logs and runs.

### Bug fixes
1. [#209](https://github.com/influxdata/influxdb-client-go/pull/209) Synchronizing access to the write service in WriteAPIBlocking.

## 2.1.0 [2020-10-02]
### Features
1. [#193](https://github.com/influxdata/influxdb-client-go/pull/193) Added authentication using username and password. See `UsersAPI.SignIn()` and `UsersAPI.SignOut()`
1. [#204](https://github.com/influxdata/influxdb-client-go/pull/204) Synced with InfluxDB 2 RC0 swagger. Added pagination to Organizations API and `After` paging param to Buckets API. 

### Bug fixes
1. [#191](https://github.com/influxdata/influxdb-client-go/pull/191) Fixed QueryTableResult.Next() failed to parse boolean datatype.
1. [#192](https://github.com/influxdata/influxdb-client-go/pull/192) Client.Close() closes idle connections of internally created HTTP client

### Documentation
1. [#189](https://github.com/influxdata/influxdb-client-go/pull/189) Added clarification that server URL has to be the InfluxDB server base URL to API docs and all examples.   
1. [#196](https://github.com/influxdata/influxdb-client-go/pull/196) Changed default server port 9999 to 8086 in docs and examples  
1. [#200](https://github.com/influxdata/influxdb-client-go/pull/200) Fix example code in the Readme

## 2.0.1 [2020-08-14]
### Bug fixes 
1. [#187](https://github.com/influxdata/influxdb-client-go/pull/187) Properly updated library for new major version.

## 2.0.0 [2020-08-14]
### Breaking changes
1. [#173](https://github.com/influxdata/influxdb-client-go/pull/173) Removed deprecated API.
1. [#174](https://github.com/influxdata/influxdb-client-go/pull/174) Removed orgs labels API cause [it has been removed from the server API](https://github.com/influxdata/influxdb/pull/19104)
1. [#175](https://github.com/influxdata/influxdb-client-go/pull/175) Removed WriteAPI.Close()

### Features
1. [#165](https://github.com/influxdata/influxdb-client-go/pull/165) Allow overriding the http.Client for the http service.
1. [#179](https://github.com/influxdata/influxdb-client-go/pull/179) Unifying retry strategy among InfluxDB 2 clients: added exponential backoff.
1. [#180](https://github.com/influxdata/influxdb-client-go/pull/180) Provided public logger API to enable overriding logging. It is also possible to disable logging. 
1. [#181](https://github.com/influxdata/influxdb-client-go/pull/181) Exposed HTTP service to allow custom server API calls. Added example. 

### Bug fixes 
1. [#175](https://github.com/influxdata/influxdb-client-go/pull/175) Fixed WriteAPIs management. Keeping single instance for each org and bucket pair.

### Documentation
1. [#185](https://github.com/influxdata/influxdb-client-go/pull/185) DeleteAPI and sample WriteAPIBlocking wrapper for implicit batching 

## 1.4.0 [2020-07-17]
### Breaking changes
1. [#156](https://github.com/influxdata/influxdb-client-go/pull/156) Fixing Go naming and code style violations: 
- Introducing new *API interfaces with proper name of types, methods and arguments. 
- This also affects the `Client` interface and the `Options` type. 
- Affected types and methods have been deprecated and they will be removed in the next release. 

### Bug fixes 
1. [#152](https://github.com/influxdata/influxdb-client-go/pull/152) Allow connecting to server on a URL path
1. [#154](https://github.com/influxdata/influxdb-client-go/pull/154) Use idiomatic go style for write channels (internal)
1. [#155](https://github.com/influxdata/influxdb-client-go/pull/155) Fix panic in FindOrganizationByName in case of no permissions


## 1.3.0 [2020-06-19]
### Features
1. [#131](https://github.com/influxdata/influxdb-client-go/pull/131) Labels API
1. [#136](https://github.com/influxdata/influxdb-client-go/pull/136) Possibility to specify default tags
1. [#138](https://github.com/influxdata/influxdb-client-go/pull/138) Fix errors from InfluxDB 1.8 being empty

### Bug fixes 
1. [#132](https://github.com/influxdata/influxdb-client-go/pull/132) Handle unsupported write type as string instead of generating panic
1. [#134](https://github.com/influxdata/influxdb-client-go/pull/134) FluxQueryResult: support reordering of annotations

## 1.2.0 [2020-05-15]
### Breaking Changes
 - [#107](https://github.com/influxdata/influxdb-client-go/pull/107) Renamed `InfluxDBClient` interface to `Client`, so the full name `influxdb2.Client` suits better to Go naming conventions
 - [#125](https://github.com/influxdata/influxdb-client-go/pull/125) `WriteApi`,`WriteApiBlocking`,`QueryApi` interfaces and related objects like `Point`, `FluxTableMetadata`, `FluxTableColumn`, `FluxRecord`, moved to the `api` ( and `api/write`, `api/query`) packages
 to provide consistent interface 
 
### Features
1. [#120](https://github.com/influxdata/influxdb-client-go/pull/120) Health check API   
1. [#122](https://github.com/influxdata/influxdb-client-go/pull/122) Delete API
1. [#124](https://github.com/influxdata/influxdb-client-go/pull/124) Buckets API

### Bug fixes 
1. [#108](https://github.com/influxdata/influxdb-client-go/issues/108) Fix default retry interval doc
1. [#110](https://github.com/influxdata/influxdb-client-go/issues/110) Allowing empty (nil) values in query result

### Documentation
 - [#112](https://github.com/influxdata/influxdb-client-go/pull/112) Clarify how to use client with InfluxDB 1.8+
 - [#115](https://github.com/influxdata/influxdb-client-go/pull/115) Doc and examples for reading write api errors 

## 1.1.0 [2020-04-24]
### Features
1. [#100](https://github.com/influxdata/influxdb-client-go/pull/100)  HTTP request timeout made configurable
1. [#99](https://github.com/influxdata/influxdb-client-go/pull/99)  Organizations API and Users API
1. [#96](https://github.com/influxdata/influxdb-client-go/pull/96)  Authorization API

### Docs
1. [#101](https://github.com/influxdata/influxdb-client-go/pull/101) Added examples to API docs

## 1.0.0 [2020-04-01]
### Core

- initial release of new client version

### APIs

- initial release of new client version
