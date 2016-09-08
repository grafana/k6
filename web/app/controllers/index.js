import Ember from 'ember';

export default Ember.Controller.extend({
  application: Ember.inject.controller(),

  vus: Ember.computed.alias('application.model.vus'),
  vusMax: Ember.computed.alias('application.model.vusMax'),
  vusPercent: Ember.computed('vus', 'vusMax', function() {
    return (this.get('vus') / this.get('vusMax')) * 100;
  }),
  vusLabel: Ember.computed('vus', 'vusMax', function() {
    return this.get('vus') + ' / ' + this.get('vusMax');
  }),
});
