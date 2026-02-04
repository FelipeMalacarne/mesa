package domain

type Cryptographer interface {
	Encrypt(plainText string) (string, error)
	Decrypt(cipherText string) (string, error)
}
