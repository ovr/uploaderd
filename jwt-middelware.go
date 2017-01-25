package main

import (
	jwt "github.com/dgrijalva/jwt-go"
	"github.com/kataras/iris"
)

func createJWTMiddelWare(config JWTConfig) iris.HandlerFunc {

	return iris.HandlerFunc(func(ctx *iris.Context) {
		authToken := ctx.RequestHeader("X-AUTH-TOKEN")
		if authToken == "" {
			ctx.JSON(
				iris.StatusForbidden,
				newErrorJson("You should pass X-AUTH-TOKEN in Headers"),
			)
			return
		}

		parser := &jwt.Parser{
			UseJSONNumber: true,
		}

		parsedToken, err := parser.Parse(authToken, func(token *jwt.Token) (interface{}, error) {
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
			return
		}

		if !parsedToken.Valid {
			ctx.JSON(
				iris.StatusForbidden,
				newErrorJson("Your X-AUTH-TOKEN is not valid"),
			)
			return
		}

		ctx.Set("jwt", parsedToken)
		ctx.Next()
	})
}
