package auth

import "golang.org/x/crypto/bcrypt"

func HashPass(pass string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(pass), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(hash), nil

}

func ValPass(hashedpass, pass string) error {
	return bcrypt.CompareHashAndPassword([]byte(hashedpass), []byte(pass))
}
