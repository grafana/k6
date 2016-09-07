import Ember from 'ember';

export function initialize(/* application */) {
  var inflector = Ember.Inflector.inflector;
  inflector.uncountable('status');
}

export default {
  name: 'inflector',
  initialize
};
