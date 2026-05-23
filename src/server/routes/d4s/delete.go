package d4s

import (
	"errors"
	"net/http"
	"strconv"

	"gorm.io/gorm"

	"paperlink/db/repo"
	"paperlink/server/routes"

	"github.com/gin-gonic/gin"
)

// DeleteBook godoc
// @Summary      Delete Digi4School book
// @Description  Deletes a synced Digi4School book and removes its stored file when unused.
// @Tags         digi4school
// @Param        id path int true "Book ID"
// @Success      204 "No Content"
// @Failure      400 {object} routes.ErrorResponse "Invalid book ID"
// @Failure      401 {object} routes.ErrorResponse "Unauthorized"
// @Failure      404 {object} routes.ErrorResponse "Book not found"
// @Failure      500 {object} routes.ErrorResponse "Internal server error"
// @Router       /api/v1/d4s/book/{id} [delete]
// @Security     BearerAuth
func DeleteBook(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		routes.JSONError(c, http.StatusBadRequest, "invalid book id")
		return
	}

	if err := repo.Digi4SchoolBook.DeleteWithUnusedFile(id); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			routes.JSONError(c, http.StatusNotFound, "book not found")
			return
		}
		log.Errorf("failed to delete d4s book %d: %v", id, err)
		routes.JSONError(c, http.StatusInternalServerError, "failed to delete book")
		return
	}

	c.Status(http.StatusNoContent)
}
