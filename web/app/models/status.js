import DS from 'ember-data';

export default DS.Model.extend({
  running: DS.attr('boolean'),
  paused: DS.attr('boolean'),
  tainted: DS.attr('boolean'),
  vus: DS.attr('number'),
  vusMax: DS.attr('number')
});
