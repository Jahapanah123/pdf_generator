package handler

import (
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jahapanah123/pdf_generator/internal/domain"
	jwtpkg "github.com/jahapanah123/pdf_generator/internal/pkg/jwt"
)

type AuthHandler struct {
	jwt    *jwtpkg.Manager
	logger *slog.Logger
}

func NewAuthHandler(jwt *jwtpkg.Manager, logger *slog.Logger) *AuthHandler {
	return &AuthHandler{jwt: jwt, logger: logger}
}

type LoginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required,min=6"`
}

type RefreshRequest struct {
	RefreshToken string `json:"refresh_token" binding:"required"`
}

func (h *AuthHandler) Login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest,
			domain.NewAPIError(http.StatusBadRequest, "invalid request"))
		return
	}

	// In production: validate credentials against user DB
	userID := uuid.New().String()

	pair, err := h.jwt.GenerateTokenPair(userID, req.Email)
	if err != nil {
		h.logger.Error("generate tokens failed", slog.Any("error", err))
		c.JSON(http.StatusInternalServerError,
			domain.NewAPIError(http.StatusInternalServerError, "token generation failed"))
		return
	}

	c.JSON(http.StatusOK, pair)
}

func (h *AuthHandler) Refresh(c *gin.Context) {
	var req RefreshRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest,
			domain.NewAPIError(http.StatusBadRequest, "invalid request"))
		return
	}

	claims, err := h.jwt.ValidateRefreshToken(req.RefreshToken)
	if err != nil {
		c.JSON(http.StatusUnauthorized,
			domain.NewAPIError(http.StatusUnauthorized, "invalid refresh token"))
		return
	}

	pair, err := h.jwt.GenerateTokenPair(claims.UserID, claims.Email)
	if err != nil {
		h.logger.Error("generate tokens failed", slog.Any("error", err))
		c.JSON(http.StatusInternalServerError,
			domain.NewAPIError(http.StatusInternalServerError, "token generation failed"))
		return
	}

	c.JSON(http.StatusOK, pair)
}
