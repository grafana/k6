/**
 * **ICMP protocol support for k6**
 *
 * > **⚠️ Warning**
 * > This is an **experimental extension** for k6. It is **not officially supported yet**.
 * > We are actively working on it to make it officially supported in the future.
 *
 * **xk6-icmp** is a k6 extension that adds support for sending ICMP echo requests ({@link ping}) to measure network latency and availability.
 * This allows you to measure network latency and reachability of
 * hosts directly within your load testing and synthetic monitoring scenarios.
 *
 * The main use case for this extension is integration with Grafana's Synthetic Monitoring product
 * to make a quick reachability check after a scripted check fails.
 * It can also be used directly in k6 scripts, for example as a pre-check to verify network connectivity
 * before running your main test.
 *
 * @module icmp
 *
 * @example Basic Usage
 *
 * Here is a basic example showing how to use the {@link ping} function:
 *
 * ```javascript
 * import { ping } from "k6/x/icmp"
 *
 * export default function () {
 *   const host = "8.8.8.8"
 *
 *   console.log(`Pinging ${host}:`);
 *
 *   if (ping(host)) {
 *     console.log(`Host ${host} is reachable`);
 *   } else {
 *     console.error(`Host ${host} is unreachable`);
 *   }
 * }
 * ```
 *
 * @example Advanced Usage
 *
 * A more advanced example below demonstrates how to use the {@link PingCallback} to access detailed
 * ping results for each request.
 *
 * ```javascript
 *
 * import { ping } from "k6/x/icmp"
 *
 * export default async function () {
 *   const host = "8.8.8.8"
 *
 *   console.log(`Pinging ${host} with callback:`);
 *
 *   const opts = {
 *     timeout: 3000,
 *     count: 5
 *   };
 *
 *   const result = await pingAsync(host, opts, (err, { target, sent_at, received_at, seq, ttl, size }) => {
 *     if (err) {
 *       console.error(`${target}: ${err}`);
 *
 *       return
 *     }
 *
 *     const rtt = received_at - sent_at;
 *
 *     console.log(`${size} bytes from ${target}: icmp_seq=${seq} ttl=${ttl} time=${rtt} ms`);
 *   });
 *
 *   if (result) {
 *     console.log(`Host ${host} is reachable`);
 *   } else {
 *     console.error(`Host ${host} is unreachable`);
 *   }
 * }
 * ```
 */
export as namespace icmp;

/**
 * Duration type for timeouts and intervals.
 * The value is a number or string. The number value is in milliseconds,
 * the string value contains the unit (e.g. `2s`).
 */
export declare type Duration = number | string;

/**
 * IP protocol version.
 */
export declare enum IP {
  /** IPv4 */
  V4 = "ip4",
  /** IPv6 */
  V6 = "ip6"
}

/**
 * Options for configuring the ping operation.
 */
export declare interface PingOptions {
  /**
   * Identifier for the ICMP session.
   * It is a 16-bit unsigned integer. Default is a k6 process ID plus a VU ID modulo 65536.
   */
  id?: number;
  /**
   * Starting sequence number for the ICMP echo request.
   * It is a 16-bit unsigned integer. Default is a random value.
   */
  seq?: number;
  /**
   * Time to live for the ping packets.
   * This value is decremented by each router the packet passes through.
   */
  ttl?: number;
  /**
   * Timeout for the single echo request and its response.
   * The value is a number or string. The number value is in milliseconds,
   * the string value contains the unit (e.g. `2s`).
   *
   * If the timeout is exceeded, the callback will be invoked with an error
   * and the result will be false.
   * Default is `10s` (10 seconds).
   */
  timeout?: Duration;
  /**
   * Deadline for the whole ping operation.
   * The value is a number or string. The number value is in milliseconds,
   * the string value contains the unit (e.g. `2s`).
   * The deadline includes all echo requests and their responses.
   *
   * If the deadline is exceeded, the callback will be invoked with an error
   * and the result will be false.
   * Default is no deadline.
   */
  deadline?: Duration;
  /**
   * Number of ping requests to send.
   * Default is 1.
   *
   * If the count is set to a value greater than 1, the ping process will send multiple requests
   * and returns true if at least `threshold` percent of the requests succeed.
   */
  count?: number;
  /**
   * Minimum percent of successful ping responses required for the ping operation to be considered successful.
   * If this option is not set, the default is `100` (%).
   */
  threshold?: number;
  /**
   * Size of the data to include in the ping request, in bytes.
   * If this option is not set, 56 bytes will be used (ping command default).
   */
  size?: number;
  /**
   * Interval between ping requests if `count` is set to a value greater than 1.
   * The value is a number or string. The number value is in milliseconds,
   * the string value contains the unit (e.g. `2s`).
   * Default is `1s` (1 second).
   *
   * A very low value can be used for DoS attacks, so the allowed minimum value
   * can be configured via the `K6_PING_MINIMUM_INTERVAL` environment variable for safety.
   * Default allowed minimum value is `500ms` (0.5 seconds).
   */
  interval?: Duration;
  /**
   * Source IP address to use for the ping request.
   * If not set, the system's default source address will be used.
   */
  source?: string;
  /**
   * Preferred IP protocol version to use for the ping request if the target
   * is not an IPv4 or IPv6 address.
   * Valid values are "ip4" and "ip6".
   * If not set, "ip4" protocol will be used.
   */
  preferred_ip_version?: IP;
  /**
   * Optional tags for metrics and logging.
   */
  tags?: Record<string, string>;
}

