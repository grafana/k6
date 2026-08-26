/**
 * k6 extension for Prometheus Remote Write load testing.
 * 
 * This module provides the ability to send metrics to any Prometheus Remote Write compatible endpoint,
 * including Prometheus, Cortex, Thanos, Mimir, and other services that implement the remote_write API.
 * 
 * ## Features
 * 
 * - Send custom time series data to remote write endpoints
 * - Generate metrics efficiently using templates to reduce JavaScript overhead
 * - Support for custom headers, authentication, and multi-tenant configurations
 * - Built-in support for high-cardinality testing scenarios
 * 
 * ## Basic Usage
 * 
 * ```javascript
 * import remote from 'k6/x/remotewrite';
 * 
 * const client = new remote.Client({
 *     url: "https://prometheus.example.com/api/v1/write"
 * });
 * 
 * export default function () {
 *     let res = client.store([{
 *         labels: [
 *             { name: "__name__", value: "my_metric" },
 *             { name: "service", value: "api" }
 *         ],
 *         samples: [
 *             { value: 42.5 }
 *         ]
 *     }]);
 * }
 * ```
 * 
 * @module k6/x/remotewrite
 */

/**
 * Configuration options for the Remote Write client.
 * 
 * @example
 * ```javascript
 * const config = {
 *     url: "https://user:pass@prometheus.example.com/api/v1/write",
 *     user_agent: "my-k6-test",
 *     tenant_name: "team-a",
 *     timeout: "30s",
 *     headers: {
 *         "X-Custom-Header": "value"
 *     }
 * };
 * ```
 */
export interface ClientConfig {
    /**
     * The URL of the Prometheus remote write endpoint.
     * Can include basic auth credentials: https://user:password@host/path
     */
    url: string;

    /**
     * Optional custom User-Agent header value.
     * Default is "k6-remote-write/0.0.2".
     */
    user_agent?: string;

    /**
     * Optional tenant name for multi-tenant environments.
     * Typically sent as X-Scope-OrgID header.
     */
    tenant_name?: string;

    /**
     * Optional request timeout (e.g., "20s", "5m").
     * Default is "10s".
     */
    timeout?: string;

    /**
     * Optional custom headers to send with requests.
     */
    headers?: Record<string, string>;
}

/**
 * A Prometheus label with name and value.
 * 
 * Labels are key-value pairs that uniquely identify a time series.
 * The special label `__name__` is used for the metric name.
 * 
 * @example
 * ```javascript
 * { name: "__name__", value: "http_requests_total" }
 * { name: "method", value: "GET" }
 * { name: "status", value: "200" }
 * ```
 */
export interface Label {
    /**
     * The label name. Use "__name__" for the metric name.
     */
    name: string;

    /**
     * The label value.
     */
    value: string;
}

/**
 * A sample (data point) with value and optional timestamp.
 * 
 * Each sample represents a single measurement at a specific point in time.
 * If no timestamp is provided, the current time will be used automatically.
 * 
 * @example
 * ```javascript
 * { value: 42.5 }
 * { value: 100, timestamp: Date.now() }
 * { value: Math.random() * 100, timestamp: 1643235433000 }
 * ```
 */
export interface Sample {
    /**
     * The numeric value of the sample.
     */
    value: number;

    /**
     * Optional timestamp in milliseconds.
     * If not provided, the current time is used.
     */
    timestamp?: number;
}

/**
 * A time series with labels and samples.
 * 
 * A time series is identified by a unique set of labels and contains one or more sample data points.
 * Each time series must include a `__name__` label to specify the metric name.
 * 
 * @example
 * ```javascript
 * {
 *     labels: [
 *         { name: "__name__", value: "http_requests_total" },
 *         { name: "method", value: "POST" },
 *         { name: "status", value: "200" }
 *     ],
 *     samples: [
 *         { value: 42, timestamp: Date.now() }
 *     ]
 * }
 * ```
 */
export interface TimeSeries {
    /**
     * Array of labels that identify this time series.
     * Must include a label with name "__name__" for the metric name.
     */
    labels: Label[];

    /**
     * Array of sample data points for this time series.
     */
    samples: Sample[];
}

