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
