import Ember from 'ember';

export default Ember.Controller.extend({
  running: Ember.computed.alias('model.running'),
  actions: {
    abort() {
      var model = this.get('model');
      model.set('running', false);
      return model.save();
    },
  },
});