/**
 * Template for generating metrics automatically with variable substitution.
 * 
 * Templates allow efficient generation of many time series without JavaScript overhead.
 * Use template variables to create dynamic label values based on series ID.
 * 
 * ## Template Variables
 * 
 * - `${series_id}` - The current series ID
 * - `${series_id/N}` - Series ID divided by N (integer division) - useful for creating shared label values
 * - `${series_id%N}` - Series ID modulo N - useful for creating cyclic patterns
 * 
 * ## Use Cases
 * 
 * - **High cardinality testing**: Generate thousands of unique time series
 * - **Label distribution patterns**: Control how labels are distributed across series
 * - **Performance optimization**: Generate metrics in Go rather than JavaScript
 * 
 * @example Basic template
 * ```javascript
 * const template = {
 *     __name__: 'my_metric',
 *     series_id: '${series_id}',
 *     instance: 'host-${series_id}'
 * };
 * ```
 * 
 * @example Cardinality control
 * ```javascript
 * const template = {
 *     __name__: 'k6_generated_metric_${series_id/4}',    // Every 4 series share same metric name
 *     series_id: '${series_id}',                         // Unique per series
 *     cardinality_1e1: '${series_id/10}',                // Every 10 series share this label value
 *     cardinality_2: '${series_id%2}',                   // Alternates between 0 and 1
 * };
 * ```
 */
export interface MetricTemplate {
    /**
     * The metric name template. Use "__name__" as the key.
     */
    __name__: string;

    /**
     * Additional label templates.
     * Each label can use template variables like ${series_id}, ${series_id/N}, ${series_id%N}
     */
    [labelName: string]: string;
}

/**
 * Precompiled label templates for efficient metric generation.
 * 
 * This is an opaque type returned by {@link precompileLabelTemplates}.
 * Precompiling templates once and reusing them across multiple calls provides
 * better performance than compiling templates on every invocation.
 * 
 * @see {@link precompileLabelTemplates}
 * @see {@link Client.storeFromPrecompiledTemplates}
 */
export interface PrecompiledLabelTemplates {
    /** @internal */
    readonly _opaque: unique symbol;
}

/**
 * Response from a remote write operation.
 * 
 * This extends the standard k6 HTTP response with information about the remote write request.
 * Use the `status` field to check if the write was successful (typically 200 or 204).
 * 
 * @example Check response status
 * ```javascript
 * import { check } from 'k6';
 * 
 * const res = client.store([...]);
 * check(res, {
 *     'is status 200': (r) => r.status === 200,
 * });
 * ```
 */
export interface RemoteWriteResponse {
    /**
     * HTTP status code of the response.
     */
    status: number;

    /**
     * Response body (if any).
     */
    body?: string;

    /**
     * Whether the request was successful (status 2xx).
     */
    error?: string;

    /**
     * Response headers.
     */
    headers?: Record<string, string>;
}

/**
 * Client for sending metrics to Prometheus Remote Write endpoints.
 * 
 * The Client provides multiple methods for sending metrics, optimized for different use cases:
 * - {@link store} - Send manually constructed time series (most flexible)
 * - {@link storeFromTemplates} - Generate and send metrics using templates (more efficient)
 * - {@link storeFromPrecompiledTemplates} - Use precompiled templates (most efficient for repeated calls)
 * - {@link storeGenerated} - Generate metrics with automatic cardinality labels
 * 
 * @example Basic usage
 * ```javascript
 * import remote from 'k6/x/remotewrite';
 * 
 * const client = new remote.Client({
 *     url: "https://prometheus.example.com/api/v1/write"
 * });
 * 
 * export default function () {
 *     client.store([{
 *         labels: [
 *             { name: "__name__", value: "my_metric" },
 *             { name: "environment", value: "prod" }
 *         ],
 *         samples: [{ value: 42 }]
 *     }]);
 * }
 * ```
 * 
 * @example With authentication
 * ```javascript
 * const client = new remote.Client({
 *     url: "https://user:password@prometheus.example.com/api/v1/write",
 *     tenant_name: "team-a",
 *     timeout: "30s"
 * });
 * ```
 */
