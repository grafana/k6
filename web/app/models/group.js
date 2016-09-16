import Ember from 'ember';
import DS from 'ember-data';

export default DS.Model.extend({
  name: DS.attr('string'),
  parent: DS.belongsTo('group', { inverse: 'groups' }),
  groups: DS.hasMany('group', { inverse: 'parent' }),
  tests: DS.hasMany('test'),

  testsSortedBy: ['id'],
  testsSorted: Ember.computed.sort('tests', 'testsSortedBy')
});
