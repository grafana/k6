import Ember from 'ember';

export default Ember.Controller.extend({
  vus: Ember.computed.alias('model.metrics.vus.data.value'),
  vus_pooled: Ember.computed.alias('model.metrics.vus_pooled.data.value'),
  vus_max: Ember.computed('vus', 'vus_pooled', function() {
    return this.get('vus') + this.get('vus_pooled');
  }),
  vus_pct: Ember.computed('vus', 'vus_max', function() {
    return (this.get('vus') / this.get('vus_max')) * 100;
  }),
});