export class Client {
    /**
     * Creates a new Remote Write client.
     * 
     * @param config - Configuration for the client
     * 
     * @example
     * ```javascript
     * import remote from 'k6/x/remotewrite';
     * 
     * const client = new remote.Client({
     *     url: "https://prometheus.example.com/api/v1/write"
     * });
     * ```
     */
    constructor(config: ClientConfig);

    /**
     * Stores (sends) time series data to the remote write endpoint.
     * 
     * @param timeSeries - Array of time series to send
     * @returns Response from the remote write endpoint
     * 
     * @example
     * ```javascript
     * const res = client.store([{
     *     labels: [
     *         { name: "__name__", value: "my_metric" },
     *         { name: "service", value: "api" }
     *     ],
     *     samples: [
     *         { value: 42, timestamp: Date.now() }
     *     ]
     * }]);
     * ```
     */
    store(timeSeries: TimeSeries[]): RemoteWriteResponse;

    /**
     * Generates and stores time series data from a template.
     * 
     * This method is more efficient than {@link store} for generating large numbers of metrics
     * because the samples are generated inside the Go extension, avoiding the overhead of
     * passing objects from JavaScript to Go.
     * 
     * The template string values can include special variables that are substituted based
     * on the series ID. This allows you to control label cardinality and distribution patterns.
     * 
     * @param minValue - Minimum random value for samples
     * @param maxValue - Maximum random value for samples (exclusive)
     * @param timestamp - Timestamp in milliseconds
     * @param seriesIdStart - Start of series ID range (inclusive)
     * @param seriesIdEnd - End of series ID range (exclusive)
     * @param template - Template for generating metric labels
     * @returns Response from the remote write endpoint
     * 
     * @example Generate 100 series with controlled cardinality
     * ```javascript
     * const template = {
     *     __name__: 'k6_generated_metric_${series_id/4}',
     *     series_id: '${series_id}',
     *     cardinality_1e1: '${series_id/10}',
     *     cardinality_2: '${series_id%2}',
     * };
     * 
     * client.storeFromTemplates(
     *     100,                // min random value
     *     200,                // max random value
     *     Date.now(),         // timestamp in ms
     *     0,                  // series id start
     *     100,                // series id end (exclusive) - generates IDs 0-99
     *     template
     * );
     * ```
     * 
     * @see {@link MetricTemplate} for template variable syntax
     */
    storeFromTemplates(
        minValue: number,
        maxValue: number,
        timestamp: number,
        seriesIdStart: number,
        seriesIdEnd: number,
        template: MetricTemplate
    ): RemoteWriteResponse;

    /**
     * Stores metrics using precompiled templates for maximum performance.
     * 
     * Use {@link precompileLabelTemplates} to compile templates once, then reuse them
     * across multiple calls to this method. This is the most efficient approach when
     * generating metrics repeatedly with the same template structure.
     * 
     * @param minValue - Minimum random value for samples
     * @param maxValue - Maximum random value for samples (exclusive)
     * @param timestamp - Timestamp in milliseconds
     * @param seriesIdStart - Start of series ID range (inclusive)
     * @param seriesIdEnd - End of series ID range (exclusive)
     * @param template - Precompiled label templates from {@link precompileLabelTemplates}
     * @returns Response from the remote write endpoint
     * 
     * @example
     * ```javascript
     * const template = {
     *     __name__: 'k6_metric_${series_id/10}',
     *     series_id: '${series_id}'
     * };
     * const compiled = remote.precompileLabelTemplates(template);
     * 
     * // Reuse compiled template in each iteration
     * export default function() {
     *     client.storeFromPrecompiledTemplates(100, 200, Date.now(), 0, 50, compiled);
     * }
     * ```
     * 
     * @see {@link precompileLabelTemplates}
     * @see {@link storeFromTemplates} for a simpler API that compiles templates internally
     */
    storeFromPrecompiledTemplates(
        minValue: number,
        maxValue: number,
        timestamp: number,
        seriesIdStart: number,
        seriesIdEnd: number,
        template: PrecompiledLabelTemplates
    ): RemoteWriteResponse;

