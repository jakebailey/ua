// This file implements a similar interface as the Go library, but in node.js.
// This is only for demonstration, and isn't intended to be imported directly
// with something like npm without some work.

var crypto = require('crypto');

/**
 * Generates an algorithm name based on a key's size. I.e., providing a
 * 16 byte key will return "aes-128-cfb".
 * @param {Buffer} key The AES key used for encryption.
 * @return {string} The algorithm name to give to createCipher.
 */
function algoFromKey(key) {
    return 'aes-' + (key.length * 8) + '-cfb';
}

/**
 * Encrypts a payload with AES CFB.
 * @param {Buffer} key The AES key used for encryption.
 * @param {Buffer} payload The payload to be encrypted.
 * @return {Buffer} Ciphertext (IV + encrypted payload).
 */
function encrypt(key, payload) {
    const algo = algoFromKey(key);

    const iv = crypto.randomBytes(16);
    const cipher = crypto.createCipheriv(algo, key, iv);

    let encrypted = cipher.update(payload);
    encrypted = Buffer.concat([encrypted, cipher.final()]);

    return Buffer.concat([iv, encrypted]);
}

/**
 * Decrypts a payload with AES CFB.
 * @param {Buffer} key The AES key used for decryption.
 * @param {Buffer} ciphertext Ciphertext (IV + encrypted payload).
 * @return {Buffer} The original payload. 
 */
function decrypt(key, ciphertext) {
    const algo = algoFromKey(key);
    
    const iv = ciphertext.slice(0, 16);
    ciphertext = ciphertext.slice(16);

    const decipher = crypto.createDecipheriv(algo, key, iv);

    let payload = decipher.update(ciphertext);
    payload = Buffer.concat([payload, decipher.final()]);
    return payload;
}


/**
 * Calculates the HMAC of the message using SHA256.
 * @param {Buffer} key The encryption key.
 * @param {Buffer} message The message being HMAC'd.
 * @return {Buffer} The calculated MAC.
 */
function hmac(key, message) {
    const mac = crypto.createHmac('sha256');
    mac.update(message);
    return mac.digest();
}

/**
 * Checks whether messageMAC is a valid HMAC tag for a message.
 * @param {Buffer} key The encryption key.
 * @param {Buffer} message The message being HMAC'd.
 * @param {Buffer} messageMAC The (proposed) MAC of the message.
 * @return {boolean} True if the MACs match.
 */
function checkMAC(key, message, messageMAC) {
    const expectedMAC = hmac(key, message);
    return crypto.timingSafeEqual(messageMAC, expectedMAC);
}