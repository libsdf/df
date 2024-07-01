package f1

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
)

func KeySize() int {
	return aes.BlockSize
}

func NewKey() []byte {
	key := make([]byte, aes.BlockSize)
	rand.Read(key)
	return key
}

func Encrypt(key, dat []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	ciphertext := make([]byte, aes.BlockSize+len(dat))
	iv := ciphertext[:aes.BlockSize]
	if _, err := rand.Read(iv); err != nil {
		return nil, err
	}

	stream := cipher.NewCFBEncrypter(block, iv)
	stream.XORKeyStream(ciphertext[aes.BlockSize:], dat)

	return ciphertext, nil
}

func Decrypt(key, dat []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	iv := dat[:aes.BlockSize]
	plaintext := make([]byte, len(dat)-aes.BlockSize)

	stream := cipher.NewCFBDecrypter(block, iv)
	stream.XORKeyStream(plaintext, dat[aes.BlockSize:])

	return plaintext, nil
}

func Xor(x []byte, y []byte) []byte {
	lenX := len(x)
	lenY := len(y)
	rs := make([]byte, lenX+lenY)

	j := lenX
	if lenX < lenY {
		copy(rs, x)
		copy(rs[lenX:], y)
	} else {
		copy(rs, y)
		copy(rs[lenY:], x)
		j = lenY
	}

	for i := 0; i < j; i += 1 {
		rs[j+i] = rs[i] ^ rs[j+i]
	}

	return rs[j:]
}
