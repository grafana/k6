import Ember from 'ember';

export default Ember.Route.extend({
  _scheduleRefresh: Ember.on('init', function() {
    Ember.run.later(()=> {
      this.refresh();
      if (this.get('model.running')) {
        this._scheduleRefresh();
      }
    }, 5000);
  }),
  model() {
    return Ember.$.getJSON("/v1/status");
  },
  actions: {
    abort() {
      return Ember.$.post("/v1/abort").then(()=> {
        this.refresh();
      });
    },
  },
});
