import Ember from 'ember';

export default Ember.Controller.extend({
  flashMessages: Ember.inject.service(),

  running: Ember.computed.alias('model.running'),

  actions: {
    pause() {
      var model = this.get('model');
      model.set('running', false);
      return model.save().catch((err) => {
        for (var e of err.errors) {
          this.get('flashMessages').danger(e.title);
        }
      });
    },
    resume() {
      var model = this.get('model');
      model.set('running', true);
      return model.save().catch((err) => {
        for (var e of err.errors) {
          this.get('flashMessages').danger(e.title);
        }
      });
    },
  },
});