/**
 * Data you receive as a parameter in the ping callback.
 */
export declare interface PingDetail {
  /**
   * Indicates whether the target is reachable (if an echo reply was received).
   */
  alive: boolean;
  /**
   * Hostname or IP address that was pinged.
   */
  target: string;
  /**
   * Target IP address that was pinged.
   */
  target_ip: string;
  /**
   * Target IP version that was used for the ping request.
   * Valid values are "ip4" and "ip6".
   */
  target_ip_version: IP;
  /**
   * Timestamp when the request was sent.
   * Value is in UTC milliseconds since the Unix epoch.
   */
  sent_at?: number;
  /**
   * Timestamp when the response was received.
   * Value is in UTC milliseconds since the Unix epoch.
   */
  received_at?: number;
  /**
   * Time to live from the echo reply, if a response was received.
   */
  ttl?: number;
  /**
   * Identifier for the ICMP session from the echo reply, if a response was received.
   */
  id?: number;
  /**
   * Sequence number for the ICMP echo request from the echo reply, if a response was received.
   */
  seq?: number;
  /**
   * Size of the ICMP echo reply, if a response was received.
   */
  size?: number;
  /**
   * Ping options used for the request.
   */
  options: PingOptions;
}

/**
 * Callback function for ping results.
 * This function is called for every echo packet sent.
 * If the deadline is exceeded, it is not called for the remaining packets.
 * If the echo response has been received, the error value will be null,
 * otherwise it contains the error.
 *
 * @param err  Error object if an error occurred, otherwise null.
 * @param data Data about the ping attempt, including timing and echo details.
 */
export declare type PingCallback = (err: Error | null, data?: PingDetail) => void;

/**
 * Sends ICMP echo requests (pings) to the specified target.
 *
 * @param target   Hostname or IP address to ping.
 * @param options  Optional ping options.
 * @returns        true if the number of successful pings is greater than or equal to the threshold, otherwise false.
 *
 * @example
 * ```javascript
 * import { ping } from "k6/x/icmp"
 *
 * export default function () {
 *   const host = "8.8.8.8"
 *
 *   console.log(`Pinging ${host}:`);
 *
 *   if (ping(host)) {
 *     console.log(`Host ${host} is reachable`);
 *   } else {
 *     console.error(`Host ${host} is unreachable`);
 *   }
 * }
 * ```
 */
export declare function ping(target: string, opts?: PingOptions): boolean;

/**
 * Sends ICMP echo requests (pings) to the specified target.
 *
 * @param target           Hostname or IP address to ping.
 * @param optsOrCallback   Optional ping options or callback function.
 * @param callback         Optional callback function for ping results.
 * @returns                Promise that resolves to true if the number of successful pings is greater than or equal to the threshold, otherwise false.
 *
 * @example
 * ```javascript
 *
 * import { pingAsync } from "k6/x/icmp"
 *
 * export default async function () {
 *   const host = "8.8.8.8"
 *
 *   console.log(`Pinging ${host} with callback:`);
 *
 *   const opts = {
 *     timeout: 1000,
 *     retries: 2,
 *     count: 5
 *   };
 *
 *   const result = await pingAsync(host, opts, (err, { target, sent_at, received_at, seq, ttl, size, options }) => {
 *     if (err) {
 *       console.error(`${target}: ${err}`);
 *
 *       return
 *     }
 *
 *     const rtt = received_at - sent_at;
 *
 *     console.log(`${size} bytes from ${target}: icmp_seq=${seq} ttl=${ttl} time=${rtt} ms`);
 *   });
 *
 *   if (result) {
 *     console.log(`Host ${host} is reachable`);
 *   } else {
 *     console.error(`Host ${host} is unreachable`);
 *   }
 * }
 * ```
 */
export declare function pingAsync(target: string, optsOrCallback?: PingOptions | PingCallback, callback?: PingCallback): Promise<boolean>;

/**
 * Represents an error that occurred during an ICMP operation.
 */
export declare interface ICMPError {
  /** The error name. */
  name: "ICMPError"
  /** The error message. */
  message: string;
}
