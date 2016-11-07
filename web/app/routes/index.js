import Ember from 'ember';

export default Ember.Route.extend({
  model() {
    return Ember.RSVP.hash({
      "metrics": this.get('store').findAll('metric'),
      "groups": this.get('store').findAll('group'),
      "checks": this.get('store').findAll('check'),
    });
  },
  afterModel(model) {
    model["defaultGroup"] = this.get('store').peekRecord('group', 0);
  },
});
