package document

import (
	"net/http"
	"strconv"
	"strings"

	"paperlink/db/repo"
	"paperlink/server/routes"

	"github.com/gin-gonic/gin"
)

type UserSuggestionResponse struct {
	ID       int    `json:"id"`
	Username string `json:"username"`
}

func SearchUsers(c *gin.Context) {
	userID := c.GetInt("userId")
	query := strings.TrimSpace(c.Query("q"))

	limit := 10
	if rawLimit := c.Query("limit"); rawLimit != "" {
		parsed, err := strconv.Atoi(rawLimit)
		if err == nil && parsed > 0 && parsed <= 20 {
			limit = parsed
		}
	}

	users, err := repo.User.SearchUsers(query, userID, limit)
	if err != nil {
		log.Errorf("failed to search users: %v", err)
		routes.JSONError(c, http.StatusInternalServerError, "failed to search users")
		return
	}

	out := make([]UserSuggestionResponse, 0, len(users))
	for _, user := range users {
		out = append(out, UserSuggestionResponse{
			ID:       user.ID,
			Username: user.Username,
		})
	}

	routes.JSONSuccessOK(c, out)
}
