package public

import (
	"time"

	"github.com/google/uuid"

	"github.com/tokoonline/app/internal/models"
)

type orderRow struct {
	ID            uuid.UUID
	Code          string
	Channel       string
	Status        string
	PaymentStatus string
	GrandTotal    float64
	CreatedAt     time.Time
}

func toModelOrders(rs []*orderRow) []*models.Order {
	out := make([]*models.Order, 0, len(rs))
	for _, r := range rs {
		out = append(out, &models.Order{
			ID: r.ID, Code: r.Code, Channel: r.Channel, Status: r.Status, PaymentStatus: r.PaymentStatus, GrandTotal: r.GrandTotal, CreatedAt: r.CreatedAt,
		})
	}
	return out
}
