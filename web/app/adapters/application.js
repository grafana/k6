import DS from 'ember-data';

export default DS.JSONAPIAdapter.extend({
  namespace: "v1",
  buildURL(modelName, id, snapshot, requestType, query) {
    if (id == "default") {
      return this.urlForFindAll(modelName, snapshot);
    }
    return this._super(modelName, id, snapshot, requestType, query);
  }
});
