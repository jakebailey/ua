import base64
import json
from Crypto.Cipher import AES
from Crypto.Random import get_random_bytes
from Crypto.Hash import HMAC, SHA256


def encrypt(key, payload):
    iv = get_random_bytes(AES.block_size)
    cipher = AES.new(key, AES.MODE_CFB, iv=iv)

    ciphertext = cipher.encrypt(payload)
    return iv + ciphertext


def decrypt(key, ciphertext):
    iv = ciphertext[:AES.block_size]
    ciphertext = ciphertext[AES.block_size:]

    cipher = AES.new(key, AES.MODE_CFB, iv=iv)
    return cipher.decrypt(ciphertext)


def hmac(key, message):
    hm = HMAC.new(key, digestmod=SHA256)
    hm.update(message)
    return hm.digest()


def check_hmac(key, message, h):
    hm = HMAC.new(key, digestmod=SHA256)
    hm.update(message)
    try:
        hm.verify(h)
        return True
    except ValueError:
        return False


def encrypt_and_hmac(key, payload):
    ciphertext = encrypt(key, payload)
    h = hmac(key, ciphertext)
    return ciphertext, h


def check_and_decrypt(key, ciphertext, h):
    if not check_hmac(key, ciphertext, h):
        return None

    return decrypt(key, ciphertext)


def encode_json(key, payload):
    ciphertext, h = encrypt_and_hmac(key, payload)
    return json.dumps({
        "ciphertext": str(base64.standard_b64encode(ciphertext), "utf-8"),
        "hmac": str(base64.standard_b64encode(h), "utf-8"),
    })


def decode_json(key, data):
    d = json.loads(data)
    ciphertext = base64.standard_b64decode(d["ciphertext"])
    h = base64.standard_b64decode(d["hmac"])
    return check_and_decrypt(key, ciphertext, h)
