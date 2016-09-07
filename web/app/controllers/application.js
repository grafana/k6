import Ember from 'ember';

export default Ember.Controller.extend({
  flashMessages: Ember.inject.service(),

  running: Ember.computed.alias('model.running'),

  actions: {
    abort() {
      var model = this.get('model');
      model.set('running', false);
      return model.save().then(() => {
        this.get('flashMessages').success("Test stopped");
      }, (err) => {
        for (var e of err.errors) {
          this.get('flashMessages').danger(e.title);
        }
      });
    },
  },
});
