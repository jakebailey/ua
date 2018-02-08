define(['crypto'], function(crypto) {

    function SimpleCrypto() {}

    /**
     * Generates an algorithm name based on a key's size. I.e., providing a
     * 16 byte key will return "aes-128-cfb".
     * @param {Buffer} key The AES key used for encryption.
     * @return {string} The algorithm name to give to createCipher.
     */
    SimpleCrypto.prototype.algoFromKey = function(key) {
        return 'aes-' + (key.length * 8) + '-cfb';
    };

    /**
     * Encrypts a payload with AES CFB.
     * @param {Buffer} key The AES key used for encryption.
     * @param {Buffer} payload The payload to be encrypted.
     * @return {Buffer} Ciphertext (IV + encrypted payload).
     */
    SimpleCrypto.prototype.encrypt = function(key, payload) {
        const algo = algoFromKey(key);

        const iv = crypto.randomBytes(16);
        const cipher = crypto.createCipheriv(algo, key, iv);

        let encrypted = cipher.update(payload);
        encrypted = Buffer.concat([encrypted, cipher.final()]);

        return Buffer.concat([iv, encrypted]);
    };

    /**
     * Decrypts a payload with AES CFB.
     * @param {Buffer} key The AES key used for decryption.
     * @param {Buffer} ciphertext Ciphertext (IV + encrypted payload).
     * @return {Buffer} The original payload. 
     */
    SimpleCrypto.prototype.decrypt = function(key, ciphertext) {
        const algo = algoFromKey(key);

        if (ciphertext.length < 16) {
            throw Error("simplecrypto: ciphertext too short");
        }

        const iv = ciphertext.slice(0, 16);
        ciphertext = ciphertext.slice(16);

        const decipher = crypto.createDecipheriv(algo, key, iv);

        let payload = decipher.update(ciphertext);
        payload = Buffer.concat([payload, decipher.final()]);
        return payload;
    };


    /**
     * Calculates the HMAC of the message using SHA256.
     * @param {Buffer} key The encryption key.
     * @param {Buffer} message The message being HMAC'd.
     * @return {Buffer} The calculated MAC.
     */
    SimpleCrypto.prototype.hmac = function(key, message) {
        const mac = crypto.createHmac('sha256');
        mac.update(message);
        return mac.digest();
    };

    /**
     * Checks whether messageMAC is a valid HMAC tag for a message.
     * @param {Buffer} key The encryption key.
     * @param {Buffer} message The message being HMAC'd.
     * @param {Buffer} messageMAC The (proposed) MAC of the message.
     * @return {boolean} True if the MACs match.
     */
    SimpleCrypto.prototype.checkMAC = function(key, message, messageMAC) {
        const expectedMAC = hmac(key, message);
        return crypto.timingSafeEqual(messageMAC, expectedMAC);
    };

    /**
     * Decodes a serialized message (data) using a key, and returns the
     * decrypted payload. If the decoded ciphertext does not match the
     * decoded HMAC, an error is thrown.
     * @param {Buffer} key The encryption key.
     * @param {String} data The encoded message as a string.
     * @return {Buffer} The decrypted payload.
     */
    SimpleCrypto.prototype.decodeJSON = function(key, data) {
        const obj = JSON.parse(data);
        const ciphertext = Buffer.from(obj.ciphertext, 'base64');
        const hmac = Buffer.from(obj.hmac, 'base64');

        if (!this.checkMAC(key, ciphertext, hmac)) {
            throw Error("simplecrypto: HMAC does not match ciphertext");
        }

        return this.decrypt(key, ciphertext);
    };

    /**
     * Encodes a payload using a key, then encodes it as a JSON object,
     * which includes the ciphertext and its HMAC.
     * @param {Buffer} key The encryption key.
     * @param {Buffer} payload The payload to be encrypted.
     * @return {String} The encoded message as a string.
     */
    SimpleCrypto.prototype.encodeJSON = function(key, payload) {
        const ciphertext = this.encrypt(key, payload);
        const hmac = this.hmac(key, ciphertext);

        return JSON.stringify({
            ciphertext: ciphertext,
            hmac: hmac
        });
    };

    return new SimpleCrypto();
})