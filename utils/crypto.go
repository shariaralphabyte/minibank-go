package utils

import (
    "crypto/aes"
    "crypto/cipher"
    "crypto/rand"
    "encoding/base64"
    "fmt"
    "io"
    "log"

    "golang.org/x/crypto/bcrypt"
)

var encryptionKey []byte

// InitializeEncryption sets up the encryption key
func InitializeEncryption(key string) error {
    if len(key) != 32 {
        return fmt.Errorf("encryption key must be exactly 32 characters, got %d", len(key))
    }
    encryptionKey = []byte(key)
    log.Println("Encryption initialized successfully")
    return nil
}

func HashPassword(password string) (string, error) {
    bytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
    return string(bytes), err
}

func CheckPasswordHash(password, hash string) bool {
    err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
    return err == nil
}

func EncryptSensitiveData(data string) (string, error) {
    if encryptionKey == nil {
        return "", fmt.Errorf("encryption key not initialized")
    }

    if data == "" {
        return "", nil // Return empty string for empty input
    }

    block, err := aes.NewCipher(encryptionKey)
    if err != nil {
        return "", fmt.Errorf("failed to create cipher: %v", err)
    }

    ciphertext := make([]byte, aes.BlockSize+len(data))
    iv := ciphertext[:aes.BlockSize]
    
    if _, err := io.ReadFull(rand.Reader, iv); err != nil {
        return "", fmt.Errorf("failed to generate IV: %v", err)
    }

    stream := cipher.NewCFBEncrypter(block, iv)
    stream.XORKeyStream(ciphertext[aes.BlockSize:], []byte(data))

    encoded := base64.URLEncoding.EncodeToString(ciphertext)
    log.Printf("Successfully encrypted data of length %d", len(data))
    return encoded, nil
}

func DecryptSensitiveData(encryptedData string) (string, error) {
    if encryptionKey == nil {
        return "", fmt.Errorf("encryption key not initialized")
    }

    if encryptedData == "" {
        return "", nil // Return empty string for empty input
    }

    ciphertext, err := base64.URLEncoding.DecodeString(encryptedData)
    if err != nil {
        return "", fmt.Errorf("failed to decode base64: %v", err)
    }

    block, err := aes.NewCipher(encryptionKey)
    if err != nil {
        return "", fmt.Errorf("failed to create cipher: %v", err)
    }

    if len(ciphertext) < aes.BlockSize {
        return "", fmt.Errorf("ciphertext too short")
    }

    iv := ciphertext[:aes.BlockSize]
    ciphertext = ciphertext[aes.BlockSize:]

    stream := cipher.NewCFBDecrypter(block, iv)
    stream.XORKeyStream(ciphertext, ciphertext)

    return string(ciphertext), nil
}