package authy

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

var (
	ErrParse = errors.New("неверные учетные данные")
	secretKey = []byte("your-secret-key")
)

type User struct {
	ID             int64
	Name           string
	Password       string
}
//генерация JWT токена
func GenerateJWT(ID int) (string, error){
	const hmacSampleSecret = "super_secret_signature"
	now := time.Now()
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"name": "user_name",
		"exp":  now.Add(24 * time.Hour).Unix(),
	})
	return token.SignedString(secretKey)
}

func ParseJWT(tokenstr string) (int, error){
	token, err := jwt.Parse(tokenstr, func(token *jwt.Token) (interface{}, error) {
		return secretKey, nil
	})
	if err != nil {
		return 0, err
	}

	if claims, ok := token.Claims.(jwt.MapClaims); ok && token.Valid {
		userID := int(claims["user_id"].(float64))
		return userID, nil
	}
	return 0, ErrParse
}

//создание хэша из пароля
func GenerateHash(password string) (string, error) {
	hashedBytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}

	hash := string(hashedBytes[:])
	return hash, nil
}
//сравнение хэша и пароля, что был введен
func CompareHash(hash string, password string) error {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
}
