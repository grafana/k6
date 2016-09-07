import Ember from 'ember';
import moment from 'moment';

export function metricFormat(params/*, hash*/) {
  let [value, type] = params;
  if (type === "time") {
    return moment.duration(value / 1000000, 'milliseconds').format('h[h]m[m]s[s]S[ms]');
  }
  return value;
}

export default Ember.Helper.helper(metricFormat);
