import Ember from 'ember';
import FlashObject from 'ember-cli-flash/flash/object';

const { K } = Ember;

FlashObject.reopen({ init: K });
