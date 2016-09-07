import DS from 'ember-data';

export default DS.Model.extend({
  running: DS.attr('boolean'),
  activeVUs: DS.attr('number'),
  inactiveVUs: DS.attr('number')
});
