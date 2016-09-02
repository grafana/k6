import Ember from 'ember';

export default Ember.Controller.extend({
  running: Ember.computed.alias('model.running'),
});
