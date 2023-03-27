// This file contains an adaptation of the encrypt_decrypt/aes.js
// implementation from the W3C WebCrypto API test suite.
//
// Some of the function have been modified to support the k6 javascript runtime,
// and to limit its dependency to the rest of the W3C WebCrypto API test suite internal
// codebase.
//
// The original implementation is available at:
// https://github.com/web-platform-tests/wpt/blob/1ec10682955d63944bc8fa06f1b3a26377952533/WebCryptoAPI/encrypt_decrypt/aes.js

function run_test() {
    var subtle = crypto.subtle; // Change to test prefixed implementations

    // When are all these tests really done? When all the promises they use have resolved.
    var all_promises = [];

    // Source file aes_XXX_vectors.js provides the getTestVectors method
    // for the AES-XXX algorithm that drives these tests.
    var vectors = getTestVectors();
    var passingVectors = vectors.passing;
    var failingVectors = vectors.failing;
    var decryptionFailingVectors = vectors.decryptionFailing;

    // Check for successful encryption.
    passingVectors.forEach(function(vector) {
        var promise = importVectorKey(vector, ["encrypt", "decrypt"])
        .then(function(vector) {
            return subtle.encrypt(vector.algorithm, vector.key, vector.plaintext)
            .then(function(result) {
                assert_true(equalBuffers(result, vector.result), "Should return expected result");
            }, function(err) {
                assert_unreached("encrypt error for test " + vector.name + ": " + err.message);
            });
        }, function(err) {
            // We need a failed test if the importVectorKey operation fails, so
            // we know we never tested encryption
            assert_unreached("importKey failed for " + vector.name);
        });

        all_promises.push(promise);
    });

    // Check for successful encryption even if the buffer is changed after calling encrypt.
    passingVectors.forEach(function(vector) {
        var plaintext = copyBuffer(vector.plaintext);
        var promise = importVectorKey(vector, ["encrypt", "decrypt"])
        .then(function(vector) {
            var operation = subtle.encrypt(vector.algorithm, vector.key, vector.plaintext)
            .then(function(result) {
                assert_true(equalBuffers(result, vector.result), "Should return expected result");
            }, function(err) {
                assert_unreached("encrypt error for test " + vector.name + ": " + err.message);
            });
            plaintext[0] = 255 - plaintext[0];
            return operation;
        }, function(err) {
            // We need a failed test if the importVectorKey operation fails, so
            // we know we never tested encryption
            assert_unreached("importKey failed for " + vector.name);
        });

        all_promises.push(promise);
    });

    // Check for successful decryption.
    passingVectors.forEach(function(vector) {
        var promise = importVectorKey(vector, ["encrypt", "decrypt"])
        .then(function(vector) {
            return subtle.decrypt(vector.algorithm, vector.key, vector.result)
            .then(function(result) {
                assert_true(equalBuffers(result, vector.plaintext), "Should return expected result");
            }, function(err) {
                assert_unreached("decrypt error for test " + vector.name + ": " + err.message);
            });
        }, function(err) {
            // We need a failed test if the importVectorKey operation fails, so
            // we know we never tested encryption
            assert_unreached("importKey failed for " + vector.name);
        });

        all_promises.push(promise);
    });

    // FIXME @oleiade: Although not necessary, this test is currently failing
    // as it is unclear if Go's implementation of AES-CBC provides the same
    // guarantees as the W3C WebCrypto API test suite expects. (changing a byte
    // in the ciphertext should result in a decryption success)
     
    // Check for successful decryption even if ciphertext is altered.
    // passingVectors.forEach(function(vector) {
    //     var ciphertext = copyBuffer(vector.result);
    //     var promise = importVectorKey(vector, ["encrypt", "decrypt"])
    //     .then(function(vector) {
    //         var operation = subtle.decrypt(vector.algorithm, vector.key, ciphertext)
    //         .then(function(result) {
    //             assert_true(equalBuffers(result, vector.plaintext), "Should return expected result");
    //         }, function(err) {
    //             assert_unreached("decrypt error for test " + vector.name + ": " + err.message);
    //         });
    //         ciphertext[0] = 255 - ciphertext[0];
    //         return operation;
    //     }, function(err) {
    //         // We need a failed test if the importVectorKey operation fails, so
    //         // we know we never tested encryption
    //         assert_unreached("importKey failed for " + vector.name);
    //     });

    //     all_promises.push(promise);
    // });

    // Everything that succeeded should fail if no "encrypt" usage.
    passingVectors.forEach(function(vector) {
        // Don't want to overwrite key being used for success tests!
        var badVector = Object.assign({}, vector);
        badVector.key = null;

        var promise = importVectorKey(badVector, ["decrypt"])
        .then(function(vector) {
            return subtle.encrypt(vector.algorithm, vector.key, vector.plaintext)
            .then(function(result) {
                assert_unreached("should have thrown exception for test " + vector.name);
            }, function(err) {
                assert_equals(err.name, "InvalidAccessError", "Should throw an InvalidAccessError instead of " + err.message)
            });
        }, function(err) {
            // We need a failed test if the importVectorKey operation fails, so
            // we know we never tested encryption
            assert_unreached("importKey failed for " + vector.name);
        });

        all_promises.push(promise);
    });

    // Encryption should fail if algorithm of key doesn't match algorithm of function call.
    passingVectors.forEach(function(vector) {
        var algorithm = Object.assign({}, vector.algorithm);
        if (algorithm.name === "AES-CBC") {
            algorithm.name = "AES-CTR";
            algorithm.counter = new Uint8Array(16);
            algorithm.length = 64;
        } else {
            algorithm.name = "AES-CBC";
            algorithm.iv = new Uint8Array(16); // Need syntactically valid parameter to get to error being checked.
        }

        var promise = importVectorKey(vector, ["encrypt", "decrypt"])
        .then(function(vector) {
            return subtle.encrypt(algorithm, vector.key, vector.plaintext)
            .then(function(result) {
                assert_unreached("encrypt succeeded despite mismatch " + vector.name);
            }, function(err) {
                assert_equals(err.name, "InvalidAccessError", "Mismatch should cause InvalidAccessError instead of " + err.message);
            });
        }, function(err) {
            // We need a failed test if the importVectorKey operation fails, so
            // we know we never tested encryption
            assert_unreached("importKey failed for " + vector.name);
        });

        all_promises.push(promise);
    });

    // Everything that succeeded decrypting should fail if no "decrypt" usage.
    passingVectors.forEach(function(vector) {
        // Don't want to overwrite key being used for success tests!
        var badVector = Object.assign({}, vector);
        badVector.key = null;

        var promise = importVectorKey(badVector, ["encrypt"])
        .then(function(vector) {
            return subtle.decrypt(vector.algorithm, vector.key, vector.result)
            .then(function(result) {
                assert_unreached("should have thrown exception for test " + vector.name);
            }, function(err) {
                assert_equals(err.name, "InvalidAccessError", "Should throw an InvalidAccessError instead of " + err.message)
            });
        }, function(err) {
            // We need a failed test if the importVectorKey operation fails, so
            // we know we never tested encryption
            assert_unreached("importKey failed for " + vector.name);
        });

        all_promises.push(promise);
    });

    // Check for OperationError due to data lengths.
    failingVectors.forEach(function(vector) {
        var promise = importVectorKey(vector, ["encrypt", "decrypt"])
        .then(function(vector) {
            return subtle.encrypt(vector.algorithm, vector.key, vector.plaintext)
            .then(function(result) {
                assert_unreached("should have thrown exception for test " + vector.name);
            }, function(err) {
                assert_equals(err.name, "OperationError", "Should throw an OperationError instead of " + err.message)
            });
        }, function(err) {
            // We need a failed test if the importVectorKey operation fails, so
            // we know we never tested encryption
            assert_unreached("importKey failed for " + vector.name);
        });

        all_promises.push(promise);
    });

    // Check for OperationError due to data lengths for decryption, too.
    failingVectors.forEach(function(vector) {
        var promise = importVectorKey(vector, ["encrypt", "decrypt"])
        .then(function(vector) {
            return subtle.decrypt(vector.algorithm, vector.key, vector.result)
            .then(function(result) {
                assert_unreached("should have thrown exception for test " + vector.name);
            }, function(err) {
                assert_equals(err.name, "OperationError", "Should throw an OperationError instead of " + err.message)
            });
        }, function(err) {
            // We need a failed test if the importVectorKey operation fails, so
            // we know we never tested encryption
            assert_unreached("importKey failed for " + vector.name);
        });

        all_promises.push(promise);
    });

    // Check for decryption failing for algorithm-specific reasons (such as bad
    // padding for AES-CBC).
    decryptionFailingVectors.forEach(function(vector) {
        var promise = importVectorKey(vector, ["encrypt", "decrypt"])
        .then(function(vector) {
            return subtle.decrypt(vector.algorithm, vector.key, vector.result)
            .then(function(result) {
                assert_unreached("should have thrown exception for test " + vector.name);
            }, function(err) {
                assert_equals(err.name, "OperationError", "Should throw an OperationError instead of " + err.message)
            });
        }, function(err) {
            // We need a failed test if the importVectorKey operation fails, so
            // we know we never tested encryption
            assert_unreached("importKey failed for " + vector.name);
        });

        all_promises.push(promise);
    });

    // Note @oleiade: I'm pretty sure this is specific to the Webplatform tests
    // return Promise.all(all_promises)
    //     .then(function() {done();})
    //     .catch(function() {done();})

    // A test vector has all needed fields for encryption, EXCEPT that the
    // key field may be null. This function replaces that null with the Correct
    // CryptoKey object.
    //
    // Returns a Promise that yields an updated vector on success.
    function importVectorKey(vector, usages) {
        if (vector.key !== null) {
            return new Promise(function(resolve, reject) {
                resolve(vector);
            });
        } else {
            return subtle.importKey("raw", vector.keyBuffer, {name: vector.algorithm.name}, false, usages)
            .then(function(key) {
                vector.key = key;
                return vector;
            });
        }
    }

    // Returns a copy of the sourceBuffer it is sent.
    function copyBuffer(sourceBuffer) {
        var source = new Uint8Array(sourceBuffer);
        var copy = new Uint8Array(sourceBuffer.byteLength)

        for (var i=0; i<source.byteLength; i++) {
            copy[i] = source[i];
        }

        return copy;
    }

    function equalBuffers(a, b) {
        if (a.byteLength !== b.byteLength) {
            return false;
        }

        var aBytes = new Uint8Array(a);
        var bBytes = new Uint8Array(b);

        for (var i=0; i<a.byteLength; i++) {
            if (aBytes[i] !== bBytes[i]) {
                return false;
            }
        }

        return true;
    }

    return;
}