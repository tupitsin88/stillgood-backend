package offers

import (
	"github.com/gin-gonic/gin"
)

func RegisterRoutes(r *gin.Engine, h *OfferHandler, authMiddleware gin.HandlerFunc) {
	public := r.Group("/api/v1")
	{
		public.GET("/offers", h.GetPublicOffers)
		public.GET("/offers/:id", h.GetOfferByID)
	}

	partner := r.Group("/api/v1/partner")
	partner.Use(authMiddleware)
	// Тут хорошо бы добавить middleware проверки роли, например: roleMiddleware("PARTNER")
	{
		partner.GET("/offers", h.GetPartnerOffers)
		partner.POST("/offers", h.CreateOffer)
		partner.PATCH("/offers/:id", h.UpdateOffer)
	}
}
