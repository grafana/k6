import Ember from 'ember';
import DS from 'ember-data';

export default DS.Model.extend({
  name: DS.attr('string'),
  parent: DS.belongsTo('group', { inverse: 'groups' }),
  groups: DS.hasMany('group', { inverse: 'parent' }),
  checks: DS.hasMany('check'),

  checksSortedBy: ['id'],
  checksSorted: Ember.computed.sort('checks', 'checksSortedBy')
});
