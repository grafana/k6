function print(metric) {
  const m = {
    id: metric.id,
    name: metric.name,
    value: metric.value,
    rating: metric.rating,
    delta: metric.delta,
    numEntries: metric.entries.length,
    navigationType: metric.navigationType,
    url: window.location.href,
    // To be able to associate a Web Vital measurement to the PageNavigation
    // span, we need to collect the span ID that was previously set in the
    // page after the navigation, and pass it back to k6 browser included in
    // the WV event so the measurement can be correctly linked to the page
    // navigation span
    spanID: window.k6SpanId,
  }
  window.k6browserSendWebVitalMetric(JSON.stringify(m))
}

function load() {
  webVitals.onCLS(print);
  webVitals.onFID(print);
  webVitals.onLCP(print);

  webVitals.onFCP(print);
  webVitals.onINP(print);
  webVitals.onTTFB(print);
}

load();
