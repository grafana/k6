import DS from 'ember-data';

export default DS.JSONAPIAdapter.extend({
  namespace: "v1",
  pathForType(modelName) {
    switch (modelName) {
    case 'status':
      return modelName
    default:
      return this._super(modelName);
    }
  },
  urlForFindRecord(id, modelName, snapshot) {
    if (id === "default") {
      return this.urlForFindAll(modelName, snapshot);
    }
    return this._super(id, modelName, snapshot);
  },
});
