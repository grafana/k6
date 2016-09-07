import Ember from 'ember';

export default Ember.Controller.extend({
  application: Ember.inject.controller(),

  vusActive: Ember.computed.alias('application.model.activeVUs'),
  vusInactive: Ember.computed.alias('application.model.inactiveVUs'),
  vusMax: Ember.computed('vusActive', 'vusInactive', function() {
    return this.get('vusActive') + this.get('vusInactive');
  }),
  vusPercent: Ember.computed('vusActive', 'maxVUs', function() {
    return (this.get('vusActive') / this.get('maxVUs')) * 100;
  }),
  vusLabel: Ember.computed('vusActive', 'maxVUs', function() {
    return this.get('vusActive') + ' / ' + this.get('vusMax');
  }),
});
