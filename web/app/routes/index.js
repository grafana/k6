import Ember from 'ember';

export default Ember.Route.extend({
  model() {
    return Ember.RSVP.hash({
      "metrics": this.get('store').findAll('metric'),
      "groups": this.get('store').findAll('group'),
      "tests": this.get('store').findAll('test'),
    });
  },
  afterModel(model) {
    model["defaultGroup"] = this.get('store').peekRecord('group', 0);
  },
});
