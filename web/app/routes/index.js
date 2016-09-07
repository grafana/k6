import Ember from 'ember';

export default Ember.Route.extend({
  model() {
    return Ember.RSVP.hash({
      "metrics": this.get('store').findAll('metric'),
    });
  },
});
