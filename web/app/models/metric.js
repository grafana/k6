import DS from 'ember-data';

export default DS.Model.extend({
  type: DS.attr('string'),
  contains: DS.attr('string'),
  sample: DS.attr()
});
