package summary

import (
	"go.k6.io/k6/metrics"
	"strings"
	"time"
)

func isHTTPMetric(metricName string) bool {
	return oneOfMetrics(metricName,
		metrics.HTTPReqsName,
		metrics.HTTPReqFailedName,
		metrics.HTTPReqDurationName,
		metrics.HTTPReqBlockedName,
		metrics.HTTPReqConnectingName,
		metrics.HTTPReqTLSHandshakingName,
		metrics.HTTPReqSendingName,
		metrics.HTTPReqWaitingName,
		metrics.HTTPReqReceivingName,
	)
}

func isExecutionMetric(metricName string) bool {
	return oneOfMetrics(metricName, metrics.VUsName,
		metrics.VUsMaxName,
		metrics.IterationsName,
		metrics.IterationDurationName,
		metrics.DroppedIterationsName,
	)
}

func isNetworkMetric(metricName string) bool {
	return oneOfMetrics(metricName, metrics.DataSentName, metrics.DataReceivedName)
}

func isBrowserMetric(metricName string) bool {
	return strings.HasPrefix(metricName, "browser_") && !isWebVitalsMetric(metricName)
}

func isWebVitalsMetric(metricName string) bool {
	return strings.HasPrefix(metricName, "browser_web_vital_")
}

func isGrpcMetric(metricName string) bool {
	return strings.HasPrefix(metricName, "grpc_")
}

func isWebSocketsMetric(metricName string) bool {
	return strings.HasPrefix(metricName, "ws_")
}

func isSkippedMetric(metricName string) bool {
	return oneOfMetrics(metricName, metrics.ChecksName, metrics.GroupDurationName)
}

func oneOfMetrics(metricName string, values ...string) bool {
	for _, v := range values {
		if strings.HasPrefix(metricName, v) {
			return true
		}
	}
	return false
}

func calculateCounterRate(count float64, duration time.Duration) float64 {
	if duration == 0 {
		return 0
	}
	return count / (float64(duration) / float64(time.Second))
}
