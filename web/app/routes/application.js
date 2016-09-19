import Ember from 'ember';

export default Ember.Route.extend({
  _scheduleRefresh: Ember.on('init', function(delay = 5000) {
    Ember.run.later(()=> {
      this.refresh();
      this._scheduleRefresh(this.get('controller.model.running') ? 5000 : 15000);
    }, delay);
  }),
  model() {
    return this.get('store').findRecord('status', 'default');
  }
});
