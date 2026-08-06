// Legado-compatible symmetric cryptography exposed to source JavaScript.
package analyzer

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"strings"
)

// ToBytes converts JavaScript byte-array exports into Go bytes.
func ToBytes(value interface{}) ([]byte, error) {
	switch typed := value.(type) {
	case []byte:
		return append([]byte(nil), typed...), nil
	case string:
		return []byte(typed), nil
	case []interface{}:
		result := make([]byte, len(typed))
		for index, item := range typed {
			number, ok := item.(int64)
			if !ok {
				if float, floatOK := item.(float64); floatOK {
					number, ok = int64(float), true
				}
			}
			if !ok || number < -128 || number > 255 {
				return nil, fmt.Errorf("js crypto: invalid byte at %d", index)
			}
			result[index] = byte(number)
		}
		return result, nil
	default:
		return nil, fmt.Errorf("js crypto: unsupported key type %T", value)
	}
}

type jsSymmetricCrypto struct {
	block cipher.Block
	iv    []byte
}

func jsBytes(value interface{}) ([]byte, error) { return ToBytes(value) }

func newJSSymmetricCrypto(transformation string, key, iv []byte) (*jsSymmetricCrypto, error) {
	parts := strings.Split(strings.ToUpper(strings.TrimSpace(transformation)), "/")
	if len(parts) != 3 || parts[0] != "AES" || parts[1] != "CBC" || (parts[2] != "PKCS5PADDING" && parts[2] != "PKCS7PADDING") {
		return nil, fmt.Errorf("js crypto: unsupported transformation %q", transformation)
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("js crypto: AES key: %w", err)
	}
	if len(iv) == 0 {
		iv = make([]byte, block.BlockSize())
	}
	if len(iv) != block.BlockSize() {
		return nil, fmt.Errorf("js crypto: IV length %d, want %d", len(iv), block.BlockSize())
	}
	return &jsSymmetricCrypto{block: block, iv: append([]byte(nil), iv...)}, nil
}

func (c *jsSymmetricCrypto) Encrypt(data interface{}) ([]byte, error) {
	plain, err := jsBytes(data)
	if err != nil {
		return nil, err
	}
	padding := c.block.BlockSize() - len(plain)%c.block.BlockSize()
	plain = append(plain, bytes.Repeat([]byte{byte(padding)}, padding)...)
	ciphertext := make([]byte, len(plain))
	cipher.NewCBCEncrypter(c.block, c.iv).CryptBlocks(ciphertext, plain)
	return ciphertext, nil
}

func (c *jsSymmetricCrypto) EncryptBase64(data interface{}) (string, error) {
	ciphertext, err := c.Encrypt(data)
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

func (c *jsSymmetricCrypto) EncryptHex(data interface{}) (string, error) {
	ciphertext, err := c.Encrypt(data)
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(ciphertext), nil
}

func (c *jsSymmetricCrypto) Decrypt(data interface{}) ([]byte, error) {
	var ciphertext []byte
	switch value := data.(type) {
	case string:
		var err error
		if decoded, decodeErr := hex.DecodeString(value); decodeErr == nil && len(value)%2 == 0 {
			ciphertext = decoded
		} else {
			ciphertext, err = base64.StdEncoding.DecodeString(value)
			if err != nil {
				return nil, fmt.Errorf("js crypto: decode ciphertext: %w", err)
			}
		}
	default:
		var err error
		ciphertext, err = jsBytes(value)
		if err != nil {
			return nil, err
		}
	}
	if len(ciphertext) == 0 || len(ciphertext)%c.block.BlockSize() != 0 {
		return nil, fmt.Errorf("js crypto: ciphertext is not block aligned")
	}
	plain := make([]byte, len(ciphertext))
	cipher.NewCBCDecrypter(c.block, c.iv).CryptBlocks(plain, ciphertext)
	padding := int(plain[len(plain)-1])
	if padding == 0 || padding > c.block.BlockSize() || padding > len(plain) {
		return nil, fmt.Errorf("js crypto: invalid padding")
	}
	for _, value := range plain[len(plain)-padding:] {
		if int(value) != padding {
			return nil, fmt.Errorf("js crypto: invalid padding")
		}
	}
	return plain[:len(plain)-padding], nil
}

func (c *jsSymmetricCrypto) DecryptStr(data interface{}) (string, error) {
	plain, err := c.Decrypt(data)
	if err != nil {
		return "", err
	}
	return string(plain), nil
}
