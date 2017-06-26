package main

import (
	"context"
	jwt "github.com/dgrijalva/jwt-go"
	"net/http"
)

type JWTMiddelware struct {
	SecretKey string
	Parser    *jwt.Parser
}

func NewJWT(key string) *JWTMiddelware {
	return &JWTMiddelware{
		SecretKey: key,
		Parser: &jwt.Parser{
			UseJSONNumber: true,
		},
	}
}

func (this *JWTMiddelware) ServeHTTP(rw http.ResponseWriter, r *http.Request, next http.HandlerFunc) {
	authToken := r.Header.Get("X-AUTH-TOKEN")
	if authToken == "" {
		http.Error(rw, "You should pass X-AUTH-TOKEN in Headers", http.StatusForbidden)
		return
	}

	parsedToken, err := this.Parser.Parse(authToken, func(token *jwt.Token) (interface{}, error) {
		// Don't forget to validate the alg is what you expect:

		//jwt.SigningMethodHS256.Verify()
		//if _, ok := token.Method.(*jwt.SigningMethodRS256); !ok {
		//	return nil, fmt.Errorf("Unexpected signing method: %v", token.Header["alg"])
		//}

		return []byte(this.SecretKey), nil
	})

	if err != nil {
		http.Error(rw, "Your X-AUTH-TOKEN is not valid", http.StatusForbidden)
		return
	}

	if !parsedToken.Valid {
		http.Error(rw, "Your X-AUTH-TOKEN is not valid", http.StatusForbidden)
		return
	}

	ctx := context.WithValue(r.Context(), "jwt", parsedToken)
	next(rw, r.WithContext(ctx))
}
