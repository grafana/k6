import Ember from 'ember';
import InflectorInitializer from 'k6/initializers/inflector';
import { module, test } from 'qunit';

let application;

module('Unit | Initializer | inflector', {
  beforeEach() {
    Ember.run(function() {
      application = Ember.Application.create();
      application.deferReadiness();
    });
  }
});

// Replace this with your real tests.
test('it works', function(assert) {
  InflectorInitializer.initialize(application);

  // you would normally confirm the results of the initializer here
  assert.ok(true);
});
