import Ember from 'ember';

export default Ember.Controller.extend({
  flashMessages: Ember.inject.service(),

  paused: Ember.computed.alias('model.paused'),

  actions: {
    pause() {
      var model = this.get('model');
      model.set('paused', true);
      return model.save().catch((err) => {
        for (var e of err.errors) {
          this.get('flashMessages').danger(e.title);
        }
      });
    },
    resume() {
      var model = this.get('model');
      model.set('paused', false);
      return model.save().catch((err) => {
        for (var e of err.errors) {
          this.get('flashMessages').danger(e.title);
        }
      });
    },
  },
});
