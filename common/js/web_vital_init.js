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
  const reportAllChanges = {
    reportAllChanges: true,
  }
  webVitals.onCLS(print, reportAllChanges);
  webVitals.onFID(print, reportAllChanges);
  webVitals.onLCP(print, reportAllChanges);
  webVitals.onFCP(print, reportAllChanges);
  webVitals.onINP(print, reportAllChanges);
  webVitals.onTTFB(print, reportAllChanges);
}

load();
