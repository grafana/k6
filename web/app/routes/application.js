import Ember from 'ember';

export default Ember.Route.extend({
  _scheduleRefresh: Ember.on('init', function() {
    Ember.run.later(()=> {
      this.refresh();
      if (this.get('controller.model.running')) {
        this._scheduleRefresh();
      }
    }, 5000);
  }),
  model() {
    return this.get('store').findRecord('status', 'default');
  }
});