    /**
     * Generates and stores time series data with automatic cardinality labels.
     * 
     * This method automatically generates time series with cardinality labels
     * (cardinality_1e1, cardinality_1e2, etc.) based on the total number of series.
     * It's useful for testing scenarios where you need to distribute series across
     * multiple batches or simulate realistic label distributions.
     * 
     * Each series will have:
     * - A metric name like `k6_generated_metric_{series_id}`
     * - A `series_id` label with the unique series ID
     * - Automatic cardinality labels based on powers of 10
     * 
     * @param totalSeries - Total number of series to generate across all batches
     * @param batches - Total number of batches
     * @param batchSize - Number of series per batch (must divide evenly: totalSeries = batches × batchSize)
     * @param batch - Current batch number (1-indexed)
     * @returns Response from the remote write endpoint
     * 
     * @example Batch processing
     * ```javascript
     * // Generate 1000 total series, split into 10 batches of 100 each
     * export default function() {
     *     const batchNumber = (__ITER % 10) + 1;  // Cycles through batches 1-10
     *     client.storeGenerated(
     *         1000,  // total series
     *         10,    // number of batches
     *         100,   // series per batch
     *         batchNumber
     *     );
     * }
     * ```
     */
    storeGenerated(
        totalSeries: number,
        batches: number,
        batchSize: number,
        batch: number
    ): RemoteWriteResponse;
}

/**
 * Creates a Sample object.
 * 
 * This constructor function creates a sample (data point) with a value and timestamp.
 * 
 * @param value - The numeric value of the sample
 * @param timestamp - Timestamp in milliseconds
 * @returns A Sample object
 * 
 * @example
 * ```javascript
 * import remote from 'k6/x/remotewrite';
 * 
 * const sample1 = remote.Sample(42, Date.now());
 * const sample2 = remote.Sample(100.5, 1643235433000);
 * ```
 */
export function Sample(value: number, timestamp: number): Sample;

/**
 * Creates a Timeseries object.
 * 
 * This constructor function creates a time series from a map of labels and an array of samples.
 * Note that this takes labels as a simple object/map, unlike {@link TimeSeries} interface which uses an array.
 * 
 * @param labels - Map of label names to values (including __name__ for metric name)
 * @param samples - Array of samples
 * @returns A Timeseries object
 * 
 * @example
 * ```javascript
 * import remote from 'k6/x/remotewrite';
 * 
 * const ts = remote.Timeseries(
 *     { __name__: 'my_metric', service: 'api' },
 *     [remote.Sample(42, Date.now())]
 * );
 * client.store([ts]);
 * ```
 */
export function Timeseries(labels: Record<string, string>, samples: Sample[]): TimeSeries;

/**
 * Precompiles label templates for efficient metric generation.
 * 
 * Compiling templates once and reusing them is more efficient than compiling
 * on every call to {@link Client.storeFromTemplates}. Use this when you need
 * to call {@link Client.storeFromPrecompiledTemplates} repeatedly with the same template structure.
 * 
 * The returned object is opaque and can only be used with {@link Client.storeFromPrecompiledTemplates}.
 * 
 * @param labelsTemplate - Template for generating metric labels
 * @returns Precompiled label templates that can be reused across multiple calls
 * 
 * @example Precompile once, use many times
 * ```javascript
 * import remote from 'k6/x/remotewrite';
 * 
 * const template = {
 *     __name__: 'k6_generated_metric_${series_id/4}',
 *     series_id: '${series_id}',
 *     environment: 'production'
 * };
 * 
 * // Compile once during setup
 * const compiled = remote.precompileLabelTemplates(template);
 * 
 * const client = new remote.Client({ url: "https://..." });
 * 
 * // Reuse in every iteration
 * export default function() {
 *     client.storeFromPrecompiledTemplates(100, 200, Date.now(), 0, 100, compiled);
 * }
 * ```
 * 
 * @see {@link MetricTemplate} for template variable syntax
 * @see {@link Client.storeFromPrecompiledTemplates}
 */
export function precompileLabelTemplates(labelsTemplate: MetricTemplate): PrecompiledLabelTemplates;

/**
 * Default export containing the Client class and related types.
 */
declare const remotewrite: {
    Client: typeof Client;
    Sample: typeof Sample;
    Timeseries: typeof Timeseries;
    precompileLabelTemplates: typeof precompileLabelTemplates;
};

export default remotewrite;
