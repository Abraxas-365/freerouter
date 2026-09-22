package httpx

import (
	"strconv"

	"github.com/Abraxas-365/freerouter/internal/query"
	"github.com/gofiber/fiber/v2"
)

// PaginationFromCtx parses ?limit= and ?offset= query params into a
// normalized query.Pagination. Missing or invalid values get defaults.
func PaginationFromCtx(c *fiber.Ctx) query.Pagination {
	limit, _ := strconv.Atoi(c.Query("limit"))
	offset, _ := strconv.Atoi(c.Query("offset"))
	return query.Pagination{Limit: limit, Offset: offset}.Normalize()
}
