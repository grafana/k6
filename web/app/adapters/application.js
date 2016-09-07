import DS from 'ember-data';

export default DS.JSONAPIAdapter.extend({
  namespace: "v1",
  urlForFindRecord(id, modelName, snapshot) {
    if (id === "default") {
      return this.urlForFindAll(modelName, snapshot);
    }
    return this._super(id, modelName, snapshot);
  },
});
