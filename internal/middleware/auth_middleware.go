package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/jahapanah123/pdf_generator/internal/domain"
	jwtpkg "github.com/jahapanah123/pdf_generator/internal/pkg/jwt"
)

func JWTAuth(manager *jwtpkg.Manager) gin.HandlerFunc {
	return func(c *gin.Context) {
		header := c.GetHeader("Authorization")
		if header == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized,
				domain.NewAPIError(http.StatusUnauthorized, "authorization header required"),
			)
			return
		}

		parts := strings.SplitN(header, " ", 2)
		if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
			c.AbortWithStatusJSON(http.StatusUnauthorized,
				domain.NewAPIError(http.StatusUnauthorized, "invalid authorization format"),
			)
			return
		}

		claims, err := manager.ValidateAccessToken(parts[1])
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized,
				domain.NewAPIError(http.StatusUnauthorized, "invalid or expired token"),
			)
			return
		}

		c.Set("user_id", claims.UserID)
		c.Set("email", claims.Email)
		c.Next()
	}
}
