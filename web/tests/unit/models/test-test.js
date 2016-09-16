import { moduleForModel, test } from 'ember-qunit';

moduleForModel('test', 'Unit | Model | test', {
  // Specify the other units that are required for this test.
  needs: ['model:group']
});

test('it exists', function(assert) {
  let model = this.subject();
  // let store = this.store();
  assert.ok(!!model);
});
