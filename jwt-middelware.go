package main

import (
	"github.com/kataras/iris"
	jwt "github.com/dgrijalva/jwt-go"
)

func createJWTMiddelWare(config JWTConfig) iris.HandlerFunc {

	return iris.HandlerFunc(func(ctx *iris.Context) {
		authToken := ctx.RequestHeader("X-AUTH-TOKEN");
		if authToken == "" {
			ctx.JSON(
				iris.StatusForbidden,
				newErrorJson("You should pass X-AUTH-TOKEN in Headers"),
			)
			return;
		}

		parser := &jwt.Parser{
			UseJSONNumber: true,
		}

		_, err := parser.Parse(authToken, func(token *jwt.Token) (interface{}, error) {
			// Don't forget to validate the alg is what you expect:

			//jwt.SigningMethodHS256.Verify()
			//if _, ok := token.Method.(*jwt.SigningMethodRS256); !ok {
			//	return nil, fmt.Errorf("Unexpected signing method: %v", token.Header["alg"])
			//}

			return []byte(config.SecretKey), nil
		})

		if err != nil {
			ctx.JSON(
				iris.StatusForbidden,
				newErrorJson("Your X-AUTH-TOKEN is not valid"),
			)
		} else {
			ctx.Next()
		}
	})
}
